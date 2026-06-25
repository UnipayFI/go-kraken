package client

import (
	"github.com/UnipayFI/go-kraken/pkg/log"
	"github.com/go-resty/resty/v2"
)

// SignFn produces the API-Sign header value for a private Kraken request. The
// default implementation (request.HMACSign) computes
//
//	base64( HMAC-SHA512( base64decode(secret), uriPath + SHA256(nonce + postData) ) )
//
// Supply a custom one via WithSignFn to delegate signing to an HSM or remote
// signer (in which case secret may carry an opaque key handle).
type SignFn = func(secret, uriPath, nonce, postData string) (signature string, err error)

// Client is the shared, product-agnostic REST core. Kraken's whole spot API
// (market data, account, trading, funding, earn) is just a set of request paths
// layered on top of this same signing + transport machinery, so the core
// carries no endpoint-specific state.
type Client struct {
	client *resty.Client

	apiKey    string
	apiSecret string
	signFn    SignFn
	nonceFn   func() int64
	logger    log.Logger
}

func NewClient(options ...Options) *Client {
	opt := defaultOption()
	for _, option := range options {
		option(opt)
	}

	baseURL := opt.network.RestBaseURL()
	if opt.baseURL != "" {
		baseURL = opt.baseURL
	}
	opt.client.SetBaseURL(baseURL)

	return &Client{
		client:    opt.client,
		apiKey:    opt.apiKey,
		apiSecret: opt.apiSecret,
		signFn:    opt.signFn,
		nonceFn:   opt.nonceFn,
		logger:    opt.logger,
	}
}

func (c *Client) GetHttpClient() *resty.Client { return c.client }

func (c *Client) GetAPIKey() string { return c.apiKey }

func (c *Client) GetAPISecret() string { return c.apiSecret }

func (c *Client) GetLogger() log.Logger { return c.logger }

func (c *Client) GetSignFn() SignFn { return c.signFn }

// Nonce returns the next request nonce: an always-increasing, unsigned 64-bit
// integer required on every private request and folded into its signature.
// Kraken rejects a nonce that is not strictly greater than the previous one
// seen for the key, so the generator is strictly monotonic.
func (c *Client) Nonce() int64 { return c.nonceFn() }
