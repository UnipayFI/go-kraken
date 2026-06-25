// Package kraken is a Go SDK for the Kraken spot exchange REST API.
//
// Install: go get github.com/UnipayFI/go-kraken
// Import:  import "github.com/UnipayFI/go-kraken"
//
// The SDK covers Kraken's spot REST API at https://api.kraken.com/0 — public
// market data plus the private account, trading, funding, subaccount and Earn
// endpoints. One signing/transport core (client + request + common) is shared
// by every endpoint; the public/private split and the endpoint name are encoded
// in the request path.
//
// Authentication uses Kraken's HMAC-SHA512 scheme: each private request carries
// an always-increasing nonce in its url-encoded body and an API-Sign header
// computed as base64(HMAC-SHA512(base64decode(secret), path + SHA256(nonce +
// postData))). See client.WithAuth.
//
// Quick start:
//
//	c := kraken.NewClient(client.WithAuth(apiKey, apiSecret))
//
//	// Public market data (no auth).
//	srv, _ := c.NewGetServerTimeService().Do(ctx)
//	fmt.Println(srv.UnixTime)
//
//	// Private account data.
//	bal, _ := c.NewGetAccountBalanceService().Do(ctx)
//	fmt.Println(bal["XXBT"])
package kraken

import (
	"github.com/UnipayFI/go-kraken/client"
	"github.com/UnipayFI/go-kraken/request"
)

var _ request.Client = (*Client)(nil)

// Client is the Kraken spot REST client. It embeds the shared transport/signing
// core and exposes every endpoint as a NewXxxService(...).Do(ctx) method.
type Client struct {
	*client.Client
}

// NewClient constructs a Kraken spot REST client from the standard options
// (client.WithAuth, client.WithProxy, client.WithBaseURL, ...).
func NewClient(options ...client.Options) *Client {
	return &Client{client.NewClient(options...)}
}
