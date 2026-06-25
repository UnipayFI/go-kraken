package request

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"github.com/UnipayFI/go-kraken/common"
	"github.com/UnipayFI/go-kraken/pkg/log"
	"github.com/gorilla/websocket"
)

// WsTradeConn is a persistent private connection for placing/managing orders
// over WebSocket. Unlike the subscription channels it is request/response: each
// call sends a method frame tagged with a unique req_id and waits for the
// matching reply. One connection serves any number of concurrent calls. The
// auth token (fetched at dial) is injected into every request's params.
type WsTradeConn struct {
	conn      *websocket.Conn
	token     string
	logger    log.Logger
	mu        sync.Mutex
	pending   map[int64]chan []byte
	nextID    atomic.Int64
	done      chan struct{}
	closeOnce sync.Once
}

// WsTradeResponse is the reply to a trade method frame. Result carries the
// method-specific payload (e.g. the order id).
type WsTradeResponse[T any] struct {
	Method  string `json:"method"`
	ReqID   int64  `json:"req_id"`
	Result  T      `json:"result"`
	Success bool   `json:"success"`
	Error   string `json:"error"`
	TimeIn  string `json:"time_in"`
	TimeOut string `json:"time_out"`
}

// DialWsTrade opens the private v2 gateway, fetches an auth token, and returns a
// ready trade connection. Close it when done.
func DialWsTrade(ctx context.Context, c WsClient) (*WsTradeConn, error) {
	tok, err := c.Token(ctx)
	if err != nil {
		return nil, err
	}
	conn, _, err := c.GetDialer().DialContext(ctx, c.GetPrivateURL(), nil)
	if err != nil {
		return nil, err
	}
	conn.SetReadLimit(10 << 20)
	t := &WsTradeConn{
		conn:    conn,
		token:   tok,
		logger:  c.GetLogger(),
		pending: make(map[int64]chan []byte),
		done:    make(chan struct{}),
	}
	go t.readLoop()
	go t.keepAlive()
	return t, nil
}

func (t *WsTradeConn) readLoop() {
	for {
		_, msg, err := t.conn.ReadMessage()
		if err != nil {
			t.failAll()
			return
		}
		t.logger.Debugf("ws trade recv: %s", common.BytesToString(msg))
		var hdr struct {
			ReqID int64 `json:"req_id"`
		}
		if err := common.JSONUnmarshal(msg, &hdr); err != nil || hdr.ReqID == 0 {
			continue // status/heartbeat/other control frame
		}
		t.mu.Lock()
		ch := t.pending[hdr.ReqID]
		delete(t.pending, hdr.ReqID)
		t.mu.Unlock()
		if ch != nil {
			ch <- msg
		}
	}
}

func (t *WsTradeConn) keepAlive() {
	ticker := time.NewTicker(common.DEFAULT_WS_PING_INTERVAL)
	defer ticker.Stop()
	ping, _ := common.JSONMarshal(wsRequest{Method: "ping", ReqID: t.nextID.Add(1)})
	for {
		select {
		case <-t.done:
			return
		case <-ticker.C:
			if err := t.conn.WriteMessage(websocket.TextMessage, ping); err != nil {
				return
			}
		}
	}
}

func (t *WsTradeConn) failAll() {
	t.mu.Lock()
	for id, ch := range t.pending {
		close(ch)
		delete(t.pending, id)
	}
	t.mu.Unlock()
}

func (t *WsTradeConn) clearPending(id int64) {
	t.mu.Lock()
	delete(t.pending, id)
	t.mu.Unlock()
}

// call sends a method frame (token injected into params) and blocks for its
// reply (or ctx cancellation).
func (t *WsTradeConn) call(ctx context.Context, method string, params map[string]any) ([]byte, error) {
	id := t.nextID.Add(1)
	ch := make(chan []byte, 1)
	t.mu.Lock()
	t.pending[id] = ch
	t.mu.Unlock()

	p := map[string]any{"token": t.token}
	for k, v := range params {
		p[k] = v
	}
	req := wsRequest{Method: method, Params: p, ReqID: id}
	data, err := common.JSONMarshal(req)
	if err != nil {
		t.clearPending(id)
		return nil, err
	}
	if err := t.conn.WriteMessage(websocket.TextMessage, data); err != nil {
		t.clearPending(id)
		return nil, err
	}

	select {
	case msg, ok := <-ch:
		if !ok {
			return nil, errors.New("ws trade: connection closed")
		}
		return msg, nil
	case <-ctx.Done():
		t.clearPending(id)
		return nil, ctx.Err()
	case <-t.done:
		return nil, errors.New("ws trade: connection closed")
	}
}

// WsTradeCall sends a trade method and decodes the reply, returning a *WsError
// on a non-success response.
func WsTradeCall[T any](ctx context.Context, t *WsTradeConn, method string, params map[string]any) (*WsTradeResponse[T], error) {
	msg, err := t.call(ctx, method, params)
	if err != nil {
		return nil, err
	}
	var resp WsTradeResponse[T]
	if err := common.JSONUnmarshal(msg, &resp); err != nil {
		return nil, err
	}
	if !resp.Success {
		return nil, &WsError{Message: resp.Error}
	}
	return &resp, nil
}

// Close terminates the connection and fails any in-flight calls.
func (t *WsTradeConn) Close() error {
	var err error
	t.closeOnce.Do(func() {
		close(t.done)
		err = t.conn.Close()
	})
	return err
}
