package kraken

import (
	"context"
	"testing"
	"time"

	"github.com/UnipayFI/go-kraken/internal/apitest"
)

// TestAccountExtra exercises the later additions to the account/market surface
// that require authentication: the L3 order book, credit lines and API-key info.
func TestAccountExtra(t *testing.T) {
	c := NewClient(apitest.AuthOptions(t)...)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	// 1. Query L3 Order Book (authenticated market data).
	{
		params := map[string]string{"pair": "XBTUSD", "count": "10"}
		raw := apitest.FetchRawPost(t, c, ctx, "/0/private/Level3", params)
		resp, err := c.NewQueryL3OrderBookService("XBTUSD").SetCount(10).Do(ctx)
		if err != nil {
			t.Fatalf("Level3: %v", err)
		}
		apitest.AssertCovers(t, "Level3", raw, resp)
		if len(resp.Asks) == 0 || resp.Asks[0].OrderID == "" || resp.Asks[0].Timestamp.Time().IsZero() {
			t.Errorf("Level3 ask entry invalid: %+v", resp.Asks)
		}
		pace()
	}

	// 2. Get Credit Lines.
	{
		raw := apitest.FetchRawPost(t, c, ctx, "/0/private/CreditLines", nil)
		resp, err := c.NewGetCreditLinesService().Do(ctx)
		if err != nil {
			if apitest.Tolerable(t, "CreditLines", err, "Permission denied", "permission", "not allowed") {
				return
			}
			t.Fatalf("CreditLines: %v", err)
		}
		apitest.AssertCovers(t, "CreditLines", raw, resp)
		t.Logf("CreditLines: %d asset(s), equity_usd=%s", len(resp.AssetDetails), resp.LimitsMonitor.EquityUSD)
		pace()
	}

	// 3. Get API Key Info.
	{
		raw := apitest.FetchRawPost(t, c, ctx, "/0/private/GetApiKeyInfo", nil)
		resp, err := c.NewGetApiKeyInfoService().Do(ctx)
		if err != nil {
			t.Fatalf("GetApiKeyInfo: %v", err)
		}
		apitest.AssertCovers(t, "GetApiKeyInfo", raw, resp)
		if resp.APIKey == "" || len(resp.Permissions) == 0 {
			t.Errorf("GetApiKeyInfo missing key/permissions: %+v", resp)
		}
		t.Logf("GetApiKeyInfo: name=%q perms=%d created=%s", resp.APIKeyName, len(resp.Permissions), resp.CreatedTime.Format(time.RFC3339))
	}
}
