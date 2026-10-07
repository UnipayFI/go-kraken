package kraken

import (
	"errors"
	"testing"
	"time"

	"github.com/UnipayFI/go-kraken/client"
	"github.com/UnipayFI/go-kraken/internal/apitest"
)

// TestAffiliate exercises the affiliate (KOL) reporting endpoints. Keys that are
// not the main referrer on a live KOL plan get 403 ReferrerNotWhitelisted, which
// is tolerated: the path and GET signing were still accepted.
func TestAffiliate(t *testing.T) {
	c := NewClient(apitest.AuthOptions(t)...)
	ctx := apitest.Ctx(t)

	// 1. Get Daily Activity (day variant, yesterday UTC).
	{
		date := time.Now().UTC().AddDate(0, 0, -1).Format(time.DateOnly)
		resp, err := c.NewGetDailyActivityService().SetActivityDate(date).SetLimit(10).Do(ctx)
		var problem *client.ProblemError
		if errors.As(err, &problem) && problem.Status == 403 {
			t.Logf("DailyActivity: account is not an affiliate (%s) — endpoint+signing OK", problem.Error())
			return
		}
		if err != nil {
			t.Fatalf("DailyActivity: %v", err)
		}
		params := map[string]string{"activity_date": date, "limit": "10"}
		raw := apitest.FetchRawBareGet(t, c, ctx, "/affiliate/v1/daily-activity", params)
		apitest.AssertCovers(t, "DailyActivity", raw, resp)
		if resp.Currency == "" || resp.GeneratedAt.IsZero() || resp.Limit != 10 {
			t.Errorf("DailyActivity has zero fields: %+v", resp)
		}
		for _, it := range resp.Items {
			if it.RefereeReference == "" || len(it.Plans) == 0 {
				t.Errorf("DailyActivity item has zero fields: %+v", it)
			}
		}
	}

	// 2. Get Daily Activity (full roster: quiet users and opted-out stubs; also
	// yields an IIBAN for CPA progress).
	var iiban string
	{
		date := time.Now().UTC().AddDate(0, 0, -1).Format(time.DateOnly)
		resp, err := c.NewGetDailyActivityService().SetActivityDate(date).SetAllUsers(true).SetLimit(10).Do(ctx)
		if err != nil {
			t.Fatalf("DailyActivity(all_users): %v", err)
		}
		params := map[string]string{"activity_date": date, "all_users": "true", "limit": "10"}
		raw := apitest.FetchRawBareGet(t, c, ctx, "/affiliate/v1/daily-activity", params)
		apitest.AssertCovers(t, "DailyActivity(all_users)", raw, resp)
		for _, it := range resp.Items {
			if it.OptedOut && it.EnrolledAt.IsZero() {
				t.Errorf("DailyActivity(all_users) opted-out stub has no enrolled_at: %+v", it)
			}
			if !it.OptedOut && it.IIBAN == "" {
				t.Errorf("DailyActivity(all_users) visible item has no iiban: %+v", it)
			}
			if !it.OptedOut && iiban == "" {
				iiban = it.IIBAN
			}
		}
	}

	// 3. Get CPA Progress.
	if iiban != "" {
		resp, err := c.NewGetAffiliateCPAProgressService(iiban).Do(ctx)
		if err != nil {
			t.Fatalf("CPAProgress: %v", err)
		}
		raw := apitest.FetchRawBareGet(t, c, ctx, "/affiliate/v1/cpa-progress", map[string]string{"iiban": iiban})
		apitest.AssertCovers(t, "CPAProgress", raw, resp)
		for _, b := range resp.Progress.Bounties {
			if b.Status == "" || b.RewardAsset == "" || b.ProgressAvailability == "" {
				t.Errorf("CPAProgress bounty has zero fields: %+v", b)
			}
		}
		t.Logf("CPAProgress: %d bount(ies)", len(resp.Progress.Bounties))
	} else {
		t.Log("CPAProgress: no visible referred user to query; skipped")
	}

	// 4. Get Payout History (two pages when there are more than two payouts).
	{
		resp, err := c.NewGetAffiliatePayoutHistoryService().SetLimit(2).Do(ctx)
		if err != nil {
			t.Fatalf("PayoutHistory: %v", err)
		}
		raw := apitest.FetchRawBareGet(t, c, ctx, "/affiliate/v1/payout-history", map[string]string{"limit": "2"})
		apitest.AssertCovers(t, "PayoutHistory", raw, resp)
		if resp.Limit != 2 || len(resp.Items) > 2 {
			t.Errorf("PayoutHistory: limit=%d with %d item(s)", resp.Limit, len(resp.Items))
		}
		seen := map[string]bool{}
		for _, p := range resp.Items {
			if p.PayoutID == "" || p.Status == "" || p.Source == "" || p.CreatedAt.IsZero() {
				t.Errorf("PayoutHistory item has zero fields: %+v", p)
			}
			seen[p.PayoutID] = true
		}
		if resp.NextCursor != "" {
			next, err := c.NewGetAffiliatePayoutHistoryService().SetLimit(2).SetCursor(resp.NextCursor).Do(ctx)
			if err != nil {
				t.Fatalf("PayoutHistory(page 2): %v", err)
			}
			for _, p := range next.Items {
				if seen[p.PayoutID] {
					t.Errorf("PayoutHistory: %s on both pages", p.PayoutID)
				}
			}
		}
		t.Logf("PayoutHistory: %d item(s), paid=%s pending=%s, next=%q", len(resp.Items), resp.Summary.Paid, resp.Summary.Pending, resp.NextCursor)
	}
}
