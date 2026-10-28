package kraken

import (
	"context"
	"strings"
	"time"

	"github.com/UnipayFI/go-kraken/request"
	"github.com/shopspring/decimal"
)

// Algo Trading endpoints manage strategies: parent instructions that Kraken
// executes as a series of regular child orders. The only strategy type today is
// TWAP, which works a total volume evenly between starttm and expiretm with
// immediate-or-cancel limit child orders (they never rest on the book).
// Completed child orders appear in ClosedOrders and trade history.
//
// Lifecycle: pending (waiting for starttm) -> open (running) -> closed (fully
// filled) / expired (expiretm reached first) / cancelled (CancelStrategy).

// StrategyType is the kind of algo strategy.
type StrategyType string

const (
	StrategyTypeTWAP StrategyType = "twap" // time-weighted average price
)

// StrategyStatus is an algo strategy's lifecycle status.
type StrategyStatus string

const (
	StrategyStatusPending   StrategyStatus = "pending"   // accepted, waiting for starttm
	StrategyStatusOpen      StrategyStatus = "open"      // running, child orders are being placed
	StrategyStatusClosed    StrategyStatus = "closed"    // fully filled before expiretm
	StrategyStatusExpired   StrategyStatus = "expired"   // expiretm reached before being fully filled
	StrategyStatusCancelled StrategyStatus = "cancelled" // cancelled with CancelStrategy
)

// StrategyInfo is the state of a strategy, shared by GetOpenStrategies,
// QueryStrategies and GetClosedStrategies.
type StrategyInfo struct {
	StrategyID string          `json:"strategy_id"` // Kraken strategy id (always starts with "AS")
	Status     StrategyStatus  `json:"status"`      // pending, open, closed, expired, cancelled
	Reason     string          `json:"reason"`      // status explanation when applicable (insufficientFunds, remainingBelowMinimum, userRequested, ...); may also qualify an open strategy
	Details    StrategyDetails `json:"details"`     // strategy parameters and execution progress
}

// StrategyDetails holds a TWAP strategy's parameters and execution progress.
type StrategyDetails struct {
	StrategyType StrategyType    `json:"strategy_type"`        // twap
	Side         OrderSide       `json:"side"`                 // buy or sell
	Volume       decimal.Decimal `json:"volume"`               // total quantity (base currency)
	FilledVolume decimal.Decimal `json:"filled_volume"`        // quantity executed so far (base currency)
	AvgPrice     decimal.Decimal `json:"avg_price"`            // average execution price (total_cost / filled_volume); zero until the first fill
	TotalCost    decimal.Decimal `json:"total_cost"`           // total cost of the executed quantity (quote currency)
	TotalFee     decimal.Decimal `json:"total_fee"`            // aggregate fees across fills (quote currency)
	StartTime    time.Time       `json:"starttm,format:unix"`  // strategy start time
	ExpireTime   time.Time       `json:"expiretm,format:unix"` // strategy end time
	Symbol       string          `json:"symbol"`               // asset pair, e.g. BTC/USD
	AssetClass   string          `json:"asset_class"`          // forex, tokenized_asset or synthetic_pair
	OrderType    OrderType       `json:"ordertype"`            // child order type: market or limit
	Price        decimal.Decimal `json:"price"`                // child order limit price (limit only)
}

// ===========================================================================
// 1. Add Strategy -- POST /0/private/AddStrategy
// ===========================================================================

// AddStrategyService creates an algo strategy. The resolved horizon (expiretm -
// starttm) must be at least 60 seconds and both times no more than 30 days from
// now; symbol, volume and price follow the same pair, precision and order
// minimum rules as AddOrder. xStocks pairs are not supported. Use
// SetValidate(true) to validate without creating the strategy.
type AddStrategyService struct {
	c      *Client
	params map[string]string
}

// NewAddStrategyService builds a strategy of the given type (StrategyTypeTWAP)
// on symbol (pair id or altname, e.g. "BTC/USD" or "XBTUSD") that executes
// volume (base currency) as orderType (OrderTypeMarket or OrderTypeLimit) child
// orders until expireTime (a unix timestamp or "+<n>" seconds from now). The
// asset class is "forex", the only one supported for creation.
func (c *Client) NewAddStrategyService(strategyType StrategyType, symbol string, side OrderSide, orderType OrderType, volume decimal.Decimal, expireTime string) *AddStrategyService {
	return &AddStrategyService{c: c, params: map[string]string{
		"strategy_type": string(strategyType),
		"symbol":        symbol,
		"asset_class":   "forex",
		"side":          string(side),
		"volume":        volume.String(),
		"ordertype":     string(orderType),
		"expiretm":      expireTime,
	}}
}

// SetPrice sets the child orders' limit price. Required for OrderTypeLimit and
// must be omitted for OrderTypeMarket.
func (s *AddStrategyService) SetPrice(price decimal.Decimal) *AddStrategyService {
	s.params["price"] = price.String()
	return s
}

// SetStartTime sets the strategy start time ("0" = now, the default; a unix
// timestamp; or "+<n>" seconds from now).
func (s *AddStrategyService) SetStartTime(startTime string) *AddStrategyService {
	s.params["starttm"] = startTime
	return s
}

// SetValidate validates the request without creating the strategy when true
// (the response's StrategyID is then empty). The creation-time balance check is
// not run.
func (s *AddStrategyService) SetValidate(validate bool) *AddStrategyService {
	s.params["validate"] = formatBool(validate)
	return s
}

func (s *AddStrategyService) Do(ctx context.Context) (*AddStrategyResult, error) {
	return request.Do[AddStrategyResult](request.Post(ctx, s.c, "/0/private/AddStrategy", s.params).WithSign())
}

// AddStrategyResult is the AddStrategy response.
type AddStrategyResult struct {
	StrategyID string `json:"strategy_id"` // id of the new strategy; empty in validate mode
}

// ===========================================================================
// 2. Cancel Strategy -- POST /0/private/CancelStrategy
// ===========================================================================

// CancelStrategyService cancels a pending or open strategy. Child-order
// scheduling stops, but an already-submitted child order may still fill (its
// fills are kept). The strategy normally ends as cancelled with reason
// userRequested, or as closed if the in-flight order completes the volume; poll
// QueryStrategies for the terminal status and final totals.
type CancelStrategyService struct {
	c      *Client
	params map[string]string
}

func (c *Client) NewCancelStrategyService(strategyID string) *CancelStrategyService {
	return &CancelStrategyService{c: c, params: map[string]string{"strategy_id": strategyID}}
}

func (s *CancelStrategyService) Do(ctx context.Context) (*CancelStrategyResult, error) {
	return request.Do[CancelStrategyResult](request.Post(ctx, s.c, "/0/private/CancelStrategy", s.params).WithSign())
}

// CancelStrategyResult is the CancelStrategy response.
type CancelStrategyResult struct {
	StrategyID string `json:"strategy_id"` // id of the cancelled strategy
}

// ===========================================================================
// 3. Get Open Strategies -- POST /0/private/GetOpenStrategies
// ===========================================================================

// GetOpenStrategiesService returns the account's active (pending or open)
// strategies. On a selected sub-account only its strategies are returned.
type GetOpenStrategiesService struct {
	c      *Client
	params map[string]string
}

func (c *Client) NewGetOpenStrategiesService() *GetOpenStrategiesService {
	return &GetOpenStrategiesService{c: c, params: map[string]string{}}
}

// SetStrategyType restricts results to one strategy type (default all).
func (s *GetOpenStrategiesService) SetStrategyType(strategyType StrategyType) *GetOpenStrategiesService {
	s.params["strategy_type"] = string(strategyType)
	return s
}

func (s *GetOpenStrategiesService) Do(ctx context.Context) (*OpenStrategiesResult, error) {
	return request.Do[OpenStrategiesResult](request.Post(ctx, s.c, "/0/private/GetOpenStrategies", s.params).WithSign())
}

// OpenStrategiesResult holds the open strategies keyed by strategy id (always
// present, possibly empty).
type OpenStrategiesResult struct {
	Open map[string]StrategyInfo `json:"open"`
}

// ===========================================================================
// 4. Query Strategies -- POST /0/private/QueryStrategies
// ===========================================================================

// QueryStrategiesService returns specific strategies (open or terminal) by id.
// Ids that are not found or belong to another account are silently left out.
type QueryStrategiesService struct {
	c      *Client
	params map[string]string
}

// NewQueryStrategiesService queries up to 20 strategies by their ids.
func (c *Client) NewQueryStrategiesService(strategyIDs ...string) *QueryStrategiesService {
	return &QueryStrategiesService{c: c, params: map[string]string{"strategy_ids": strings.Join(strategyIDs, ",")}}
}

func (s *QueryStrategiesService) Do(ctx context.Context) (*QueryStrategiesResult, error) {
	return request.Do[QueryStrategiesResult](request.Post(ctx, s.c, "/0/private/QueryStrategies", s.params).WithSign())
}

// QueryStrategiesResult holds the matched strategies keyed by strategy id
// (empty when none match).
type QueryStrategiesResult struct {
	Strategies map[string]StrategyInfo `json:"strategies"`
}

// ===========================================================================
// 5. Get Closed Strategies -- POST /0/private/GetClosedStrategies
// ===========================================================================

// GetClosedStrategiesService returns terminal (closed, expired or cancelled)
// strategies, newest first, with cursor pagination: while more remain the
// response's NextCursor is set; pass it to SetCursor for the next page. The
// cursor stores only a sort position, so repeat the same filters on every page.
// Offset paging and a total count are not supported.
type GetClosedStrategiesService struct {
	c      *Client
	params map[string]string
}

func (c *Client) NewGetClosedStrategiesService() *GetClosedStrategiesService {
	return &GetClosedStrategiesService{c: c, params: map[string]string{}}
}

// SetAssetClass restricts results to one asset class (forex, tokenized_asset or
// synthetic_pair; default all).
func (s *GetClosedStrategiesService) SetAssetClass(assetClass string) *GetClosedStrategiesService {
	s.params["asset_class"] = assetClass
	return s
}

// SetStrategyType restricts results to one strategy type (default all).
func (s *GetClosedStrategiesService) SetStrategyType(strategyType StrategyType) *GetClosedStrategiesService {
	s.params["strategy_type"] = string(strategyType)
	return s
}

// SetStatus restricts results to one terminal status (StrategyStatusClosed,
// StrategyStatusExpired or StrategyStatusCancelled; default all three).
func (s *GetClosedStrategiesService) SetStatus(status StrategyStatus) *GetClosedStrategiesService {
	s.params["status"] = string(status)
	return s
}

// SetStart sets the inclusive lower time bound (applied per SetQueryTimeMode).
func (s *GetClosedStrategiesService) SetStart(t time.Time) *GetClosedStrategiesService {
	s.params["start"] = formatUnix(t)
	return s
}

// SetEnd sets the inclusive upper time bound, including the entire end second
// (applied per SetQueryTimeMode).
func (s *GetClosedStrategiesService) SetEnd(t time.Time) *GetClosedStrategiesService {
	s.params["end"] = formatUnix(t)
	return s
}

// SetQueryTimeMode selects which timestamps the bounds apply to: "open" (the
// opening time, not the scheduled starttm; results ordered by opening time),
// "close" (the time the strategy became terminal), or "both" (default: start
// bounds the opening time and end the closing time).
func (s *GetClosedStrategiesService) SetQueryTimeMode(mode string) *GetClosedStrategiesService {
	s.params["query_time_mode"] = mode
	return s
}

// SetLimit sets the page size (1-100, default 100).
func (s *GetClosedStrategiesService) SetLimit(limit int) *GetClosedStrategiesService {
	s.params["limit"] = formatInt(limit)
	return s
}

// SetCursor continues from a previous page's NextCursor.
func (s *GetClosedStrategiesService) SetCursor(cursor string) *GetClosedStrategiesService {
	s.params["cursor"] = cursor
	return s
}

func (s *GetClosedStrategiesService) Do(ctx context.Context) (*ClosedStrategiesResult, error) {
	return request.Do[ClosedStrategiesResult](request.Post(ctx, s.c, "/0/private/GetClosedStrategies", s.params).WithSign())
}

// ClosedStrategiesResult is one page of terminal strategies.
type ClosedStrategiesResult struct {
	Closed     []StrategyInfo `json:"closed"`      // newest first (by closing time, or opening time with query_time_mode=open)
	NextCursor string         `json:"next_cursor"` // pass to SetCursor; empty on the last page
}
