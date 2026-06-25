package kraken

import (
	"context"
	"testing"
	"time"

	"github.com/UnipayFI/go-kraken/internal/apitest"
	"github.com/shopspring/decimal"
)

// TestFunding exercises the funding endpoints against the live API. The
// fund-moving Withdraw endpoint is implemented but intentionally never executed;
// WithdrawCancel and WalletTransfer are driven into expected error responses so
// their signing + request path are verified without moving funds.
func TestFunding(t *testing.T) {
	c := NewClient(apitest.AuthOptions(t)...)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	// 1. Get Deposit Methods.
	{
		raw := apitest.FetchRawPost(t, c, ctx, "/0/private/DepositMethods", map[string]string{"asset": "USDT"})
		resp, err := c.NewGetDepositMethodsService("USDT").Do(ctx)
		if err != nil {
			t.Fatalf("DepositMethods: %v", err)
		}
		apitest.AssertCovers(t, "DepositMethods", raw, resp)
		if len(resp) == 0 {
			t.Error("DepositMethods: empty")
		}
		pace()
	}

	// 2. Get Deposit Addresses (generate one so there is a row to cover).
	{
		params := map[string]string{"asset": "USDT", "method": "Tether USD (TRC20)", "new": "true"}
		raw := apitest.FetchRawPost(t, c, ctx, "/0/private/DepositAddresses", params)
		resp, err := c.NewGetDepositAddressesService("USDT", "Tether USD (TRC20)").SetNew(true).Do(ctx)
		if err != nil {
			t.Fatalf("DepositAddresses: %v", err)
		}
		apitest.AssertCovers(t, "DepositAddresses", raw, resp)
		if len(resp) > 0 && resp[0].Address == "" {
			t.Error("DepositAddresses: empty address")
		}
		pace()
	}

	// 3. Get Status of Recent Deposits.
	{
		raw := apitest.FetchRawPost(t, c, ctx, "/0/private/DepositStatus", map[string]string{"asset": "USDT"})
		resp, err := c.NewGetDepositStatusService().SetAsset("USDT").Do(ctx)
		if err != nil {
			t.Fatalf("DepositStatus: %v", err)
		}
		apitest.AssertCovers(t, "DepositStatus", raw, resp)
		t.Logf("DepositStatus: %d record(s)", len(resp))
		pace()
	}

	// 4. Get Withdrawal Methods (nested fee + limits).
	{
		raw := apitest.FetchRawPost(t, c, ctx, "/0/private/WithdrawMethods", map[string]string{"asset": "USDT"})
		resp, err := c.NewGetWithdrawMethodsService().SetAsset("USDT").Do(ctx)
		if err != nil {
			t.Fatalf("WithdrawMethods: %v", err)
		}
		apitest.AssertCovers(t, "WithdrawMethods", raw, resp)
		pace()
	}

	// 5. Get Withdrawal Addresses.
	{
		raw := apitest.FetchRawPost(t, c, ctx, "/0/private/WithdrawAddresses", map[string]string{"asset": "USDT"})
		resp, err := c.NewGetWithdrawAddressesService().SetAsset("USDT").Do(ctx)
		if err != nil {
			t.Fatalf("WithdrawAddresses: %v", err)
		}
		apitest.AssertCovers(t, "WithdrawAddresses", raw, resp)
		t.Logf("WithdrawAddresses: %d saved", len(resp))
		pace()
	}

	// 6. Get Withdrawal Information (uses a real saved key, no funds move).
	{
		key := firstWithdrawKey(t, c, ctx)
		if key == "" {
			t.Log("WithdrawInfo: no saved withdrawal key; skipped")
		} else {
			params := map[string]string{"asset": "USDT", "key": key, "amount": "10"}
			raw := apitest.FetchRawPost(t, c, ctx, "/0/private/WithdrawInfo", params)
			resp, err := c.NewGetWithdrawInfoService("USDT", key, decimal.RequireFromString("10")).Do(ctx)
			if err != nil {
				if !apitest.Tolerable(t, "WithdrawInfo", err, "limit", "Unknown", "Invalid amount") {
					t.Fatalf("WithdrawInfo: %v", err)
				}
			} else {
				apitest.AssertCovers(t, "WithdrawInfo", raw, resp)
				t.Logf("WithdrawInfo: method=%s fee=%s amount=%s", resp.Method, resp.Fee, resp.Amount)
			}
		}
		pace()
	}

	// 7. Withdraw Funds — implemented but never executed (fund-moving). Compile
	// reference only.
	_ = c.NewWithdrawService

	// 8. Get Status of Recent Withdrawals.
	{
		raw := apitest.FetchRawPost(t, c, ctx, "/0/private/WithdrawStatus", map[string]string{"asset": "USDT"})
		resp, err := c.NewGetWithdrawStatusService().SetAsset("USDT").Do(ctx)
		if err != nil {
			t.Fatalf("WithdrawStatus: %v", err)
		}
		apitest.AssertCovers(t, "WithdrawStatus", raw, resp)
		t.Logf("WithdrawStatus: %d record(s)", len(resp))
		pace()
	}

	// 9. Request Withdrawal Cancellation — no pending withdrawal exists, so an
	// unknown refid is expected to error (verifies signing + path).
	{
		_, err := c.NewWithdrawCancelService("USDT", "FAKEREF-000000-000000").Do(ctx)
		if err == nil {
			t.Log("WithdrawCancel: unexpectedly succeeded (no pending withdrawal expected)")
		} else if !apitest.Tolerable(t, "WithdrawCancel", err, "Unknown", "Invalid", "No withdrawal", "not found") {
			t.Fatalf("WithdrawCancel: unexpected error: %v", err)
		}
		pace()
	}

	// 10. Request Wallet Transfer — a deliberately oversized amount triggers an
	// expected funding error (verifies signing + path; no funds move).
	{
		_, err := c.NewWalletTransferService("USDT", decimal.RequireFromString("999999")).Do(ctx)
		if err == nil {
			t.Error("WalletTransfer: oversized transfer unexpectedly succeeded")
		} else if !apitest.Tolerable(t, "WalletTransfer", err, "Insufficient", "funding", "Unknown", "Permission", "wallet") {
			t.Fatalf("WalletTransfer: unexpected error: %v", err)
		}
	}
}

// firstWithdrawKey returns the first saved USDT withdrawal key, or "".
func firstWithdrawKey(t *testing.T, c *Client, ctx context.Context) string {
	t.Helper()
	addrs, err := c.NewGetWithdrawAddressesService().SetAsset("USDT").Do(ctx)
	if err != nil || len(addrs) == 0 {
		return ""
	}
	return addrs[0].Key
}
