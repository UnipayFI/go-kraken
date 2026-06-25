package kraken

import (
	"testing"

	"github.com/UnipayFI/go-kraken/internal/apitest"
	"github.com/shopspring/decimal"
)

// TestSubaccounts exercises the institutional-only subaccount endpoints. On a
// standard account they return a permission error; the test verifies the
// request path and signing are correct and tolerates that error.
func TestSubaccounts(t *testing.T) {
	c := NewClient(apitest.AuthOptions(t)...)
	ctx := apitest.Ctx(t)

	// 1. Create Subaccount.
	{
		_, err := c.NewCreateSubaccountService("gokrakentest", "gokrakentest@example.com").Do(ctx)
		if err == nil {
			t.Log("CreateSubaccount: unexpectedly succeeded (institutional capability present)")
		} else if !apitest.Tolerable(t, "CreateSubaccount", err,
			"Permission denied", "permission", "not allowed", "institutional", "Unable") {
			t.Fatalf("CreateSubaccount: unexpected error: %v", err)
		}
		pace()
	}

	// 2. Account Transfer.
	{
		_, err := c.NewAccountTransferService("USDT", decimal.RequireFromString("1"), "master", "subaccount").Do(ctx)
		if err == nil {
			t.Log("AccountTransfer: unexpectedly succeeded")
		} else if !apitest.Tolerable(t, "AccountTransfer", err,
			"Permission denied", "permission", "not allowed", "institutional", "Unknown", "Invalid") {
			t.Fatalf("AccountTransfer: unexpected error: %v", err)
		}
	}
}
