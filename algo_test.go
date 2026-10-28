package kraken

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/UnipayFI/go-kraken/internal/apitest"
	"github.com/shopspring/decimal"
)

// TestAddStrategyValidate runs unconditionally: it validates a TWAP without
// creating it, exercising AddStrategy signing + response shape with zero risk.
func TestAddStrategyValidate(t *testing.T) {
	c := NewClient(apitest.AuthOptions(t)...)
	ctx := apitest.Ctx(t)

	res, err := c.NewAddStrategyService(StrategyTypeTWAP, tradePair, OrderSideBuy, OrderTypeLimit,
		decimal.RequireFromString("0.001"), "+600").
		SetPrice(decimal.RequireFromString(farBelowBid)).
		SetValidate(true).
		Do(ctx)
	if err != nil {
		t.Fatalf("AddStrategy(validate): %v", err)
	}
	if res.StrategyID != "" {
		t.Errorf("AddStrategy(validate): strategy_id=%q (want empty)", res.StrategyID)
	}
}

// TestAlgoStrategies exercises the read-only Algo Trading endpoints and verifies
// the typed structs cover the fields returned.
func TestAlgoStrategies(t *testing.T) {
	c := NewClient(apitest.AuthOptions(t)...)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	var ids []string

	// 1. Get Open Strategies.
	{
		raw := apitest.FetchRawPost(t, c, ctx, "/0/private/GetOpenStrategies", nil)
		resp, err := c.NewGetOpenStrategiesService().Do(ctx)
		if err != nil {
			t.Fatalf("GetOpenStrategies: %v", err)
		}
		apitest.AssertCovers(t, "GetOpenStrategies", raw, resp)
		for id, s := range resp.Open {
			if s.StrategyID != id || (s.Status != StrategyStatusPending && s.Status != StrategyStatusOpen) {
				t.Errorf("GetOpenStrategies: unexpected entry %s: %+v", id, s)
			}
			checkStrategyTimes(t, "GetOpenStrategies", s)
			ids = append(ids, id)
		}
		t.Logf("GetOpenStrategies: %d open", len(resp.Open))
		pace()
	}

	// 2. Get Closed Strategies (one per page, so a second page exercises the cursor).
	{
		raw := apitest.FetchRawPost(t, c, ctx, "/0/private/GetClosedStrategies", map[string]string{"limit": "1"})
		page, err := c.NewGetClosedStrategiesService().SetLimit(1).Do(ctx)
		if err != nil {
			t.Fatalf("GetClosedStrategies: %v", err)
		}
		apitest.AssertCovers(t, "GetClosedStrategies", raw, page)
		if len(page.Closed) > 1 {
			t.Errorf("GetClosedStrategies: limit=1 returned %d strategies", len(page.Closed))
		}
		for _, s := range page.Closed {
			checkStrategyTimes(t, "GetClosedStrategies", s)
			ids = append(ids, s.StrategyID)
		}
		t.Logf("GetClosedStrategies: %d on first page, next_cursor=%q", len(page.Closed), page.NextCursor)
		pace()
		if page.NextCursor != "" {
			next, err := c.NewGetClosedStrategiesService().SetLimit(1).SetCursor(page.NextCursor).Do(ctx)
			if err != nil {
				t.Fatalf("GetClosedStrategies(page 2): %v", err)
			}
			for _, s := range next.Closed {
				if len(page.Closed) > 0 && s.StrategyID == page.Closed[0].StrategyID {
					t.Errorf("GetClosedStrategies: page 2 repeats %s", s.StrategyID)
				}
			}
			pace()
		}
	}

	// 3. Query Strategies (the ids found above, or an unknown id, which yields an
	// empty map rather than an error).
	{
		if len(ids) == 0 {
			ids = []string{"ASAAAAA-AAAAA-AAAAAA"}
		}
		if len(ids) > 20 {
			ids = ids[:20]
		}
		params := map[string]string{"strategy_ids": strings.Join(ids, ",")}
		raw := apitest.FetchRawPost(t, c, ctx, "/0/private/QueryStrategies", params)
		resp, err := c.NewQueryStrategiesService(ids...).Do(ctx)
		if err != nil {
			t.Fatalf("QueryStrategies: %v", err)
		}
		apitest.AssertCovers(t, "QueryStrategies", raw, resp)
		for id, s := range resp.Strategies {
			if s.StrategyID != id {
				t.Errorf("QueryStrategies: key %s holds %s", id, s.StrategyID)
			}
		}
		t.Logf("QueryStrategies: %d of %d matched", len(resp.Strategies), len(ids))
	}
}

// TestStrategyLifecycle creates a real TWAP and cancels it before it starts.
// Gated behind KRAKEN_TEST_WRITE=1. The strategy is scheduled 10 minutes out
// and its child orders would be immediate-or-cancel limit buys far below
// market, so nothing can fill even if cancellation were late.
func TestStrategyLifecycle(t *testing.T) {
	if os.Getenv("KRAKEN_TEST_WRITE") != "1" {
		t.Skip("set KRAKEN_TEST_WRITE=1 to run live strategy tests (cancelled before start)")
	}
	c := NewClient(apitest.AuthOptions(t)...)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	// 1. Add Strategy (pending: starts in 10 minutes).
	add, err := c.NewAddStrategyService(StrategyTypeTWAP, tradePair, OrderSideBuy, OrderTypeLimit,
		decimal.RequireFromString("0.0001"), "+1200").
		SetPrice(decimal.RequireFromString(farBelowBid)).
		SetStartTime("+600").
		Do(ctx)
	if err != nil {
		t.Fatalf("AddStrategy: %v", err)
	}
	id := add.StrategyID
	if id == "" {
		t.Fatalf("AddStrategy returned no strategy_id: %+v", add)
	}
	cancelled := false
	defer func() {
		if !cancelled {
			if _, err := c.NewCancelStrategyService(id).Do(context.Background()); err != nil {
				t.Logf("CancelStrategy (cleanup) %s: %v", id, err)
			}
		}
	}()
	t.Logf("AddStrategy: strategy_id=%s", id)
	pace()

	// Verify it is listed as open (pending).
	{
		raw := apitest.FetchRawPost(t, c, ctx, "/0/private/GetOpenStrategies", map[string]string{"strategy_type": "twap"})
		resp, err := c.NewGetOpenStrategiesService().SetStrategyType(StrategyTypeTWAP).Do(ctx)
		if err != nil {
			t.Fatalf("GetOpenStrategies: %v", err)
		}
		apitest.AssertCovers(t, "GetOpenStrategies", raw, resp)
		s, ok := resp.Open[id]
		if !ok || s.Status != StrategyStatusPending {
			t.Errorf("strategy %s not pending: %+v", id, s)
		}
		checkStrategyTimes(t, "GetOpenStrategies", s)
		if d := time.Until(s.Details.StartTime); d < 8*time.Minute || d > 11*time.Minute {
			t.Errorf("GetOpenStrategies: starttm %v is %v from now (want ~10m); wrong format unit?", s.Details.StartTime, d)
		}
		pace()
	}

	// 2. Cancel Strategy.
	cs, err := c.NewCancelStrategyService(id).Do(ctx)
	if err != nil {
		t.Fatalf("CancelStrategy: %v", err)
	}
	cancelled = true
	if cs.StrategyID != id {
		t.Errorf("CancelStrategy: strategy_id=%q (want %s)", cs.StrategyID, id)
	}
	pace()

	// Poll QueryStrategies until the strategy is terminal.
	var final StrategyInfo
	for range 10 {
		raw := apitest.FetchRawPost(t, c, ctx, "/0/private/QueryStrategies", map[string]string{"strategy_ids": id})
		resp, err := c.NewQueryStrategiesService(id).Do(ctx)
		if err != nil {
			t.Fatalf("QueryStrategies: %v", err)
		}
		apitest.AssertCovers(t, "QueryStrategies", raw, resp)
		final = resp.Strategies[id]
		if final.Status != StrategyStatusPending && final.Status != StrategyStatusOpen {
			break
		}
		pace()
	}
	if final.Status != StrategyStatusCancelled || final.Details.FilledVolume.Sign() != 0 {
		t.Errorf("QueryStrategies: strategy %s did not end cancelled and unfilled: %+v", id, final)
	}
	t.Logf("QueryStrategies: %s status=%s reason=%s", id, final.Status, final.Reason)
	pace()

	// 3. Get Closed Strategies (filtered to cancelled TWAPs closed just now).
	params := map[string]string{
		"strategy_type":   "twap",
		"status":          "cancelled",
		"query_time_mode": "close",
		"start":           formatUnix(time.Now().Add(-10 * time.Minute)),
	}
	raw := apitest.FetchRawPost(t, c, ctx, "/0/private/GetClosedStrategies", params)
	closed, err := c.NewGetClosedStrategiesService().SetStrategyType(StrategyTypeTWAP).
		SetStatus(StrategyStatusCancelled).SetQueryTimeMode("close").
		SetStart(time.Now().Add(-10 * time.Minute)).Do(ctx)
	if err != nil {
		t.Fatalf("GetClosedStrategies: %v", err)
	}
	apitest.AssertCovers(t, "GetClosedStrategies", raw, closed)
	found := false
	for _, s := range closed.Closed {
		found = found || s.StrategyID == id
	}
	if !found {
		t.Errorf("GetClosedStrategies: cancelled strategy %s not listed", id)
	}
}

// checkStrategyTimes flags start/expire times that cannot be right, which is how
// a wrong format unit shows up (it never errors).
func checkStrategyTimes(t *testing.T, label string, s StrategyInfo) {
	t.Helper()
	start, expire := s.Details.StartTime, s.Details.ExpireTime
	if start.Year() < 2025 || expire.Before(start) || time.Until(expire) > 31*24*time.Hour {
		t.Errorf("%s: strategy %s has implausible starttm=%v expiretm=%v; wrong format unit?", label, s.StrategyID, start, expire)
	}
}
