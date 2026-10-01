package kraken

import (
	"context"
	"strings"
	"time"

	"github.com/UnipayFI/go-kraken/client"
	"github.com/UnipayFI/go-kraken/common"
	"github.com/UnipayFI/go-kraken/request"
	"github.com/shopspring/decimal"
)

// ===========================================================================
// 1. Get Account Balance -- POST /0/private/Balance
// ===========================================================================

// GetAccountBalanceService returns all cash balances, net of pending withdrawals,
// keyed by Kraken asset name.
type GetAccountBalanceService struct {
	c      *Client
	params map[string]string
}

func (c *Client) NewGetAccountBalanceService() *GetAccountBalanceService {
	return &GetAccountBalanceService{c: c, params: map[string]string{}}
}

// SetAccountID selects the wallet account to report on by its public id (see
// ListWalletAccountsService). Omit to use the authenticated user's default wallet.
func (s *GetAccountBalanceService) SetAccountID(accountID string) *GetAccountBalanceService {
	s.params["account_id"] = accountID
	return s
}

func (s *GetAccountBalanceService) Do(ctx context.Context) (map[string]decimal.Decimal, error) {
	resp, err := request.Do[map[string]decimal.Decimal](request.Post(ctx, s.c, "/0/private/Balance", s.params).WithSign())
	if err != nil {
		return nil, err
	}
	return *resp, nil
}

// ===========================================================================
// 2. Get Extended Balance -- POST /0/private/BalanceEx
// ===========================================================================

// GetExtendedBalanceService returns all cash balances with a breakdown of the
// amount held for open orders/positions, keyed by Kraken asset name.
type GetExtendedBalanceService struct {
	c      *Client
	params map[string]string
}

func (c *Client) NewGetExtendedBalanceService() *GetExtendedBalanceService {
	return &GetExtendedBalanceService{c: c, params: map[string]string{}}
}

// SetAccountID selects the wallet account to report on by its public id (see
// ListWalletAccountsService). Omit to use the authenticated user's default wallet.
func (s *GetExtendedBalanceService) SetAccountID(accountID string) *GetExtendedBalanceService {
	s.params["account_id"] = accountID
	return s
}

func (s *GetExtendedBalanceService) Do(ctx context.Context) (map[string]ExtendedBalance, error) {
	resp, err := request.Do[map[string]ExtendedBalance](request.Post(ctx, s.c, "/0/private/BalanceEx", s.params).WithSign())
	if err != nil {
		return nil, err
	}
	return *resp, nil
}

// ExtendedBalance is one asset's total balance and the portion held for orders.
type ExtendedBalance struct {
	Balance    decimal.Decimal `json:"balance"`     // total balance of the asset
	Credit     decimal.Decimal `json:"credit"`      // total credit (only with a credit line)
	CreditUsed decimal.Decimal `json:"credit_used"` // used credit (only with a credit line)
	HoldTrade  decimal.Decimal `json:"hold_trade"`  // balance held for open orders/positions
}

// ===========================================================================
// 3. Get Trade Balance -- POST /0/private/TradeBalance
// ===========================================================================

// GetTradeBalanceService returns a summary of collateral balances, margin
// position valuations, equity and margin level.
type GetTradeBalanceService struct {
	c      *Client
	params map[string]string
}

func (c *Client) NewGetTradeBalanceService() *GetTradeBalanceService {
	return &GetTradeBalanceService{c: c, params: map[string]string{}}
}

// SetAsset sets the base asset used to determine the balance (default ZUSD).
func (s *GetTradeBalanceService) SetAsset(asset string) *GetTradeBalanceService {
	s.params["asset"] = asset
	return s
}

// SetAccountID selects the wallet account to report on by its public id (see
// ListWalletAccountsService). Omit to use the authenticated user's default wallet.
func (s *GetTradeBalanceService) SetAccountID(accountID string) *GetTradeBalanceService {
	s.params["account_id"] = accountID
	return s
}

func (s *GetTradeBalanceService) Do(ctx context.Context) (*TradeBalance, error) {
	return request.Do[TradeBalance](request.Post(ctx, s.c, "/0/private/TradeBalance", s.params).WithSign())
}

// TradeBalance summarizes collateral, margin and equity.
type TradeBalance struct {
	EquivalentBalance   decimal.Decimal `json:"eb"`  // combined balance of all currencies
	TradeBalance        decimal.Decimal `json:"tb"`  // combined balance of all equity currencies
	MarginOpenPositions decimal.Decimal `json:"m"`   // margin amount of open positions
	UnrealizedNetPnL    decimal.Decimal `json:"n"`   // unrealized net profit/loss of open positions
	CostBasis           decimal.Decimal `json:"c"`   // cost basis of open positions
	FloatingValuation   decimal.Decimal `json:"v"`   // current floating valuation of open positions
	Equity              decimal.Decimal `json:"e"`   // trade balance + unrealized net profit/loss
	FreeMargin          decimal.Decimal `json:"mf"`  // equity - initial margin (max margin available)
	MarginFreeForOrders decimal.Decimal `json:"mfo"` // available margin usable for new orders
	MarginLevel         decimal.Decimal `json:"ml"`  // (equity / initial margin) * 100, when positions open
	UnexecutedValue     decimal.Decimal `json:"uv"`  // value of unfilled and partially filled orders
}

// ===========================================================================
// Shared order/trade/position/ledger types
// ===========================================================================

// OrderDescription is the human-readable description of an order's parameters.
type OrderDescription struct {
	Pair       string          `json:"pair"`      // asset pair
	Type       string          `json:"type"`      // buy or sell
	OrderType  string          `json:"ordertype"` // market, limit, stop-loss, take-profit, ...
	Price      decimal.Decimal `json:"price"`     // primary price
	Price2     decimal.Decimal `json:"price2"`    // secondary price
	Leverage   string          `json:"leverage"`  // amount of leverage (e.g. "none", "5:1")
	Order      string          `json:"order"`     // order description
	Close      string          `json:"close"`     // conditional close order description, if any
	AssetClass string          `json:"aclass"`    // asset class of the pair
}

// OrderInfo is the full state of an order, shared by OpenOrders, ClosedOrders
// and QueryOrders. Optional fields (closetm, reason, margin, trigger,
// sender_sub_id, trades, displayvol, link_id, reduce_only, ...) appear only in
// the relevant context.
type OrderInfo struct {
	RefID                  string           `json:"refid"`                // referral order tx id that created this order (nullable)
	UserRef                int64            `json:"userref"`              // optional client identifier (nullable)
	ClientOrderID          string           `json:"cl_ord_id"`            // optional alphanumeric client identifier (nullable)
	Status                 string           `json:"status"`               // pending, open, closed, canceled, expired
	Reason                 string           `json:"reason"`               // reason order was closed/canceled (nullable)
	OpenTime               time.Time        `json:"opentm,format:unix"`   // time order was placed
	StartTime              time.Time        `json:"starttm,format:unix"`  // order start time (0 if not set)
	ExpireTime             time.Time        `json:"expiretm,format:unix"` // order end time (0 if not set)
	CloseTime              time.Time        `json:"closetm,format:unix"`  // time order was closed (ClosedOrders/QueryOrders)
	Description            OrderDescription `json:"descr"`                // order description
	Volume                 decimal.Decimal  `json:"vol"`                  // order volume (base currency)
	VolumeExecuted         decimal.Decimal  `json:"vol_exec"`             // volume executed (base currency)
	Cost                   decimal.Decimal  `json:"cost"`                 // total cost (quote currency)
	Fee                    decimal.Decimal  `json:"fee"`                  // total fee (quote currency)
	Price                  decimal.Decimal  `json:"price"`                // average price (quote currency)
	StopPrice              decimal.Decimal  `json:"stopprice"`            // stop price (quote currency)
	LimitPrice             decimal.Decimal  `json:"limitprice"`           // triggered limit price (quote currency)
	DisplayVolume          decimal.Decimal  `json:"displayvol"`           // iceberg visible quantity (base currency)
	DisplayVolumeRemaining decimal.Decimal  `json:"displayvolremain"`     // iceberg visible quantity still in the book (base currency)
	ExternalOrderID        string           `json:"ext_ord_id"`           // external order id
	LinkID                 string           `json:"link_id"`              // original order id of an edited order
	ReduceOnly             bool             `json:"reduce_only"`          // reduce-only order (only present if true)
	Trigger                string           `json:"trigger"`              // price signal for stop/take-profit: last or index
	Margin                 bool             `json:"margin"`               // whether order is funded on margin
	Misc                   string           `json:"misc"`                 // comma-delimited misc info (stopped, touched, ...)
	SenderSubID            string           `json:"sender_sub_id"`        // underlying sub-account for STP (nullable)
	OrderFlags             string           `json:"oflags"`               // comma-delimited order flags (post, fcib, fciq, ...)
	TimeInForce            string           `json:"time_in_force"`        // gtc, ioc, gtd, fok
	Trades                 []string         `json:"trades"`               // related trade ids (if requested and available)
}

// PageCursor points at the next page of a cursor-paginated listing (requests
// sent with SetWithCursor(true)).
type PageCursor struct {
	Next string `json:"next"` // pass to SetCursor; empty when there are no more pages
}

// LedgerEntry is one ledger record.
type LedgerEntry struct {
	RefID       string          `json:"refid"`            // reference id
	Time        time.Time       `json:"time,format:unix"` // time of ledger entry
	Type        string          `json:"type"`             // deposit, withdrawal, trade, margin, rollover, ...
	SubType     string          `json:"subtype"`          // additional info on the type
	AssetClass  string          `json:"aclass"`           // asset class
	Asset       string          `json:"asset"`            // asset
	Amount      decimal.Decimal `json:"amount"`           // transaction amount (signed)
	Fee         decimal.Decimal `json:"fee"`              // transaction fee
	Balance     decimal.Decimal `json:"balance"`          // resulting balance
	AmountToken decimal.Decimal `json:"amount_token"`     // amount in the token's raw base units (QueryLedgers, tokenized assets only)
}

// ===========================================================================
// 4. Get Open Orders -- POST /0/private/OpenOrders
// ===========================================================================

// GetOpenOrdersService returns the account's open orders.
type GetOpenOrdersService struct {
	c      *Client
	params map[string]string
}

func (c *Client) NewGetOpenOrdersService() *GetOpenOrdersService {
	return &GetOpenOrdersService{c: c, params: map[string]string{}}
}

// SetTrades includes related trade ids in the output.
func (s *GetOpenOrdersService) SetTrades(trades bool) *GetOpenOrdersService {
	s.params["trades"] = formatBool(trades)
	return s
}

// SetUserRef filters by user reference id.
func (s *GetOpenOrdersService) SetUserRef(userRef int) *GetOpenOrdersService {
	s.params["userref"] = formatInt(userRef)
	return s
}

// SetClientOrderID filters by client order id.
func (s *GetOpenOrdersService) SetClientOrderID(clOrdID string) *GetOpenOrdersService {
	s.params["cl_ord_id"] = clOrdID
	return s
}

// SetConsolidateTaker consolidates trades by individual taker trades.
func (s *GetOpenOrdersService) SetConsolidateTaker(consolidate bool) *GetOpenOrdersService {
	s.params["consolidate_taker"] = formatBool(consolidate)
	return s
}

// SetWithCursor switches to cursor-based pagination: each page holds at most
// SetLimit orders (default 50) and the response's Cursor.Next is set while more
// remain.
func (s *GetOpenOrdersService) SetWithCursor(withCursor bool) *GetOpenOrdersService {
	s.params["with_cursor"] = formatBool(withCursor)
	return s
}

// SetCursor continues from a previous page's Cursor.Next (requires
// SetWithCursor(true)).
func (s *GetOpenOrdersService) SetCursor(cursor string) *GetOpenOrdersService {
	s.params["cursor"] = cursor
	return s
}

// SetLimit caps the number of orders returned: per page (max 100, default 50)
// with SetWithCursor(true), otherwise overall (max 1000; default all).
func (s *GetOpenOrdersService) SetLimit(limit int) *GetOpenOrdersService {
	s.params["limit"] = formatInt(limit)
	return s
}

// SetAccountID selects the wallet account to read orders from by its public id (see
// ListWalletAccountsService). Omit to use the authenticated user's default wallet.
func (s *GetOpenOrdersService) SetAccountID(accountID string) *GetOpenOrdersService {
	s.params["account_id"] = accountID
	return s
}

func (s *GetOpenOrdersService) Do(ctx context.Context) (*OpenOrdersResult, error) {
	return request.Do[OpenOrdersResult](request.Post(ctx, s.c, "/0/private/OpenOrders", s.params).WithSign())
}

// OpenOrdersResult holds the open-orders map keyed by order tx id.
type OpenOrdersResult struct {
	Open   map[string]OrderInfo `json:"open"`
	Cursor PageCursor           `json:"cursor"` // next page (with_cursor only)
}

// ===========================================================================
// 5. Get Closed Orders -- POST /0/private/ClosedOrders
// ===========================================================================

// GetClosedOrdersService returns the account's closed orders (most recent
// first), with pagination.
type GetClosedOrdersService struct {
	c      *Client
	params map[string]string
}

func (c *Client) NewGetClosedOrdersService() *GetClosedOrdersService {
	return &GetClosedOrdersService{c: c, params: map[string]string{}}
}

// SetTrades includes related trade ids in the output.
func (s *GetClosedOrdersService) SetTrades(trades bool) *GetClosedOrdersService {
	s.params["trades"] = formatBool(trades)
	return s
}

// SetUserRef filters by user reference id.
func (s *GetClosedOrdersService) SetUserRef(userRef int) *GetClosedOrdersService {
	s.params["userref"] = formatInt(userRef)
	return s
}

// SetClientOrderID filters by client order id.
func (s *GetClosedOrdersService) SetClientOrderID(clOrdID string) *GetClosedOrdersService {
	s.params["cl_ord_id"] = clOrdID
	return s
}

// SetStart sets the starting unix timestamp or order tx id (inclusive).
func (s *GetClosedOrdersService) SetStart(start string) *GetClosedOrdersService {
	s.params["start"] = start
	return s
}

// SetEnd sets the ending unix timestamp or order tx id (inclusive).
func (s *GetClosedOrdersService) SetEnd(end string) *GetClosedOrdersService {
	s.params["end"] = end
	return s
}

// SetOffset sets the result offset for pagination.
//
// DEPRECATED since 2026-09-30: use SetWithCursor and SetCursor instead (ofs
// must be absent or 0 with with_cursor).
func (s *GetClosedOrdersService) SetOffset(ofs int) *GetClosedOrdersService {
	s.params["ofs"] = formatInt(ofs)
	return s
}

// SetCloseTime selects which timestamp to use for searching (open, close, both).
func (s *GetClosedOrdersService) SetCloseTime(closeTime string) *GetClosedOrdersService {
	s.params["closetime"] = closeTime
	return s
}

// SetWithoutCount skips the total-count calculation to improve speed.
func (s *GetClosedOrdersService) SetWithoutCount(withoutCount bool) *GetClosedOrdersService {
	s.params["without_count"] = formatBool(withoutCount)
	return s
}

// SetWithCursor switches to cursor-based pagination: each page holds up to 50
// orders and the response's Cursor.Next is set while more remain.
func (s *GetClosedOrdersService) SetWithCursor(withCursor bool) *GetClosedOrdersService {
	s.params["with_cursor"] = formatBool(withCursor)
	return s
}

// SetCursor continues from a previous page's Cursor.Next (requires
// SetWithCursor(true)).
func (s *GetClosedOrdersService) SetCursor(cursor string) *GetClosedOrdersService {
	s.params["cursor"] = cursor
	return s
}

// SetAccountID selects the wallet account to read orders from by its public id (see
// ListWalletAccountsService). Omit to use the authenticated user's default wallet.
func (s *GetClosedOrdersService) SetAccountID(accountID string) *GetClosedOrdersService {
	s.params["account_id"] = accountID
	return s
}

func (s *GetClosedOrdersService) Do(ctx context.Context) (*ClosedOrdersResult, error) {
	return request.Do[ClosedOrdersResult](request.Post(ctx, s.c, "/0/private/ClosedOrders", s.params).WithSign())
}

// ClosedOrdersResult holds the closed-orders map plus the total count.
type ClosedOrdersResult struct {
	Closed map[string]OrderInfo `json:"closed"`
	Count  int                  `json:"count"`  // 0 with without_count or with_cursor
	Cursor PageCursor           `json:"cursor"` // next page (with_cursor only)
}

// ===========================================================================
// 6. Query Orders Info -- POST /0/private/QueryOrders
// ===========================================================================

// QueryOrdersService returns full information for specific orders by tx id.
type QueryOrdersService struct {
	c      *Client
	params map[string]string
}

// NewQueryOrdersService queries up to 50 orders by their tx ids.
func (c *Client) NewQueryOrdersService(txids ...string) *QueryOrdersService {
	return &QueryOrdersService{c: c, params: map[string]string{"txid": strings.Join(txids, ",")}}
}

// SetTrades includes related trade ids in the output.
func (s *QueryOrdersService) SetTrades(trades bool) *QueryOrdersService {
	s.params["trades"] = formatBool(trades)
	return s
}

// SetUserRef restricts results to the given user reference id.
func (s *QueryOrdersService) SetUserRef(userRef int) *QueryOrdersService {
	s.params["userref"] = formatInt(userRef)
	return s
}

// SetClientOrderID restricts results to the given client order id.
func (s *QueryOrdersService) SetClientOrderID(clOrdID string) *QueryOrdersService {
	s.params["cl_ord_id"] = clOrdID
	return s
}

// SetConsolidateTaker consolidates trades by individual taker trades.
func (s *QueryOrdersService) SetConsolidateTaker(consolidate bool) *QueryOrdersService {
	s.params["consolidate_taker"] = formatBool(consolidate)
	return s
}

// SetAccountID selects the wallet account to read orders from by its public id (see
// ListWalletAccountsService). Omit to use the authenticated user's default wallet.
func (s *QueryOrdersService) SetAccountID(accountID string) *QueryOrdersService {
	s.params["account_id"] = accountID
	return s
}

func (s *QueryOrdersService) Do(ctx context.Context) (map[string]OrderInfo, error) {
	resp, err := request.Do[map[string]OrderInfo](request.Post(ctx, s.c, "/0/private/QueryOrders", s.params).WithSign())
	if err != nil {
		return nil, err
	}
	return *resp, nil
}

// ===========================================================================
// 7. Get Order Amends -- POST /0/private/OrderAmends
// ===========================================================================

// GetOrderAmendsService returns the amendment history of a single order.
type GetOrderAmendsService struct {
	c      *Client
	params map[string]string
}

// NewGetOrderAmendsService queries amends for one Kraken order id.
func (c *Client) NewGetOrderAmendsService(orderID string) *GetOrderAmendsService {
	return &GetOrderAmendsService{c: c, params: map[string]string{"order_id": orderID}}
}

// SetAccountID selects the wallet account to read the order from by its public id (see
// ListWalletAccountsService). Omit to use the authenticated user's default wallet.
func (s *GetOrderAmendsService) SetAccountID(accountID string) *GetOrderAmendsService {
	s.params["account_id"] = accountID
	return s
}

func (s *GetOrderAmendsService) Do(ctx context.Context) (*OrderAmendsResult, error) {
	return request.Do[OrderAmendsResult](request.Post(ctx, s.c, "/0/private/OrderAmends", s.params).WithSign())
}

// OrderAmendsResult holds the amendment history of an order.
type OrderAmendsResult struct {
	Count  int          `json:"count"`  // total count incl. the original order
	Amends []OrderAmend `json:"amends"` // amendment records
}

// OrderAmend is one amendment record.
type OrderAmend struct {
	AmendID      string          `json:"amend_id"`                   // amendment identifier
	AmendType    string          `json:"amend_type"`                 // original, user, or restated
	OrderQty     decimal.Decimal `json:"order_qty"`                  // order quantity (base asset)
	DisplayQty   decimal.Decimal `json:"display_qty"`                // quantity shown in book for iceberg orders
	RemainingQty decimal.Decimal `json:"remaining_qty"`              // remaining un-traded quantity
	LimitPrice   decimal.Decimal `json:"limit_price"`                // limit price restriction
	TriggerPrice decimal.Decimal `json:"trigger_price"`              // trigger price on trigger order types
	Reason       string          `json:"reason"`                     // reason for this amend
	PostOnly     bool            `json:"post_only"`                  // whether restricted from taking liquidity
	Timestamp    time.Time       `json:"timestamp,format:unixmilli"` // amendment time (UNIX milliseconds, although the docs' examples show nanoseconds)
}

// ===========================================================================
// 8. Get Trades History -- POST /0/private/TradesHistory
// ===========================================================================

// TradeHistoryEntry is one trade execution, shared by TradesHistory and
// QueryTrades. Position-close fields (cprice, ccost, ...) appear only when the
// trade closed a margin position.
type TradeHistoryEntry struct {
	OrderTxID      string          `json:"ordertxid"`        // order responsible for the trade
	PositionTxID   string          `json:"postxid"`          // position responsible for the trade
	Pair           string          `json:"pair"`             // asset pair
	Time           time.Time       `json:"time,format:unix"` // time of trade
	Type           string          `json:"type"`             // buy or sell
	OrderType      string          `json:"ordertype"`        // order type
	Price          decimal.Decimal `json:"price"`            // average execution price (quote currency)
	Cost           decimal.Decimal `json:"cost"`             // total cost (quote currency)
	Fee            decimal.Decimal `json:"fee"`              // total fee (quote currency)
	Volume         decimal.Decimal `json:"vol"`              // volume (base currency)
	Margin         decimal.Decimal `json:"margin"`           // initial margin (quote currency)
	Leverage       decimal.Decimal `json:"leverage"`         // amount of leverage used
	Misc           string          `json:"misc"`             // comma-delimited misc info
	TradeID        int64           `json:"trade_id"`         // unique trade id
	Maker          bool            `json:"maker"`            // true if maker, false if taker
	AssetClass     string          `json:"aclass"`           // asset class of the traded pair
	ExternalExecID string          `json:"ext_exec_id"`      // external execution id (if any)
	TradeOrderType string          `json:"tradeordertype"`   // actual execution order type (may differ)
	PositionStatus string          `json:"posstatus"`        // position status (only if trade opened a position)
	ClosedPrice    decimal.Decimal `json:"cprice"`           // avg price of closed portion of position
	ClosedCost     decimal.Decimal `json:"ccost"`            // total cost of closed portion of position
	ClosedFee      decimal.Decimal `json:"cfee"`             // total fee of closed portion of position
	ClosedVolume   decimal.Decimal `json:"cvol"`             // total volume of closed portion of position
	ClosedMargin   decimal.Decimal `json:"cmargin"`          // total margin freed in closed portion
	Net            decimal.Decimal `json:"net"`              // net profit/loss of closed portion
	ClosingTrades  []string        `json:"trades"`           // list of closing trades for position
	Ledgers        []string        `json:"ledgers"`          // related ledger ids (if requested)
}

// GetTradesHistoryService returns the account's trade history with pagination.
type GetTradesHistoryService struct {
	c      *Client
	params map[string]string
}

func (c *Client) NewGetTradesHistoryService() *GetTradesHistoryService {
	return &GetTradesHistoryService{c: c, params: map[string]string{}}
}

// SetType filters by trade type (all, any position, closed position, closing
// position, no position).
func (s *GetTradesHistoryService) SetType(tradeType string) *GetTradesHistoryService {
	s.params["type"] = tradeType
	return s
}

// SetTrades includes trades related to a position in the output.
func (s *GetTradesHistoryService) SetTrades(trades bool) *GetTradesHistoryService {
	s.params["trades"] = formatBool(trades)
	return s
}

// SetStart sets the starting unix timestamp or trade tx id (exclusive).
func (s *GetTradesHistoryService) SetStart(start string) *GetTradesHistoryService {
	s.params["start"] = start
	return s
}

// SetEnd sets the ending unix timestamp or trade tx id (inclusive).
func (s *GetTradesHistoryService) SetEnd(end string) *GetTradesHistoryService {
	s.params["end"] = end
	return s
}

// SetOffset sets the result offset for pagination.
//
// DEPRECATED since 2026-09-30: use SetWithCursor and SetCursor instead (ofs
// must be absent or 0 with with_cursor).
func (s *GetTradesHistoryService) SetOffset(ofs int) *GetTradesHistoryService {
	s.params["ofs"] = formatInt(ofs)
	return s
}

// SetWithoutCount skips the count calculation to improve speed.
func (s *GetTradesHistoryService) SetWithoutCount(withoutCount bool) *GetTradesHistoryService {
	s.params["without_count"] = formatBool(withoutCount)
	return s
}

// SetConsolidateTaker consolidates trades by individual taker trades.
func (s *GetTradesHistoryService) SetConsolidateTaker(consolidate bool) *GetTradesHistoryService {
	s.params["consolidate_taker"] = formatBool(consolidate)
	return s
}

// SetLedgers includes related ledger ids for each trade.
func (s *GetTradesHistoryService) SetLedgers(ledgers bool) *GetTradesHistoryService {
	s.params["ledgers"] = formatBool(ledgers)
	return s
}

// SetAssetClass sets the asset class of the pair filter (default forex; also
// equity_pair, futures_contract, synthetic_pair, external_pair).
func (s *GetTradesHistoryService) SetAssetClass(aclass string) *GetTradesHistoryService {
	s.params["aclass"] = aclass
	return s
}

// SetPair filters results to a single trading pair.
func (s *GetTradesHistoryService) SetPair(pair string) *GetTradesHistoryService {
	s.params["pair"] = pair
	return s
}

// SetLimit sets the number of trades returned per page (1-100, default 50).
func (s *GetTradesHistoryService) SetLimit(limit int) *GetTradesHistoryService {
	s.params["limit"] = formatInt(limit)
	return s
}

// SetWithCursor switches to cursor-based pagination: each page holds up to
// SetLimit trades and the response's Cursor.Next is set while more remain.
func (s *GetTradesHistoryService) SetWithCursor(withCursor bool) *GetTradesHistoryService {
	s.params["with_cursor"] = formatBool(withCursor)
	return s
}

// SetCursor continues from a previous page's Cursor.Next (requires
// SetWithCursor(true)).
func (s *GetTradesHistoryService) SetCursor(cursor string) *GetTradesHistoryService {
	s.params["cursor"] = cursor
	return s
}

// SetAccountID selects the wallet account to read trades from by its public id (see
// ListWalletAccountsService). Omit to use the authenticated user's default wallet.
func (s *GetTradesHistoryService) SetAccountID(accountID string) *GetTradesHistoryService {
	s.params["account_id"] = accountID
	return s
}

func (s *GetTradesHistoryService) Do(ctx context.Context) (*TradesHistoryResult, error) {
	return request.Do[TradesHistoryResult](request.Post(ctx, s.c, "/0/private/TradesHistory", s.params).WithSign())
}

// TradesHistoryResult holds the trades map plus the total count.
type TradesHistoryResult struct {
	Trades map[string]TradeHistoryEntry `json:"trades"`
	Count  int                          `json:"count"`  // 0 with without_count or with_cursor
	Cursor PageCursor                   `json:"cursor"` // next page (with_cursor only)
}

// ===========================================================================
// 9. Query Trades Info -- POST /0/private/QueryTrades
// ===========================================================================

// QueryTradesService returns full information for specific trades by tx id.
type QueryTradesService struct {
	c      *Client
	params map[string]string
}

// NewQueryTradesService queries up to 20 trades by their tx ids.
func (c *Client) NewQueryTradesService(txids ...string) *QueryTradesService {
	return &QueryTradesService{c: c, params: map[string]string{"txid": strings.Join(txids, ",")}}
}

// SetTrades includes trades related to a position in the output.
func (s *QueryTradesService) SetTrades(trades bool) *QueryTradesService {
	s.params["trades"] = formatBool(trades)
	return s
}

// SetLedgers includes related ledger ids for each trade.
func (s *QueryTradesService) SetLedgers(ledgers bool) *QueryTradesService {
	s.params["ledgers"] = formatBool(ledgers)
	return s
}

// SetAccountID selects the wallet account to read trades from by its public id (see
// ListWalletAccountsService). Omit to use the authenticated user's default wallet.
func (s *QueryTradesService) SetAccountID(accountID string) *QueryTradesService {
	s.params["account_id"] = accountID
	return s
}

func (s *QueryTradesService) Do(ctx context.Context) (map[string]TradeHistoryEntry, error) {
	resp, err := request.Do[map[string]TradeHistoryEntry](request.Post(ctx, s.c, "/0/private/QueryTrades", s.params).WithSign())
	if err != nil {
		return nil, err
	}
	return *resp, nil
}

// ===========================================================================
// 10. Get Open Positions -- POST /0/private/OpenPositions
// ===========================================================================

// GetOpenPositionsService returns the account's open margin positions.
type GetOpenPositionsService struct {
	c      *Client
	params map[string]string
}

func (c *Client) NewGetOpenPositionsService() *GetOpenPositionsService {
	return &GetOpenPositionsService{c: c, params: map[string]string{}}
}

// SetTxID limits output to specific position tx ids (comma-separated).
func (s *GetOpenPositionsService) SetTxID(txids ...string) *GetOpenPositionsService {
	s.params["txid"] = strings.Join(txids, ",")
	return s
}

// SetDoCalcs includes profit/loss calculations (value and net).
func (s *GetOpenPositionsService) SetDoCalcs(doCalcs bool) *GetOpenPositionsService {
	s.params["docalcs"] = formatBool(doCalcs)
	return s
}

// SetConsolidation consolidates positions by market/pair (value: "market").
// The response is then a list that Do cannot decode; use DoConsolidated.
func (s *GetOpenPositionsService) SetConsolidation(consolidation string) *GetOpenPositionsService {
	s.params["consolidation"] = consolidation
	return s
}

// SetAccountID selects the wallet account to read positions from by its public id (see
// ListWalletAccountsService). Omit to use the authenticated user's default wallet.
func (s *GetOpenPositionsService) SetAccountID(accountID string) *GetOpenPositionsService {
	s.params["account_id"] = accountID
	return s
}

func (s *GetOpenPositionsService) Do(ctx context.Context) (map[string]PositionInfo, error) {
	resp, err := request.Do[map[string]PositionInfo](request.Post(ctx, s.c, "/0/private/OpenPositions", s.params).WithSign())
	if err != nil {
		return nil, err
	}
	return *resp, nil
}

// DoConsolidated requests consolidation=market and returns the positions
// consolidated by pair.
func (s *GetOpenPositionsService) DoConsolidated(ctx context.Context) ([]ConsolidatedPosition, error) {
	s.params["consolidation"] = "market"
	resp, err := request.Do[[]ConsolidatedPosition](request.Post(ctx, s.c, "/0/private/OpenPositions", s.params).WithSign())
	if err != nil {
		return nil, err
	}
	return *resp, nil
}

// PositionInfo is one open margin position.
type PositionInfo struct {
	OrderTxID      string          `json:"ordertxid"`              // order id responsible for the position
	AssetClass     string          `json:"class"`                  // asset class of the position
	PositionStatus string          `json:"posstatus"`              // position status (open)
	Pair           string          `json:"pair"`                   // asset pair
	Time           time.Time       `json:"time,format:unix"`       // time the position was opened
	Type           string          `json:"type"`                   // buy or sell (direction)
	OrderType      string          `json:"ordertype"`              // order type used to open
	Cost           decimal.Decimal `json:"cost"`                   // opening cost (quote currency)
	Fee            decimal.Decimal `json:"fee"`                    // opening fee (quote currency)
	Volume         decimal.Decimal `json:"vol"`                    // opening size (base currency)
	VolumeClosed   decimal.Decimal `json:"vol_closed"`             // quantity closed (base currency)
	Margin         decimal.Decimal `json:"margin"`                 // initial margin consumed (quote currency)
	Value          decimal.Decimal `json:"value"`                  // current value (if docalcs)
	Net            decimal.Decimal `json:"net"`                    // unrealized P&L of remaining position (if docalcs)
	Terms          string          `json:"terms"`                  // funding cost and term of position
	RolloverTime   time.Time       `json:"rollovertm,format:unix"` // timestamp of next margin rollover fee
	Misc           string          `json:"misc"`                   // comma-delimited additional info
	OrderFlags     string          `json:"oflags"`                 // comma-delimited opening order flags
}

// ConsolidatedPosition is the open margin positions of one pair, summed
// (consolidation=market).
type ConsolidatedPosition struct {
	Pair         string          `json:"pair"`       // asset pair
	AssetClass   string          `json:"class"`      // asset class of the positions
	Positions    decimal.Decimal `json:"positions"`  // number of positions consolidated into this entry
	Type         string          `json:"type"`       // buy or sell (direction)
	Leverage     string          `json:"leverage"`   // average leverage, or "n/a"
	Cost         decimal.Decimal `json:"cost"`       // opening cost (quote currency)
	Fee          decimal.Decimal `json:"fee"`        // opening fee (quote currency)
	Volume       decimal.Decimal `json:"vol"`        // opening size (base currency)
	VolumeClosed decimal.Decimal `json:"vol_closed"` // quantity closed (base currency)
	Margin       decimal.Decimal `json:"margin"`     // initial margin consumed (quote currency)
	Value        decimal.Decimal `json:"value"`      // current value of remaining positions (if docalcs)
	Net          decimal.Decimal `json:"net"`        // unrealized P&L of remaining positions (if docalcs)
}

// ===========================================================================
// 11. Get Ledgers Info -- POST /0/private/Ledgers
// ===========================================================================

// GetLedgersService returns the account's ledger entries with pagination.
type GetLedgersService struct {
	c      *Client
	params map[string]string
}

func (c *Client) NewGetLedgersService() *GetLedgersService {
	return &GetLedgersService{c: c, params: map[string]string{}}
}

// SetAccountID selects the wallet account to read ledgers from by its public id
// (see ListWalletAccountsService). Omit to use the authenticated user's default
// wallet.
func (s *GetLedgersService) SetAccountID(accountID string) *GetLedgersService {
	s.params["account_id"] = accountID
	return s
}

// SetAsset filters by asset(s) (comma-separated, default all).
func (s *GetLedgersService) SetAsset(assets ...string) *GetLedgersService {
	s.params["asset"] = strings.Join(assets, ",")
	return s
}

// SetAssetClass filters by asset class (default currency).
//
// DEPRECATED since 2026-09-30: Kraken now wants the list-of-{asset, aclass}
// form of asset instead, which needs a JSON request body this SDK does not send.
func (s *GetLedgersService) SetAssetClass(aclass string) *GetLedgersService {
	s.params["aclass"] = aclass
	return s
}

// SetType filters by ledger entry type (all, deposit, withdrawal, trade,
// margin, rollover, credit, transfer, settled, staking, sale, ...).
func (s *GetLedgersService) SetType(ledgerType string) *GetLedgersService {
	s.params["type"] = ledgerType
	return s
}

// SetStart sets the starting unix timestamp or ledger id (exclusive).
func (s *GetLedgersService) SetStart(start string) *GetLedgersService {
	s.params["start"] = start
	return s
}

// SetEnd sets the ending unix timestamp or ledger id (inclusive).
func (s *GetLedgersService) SetEnd(end string) *GetLedgersService {
	s.params["end"] = end
	return s
}

// SetOffset sets the result offset for pagination.
//
// DEPRECATED since 2026-09-30: page with SetStart and SetEnd instead.
func (s *GetLedgersService) SetOffset(ofs int) *GetLedgersService {
	s.params["ofs"] = formatInt(ofs)
	return s
}

// SetWithoutCount skips the count calculation to improve speed.
func (s *GetLedgersService) SetWithoutCount(withoutCount bool) *GetLedgersService {
	s.params["without_count"] = formatBool(withoutCount)
	return s
}

func (s *GetLedgersService) Do(ctx context.Context) (*LedgersResult, error) {
	return request.Do[LedgersResult](request.Post(ctx, s.c, "/0/private/Ledgers", s.params).WithSign())
}

// LedgersResult holds the ledger map plus the total count.
type LedgersResult struct {
	Ledger map[string]LedgerEntry `json:"ledger"`
	Count  int                    `json:"count"`
}

// ===========================================================================
// 12. Query Ledgers -- POST /0/private/QueryLedgers
// ===========================================================================

// QueryLedgersService returns full information for specific ledger entries by id.
type QueryLedgersService struct {
	c      *Client
	params map[string]string
}

// NewQueryLedgersService queries up to 20 ledger entries by their ids.
func (c *Client) NewQueryLedgersService(ids ...string) *QueryLedgersService {
	return &QueryLedgersService{c: c, params: map[string]string{"id": strings.Join(ids, ",")}}
}

// SetTrades includes related trade info in the output.
//
// DEPRECATED since 2026-09-30: Kraken says not to rely on this parameter.
func (s *QueryLedgersService) SetTrades(trades bool) *QueryLedgersService {
	s.params["trades"] = formatBool(trades)
	return s
}

// SetAccountID selects the wallet account to read ledgers from by its public id (see
// ListWalletAccountsService). Omit to use the authenticated user's default wallet.
func (s *QueryLedgersService) SetAccountID(accountID string) *QueryLedgersService {
	s.params["account_id"] = accountID
	return s
}

func (s *QueryLedgersService) Do(ctx context.Context) (map[string]LedgerEntry, error) {
	resp, err := request.Do[map[string]LedgerEntry](request.Post(ctx, s.c, "/0/private/QueryLedgers", s.params).WithSign())
	if err != nil {
		return nil, err
	}
	return *resp, nil
}

// ===========================================================================
// 13. Get Trade Volume -- POST /0/private/TradeVolume
// ===========================================================================

// GetTradeVolumeService returns the account's 30-day USD trade volume and the
// resulting fee schedule.
type GetTradeVolumeService struct {
	c      *Client
	params map[string]string
}

func (c *Client) NewGetTradeVolumeService() *GetTradeVolumeService {
	return &GetTradeVolumeService{c: c, params: map[string]string{}}
}

// SetPair returns fee info for the given pair(s) (comma-separated).
func (s *GetTradeVolumeService) SetPair(pairs ...string) *GetTradeVolumeService {
	s.params["pair"] = strings.Join(pairs, ",")
	return s
}

// SetFeeSchedule additionally returns the full fee schedule of every trading
// pair in the Schedules field.
func (s *GetTradeVolumeService) SetFeeSchedule(feeSchedule bool) *GetTradeVolumeService {
	s.params["fee_schedule"] = formatBool(feeSchedule)
	return s
}

func (s *GetTradeVolumeService) Do(ctx context.Context) (*TradeVolume, error) {
	return request.Do[TradeVolume](request.Post(ctx, s.c, "/0/private/TradeVolume", s.params).WithSign())
}

// TradeVolume is the account's fee-volume summary.
type TradeVolume struct {
	Currency   string             `json:"currency"`    // volume currency
	Volume     decimal.Decimal    `json:"volume"`      // current 30-day discount volume
	Fees       map[string]FeeInfo `json:"fees"`        // taker fee schedule per pair (if pair given)
	FeesMaker  map[string]FeeInfo `json:"fees_maker"`  // maker fee schedule per pair (if pair given)
	AssetClass string             `json:"asset_class"` // asset class of the volume
	Inputs     TradeVolumeInputs  `json:"inputs"`      // inputs used to compute the discount volume
	Schedules  []FeeSchedule      `json:"schedules"`   // full fee schedule per pair (only with SetFeeSchedule(true))
}

// FeeInfo is the fee schedule for one pair.
type FeeInfo struct {
	Fee               decimal.Decimal `json:"fee"`               // current fee (percent)
	MinFee            decimal.Decimal `json:"minfee"`            // minimum fee (percent, if not fixed)
	MaxFee            decimal.Decimal `json:"maxfee"`            // maximum fee (percent, if not fixed)
	NextFee           decimal.Decimal `json:"nextfee"`           // next-tier fee (percent, if not fixed)
	TierVolume        decimal.Decimal `json:"tiervolume"`        // volume level of current tier
	NextVolume        decimal.Decimal `json:"nextvolume"`        // volume level of next tier
	TierFuturesVolume decimal.Decimal `json:"tierfuturesvolume"` // current futures-volume tier threshold, when visible in the schedule
	NextFuturesVolume decimal.Decimal `json:"nextfuturesvolume"` // next futures-volume tier threshold, when visible in the schedule
	VolumeOffset      decimal.Decimal `json:"volumeoffset"`      // volume adjustment added to the trade volume when computing fees
}

// FeeSchedule is the full fee schedule of one trading pair.
type FeeSchedule struct {
	Pair  string            `json:"pair"`  // trading pair the schedule applies to
	Class string            `json:"class"` // currency | forex | equity | equity_pair | nft | derivatives | tokenized_asset | futures_contract | volume
	Tiers []FeeScheduleTier `json:"tiers"` // tiers of the schedule, cheapest tier last
}

// FeeScheduleTier is one tier of a FeeSchedule.
type FeeScheduleTier struct {
	MakerFee            decimal.Decimal `json:"maker_fee"`              // maker fee (percent) at this tier
	TakerFee            decimal.Decimal `json:"taker_fee"`              // taker fee (percent) at this tier
	MinSpotVolume       decimal.Decimal `json:"min_spot_volume"`        // minimum spot volume for this tier (if applicable)
	MinFuturesVolume    decimal.Decimal `json:"min_futures_volume"`     // minimum 30-day futures volume for this tier (if applicable)
	MinAssetsOnPlatform decimal.Decimal `json:"min_assets_on_platform"` // minimum assets on platform for this tier (if applicable)
	Active              bool            `json:"active"`                 // whether this is the account's current tier
}

// TradeVolumeInputs are the volume inputs used to compute the fee tier.
type TradeVolumeInputs struct {
	DomainAssetsOnPlatform decimal.Decimal `json:"domain_assets_on_platform"`
	DomainFuturesVolume30d decimal.Decimal `json:"domain_futures_volume_30d"`
	DomainSpotVolume30d    decimal.Decimal `json:"domain_spot_volume_30d"`
}

// ===========================================================================
// 14. Request Export Report -- POST /0/private/AddExport
// ===========================================================================

// ReportType is the kind of data export to generate.
type ReportType string

const (
	ReportTypeTrades  ReportType = "trades"
	ReportTypeLedgers ReportType = "ledgers"
)

// RequestExportReportService requests generation of a trades/ledgers export
// report and returns its id.
type RequestExportReportService struct {
	c      *Client
	params map[string]string
}

// NewRequestExportReportService starts an export of the given report type with a
// human-readable description.
func (c *Client) NewRequestExportReportService(report ReportType, description string) *RequestExportReportService {
	return &RequestExportReportService{c: c, params: map[string]string{
		"report":      string(report),
		"description": description,
	}}
}

// SetFormat sets the file format (CSV or TSV; default CSV).
func (s *RequestExportReportService) SetFormat(format string) *RequestExportReportService {
	s.params["format"] = format
	return s
}

// SetFields sets the comma-delimited fields to include (default all).
func (s *RequestExportReportService) SetFields(fields string) *RequestExportReportService {
	s.params["fields"] = fields
	return s
}

// SetStartTime sets the report data start time.
func (s *RequestExportReportService) SetStartTime(t time.Time) *RequestExportReportService {
	s.params["starttm"] = formatUnix(t)
	return s
}

// SetEndTime sets the report data end time.
func (s *RequestExportReportService) SetEndTime(t time.Time) *RequestExportReportService {
	s.params["endtm"] = formatUnix(t)
	return s
}

func (s *RequestExportReportService) Do(ctx context.Context) (*ExportReportRef, error) {
	return request.Do[ExportReportRef](request.Post(ctx, s.c, "/0/private/AddExport", s.params).WithSign())
}

// ExportReportRef references a newly created export report.
type ExportReportRef struct {
	ID string `json:"id"` // unique report identifier
}

// ===========================================================================
// 15. Get Export Report Status -- POST /0/private/ExportStatus
// ===========================================================================

// GetExportReportStatusService lists the status of the account's export reports
// of a given type.
type GetExportReportStatusService struct {
	c      *Client
	params map[string]string
}

// NewGetExportReportStatusService lists reports of the given type (trades/ledgers).
func (c *Client) NewGetExportReportStatusService(report ReportType) *GetExportReportStatusService {
	return &GetExportReportStatusService{c: c, params: map[string]string{"report": string(report)}}
}

func (s *GetExportReportStatusService) Do(ctx context.Context) ([]ExportReport, error) {
	resp, err := request.Do[[]ExportReport](request.Post(ctx, s.c, "/0/private/ExportStatus", s.params).WithSign())
	if err != nil {
		return nil, err
	}
	return *resp, nil
}

// ExportReport is the status of one export report.
type ExportReport struct {
	ID            string    `json:"id"`                      // unique report identifier
	Description   string    `json:"descr"`                   // report description/name
	Format        string    `json:"format"`                  // file format (CSV/TSV)
	Report        string    `json:"report"`                  // report type (trades/ledgers)
	SubType       string    `json:"subtype"`                 // report subtype
	Status        string    `json:"status"`                  // Queued, Processing, Processed
	Error         string    `json:"error"`                   // error code if failed
	Flags         string    `json:"flags"`                   // legacy flag field (deprecated)
	Fields        string    `json:"fields"`                  // fields included
	CreatedTime   time.Time `json:"createdtm,format:unix"`   // time the report was requested
	ExpireTime    time.Time `json:"expiretm,format:unix"`    // expiration timestamp (deprecated)
	StartTime     time.Time `json:"starttm,format:unix"`     // time processing began
	CompletedTime time.Time `json:"completedtm,format:unix"` // time processing finished
	DataStartTime time.Time `json:"datastarttm,format:unix"` // report data period start
	DataEndTime   time.Time `json:"dataendtm,format:unix"`   // report data period end
	AssetClass    string    `json:"aclass"`                  // asset class (deprecated)
	Asset         string    `json:"asset"`                   // assets included
	Assets        string    `json:"assets"`                  // assets included (current key)
	AssetClasses  []string  `json:"asset_classes"`           // asset classes covered
	EndTime       time.Time `json:"endtm,format:unix"`       // report end time
	Delete        bool      `json:"delete"`                  // whether marked for deletion
}

// ===========================================================================
// 16. Retrieve Data Export -- POST /0/private/RetrieveExport
// ===========================================================================

// RetrieveDataExportService downloads a processed export report. The response is
// a binary ZIP archive, returned as raw bytes (not the usual JSON envelope).
type RetrieveDataExportService struct {
	c      *Client
	params map[string]string
}

// NewRetrieveDataExportService retrieves the report with the given id.
func (c *Client) NewRetrieveDataExportService(id string) *RetrieveDataExportService {
	return &RetrieveDataExportService{c: c, params: map[string]string{"id": id}}
}

// Do returns the raw ZIP archive bytes. If the API instead returns a JSON error
// envelope (e.g. an unknown id), that error is surfaced.
func (s *RetrieveDataExportService) Do(ctx context.Context) ([]byte, error) {
	body, err := request.DoRaw(request.Post(ctx, s.c, "/0/private/RetrieveExport", s.params).WithSign())
	if err != nil {
		return nil, err
	}
	// On success the body is a binary ZIP ("PK..."); on failure it is a JSON
	// error envelope. Detect and surface the latter.
	if len(body) > 0 && body[0] == '{' {
		var env struct {
			Error []string `json:"error"`
		}
		if uerr := common.JSONUnmarshal(body, &env); uerr == nil && len(env.Error) > 0 {
			return nil, &client.APIError{Errors: env.Error}
		}
	}
	return body, nil
}

// ===========================================================================
// 17. Delete Export Report -- POST /0/private/RemoveExport
// ===========================================================================

// DeleteExportReportType is the operation to perform on an export report.
type DeleteExportReportType string

const (
	DeleteExportReportTypeCancel DeleteExportReportType = "cancel"
	DeleteExportReportTypeDelete DeleteExportReportType = "delete"
)

// DeleteExportReportService cancels a queued report or deletes a processed one.
type DeleteExportReportService struct {
	c      *Client
	params map[string]string
}

// NewDeleteExportReportService removes the report with the given id. Use
// DeleteExportReportTypeCancel for a queued report and ...Delete for a
// processed one.
func (c *Client) NewDeleteExportReportService(id string, opType DeleteExportReportType) *DeleteExportReportService {
	return &DeleteExportReportService{c: c, params: map[string]string{
		"id":   id,
		"type": string(opType),
	}}
}

func (s *DeleteExportReportService) Do(ctx context.Context) (*DeleteExportResult, error) {
	return request.Do[DeleteExportResult](request.Post(ctx, s.c, "/0/private/RemoveExport", s.params).WithSign())
}

// DeleteExportResult reports the outcome of a delete/cancel operation.
type DeleteExportResult struct {
	Delete bool `json:"delete"` // whether deletion was successful
	Cancel bool `json:"cancel"` // whether cancellation was successful
}

// ===========================================================================
// 18. Get Credit Lines -- POST /0/private/CreditLines
// ===========================================================================

// GetCreditLinesService returns the account's margin credit lines: per-asset
// collateral details and a margin-limits monitor. Available to eligible
// (typically VIP) accounts.
type GetCreditLinesService struct {
	c      *Client
	params map[string]string
}

func (c *Client) NewGetCreditLinesService() *GetCreditLinesService {
	return &GetCreditLinesService{c: c, params: map[string]string{}}
}

// SetAccountID selects the wallet account to report on by its public id (see
// ListWalletAccountsService). Omit to use the authenticated user's default wallet.
func (s *GetCreditLinesService) SetAccountID(accountID string) *GetCreditLinesService {
	s.params["account_id"] = accountID
	return s
}

func (s *GetCreditLinesService) Do(ctx context.Context) (*CreditLines, error) {
	return request.Do[CreditLines](request.Post(ctx, s.c, "/0/private/CreditLines", s.params).WithSign())
}

// CreditLines summarizes margin collateral and limits.
type CreditLines struct {
	AssetDetails  map[string]CreditAssetDetail `json:"asset_details"`  // per-asset collateral details
	LimitsMonitor CreditLimitsMonitor          `json:"limits_monitor"` // aggregate margin limits
}

// CreditAssetDetail is the collateral detail for one asset. The credit and fee
// fields are present only for assets with a credit line.
type CreditAssetDetail struct {
	Balance         decimal.Decimal `json:"balance"`          // asset balance
	HoldTrade       decimal.Decimal `json:"hold_trade"`       // balance held for open orders
	Credit          decimal.Decimal `json:"credit"`           // credit limit for the asset
	CreditUsed      decimal.Decimal `json:"credit_used"`      // credit currently used
	CollateralValue decimal.Decimal `json:"collateral_value"` // collateral value factor, 0 to 1 (eligible collateral only)
	RolloverFees    decimal.Decimal `json:"rollover_fees"`    // rate on drawn credit, percent per rollover period
	ReserveFees     decimal.Decimal `json:"reserve_fees"`     // rate on undrawn credit, percent per rollover period
}

// CreditLimitsMonitor is the aggregate margin-limits snapshot.
type CreditLimitsMonitor struct {
	DebtToEquity            decimal.Decimal `json:"debt_to_equity"`             // debt-to-equity ratio
	EquityUSD               decimal.Decimal `json:"equity_usd"`                 // total equity (USD)
	OngoingBalance          decimal.Decimal `json:"ongoing_balance"`            // total collateral / total credit (USD)
	TotalCollateralValueUSD decimal.Decimal `json:"total_collateral_value_usd"` // total collateral value (USD)
	TotalCreditUSD          decimal.Decimal `json:"total_credit_usd"`           // total credit line (USD)
	TotalCreditUsedUSD      decimal.Decimal `json:"total_credit_used_usd"`      // credit currently used (USD)
}

// ===========================================================================
// 19. Get API Key Info -- POST /0/private/GetApiKeyInfo
// ===========================================================================

// GetApiKeyInfoService returns metadata about the API key used to sign the
// request: its name, permissions, IP allowlist and nonce settings.
type GetApiKeyInfoService struct {
	c *Client
}

func (c *Client) NewGetApiKeyInfoService() *GetApiKeyInfoService {
	return &GetApiKeyInfoService{c: c}
}

func (s *GetApiKeyInfoService) Do(ctx context.Context) (*APIKeyInfo, error) {
	return request.Do[APIKeyInfo](request.Post(ctx, s.c, "/0/private/GetApiKeyInfo").WithSign())
}

// APIKeyInfo describes the signing API key.
type APIKeyInfo struct {
	APIKey       string    `json:"api_key"`                   // the public API key
	APIKeyName   string    `json:"api_key_name"`              // user-assigned key name
	IBAN         string    `json:"iban"`                      // the account IBAN
	Permissions  []string  `json:"permissions"`               // granted permissions
	IPAllowlist  []string  `json:"ip_allowlist"`              // allowed source IPs (empty = any)
	Nonce        string    `json:"nonce"`                     // last nonce seen for the key
	NonceWindow  string    `json:"nonce_window"`              // configured nonce window (seconds)
	CreatedTime  time.Time `json:"created_time,format:unix"`  // when the key was created
	ModifiedTime time.Time `json:"modified_time,format:unix"` // when the key was last modified
	LastUsed     time.Time `json:"last_used,format:unix"`     // when the key was last used
	QueryFrom    time.Time `json:"query_from,format:unix"`    // start of the key's allowed query window
	QueryTo      time.Time `json:"query_to,format:unix"`      // end of the key's allowed query window
	ValidUntil   time.Time `json:"valid_until,format:unix"`   // key expiry (zero = no expiry)
}

// ===========================================================================
// 20. List Wallet Accounts -- POST /0/private/ListWalletAccounts
// ===========================================================================

// ListWalletAccountsService returns the wallet accounts linked to the
// authenticated user. The returned account ids are what the account-data
// services (Balance, BalanceEx, TradeBalance, orders, trades, positions, ledgers,
// CreditLines) accept via SetAccountID.
type ListWalletAccountsService struct {
	c *Client
}

func (c *Client) NewListWalletAccountsService() *ListWalletAccountsService {
	return &ListWalletAccountsService{c: c}
}

func (s *ListWalletAccountsService) Do(ctx context.Context) (*WalletAccounts, error) {
	return request.Do[WalletAccounts](request.Post(ctx, s.c, "/0/private/ListWalletAccounts").WithSign())
}

// WalletAccounts is a page of wallet accounts. Kraken returns a forward cursor
// but does not (yet) document a request parameter to consume it.
type WalletAccounts struct {
	Accounts []WalletAccount `json:"accounts"` // the user's wallet accounts
	Cursor   WalletCursor    `json:"cursor"`   // pagination cursor
}

// WalletCursor points at the next page of wallet accounts.
type WalletCursor struct {
	Next string `json:"next"` // opaque cursor, empty when there are no more pages
}

// WalletAccount is one wallet account linked to the user.
type WalletAccount struct {
	AccountID string             `json:"account_id"` // public wallet account id
	Flags     WalletAccountFlags `json:"flags"`      // wallet lifecycle flags
	Status    WalletStatus       `json:"status"`     // wallet lifecycle status
	Type      WalletType         `json:"type"`       // account type
	Name      string             `json:"name"`       // display name (empty when unnamed)
}

// WalletAccountFlags describes how a wallet was created and whether it is live.
type WalletAccountFlags struct {
	UserDefined bool `json:"user_defined"` // whether the wallet was created by the user
	Active      bool `json:"active"`       // whether the wallet is currently active
}

// WalletStatus is a wallet account's lifecycle status.
type WalletStatus string

const (
	WalletStatusActive   WalletStatus = "active"
	WalletStatusDisabled WalletStatus = "disabled"
	WalletStatusClosed   WalletStatus = "closed"
	WalletStatusUnknown  WalletStatus = "unknown"
)

// WalletType is the kind of wallet account.
type WalletType string

const (
	WalletTypeMain      WalletType = "main"
	WalletTypeSpot      WalletType = "spot"
	WalletTypePay       WalletType = "pay"
	WalletTypePropPaper WalletType = "prop_paper"
	WalletTypePropReal  WalletType = "prop_real"
	WalletTypeUnknown   WalletType = "unknown"
)
