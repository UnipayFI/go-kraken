package kraken

import (
	"context"

	"github.com/UnipayFI/go-kraken/client"
	"github.com/UnipayFI/go-kraken/request"
)

var _ request.WsClient = (*WebSocketClient)(nil)

// WsPush is the channel data envelope Kraken pushes ({channel, type, data}). Type
// is WsPushTypeSnapshot (full) or WsPushTypeUpdate (incremental). It is
// re-exported from the request package for convenience: a callback receives
// *kraken.WsPush[[]WsTicker] and similar.
type WsPush[T any] = request.WsPush[T]

// WsPushType classifies a channel data frame as a full snapshot or an
// incremental update. Re-exported from the request package.
type WsPushType = request.WsPushType

const (
	WsPushTypeSnapshot = request.WsPushTypeSnapshot // full snapshot frame
	WsPushTypeUpdate   = request.WsPushTypeUpdate   // incremental update frame
)

// WebSocketClient streams Kraken's v2 public and private channels and supports
// WebSocket order entry. Public channels need no credentials; private channels
// and order entry require client.WithWebSocketAuth (used to fetch a short-lived
// auth token from the REST GetWebSocketsToken endpoint).
type WebSocketClient struct {
	*client.WebSocketClient
}

// NewWebSocketClient constructs a Kraken v2 WebSocket client. When
// client.WithWebSocketAuth is supplied, the token required by private channels
// and order entry is fetched (and cached, honoring WithWebSocketProxy) from the
// REST API automatically.
func NewWebSocketClient(options ...client.WebSocketOptions) *WebSocketClient {
	wc := client.NewWebSocketClient(options...)
	if wc.GetAPIKey() != "" && wc.GetAPISecret() != "" {
		restOpts := []client.Options{client.WithAuth(wc.GetAPIKey(), wc.GetAPISecret())}
		if p := wc.GetProxyURL(); p != "" {
			restOpts = append(restOpts, client.WithProxy(p))
		}
		rest := NewClient(restOpts...)
		wc.SetTokenFn(func(ctx context.Context) (string, int64, error) {
			tok, err := rest.NewGetWebSocketsTokenService().Do(ctx)
			if err != nil {
				return "", 0, err
			}
			return tok.Token, tok.Expires, nil
		})
	}
	return &WebSocketClient{wc}
}

// SubscribeRaw is the low-level escape hatch: it subscribes to an arbitrary
// channel (public or private) with the given params and delivers each data
// frame's raw bytes. Prefer the typed NewSubscribe* services.
func (c *WebSocketClient) SubscribeRaw(ctx context.Context, channel string, params map[string]any, private bool, cb func([]byte, error)) (chan<- struct{}, <-chan struct{}, error) {
	return request.SubscribeRaw(ctx, c, channel, params, private, cb)
}
