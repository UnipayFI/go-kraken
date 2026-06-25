package kraken

import (
	"testing"

	"github.com/UnipayFI/go-kraken/internal/apitest"
)

// TestMarketData exercises every public market-data endpoint against the live
// Kraken API and verifies the typed structs cover every field the API returns.
func TestMarketData(t *testing.T) {
	c := NewClient(apitest.PublicOptions()...)
	ctx := apitest.Ctx(t)

	// 1. Get Server Time.
	{
		raw := apitest.FetchRawGet(t, c, ctx, "/0/public/Time", nil)
		resp, err := c.NewGetServerTimeService().Do(ctx)
		if err != nil {
			t.Fatalf("ServerTime: %v", err)
		}
		apitest.AssertCovers(t, "ServerTime", raw, resp)
		if resp.UnixTime.IsZero() {
			t.Error("ServerTime.UnixTime is zero")
		}
	}

	// 2. Get System Status.
	{
		raw := apitest.FetchRawGet(t, c, ctx, "/0/public/SystemStatus", nil)
		resp, err := c.NewGetSystemStatusService().Do(ctx)
		if err != nil {
			t.Fatalf("SystemStatus: %v", err)
		}
		apitest.AssertCovers(t, "SystemStatus", raw, resp)
		if resp.Status == "" {
			t.Error("SystemStatus.Status is empty")
		}
	}

	// 3. Get Asset Info.
	{
		params := map[string]string{"asset": "XBT,ETH,USDT"}
		raw := apitest.FetchRawGet(t, c, ctx, "/0/public/Assets", params)
		resp, err := c.NewGetAssetInfoService().SetAsset("XBT", "ETH", "USDT").Do(ctx)
		if err != nil {
			t.Fatalf("Assets: %v", err)
		}
		apitest.AssertCovers(t, "Assets", raw, resp)
		if len(resp) == 0 {
			t.Error("Assets returned no assets")
		}
	}

	// 4. Get Tradable Asset Pairs (all pairs, to surface optional fields).
	{
		raw := apitest.FetchRawGet(t, c, ctx, "/0/public/AssetPairs", nil)
		resp, err := c.NewGetTradableAssetPairsService().Do(ctx)
		if err != nil {
			t.Fatalf("AssetPairs: %v", err)
		}
		apitest.AssertCovers(t, "AssetPairs", raw, resp)
		if p, ok := resp["XXBTZUSD"]; !ok || p.AltName != "XBTUSD" {
			t.Errorf("AssetPairs missing/incorrect XXBTZUSD: %+v", p)
		}
	}

	// 5. Get Ticker Information.
	{
		params := map[string]string{"pair": "XBTUSD,ETHUSD"}
		raw := apitest.FetchRawGet(t, c, ctx, "/0/public/Ticker", params)
		resp, err := c.NewGetTickerService().SetPair("XBTUSD", "ETHUSD").Do(ctx)
		if err != nil {
			t.Fatalf("Ticker: %v", err)
		}
		apitest.AssertCovers(t, "Ticker", raw, resp)
		if tk, ok := resp["XXBTZUSD"]; !ok || tk.Ask.Price.IsZero() {
			t.Errorf("Ticker missing/zero XXBTZUSD ask: %+v", tk)
		}
	}

	// 6. Get OHLC Data (tuple endpoint: verify columns decode + sanity).
	{
		resp, err := c.NewGetOHLCDataService("XBTUSD").SetInterval(Interval1h).Do(ctx)
		if err != nil {
			t.Fatalf("OHLC: %v", err)
		}
		if resp.Pair != "XXBTZUSD" {
			t.Errorf("OHLC pair = %q, want XXBTZUSD", resp.Pair)
		}
		if len(resp.Candles) == 0 {
			t.Fatal("OHLC returned no candles")
		}
		k := resp.Candles[0]
		if k.Time.IsZero() || k.Open.IsZero() || k.Close.IsZero() {
			t.Errorf("OHLC candle has zero fields: %+v", k)
		}
		if resp.Last.IsZero() {
			t.Error("OHLC.Last cursor is zero")
		}
	}

	// 7. Get Order Book.
	{
		params := map[string]string{"pair": "XBTUSD", "count": "10"}
		raw := apitest.FetchRawGet(t, c, ctx, "/0/public/Depth", params)
		resp, err := c.NewGetOrderBookService("XBTUSD").SetCount(10).Do(ctx)
		if err != nil {
			t.Fatalf("Depth: %v", err)
		}
		// Rebuild the pair-keyed shape Kraken returns for coverage.
		typed := map[string]OrderBook{resp.Pair: {Asks: resp.Asks, Bids: resp.Bids}}
		apitest.AssertCovers(t, "Depth", raw, typed)
		if len(resp.Asks) == 0 || len(resp.Bids) == 0 {
			t.Fatal("Depth returned empty asks/bids")
		}
		if resp.Asks[0].Price.IsZero() || resp.Asks[0].Timestamp.IsZero() {
			t.Errorf("Depth ask entry has zero fields: %+v", resp.Asks[0])
		}
	}

	// 8. Get Recent Trades (tuple endpoint).
	{
		resp, err := c.NewGetRecentTradesService("XBTUSD").SetCount(10).Do(ctx)
		if err != nil {
			t.Fatalf("Trades: %v", err)
		}
		if resp.Pair != "XXBTZUSD" || len(resp.Trades) == 0 {
			t.Fatalf("Trades pair=%q count=%d", resp.Pair, len(resp.Trades))
		}
		tr := resp.Trades[0]
		if tr.Price.IsZero() || tr.Time.IsZero() || (tr.Side != "b" && tr.Side != "s") {
			t.Errorf("Trade has bad fields: %+v", tr)
		}
		if resp.Last == "" {
			t.Error("Trades.Last cursor is empty")
		}
	}

	// 9. Get Recent Spreads (tuple endpoint).
	{
		resp, err := c.NewGetRecentSpreadsService("XBTUSD").Do(ctx)
		if err != nil {
			t.Fatalf("Spread: %v", err)
		}
		if resp.Pair != "XXBTZUSD" || len(resp.Spreads) == 0 {
			t.Fatalf("Spread pair=%q count=%d", resp.Pair, len(resp.Spreads))
		}
		sp := resp.Spreads[0]
		if sp.Time.IsZero() || sp.Bid.IsZero() || sp.Ask.IsZero() {
			t.Errorf("Spread has zero fields: %+v", sp)
		}
	}

	// 10. Get Grouped Order Book.
	{
		params := map[string]string{"pair": "XBTUSD", "depth": "10", "grouping": "10"}
		raw := apitest.FetchRawGet(t, c, ctx, "/0/public/GroupedBook", params)
		resp, err := c.NewGetGroupedOrderBookService("XBTUSD").SetDepth(10).SetGrouping(10).Do(ctx)
		if err != nil {
			t.Fatalf("GroupedBook: %v", err)
		}
		apitest.AssertCovers(t, "GroupedBook", raw, resp)
		if len(resp.Asks) == 0 || resp.Asks[0].Price.IsZero() {
			t.Errorf("GroupedBook empty/zero asks: %+v", resp)
		}
	}
}
