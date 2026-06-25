package kraken

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/UnipayFI/go-kraken/request"
	"github.com/shopspring/decimal"
)

// ===========================================================================
// 1. Add Order -- POST /0/private/AddOrder
// ===========================================================================

// AddOrderService places a new order. Required parameters (pair, side, order
// type, volume) are constructor arguments; everything else is an optional
// setter. Use SetValidate(true) to validate without submitting.
type AddOrderService struct {
	c      *Client
	params map[string]string
}

// NewAddOrderService builds an order for pair with the given side, order type
// and volume (base currency).
func (c *Client) NewAddOrderService(pair string, side OrderSide, orderType OrderType, volume decimal.Decimal) *AddOrderService {
	return &AddOrderService{c: c, params: map[string]string{
		"pair":      pair,
		"type":      string(side),
		"ordertype": string(orderType),
		"volume":    volume.String(),
	}}
}

// SetPrice sets the limit price or trigger price (depending on order type).
func (s *AddOrderService) SetPrice(price decimal.Decimal) *AddOrderService {
	s.params["price"] = price.String()
	return s
}

// SetPrice2 sets the secondary price (for stop-loss-limit / take-profit-limit).
func (s *AddOrderService) SetPrice2(price2 decimal.Decimal) *AddOrderService {
	s.params["price2"] = price2.String()
	return s
}

// SetLeverage sets the desired leverage amount (e.g. "2:1" or "2").
func (s *AddOrderService) SetLeverage(leverage string) *AddOrderService {
	s.params["leverage"] = leverage
	return s
}

// SetOrderFlags sets the comma-delimited order flags (OrderFlagPost, ...).
func (s *AddOrderService) SetOrderFlags(oflags string) *AddOrderService {
	s.params["oflags"] = oflags
	return s
}

// SetTimeInForce sets the time-in-force (default GTC).
func (s *AddOrderService) SetTimeInForce(tif TimeInForce) *AddOrderService {
	s.params["timeinforce"] = string(tif)
	return s
}

// SetTrigger sets the price signal for stop/take-profit triggers (default last).
func (s *AddOrderService) SetTrigger(trigger TriggerSignal) *AddOrderService {
	s.params["trigger"] = string(trigger)
	return s
}

// SetSTPType sets the self-trade-prevention behavior.
func (s *AddOrderService) SetSTPType(stp SelfTradePrevention) *AddOrderService {
	s.params["stptype"] = string(stp)
	return s
}

// SetReduceOnly marks the order as reduce-only (margin).
func (s *AddOrderService) SetReduceOnly(reduceOnly bool) *AddOrderService {
	s.params["reduce_only"] = formatBool(reduceOnly)
	return s
}

// SetDisplayVolume sets the displayed quantity for iceberg orders.
func (s *AddOrderService) SetDisplayVolume(displayVol decimal.Decimal) *AddOrderService {
	s.params["displayvol"] = displayVol.String()
	return s
}

// SetUserRef sets a non-unique numeric identifier for grouping orders.
func (s *AddOrderService) SetUserRef(userRef int) *AddOrderService {
	s.params["userref"] = formatInt(userRef)
	return s
}

// SetClientOrderID sets a client order id (UUID or free text up to 18 chars).
func (s *AddOrderService) SetClientOrderID(clOrdID string) *AddOrderService {
	s.params["cl_ord_id"] = clOrdID
	return s
}

// SetStartTime sets the scheduled start time ("0", a unix timestamp, or
// "+<n>" seconds from now).
func (s *AddOrderService) SetStartTime(startTime string) *AddOrderService {
	s.params["starttm"] = startTime
	return s
}

// SetExpireTime sets the GTD expiry time (a unix timestamp or "+<n>" seconds).
func (s *AddOrderService) SetExpireTime(expireTime string) *AddOrderService {
	s.params["expiretm"] = expireTime
	return s
}

// SetCloseOrder attaches a conditional close order. Pass a zero price2 to omit
// the secondary price.
func (s *AddOrderService) SetCloseOrder(orderType OrderType, price, price2 decimal.Decimal) *AddOrderService {
	s.params["close[ordertype]"] = string(orderType)
	if !price.IsZero() {
		s.params["close[price]"] = price.String()
	}
	if !price2.IsZero() {
		s.params["close[price2]"] = price2.String()
	}
	return s
}

// SetDeadline sets an RFC3339 rejection deadline (now + 2..60s).
func (s *AddOrderService) SetDeadline(deadline string) *AddOrderService {
	s.params["deadline"] = deadline
	return s
}

// SetValidate validates the order without submitting it when true.
func (s *AddOrderService) SetValidate(validate bool) *AddOrderService {
	s.params["validate"] = formatBool(validate)
	return s
}

func (s *AddOrderService) Do(ctx context.Context) (*AddOrderResult, error) {
	return request.Do[AddOrderResult](request.Post(ctx, s.c, "/0/private/AddOrder", s.params).WithSign())
}

// OrderDescr is the human-readable description returned for a placed order.
type OrderDescr struct {
	Order string `json:"order"` // order description
	Close string `json:"close"` // conditional close order description, if any
}

// AddOrderResult is the AddOrder response. In validate mode TxID is empty.
type AddOrderResult struct {
	Description OrderDescr `json:"descr"` // order description(s)
	TxID        []string   `json:"txid"`  // transaction id(s) of the placed order(s)
}

// ===========================================================================
// 2. Add Order Batch -- POST /0/private/AddOrderBatch
// ===========================================================================

// BatchOrder is one leg of an AddOrderBatch request. Zero-valued optional
// fields are omitted from the request.
type BatchOrder struct {
	OrderType     OrderType           // required: execution model
	Side          OrderSide           // required: buy or sell
	Volume        decimal.Decimal     // required: base-currency quantity
	Price         decimal.Decimal     // limit/trigger price (omitted if zero)
	Price2        decimal.Decimal     // secondary price (omitted if zero)
	OrderFlags    string              // comma-delimited order flags
	TimeInForce   TimeInForce         // time in force
	Trigger       TriggerSignal       // trigger price signal
	STPType       SelfTradePrevention // self-trade prevention
	ReduceOnly    bool                // reduce-only (margin)
	DisplayVolume decimal.Decimal     // iceberg display quantity (omitted if zero)
	UserRef       int                 // group identifier (omitted if zero)
	ClientOrderID string              // client order id
	StartTime     string              // scheduled start time
	ExpireTime    string              // GTD expiry
}

func (o BatchOrder) toParams(prefix string, dst map[string]string) {
	dst[prefix+"[ordertype]"] = string(o.OrderType)
	dst[prefix+"[type]"] = string(o.Side)
	dst[prefix+"[volume]"] = o.Volume.String()
	if !o.Price.IsZero() {
		dst[prefix+"[price]"] = o.Price.String()
	}
	if !o.Price2.IsZero() {
		dst[prefix+"[price2]"] = o.Price2.String()
	}
	if o.OrderFlags != "" {
		dst[prefix+"[oflags]"] = o.OrderFlags
	}
	if o.TimeInForce != "" {
		dst[prefix+"[timeinforce]"] = string(o.TimeInForce)
	}
	if o.Trigger != "" {
		dst[prefix+"[trigger]"] = string(o.Trigger)
	}
	if o.STPType != "" {
		dst[prefix+"[stptype]"] = string(o.STPType)
	}
	if o.ReduceOnly {
		dst[prefix+"[reduce_only]"] = "true"
	}
	if !o.DisplayVolume.IsZero() {
		dst[prefix+"[displayvol]"] = o.DisplayVolume.String()
	}
	if o.UserRef != 0 {
		dst[prefix+"[userref]"] = formatInt(o.UserRef)
	}
	if o.ClientOrderID != "" {
		dst[prefix+"[cl_ord_id]"] = o.ClientOrderID
	}
	if o.StartTime != "" {
		dst[prefix+"[starttm]"] = o.StartTime
	}
	if o.ExpireTime != "" {
		dst[prefix+"[expiretm]"] = o.ExpireTime
	}
}

// AddOrderBatchService places 2-15 orders for a single pair in one request.
type AddOrderBatchService struct {
	c        *Client
	pair     string
	orders   []BatchOrder
	deadline string
	validate bool
}

// NewAddOrderBatchService starts a batch for the given pair.
func (c *Client) NewAddOrderBatchService(pair string) *AddOrderBatchService {
	return &AddOrderBatchService{c: c, pair: pair}
}

// AddOrder appends an order to the batch.
func (s *AddOrderBatchService) AddOrder(order BatchOrder) *AddOrderBatchService {
	s.orders = append(s.orders, order)
	return s
}

// SetDeadline sets an RFC3339 rejection deadline (now + 2..60s).
func (s *AddOrderBatchService) SetDeadline(deadline string) *AddOrderBatchService {
	s.deadline = deadline
	return s
}

// SetValidate validates the batch without submitting it when true.
func (s *AddOrderBatchService) SetValidate(validate bool) *AddOrderBatchService {
	s.validate = validate
	return s
}

func (s *AddOrderBatchService) Do(ctx context.Context) (*AddOrderBatchResult, error) {
	params := map[string]string{"pair": s.pair}
	if s.deadline != "" {
		params["deadline"] = s.deadline
	}
	if s.validate {
		params["validate"] = "true"
	}
	for i, o := range s.orders {
		o.toParams(fmt.Sprintf("orders[%d]", i), params)
	}
	return request.Do[AddOrderBatchResult](request.Post(ctx, s.c, "/0/private/AddOrderBatch", params).WithSign())
}

// AddOrderBatchResult is the AddOrderBatch response, one entry per submitted
// order in request order.
type AddOrderBatchResult struct {
	Orders []BatchOrderResult `json:"orders"`
}

// BatchOrderResult is the outcome of one batched order.
type BatchOrderResult struct {
	TxID        string     `json:"txid"`  // transaction id, if successful
	Description OrderDescr `json:"descr"` // order description
	Error       string     `json:"error"` // per-order error message, if any
}

// ===========================================================================
// 3. Amend Order -- POST /0/private/AmendOrder
// ===========================================================================

// AmendOrderService amends an open order in place (preserving queue priority
// where possible) without a cancel-replace.
type AmendOrderService struct {
	c      *Client
	params map[string]string
}

// NewAmendOrderService amends the order with the given Kraken txid. To amend by
// client order id instead, pass "" and call SetClientOrderID.
func (c *Client) NewAmendOrderService(txid string) *AmendOrderService {
	params := map[string]string{}
	if txid != "" {
		params["txid"] = txid
	}
	return &AmendOrderService{c: c, params: params}
}

// SetClientOrderID amends by client order id instead of txid.
func (s *AmendOrderService) SetClientOrderID(clOrdID string) *AmendOrderService {
	s.params["cl_ord_id"] = clOrdID
	return s
}

// SetOrderQty sets the new order quantity (base asset).
func (s *AmendOrderService) SetOrderQty(qty decimal.Decimal) *AmendOrderService {
	s.params["order_qty"] = qty.String()
	return s
}

// SetDisplayQty sets the new iceberg display quantity.
func (s *AmendOrderService) SetDisplayQty(qty decimal.Decimal) *AmendOrderService {
	s.params["display_qty"] = qty.String()
	return s
}

// SetLimitPrice sets the new limit price.
func (s *AmendOrderService) SetLimitPrice(price decimal.Decimal) *AmendOrderService {
	s.params["limit_price"] = price.String()
	return s
}

// SetTriggerPrice sets the new trigger price (for trigger order types).
func (s *AmendOrderService) SetTriggerPrice(price decimal.Decimal) *AmendOrderService {
	s.params["trigger_price"] = price.String()
	return s
}

// SetPostOnly restricts the amended order from taking liquidity.
func (s *AmendOrderService) SetPostOnly(postOnly bool) *AmendOrderService {
	s.params["post_only"] = formatBool(postOnly)
	return s
}

// SetDeadline sets an RFC3339 rejection deadline.
func (s *AmendOrderService) SetDeadline(deadline string) *AmendOrderService {
	s.params["deadline"] = deadline
	return s
}

func (s *AmendOrderService) Do(ctx context.Context) (*AmendOrderResult, error) {
	return request.Do[AmendOrderResult](request.Post(ctx, s.c, "/0/private/AmendOrder", s.params).WithSign())
}

// AmendOrderResult is the AmendOrder response.
type AmendOrderResult struct {
	AmendID string `json:"amend_id"` // unique id of this amend transaction
}

// ===========================================================================
// 4. Edit Order -- POST /0/private/EditOrder
// ===========================================================================

// EditOrderService cancel-replaces an existing order with new parameters
// (yielding a new txid).
type EditOrderService struct {
	c      *Client
	params map[string]string
}

// NewEditOrderService edits the order txid on pair.
func (c *Client) NewEditOrderService(pair, txid string) *EditOrderService {
	return &EditOrderService{c: c, params: map[string]string{
		"pair": pair,
		"txid": txid,
	}}
}

// SetVolume sets the new order volume (base currency).
func (s *EditOrderService) SetVolume(volume decimal.Decimal) *EditOrderService {
	s.params["volume"] = volume.String()
	return s
}

// SetPrice sets the new price.
func (s *EditOrderService) SetPrice(price decimal.Decimal) *EditOrderService {
	s.params["price"] = price.String()
	return s
}

// SetPrice2 sets the new secondary price.
func (s *EditOrderService) SetPrice2(price2 decimal.Decimal) *EditOrderService {
	s.params["price2"] = price2.String()
	return s
}

// SetOrderFlags sets the comma-delimited order flags.
func (s *EditOrderService) SetOrderFlags(oflags string) *EditOrderService {
	s.params["oflags"] = oflags
	return s
}

// SetUserRef sets a new user reference id.
func (s *EditOrderService) SetUserRef(userRef int) *EditOrderService {
	s.params["userref"] = formatInt(userRef)
	return s
}

// SetDisplayVolume sets the new iceberg display volume.
func (s *EditOrderService) SetDisplayVolume(displayVol decimal.Decimal) *EditOrderService {
	s.params["displayvol"] = displayVol.String()
	return s
}

// SetCancelResponse includes the cancellation result in the response.
func (s *EditOrderService) SetCancelResponse(cancelResponse bool) *EditOrderService {
	s.params["cancel_response"] = formatBool(cancelResponse)
	return s
}

// SetDeadline sets an RFC3339 rejection deadline.
func (s *EditOrderService) SetDeadline(deadline string) *EditOrderService {
	s.params["deadline"] = deadline
	return s
}

// SetValidate validates the edit without submitting it when true.
func (s *EditOrderService) SetValidate(validate bool) *EditOrderService {
	s.params["validate"] = formatBool(validate)
	return s
}

func (s *EditOrderService) Do(ctx context.Context) (*EditOrderResult, error) {
	return request.Do[EditOrderResult](request.Post(ctx, s.c, "/0/private/EditOrder", s.params).WithSign())
}

// EditOrderResult is the EditOrder response.
type EditOrderResult struct {
	Status          string          `json:"status"`           // Ok or Err
	TxID            string          `json:"txid"`             // new transaction id
	OriginalTxID    string          `json:"originaltxid"`     // original transaction id
	OrdersCancelled int             `json:"orders_cancelled"` // number of orders cancelled (0 or 1)
	Description     OrderDescr      `json:"descr"`            // new order description
	NewUserRef      string          `json:"newuserref"`       // new user reference
	OldUserRef      string          `json:"olduserref"`       // original user reference
	Volume          decimal.Decimal `json:"volume"`           // updated volume
	Price           decimal.Decimal `json:"price"`            // updated price
	Price2          decimal.Decimal `json:"price2"`           // updated secondary price
	ErrorMessage    string          `json:"error_message"`    // error detail, if unsuccessful
}

// ===========================================================================
// 5. Cancel Order -- POST /0/private/CancelOrder
// ===========================================================================

// CancelOrderService cancels a single open order by txid (or userref).
type CancelOrderService struct {
	c      *Client
	params map[string]string
}

// NewCancelOrderService cancels the order with the given txid. A numeric userref
// string cancels all orders sharing that reference.
func (c *Client) NewCancelOrderService(txid string) *CancelOrderService {
	return &CancelOrderService{c: c, params: map[string]string{"txid": txid}}
}

// SetClientOrderID cancels by client order id instead of txid.
func (s *CancelOrderService) SetClientOrderID(clOrdID string) *CancelOrderService {
	s.params["cl_ord_id"] = clOrdID
	return s
}

func (s *CancelOrderService) Do(ctx context.Context) (*CancelOrderResult, error) {
	return request.Do[CancelOrderResult](request.Post(ctx, s.c, "/0/private/CancelOrder", s.params).WithSign())
}

// CancelOrderResult is the CancelOrder response.
type CancelOrderResult struct {
	Count   int  `json:"count"`   // number of orders cancelled
	Pending bool `json:"pending"` // whether cancellation is still pending
}

// ===========================================================================
// 6. Cancel All Orders -- POST /0/private/CancelAll
// ===========================================================================

// CancelAllOrdersService cancels all open orders.
type CancelAllOrdersService struct {
	c *Client
}

func (c *Client) NewCancelAllOrdersService() *CancelAllOrdersService {
	return &CancelAllOrdersService{c: c}
}

func (s *CancelAllOrdersService) Do(ctx context.Context) (*CancelAllOrdersResult, error) {
	return request.Do[CancelAllOrdersResult](request.Post(ctx, s.c, "/0/private/CancelAll").WithSign())
}

// CancelAllOrdersResult is the CancelAll response.
type CancelAllOrdersResult struct {
	Count int `json:"count"` // number of orders cancelled
}

// ===========================================================================
// 7. Cancel All Orders After X -- POST /0/private/CancelAllOrdersAfter
// ===========================================================================

// CancelAllOrdersAfterService arms (or disarms, with timeout 0) a dead-man's
// switch that cancels all open orders after the given number of seconds unless
// the timer is reset.
type CancelAllOrdersAfterService struct {
	c      *Client
	params map[string]string
}

// NewCancelAllOrdersAfterService sets the timer to timeout seconds (0 disables).
func (c *Client) NewCancelAllOrdersAfterService(timeout int) *CancelAllOrdersAfterService {
	return &CancelAllOrdersAfterService{c: c, params: map[string]string{
		"timeout": strconv.Itoa(timeout),
	}}
}

func (s *CancelAllOrdersAfterService) Do(ctx context.Context) (*CancelAllOrdersAfterResult, error) {
	return request.Do[CancelAllOrdersAfterResult](request.Post(ctx, s.c, "/0/private/CancelAllOrdersAfter", s.params).WithSign())
}

// CancelAllOrdersAfterResult is the CancelAllOrdersAfter response.
type CancelAllOrdersAfterResult struct {
	CurrentTime time.Time `json:"currentTime"` // when the request was received (RFC3339)
	TriggerTime time.Time `json:"triggerTime"` // when orders will be cancelled (RFC3339)
}

// ===========================================================================
// 8. Cancel Order Batch -- POST /0/private/CancelOrderBatch
// ===========================================================================

// CancelOrderBatchService cancels up to 50 open orders by txid (or userref) in
// one request.
type CancelOrderBatchService struct {
	c     *Client
	txids []string
}

// NewCancelOrderBatchService cancels the given order txids (or userref strings).
func (c *Client) NewCancelOrderBatchService(txids ...string) *CancelOrderBatchService {
	return &CancelOrderBatchService{c: c, txids: txids}
}

func (s *CancelOrderBatchService) Do(ctx context.Context) (*CancelOrderBatchResult, error) {
	params := map[string]string{}
	for i, txid := range s.txids {
		params[fmt.Sprintf("orders[%d]", i)] = txid
	}
	return request.Do[CancelOrderBatchResult](request.Post(ctx, s.c, "/0/private/CancelOrderBatch", params).WithSign())
}

// CancelOrderBatchResult is the CancelOrderBatch response.
type CancelOrderBatchResult struct {
	Count int `json:"count"` // number of orders cancelled
}
