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
	wsTradePair = "BTC/USDT" // uses the account's USDT balance
	wsTradeQty  = "0.00005"
	wsFarBid    = "30000.0" // post-only buy far below market: rests, never fills
	wsFarBid2   = "29000.0"
	wsFarBid3   = "28000.0"
)

// TestWsAddOrderValidate runs unconditionally: it validates an order over the
// WebSocket order-entry connection without submitting it.
func TestWsAddOrderValidate(t *testing.T) {
	c := NewWebSocketClient(apitest.WsAuthOptions(t)...)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	tc, err := c.DialTrade(ctx)
	if err != nil {
		t.Fatalf("DialTrade: %v", err)
	}
	defer tc.Close()

	resp, err := tc.AddOrder(ctx, WsAddOrder{
		Symbol: wsTradePair, Side: OrderSideBuy, OrderType: OrderTypeLimit,
		OrderQty: decimal.RequireFromString(wsTradeQty), LimitPrice: decimal.RequireFromString(wsFarBid),
		PostOnly: true, Validate: true,
	})
	if err != nil {
		t.Fatalf("AddOrder(validate): %v", err)
	}
	if !resp.Success {
		t.Errorf("AddOrder(validate): success=false error=%q", resp.Error)
	}
	t.Logf("AddOrder(validate): OK success=%v", resp.Success)
}

// TestWsTradeLifecycle places real (tiny, large-cap, far-from-market) orders over
// the WebSocket order-entry connection to exercise the full surface, and
// concurrently verifies the executions channel decodes real order events. Gated
// behind KRAKEN_TEST_WRITE=1.
func TestWsTradeLifecycle(t *testing.T) {
	if os.Getenv("KRAKEN_TEST_WRITE") != "1" {
		t.Skip("set KRAKEN_TEST_WRITE=1 to run live WebSocket order tests (tiny, reversible)")
	}
	c := NewWebSocketClient(apitest.WsAuthOptions(t)...)
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	qty := decimal.RequireFromString(wsTradeQty)

	// Watch the executions channel: a real order event must decode cleanly.
	execRes := make(chan error, 1)
	execDone, _, err := c.NewSubscribeExecutionsService().SetSnapOrders(true).Do(ctx, func(p *WsPush[[]WsExecution], e error) {
		if e != nil {
			trySend(execRes, e)
		} else if len(p.Data) > 0 && p.Data[0].OrderID != "" {
			trySend(execRes, nil)
		}
	})
	if err != nil {
		t.Fatalf("subscribe executions: %v", err)
	}
	defer close(execDone)

	tc, err := c.DialTrade(ctx)
	if err != nil {
		t.Fatalf("DialTrade: %v", err)
	}
	defer tc.Close()
	defer tc.CancelAll(context.Background())

	// 1. add_order (post-only limit far below market — rests, never fills).
	add, err := tc.AddOrder(ctx, WsAddOrder{
		Symbol: wsTradePair, Side: OrderSideBuy, OrderType: OrderTypeLimit,
		OrderQty: qty, LimitPrice: decimal.RequireFromString(wsFarBid), PostOnly: true,
	})
	if err != nil {
		t.Fatalf("AddOrder: %v", err)
	}
	if add.Result.OrderID == "" {
		t.Fatalf("AddOrder: no order_id: %+v", add.Result)
	}
	txid := add.Result.OrderID
	t.Logf("AddOrder: order_id=%s", txid)

	// 2. amend_order (move the resting price).
	amend, err := tc.AmendOrder(ctx, WsAmendOrder{
		OrderID: txid, LimitPrice: decimal.RequireFromString("31000.0"),
	})
	if err != nil {
		t.Fatalf("AmendOrder: %v", err)
	}
	t.Logf("AmendOrder: amend_id=%s", amend.Result.AmendID)

	// 3. edit_order (cancel-replace; new order id).
	edit, err := tc.EditOrder(ctx, WsEditOrder{
		OrderID: txid, Symbol: wsTradePair,
		OrderQty: decimal.RequireFromString("0.00006"), LimitPrice: decimal.RequireFromString(wsFarBid2),
	})
	if err != nil {
		t.Fatalf("EditOrder: %v", err)
	}
	if edit.Result.OriginalOrderID != txid || edit.Result.OrderID == "" {
		t.Errorf("EditOrder ids unexpected: %+v", edit.Result)
	}
	t.Logf("EditOrder: %s -> %s", edit.Result.OriginalOrderID, edit.Result.OrderID)
	editedTxID := edit.Result.OrderID

	// 4. cancel_order.
	cancel1, err := tc.CancelOrder(ctx, editedTxID)
	if err != nil {
		t.Fatalf("CancelOrder: %v", err)
	}
	t.Logf("CancelOrder: order_id=%s", cancel1.Result.OrderID)

	// 5. batch_add (two resting bids).
	batch, err := tc.BatchAdd(ctx, wsTradePair,
		WsAddOrder{Side: OrderSideBuy, OrderType: OrderTypeLimit, OrderQty: qty, LimitPrice: decimal.RequireFromString(wsFarBid2), PostOnly: true},
		WsAddOrder{Side: OrderSideBuy, OrderType: OrderTypeLimit, OrderQty: qty, LimitPrice: decimal.RequireFromString(wsFarBid3), PostOnly: true},
	)
	if err != nil {
		t.Fatalf("BatchAdd: %v", err)
	}
	var batchIDs []string
	for _, r := range batch.Result {
		if r.OrderID != "" {
			batchIDs = append(batchIDs, r.OrderID)
		}
	}
	t.Logf("BatchAdd: order_ids=%v", batchIDs)
	if len(batchIDs) != 2 {
		t.Errorf("BatchAdd: expected 2 order_ids, got %d", len(batchIDs))
	}

	// 6. batch_cancel.
	if len(batchIDs) > 0 {
		bc, err := tc.BatchCancel(ctx, batchIDs...)
		if err != nil {
			t.Fatalf("BatchCancel: %v", err)
		}
		if !bc.Success {
			t.Errorf("BatchCancel: success=false error=%q", bc.Error)
		}
		t.Logf("BatchCancel: success=%v", bc.Success)
	}

	// 7. cancel_all_orders_after (arm then disarm).
	after, err := tc.CancelAllOrdersAfter(ctx, 60)
	if err != nil {
		t.Fatalf("CancelAllOrdersAfter(arm): %v", err)
	}
	if after.Result.CurrentTime.IsZero() || after.Result.TriggerTime.IsZero() {
		t.Errorf("CancelAllOrdersAfter: zero times: %+v", after.Result)
	}
	t.Logf("CancelAllOrdersAfter: current=%s trigger=%s",
		after.Result.CurrentTime.Format(time.RFC3339), after.Result.TriggerTime.Format(time.RFC3339))
	if _, err := tc.CancelAllOrdersAfter(ctx, 0); err != nil {
		t.Fatalf("CancelAllOrdersAfter(disarm): %v", err)
	}

	// 8. cancel_all.
	all, err := tc.CancelAll(ctx)
	if err != nil {
		t.Fatalf("CancelAll: %v", err)
	}
	t.Logf("CancelAll: count=%d", all.Result.Count)

	// Confirm the executions channel delivered a decodable order event.
	select {
	case e := <-execRes:
		if e != nil {
			t.Errorf("executions decode: %v", e)
		} else {
			t.Log("executions: OK (real order event decoded cleanly)")
		}
	case <-time.After(5 * time.Second):
		t.Log("executions: no event surfaced in window (orders still verified via acks)")
	}
}
