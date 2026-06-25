// Package apitest holds the shared, test-only helpers used by the Kraken SDK's
// _test.go files to verify that the typed endpoint structs cover every field the
// live Kraken API returns.
//
// It is imported exclusively from _test.go files; nothing in the shipped library
// depends on it. AssertCovers marshals a decoded response back to JSON and diffs
// its key set against the raw API response, so any field the struct forgot
// surfaces as an uncovered key path.
package apitest

import (
	"context"
	"errors"
	"maps"
	"os"
	"sort"
	"testing"
	"time"

	"github.com/UnipayFI/go-kraken/client"
	"github.com/UnipayFI/go-kraken/common"
	"github.com/UnipayFI/go-kraken/request"
)

// Creds reads the API credentials from the environment. Tests that need a
// private endpoint call this and skip themselves when credentials are absent,
// so the suite stays runnable without secrets.
func Creds(t *testing.T) (apiKey, apiSecret string) {
	t.Helper()
	apiKey = os.Getenv("KRAKEN_API_KEY")
	apiSecret = os.Getenv("KRAKEN_API_SECRET")
	if apiKey == "" || apiSecret == "" {
		t.Skip("KRAKEN_API_KEY/SECRET not set; skipping private test")
	}
	return apiKey, apiSecret
}

// AuthOptions builds the standard authenticated client options (auth + optional
// proxy) from the environment, skipping the test if credentials are absent.
func AuthOptions(t *testing.T) []client.Options {
	t.Helper()
	apiKey, apiSecret := Creds(t)
	opts := []client.Options{client.WithAuth(apiKey, apiSecret)}
	if proxy := os.Getenv("KRAKEN_PROXY"); proxy != "" {
		opts = append(opts, client.WithProxy(proxy))
	}
	return opts
}

// PublicOptions builds client options for public (unsigned) endpoints, honoring
// KRAKEN_PROXY when set.
func PublicOptions() []client.Options {
	opts := []client.Options{}
	if proxy := os.Getenv("KRAKEN_PROXY"); proxy != "" {
		opts = append(opts, client.WithProxy(proxy))
	}
	return opts
}

// WsPublicOptions builds WebSocket client options for public channels, honoring
// KRAKEN_PROXY when set.
func WsPublicOptions() []client.WebSocketOptions {
	opts := []client.WebSocketOptions{}
	if proxy := os.Getenv("KRAKEN_PROXY"); proxy != "" {
		opts = append(opts, client.WithWebSocketProxy(proxy))
	}
	return opts
}

// WsAuthOptions builds authenticated WebSocket client options (for private
// channels and order entry), skipping the test if credentials are absent.
func WsAuthOptions(t *testing.T) []client.WebSocketOptions {
	t.Helper()
	apiKey, apiSecret := Creds(t)
	opts := []client.WebSocketOptions{client.WithWebSocketAuth(apiKey, apiSecret)}
	if proxy := os.Getenv("KRAKEN_PROXY"); proxy != "" {
		opts = append(opts, client.WithWebSocketProxy(proxy))
	}
	return opts
}

// Ctx returns a request context with a 30s timeout, cleaned up via t.Cleanup.
func Ctx(t *testing.T) context.Context {
	t.Helper()
	c, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)
	return c
}

// FetchRawGet returns the raw JSON of the "result" field for a GET endpoint,
// used to diff the real response shape against the typed structs.
func FetchRawGet(t *testing.T, c request.Client, ctx context.Context, path string, params map[string]string) []byte {
	t.Helper()
	raw, err := request.DoRawResult(request.Get(ctx, c, path, params))
	if err != nil {
		t.Fatalf("raw GET %s: %v", path, err)
	}
	return raw
}

// FetchRawPost mirrors FetchRawGet for signed POST endpoints.
func FetchRawPost(t *testing.T, c request.Client, ctx context.Context, path string, params map[string]string) []byte {
	t.Helper()
	raw, err := request.DoRawResult(request.Post(ctx, c, path, params).WithSign())
	if err != nil {
		t.Fatalf("raw POST %s: %v", path, err)
	}
	return raw
}

// AssertCovers checks that every JSON key present in the real response (raw) is
// also produced by marshaling the typed value. It compares key *sets* (not
// values), recursing into nested objects and merging array/map elements, so a
// missing struct field surfaces as an uncovered key path.
func AssertCovers(t *testing.T, label string, raw []byte, typed any) {
	t.Helper()
	missing, err := coverGaps(raw, typed)
	if err != nil {
		t.Fatalf("%s: %v", label, err)
	}
	if len(missing) > 0 {
		t.Errorf("%s: %d field(s) in real response NOT captured by struct:\n  %v", label, len(missing), missing)
		return
	}
	t.Logf("%s: OK, all response keys covered by struct", label)
}

// Tolerable reports whether err is an expected "this account lacks the
// capability or simply has no data" Kraken response rather than a code bug, so
// capability-gated read tests can treat it as a pass: the request path and
// signing were correct, the account just isn't enrolled in this product.
// Callers pass error substrings (e.g. "Permission denied", "Unknown asset")
// they consider tolerable.
func Tolerable(t *testing.T, label string, err error, okSubstrings ...string) bool {
	t.Helper()
	var apiErr *client.APIError
	if errors.As(err, &apiErr) {
		for _, sub := range okSubstrings {
			if apiErr.Has(sub) {
				t.Logf("%s: account lacks this capability/data (%s) — endpoint+signing OK", label, apiErr.Error())
				return true
			}
		}
	}
	return false
}

// coverGaps unmarshals raw and the marshaled typed value, then returns the
// sorted key paths present in raw but missing from typed.
func coverGaps(raw []byte, typed any) ([]string, error) {
	var rawAny any
	if err := common.JSONUnmarshal(raw, &rawAny); err != nil {
		return nil, err
	}
	typedBytes, err := common.JSONMarshal(typed)
	if err != nil {
		return nil, err
	}
	var haveAny any
	if err := common.JSONUnmarshal(typedBytes, &haveAny); err != nil {
		return nil, err
	}
	var missing []string
	diffKeys(rawAny, haveAny, "", &missing)
	sort.Strings(missing)
	return missing, nil
}

// diffKeys walks raw and records the paths of keys absent from have. Objects
// recurse key-by-key; arrays merge keys across all elements so optional fields
// present only on some rows are still checked against the single-shape struct.
func diffKeys(raw, have any, path string, out *[]string) {
	switch r := raw.(type) {
	case map[string]any:
		h, ok := have.(map[string]any)
		if !ok {
			*out = append(*out, path+" (expected object)")
			return
		}
		for k, rv := range r {
			child := path + "/" + k
			hv, present := h[k]
			if !present {
				*out = append(*out, child)
				continue
			}
			diffKeys(rv, hv, child, out)
		}
	case []any:
		h, ok := have.([]any)
		if !ok || len(r) == 0 || len(h) == 0 {
			return
		}
		merged := map[string]any{}
		for _, e := range r {
			if em, ok := e.(map[string]any); ok {
				maps.Copy(merged, em)
			}
		}
		if len(merged) > 0 {
			diffKeys(merged, h[0], path+"[]", out)
		} else {
			diffKeys(r[0], h[0], path+"[]", out)
		}
	}
}
