package kraken

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/UnipayFI/go-kraken/internal/apitest"
)

// pace throttles between private calls to stay under Kraken's per-key rate
// counter when a test fires many signed requests in a row.
func pace() { time.Sleep(1 * time.Second) }

// TestAccountData exercises every private account-data endpoint against the live
// Kraken API and verifies the typed structs cover the fields returned.
func TestAccountData(t *testing.T) {
	c := NewClient(apitest.AuthOptions(t)...)
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()

	// 1. Get Account Balance.
	{
		raw := apitest.FetchRawPost(t, c, ctx, "/0/private/Balance", nil)
		resp, err := c.NewGetAccountBalanceService().Do(ctx)
		if err != nil {
			t.Fatalf("Balance: %v", err)
		}
		apitest.AssertCovers(t, "Balance", raw, resp)
		t.Logf("Balance: %d asset(s)", len(resp))
		pace()
	}

	// 2. Get Extended Balance.
	{
		raw := apitest.FetchRawPost(t, c, ctx, "/0/private/BalanceEx", nil)
		resp, err := c.NewGetExtendedBalanceService().Do(ctx)
		if err != nil {
			t.Fatalf("BalanceEx: %v", err)
		}
		apitest.AssertCovers(t, "BalanceEx", raw, resp)
		pace()
	}

	// 3. Get Trade Balance.
	{
		raw := apitest.FetchRawPost(t, c, ctx, "/0/private/TradeBalance", nil)
		resp, err := c.NewGetTradeBalanceService().Do(ctx)
		if err != nil {
			t.Fatalf("TradeBalance: %v", err)
		}
		apitest.AssertCovers(t, "TradeBalance", raw, resp)
		pace()
	}

	// 4. Get Open Orders.
	{
		raw := apitest.FetchRawPost(t, c, ctx, "/0/private/OpenOrders", nil)
		resp, err := c.NewGetOpenOrdersService().Do(ctx)
		if err != nil {
			t.Fatalf("OpenOrders: %v", err)
		}
		apitest.AssertCovers(t, "OpenOrders", raw, resp)
		t.Logf("OpenOrders: %d open", len(resp.Open))
		pace()
	}

	// 5. Get Closed Orders (also yields a txid for QueryOrders / OrderAmends).
	var closedTxID string
	{
		raw := apitest.FetchRawPost(t, c, ctx, "/0/private/ClosedOrders", nil)
		resp, err := c.NewGetClosedOrdersService().Do(ctx)
		if err != nil {
			t.Fatalf("ClosedOrders: %v", err)
		}
		apitest.AssertCovers(t, "ClosedOrders", raw, resp)
		for txid := range resp.Closed {
			closedTxID = txid
			break
		}
		t.Logf("ClosedOrders: count=%d sample=%s", resp.Count, closedTxID)
		pace()
	}

	// 6. Query Orders Info.
	if closedTxID != "" {
		params := map[string]string{"txid": closedTxID}
		raw := apitest.FetchRawPost(t, c, ctx, "/0/private/QueryOrders", params)
		resp, err := c.NewQueryOrdersService(closedTxID).Do(ctx)
		if err != nil {
			t.Fatalf("QueryOrders: %v", err)
		}
		apitest.AssertCovers(t, "QueryOrders", raw, resp)
		if _, ok := resp[closedTxID]; !ok {
			t.Errorf("QueryOrders missing %s", closedTxID)
		}
		pace()
	} else {
		t.Log("QueryOrders: no closed order to query; skipped")
	}

	// 7. Get Order Amends (top-level shape; field-level checked in trade test).
	if closedTxID != "" {
		raw := apitest.FetchRawPost(t, c, ctx, "/0/private/OrderAmends", map[string]string{"order_id": closedTxID})
		resp, err := c.NewGetOrderAmendsService(closedTxID).Do(ctx)
		if err != nil {
			t.Fatalf("OrderAmends: %v", err)
		}
		apitest.AssertCovers(t, "OrderAmends", raw, resp)
		t.Logf("OrderAmends: count=%d", resp.Count)
		pace()
	}

	// 8. Get Trades History.
	{
		raw := apitest.FetchRawPost(t, c, ctx, "/0/private/TradesHistory", nil)
		resp, err := c.NewGetTradesHistoryService().Do(ctx)
		if err != nil {
			t.Fatalf("TradesHistory: %v", err)
		}
		apitest.AssertCovers(t, "TradesHistory", raw, resp)
		t.Logf("TradesHistory: count=%d", resp.Count)
		pace()
	}

	// 9. Query Trades Info (path+signing now; deep field check in trade test).
	{
		trades, err := c.NewQueryTradesService("AAAAAA-BBBBB-CCCCCC").Do(ctx)
		if err != nil {
			if !apitest.Tolerable(t, "QueryTrades", err, "Invalid arguments", "Invalid order", "Unknown") {
				t.Fatalf("QueryTrades: %v", err)
			}
		} else {
			t.Logf("QueryTrades: OK (%d trades); deep field check in trade lifecycle test", len(trades))
		}
		pace()
	}

	// 10. Get Open Positions.
	{
		raw := apitest.FetchRawPost(t, c, ctx, "/0/private/OpenPositions", nil)
		resp, err := c.NewGetOpenPositionsService().Do(ctx)
		if err != nil {
			t.Fatalf("OpenPositions: %v", err)
		}
		// Empty map for a spot-only account; coverage is trivial but the call
		// and signing are exercised.
		apitest.AssertCovers(t, "OpenPositions", raw, resp)
		t.Logf("OpenPositions: %d", len(resp))
		pace()
	}

	// 11. Get Ledgers Info (also yields a ledger id for QueryLedgers).
	var ledgerID string
	{
		raw := apitest.FetchRawPost(t, c, ctx, "/0/private/Ledgers", nil)
		resp, err := c.NewGetLedgersService().Do(ctx)
		if err != nil {
			t.Fatalf("Ledgers: %v", err)
		}
		apitest.AssertCovers(t, "Ledgers", raw, resp)
		for id := range resp.Ledger {
			ledgerID = id
			break
		}
		t.Logf("Ledgers: count=%d sample=%s", resp.Count, ledgerID)
		pace()
	}

	// 12. Query Ledgers.
	if ledgerID != "" {
		params := map[string]string{"id": ledgerID}
		raw := apitest.FetchRawPost(t, c, ctx, "/0/private/QueryLedgers", params)
		resp, err := c.NewQueryLedgersService(ledgerID).Do(ctx)
		if err != nil {
			t.Fatalf("QueryLedgers: %v", err)
		}
		apitest.AssertCovers(t, "QueryLedgers", raw, resp)
		if _, ok := resp[ledgerID]; !ok {
			t.Errorf("QueryLedgers missing %s", ledgerID)
		}
		pace()
	} else {
		t.Log("QueryLedgers: no ledger entry to query; skipped")
	}

	// 13. Get Trade Volume.
	{
		params := map[string]string{"pair": "XBTUSD"}
		raw := apitest.FetchRawPost(t, c, ctx, "/0/private/TradeVolume", params)
		resp, err := c.NewGetTradeVolumeService().SetPair("XBTUSD").Do(ctx)
		if err != nil {
			t.Fatalf("TradeVolume: %v", err)
		}
		apitest.AssertCovers(t, "TradeVolume", raw, resp)
		pace()
	}

	// 14-17. Export report lifecycle: Add -> Status -> Retrieve -> Remove.
	testExportLifecycle(t, c, ctx)
}

func testExportLifecycle(t *testing.T, c *Client, ctx context.Context) {
	// 14. Request Export Report.
	ref, err := c.NewRequestExportReportService(ReportTypeTrades, "gokraken_test").Do(ctx)
	if err != nil {
		if apitest.Tolerable(t, "AddExport", err, "Permission denied", "permission") {
			return
		}
		t.Fatalf("AddExport: %v", err)
	}
	if ref.ID == "" {
		t.Fatal("AddExport returned empty id")
	}
	t.Logf("AddExport: id=%s", ref.ID)
	pace()

	// 15. Get Export Report Status (cover the report object fields).
	raw := apitest.FetchRawPost(t, c, ctx, "/0/private/ExportStatus", map[string]string{"report": "trades"})
	reports, err := c.NewGetExportReportStatusService(ReportTypeTrades).Do(ctx)
	if err != nil {
		t.Fatalf("ExportStatus: %v", err)
	}
	apitest.AssertCovers(t, "ExportStatus", raw, reports)

	// Find our report and wait briefly for it to be Processed.
	var status string
	for attempt := 0; attempt < 5; attempt++ {
		reports, err = c.NewGetExportReportStatusService(ReportTypeTrades).Do(ctx)
		if err != nil {
			t.Fatalf("ExportStatus poll: %v", err)
		}
		for _, r := range reports {
			if r.ID == ref.ID {
				status = r.Status
			}
		}
		if status == "Processed" {
			break
		}
		time.Sleep(2 * time.Second)
	}
	t.Logf("ExportStatus: report %s status=%s", ref.ID, status)

	// 16. Retrieve Data Export (binary ZIP) once processed.
	if status == "Processed" {
		data, err := c.NewRetrieveDataExportService(ref.ID).Do(ctx)
		if err != nil {
			t.Fatalf("RetrieveExport: %v", err)
		}
		if !bytes.HasPrefix(data, []byte("PK")) {
			t.Errorf("RetrieveExport: expected a ZIP archive, got %d bytes prefix %q", len(data), firstBytes(data, 8))
		} else {
			t.Logf("RetrieveExport: OK, %d-byte ZIP", len(data))
		}
		pace()
	} else {
		t.Logf("RetrieveExport: report not processed yet; skipped")
	}

	// 17. Delete Export Report (delete if processed, else cancel).
	opType := DeleteExportReportTypeDelete
	if status != "Processed" {
		opType = DeleteExportReportTypeCancel
	}
	del, err := c.NewDeleteExportReportService(ref.ID, opType).Do(ctx)
	if err != nil {
		t.Fatalf("RemoveExport: %v", err)
	}
	if !del.Delete && !del.Cancel {
		t.Errorf("RemoveExport: neither delete nor cancel succeeded: %+v", del)
	}
	t.Logf("RemoveExport: %+v", del)
}

func firstBytes(b []byte, n int) []byte {
	if len(b) < n {
		return b
	}
	return b[:n]
}
