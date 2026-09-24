package kraken

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/UnipayFI/go-kraken/request"
	"github.com/shopspring/decimal"
)

// Affiliate endpoints report referred-user activity for approved affiliate (KOL)
// partners. Unlike the /0 spot API they sit directly on https://api.kraken.com
// (no /0 prefix), are signed GETs (nonce in the API-Nonce header, query string
// in the signed path) and return the object itself instead of the
// {error, result} envelope; errors come back as *client.ProblemError. The key
// needs permission to query referrals, and the caller must be the main referrer
// on a live KOL plan (otherwise 403 ReferrerNotWhitelisted).

// Affiliate product keys used in the open-ended Products / Totals maps. Unknown
// keys may appear and should be ignored; an absent key means zero activity.
const (
	AffiliateProductSpotTrading                = "spot_trading"                  // order-book spot
	AffiliateProductPTLTrading                 = "ptl_trading"                   // instant buy/sell (no maker/taker)
	AffiliateProductMarginTrading              = "margin_trading"                // margin trades
	AffiliateProductMarginRollover             = "margin_rollover"               // financing charge (fee-only, no volume)
	AffiliateProductFuturesOrderFill           = "futures_order_fill"            // futures
	AffiliateProductTokenizedEquitySpotTrading = "tokenized_equity_spot_trading" // xStocks, order book
	AffiliateProductTokenizedEquityPTLTrading  = "tokenized_equity_ptl_trading"  // xStocks, instant (no maker/taker)
)

// ===========================================================================
// 1. Get Daily Activity -- GET /affiliate/v1/daily-activity
// ===========================================================================

// GetDailyActivityService returns one page of referred-user activity for the
// authenticated affiliate partner. It has two mutually exclusive variants
// (sending both is a 400):
//
//   - Day: SetActivityDate — every visible enrolled referred user active that
//     UTC trade day, plus day-wide totals.
//   - History: SetIIBAN + SetStartDate + SetEndDate — the same per-day entries
//     for up to ten known participants over an inclusive date range; day-wide
//     fields (Totals, ActiveUsers, Revision, Estimated, OptedOut) are omitted.
//
// Dates are YYYY-MM-DD and must be today or one of the previous 89 UTC days.
// Page with SetCursor(resp.NextCursor) while NextCursor is non-empty.
type GetDailyActivityService struct {
	c      *Client
	params map[string]string
}

func (c *Client) NewGetDailyActivityService() *GetDailyActivityService {
	return &GetDailyActivityService{c: c, params: map[string]string{}}
}

// SetActivityDate selects the day variant for one UTC trade date (YYYY-MM-DD).
func (s *GetDailyActivityService) SetActivityDate(date string) *GetDailyActivityService {
	s.params["activity_date"] = date
	return s
}

// SetIIBAN selects the history variant for one to ten full IIBANs that
// participants have shared. Empty, duplicate or malformed values are a 400.
func (s *GetDailyActivityService) SetIIBAN(iibans ...string) *GetDailyActivityService {
	s.params["iiban"] = strings.Join(iibans, ",")
	return s
}

// SetStartDate sets the inclusive history start date (YYYY-MM-DD).
func (s *GetDailyActivityService) SetStartDate(date string) *GetDailyActivityService {
	s.params["start_date"] = date
	return s
}

// SetEndDate sets the inclusive history end date (YYYY-MM-DD), on or after the
// start date.
func (s *GetDailyActivityService) SetEndDate(date string) *GetDailyActivityService {
	s.params["end_date"] = date
	return s
}

// SetCursor continues from a previous page's NextCursor.
func (s *GetDailyActivityService) SetCursor(cursor string) *GetDailyActivityService {
	s.params["cursor"] = cursor
	return s
}

// SetLimit sets the page size (1-200, default 50).
func (s *GetDailyActivityService) SetLimit(limit int) *GetDailyActivityService {
	s.params["limit"] = strconv.Itoa(limit)
	return s
}

func (s *GetDailyActivityService) Do(ctx context.Context) (*DailyActivity, error) {
	return request.DoBare[DailyActivity](request.Get(ctx, s.c, "/affiliate/v1/daily-activity", s.params).WithSign())
}

// DailyActivity is one page of affiliate daily activity. Monetary figures are
// decimal strings in Currency (the reporting currency, not the payout asset).
// Day-wide payable is Totals plus OptedOut.Summary when Summary is set.
type DailyActivity struct {
	ActivityDate string                     `json:"activity_date"` // day variant only: the UTC trade date every item shares
	Currency     string                     `json:"currency"`      // reporting currency for every monetary figure (always USD today)
	Revision     time.Time                  `json:"revision"`      // day variant, first page: when these figures were last written; zero when no rows
	GeneratedAt  time.Time                  `json:"generated_at"`  // response generation time
	ActiveUsers  int64                      `json:"active_users"`  // day variant, first page: visible enrolled users that day
	Totals       map[string]ProductActivity `json:"totals"`        // day variant, first page: totals for visible users, keyed by product
	NextCursor   string                     `json:"next_cursor"`   // present when more results exist; pass to SetCursor
	Items        []DailyActivityItem        `json:"items"`         // one entry per referred user (per day in the history variant)
	Estimated    bool                       `json:"estimated"`     // day variant, first page: amounts are still an estimate
	OptedOut     *OptedOutActivity          `json:"opted_out"`     // day variant, first page: opted-out remainder
	Limit        int                        `json:"limit"`         // page size used for this response
}

// DailyActivityItem is one referred user's activity, by plan and then product.
type DailyActivityItem struct {
	ActivityDate     string         `json:"activity_date"`     // history variant only: this entry's own day
	MaskedIIBAN      string         `json:"masked_iiban"`      // last four IIBAN characters; not unique, do not join on it
	Plans            []ReferralPlan `json:"plans"`             // the caller's plans this user is enrolled in
	RefereeReference string         `json:"referee_reference"` // partner-scoped stable join key across days
	Estimated        bool           `json:"estimated"`         // history variant: amounts for this date are an estimate
}

// ReferralPlan is one of the caller's reward plans a referred user is enrolled in.
type ReferralPlan struct {
	ReferralCode  string                     `json:"referral_code"`  // referral code
	Campaign      string                     `json:"campaign"`       // enrollment suffix, not the plan name
	ReferralLevel int                        `json:"referral_level"` // referral level
	EnrolledAt    time.Time                  `json:"enrolled_at"`    // enrollment time, truncated to the hour
	Status        string                     `json:"status"`         // open vocabulary; "active" and "completed" are payable
	Earning       bool                       `json:"earning"`        // whether this plan currently pays the caller
	ExpiresAt     time.Time                  `json:"expires_at"`     // plan expiry
	Products      map[string]ProductActivity `json:"products"`       // activity keyed by product (see AffiliateProduct*)
}

// ProductActivity is payable activity for one product. GeoBlocked is an extra
// breakdown not included in the parent figures. Maker/Taker are present only on
// execution products and only when classified; nil means unclassified, not zero.
type ProductActivity struct {
	Volume     decimal.Decimal     `json:"volume"`      // traded volume; absent for fee-only products (margin_rollover)
	Fees       decimal.Decimal     `json:"fees"`        // fees paid
	Commission decimal.Decimal     `json:"commission"`  // commission earned
	EventCount int                 `json:"event_count"` // number of events
	GeoBlocked *GeoBlockedActivity `json:"geo_blocked"` // blocked-region activity (pays zero commission)
	Maker      *MakerTakerActivity `json:"maker"`       // maker-side portion
	Taker      *MakerTakerActivity `json:"taker"`       // taker-side portion
}

// MakerTakerActivity is payable activity for one liquidity role.
type MakerTakerActivity struct {
	Volume     decimal.Decimal `json:"volume"`      // traded volume
	Fees       decimal.Decimal `json:"fees"`        // fees paid
	Commission decimal.Decimal `json:"commission"`  // commission earned
	EventCount int             `json:"event_count"` // number of events
}

// GeoBlockedActivity is geo-blocked activity for one product.
type GeoBlockedActivity struct {
	Volume     decimal.Decimal `json:"volume"`      // traded volume; absent for fee-only products
	Fees       decimal.Decimal `json:"fees"`        // fees paid
	EventCount int             `json:"event_count"` // number of events
}

// OptedOutActivity is the opted-out remainder: exactly one of Suppressed (group
// too small to show, or empty) or Summary is set.
type OptedOutActivity struct {
	Suppressed *struct{}        `json:"suppressed"` // set when the opted-out group is not shown
	Summary    *OptedOutSummary `json:"summary"`    // set otherwise
}

// OptedOutSummary aggregates activity of opted-out participants.
type OptedOutSummary struct {
	ActiveParticipants int64                      `json:"active_participants"` // opted-out participants active that day
	Products           map[string]ProductActivity `json:"products"`            // activity keyed by product
}
