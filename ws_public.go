package kraken

import (
	"context"
	"time"

	"github.com/UnipayFI/go-kraken/request"
	"github.com/shopspring/decimal"
)

// ===========================================================================
// ticker channel
// ===========================================================================

// SubscribeTickerService subscribes to the public "ticker" channel for one or
// more symbols (e.g. "BTC/USD").
type SubscribeTickerService struct {
	c      *WebSocketClient
	params map[string]any
}

func (c *WebSocketClient) NewSubscribeTickerService(symbols ...string) *SubscribeTickerService {
	return &SubscribeTickerService{c: c, params: map[string]any{"symbol": symbols}}
}

// SetEventTrigger selects the update trigger: "trades" (default, push on every
// trade) or "bbo" (push only when the best bid/offer changes).
func (s *SubscribeTickerService) SetEventTrigger(trigger string) *SubscribeTickerService {
	s.params["event_trigger"] = trigger
	return s
}

// SetSnapshot controls whether an initial snapshot is sent on subscribe.
func (s *SubscribeTickerService) SetSnapshot(snapshot bool) *SubscribeTickerService {
	s.params["snapshot"] = snapshot
	return s
}

func (s *SubscribeTickerService) Do(ctx context.Context, cb func(*WsPush[[]WsTicker], error)) (chan<- struct{}, <-chan struct{}, error) {
	return request.Subscribe[[]WsTicker](ctx, s.c, "ticker", s.params, false, cb)
}

// WsTicker is one element of the "ticker" channel data array.
type WsTicker struct {
	Symbol    string          `json:"symbol"`     // currency pair
	Bid       decimal.Decimal `json:"bid"`        // best bid price
	BidQty    decimal.Decimal `json:"bid_qty"`    // best bid quantity (base)
	Ask       decimal.Decimal `json:"ask"`        // best ask price
	AskQty    decimal.Decimal `json:"ask_qty"`    // best ask quantity (base)
	Last      decimal.Decimal `json:"last"`       // last traded price
	Volume    decimal.Decimal `json:"volume"`     // 24h base-currency volume
	VWAP      decimal.Decimal `json:"vwap"`       // 24h volume-weighted average price
	Low       decimal.Decimal `json:"low"`        // 24h low
	High      decimal.Decimal `json:"high"`       // 24h high
	Change    decimal.Decimal `json:"change"`     // 24h price change (quote currency)
	ChangePct decimal.Decimal `json:"change_pct"` // 24h price change (percent)
	Timestamp time.Time       `json:"timestamp"`  // event time
}

// ===========================================================================
// book channel
// ===========================================================================

// SubscribeBookService subscribes to the public "book" (level 2) channel.
type SubscribeBookService struct {
	c      *WebSocketClient
	params map[string]any
}

func (c *WebSocketClient) NewSubscribeBookService(symbols ...string) *SubscribeBookService {
	return &SubscribeBookService{c: c, params: map[string]any{"symbol": symbols}}
}

// SetDepth sets the book depth per side (10, 25, 100, 500, 1000; default 10).
func (s *SubscribeBookService) SetDepth(depth int) *SubscribeBookService {
	s.params["depth"] = depth
	return s
}

// SetSnapshot controls whether an initial snapshot is sent on subscribe.
func (s *SubscribeBookService) SetSnapshot(snapshot bool) *SubscribeBookService {
	s.params["snapshot"] = snapshot
	return s
}

func (s *SubscribeBookService) Do(ctx context.Context, cb func(*WsPush[[]WsBook], error)) (chan<- struct{}, <-chan struct{}, error) {
	return request.Subscribe[[]WsBook](ctx, s.c, "book", s.params, false, cb)
}

// WsBook is one element of the "book" channel data array. A snapshot carries the
// full depth; an update carries only changed levels (a level with qty 0 is
// removed).
type WsBook struct {
	Symbol    string        `json:"symbol"`    // currency pair
	Bids      []WsBookLevel `json:"bids"`      // changed/snapshot bid levels
	Asks      []WsBookLevel `json:"asks"`      // changed/snapshot ask levels
	Checksum  int64         `json:"checksum"`  // CRC32 of the top-10 book (uint32)
	Timestamp time.Time     `json:"timestamp"` // event time
}

// WsBookLevel is one price level in the level-2 book.
type WsBookLevel struct {
	Price decimal.Decimal `json:"price"` // price level
	Qty   decimal.Decimal `json:"qty"`   // aggregated quantity (0 = level removed)
}

// ===========================================================================
// ohlc channel
// ===========================================================================

// SubscribeOHLCService subscribes to the public "ohlc" candlestick channel.
type SubscribeOHLCService struct {
	c      *WebSocketClient
	params map[string]any
}

// NewSubscribeOHLCService subscribes for the given symbols at the given interval
// in minutes (1, 5, 15, 30, 60, 240, 1440, 10080, 21600).
func (c *WebSocketClient) NewSubscribeOHLCService(interval int, symbols ...string) *SubscribeOHLCService {
	return &SubscribeOHLCService{c: c, params: map[string]any{"symbol": symbols, "interval": interval}}
}

// SetSnapshot controls whether an initial snapshot is sent on subscribe.
func (s *SubscribeOHLCService) SetSnapshot(snapshot bool) *SubscribeOHLCService {
	s.params["snapshot"] = snapshot
	return s
}

func (s *SubscribeOHLCService) Do(ctx context.Context, cb func(*WsPush[[]WsOHLC], error)) (chan<- struct{}, <-chan struct{}, error) {
	return request.Subscribe[[]WsOHLC](ctx, s.c, "ohlc", s.params, false, cb)
}

// WsOHLC is one element of the "ohlc" channel data array.
type WsOHLC struct {
	Symbol        string          `json:"symbol"`         // currency pair
	Open          decimal.Decimal `json:"open"`           // open price
	High          decimal.Decimal `json:"high"`           // high price
	Low           decimal.Decimal `json:"low"`            // low price
	Close         decimal.Decimal `json:"close"`          // close price
	VWAP          decimal.Decimal `json:"vwap"`           // volume-weighted average price
	Trades        int64           `json:"trades"`         // number of trades
	Volume        decimal.Decimal `json:"volume"`         // traded volume (base)
	IntervalBegin time.Time       `json:"interval_begin"` // candle start time
	Interval      int             `json:"interval"`       // candle interval (minutes)
	Timestamp     time.Time       `json:"timestamp"`      // event time
}

// ===========================================================================
// trade channel
// ===========================================================================

// SubscribeTradeService subscribes to the public "trade" channel.
type SubscribeTradeService struct {
	c      *WebSocketClient
	params map[string]any
}

func (c *WebSocketClient) NewSubscribeTradeService(symbols ...string) *SubscribeTradeService {
	return &SubscribeTradeService{c: c, params: map[string]any{"symbol": symbols}}
}

// SetSnapshot controls whether an initial snapshot is sent on subscribe.
func (s *SubscribeTradeService) SetSnapshot(snapshot bool) *SubscribeTradeService {
	s.params["snapshot"] = snapshot
	return s
}

func (s *SubscribeTradeService) Do(ctx context.Context, cb func(*WsPush[[]WsTrade], error)) (chan<- struct{}, <-chan struct{}, error) {
	return request.Subscribe[[]WsTrade](ctx, s.c, "trade", s.params, false, cb)
}

// WsTrade is one element of the "trade" channel data array.
type WsTrade struct {
	Symbol    string          `json:"symbol"`    // currency pair
	Side      string          `json:"side"`      // buy or sell
	Qty       decimal.Decimal `json:"qty"`       // executed quantity (base)
	Price     decimal.Decimal `json:"price"`     // execution price
	OrderType string          `json:"ord_type"`  // market or limit
	TradeID   int64           `json:"trade_id"`  // trade id
	Timestamp time.Time       `json:"timestamp"` // execution time
}

// ===========================================================================
// instrument channel
// ===========================================================================

// SubscribeInstrumentService subscribes to the public "instrument" channel,
// which streams reference data for all assets and tradable pairs.
type SubscribeInstrumentService struct {
	c      *WebSocketClient
	params map[string]any
}

func (c *WebSocketClient) NewSubscribeInstrumentService() *SubscribeInstrumentService {
	return &SubscribeInstrumentService{c: c, params: map[string]any{}}
}

// SetSnapshot controls whether an initial snapshot is sent on subscribe.
func (s *SubscribeInstrumentService) SetSnapshot(snapshot bool) *SubscribeInstrumentService {
	s.params["snapshot"] = snapshot
	return s
}

func (s *SubscribeInstrumentService) Do(ctx context.Context, cb func(*WsPush[WsInstrument], error)) (chan<- struct{}, <-chan struct{}, error) {
	return request.Subscribe[WsInstrument](ctx, s.c, "instrument", s.params, false, cb)
}

// WsInstrument is the "instrument" channel data object (assets and pairs).
type WsInstrument struct {
	Assets []WsInstrumentAsset `json:"assets"` // asset reference data
	Pairs  []WsInstrumentPair  `json:"pairs"`  // pair reference data
}

// WsInstrumentAsset is reference data for one asset.
type WsInstrumentAsset struct {
	ID               string          `json:"id"`                // asset id
	Status           string          `json:"status"`            // asset status
	Precision        int             `json:"precision"`         // record-keeping precision
	PrecisionDisplay int             `json:"precision_display"` // display precision
	Borrowable       bool            `json:"borrowable"`        // whether the asset can be borrowed
	CollateralValue  decimal.Decimal `json:"collateral_value"`  // collateral valuation factor
	MarginRate       decimal.Decimal `json:"margin_rate"`       // margin rate (if applicable)
	Multiplier       decimal.Decimal `json:"multiplier"`        // unit multiplier
	Class            string          `json:"class"`             // asset class
}

// WsInstrumentPair is reference data for one tradable pair.
type WsInstrumentPair struct {
	Symbol                  string          `json:"symbol"`                     // pair symbol
	Base                    string          `json:"base"`                       // base asset
	Quote                   string          `json:"quote"`                      // quote asset
	Status                  string          `json:"status"`                     // pair status
	QtyPrecision            int             `json:"qty_precision"`              // quantity precision
	QtyIncrement            decimal.Decimal `json:"qty_increment"`              // minimum quantity increment
	QtyMin                  decimal.Decimal `json:"qty_min"`                    // minimum order quantity
	PricePrecision          int             `json:"price_precision"`            // price precision
	PriceIncrement          decimal.Decimal `json:"price_increment"`            // minimum price increment
	CostPrecision           int             `json:"cost_precision"`             // cost precision
	CostMin                 decimal.Decimal `json:"cost_min"`                   // minimum order cost
	Marginable              bool            `json:"marginable"`                 // whether margin trading is allowed
	MarginInitial           decimal.Decimal `json:"margin_initial"`             // initial margin (if marginable)
	PositionLimitLong       int             `json:"position_limit_long"`        // long position limit
	PositionLimitShort      int             `json:"position_limit_short"`       // short position limit
	HasIndex                bool            `json:"has_index"`                  // whether an index price exists
	WSDisplayPricePrecision int             `json:"ws_display_price_precision"` // websocket display price precision
	TickSize                decimal.Decimal `json:"tick_size"`                  // price tick size
}
