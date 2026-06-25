package kraken

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/UnipayFI/go-kraken/internal/apitest"
	"github.com/shopspring/decimal"
)

const (
	tradePair    = "XBTUSDT" // BTC/USDT — uses the account's USDT balance
	tradeVolume  = "0.00005" // ordermin for XBTUSDT (~$3)
	farBelowBid  = "30000.0" // post-only buy far below market: rests, never fills
	farBelowBid2 = "29000.0" // second resting bid for batch tests
	farBelowBid3 = "28000.0" // third resting bid for batch tests
)

// TestAddOrderValidate runs unconditionally: it validates an order without
// submitting it, exercising AddOrder signing + response shape with zero risk.
func TestAddOrderValidate(t *testing.T) {
	c := NewClient(apitest.AuthOptions(t)...)
	ctx := apitest.Ctx(t)

	res, err := c.NewAddOrderService(tradePair, OrderSideBuy, OrderTypeLimit, decimal.RequireFromString(tradeVolume)).
		SetPrice(decimal.RequireFromString(farBelowBid)).
		SetOrderFlags(OrderFlagPost).
		SetValidate(true).
		Do(ctx)
	if err != nil {
		t.Fatalf("AddOrder(validate): %v", err)
	}
	if res.Description.Order == "" {
		t.Errorf("AddOrder(validate): empty order description: %+v", res)
	}
	t.Logf("AddOrder(validate): %q", res.Description.Order)
}

// TestTradeLifecycle places real (tiny, large-cap, far-from-market) orders to
// exercise the full trading surface against the live API. Gated behind
// KRAKEN_TEST_WRITE=1 so it never runs by accident. The resting orders sit far
// below market and never fill; one tiny market round-trip verifies fill-derived
// fields (trades, amends) and is immediately unwound.
func TestTradeLifecycle(t *testing.T) {
	if os.Getenv("KRAKEN_TEST_WRITE") != "1" {
		t.Skip("set KRAKEN_TEST_WRITE=1 to run live order tests (tiny, reversible)")
	}
	c := NewClient(apitest.AuthOptions(t)...)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	vol := decimal.RequireFromString(tradeVolume)

	// Clean slate, and ensure we don't leave resting orders behind.
	if _, err := c.NewCancelAllOrdersService().Do(ctx); err != nil {
		t.Fatalf("CancelAll (pre-clean): %v", err)
	}
	defer func() {
		if _, err := c.NewCancelAllOrdersService().Do(context.Background()); err != nil {
			t.Logf("CancelAll (cleanup): %v", err)
		}
	}()

	// 1. Add Order (real, post-only limit far below market — rests, never fills).
	// A mutation, so it is placed exactly once via the service; the response
	// shape is small (descr + txid) and asserted directly. Rich nested order
	// fields are covered separately via QueryOrders below.
	add, err := c.NewAddOrderService(tradePair, OrderSideBuy, OrderTypeLimit, vol).
		SetPrice(decimal.RequireFromString(farBelowBid)).SetOrderFlags(OrderFlagPost).Do(ctx)
	if err != nil {
		t.Fatalf("AddOrder: %v", err)
	}
	if len(add.TxID) == 0 {
		t.Fatalf("AddOrder returned no txid: %+v", add)
	}
	if add.Description.Order == "" {
		t.Errorf("AddOrder: empty order description")
	}
	txid := add.TxID[0]
	t.Logf("AddOrder: txid=%s descr=%q", txid, add.Description.Order)
	pace()

	// Verify it is open.
	if orders, err := c.NewQueryOrdersService(txid).Do(ctx); err != nil {
		t.Fatalf("QueryOrders(open): %v", err)
	} else if o, ok := orders[txid]; !ok || o.Status != "open" {
		t.Errorf("order %s not open: %+v", txid, o)
	}
	pace()

	// 2. Amend Order (move the resting price), then verify via OrderAmends.
	amend, err := c.NewAmendOrderService(txid).
		SetLimitPrice(decimal.RequireFromString("31000.0")).Do(ctx)
	if err != nil {
		t.Fatalf("AmendOrder: %v", err)
	}
	if amend.AmendID == "" {
		t.Errorf("AmendOrder: empty amend_id")
	}
	t.Logf("AmendOrder: amend_id=%s", amend.AmendID)
	pace()

	amendsRaw := signedRaw(t, c, ctx, "/0/private/OrderAmends", map[string]string{"order_id": txid})
	amends, err := c.NewGetOrderAmendsService(txid).Do(ctx)
	if err != nil {
		t.Fatalf("OrderAmends: %v", err)
	}
	apitest.AssertCovers(t, "OrderAmends", amendsRaw, amends)
	if amends.Count == 0 || len(amends.Amends) == 0 {
		t.Errorf("OrderAmends: expected amend records, got %+v", amends)
	} else {
		t.Logf("OrderAmends: count=%d first=%+v", amends.Count, amends.Amends[0])
	}
	pace()

	// 3. Edit Order (cancel-replace; yields a new txid).
	edit, err := c.NewEditOrderService(tradePair, txid).
		SetVolume(decimal.RequireFromString("0.00006")).
		SetPrice(decimal.RequireFromString(farBelowBid2)).Do(ctx)
	if err != nil {
		t.Fatalf("EditOrder: %v", err)
	}
	if edit.Status != "ok" && edit.Status != "Ok" {
		t.Errorf("EditOrder status=%q (want ok): %+v", edit.Status, edit)
	}
	if edit.OriginalTxID != txid || edit.TxID == "" {
		t.Errorf("EditOrder ids unexpected: orig=%s new=%s (want orig=%s)", edit.OriginalTxID, edit.TxID, txid)
	}
	t.Logf("EditOrder: %s -> %s status=%s", edit.OriginalTxID, edit.TxID, edit.Status)
	editedTxID := edit.TxID
	pace()

	// 4. Cancel Order (the edited order).
	cancel1, err := c.NewCancelOrderService(editedTxID).Do(ctx)
	if err != nil {
		t.Fatalf("CancelOrder: %v", err)
	}
	if cancel1.Count != 1 {
		t.Errorf("CancelOrder count=%d (want 1)", cancel1.Count)
	}
	t.Logf("CancelOrder: count=%d pending=%v", cancel1.Count, cancel1.Pending)
	pace()

	// 5. Add Order Batch (two resting bids).
	batch, err := c.NewAddOrderBatchService(tradePair).
		AddOrder(BatchOrder{OrderType: OrderTypeLimit, Side: OrderSideBuy, Volume: vol,
			Price: decimal.RequireFromString(farBelowBid2), OrderFlags: OrderFlagPost}).
		AddOrder(BatchOrder{OrderType: OrderTypeLimit, Side: OrderSideBuy, Volume: vol,
			Price: decimal.RequireFromString(farBelowBid3), OrderFlags: OrderFlagPost}).
		Do(ctx)
	if err != nil {
		t.Fatalf("AddOrderBatch: %v", err)
	}
	var batchTxIDs []string
	for _, o := range batch.Orders {
		if o.Error != "" {
			t.Errorf("AddOrderBatch order error: %s", o.Error)
			continue
		}
		if o.TxID != "" {
			batchTxIDs = append(batchTxIDs, o.TxID)
		}
	}
	t.Logf("AddOrderBatch: txids=%v", batchTxIDs)
	if len(batchTxIDs) != 2 {
		t.Errorf("AddOrderBatch: expected 2 txids, got %d", len(batchTxIDs))
	}
	pace()

	// 6. Cancel Order Batch.
	if len(batchTxIDs) > 0 {
		cb, err := c.NewCancelOrderBatchService(batchTxIDs...).Do(ctx)
		if err != nil {
			t.Fatalf("CancelOrderBatch: %v", err)
		}
		if cb.Count != len(batchTxIDs) {
			t.Errorf("CancelOrderBatch count=%d (want %d)", cb.Count, len(batchTxIDs))
		}
		t.Logf("CancelOrderBatch: count=%d", cb.Count)
		pace()
	}

	// 7. Cancel All Orders After (arm then disarm the dead-man's switch).
	after, err := c.NewCancelAllOrdersAfterService(60).Do(ctx)
	if err != nil {
		t.Fatalf("CancelAllOrdersAfter(arm): %v", err)
	}
	if after.CurrentTime.IsZero() || after.TriggerTime.IsZero() {
		t.Errorf("CancelAllOrdersAfter: zero times: %+v", after)
	}
	t.Logf("CancelAllOrdersAfter: current=%s trigger=%s", after.CurrentTime.Format(time.RFC3339), after.TriggerTime.Format(time.RFC3339))
	if _, err := c.NewCancelAllOrdersAfterService(0).Do(ctx); err != nil {
		t.Fatalf("CancelAllOrdersAfter(disarm): %v", err)
	}
	pace()

	// 8. Cancel All (cancel anything still resting).
	all, err := c.NewCancelAllOrdersService().Do(ctx)
	if err != nil {
		t.Fatalf("CancelAll: %v", err)
	}
	t.Logf("CancelAll: count=%d", all.Count)
	pace()

	// 9. Real fill round-trip to verify trade-derived fields.
	verifyTradeFields(t, c, ctx, vol)
}

// verifyTradeFields executes one tiny market buy (which fills), checks the
// fill-derived endpoints (TradesHistory, QueryTrades, filled order), then sells
// the position back to return to the base balance.
func verifyTradeFields(t *testing.T, c *Client, ctx context.Context, vol decimal.Decimal) {
	buy, err := c.NewAddOrderService(tradePair, OrderSideBuy, OrderTypeMarket, vol).
		SetOrderFlags(OrderFlagFCIQ). // pay fee in USDT so we receive the full BTC volume
		Do(ctx)
	if err != nil {
		t.Fatalf("market buy: %v", err)
	}
	if len(buy.TxID) == 0 {
		t.Fatalf("market buy returned no txid")
	}
	buyTxID := buy.TxID[0]
	t.Logf("market buy: txid=%s", buyTxID)
	pace()

	// Wait for the order to close (fill).
	var filled OrderInfo
	for attempt := 0; attempt < 6; attempt++ {
		orders, err := c.NewQueryOrdersService(buyTxID).SetTrades(true).Do(ctx)
		if err != nil {
			t.Fatalf("QueryOrders(fill): %v", err)
		}
		filled = orders[buyTxID]
		if filled.Status == "closed" {
			break
		}
		time.Sleep(1 * time.Second)
	}
	if filled.Status != "closed" {
		t.Fatalf("market buy not filled, status=%s", filled.Status)
	}
	t.Logf("filled: vol_exec=%s cost=%s fee=%s trades=%v", filled.VolumeExecuted, filled.Cost, filled.Fee, filled.Trades)

	// TradesHistory now has at least one trade — verify entry fields.
	thRaw := signedRaw(t, c, ctx, "/0/private/TradesHistory", nil)
	th, err := c.NewGetTradesHistoryService().Do(ctx)
	if err != nil {
		t.Fatalf("TradesHistory: %v", err)
	}
	apitest.AssertCovers(t, "TradesHistory(filled)", thRaw, th)
	if th.Count == 0 || len(th.Trades) == 0 {
		t.Errorf("TradesHistory: expected trades after fill")
	}
	pace()

	// QueryTrades for the executed trade id.
	var tradeID string
	for id := range th.Trades {
		tradeID = id
		break
	}
	if tradeID != "" {
		qtRaw := signedRaw(t, c, ctx, "/0/private/QueryTrades", map[string]string{"txid": tradeID})
		qt, err := c.NewQueryTradesService(tradeID).Do(ctx)
		if err != nil {
			t.Fatalf("QueryTrades: %v", err)
		}
		apitest.AssertCovers(t, "QueryTrades", qtRaw, qt)
		if tr, ok := qt[tradeID]; ok {
			t.Logf("QueryTrades: %s %s vol=%s price=%s fee=%s maker=%v", tradeID, tr.Type, tr.Volume, tr.Price, tr.Fee, tr.Maker)
		}
		pace()
	}

	// Unwind: sell the acquired base back to USDT (best-effort).
	bal, err := c.NewGetAccountBalanceService().Do(ctx)
	if err != nil {
		t.Logf("balance (pre-unwind): %v", err)
		return
	}
	btc := bal["XXBT"]
	if btc.GreaterThanOrEqual(vol) {
		sellVol := btc.Truncate(8)
		sell, err := c.NewAddOrderService(tradePair, OrderSideSell, OrderTypeMarket, sellVol).
			SetOrderFlags(OrderFlagFCIQ).Do(ctx)
		if err != nil {
			t.Logf("market sell (unwind): %v — manual cleanup may be needed for %s XXBT", err, sellVol)
			return
		}
		t.Logf("market sell (unwind): %s XXBT txid=%v", sellVol, sell.TxID)
	} else {
		t.Logf("unwind skipped: XXBT balance %s < %s", btc, vol)
	}
}

// signedRaw fetches the raw "result" JSON of a signed POST endpoint for coverage
// checks (a thin wrapper over apitest.FetchRawPost kept local for readability).
func signedRaw(t *testing.T, c *Client, ctx context.Context, path string, params map[string]string) []byte {
	t.Helper()
	return apitest.FetchRawPost(t, c, ctx, path, params)
}
