package kraken

import (
	"testing"

	"github.com/UnipayFI/go-kraken/internal/apitest"
)

// TestTransparency exercises the public pre-/post-trade transparency endpoints.
func TestTransparency(t *testing.T) {
	c := NewClient(apitest.PublicOptions()...)
	ctx := apitest.Ctx(t)

	// 1. Pre-Trade Data.
	{
		params := map[string]string{"symbol": "BTC/USD"}
		raw := apitest.FetchRawGet(t, c, ctx, "/0/public/PreTrade", params)
		resp, err := c.NewGetPreTradeDataService("BTC/USD").Do(ctx)
		if err != nil {
			t.Fatalf("PreTrade: %v", err)
		}
		apitest.AssertCovers(t, "PreTrade", raw, resp)
		if resp.Symbol == "" || (len(resp.Asks) == 0 && len(resp.Bids) == 0) {
			t.Errorf("PreTrade empty: %+v", resp)
		}
		if len(resp.Asks) > 0 {
			a := resp.Asks[0]
			if a.Price.IsZero() || a.PublicationTime.IsZero() || a.Side == "" {
				t.Errorf("PreTrade level has zero fields: %+v", a)
			}
		}
	}

	// 2. Post-Trade Data.
	{
		params := map[string]string{"symbol": "BTC/USD", "count": "5"}
		raw := apitest.FetchRawGet(t, c, ctx, "/0/public/PostTrade", params)
		resp, err := c.NewGetPostTradeDataService().SetSymbol("BTC/USD").SetCount(5).Do(ctx)
		if err != nil {
			t.Fatalf("PostTrade: %v", err)
		}
		apitest.AssertCovers(t, "PostTrade", raw, resp)
		if len(resp.Trades) == 0 {
			t.Fatal("PostTrade returned no trades")
		}
		tr := resp.Trades[0]
		if tr.TradeID == "" || tr.Price.IsZero() || tr.Quantity.IsZero() || tr.TradeTime.IsZero() {
			t.Errorf("PostTrade trade has zero fields: %+v", tr)
		}
	}
}
