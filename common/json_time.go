package common

import (
	"bytes"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"time"
	"unsafe"
)

// timeFormat is the parsed `format` tag option of a time.Time field.
type timeFormat struct {
	pow10   uint64 // unix formats: 1e0, 1e3, 1e6 or 1e9 units per second
	rfc3339 bool   // RFC3339 or RFC3339Nano, which get extra validation
	layout  string // other formats: the time.Parse layout
}

// parseTimeFormat mirrors encoding/json/v2's interpretation of a time.Time
// `format` value. ok is false for formats this codec leaves to the standard
// library: no format (RFC 3339 default) and invalid values (reported there).
func parseTimeFormat(format string) (f timeFormat, ok bool) {
	if format == "" {
		return f, false
	}
	// We assume that an exported constant in the time package will
	// always start with an uppercase ASCII letter.
	if c := format[0]; !('a' <= c && c <= 'z') && !('A' <= c && c <= 'Z') {
		return timeFormat{layout: format}, true
	}
	switch format {
	case "unix":
		return timeFormat{pow10: 1e0}, true
	case "unixmilli":
		return timeFormat{pow10: 1e3}, true
	case "unixmicro":
		return timeFormat{pow10: 1e6}, true
	case "unixnano":
		return timeFormat{pow10: 1e9}, true
	case "RFC3339":
		return timeFormat{rfc3339: true, layout: time.RFC3339}, true
	case "RFC3339Nano":
		return timeFormat{rfc3339: true, layout: time.RFC3339Nano}, true
	case "ANSIC":
		return timeFormat{layout: time.ANSIC}, true
	case "UnixDate":
		return timeFormat{layout: time.UnixDate}, true
	case "RubyDate":
		return timeFormat{layout: time.RubyDate}, true
	case "RFC822":
		return timeFormat{layout: time.RFC822}, true
	case "RFC822Z":
		return timeFormat{layout: time.RFC822Z}, true
	case "RFC850":
		return timeFormat{layout: time.RFC850}, true
	case "RFC1123":
		return timeFormat{layout: time.RFC1123}, true
	case "RFC1123Z":
		return timeFormat{layout: time.RFC1123Z}, true
	case "Kitchen":
		return timeFormat{layout: time.Kitchen}, true
	case "Stamp":
		return timeFormat{layout: time.Stamp}, true
	case "StampMilli":
		return timeFormat{layout: time.StampMilli}, true
	case "StampMicro":
		return timeFormat{layout: time.StampMicro}, true
	case "StampNano":
		return timeFormat{layout: time.StampNano}, true
	case "DateTime":
		return timeFormat{layout: time.DateTime}, true
	case "DateOnly":
		return timeFormat{layout: time.DateOnly}, true
	case "TimeOnly":
		return timeFormat{layout: time.TimeOnly}, true
	}
	// Reject any Go identifier in case new constants are supported.
	if strings.TrimFunc(format, isLetterOrDigit) == "" {
		return f, false
	}
	return timeFormat{layout: format}, true
}

func isLetterOrDigit(r rune) bool {
	return r == '_' || ('a' <= r && r <= 'z') || ('A' <= r && r <= 'Z') || ('0' <= r && r <= '9')
}

// decodeTime decodes a time.Time field that declares a `format`, accepting a
// unix value quoted or bare and mapping Kraken's "not set" sentinels to the
// zero time. Normal values decode exactly as the standard format decodes them;
// fields without a format are left to the standard library.
func decodeTime(dec *jsontext.Decoder, t *time.Time) error {
	f, ok := parseTimeFormat(formatOf(dec.Options()))
	if !ok {
		return errors.ErrUnsupported
	}
	val, err := dec.ReadValue()
	if err != nil {
		return err
	}
	tt, err := f.decode(val)
	if err != nil {
		return err
	}
	*t = tt
	return nil
}

// decode parses a single JSON value in format f: null, "", "0" and 0 are the
// zero time; unix values may be quoted or bare; everything else must be a
// string in the format's layout.
func (f timeFormat) decode(val jsontext.Value) (time.Time, error) {
	var b []byte
	switch val.Kind() {
	case 'n': // null
		return time.Time{}, nil
	case '"': // quoted string
		var err error
		if b, err = unquote(val); err != nil {
			return time.Time{}, err
		}
	case '0': // bare number
		b = val
	default:
		return time.Time{}, fmt.Errorf("kraken: cannot decode %v value into time", val.Kind())
	}
	switch string(b) {
	case "", "0": // "not set" sentinels
		return time.Time{}, nil
	}
	switch {
	case f.pow10 != 0:
		return parseTimeUnix(b, f.pow10)
	case val.Kind() == '0':
		return time.Time{}, fmt.Errorf("kraken: cannot decode JSON number %s into %q time", b, f.layout)
	case f.rfc3339:
		return parseTimeRFC3339(b)
	default:
		return time.Parse(f.layout, string(b))
	}
}

// encodeTime writes the zero time of a field that declares a `format` as
// Kraken's "not set" form, which decodeTime reads back as the zero time: 0 for
// unix formats (as in "starttm": 0) and "0" for string formats (as in
// CancelAllOrdersAfter's "triggerTime": "0"). Every other value is left to the
// standard library, which writes it in the field's format — unix values as
// bare, possibly fractional numbers, so Kraken's sub-second precision is kept
// rather than truncated.
func encodeTime(enc *jsontext.Encoder, t time.Time) error {
	if !t.IsZero() {
		return errors.ErrUnsupported
	}
	f, ok := parseTimeFormat(formatOf(enc.Options()))
	switch {
	case !ok:
		return errors.ErrUnsupported
	case f.pow10 != 0:
		return enc.WriteToken(jsontext.Int(0))
	default:
		return enc.WriteToken(jsontext.String("0"))
	}
}

// UnmarshalUnixTime decodes data, a single JSON value holding UNIX seconds, into
// t exactly as a `format:unix` field decodes: quoted or bare, possibly
// fractional, with null, "", "0" and 0 read as the zero time. It is for
// timestamps that have no struct field to carry the tag, such as the columns of
// Kraken's positional arrays (OHLC candles, trades, spreads, book levels).
func UnmarshalUnixTime(data []byte, t *time.Time) error {
	val := jsontext.Value(bytes.TrimSpace(data))
	if !val.IsValid() {
		return fmt.Errorf("kraken: invalid JSON timestamp %q", data)
	}
	tt, err := timeFormat{pow10: 1e0}.decode(val)
	if err != nil {
		return err
	}
	*t = tt
	return nil
}

// MarshalUnixTime is the inverse of UnmarshalUnixTime: t as a bare, possibly
// fractional number of UNIX seconds, or 0 for the zero time.
func MarshalUnixTime(t time.Time) jsontext.Value {
	if t.IsZero() {
		return jsontext.Value("0")
	}
	return appendTimeUnix(nil, t, 1e0)
}

// unquote returns the contents of a JSON string, avoiding a copy when it
// contains no escape sequences (timestamps never do).
func unquote(val jsontext.Value) ([]byte, error) {
	if b := val[1 : len(val)-1]; bytes.IndexByte(b, '\\') < 0 {
		return b, nil
	}
	return jsontext.AppendUnquote(nil, val)
}

// formatOf returns the `format` tag option of the struct field currently
// being marshaled or unmarshaled, or "" if it has none. Pass it the Options
// of the Encoder/Decoder given to a MarshalToFunc/UnmarshalFromFunc; they are
// only valid for the duration of that call.
//
// encoding/json/v2 has no public accessor for the format, so this reads the
// Format field of the options struct those methods return (a
// *jsonopts.Struct in Go 1.27). errFormatOf reports when that layout is not
// what this code expects, in which case formatOf always returns "".
//
// Format is only reset between struct fields, so it must only be consulted for
// the value a format-tagged field holds directly (time.Time or *time.Time), not
// for values nested inside a format-tagged type with its own JSON methods.
func formatOf(opts json.Options) string {
	if errFormatOf != nil || reflect.TypeOf(opts) != optionsType {
		return ""
	}
	// An interface value is a (type, data) word pair; data points at the struct.
	p := (*[2]unsafe.Pointer)(unsafe.Pointer(&opts))[1]
	return *(*string)(unsafe.Add(p, formatOffset))
}

var (
	optionsType  = reflect.TypeOf(new(jsontext.Decoder).Options())
	formatOffset uintptr
	errFormatOf  = func() error {
		if enc := reflect.TypeOf(new(jsontext.Encoder).Options()); enc != optionsType {
			return fmt.Errorf("kraken: encoder options type %v differs from decoder options type %v", enc, optionsType)
		}
		if optionsType == nil || optionsType.Kind() != reflect.Pointer || optionsType.Elem().Kind() != reflect.Struct {
			return fmt.Errorf("kraken: unexpected encoding/json/v2 options type %v", optionsType)
		}
		sf, ok := optionsType.Elem().FieldByName("Format")
		if !ok || sf.Type.Kind() != reflect.String {
			return fmt.Errorf("kraken: encoding/json/v2 options type %v has no Format string field", optionsType)
		}
		// FieldByName reports the offset within the innermost embedded
		// struct; sum the offsets along the embedding path.
		st := optionsType.Elem()
		for _, i := range sf.Index {
			if st.Kind() != reflect.Struct {
				return fmt.Errorf("kraken: encoding/json/v2 options field Format is not embedded by value in %v", optionsType)
			}
			f := st.Field(i)
			formatOffset += f.Offset
			st = f.Type
		}
		return nil
	}()
)
