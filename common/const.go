package common

import "time"

const (
	GO_KRAKEN_USER_AGENT = "go-kraken/1.0"

	// REST endpoint. Kraken serves every spot business line (market data,
	// account, trading, funding, earn) from a single domain; the API version
	// ("0") and the public/private split are encoded in the request path, not
	// the host.
	DEFAULT_REST_BASE_URL = "https://api.kraken.com"

	// API version prefix shared by every REST path: /0/public/* and /0/private/*.
	API_VERSION = "0"

	// WebSocket endpoints. Kraken's v2 spot streams; public needs no auth,
	// private requires a token from the REST GetWebSocketsToken endpoint.
	DEFAULT_WS_PUBLIC_URL  = "wss://ws.kraken.com/v2"
	DEFAULT_WS_PRIVATE_URL = "wss://ws-auth.kraken.com/v2"

	// DEFAULT_WS_PING_INTERVAL is how often the client sends a {"method":"ping"}
	// keepalive frame on a stream connection.
	DEFAULT_WS_PING_INTERVAL = 30 * time.Second
)

// Network identifies which Kraken environment a client targets. Kraken exposes a
// single production domain for spot; the type is kept for forward symmetry with
// sibling SDKs and to leave room for a future dedicated environment.
type Network int

const (
	Mainnet Network = iota
)

// RestBaseURL returns the REST base URL for this network.
func (n Network) RestBaseURL() string {
	return DEFAULT_REST_BASE_URL
}

// WsPublicURL returns the public WebSocket URL for this network.
func (n Network) WsPublicURL() string {
	return DEFAULT_WS_PUBLIC_URL
}

// WsPrivateURL returns the private WebSocket URL for this network.
func (n Network) WsPrivateURL() string {
	return DEFAULT_WS_PRIVATE_URL
}
