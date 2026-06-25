// Command kraw signs and executes a single Kraken spot REST call and pretty
// prints the raw response. It is a development aid for capturing the exact shape
// of private endpoints (which cannot be curled without HMAC-SHA512 signing) so
// the typed response structs can be reconciled against reality.
//
// Usage:
//
//	KRAKEN_API_KEY=... KRAKEN_API_SECRET=... \
//	  go run ./cmd/kraw GET  /0/public/Time
//	  go run ./cmd/kraw GET  /0/public/Ticker "pair=XBTUSD"
//	  go run ./cmd/kraw POST /0/private/Balance
//	  go run ./cmd/kraw POST /0/private/Ledgers "asset=XBT&type=deposit"
//
// The third argument is the query string (GET) or url-encoded form body
// (POST). Set KRAKEN_PROXY to route through a proxy.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/UnipayFI/go-kraken/client"
	"github.com/UnipayFI/go-kraken/request"
)

func main() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "usage: kraw <GET|POST> <path> [query-or-formbody]")
		os.Exit(2)
	}
	method := strings.ToUpper(os.Args[1])
	path := os.Args[2]
	arg := ""
	if len(os.Args) > 3 {
		arg = os.Args[3]
	}

	opts := []client.Options{
		client.WithAuth(os.Getenv("KRAKEN_API_KEY"), os.Getenv("KRAKEN_API_SECRET")),
	}
	if proxy := os.Getenv("KRAKEN_PROXY"); proxy != "" {
		opts = append(opts, client.WithProxy(proxy))
	}
	c := client.NewClient(opts...)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var req *request.Request
	switch method {
	case "GET":
		req = request.Get(ctx, c, path, parseParams(arg))
	case "POST":
		req = request.Post(ctx, c, path, parseParams(arg)).WithSign()
	default:
		fail("unsupported method %q", method)
	}

	body, err := request.DoRaw(req)
	if err != nil {
		fail("request error: %v", err)
	}
	fmt.Println(pretty(body))
}

func parseParams(q string) map[string]string {
	out := map[string]string{}
	q = strings.TrimPrefix(q, "?")
	for pair := range strings.SplitSeq(q, "&") {
		if pair == "" {
			continue
		}
		k, v, _ := strings.Cut(pair, "=")
		out[k] = v
	}
	return out
}

func pretty(b []byte) string {
	var v any
	if err := json.Unmarshal(b, &v); err != nil {
		return string(b)
	}
	out, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return string(b)
	}
	return string(out)
}

func fail(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
