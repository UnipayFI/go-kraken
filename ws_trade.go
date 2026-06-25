package kraken

import (
	"context"
	"time"

	"github.com/UnipayFI/go-kraken/request"
	"github.com/go-json-experiment/json/jsontext"
	"github.com/shopspring/decimal"
)

// WsTradeResponse re-exports the request-layer trade response for convenience.
type WsTradeResponse[T any] = request.WsTradeResponse[T]

// TradeConn is a persistent, authenticated connection for placing and managing
// orders over WebSocket — a low-latency alternative to the REST trade endpoints.
// It is request/response: each method blocks for its matching reply.
type TradeConn struct {
	*request.WsTradeConn
}

// DialTrade opens the private v2 gateway, fetches an auth token, and returns a
// ready order-entry connection. Close it when done.
func (c *WebSocketClient) DialTrade(ctx context.Context) (*TradeConn, error) {
	tc, err := request.DialWsTrade(ctx, c)
	if err != nil {
		return nil, err
	}
	return &TradeConn{tc}, nil
}

// Trade is the low-level escape hatch: it sends an arbitrary method with the
// given params (the auth token is injected automatically) and returns the raw
// decoded result. Prefer the typed methods below.
func (t *TradeConn) Trade(ctx context.Context, method string, params map[string]any) (*WsTradeResponse[jsonRaw], error) {
	return request.WsTradeCall[jsonRaw](ctx, t.WsTradeConn, method, params)
}

// jsonRaw captures a result of any shape as raw JSON.
type jsonRaw = jsontext.Value

// wsNum renders a decimal as a JSON number (Kraken WS expects numeric, not
// string, prices/quantities) while preserving full precision.
type wsNum string

func (n wsNum) MarshalJSON() ([]byte, error) { return []byte(n), nil }

func num(d decimal.Decimal) wsNum { return wsNum(d.String()) }

// ===========================================================================
// add_order
// ===========================================================================

// WsAddOrder describes a new order placed over WebSocket. Required: Symbol,
// Side, OrderType, OrderQty. Zero-valued optional fields are omitted.
type WsAddOrder struct {
	Symbol        string          // required: currency pair, e.g. "BTC/USD"
	Side          OrderSide       // required: buy or sell
	OrderType     OrderType       // required: limit, market, ...
	OrderQty      decimal.Decimal // required: order quantity (base)
	LimitPrice    decimal.Decimal // limit price (omitted if zero)
	TimeInForce   TimeInForce     // time in force
	PostOnly      bool            // post-only
	ReduceOnly    bool            // reduce-only (margin)
	Margin        bool            // fund on margin
	Validate      bool            // validate only, do not submit
	DisplayQty    decimal.Decimal // iceberg display quantity (omitted if zero)
	ClientOrderID string          // client order id (see MaxClOrdIDLen / ValidateClOrdID)
	OrderUserRef  int             // numeric client reference (omitted if zero)
	FeePreference string          // "base" or "quote"
	StpType       string          // self-trade prevention
}

func (o WsAddOrder) toParams() map[string]any {
	p := map[string]any{
		"symbol":     o.Symbol,
		"side":       string(o.Side),
		"order_type": string(o.OrderType),
		"order_qty":  num(o.OrderQty),
	}
	if !o.LimitPrice.IsZero() {
		p["limit_price"] = num(o.LimitPrice)
	}
	if o.TimeInForce != "" {
		p["time_in_force"] = string(o.TimeInForce)
	}
	if o.PostOnly {
		p["post_only"] = true
	}
	if o.ReduceOnly {
		p["reduce_only"] = true
	}
	if o.Margin {
		p["margin"] = true
	}
	if o.Validate {
		p["validate"] = true
	}
	if !o.DisplayQty.IsZero() {
		p["display_qty"] = num(o.DisplayQty)
	}
	if o.ClientOrderID != "" {
		p["cl_ord_id"] = o.ClientOrderID
	}
	if o.OrderUserRef != 0 {
		p["order_userref"] = o.OrderUserRef
	}
	if o.FeePreference != "" {
		p["fee_preference"] = o.FeePreference
	}
	if o.StpType != "" {
		p["stp_type"] = o.StpType
	}
	return p
}

// AddOrder places a new order.
func (t *TradeConn) AddOrder(ctx context.Context, order WsAddOrder) (*WsTradeResponse[WsAddOrderResult], error) {
	return request.WsTradeCall[WsAddOrderResult](ctx, t.WsTradeConn, "add_order", order.toParams())
}

// WsAddOrderResult is the add_order result.
type WsAddOrderResult struct {
	OrderID       string   `json:"order_id"`      // new order id
	ClientOrderID string   `json:"cl_ord_id"`     // client order id, if supplied
	OrderUserRef  int64    `json:"order_userref"` // numeric client reference, if supplied
	Warnings      []string `json:"warnings"`      // non-fatal warnings
}

// ===========================================================================
// batch_add
// ===========================================================================

// BatchAdd places 2-15 orders for a single pair in one request.
func (t *TradeConn) BatchAdd(ctx context.Context, symbol string, orders ...WsAddOrder) (*WsTradeResponse[[]WsAddOrderResult], error) {
	list := make([]map[string]any, 0, len(orders))
	for _, o := range orders {
		op := o.toParams()
		delete(op, "symbol") // symbol is a top-level batch param
		list = append(list, op)
	}
	params := map[string]any{"symbol": symbol, "orders": list}
	return request.WsTradeCall[[]WsAddOrderResult](ctx, t.WsTradeConn, "batch_add", params)
}

// ===========================================================================
// amend_order
// ===========================================================================

// WsAmendOrder amends an open order in place. Identify it by OrderID or
// ClientOrderID; set the fields to change.
type WsAmendOrder struct {
	OrderID       string          // order to amend (or use ClientOrderID)
	ClientOrderID string          // client order id (alternative to OrderID)
	OrderQty      decimal.Decimal // new quantity (omitted if zero)
	DisplayQty    decimal.Decimal // new iceberg display quantity (omitted if zero)
	LimitPrice    decimal.Decimal // new limit price (omitted if zero)
	TriggerPrice  decimal.Decimal // new trigger price (omitted if zero)
	PostOnly      *bool           // toggle post-only (nil leaves unchanged)
}

// AmendOrder amends an open order in place (preserving queue priority where
// possible).
func (t *TradeConn) AmendOrder(ctx context.Context, amend WsAmendOrder) (*WsTradeResponse[WsAmendOrderResult], error) {
	p := map[string]any{}
	if amend.OrderID != "" {
		p["order_id"] = amend.OrderID
	}
	if amend.ClientOrderID != "" {
		p["cl_ord_id"] = amend.ClientOrderID
	}
	if !amend.OrderQty.IsZero() {
		p["order_qty"] = num(amend.OrderQty)
	}
	if !amend.DisplayQty.IsZero() {
		p["display_qty"] = num(amend.DisplayQty)
	}
	if !amend.LimitPrice.IsZero() {
		p["limit_price"] = num(amend.LimitPrice)
	}
	if !amend.TriggerPrice.IsZero() {
		p["trigger_price"] = num(amend.TriggerPrice)
	}
	if amend.PostOnly != nil {
		p["post_only"] = *amend.PostOnly
	}
	return request.WsTradeCall[WsAmendOrderResult](ctx, t.WsTradeConn, "amend_order", p)
}

// WsAmendOrderResult is the amend_order result.
type WsAmendOrderResult struct {
	AmendID       string   `json:"amend_id"`  // amendment id
	OrderID       string   `json:"order_id"`  // amended order id
	ClientOrderID string   `json:"cl_ord_id"` // client order id, if supplied
	Warnings      []string `json:"warnings"`  // non-fatal warnings
}

// ===========================================================================
// edit_order
// ===========================================================================

// WsEditOrder cancel-replaces an existing order (yielding a new order id).
type WsEditOrder struct {
	OrderID      string          // required: order to replace
	Symbol       string          // required: currency pair
	OrderQty     decimal.Decimal // new quantity (omitted if zero)
	LimitPrice   decimal.Decimal // new limit price (omitted if zero)
	DisplayQty   decimal.Decimal // new iceberg display quantity (omitted if zero)
	OrderUserRef int             // new numeric reference (omitted if zero)
	PostOnly     *bool           // toggle post-only (nil leaves unchanged)
}

// EditOrder cancel-replaces an existing order with new parameters.
func (t *TradeConn) EditOrder(ctx context.Context, edit WsEditOrder) (*WsTradeResponse[WsEditOrderResult], error) {
	p := map[string]any{"order_id": edit.OrderID, "symbol": edit.Symbol}
	if !edit.OrderQty.IsZero() {
		p["order_qty"] = num(edit.OrderQty)
	}
	if !edit.LimitPrice.IsZero() {
		p["limit_price"] = num(edit.LimitPrice)
	}
	if !edit.DisplayQty.IsZero() {
		p["display_qty"] = num(edit.DisplayQty)
	}
	if edit.OrderUserRef != 0 {
		p["order_userref"] = edit.OrderUserRef
	}
	if edit.PostOnly != nil {
		p["post_only"] = *edit.PostOnly
	}
	return request.WsTradeCall[WsEditOrderResult](ctx, t.WsTradeConn, "edit_order", p)
}

// WsEditOrderResult is the edit_order result.
type WsEditOrderResult struct {
	OrderID         string   `json:"order_id"`          // new order id
	OriginalOrderID string   `json:"original_order_id"` // replaced order id
	Warnings        []string `json:"warnings"`          // non-fatal warnings
}

// ===========================================================================
// cancel_order
// ===========================================================================

// CancelOrder cancels one or more open orders by order id.
func (t *TradeConn) CancelOrder(ctx context.Context, orderIDs ...string) (*WsTradeResponse[WsCancelOrderResult], error) {
	return request.WsTradeCall[WsCancelOrderResult](ctx, t.WsTradeConn, "cancel_order", map[string]any{"order_id": orderIDs})
}

// CancelOrderByClientID cancels one or more open orders by client order id.
func (t *TradeConn) CancelOrderByClientID(ctx context.Context, clOrdIDs ...string) (*WsTradeResponse[WsCancelOrderResult], error) {
	return request.WsTradeCall[WsCancelOrderResult](ctx, t.WsTradeConn, "cancel_order", map[string]any{"cl_ord_id": clOrdIDs})
}

// WsCancelOrderResult is the cancel_order result.
type WsCancelOrderResult struct {
	OrderID       string   `json:"order_id"`  // cancelled order id
	ClientOrderID string   `json:"cl_ord_id"` // client order id, if supplied
	Warnings      []string `json:"warnings"`  // non-fatal warnings
}

// ===========================================================================
// cancel_all
// ===========================================================================

// CancelAll cancels all open orders.
func (t *TradeConn) CancelAll(ctx context.Context) (*WsTradeResponse[WsCancelAllResult], error) {
	return request.WsTradeCall[WsCancelAllResult](ctx, t.WsTradeConn, "cancel_all", map[string]any{})
}

// WsCancelAllResult is the cancel_all / batch_cancel result.
type WsCancelAllResult struct {
	Count    int      `json:"count"`    // number of orders cancelled
	Warnings []string `json:"warnings"` // non-fatal warnings
}

// ===========================================================================
// cancel_all_orders_after
// ===========================================================================

// CancelAllOrdersAfter arms (or, with timeout 0, disarms) a dead-man's switch
// that cancels all open orders after timeout seconds unless reset.
func (t *TradeConn) CancelAllOrdersAfter(ctx context.Context, timeout int) (*WsTradeResponse[WsCancelAllAfterResult], error) {
	return request.WsTradeCall[WsCancelAllAfterResult](ctx, t.WsTradeConn, "cancel_all_orders_after", map[string]any{"timeout": timeout})
}

// WsCancelAllAfterResult is the cancel_all_orders_after result.
type WsCancelAllAfterResult struct {
	CurrentTime time.Time `json:"currentTime"` // when the request was received
	TriggerTime time.Time `json:"triggerTime"` // when orders will be cancelled
}

// ===========================================================================
// batch_cancel
// ===========================================================================

// BatchCancel cancels up to 50 open orders by order id in one request. Kraken
// returns an (empty) result on success, so the response's Success flag — not a
// count — is the authoritative outcome.
func (t *TradeConn) BatchCancel(ctx context.Context, orderIDs ...string) (*WsTradeResponse[WsBatchCancelResult], error) {
	return request.WsTradeCall[WsBatchCancelResult](ctx, t.WsTradeConn, "batch_cancel", map[string]any{"orders": orderIDs})
}

// WsBatchCancelResult is the batch_cancel result. Kraken returns an empty result
// on success; Warnings is populated only when present.
type WsBatchCancelResult struct {
	Count    int      `json:"count"`    // number cancelled (not always returned)
	Warnings []string `json:"warnings"` // non-fatal warnings
}
