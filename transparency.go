package kraken

import (
	"context"
	"strconv"
	"time"

	"github.com/UnipayFI/go-kraken/request"
	"github.com/shopspring/decimal"
)

// Transparency endpoints expose MiFID-style pre- and post-trade transparency
// data for Kraken's regulated spot venue.

// ===========================================================================
// 1. Pre-Trade Data -- GET /0/public/PreTrade
// ===========================================================================

// GetPreTradeDataService returns the top aggregated order-book levels (pre-trade
// transparency) for a symbol.
type GetPreTradeDataService struct {
	c      *Client
	params map[string]string
}

// NewGetPreTradeDataService queries pre-trade data for a symbol (e.g. "BTC/USD").
func (c *Client) NewGetPreTradeDataService(symbol string) *GetPreTradeDataService {
	return &GetPreTradeDataService{c: c, params: map[string]string{"symbol": symbol}}
}

func (s *GetPreTradeDataService) Do(ctx context.Context) (*PreTradeData, error) {
	return request.Do[PreTradeData](request.Get(ctx, s.c, "/0/public/PreTrade", s.params))
}

// PreTradeData is the aggregated order book for one symbol with instrument
// reference data.
type PreTradeData struct {
	Symbol            string          `json:"symbol"`               // currency pair symbol
	Description       string          `json:"description"`          // human-readable description
	BaseAsset         string          `json:"base_asset"`           // base asset
	BaseDTICode       string          `json:"base_dti_code"`        // base Digital Token Identifier
	BaseDTIShortName  string          `json:"base_dti_short_name"`  // base DTI short name
	BaseNotation      string          `json:"base_notation"`        // base quantity notation
	QuoteAsset        string          `json:"quote_asset"`          // quote asset
	QuoteDTICode      string          `json:"quote_dti_code"`       // quote Digital Token Identifier
	QuoteDTIShortName string          `json:"quote_dti_short_name"` // quote DTI short name
	QuoteNotation     string          `json:"quote_notation"`       // quote notation (e.g. MONE)
	Venue             string          `json:"venue"`                // market identifier code (MIC)
	System            string          `json:"system"`               // trading system (e.g. CLOB)
	Asks              []PreTradeLevel `json:"asks"`                 // aggregated ask levels
	Bids              []PreTradeLevel `json:"bids"`                 // aggregated bid levels
}

// PreTradeLevel is one aggregated transparency order-book level.
type PreTradeLevel struct {
	Side            string          `json:"side"`           // BUY or SELL
	Price           decimal.Decimal `json:"price"`          // price level
	Qty             decimal.Decimal `json:"qty"`            // aggregated quantity
	Count           int             `json:"count"`          // number of orders aggregated
	SubmissionTime  time.Time       `json:"submission_ts"`  // order submission time
	PublicationTime time.Time       `json:"publication_ts"` // data publication time
}

// ===========================================================================
// 2. Post-Trade Data -- GET /0/public/PostTrade
// ===========================================================================

// GetPostTradeDataService returns executed-trade (post-trade transparency) data.
// With no filters it returns the most recent trades across all pairs.
type GetPostTradeDataService struct {
	c      *Client
	params map[string]string
}

func (c *Client) NewGetPostTradeDataService() *GetPostTradeDataService {
	return &GetPostTradeDataService{c: c, params: map[string]string{}}
}

// SetSymbol filters to one currency pair (e.g. "BTC/USD").
func (s *GetPostTradeDataService) SetSymbol(symbol string) *GetPostTradeDataService {
	s.params["symbol"] = symbol
	return s
}

// SetFromTime returns trades at or after t.
func (s *GetPostTradeDataService) SetFromTime(t time.Time) *GetPostTradeDataService {
	s.params["from_ts"] = t.UTC().Format(time.RFC3339Nano)
	return s
}

// SetToTime returns trades at or before t.
func (s *GetPostTradeDataService) SetToTime(t time.Time) *GetPostTradeDataService {
	s.params["to_ts"] = t.UTC().Format(time.RFC3339Nano)
	return s
}

// SetCount caps the number of trades returned (1-1000, default 1000).
func (s *GetPostTradeDataService) SetCount(count int) *GetPostTradeDataService {
	s.params["count"] = strconv.Itoa(count)
	return s
}

func (s *GetPostTradeDataService) Do(ctx context.Context) (*PostTradeData, error) {
	return request.Do[PostTradeData](request.Get(ctx, s.c, "/0/public/PostTrade", s.params))
}

// PostTradeData is a page of executed-trade transparency records.
type PostTradeData struct {
	Count    int         `json:"count"`   // number of trades returned
	LastTime time.Time   `json:"last_ts"` // timestamp of the last trade in the page
	Trades   []PostTrade `json:"trades"`  // executed trades
}

// PostTrade is one executed-trade transparency record.
type PostTrade struct {
	TradeID           string          `json:"trade_id"`             // trade identifier
	Symbol            string          `json:"symbol"`               // currency pair symbol
	Description       string          `json:"description"`          // human-readable description
	Price             decimal.Decimal `json:"price"`                // execution price
	Quantity          decimal.Decimal `json:"quantity"`             // executed quantity
	BaseAsset         string          `json:"base_asset"`           // base asset
	BaseDTICode       string          `json:"base_dti_code"`        // base Digital Token Identifier
	BaseDTIShortName  string          `json:"base_dti_short_name"`  // base DTI short name
	BaseNotation      string          `json:"base_notation"`        // base quantity notation
	QuoteAsset        string          `json:"quote_asset"`          // quote asset
	QuoteDTICode      string          `json:"quote_dti_code"`       // quote Digital Token Identifier
	QuoteDTIShortName string          `json:"quote_dti_short_name"` // quote DTI short name
	QuoteNotation     string          `json:"quote_notation"`       // quote notation (e.g. MONE)
	TradeVenue        string          `json:"trade_venue"`          // execution venue (MIC)
	TradeTime         time.Time       `json:"trade_ts"`             // execution time
	PublicationVenue  string          `json:"publication_venue"`    // publication venue (MIC)
	PublicationTime   time.Time       `json:"publication_ts"`       // data publication time
}
