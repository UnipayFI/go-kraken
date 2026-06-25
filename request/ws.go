package request

import (
	"context"
	"sync/atomic"
	"time"

	"github.com/UnipayFI/go-kraken/common"
	"github.com/UnipayFI/go-kraken/pkg/log"
	"github.com/gorilla/websocket"
)

// WsClient is what the subscribe/trade framework needs from a
// *client.WebSocketClient.
type WsClient interface {
	GetPublicURL() string
	GetPrivateURL() string
	GetLogger() log.Logger
	GetDialer() *websocket.Dialer
	Token(ctx context.Context) (string, error)
}

// WsPush is the envelope Kraken pushes for a channel data event. Type is
// "snapshot" (full) or "update" (incremental).
type WsPush[T any] struct {
	Channel string `json:"channel"`
	Type    string `json:"type"`
	Data    T      `json:"data"`
}

// wsRequest is a Kraken v2 control frame (subscribe/unsubscribe/ping/...).
type wsRequest struct {
	Method string         `json:"method"`
	Params map[string]any `json:"params,omitempty"`
	ReqID  int64          `json:"req_id"`
}

// wsHeader is a lightweight view used to classify an inbound frame before
// committing to a typed decode.
type wsHeader struct {
	Channel string `json:"channel"`
	Type    string `json:"type"`
	Method  string `json:"method"`
	Success *bool  `json:"success"`
	Error   string `json:"error"`
}

var wsReqID atomic.Int64

func nextReqID() int64 { return wsReqID.Add(1) }

// Subscribe opens a dedicated connection to the public or private v2 gateway,
// subscribes to the channel with the given params (injecting the auth token when
// private), and invokes cb for every data push (decoded into *WsPush[T]). It
// returns a done channel (close to stop) and a stop channel (closed when the
// reader exits).
func Subscribe[T any](ctx context.Context, c WsClient, channel string, params map[string]any, private bool, cb func(*WsPush[T], error)) (done chan<- struct{}, stop <-chan struct{}, err error) {
	return subscribeBytes(ctx, c, channel, params, private, func(message []byte, e error) {
		if e != nil {
			cb(nil, e)
			return
		}
		var push WsPush[T]
		if err := common.JSONUnmarshal(message, &push); err != nil {
			cb(nil, err)
			return
		}
		cb(&push, nil)
	})
}

// SubscribeRaw is like Subscribe but delivers each data frame's raw bytes.
func SubscribeRaw(ctx context.Context, c WsClient, channel string, params map[string]any, private bool, cb func(message []byte, err error)) (done chan<- struct{}, stop <-chan struct{}, err error) {
	return subscribeBytes(ctx, c, channel, params, private, cb)
}

func subscribeBytes(ctx context.Context, c WsClient, channel string, params map[string]any, private bool, cb func(message []byte, err error)) (done chan<- struct{}, stop <-chan struct{}, err error) {
	endpoint := c.GetPublicURL()
	if private {
		endpoint = c.GetPrivateURL()
	}
	conn, _, err := c.GetDialer().DialContext(ctx, endpoint, nil)
	if err != nil {
		return nil, nil, err
	}
	conn.SetReadLimit(10 << 20)

	p := map[string]any{"channel": channel}
	for k, v := range params {
		p[k] = v
	}
	if private {
		tok, terr := c.Token(ctx)
		if terr != nil {
			conn.Close()
			return nil, nil, terr
		}
		p["token"] = tok
	}

	sub := wsRequest{Method: "subscribe", Params: p, ReqID: nextReqID()}
	data, _ := common.JSONMarshal(sub)
	if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
		conn.Close()
		return nil, nil, err
	}

	doneC := make(chan struct{})
	stopC := make(chan struct{})
	silent := false

	go wsKeepAlive(conn, common.DEFAULT_WS_PING_INTERVAL)
	go func() {
		select {
		case <-stopC:
			silent = true
		case <-doneC:
		}
		// Best-effort unsubscribe before closing.
		unsub := wsRequest{Method: "unsubscribe", Params: p, ReqID: nextReqID()}
		if b, e := common.JSONMarshal(unsub); e == nil {
			_ = conn.WriteMessage(websocket.TextMessage, b)
		}
		conn.Close()
	}()
	go func() {
		for {
			_, message, err := conn.ReadMessage()
			if err != nil {
				if !silent {
					cb(nil, err)
				}
				close(stopC)
				return
			}
			c.GetLogger().Debugf("ws recv: %s", common.BytesToString(message))

			var hdr wsHeader
			if err := common.JSONUnmarshal(message, &hdr); err != nil {
				cb(nil, err)
				continue
			}
			switch {
			case hdr.Method == "subscribe":
				// Subscription acknowledgement: surface a failure, ignore success.
				if hdr.Success != nil && !*hdr.Success {
					cb(nil, &WsError{Message: subErr(hdr)})
				}
			case hdr.Method != "":
				// pong / unsubscribe ack / other control frames: ignore.
			case hdr.Error != "":
				cb(nil, &WsError{Message: hdr.Error})
			case hdr.Channel == channel && hdr.Type != "":
				cb(message, nil)
			default:
				// heartbeat, status, and other channels' frames: ignore.
			}
		}
	}()
	return doneC, stopC, nil
}

func subErr(h wsHeader) string {
	if h.Error != "" {
		return h.Error
	}
	return "subscription failed"
}

// wsKeepAlive sends Kraken's {"method":"ping"} frame on an interval; the server
// replies with a pong (ignored in the read loop) and pushes a heartbeat channel
// every second of its own accord.
func wsKeepAlive(conn *websocket.Conn, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	ping, _ := common.JSONMarshal(wsRequest{Method: "ping", ReqID: nextReqID()})
	for range ticker.C {
		if err := conn.WriteMessage(websocket.TextMessage, ping); err != nil {
			return
		}
	}
}

// WsError is a Kraken WebSocket control-frame error.
type WsError struct {
	Message string
}

func (e *WsError) Error() string {
	return "<WsError> " + e.Message
}
