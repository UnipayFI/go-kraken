package kraken

import (
	"strconv"
	"strings"
	"time"

	"github.com/UnipayFI/go-kraken/common"
	"github.com/shopspring/decimal"
)

// MethodLimit is the "limit" field of a deposit/withdrawal method: Kraken
// encodes it as either the boolean false (no limit) or a maximum amount. It is
// a small union type so a plain field can decode both forms.
type MethodLimit struct {
	HasLimit bool            // whether a limit applies
	Amount   decimal.Decimal // the limit amount, when HasLimit and a number was returned
}

func (m *MethodLimit) UnmarshalJSON(data []byte) error {
	switch strings.TrimSpace(string(data)) {
	case "false", "null":
		m.HasLimit, m.Amount = false, decimal.Zero
		return nil
	case "true":
		m.HasLimit, m.Amount = true, decimal.Zero
		return nil
	}
	var d decimal.Decimal
	if err := common.JSONUnmarshal(data, &d); err != nil {
		return err
	}
	m.HasLimit, m.Amount = true, d
	return nil
}

func (m MethodLimit) MarshalJSON() ([]byte, error) {
	if !m.HasLimit {
		return []byte("false"), nil
	}
	if m.Amount.IsZero() {
		return []byte("true"), nil
	}
	return []byte(`"` + m.Amount.String() + `"`), nil
}

// OrderSide is the direction of an order.
type OrderSide string

const (
	OrderSideBuy  OrderSide = "buy"
	OrderSideSell OrderSide = "sell"
)

// OrderType is the execution model of an order.
type OrderType string

const (
	OrderTypeMarket            OrderType = "market"
	OrderTypeLimit             OrderType = "limit"
	OrderTypeIceberg           OrderType = "iceberg"
	OrderTypeStopLoss          OrderType = "stop-loss"
	OrderTypeTakeProfit        OrderType = "take-profit"
	OrderTypeStopLossLimit     OrderType = "stop-loss-limit"
	OrderTypeTakeProfitLimit   OrderType = "take-profit-limit"
	OrderTypeTrailingStop      OrderType = "trailing-stop"
	OrderTypeTrailingStopLimit OrderType = "trailing-stop-limit"
	OrderTypeSettlePosition    OrderType = "settle-position"
)

// TimeInForce controls how long an order remains active.
type TimeInForce string

const (
	TimeInForceGTC TimeInForce = "GTC" // good till canceled
	TimeInForceIOC TimeInForce = "IOC" // immediate or cancel
	TimeInForceGTD TimeInForce = "GTD" // good till date
	TimeInForceFOK TimeInForce = "FOK" // fill or kill
)

// SelfTradePrevention selects how self-trades are prevented (stptype).
type SelfTradePrevention string

const (
	SelfTradePreventionCancelNewest SelfTradePrevention = "cancel-newest"
	SelfTradePreventionCancelOldest SelfTradePrevention = "cancel-oldest"
	SelfTradePreventionCancelBoth   SelfTradePrevention = "cancel-both"
)

// TriggerSignal selects the price signal for stop/take-profit triggers.
type TriggerSignal string

const (
	TriggerSignalLast  TriggerSignal = "last"
	TriggerSignalIndex TriggerSignal = "index"
)

// Order flags (oflags) — combine as a comma-delimited string.
const (
	OrderFlagPost  = "post"  // post-only (limit orders only)
	OrderFlagFCIB  = "fcib"  // prefer fee in base currency
	OrderFlagFCIQ  = "fciq"  // prefer fee in quote currency
	OrderFlagNOMPP = "nompp" // disable market price protection
	OrderFlagVIQC  = "viqc"  // order volume expressed in quote currency
)

// NanoTime is a timestamp Kraken encodes as UNIX *nanoseconds* — used by a few
// newer endpoints (e.g. order amends) — as opposed to the UNIX seconds used by
// the rest of the API and handled by the global time.Time codec. It is a
// distinct type so the seconds-based codec does not misread it.
type NanoTime time.Time

func (t *NanoTime) UnmarshalJSON(data []byte) error {
	// Parse the raw token directly as a 64-bit integer: a nanosecond epoch has
	// up to 19 digits, which exceeds float64's ~16 significant digits, so going
	// through float64 (or any) would silently lose precision.
	s := strings.Trim(strings.TrimSpace(string(data)), `"`)
	if s == "" || s == "0" || s == "null" {
		*t = NanoTime(time.Time{})
		return nil
	}
	ns, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		f, ferr := strconv.ParseFloat(s, 64)
		if ferr != nil {
			return err
		}
		ns = int64(f)
	}
	if ns == 0 {
		*t = NanoTime(time.Time{})
		return nil
	}
	*t = NanoTime(time.Unix(0, ns).UTC())
	return nil
}

func (t NanoTime) MarshalJSON() ([]byte, error) {
	tt := time.Time(t)
	if tt.IsZero() {
		return []byte("0"), nil
	}
	return []byte(strconv.FormatInt(tt.UnixNano(), 10)), nil
}

// Time returns the timestamp as a standard time.Time.
func (t NanoTime) Time() time.Time { return time.Time(t) }

// formatBool / formatInt / formatTime are small helpers for building request
// params (which Kraken expects as form-encoded strings).
func formatBool(b bool) string { return strconv.FormatBool(b) }

func formatInt(i int) string { return strconv.Itoa(i) }

func formatUnix(t time.Time) string { return strconv.FormatInt(t.Unix(), 10) }
