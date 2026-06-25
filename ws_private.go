package kraken

import (
	"context"
	"time"

	"github.com/UnipayFI/go-kraken/request"
	"github.com/shopspring/decimal"
)

// ===========================================================================
// executions channel (private)
// ===========================================================================

// SubscribeExecutionsService subscribes to the private "executions" channel,
// which streams order lifecycle events and trade fills.
type SubscribeExecutionsService struct {
	c      *WebSocketClient
	params map[string]any
}

func (c *WebSocketClient) NewSubscribeExecutionsService() *SubscribeExecutionsService {
	return &SubscribeExecutionsService{c: c, params: map[string]any{}}
}

// SetSnapTrades includes a snapshot of recent trades on subscribe.
func (s *SubscribeExecutionsService) SetSnapTrades(snap bool) *SubscribeExecutionsService {
	s.params["snap_trades"] = snap
	return s
}

// SetSnapOrders includes a snapshot of open orders on subscribe.
func (s *SubscribeExecutionsService) SetSnapOrders(snap bool) *SubscribeExecutionsService {
	s.params["snap_orders"] = snap
	return s
}

// SetOrderStatus controls whether full order-status detail is included.
func (s *SubscribeExecutionsService) SetOrderStatus(enabled bool) *SubscribeExecutionsService {
	s.params["order_status"] = enabled
	return s
}

// SetRateCounter includes API rate-limit counter info in updates.
func (s *SubscribeExecutionsService) SetRateCounter(enabled bool) *SubscribeExecutionsService {
	s.params["ratecounter"] = enabled
	return s
}

func (s *SubscribeExecutionsService) Do(ctx context.Context, cb func(*WsPush[[]WsExecution], error)) (chan<- struct{}, <-chan struct{}, error) {
	return request.Subscribe[[]WsExecution](ctx, s.c, "executions", s.params, true, cb)
}

// WsExecution is one element of the "executions" channel data array: an order or
// fill event. Fields are populated according to exec_type; many are present only
// for the relevant event.
type WsExecution struct {
	OrderID       string           `json:"order_id"`       // Kraken order id
	OrderUserRef  int64            `json:"order_userref"`  // client numeric reference
	ClientOrderID string           `json:"cl_ord_id"`      // client order id
	ExecID        string           `json:"exec_id"`        // execution id
	TradeID       int64            `json:"trade_id"`       // trade id (fills)
	ExecType      string           `json:"exec_type"`      // new, filled, canceled, expired, trade, ...
	OrderStatus   string           `json:"order_status"`   // pending, open, closed, canceled, expired
	OrderType     string           `json:"order_type"`     // limit, market, ...
	Symbol        string           `json:"symbol"`         // currency pair
	Side          string           `json:"side"`           // buy or sell
	TimeInForce   string           `json:"time_in_force"`  // gtc, ioc, gtd
	OrderQty      decimal.Decimal  `json:"order_qty"`      // order quantity (base)
	CumQty        decimal.Decimal  `json:"cum_qty"`        // cumulative filled quantity
	LastQty       decimal.Decimal  `json:"last_qty"`       // quantity of the last fill
	DisplayQty    decimal.Decimal  `json:"display_qty"`    // iceberg display quantity
	LimitPrice    decimal.Decimal  `json:"limit_price"`    // limit price
	StopPrice     decimal.Decimal  `json:"stop_price"`     // stop/trigger price
	AvgPrice      decimal.Decimal  `json:"avg_price"`      // average fill price
	LastPrice     decimal.Decimal  `json:"last_price"`     // last fill price
	Cost          decimal.Decimal  `json:"cost"`           // cost of the last fill
	CumCost       decimal.Decimal  `json:"cum_cost"`       // cumulative cost
	FeeUSDEquiv   decimal.Decimal  `json:"fee_usd_equiv"`  // fee in USD equivalent
	FeeCcyPref    string           `json:"fee_ccy_pref"`   // fee currency preference
	Fees          []WsExecutionFee `json:"fees"`           // fees charged
	PostOnly      bool             `json:"post_only"`      // post-only flag
	ReduceOnly    bool             `json:"reduce_only"`    // reduce-only flag
	Margin        bool             `json:"margin"`         // funded on margin
	Liquidated    bool             `json:"liquidated"`     // resulted from a liquidation
	Amended       bool             `json:"amended"`        // order was amended
	LiquidityInd  string           `json:"liquidity_ind"`  // maker (m) or taker (t)
	Timestamp     time.Time        `json:"timestamp"`      // event time
	EffectiveTime time.Time        `json:"effective_time"` // scheduled start time
	ExpireTime    time.Time        `json:"expire_time"`    // expiry time
	Reason        string           `json:"reason"`         // status reason
	CancelReason  string           `json:"cancel_reason"`  // cancellation reason
	SenderSubID   string           `json:"sender_sub_id"`  // STP sub-account id
	Triggers      *WsExecTriggers  `json:"triggers"`       // trigger details (conditional orders)
}

// WsExecutionFee is one fee charged on an execution.
type WsExecutionFee struct {
	Asset string          `json:"asset"` // fee asset
	Qty   decimal.Decimal `json:"qty"`   // fee amount
}

// WsExecTriggers describes the trigger of a conditional order.
type WsExecTriggers struct {
	Reference   string          `json:"reference"`    // last or index
	Price       decimal.Decimal `json:"price"`        // trigger price
	PriceType   string          `json:"price_type"`   // static or pct
	ActualPrice decimal.Decimal `json:"actual_price"` // resolved trigger price
	PeakPrice   decimal.Decimal `json:"peak_price"`   // trailing peak price
	LastPrice   decimal.Decimal `json:"last_price"`   // last reference price
	Status      string          `json:"status"`       // untriggered or triggered
	Timestamp   time.Time       `json:"timestamp"`    // trigger time
}

// ===========================================================================
// balances channel (private)
// ===========================================================================

// SubscribeBalancesService subscribes to the private "balances" channel, which
// streams account balance snapshots and ledger-driven updates.
type SubscribeBalancesService struct {
	c      *WebSocketClient
	params map[string]any
}

func (c *WebSocketClient) NewSubscribeBalancesService() *SubscribeBalancesService {
	return &SubscribeBalancesService{c: c, params: map[string]any{}}
}

// SetSnapshot controls whether an initial balance snapshot is sent on subscribe.
func (s *SubscribeBalancesService) SetSnapshot(snapshot bool) *SubscribeBalancesService {
	s.params["snapshot"] = snapshot
	return s
}

func (s *SubscribeBalancesService) Do(ctx context.Context, cb func(*WsPush[[]WsBalance], error)) (chan<- struct{}, <-chan struct{}, error) {
	return request.Subscribe[[]WsBalance](ctx, s.c, "balances", s.params, true, cb)
}

// WsBalance is one element of the "balances" channel data array. Snapshot
// elements carry asset/balance/wallets; update elements additionally carry the
// driving ledger entry (amount, fee, ledger_id, ...).
type WsBalance struct {
	Asset      string            `json:"asset"`       // asset name
	AssetClass string            `json:"asset_class"` // asset class
	Balance    decimal.Decimal   `json:"balance"`     // total balance
	Wallets    []WsBalanceWallet `json:"wallets"`     // per-wallet breakdown (snapshot)
	// Update-only ledger fields:
	Amount     decimal.Decimal `json:"amount"`      // ledger amount (signed)
	Fee        decimal.Decimal `json:"fee"`         // ledger fee
	LedgerID   string          `json:"ledger_id"`   // ledger entry id
	RefID      string          `json:"ref_id"`      // reference id
	Timestamp  time.Time       `json:"timestamp"`   // ledger time
	Type       string          `json:"type"`        // ledger type
	SubType    string          `json:"subtype"`     // ledger subtype
	Category   string          `json:"category"`    // ledger category
	WalletType string          `json:"wallet_type"` // wallet type
	WalletID   string          `json:"wallet_id"`   // wallet id
}

// WsBalanceWallet is one wallet's balance within a balances snapshot.
type WsBalanceWallet struct {
	Type    string          `json:"type"`    // wallet type (spot, earn, ...)
	ID      string          `json:"id"`      // wallet id
	Balance decimal.Decimal `json:"balance"` // wallet balance
}

// ===========================================================================
// level3 channel (private order book)
// ===========================================================================

// SubscribeLevel3Service subscribes to the private "level3" (per-order) order
// book channel. It requires authentication.
type SubscribeLevel3Service struct {
	c      *WebSocketClient
	params map[string]any
}

func (c *WebSocketClient) NewSubscribeLevel3Service(symbols ...string) *SubscribeLevel3Service {
	return &SubscribeLevel3Service{c: c, params: map[string]any{"symbol": symbols}}
}

// SetDepth sets the number of orders per side (10, 100, 1000; default 10).
func (s *SubscribeLevel3Service) SetDepth(depth int) *SubscribeLevel3Service {
	s.params["depth"] = depth
	return s
}

// SetSnapshot controls whether an initial snapshot is sent on subscribe.
func (s *SubscribeLevel3Service) SetSnapshot(snapshot bool) *SubscribeLevel3Service {
	s.params["snapshot"] = snapshot
	return s
}

func (s *SubscribeLevel3Service) Do(ctx context.Context, cb func(*WsPush[[]WsLevel3], error)) (chan<- struct{}, <-chan struct{}, error) {
	return request.Subscribe[[]WsLevel3](ctx, s.c, "level3", s.params, true, cb)
}

// WsLevel3 is one element of the "level3" channel data array.
type WsLevel3 struct {
	Symbol   string          `json:"symbol"`   // currency pair
	Checksum int64           `json:"checksum"` // CRC32 of the book (uint32)
	Bids     []WsLevel3Order `json:"bids"`     // individual bid orders
	Asks     []WsLevel3Order `json:"asks"`     // individual ask orders
}

// WsLevel3Order is one resting order in the level-3 book.
type WsLevel3Order struct {
	OrderID    string          `json:"order_id"`    // order id
	LimitPrice decimal.Decimal `json:"limit_price"` // limit price
	OrderQty   decimal.Decimal `json:"order_qty"`   // remaining quantity
	Timestamp  time.Time       `json:"timestamp"`   // order timestamp
	Event      string          `json:"event"`       // add, modify, delete
}
