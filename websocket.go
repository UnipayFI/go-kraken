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
