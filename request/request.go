package request

import (
	"context"
	"net/http"
	"net/url"
	"strings"

	"github.com/UnipayFI/go-kraken/common"
	"github.com/UnipayFI/go-kraken/pkg/log"
	"github.com/go-resty/resty/v2"
)

// Client is what every endpoint Service needs from the Kraken REST client. All
// getters are read-only; the concrete *client.Client satisfies it.
type Client interface {
	GetHttpClient() *resty.Client
	GetAPIKey() string
	GetAPISecret() string
	GetLogger() log.Logger
	GetSignFn() SignFn
	Nonce() int64
}

type Request struct {
	client   Client
	r        *resty.Request
	method   string
	path     string
	params   url.Values
	needSign bool
	err      error
}

func newRequest(ctx context.Context, c Client, method, path string) *Request {
	r := c.GetHttpClient().R().
		SetHeader("User-Agent", common.GO_KRAKEN_USER_AGENT).
		SetContext(ctx)
	r.Method = method
	return &Request{
		client: c,
		r:      r,
		method: method,
		path:   path,
		params: url.Values{},
	}
}

// Get builds a GET request for a public endpoint. Any params maps are merged
// (empty values dropped) into the query string.
func Get(ctx context.Context, c Client, path string, params ...map[string]string) *Request {
	r := newRequest(ctx, c, http.MethodGet, path)
	r.setParams(params...)
	return r
}

// Post builds a POST request, used for both public and (with WithSign) private
// endpoints. Any params maps are merged into the url-encoded form body.
func Post(ctx context.Context, c Client, path string, params ...map[string]string) *Request {
	r := newRequest(ctx, c, http.MethodPost, path)
	r.setParams(params...)
	return r
}

func (r *Request) setParams(params ...map[string]string) {
	for _, p := range params {
		for k, v := range p {
			if v == "" {
				continue
			}
			r.params.Set(k, v)
		}
	}
}

// SetParam sets a single request parameter, overriding any existing value. An
// empty value is ignored (use it to skip optional fields without branching at
// the call site).
func (r *Request) SetParam(key, value string) *Request {
	if value != "" {
		r.params.Set(key, value)
	}
	return r
}

// WithSign marks the request as private: a nonce is injected into the body and
// the API-Key / API-Sign headers are attached at send time. Public market
// endpoints omit this.
func (r *Request) WithSign() *Request {
	r.needSign = true
	return r
}

// fullURL builds the absolute request URL. For GET the (sorted) params become
// the query string; for POST they travel in the body instead.
func (r *Request) fullURL() string {
	base := strings.TrimSuffix(r.client.GetHttpClient().BaseURL, "/")
	path := r.path
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	urlStr := base + path
	if r.method == http.MethodGet && len(r.params) > 0 {
		urlStr += "?" + r.params.Encode()
	}
	return urlStr
}
