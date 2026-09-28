package common

import (
	"encoding/json/jsontext"
	"encoding/json/v2"
	"fmt"
	"time"

	jsonexp "github.com/go-json-experiment/json"
	"github.com/shopspring/decimal"
)

// Kraken encodes most numbers as JSON strings (prices "30000.00000", volumes
// "1.2500") and occasionally as bare numbers. Timestamps are UNIX *seconds* —
// a bare integer (server unixtime), a bare number with sub-second precision
// ("opentm": 1688669448.7475) or a quoted string ("createdtm": "1688669085"),
// with 0 / "0" / "" when "not set" — except on the newer surfaces (WebSocket
// v2, Earn, Transparency, system status), which send RFC 3339 strings.
//
// Every time.Time field declares its wire format with the standard `format`
// tag option (e.g. `json:"opentm,format:unix"`), which Go 1.27's
// encoding/json/v2 only honours when ExperimentalSupportFormatTag is set. The
// time codec below keeps the standard semantics of that format and only adds
// Kraken's quirks on top: quoted-or-bare numbers and the "not set" sentinels.
// decimal.Decimal fields stay plain fields with a plain json tag.
var (
	unmarshalOptions json.Options
	marshalOptions   json.Options

	// errJSONSupport is non-nil when the running Go release no longer
	// provides the `format` tag hooks this codec relies on.
	errJSONSupport = initJSON()
)

// JSONMarshal marshals v with Kraken's time and decimal conventions applied.
func JSONMarshal(v any) ([]byte, error) {
	if errJSONSupport != nil {
		return nil, errJSONSupport
	}
	return json.Marshal(v, marshalOptions)
}

// JSONUnmarshal unmarshals data into v with Kraken's time and decimal
// conventions applied.
func JSONUnmarshal(data []byte, v any) error {
	if errJSONSupport != nil {
		return errJSONSupport
	}
	return json.Unmarshal(data, v, unmarshalOptions)
}

// initJSON builds the codec options and round-trips a probe through them, so
// a Go release that drops the experimental `format` tag support (by panicking
// on the unknown option or by ignoring it) or changes the options layout
// formatOf reads fails loudly instead of silently misdating fields.
func initJSON() (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("kraken: encoding/json/v2 format tag support unavailable: %v", r)
		}
	}()
	if errFormatOf != nil {
		return errFormatOf
	}
	formatTag := jsonexp.ExperimentalSupportFormatTag(true)
	unmarshalOptions = json.JoinOptions(formatTag, json.WithUnmarshalers(json.JoinUnmarshalers(
		json.UnmarshalFromFunc(decodeTime),
		json.UnmarshalFromFunc(decodeDecimal),
	)))
	marshalOptions = json.JoinOptions(formatTag, json.WithMarshalers(json.JoinMarshalers(
		json.MarshalToFunc(encodeTime),
		json.MarshalToFunc(encodeDecimal),
	)))

	// t exercises the standard unix format (exact sub-second decimal); z the
	// "not set" sentinel, which only decodes to the zero time (and encodes
	// back to 0) when formatOf sees the field's format.
	const payload = `{"t":1688669448.7475,"z":0}`
	var probe struct {
		T time.Time `json:"t,format:unix"`
		Z time.Time `json:"z,format:unix"`
	}
	if err := json.Unmarshal([]byte(payload), &probe, unmarshalOptions); err != nil {
		return fmt.Errorf("kraken: encoding/json/v2 format tag support unavailable: %w", err)
	}
	if got := probe.T.UnixNano(); got != 1688669448747500000 || !probe.Z.IsZero() {
		return fmt.Errorf("kraken: encoding/json/v2 format tag probe decoded %d and %v, want 1688669448747500000 and the zero time", got, probe.Z)
	}
	if out, err := json.Marshal(probe, marshalOptions); err != nil || string(out) != payload {
		return fmt.Errorf("kraken: encoding/json/v2 format tag probe encoded %s (%v), want %s", out, err, payload)
	}
	return nil
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
