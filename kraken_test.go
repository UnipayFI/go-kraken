package kraken

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/UnipayFI/go-kraken/client"
	"github.com/go-resty/resty/v2"
)

// TestWithHTTPClient checks that responses fetched through a caller-supplied
// resty client are decoded with the SDK's JSON codec (format-tagged times),
// and that the caller's client keeps its own JSON settings.
func TestWithHTTPClient(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"error":[],"result":{"unixtime":1790567886,"rfc1123":"Mon, 28 Sep 26 03:58:06 +0000"}}`)
	}))
	defer srv.Close()

	rc := resty.New()
	unmarshaler := reflect.ValueOf(rc.JSONUnmarshal).Pointer()
	c := NewClient(client.WithHTTPClient(rc), client.WithBaseURL(srv.URL))
	if c.GetHttpClient() != rc {
		t.Fatal("WithHTTPClient: the supplied resty client is not used")
	}
	resp, err := c.NewGetServerTimeService().Do(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got := resp.UnixTime.Unix(); got != 1790567886 {
		t.Errorf("UnixTime = %d, want 1790567886", got)
	}
	if reflect.ValueOf(rc.JSONUnmarshal).Pointer() != unmarshaler {
		t.Error("WithHTTPClient: the caller's JSON unmarshaler was replaced")
	}
}
