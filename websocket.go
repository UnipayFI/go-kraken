package kraken

import (
	"context"

	"github.com/UnipayFI/go-kraken/request"
)

// ===========================================================================
// Get WebSockets Token -- POST /0/private/GetWebSocketsToken
// ===========================================================================

// GetWebSocketsTokenService issues a single-use authentication token for
// connecting to Kraken's private WebSocket API. The token must be used within
// 15 minutes of creation; once a successful WebSocket connection is established
// it remains valid for the life of that connection.
//
// The same token (requested with a Kraken API key, not a Derivatives API key)
// also authenticates the Unified WebSocket (wss://uws.kraken.com/v2/private),
// which replaces the Derivatives WebSocket (wss://futures.kraken.com/ws/v1,
// deprecated, removed 1 March 2027). The Unified WebSocket supports
// asset_class=derivatives only until Spot support lands (planned Q1 2027);
// WebSocketClient targets the Spot WebSocket v2 and does not send asset_class.
type GetWebSocketsTokenService struct {
	c *Client
}

func (c *Client) NewGetWebSocketsTokenService() *GetWebSocketsTokenService {
	return &GetWebSocketsTokenService{c: c}
}

func (s *GetWebSocketsTokenService) Do(ctx context.Context) (*WebSocketsToken, error) {
	return request.Do[WebSocketsToken](request.Post(ctx, s.c, "/0/private/GetWebSocketsToken").WithSign())
}

// WebSocketsToken is the private-WebSocket auth token.
type WebSocketsToken struct {
	Token   string `json:"token"`   // websocket authentication token
	Expires int64  `json:"expires"` // seconds until the token expires (e.g. 900)
}
