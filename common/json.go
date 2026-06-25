package common

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/go-json-experiment/json"
	"github.com/go-json-experiment/json/jsontext"
	"github.com/shopspring/decimal"
)

// Kraken encodes most numbers as JSON strings (prices "30000.00000", volumes
// "1.2500"), occasionally as bare numbers, and timestamps as UNIX *seconds* —
// sometimes a bare integer (server unixtime, OHLC bucket), sometimes a float
// with sub-second precision ("opentm": 1688669448.7475), sometimes a quoted
// string. The stock time.Time / shopspring decimal codecs reject these forms,
// so we teach the JSON codec how to read/write both types once, globally. Every
// time.Time / decimal.Decimal field in this SDK is therefore a plain field with
// a plain json tag, and the conversions below apply.
//
// NOTE: time is UNIX SECONDS here (Kraken), not milliseconds — this is the key
// difference from millisecond-based exchanges.
var (
	unmarshalers = json.WithUnmarshalers(json.JoinUnmarshalers(
		json.UnmarshalFromFunc(decodeUnixTime),
		json.UnmarshalFromFunc(decodeDecimal),
	))
	marshalers = json.WithMarshalers(json.JoinMarshalers(
		json.MarshalToFunc(encodeUnixTime),
		json.MarshalToFunc(encodeDecimal),
	))
)

// JSONMarshal marshals v with Kraken's unix-seconds-time and decimal-string
// conventions applied.
func JSONMarshal(v any) ([]byte, error) {
	return json.Marshal(v, marshalers)
}

// JSONUnmarshal unmarshals data into v with Kraken's unix-seconds-time and
// decimal-string conventions applied.
func JSONUnmarshal(data []byte, v any) error {
	return json.Unmarshal(data, v, unmarshalers)
}

// decodeUnixTime reads a UNIX-seconds timestamp (int, float, or quoted string)
// into a time.Time, preserving sub-second precision.
func decodeUnixTime(dec *jsontext.Decoder, t *time.Time) error {
	tok, err := dec.ReadToken()
	if err != nil {
		return err
	}
	var s string
	switch tok.Kind() {
	case 'n': // null
		*t = time.Time{}
		return nil
	case '"': // quoted string
		s = tok.String()
	case '0': // bare number
		s = tok.String()
	default:
		return fmt.Errorf("kraken: cannot decode %v token into time.Time", tok.Kind())
	}
	s = strings.TrimSpace(s)
	switch s {
	case "", "0": // "not set" sentinels
		*t = time.Time{}
		return nil
	}
	// Most Kraken timestamps are UNIX seconds (int or float). A few endpoints
	// (SystemStatus, export reports, some funding records) instead use an
	// RFC3339 string, so fall back to that when the value is not numeric.
	if secs, perr := strconv.ParseFloat(s, 64); perr == nil {
		sec := int64(secs)
		nsec := int64(math.Round((secs - float64(sec)) * 1e9))
		*t = time.Unix(sec, nsec).UTC()
		return nil
	}
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339} {
		if parsed, perr := time.Parse(layout, s); perr == nil {
			*t = parsed.UTC()
			return nil
		}
	}
	return fmt.Errorf("kraken: invalid timestamp %q (not unix-seconds or RFC3339)", s)
}

// encodeUnixTime re-emits a time.Time as UNIX seconds (integer when whole,
// float when sub-second), matching the wire format Kraken sends. The zero time
// becomes 0.
func encodeUnixTime(enc *jsontext.Encoder, t time.Time) error {
	if t.IsZero() {
		return enc.WriteToken(jsontext.Int(0))
	}
	if t.Nanosecond() == 0 {
		return enc.WriteToken(jsontext.Int(t.Unix()))
	}
	secs := float64(t.UnixNano()) / 1e9
	return enc.WriteToken(jsontext.Float(secs))
}

// decodeDecimal reads a decimal from a JSON string or bare number; "" and null
// decode to zero.
func decodeDecimal(dec *jsontext.Decoder, d *decimal.Decimal) error {
	tok, err := dec.ReadToken()
	if err != nil {
		return err
	}
	var s string
	switch tok.Kind() {
	case 'n': // null
		*d = decimal.Zero
		return nil
	case '"': // quoted string
		s = tok.String()
	case '0': // bare number
		s = tok.String()
	default:
		return fmt.Errorf("kraken: cannot decode %v token into decimal", tok.Kind())
	}
	if s == "" {
		*d = decimal.Zero
		return nil
	}
	v, err := decimal.NewFromString(s)
	if err != nil {
		return fmt.Errorf("kraken: invalid decimal %q: %w", s, err)
	}
	*d = v
	return nil
}

// encodeDecimal re-emits a decimal as the quoted string form Kraken uses.
func encodeDecimal(enc *jsontext.Encoder, d decimal.Decimal) error {
	return enc.WriteToken(jsontext.String(d.String()))
}
