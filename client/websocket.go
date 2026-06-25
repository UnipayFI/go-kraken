package client

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	krakenCommon "github.com/UnipayFI/go-kraken/common"
	"github.com/UnipayFI/go-kraken/pkg/log"
	"github.com/gorilla/websocket"
)

// TokenFn fetches a fresh Kraken WebSocket authentication token together with
// its validity in seconds (via the REST GetWebSocketsToken endpoint). It is
// injected by the SDK's WebSocket constructor so the client package itself does
// not import the REST request layer.
type TokenFn = func(ctx context.Context) (token string, expiresSeconds int64, err error)

// WebSocketClient holds the configuration shared by every Kraken v2 stream.
// Public channels need no credentials; private channels and WebSocket order
// entry authenticate with a short-lived token obtained from the REST API and
// cached here.
type WebSocketClient struct {
	publicURL  string
	privateURL string
	apiKey     string
	apiSecret  string
	proxyURL   string
	logger     log.Logger
	dialer     *websocket.Dialer

	tokenFn  TokenFn
	mu       sync.Mutex
	token    string
	tokenExp time.Time
}

type WebSocketOption struct {
	network    krakenCommon.Network
	publicURL  string
	privateURL string
	apiKey     string
	apiSecret  string
	proxyURL   string
	logger     log.Logger
	dialer     *websocket.Dialer
}

type WebSocketOptions func(*WebSocketOption)

func defaultWebSocketOption() *WebSocketOption {
	return &WebSocketOption{
		network: krakenCommon.Mainnet,
		logger:  log.GetDefaultLogger(),
		dialer:  defaultDialer(),
	}
}

func defaultDialer() *websocket.Dialer {
	return &websocket.Dialer{
		Proxy:             http.ProxyFromEnvironment,
		HandshakeTimeout:  45 * time.Second,
		EnableCompression: true,
	}
}

func NewWebSocketClient(options ...WebSocketOptions) *WebSocketClient {
	opt := defaultWebSocketOption()
	for _, option := range options {
		option(opt)
	}
	publicURL := opt.network.WsPublicURL()
	if opt.publicURL != "" {
		publicURL = opt.publicURL
	}
	privateURL := opt.network.WsPrivateURL()
	if opt.privateURL != "" {
		privateURL = opt.privateURL
	}
	return &WebSocketClient{
		publicURL:  publicURL,
		privateURL: privateURL,
		apiKey:     opt.apiKey,
		apiSecret:  opt.apiSecret,
		proxyURL:   opt.proxyURL,
		logger:     opt.logger,
		dialer:     opt.dialer,
	}
}

func (c *WebSocketClient) GetPublicURL() string         { return c.publicURL }
func (c *WebSocketClient) GetPrivateURL() string        { return c.privateURL }
func (c *WebSocketClient) GetAPIKey() string            { return c.apiKey }
func (c *WebSocketClient) GetAPISecret() string         { return c.apiSecret }
func (c *WebSocketClient) GetProxyURL() string          { return c.proxyURL }
func (c *WebSocketClient) GetLogger() log.Logger        { return c.logger }
func (c *WebSocketClient) GetDialer() *websocket.Dialer { return c.dialer }

// SetTokenFn installs the token fetcher (wired by kraken.NewWebSocketClient from
// the REST GetWebSocketsToken endpoint).
func (c *WebSocketClient) SetTokenFn(fn TokenFn) { c.tokenFn = fn }

// Token returns a valid WebSocket auth token, fetching (and caching) a fresh one
// when the cached token is missing or near expiry. Kraken tokens are valid for
// ~15 minutes and may be reused across connections within that window.
func (c *WebSocketClient) Token(ctx context.Context) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.token != "" && time.Now().Before(c.tokenExp) {
		return c.token, nil
	}
	if c.tokenFn == nil {
		return "", errors.New("ws: no auth configured (client.WithWebSocketAuth)")
	}
	tok, expires, err := c.tokenFn(ctx)
	if err != nil {
		return "", err
	}
	c.token = tok
	// Refresh a little early to avoid using a token that expires mid-handshake.
	c.tokenExp = time.Now().Add(time.Duration(expires)*time.Second - 30*time.Second)
	return tok, nil
}

// WithWebSocketAuth sets the credentials used to fetch the WebSocket token
// required by private channels and order entry. Same key/secret as the REST
// client.WithAuth.
func WithWebSocketAuth(apiKey, apiSecret string) WebSocketOptions {
	return func(opt *WebSocketOption) {
		opt.apiKey = apiKey
		opt.apiSecret = apiSecret
	}
}

func WithWebSocketNetwork(network krakenCommon.Network) WebSocketOptions {
	return func(opt *WebSocketOption) { opt.network = network }
}

// WithWebSocketPublicURL overrides the public stream URL. Empty is ignored.
func WithWebSocketPublicURL(u string) WebSocketOptions {
	return func(opt *WebSocketOption) { opt.publicURL = u }
}

// WithWebSocketPrivateURL overrides the private (auth) stream URL. Empty is
// ignored.
func WithWebSocketPrivateURL(u string) WebSocketOptions {
	return func(opt *WebSocketOption) { opt.privateURL = u }
}

func WithWebSocketLogger(logger log.Logger) WebSocketOptions {
	return func(opt *WebSocketOption) { opt.logger = logger }
}

// WithWebSocketProxy routes the stream dial (and the REST token fetch) through
// the given proxy (http, https, socks5, socks5h). Invalid URLs are logged and
// skipped.
func WithWebSocketProxy(proxyURL string) WebSocketOptions {
	return func(opt *WebSocketOption) {
		if proxyURL == "" {
			return
		}
		u, err := url.Parse(proxyURL)
		if err != nil {
			opt.logger.Errorf("WithWebSocketProxy: invalid proxy URL %q: %v", proxyURL, err)
			return
		}
		opt.proxyURL = proxyURL
		switch strings.ToLower(u.Scheme) {
		case "http", "https":
			opt.dialer.Proxy = http.ProxyURL(u)
			opt.dialer.NetDialContext = nil
		case "socks5", "socks5h":
			dialCtx, err := socks5DialContext(u)
			if err != nil {
				opt.logger.Errorf("WithWebSocketProxy: socks5 setup failed: %v", err)
				return
			}
			opt.dialer.Proxy = nil
			opt.dialer.NetDialContext = dialCtx
		default:
			opt.logger.Errorf("WithWebSocketProxy: unsupported scheme %q", u.Scheme)
		}
	}
}
