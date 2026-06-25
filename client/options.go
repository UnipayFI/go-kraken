package client

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync/atomic"
	"time"

	krakenCommon "github.com/UnipayFI/go-kraken/common"
	"github.com/UnipayFI/go-kraken/pkg/log"
	"github.com/go-resty/resty/v2"
	"golang.org/x/net/proxy"
)

type Option struct {
	apiKey    string
	apiSecret string
	network   krakenCommon.Network
	baseURL   string
	logger    log.Logger
	signFn    SignFn
	nonceFn   func() int64
	client    *resty.Client
}

type Options func(*Option)

func defaultOption() *Option {
	return &Option{
		network: krakenCommon.Mainnet,
		logger:  log.GetDefaultLogger(),
		nonceFn: newNonceFn(),
		client:  defaultHttpClient(),
	}
}

func defaultHttpClient() *resty.Client {
	return resty.New().
		SetJSONMarshaler(krakenCommon.JSONMarshal).
		SetJSONUnmarshaler(krakenCommon.JSONUnmarshal)
}

// newNonceFn builds the default strictly-monotonic nonce generator. It seeds
// each value from the wall clock in nanoseconds (so nonces keep increasing
// across process restarts and comfortably exceed any millisecond/second-based
// nonce the key may have seen before) while an atomic floor guarantees strict
// monotonicity under bursts within the same nanosecond.
func newNonceFn() func() int64 {
	var last atomic.Int64
	return func() int64 {
		for {
			n := time.Now().UnixNano()
			prev := last.Load()
			if n <= prev {
				n = prev + 1
			}
			if last.CompareAndSwap(prev, n) {
				return n
			}
		}
	}
}

// WithAuth sets the API credentials used to sign private requests. Both values
// come from the Kraken API-management page; apiSecret is the base64-encoded
// "Private Key" shown when the key was created.
func WithAuth(apiKey, apiSecret string) Options {
	return func(opt *Option) {
		opt.apiKey = apiKey
		opt.apiSecret = apiSecret
	}
}

// WithNetwork selects the Kraken environment. Kraken exposes a single
// production domain for spot, so this currently only accepts common.Mainnet; it
// exists for forward symmetry with sibling SDKs.
func WithNetwork(network krakenCommon.Network) Options {
	return func(opt *Option) {
		opt.network = network
	}
}

// WithBaseURL overrides the REST base URL derived from WithNetwork. Use it to
// point the client at a custom or proxied endpoint. An empty value is ignored.
func WithBaseURL(baseURL string) Options {
	return func(opt *Option) {
		opt.baseURL = baseURL
	}
}

func WithLogger(logger log.Logger) Options {
	return func(opt *Option) {
		opt.logger = logger
	}
}

// WithSignFn replaces the default HMAC-SHA512 signer. Use it to delegate
// signing to an HSM / remote signer.
func WithSignFn(signFn SignFn) Options {
	return func(opt *Option) {
		opt.signFn = signFn
	}
}

// WithNonce replaces the default nonce generator. The supplied function must
// return a strictly increasing value on each call for the lifetime of the API
// key (Kraken rejects a non-increasing nonce). Use it to share one nonce
// sequence across several clients, or to switch to a millisecond/microsecond
// resolution to match another tool already using the key.
func WithNonce(nonceFn func() int64) Options {
	return func(opt *Option) {
		if nonceFn != nil {
			opt.nonceFn = nonceFn
		}
	}
}

// WithHTTPClient supplies a pre-configured resty client (custom transport,
// timeouts, TLS, etc.). The JSON (un)marshalers and base URL are still set by
// the SDK afterwards.
func WithHTTPClient(client *resty.Client) Options {
	return func(opt *Option) {
		if client != nil {
			opt.client = client
		}
	}
}

// WithProxy routes all REST traffic through the given proxy. Supported schemes:
// http, https, socks5, socks5h. Pass userinfo in the URL for authenticated
// proxies. Invalid URLs are logged and skipped.
func WithProxy(proxyURL string) Options {
	return func(opt *Option) {
		if proxyURL == "" {
			return
		}
		u, err := url.Parse(proxyURL)
		if err != nil {
			opt.logger.Errorf("WithProxy: invalid proxy URL %q: %v", proxyURL, err)
			return
		}
		switch strings.ToLower(u.Scheme) {
		case "http", "https":
			opt.client.SetProxy(proxyURL)
		case "socks5", "socks5h":
			dialCtx, err := socks5DialContext(u)
			if err != nil {
				opt.logger.Errorf("WithProxy: socks5 setup failed: %v", err)
				return
			}
			transport := cloneDefaultTransport()
			transport.Proxy = nil
			transport.DialContext = dialCtx
			opt.client.SetTransport(transport)
		default:
			opt.logger.Errorf("WithProxy: unsupported scheme %q (want http, https, socks5, socks5h)", u.Scheme)
		}
	}
}

// socks5DialContext builds a DialContext that tunnels TCP through the SOCKS5
// proxy described by u. socks5h is accepted as an alias of socks5: the SOCKS5
// dialer in golang.org/x/net/proxy already resolves hostnames remotely.
func socks5DialContext(u *url.URL) (func(ctx context.Context, network, addr string) (net.Conn, error), error) {
	su := *u
	if strings.EqualFold(su.Scheme, "socks5h") {
		su.Scheme = "socks5"
	}
	pd, err := proxy.FromURL(&su, proxy.Direct)
	if err != nil {
		return nil, err
	}
	if cd, ok := pd.(proxy.ContextDialer); ok {
		return cd.DialContext, nil
	}
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		return pd.Dial(network, addr)
	}, nil
}

func cloneDefaultTransport() *http.Transport {
	if t, ok := http.DefaultTransport.(*http.Transport); ok {
		return t.Clone()
	}
	panic(fmt.Sprintf("kraken: http.DefaultTransport is not *http.Transport (got %T)", http.DefaultTransport))
}
