package kraken

import (
	"context"
	"testing"
	"time"

	"github.com/UnipayFI/go-kraken/internal/apitest"
)

// TestAccountExtra exercises the later additions to the account/market surface
// that require authentication: the L3 order book, credit lines, API-key info and
// wallet accounts.
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
		if len(resp.Asks) == 0 || resp.Asks[0].OrderID == "" || resp.Asks[0].Timestamp.IsZero() {
			t.Errorf("Level3 ask entry invalid: %+v", resp.Asks)
		} else if ts := resp.Asks[0].Timestamp; ts.Year() < 2013 || ts.After(time.Now().Add(24*time.Hour)) {
			// A wrong format unit never errors; it only misdates.
			t.Errorf("Level3 ask timestamp %v is implausible; wrong format unit?", ts)
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
		pace()
	}

	// 4. List Wallet Accounts, then re-read Balance scoped to one of them.
	{
		raw := apitest.FetchRawPost(t, c, ctx, "/0/private/ListWalletAccounts", nil)
		resp, err := c.NewListWalletAccountsService().Do(ctx)
		if err != nil {
			t.Fatalf("ListWalletAccounts: %v", err)
		}
		apitest.AssertCovers(t, "ListWalletAccounts", raw, resp)
		if len(resp.Accounts) == 0 {
			t.Fatalf("ListWalletAccounts: expected at least the default wallet, got none")
		}
		acct := resp.Accounts[0]
		if acct.AccountID == "" || acct.Status == "" || acct.Type == "" {
			t.Errorf("ListWalletAccounts entry incomplete: %+v", acct)
		}
		t.Logf("ListWalletAccounts: %d wallet(s), first=%s type=%s status=%s", len(resp.Accounts), acct.AccountID, acct.Type, acct.Status)
		pace()

		params := map[string]string{"account_id": acct.AccountID}
		balRaw := apitest.FetchRawPost(t, c, ctx, "/0/private/Balance", params)
		bal, err := c.NewGetAccountBalanceService().SetAccountID(acct.AccountID).Do(ctx)
		if err != nil {
			t.Fatalf("Balance(account_id=%s): %v", acct.AccountID, err)
		}
		apitest.AssertCovers(t, "Balance(account_id)", balRaw, bal)
		t.Logf("Balance(account_id=%s): %d asset(s)", acct.AccountID, len(bal))
	}
}
