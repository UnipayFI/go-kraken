package kraken

import (
	"testing"

	"github.com/UnipayFI/go-kraken/internal/apitest"
)

// TestWebSocketsToken verifies the private-WebSocket token endpoint.
func TestWebSocketsToken(t *testing.T) {
	c := NewClient(apitest.AuthOptions(t)...)
	ctx := apitest.Ctx(t)

	raw := apitest.FetchRawPost(t, c, ctx, "/0/private/GetWebSocketsToken", nil)
	resp, err := c.NewGetWebSocketsTokenService().Do(ctx)
	if err != nil {
		t.Fatalf("GetWebSocketsToken: %v", err)
	}
	apitest.AssertCovers(t, "GetWebSocketsToken", raw, resp)
	if resp.Token == "" {
		t.Error("GetWebSocketsToken: empty token")
	}
	if resp.Expires <= 0 {
		t.Errorf("GetWebSocketsToken: non-positive expires %d", resp.Expires)
	}
	t.Logf("GetWebSocketsToken: token len=%d expires=%ds", len(resp.Token), resp.Expires)
}
