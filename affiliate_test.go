package kraken

import (
	"errors"
	"testing"
	"time"

	"github.com/UnipayFI/go-kraken/client"
	"github.com/UnipayFI/go-kraken/internal/apitest"
)

// TestAffiliate exercises the affiliate (KOL) reporting endpoint. Keys that are
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

	// 2. Get Daily Activity (full roster: quiet users and opted-out stubs).
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
		}
	}
}
