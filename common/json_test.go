package common

import (
	"encoding/json/jsontext"
	"encoding/json/v2"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	jsonexp "github.com/go-json-experiment/json"
	"github.com/shopspring/decimal"
)

func TestJSONSupport(t *testing.T) {
	if errJSONSupport != nil {
		t.Fatal(errJSONSupport)
	}
}

// TestUnixFormats pins each unix unit, and checks the units decode side by
// side without one being scaled into another. Kraken sends bare numbers, with
// sub-second digits on unix seconds.
func TestUnixFormats(t *testing.T) {
	var v struct {
		S  time.Time `json:"s,format:unix"`
		MS time.Time `json:"ms,format:unixmilli"`
		US time.Time `json:"us,format:unixmicro"`
		NS time.Time `json:"ns,format:unixnano"`
	}
	const payload = `{"s":1688669448.7475,"ms":1750034397008,"us":1750034396998123,"ns":1750034396998123456}`
	if err := JSONUnmarshal([]byte(payload), &v); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got := v.S.UnixNano(); got != 1688669448747500000 {
		t.Errorf("s = %d, want 1688669448747500000", got)
	}
	if got := v.MS.UnixMilli(); got != 1750034397008 {
		t.Errorf("ms = %d, want 1750034397008", got)
	}
	if got := v.US.UnixMicro(); got != 1750034396998123 {
		t.Errorf("us = %d, want 1750034396998123", got)
	}
	if got := v.NS.UnixNano(); got != 1750034396998123456 {
		t.Errorf("ns = %d, want 1750034396998123456", got)
	}
	if got := v.NS.Sub(v.US); got != 456*time.Nanosecond {
		t.Errorf("ns-us = %v, want 456ns", got)
	}
	for _, tt := range []time.Time{v.S, v.MS, v.US, v.NS} {
		if tt.Location() != time.UTC {
			t.Errorf("%v: location %v, want UTC", tt, tt.Location())
		}
	}
	out, err := JSONMarshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if string(out) != payload {
		t.Errorf("marshal round-trip = %s, want %s", out, payload)
	}
	// Quoted unix values decode to the same instants.
	quoted := `{"s":"1688669448.7475","ms":"1750034397008","us":"1750034396998123","ns":"1750034396998123456"}`
	w := v
	if err := JSONUnmarshal([]byte(quoted), &w); err != nil {
		t.Fatalf("unmarshal quoted: %v", err)
	}
	if w != v {
		t.Errorf("quoted = %+v, want %+v", w, v)
	}
}

// TestUnixMatchesStandard checks that normal values decode exactly like the
// standard `format` option (same instant, same UTC location), whether Kraken
// sends them quoted or bare.
func TestUnixMatchesStandard(t *testing.T) {
	std := jsonexp.ExperimentalSupportFormatTag(true)
	compared := 0
	defer func() {
		if compared < 40 {
			t.Errorf("only %d successful comparisons", compared)
		}
	}()
	for _, format := range []string{"unix", "unixmilli", "unixmicro", "unixnano"} {
		for _, n := range []string{
			"1790567886", "1790567886.4358602", "1688669448.7475", "1705259439.563000", "1733920121.745114",
			"1750034397008", "1790567889380592320", "1", "-1500", "12.000001", "0.5", "999999999999",
			"9223372036854775807", "1.0000000001", "01", "1e9", "+1", "1.", ".5", "-0",
		} {
			var want, got1, got2 time.Time
			wantErr := unmarshalWithTag(std, format, n, &want)
			err1 := unmarshalWithTag(nil, format, n, &got1)
			err2 := unmarshalWithTag(nil, format, `"`+n+`"`, &got2)
			if (wantErr != nil) != (err1 != nil) || (wantErr != nil) != (err2 != nil) {
				t.Errorf("%s %s: errors differ: std=%v bare=%v quoted=%v", format, n, wantErr, err1, err2)
				continue
			}
			if wantErr != nil {
				continue
			}
			compared++
			if want != got1 || want != got2 {
				t.Errorf("%s %s: std=%v bare=%v quoted=%v", format, n, want, got1, got2)
			}
		}
	}
}

// TestRFC3339MatchesStandard checks that RFC 3339 strings decode exactly like
// the standard RFC3339/RFC3339Nano formats, including the extra grammar checks.
func TestRFC3339MatchesStandard(t *testing.T) {
	std := jsonexp.ExperimentalSupportFormatTag(true)
	compared := 0
	for _, format := range []string{"RFC3339", "RFC3339Nano"} {
		for _, s := range []string{
			"2026-09-28T03:58:07Z", "2026-09-28T03:58:11.017365576Z", "2026-09-28T03:49:00.000000000Z",
			"2026-09-28T03:58:14.714753Z", "2026-09-28T11:58:07+08:00", "2026-09-28T03:58:07.5-05:30",
			"2026-09-28T3:58:07Z", "2026-09-28T03:58:07,5Z", "2026-09-28T03:58:07+24:00", "2026-09-28T03:58:07+01:60",
			"2026-09-28 03:58:07Z", "2026-09-28", "1790567886", "",
		} {
			var want, got time.Time
			wantErr := unmarshalWithTag(std, format, `"`+s+`"`, &want)
			err := unmarshalWithTag(nil, format, `"`+s+`"`, &got)
			if s == "" {
				// "" is a Kraken "not set" sentinel; the standard library rejects it.
				if err != nil || !got.IsZero() {
					t.Errorf("%s %q: got %v, %v; want the zero time", format, s, got, err)
				}
				continue
			}
			if (wantErr != nil) != (err != nil) {
				t.Errorf("%s %q: errors differ: std=%v kraken=%v", format, s, wantErr, err)
				continue
			}
			if wantErr != nil {
				continue
			}
			compared++
			wn, wo := want.Zone()
			gn, gdo := got.Zone()
			if !want.Equal(got) || wn != gn || wo != gdo {
				t.Errorf("%s %q: std=%v kraken=%v", format, s, want, got)
			}
		}
	}
	if compared < 10 {
		t.Errorf("only %d successful comparisons", compared)
	}
}

// unmarshalWithTag decodes {"t":raw} into a field tagged with format, using
// the standard library alone when std is set and the Kraken codec otherwise.
func unmarshalWithTag(std json.Options, format, raw string, dst *time.Time) error {
	in := []byte(`{"t":` + raw + `}`)
	var err error
	switch format {
	case "unix":
		var v struct {
			T time.Time `json:"t,format:unix"`
		}
		err = decodeWith(std, in, &v)
		*dst = v.T
	case "unixmilli":
		var v struct {
			T time.Time `json:"t,format:unixmilli"`
		}
		err = decodeWith(std, in, &v)
		*dst = v.T
	case "unixmicro":
		var v struct {
			T time.Time `json:"t,format:unixmicro"`
		}
		err = decodeWith(std, in, &v)
		*dst = v.T
	case "unixnano":
		var v struct {
			T time.Time `json:"t,format:unixnano"`
		}
		err = decodeWith(std, in, &v)
		*dst = v.T
	case "RFC3339":
		var v struct {
			T time.Time `json:"t,format:RFC3339"`
		}
		err = decodeWith(std, in, &v)
		*dst = v.T
	case "RFC3339Nano":
		var v struct {
			T time.Time `json:"t,format:RFC3339Nano"`
		}
		err = decodeWith(std, in, &v)
		*dst = v.T
	default:
		panic(format)
	}
	return err
}

func decodeWith(std json.Options, in []byte, v any) error {
	if std != nil {
		return json.Unmarshal(in, v, std)
	}
	return JSONUnmarshal(in, v)
}

// TestTimeNotSet covers the "not set" forms Kraken emits for timestamps.
func TestTimeNotSet(t *testing.T) {
	for _, raw := range []string{`""`, `"0"`, `null`, `0`} {
		var v struct {
			S   time.Time  `json:"s,format:unix"`
			NS  time.Time  `json:"ns,format:unixnano"`
			R   time.Time  `json:"r,format:RFC3339"`
			RN  time.Time  `json:"rn,format:RFC3339Nano"`
			P   *time.Time `json:"p,format:unix"`
			PR  *time.Time `json:"pr,format:RFC3339"`
			Set time.Time  `json:"set,format:unix"`
		}
		v.Set = time.Unix(1, 0)
		in := fmt.Sprintf(`{"s":%[1]s,"ns":%[1]s,"r":%[1]s,"rn":%[1]s,"p":%[1]s,"pr":%[1]s,"set":%[1]s}`, raw)
		if err := JSONUnmarshal([]byte(in), &v); err != nil {
			t.Errorf("unmarshal %s: %v", raw, err)
			continue
		}
		if !v.S.IsZero() || !v.NS.IsZero() || !v.R.IsZero() || !v.RN.IsZero() || !v.Set.IsZero() ||
			(v.P != nil && !v.P.IsZero()) || (v.PR != nil && !v.PR.IsZero()) {
			t.Errorf("unmarshal %s = %+v, want zero times", raw, v)
		}
	}
	// Only the exact sentinels are "not set": other zero-valued numbers are
	// the Unix epoch, as in the standard library.
	var v struct {
		S time.Time `json:"s,format:unix"`
	}
	if err := JSONUnmarshal([]byte(`{"s":"0.0"}`), &v); err != nil || !v.S.Equal(time.Unix(0, 0)) {
		t.Errorf(`"0.0" = %v, %v; want the Unix epoch`, v.S, err)
	}
	// A number is never an RFC 3339 timestamp.
	var r struct {
		R time.Time `json:"r,format:RFC3339"`
	}
	if err := JSONUnmarshal([]byte(`{"r":1790567886}`), &r); err == nil {
		t.Errorf("bare number into RFC3339: want error")
	}
}

func TestLayoutFormats(t *testing.T) {
	var v struct {
		D  time.Time `json:"d,format:DateOnly"`
		DT time.Time `json:"dt,format:DateTime"`
		C  time.Time `json:"c,format:'2006/01/02'"`
		E  time.Time `json:"e,format:DateOnly"`
	}
	if err := JSONUnmarshal([]byte(`{"d":"2026-08-16","dt":"2026-08-16 09:30:00","c":"2026/08/16","e":""}`), &v); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	want := time.Date(2026, 8, 16, 0, 0, 0, 0, time.UTC)
	if !v.D.Equal(want) || !v.C.Equal(want) || !v.DT.Equal(want.Add(9*time.Hour+30*time.Minute)) || !v.E.IsZero() {
		t.Errorf("got %v %v %v %v", v.D, v.DT, v.C, v.E)
	}
	out, err := JSONMarshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	// Values encode exactly as the standard library does; the zero time as "0".
	if want := `{"d":"2026-08-16","dt":"2026-08-16 09:30:00","c":"2026/08/16","e":"0"}`; string(out) != want {
		t.Errorf("marshal = %s, want %s", out, want)
	}
	if err := JSONUnmarshal([]byte(`{"d":20260816}`), &v); err == nil {
		t.Errorf("bare number into DateOnly: want error")
	}
}

// TestStandardFallback checks that fields without a format keep the standard
// library's behaviour and errors.
func TestStandardFallback(t *testing.T) {
	var v struct {
		Plain time.Time `json:"plain"`
	}
	if err := JSONUnmarshal([]byte(`{"plain":"2026-08-16T09:30:00Z"}`), &v); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	for _, in := range []string{`{"plain":1790567886}`, `{"plain":"1790567886"}`, `{"plain":""}`, `{"plain":"0"}`} {
		if err := JSONUnmarshal([]byte(in), &v); err == nil {
			t.Errorf("untagged time.Time accepted %s; it must require an explicit format", in)
		}
	}
	var zero struct {
		Plain time.Time `json:"plain"`
	}
	if out, err := JSONMarshal(zero); err != nil || string(out) != `{"plain":"0001-01-01T00:00:00Z"}` {
		t.Errorf("untagged zero time = %s, %v", out, err)
	}
	var bad struct {
		T time.Time `json:"t,format:bogus"`
	}
	if err := JSONUnmarshal([]byte(`{"t":"1"}`), &bad); err == nil || !strings.Contains(err.Error(), "format") {
		t.Errorf("invalid format error = %v", err)
	}
	if _, err := JSONMarshal(bad); err == nil || !strings.Contains(err.Error(), "format") {
		t.Errorf("invalid format marshal error = %v", err)
	}
}

func TestOtherStandardFormats(t *testing.T) {
	var v struct {
		D time.Duration     `json:"d,format:units"`
		B []byte            `json:"b,format:hex"`
		F float64           `json:"f,format:nonfinite"`
		M map[string]string `json:"m,format:emitnull"`
		S []int             `json:"s,format:emitempty"`
	}
	const payload = `{"d":"1h30m0s","b":"0102","f":"NaN","m":null,"s":[]}`
	if err := JSONUnmarshal([]byte(payload), &v); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	v.S = nil
	out, err := JSONMarshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if string(out) != payload {
		t.Errorf("marshal = %s, want %s", out, payload)
	}
}

func TestFormatOfScope(t *testing.T) {
	type inner struct {
		T time.Time `json:"t"`
	}
	var v struct {
		P  *time.Time  `json:"p,format:unix"`
		In inner       `json:"in"`
		Ts []time.Time `json:"ts"`
		Z  time.Time   `json:"z"`
	}
	// The nested/element times carry no format, so they must fall back to
	// RFC 3339 rather than inherit the sibling field's unix.
	in := `{"p":1790567886.5,"in":{"t":"2026-08-16T00:00:00Z"},"ts":["2026-08-16T00:00:00Z"]}`
	if err := JSONUnmarshal([]byte(in), &v); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if v.P == nil || v.P.UnixMilli() != 1790567886500 {
		t.Errorf("p = %v", v.P)
	}
	out, err := JSONMarshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if want := `{"p":1790567886.5,"in":{"t":"2026-08-16T00:00:00Z"},"ts":["2026-08-16T00:00:00Z"],"z":"0001-01-01T00:00:00Z"}`; string(out) != want {
		t.Errorf("marshal = %s, want %s", out, want)
	}
}

func TestDecimalCodec(t *testing.T) {
	var v struct {
		Quoted decimal.Decimal `json:"quoted"`
		Bare   decimal.Decimal `json:"bare"`
		Empty  decimal.Decimal `json:"empty"`
		Null   decimal.Decimal `json:"null"`
	}
	if err := JSONUnmarshal([]byte(`{"quoted":"30000.00000","bare":0.001,"empty":"","null":null}`), &v); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if v.Quoted.String() != "30000" || v.Bare.String() != "0.001" || !v.Empty.IsZero() || !v.Null.IsZero() {
		t.Errorf("got %v %v %v %v", v.Quoted, v.Bare, v.Empty, v.Null)
	}
	out, err := JSONMarshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if want := `{"quoted":"30000","bare":"0.001","empty":"0","null":"0"}`; string(out) != want {
		t.Errorf("marshal = %s, want %s", out, want)
	}
}

// TestEncodeKeepsPrecision checks that unix values are written exactly —
// Kraken sends sub-second precision, so nothing is truncated — and that the
// zero time is written as Kraken's "not set" form: 0 for unix formats and "0"
// for RFC 3339 ones.
func TestEncodeKeepsPrecision(t *testing.T) {
	var v struct {
		S    time.Time  `json:"s,format:unix"`
		NS   time.Time  `json:"ns,format:unixnano"`
		Neg  time.Time  `json:"neg,format:unix"`
		Zero time.Time  `json:"zero,format:unix"`
		ZNS  time.Time  `json:"zns,format:unixnano"`
		P    *time.Time `json:"p,format:unix"`
		R    time.Time  `json:"r,format:RFC3339Nano"`
		ZR   time.Time  `json:"zr,format:RFC3339"`
	}
	v.S = time.Unix(1790567886, 435860200)
	v.NS = time.Unix(1790567889, 380592320)
	v.Neg = time.Unix(-1, 999_999_999)
	v.P = &time.Time{}
	v.R = time.Date(2026, 9, 28, 3, 58, 11, 17365576, time.UTC)
	out, err := JSONMarshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if want := `{"s":1790567886.4358602,"ns":1790567889380592320,"neg":-0.000000001,"zero":0,"zns":0,"p":0,"r":"2026-09-28T03:58:11.017365576Z","zr":"0"}`; string(out) != want {
		t.Errorf("marshal = %s, want %s", out, want)
	}
	var back struct {
		S    time.Time  `json:"s,format:unix"`
		NS   time.Time  `json:"ns,format:unixnano"`
		Neg  time.Time  `json:"neg,format:unix"`
		Zero time.Time  `json:"zero,format:unix"`
		ZNS  time.Time  `json:"zns,format:unixnano"`
		P    *time.Time `json:"p,format:unix"`
		R    time.Time  `json:"r,format:RFC3339Nano"`
		ZR   time.Time  `json:"zr,format:RFC3339"`
	}
	if err := JSONUnmarshal(out, &back); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !back.S.Equal(v.S) || !back.NS.Equal(v.NS) || !back.Neg.Equal(v.Neg) || !back.Zero.IsZero() || !back.ZNS.IsZero() || back.P == nil || !back.P.IsZero() || back.R != v.R || !back.ZR.IsZero() {
		t.Errorf("round-trip = %+v", back)
	}
}

func TestUnixTimeHelpers(t *testing.T) {
	for in, want := range map[string]time.Time{
		`1790567886`:           time.Unix(1790567886, 0).UTC(),
		` 1790567886.4358602 `: time.Unix(1790567886, 435860200).UTC(),
		`"1790567886.4358602"`: time.Unix(1790567886, 435860200).UTC(),
		`0`:                    {},
		`"0"`:                  {},
		`""`:                   {},
		`null`:                 {},
		`"1705260547.4730000"`: time.Unix(1705260547, 473000000).UTC(),
		`1688671200`:           time.Unix(1688671200, 0).UTC(),
		`-1.5`:                 time.Unix(-2, 500000000).UTC(),
	} {
		var got time.Time
		if err := UnmarshalUnixTime([]byte(in), &got); err != nil {
			t.Errorf("UnmarshalUnixTime(%s): %v", in, err)
			continue
		}
		if got != want {
			t.Errorf("UnmarshalUnixTime(%s) = %v, want %v", in, got, want)
		}
	}
	// A trades "last" cursor (nanoseconds) is out of range as seconds.
	for _, in := range []string{``, `true`, `[1]`, `"abc"`, `1 2`, `"2026-09-28T03:58:07Z"`, `"1790567889380592320000"`} {
		var got time.Time
		if err := UnmarshalUnixTime([]byte(in), &got); err == nil {
			t.Errorf("UnmarshalUnixTime(%s) = %v, want error", in, got)
		}
	}
	for in, want := range map[time.Time]string{
		{}:                             `0`,
		time.Unix(1790567886, 0):       `1790567886`,
		time.Unix(1790567886, 4358602): `1790567886.004358602`,
		time.Unix(1688669448, 7475e5):  `1688669448.7475`,
	} {
		if got := string(MarshalUnixTime(in)); got != want {
			t.Errorf("MarshalUnixTime(%v) = %s, want %s", in, got, want)
		}
	}
}

// TestConcurrentFormats decodes and encodes differently-formatted fields from
// many goroutines at once; run with -race. formatOf reads per-call state, so a
// field must never see another field's format.
func TestConcurrentFormats(t *testing.T) {
	type rec struct {
		S  time.Time  `json:"s,format:unix"`
		NS time.Time  `json:"ns,format:unixnano"`
		R  time.Time  `json:"r,format:RFC3339Nano"`
		P  *time.Time `json:"p,format:unix"`
		Z  time.Time  `json:"z,format:unix"`
	}
	const payload = `{"s":1790567886.4358602,"ns":1790567889380592320,"r":"2026-09-28T03:58:11.017365576Z","p":1688669448.7475,"z":0}`
	var want rec
	if err := JSONUnmarshal([]byte(payload), &want); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for range 16 {
		wg.Go(func() {
			for range 500 {
				var got rec
				if err := JSONUnmarshal([]byte(payload), &got); err != nil {
					t.Error(err)
					return
				}
				if got.S != want.S || got.NS != want.NS || got.R != want.R || !got.P.Equal(*want.P) || !got.Z.IsZero() {
					t.Errorf("got %+v, want %+v", got, want)
					return
				}
				out, err := JSONMarshal(got)
				if err != nil || string(out) != payload {
					t.Errorf("marshal = %s, %v; want %s", out, err, payload)
					return
				}
			}
		})
	}
	wg.Wait()
}

// stamps is a user type with its own v2 JSON methods that (un)marshals the
// time.Time elements of an array itself.
type stamps []time.Time

func (s *stamps) UnmarshalJSONFrom(dec *jsontext.Decoder) error {
	if _, err := dec.ReadToken(); err != nil {
		return err
	}
	for dec.PeekKind() != ']' {
		var t time.Time
		if err := json.UnmarshalDecode(dec, &t); err != nil {
			return err
		}
		*s = append(*s, t)
	}
	_, err := dec.ReadToken()
	return err
}

func (s stamps) MarshalJSONTo(enc *jsontext.Encoder) error {
	if err := enc.WriteToken(jsontext.BeginArray); err != nil {
		return err
	}
	for _, t := range s {
		if err := json.MarshalEncode(enc, t); err != nil {
			return err
		}
	}
	return enc.WriteToken(jsontext.EndArray)
}

// TestFormatNotInherited checks that a time.Time nested below a format-tagged
// value does not inherit that format — the standard library clears the
// FormatTag flag (but not Format) on entering an array or object.
func TestFormatNotInherited(t *testing.T) {
	std := jsonexp.ExperimentalSupportFormatTag(true)
	type inner struct {
		Plain time.Time `json:"plain"`
		U     time.Time `json:"u,format:unix"`
	}
	type outer struct {
		S stamps  `json:"s,format:unix"`
		L []inner `json:"l,format:emitnull"`
	}
	for _, in := range []string{
		`{"s":["2026-01-01T00:00:00Z"],"l":[{"plain":"2026-01-01T00:00:00Z","u":1790567886}]}`,
		`{"s":[1790567886]}`,
		`{"s":["0"]}`,
	} {
		var got, want outer
		err := JSONUnmarshal([]byte(in), &got)
		wantErr := json.Unmarshal([]byte(in), &want, std)
		if (err != nil) != (wantErr != nil) || !reflect.DeepEqual(got, want) {
			t.Errorf("%s: kraken %+v, %v; std %+v, %v", in, got, err, want, wantErr)
		}
	}
	v := outer{S: stamps{time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), {}}}
	out, err := JSONMarshal(v)
	stdOut, stdErr := json.Marshal(v, std)
	if string(out) != string(stdOut) || (err != nil) != (stdErr != nil) {
		t.Errorf("marshal = %s, %v; std %s, %v", out, err, stdOut, stdErr)
	}
}

// TestFieldOffset checks that an options layout fieldOffset does not expect
// is reported as an error, never a panic during package initialization.
func TestFieldOffset(t *testing.T) {
	type flags struct{ Presence, Values uint64 }
	type inner struct{ Format string }
	for name, typ := range map[string]reflect.Type{
		"pointer": reflect.TypeOf(struct{ Flags *flags }{}),
		"array":   reflect.TypeOf(struct{ Flags [2]uint64 }{}),
		"scalar":  reflect.TypeOf(struct{ Flags uint64 }{}),
		"missing": reflect.TypeOf(struct{ Other flags }{}),
		"kind":    reflect.TypeOf(struct{ Flags struct{ Presence uint32 } }{}),
	} {
		if _, err := fieldOffset(typ, reflect.Uint64, "Flags", "Presence"); err == nil {
			t.Errorf("%s: want error", name)
		}
	}
	typ := reflect.TypeOf(struct {
		Flags flags
		inner
	}{})
	if off, err := fieldOffset(typ, reflect.Uint64, "Flags", "Presence"); err != nil || off != 0 {
		t.Errorf("Flags.Presence = %d, %v; want 0", off, err)
	}
	if off, err := fieldOffset(typ, reflect.String, "Format"); err != nil || off != 16 {
		t.Errorf("Format = %d, %v; want 16", off, err)
	}
}
