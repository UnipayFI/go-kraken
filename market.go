package kraken

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/UnipayFI/go-kraken/common"
	"github.com/UnipayFI/go-kraken/request"
	"github.com/go-json-experiment/json/jsontext"
	"github.com/shopspring/decimal"
)

// ---------------------------------------------------------------------------
// Positional-array (tuple) helpers
//
// Kraken returns candles, order-book levels, trades, spreads, fee tiers and the
// ticker sub-fields as fixed-position JSON arrays of mixed string/number types.
// decodeColumns / encodeColumns convert between such an array and a set of named
// destination pointers, decoding each element through the global codec so that
// decimal.Decimal (string-or-number) and time.Time (unix-seconds) fields parse
// uniformly.
// ---------------------------------------------------------------------------

func decodeColumns(data []byte, want int, dst ...any) error {
	var row []jsontext.Value
	if err := common.JSONUnmarshal(data, &row); err != nil {
		return err
	}
	if len(row) != want {
		return fmt.Errorf("kraken: tuple has %d columns, want %d", len(row), want)
	}
	for i, d := range dst {
		if err := common.JSONUnmarshal(row[i], d); err != nil {
			return fmt.Errorf("kraken: tuple column %d: %w", i, err)
		}
	}
	return nil
}

func encodeColumns(cols ...any) ([]byte, error) {
	return common.JSONMarshal(cols)
}

// splitResultLast separates Kraken's "{<pair>: <payload>, \"last\": <cursor>}"
// envelope into the single pair key, its raw payload, and the raw "last" cursor.
func splitResultLast(data []byte) (pair string, payload, last jsontext.Value, err error) {
	var obj map[string]jsontext.Value
	if err = common.JSONUnmarshal(data, &obj); err != nil {
		return "", nil, nil, err
	}
	for k, v := range obj {
		if k == "last" {
			last = v
			continue
		}
		pair = k
		payload = v
	}
	return pair, payload, last, nil
}

// ===========================================================================
// 1. Get Server Time -- GET /0/public/Time
// ===========================================================================

// GetServerTimeService returns Kraken's current server time. Useful as an
// unauthenticated connectivity/latency probe.
type GetServerTimeService struct {
	c *Client
}

func (c *Client) NewGetServerTimeService() *GetServerTimeService {
	return &GetServerTimeService{c: c}
}

func (s *GetServerTimeService) Do(ctx context.Context) (*ServerTime, error) {
	return request.Do[ServerTime](request.Get(ctx, s.c, "/0/public/Time"))
}

// ServerTime is the GET /0/public/Time payload.
type ServerTime struct {
	UnixTime time.Time `json:"unixtime"` // server time as unix seconds
	RFC1123  string    `json:"rfc1123"`  // server time as an RFC 1123 string
}

// ===========================================================================
// 2. Get System Status -- GET /0/public/SystemStatus
// ===========================================================================

// GetSystemStatusService returns the current trading-system status (online,
// maintenance, cancel_only, post_only).
type GetSystemStatusService struct {
	c *Client
}

func (c *Client) NewGetSystemStatusService() *GetSystemStatusService {
	return &GetSystemStatusService{c: c}
}

func (s *GetSystemStatusService) Do(ctx context.Context) (*SystemStatus, error) {
	return request.Do[SystemStatus](request.Get(ctx, s.c, "/0/public/SystemStatus"))
}

// SystemStatus is the GET /0/public/SystemStatus payload.
type SystemStatus struct {
	Status    string    `json:"status"`    // online | maintenance | cancel_only | post_only
	Timestamp time.Time `json:"timestamp"` // RFC3339 time the status was last updated
}

// ===========================================================================
// 3. Get Asset Info -- GET /0/public/Assets
// ===========================================================================

// GetAssetInfoService returns information about the assets available for
// trading and funding, keyed by Kraken asset name (e.g. "XXBT", "ZUSD").
type GetAssetInfoService struct {
	c      *Client
	params map[string]string
}

func (c *Client) NewGetAssetInfoService() *GetAssetInfoService {
	return &GetAssetInfoService{c: c, params: map[string]string{}}
}

// SetAsset filters to specific assets (comma-separated, e.g. "XBT,ETH").
func (s *GetAssetInfoService) SetAsset(assets ...string) *GetAssetInfoService {
	s.params["asset"] = strings.Join(assets, ",")
	return s
}

// SetAssetClass filters by asset class (default "currency").
func (s *GetAssetInfoService) SetAssetClass(aclass string) *GetAssetInfoService {
	s.params["aclass"] = aclass
	return s
}

func (s *GetAssetInfoService) Do(ctx context.Context) (map[string]Asset, error) {
	resp, err := request.Do[map[string]Asset](request.Get(ctx, s.c, "/0/public/Assets", s.params))
	if err != nil {
		return nil, err
	}
	return *resp, nil
}

// Asset describes one tradable/fundable asset.
type Asset struct {
	AssetClass      string          `json:"aclass"`           // asset class, usually "currency"
	AltName         string          `json:"altname"`          // alternate (common) name, e.g. "XBT"
	Decimals        int             `json:"decimals"`         // scaling decimal places for record keeping
	DisplayDecimals int             `json:"display_decimals"` // scaling decimal places for display
	CollateralValue decimal.Decimal `json:"collateral_value"` // valuation as margin collateral (if applicable)
	MarginRate      decimal.Decimal `json:"margin_rate"`      // margin rate (if applicable)
	Status          string          `json:"status"`           // asset status: enabled, deposit_only, withdrawal_only, funding_temporarily_disabled
}

// ===========================================================================
// 4. Get Tradable Asset Pairs -- GET /0/public/AssetPairs
// ===========================================================================

// AssetPairsInfo selects which subset of fields AssetPairs returns.
type AssetPairsInfo string

const (
	AssetPairsInfoAll      AssetPairsInfo = "info"
	AssetPairsInfoLeverage AssetPairsInfo = "leverage"
	AssetPairsInfoFees     AssetPairsInfo = "fees"
	AssetPairsInfoMargin   AssetPairsInfo = "margin"
)

// GetTradableAssetPairsService returns tradable asset pair info, keyed by
// Kraken pair name (e.g. "XXBTZUSD").
type GetTradableAssetPairsService struct {
	c      *Client
	params map[string]string
}

func (c *Client) NewGetTradableAssetPairsService() *GetTradableAssetPairsService {
	return &GetTradableAssetPairsService{c: c, params: map[string]string{}}
}

// SetPair filters to specific pairs (comma-separated, e.g. "XBTUSD,ETHUSD").
func (s *GetTradableAssetPairsService) SetPair(pairs ...string) *GetTradableAssetPairsService {
	s.params["pair"] = strings.Join(pairs, ",")
	return s
}

// SetInfo selects the field subset to return (default AssetPairsInfoAll).
func (s *GetTradableAssetPairsService) SetInfo(info AssetPairsInfo) *GetTradableAssetPairsService {
	s.params["info"] = string(info)
	return s
}

// SetCountryCode filters to pairs tradable from the given ISO 3166-1 alpha-2
// (optionally region-suffixed) country code.
func (s *GetTradableAssetPairsService) SetCountryCode(code string) *GetTradableAssetPairsService {
	s.params["country_code"] = code
	return s
}

func (s *GetTradableAssetPairsService) Do(ctx context.Context) (map[string]AssetPair, error) {
	resp, err := request.Do[map[string]AssetPair](request.Get(ctx, s.c, "/0/public/AssetPairs", s.params))
	if err != nil {
		return nil, err
	}
	return *resp, nil
}

// AssetPair describes one tradable asset pair.
type AssetPair struct {
	AltName            string          `json:"altname"`              // alternate pair name
	WSName             string          `json:"wsname"`               // WebSocket pair name (if available)
	AssetClassBase     string          `json:"aclass_base"`          // asset class of base component
	Base               string          `json:"base"`                 // asset id of base component
	AssetClassQuote    string          `json:"aclass_quote"`         // asset class of quote component
	Quote              string          `json:"quote"`                // asset id of quote component
	Lot                string          `json:"lot"`                  // volume lot size
	CostDecimals       int             `json:"cost_decimals"`        // scaling decimals for cost
	PairDecimals       int             `json:"pair_decimals"`        // scaling decimals for pair price
	LotDecimals        int             `json:"lot_decimals"`         // scaling decimals for volume
	LotMultiplier      int             `json:"lot_multiplier"`       // amount to multiply lot volume by to get currency volume
	LeverageBuy        []int           `json:"leverage_buy"`         // array of leverage amounts available when buying
	LeverageSell       []int           `json:"leverage_sell"`        // array of leverage amounts available when selling
	Fees               []FeeTier       `json:"fees"`                 // taker fee schedule [volume, percent]
	FeesMaker          []FeeTier       `json:"fees_maker"`           // maker fee schedule [volume, percent]
	FeeVolumeCurrency  string          `json:"fee_volume_currency"`  // volume discount currency
	MarginCall         int             `json:"margin_call"`          // margin call level
	MarginStop         int             `json:"margin_stop"`          // stop-out / liquidation margin level
	OrderMin           decimal.Decimal `json:"ordermin"`             // minimum order size (in base currency)
	CostMin            decimal.Decimal `json:"costmin"`              // minimum order cost (in quote currency)
	TickSize           decimal.Decimal `json:"tick_size"`            // minimum price increment
	Status             string          `json:"status"`               // online | cancel_only | post_only | limit_only | reduce_only
	LongPositionLimit  int             `json:"long_position_limit"`  // maximum long margin position size (in base currency)
	ShortPositionLimit int             `json:"short_position_limit"` // maximum short margin position size (in base currency)
	ExecutionVenue     string          `json:"execution_venue"`      // execution venue, e.g. "international"
}

// FeeTier is one [volume, percent] row of a fee schedule.
type FeeTier struct {
	Volume  decimal.Decimal // 30-day volume threshold (quote currency)
	Percent decimal.Decimal // fee percentage at or above that volume
}

func (f *FeeTier) UnmarshalJSON(data []byte) error {
	return decodeColumns(data, 2, &f.Volume, &f.Percent)
}

func (f FeeTier) MarshalJSON() ([]byte, error) {
	return encodeColumns(f.Volume, f.Percent)
}

// ===========================================================================
// 5. Get Ticker Information -- GET /0/public/Ticker
// ===========================================================================

// GetTickerService returns 24h ticker data, keyed by Kraken pair name. Note the
// values are calculated within the last 24 hours.
type GetTickerService struct {
	c      *Client
	params map[string]string
}

func (c *Client) NewGetTickerService() *GetTickerService {
	return &GetTickerService{c: c, params: map[string]string{}}
}

// SetPair filters to specific pairs (comma-separated). When omitted, ticker for
// all pairs is returned.
func (s *GetTickerService) SetPair(pairs ...string) *GetTickerService {
	s.params["pair"] = strings.Join(pairs, ",")
	return s
}

func (s *GetTickerService) Do(ctx context.Context) (map[string]Ticker, error) {
	resp, err := request.Do[map[string]Ticker](request.Get(ctx, s.c, "/0/public/Ticker", s.params))
	if err != nil {
		return nil, err
	}
	return *resp, nil
}

// Ticker is one pair's 24h ticker snapshot.
type Ticker struct {
	Ask       TickerLevel       `json:"a"` // ask [price, wholeLotVolume, lotVolume]
	Bid       TickerLevel       `json:"b"` // bid [price, wholeLotVolume, lotVolume]
	Last      TickerLastTrade   `json:"c"` // last trade closed [price, lotVolume]
	Volume    TickerWindow      `json:"v"` // volume [today, last24Hours]
	VWAP      TickerWindow      `json:"p"` // volume weighted average price [today, last24Hours]
	Trades    TickerCountWindow `json:"t"` // number of trades [today, last24Hours]
	Low       TickerWindow      `json:"l"` // low [today, last24Hours]
	High      TickerWindow      `json:"h"` // high [today, last24Hours]
	OpenPrice decimal.Decimal   `json:"o"` // today's opening price
}

// TickerLevel is an ask/bid level: [price, wholeLotVolume, lotVolume].
type TickerLevel struct {
	Price          decimal.Decimal
	WholeLotVolume decimal.Decimal
	LotVolume      decimal.Decimal
}

func (t *TickerLevel) UnmarshalJSON(data []byte) error {
	return decodeColumns(data, 3, &t.Price, &t.WholeLotVolume, &t.LotVolume)
}

func (t TickerLevel) MarshalJSON() ([]byte, error) {
	return encodeColumns(t.Price, t.WholeLotVolume, t.LotVolume)
}

// TickerLastTrade is the last-trade-closed level: [price, lotVolume].
type TickerLastTrade struct {
	Price     decimal.Decimal
	LotVolume decimal.Decimal
}

func (t *TickerLastTrade) UnmarshalJSON(data []byte) error {
	return decodeColumns(data, 2, &t.Price, &t.LotVolume)
}

func (t TickerLastTrade) MarshalJSON() ([]byte, error) {
	return encodeColumns(t.Price, t.LotVolume)
}

// TickerWindow holds a [today, last24Hours] pair of decimal values.
type TickerWindow struct {
	Today       decimal.Decimal
	Last24Hours decimal.Decimal
}

func (t *TickerWindow) UnmarshalJSON(data []byte) error {
	return decodeColumns(data, 2, &t.Today, &t.Last24Hours)
}

func (t TickerWindow) MarshalJSON() ([]byte, error) {
	return encodeColumns(t.Today, t.Last24Hours)
}

// TickerCountWindow holds a [today, last24Hours] pair of trade counts.
type TickerCountWindow struct {
	Today       int64
	Last24Hours int64
}

func (t *TickerCountWindow) UnmarshalJSON(data []byte) error {
	return decodeColumns(data, 2, &t.Today, &t.Last24Hours)
}

func (t TickerCountWindow) MarshalJSON() ([]byte, error) {
	return encodeColumns(t.Today, t.Last24Hours)
}

// ===========================================================================
// 6. Get OHLC Data -- GET /0/public/OHLC
// ===========================================================================

// Interval is an OHLC candle interval in minutes.
type Interval int

const (
	Interval1m  Interval = 1
	Interval5m  Interval = 5
	Interval15m Interval = 15
	Interval30m Interval = 30
	Interval1h  Interval = 60
	Interval4h  Interval = 240
	Interval1d  Interval = 1440
	Interval1w  Interval = 10080
	Interval15d Interval = 21600
)

// GetOHLCDataService returns OHLC candle data for a pair, plus a cursor (Last)
// to fetch incremental updates via SetSince.
type GetOHLCDataService struct {
	c      *Client
	params map[string]string
}

func (c *Client) NewGetOHLCDataService(pair string) *GetOHLCDataService {
	return &GetOHLCDataService{c: c, params: map[string]string{"pair": pair}}
}

// SetInterval sets the candle interval (default Interval1m).
func (s *GetOHLCDataService) SetInterval(interval Interval) *GetOHLCDataService {
	s.params["interval"] = strconv.Itoa(int(interval))
	return s
}

// SetSince returns only committed candles at or after t (use a prior result's
// Last for incremental polling).
func (s *GetOHLCDataService) SetSince(t time.Time) *GetOHLCDataService {
	s.params["since"] = strconv.FormatInt(t.Unix(), 10)
	return s
}

func (s *GetOHLCDataService) Do(ctx context.Context) (*OHLCResult, error) {
	return request.Do[OHLCResult](request.Get(ctx, s.c, "/0/public/OHLC", s.params))
}

// OHLCResult holds the candles for one pair plus the pagination cursor.
type OHLCResult struct {
	Pair    string    // Kraken pair name the candles belong to
	Candles []Candle  // committed candles, oldest first
	Last    time.Time // cursor: id of the last candle, pass to SetSince to poll
}

func (r *OHLCResult) UnmarshalJSON(data []byte) error {
	pair, payload, last, err := splitResultLast(data)
	if err != nil {
		return err
	}
	r.Pair = pair
	if len(last) > 0 {
		if err := common.JSONUnmarshal(last, &r.Last); err != nil {
			return err
		}
	}
	if len(payload) > 0 {
		return common.JSONUnmarshal(payload, &r.Candles)
	}
	return nil
}

func (r OHLCResult) MarshalJSON() ([]byte, error) {
	return common.JSONMarshal(map[string]any{r.Pair: r.Candles, "last": r.Last})
}

// Candle is one OHLC row: [time, open, high, low, close, vwap, volume, count].
type Candle struct {
	Time   time.Time       // candle start time
	Open   decimal.Decimal // opening price
	High   decimal.Decimal // highest price
	Low    decimal.Decimal // lowest price
	Close  decimal.Decimal // closing price
	VWAP   decimal.Decimal // volume weighted average price
	Volume decimal.Decimal // traded volume
	Count  int64           // number of trades
}

func (k *Candle) UnmarshalJSON(data []byte) error {
	return decodeColumns(data, 8, &k.Time, &k.Open, &k.High, &k.Low, &k.Close, &k.VWAP, &k.Volume, &k.Count)
}

func (k Candle) MarshalJSON() ([]byte, error) {
	return encodeColumns(k.Time, k.Open, k.High, k.Low, k.Close, k.VWAP, k.Volume, k.Count)
}

// ===========================================================================
// 7. Get Order Book -- GET /0/public/Depth
// ===========================================================================

// GetOrderBookService returns the order book (asks and bids) for one pair.
type GetOrderBookService struct {
	c      *Client
	params map[string]string
}

func (c *Client) NewGetOrderBookService(pair string) *GetOrderBookService {
	return &GetOrderBookService{c: c, params: map[string]string{"pair": pair}}
}

// SetCount caps the number of asks/bids returned (1-500, default 100).
func (s *GetOrderBookService) SetCount(count int) *GetOrderBookService {
	s.params["count"] = strconv.Itoa(count)
	return s
}

func (s *GetOrderBookService) Do(ctx context.Context) (*OrderBookResult, error) {
	resp, err := request.Do[map[string]OrderBook](request.Get(ctx, s.c, "/0/public/Depth", s.params))
	if err != nil {
		return nil, err
	}
	for pair, book := range *resp {
		return &OrderBookResult{Pair: pair, Asks: book.Asks, Bids: book.Bids}, nil
	}
	return &OrderBookResult{}, nil
}

// OrderBook is the raw asks/bids object Kraken returns under the pair key.
type OrderBook struct {
	Asks []BookEntry `json:"asks"`
	Bids []BookEntry `json:"bids"`
}

// OrderBookResult is the order book for one pair, flattened with its name.
type OrderBookResult struct {
	Pair string
	Asks []BookEntry
	Bids []BookEntry
}

// BookEntry is one order-book level: [price, volume, timestamp].
type BookEntry struct {
	Price     decimal.Decimal
	Volume    decimal.Decimal
	Timestamp time.Time
}

func (e *BookEntry) UnmarshalJSON(data []byte) error {
	return decodeColumns(data, 3, &e.Price, &e.Volume, &e.Timestamp)
}

func (e BookEntry) MarshalJSON() ([]byte, error) {
	return encodeColumns(e.Price, e.Volume, e.Timestamp)
}

// ===========================================================================
// 8. Get Recent Trades -- GET /0/public/Trades
// ===========================================================================

// GetRecentTradesService returns recent trades for a pair plus a nanosecond
// cursor (Last) to fetch newer trades via SetSince.
type GetRecentTradesService struct {
	c      *Client
	params map[string]string
}

func (c *Client) NewGetRecentTradesService(pair string) *GetRecentTradesService {
	return &GetRecentTradesService{c: c, params: map[string]string{"pair": pair}}
}

// SetSince returns trades after the given cursor (a prior result's Last, a
// nanosecond timestamp string).
func (s *GetRecentTradesService) SetSince(cursor string) *GetRecentTradesService {
	s.params["since"] = cursor
	return s
}

// SetCount caps the number of trades returned (1-1000, default 1000).
func (s *GetRecentTradesService) SetCount(count int) *GetRecentTradesService {
	s.params["count"] = strconv.Itoa(count)
	return s
}

func (s *GetRecentTradesService) Do(ctx context.Context) (*TradesResult, error) {
	return request.Do[TradesResult](request.Get(ctx, s.c, "/0/public/Trades", s.params))
}

// TradesResult holds recent trades for one pair plus the pagination cursor.
type TradesResult struct {
	Pair   string  // Kraken pair name the trades belong to
	Trades []Trade // recent trades, oldest first
	Last   string  // nanosecond cursor; pass to SetSince to fetch newer trades
}

func (r *TradesResult) UnmarshalJSON(data []byte) error {
	pair, payload, last, err := splitResultLast(data)
	if err != nil {
		return err
	}
	r.Pair = pair
	if len(last) > 0 {
		if err := common.JSONUnmarshal(last, &r.Last); err != nil {
			return err
		}
	}
	if len(payload) > 0 {
		return common.JSONUnmarshal(payload, &r.Trades)
	}
	return nil
}

func (r TradesResult) MarshalJSON() ([]byte, error) {
	return common.JSONMarshal(map[string]any{r.Pair: r.Trades, "last": r.Last})
}

// Trade is one public trade: [price, volume, time, side, ordertype, misc, id].
type Trade struct {
	Price     decimal.Decimal // execution price
	Volume    decimal.Decimal // executed volume (base currency)
	Time      time.Time       // execution time
	Side      string          // "b" buy, "s" sell
	OrderType string          // "l" limit, "m" market
	Misc      string          // miscellaneous info
	TradeID   int64           // unique trade id
}

func (t *Trade) UnmarshalJSON(data []byte) error {
	return decodeColumns(data, 7, &t.Price, &t.Volume, &t.Time, &t.Side, &t.OrderType, &t.Misc, &t.TradeID)
}

func (t Trade) MarshalJSON() ([]byte, error) {
	return encodeColumns(t.Price, t.Volume, t.Time, t.Side, t.OrderType, t.Misc, t.TradeID)
}

// ===========================================================================
// 9. Get Recent Spreads -- GET /0/public/Spread
// ===========================================================================

// GetRecentSpreadsService returns recent bid/ask spreads for a pair plus a
// cursor (Last) for incremental polling via SetSince.
type GetRecentSpreadsService struct {
	c      *Client
	params map[string]string
}

func (c *Client) NewGetRecentSpreadsService(pair string) *GetRecentSpreadsService {
	return &GetRecentSpreadsService{c: c, params: map[string]string{"pair": pair}}
}

// SetSince returns spreads at or after t (use a prior result's Last).
func (s *GetRecentSpreadsService) SetSince(t time.Time) *GetRecentSpreadsService {
	s.params["since"] = strconv.FormatInt(t.Unix(), 10)
	return s
}

func (s *GetRecentSpreadsService) Do(ctx context.Context) (*SpreadResult, error) {
	return request.Do[SpreadResult](request.Get(ctx, s.c, "/0/public/Spread", s.params))
}

// SpreadResult holds recent spreads for one pair plus the pagination cursor.
type SpreadResult struct {
	Pair    string    // Kraken pair name the spreads belong to
	Spreads []Spread  // recent spreads, oldest first
	Last    time.Time // cursor; pass to SetSince to poll for newer spreads
}

func (r *SpreadResult) UnmarshalJSON(data []byte) error {
	pair, payload, last, err := splitResultLast(data)
	if err != nil {
		return err
	}
	r.Pair = pair
	if len(last) > 0 {
		if err := common.JSONUnmarshal(last, &r.Last); err != nil {
			return err
		}
	}
	if len(payload) > 0 {
		return common.JSONUnmarshal(payload, &r.Spreads)
	}
	return nil
}

func (r SpreadResult) MarshalJSON() ([]byte, error) {
	return common.JSONMarshal(map[string]any{r.Pair: r.Spreads, "last": r.Last})
}

// Spread is one bid/ask spread sample: [time, bid, ask].
type Spread struct {
	Time time.Time       // sample time
	Bid  decimal.Decimal // best bid price
	Ask  decimal.Decimal // best ask price
}

func (s *Spread) UnmarshalJSON(data []byte) error {
	return decodeColumns(data, 3, &s.Time, &s.Bid, &s.Ask)
}

func (s Spread) MarshalJSON() ([]byte, error) {
	return encodeColumns(s.Time, s.Bid, s.Ask)
}

// ===========================================================================
// 10. Get Grouped Order Book -- GET /0/public/GroupedBook
// ===========================================================================

// GetGroupedOrderBookService returns an order book whose volume is aggregated
// over a configurable tick range ("grouping"), for one pair.
type GetGroupedOrderBookService struct {
	c      *Client
	params map[string]string
}

func (c *Client) NewGetGroupedOrderBookService(pair string) *GetGroupedOrderBookService {
	return &GetGroupedOrderBookService{c: c, params: map[string]string{"pair": pair}}
}

// SetDepth sets the number of price levels per side (10, 25, 100, 250, 1000;
// default 10).
func (s *GetGroupedOrderBookService) SetDepth(depth int) *GetGroupedOrderBookService {
	s.params["depth"] = strconv.Itoa(depth)
	return s
}

// SetGrouping sets the number of ticks aggregated per price level (1, 5, 10, 25,
// 50, 100, 250, 500, 1000; default 1).
func (s *GetGroupedOrderBookService) SetGrouping(grouping int) *GetGroupedOrderBookService {
	s.params["grouping"] = strconv.Itoa(grouping)
	return s
}

func (s *GetGroupedOrderBookService) Do(ctx context.Context) (*GroupedOrderBook, error) {
	return request.Do[GroupedOrderBook](request.Get(ctx, s.c, "/0/public/GroupedBook", s.params))
}

// GroupedOrderBook is the aggregated order book for one pair.
type GroupedOrderBook struct {
	Pair     string         `json:"pair"`     // canonical pair name
	Grouping int            `json:"grouping"` // tick grouping applied
	Asks     []GroupedLevel `json:"asks"`     // aggregated ask levels
	Bids     []GroupedLevel `json:"bids"`     // aggregated bid levels
}

// GroupedLevel is one aggregated price level.
type GroupedLevel struct {
	Price decimal.Decimal `json:"price"` // grouped price level
	Qty   decimal.Decimal `json:"qty"`   // aggregated quantity at the level
}

// ===========================================================================
// 11. Query L3 Order Book -- POST /0/private/Level3 (authenticated)
// ===========================================================================

// QueryL3OrderBookService returns the level-3 (per-order) order book for a pair.
// Despite being market data, it is an authenticated endpoint.
type QueryL3OrderBookService struct {
	c      *Client
	params map[string]string
}

func (c *Client) NewQueryL3OrderBookService(pair string) *QueryL3OrderBookService {
	return &QueryL3OrderBookService{c: c, params: map[string]string{"pair": pair}}
}

// SetCount caps the number of orders returned per side.
func (s *QueryL3OrderBookService) SetCount(count int) *QueryL3OrderBookService {
	s.params["count"] = strconv.Itoa(count)
	return s
}

func (s *QueryL3OrderBookService) Do(ctx context.Context) (*L3OrderBook, error) {
	return request.Do[L3OrderBook](request.Post(ctx, s.c, "/0/private/Level3", s.params).WithSign())
}

// L3OrderBook is the per-order (level 3) order book for one pair.
type L3OrderBook struct {
	Pair string    `json:"pair"` // canonical pair name
	Asks []L3Entry `json:"asks"` // individual ask orders
	Bids []L3Entry `json:"bids"` // individual bid orders
}

// L3Entry is one resting order in the level-3 book.
type L3Entry struct {
	OrderID   string          `json:"order_id"`  // order identifier
	Price     decimal.Decimal `json:"price"`     // limit price
	Qty       decimal.Decimal `json:"qty"`       // remaining quantity
	Timestamp NanoTime        `json:"timestamp"` // order timestamp (UNIX nanoseconds)
}
