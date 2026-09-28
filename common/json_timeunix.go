// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Unix and RFC 3339 timestamp parsing and formatting copied from Go 1.27's
// encoding/json/v2 (arshal_time.go) so that the unix* and RFC3339* formats
// decode and encode exactly as the standard `format` tag option does.

package common

import (
	"bytes"
	"fmt"
	"math"
	"strconv"
	"time"
)

// appendTimeUnix appends t formatted as a decimal fractional number,
// where pow10 is a power-of-10 used to scale up the number.
func appendTimeUnix(b []byte, t time.Time, pow10 uint64) []byte {
	sec, nsec := t.Unix(), int64(t.Nanosecond())
	if sec < 0 {
		b = append(b, '-')
		sec, nsec = negateSecNano(sec, nsec)
	}
	switch {
	case pow10 == 1e0: // fast case where units is in seconds
		b = strconv.AppendUint(b, uint64(sec), 10)
		return appendFracBase10(b, uint64(nsec), 1e9)
	case uint64(sec) < 1e9: // intermediate case where units is not seconds, but no overflow
		b = strconv.AppendUint(b, uint64(sec)*uint64(pow10)+uint64(uint64(nsec)/(1e9/pow10)), 10)
		return appendFracBase10(b, (uint64(nsec)*pow10)%1e9, 1e9)
	default: // slow case where units is not seconds and overflow would occur
		b = strconv.AppendUint(b, uint64(sec), 10)
		b = appendPaddedBase10(b, uint64(nsec)/(1e9/pow10), pow10)
		return appendFracBase10(b, (uint64(nsec)*pow10)%1e9, 1e9)
	}
}

// parseTimeUnix parses t formatted as a decimal fractional number,
// where pow10 is a power-of-10 used to scale down the number.
func parseTimeUnix(b []byte, pow10 uint64) (time.Time, error) {
	suffix, neg := consumeSign(b, false)                     // consume sign
	wholeBytes, fracBytes := bytesCutByte(suffix, '.', true) // consume whole and frac fields
	whole, okWhole := parseUint(wholeBytes)                  // parse whole field; may overflow
	frac, okFrac := parseFracBase10(fracBytes, 1e9/pow10)    // parse frac field
	var sec, nsec int64
	switch {
	case pow10 == 1e0: // fast case where units is in seconds
		sec = int64(whole) // check overflow later after negation
		nsec = int64(frac) // cannot overflow
	case okWhole: // intermediate case where units is not seconds, but no overflow
		sec = int64(whole / pow10)                     // check overflow later after negation
		nsec = int64((whole%pow10)*(1e9/pow10) + frac) // cannot overflow
	case !okWhole && whole == math.MaxUint64: // slow case where units is not seconds and overflow occurred
		width := int(math.Log10(float64(pow10)))                               // compute len(strconv.Itoa(pow10-1))
		whole, okWhole = parseUint(wholeBytes[:len(wholeBytes)-width])         // parse the upper whole field
		mid, _ := parsePaddedBase10(wholeBytes[len(wholeBytes)-width:], pow10) // parse the lower whole field
		sec = int64(whole)                                                     // check overflow later after negation
		nsec = int64(mid*(1e9/pow10) + frac)                                   // cannot overflow
	}
	if neg {
		sec, nsec = negateSecNano(sec, nsec)
	}
	switch t := time.Unix(sec, nsec).UTC(); {
	case (!okWhole && whole != math.MaxUint64) || !okFrac:
		return time.Time{}, fmt.Errorf("invalid time %q: %w", b, strconv.ErrSyntax)
	case !okWhole || neg != (t.Unix() < 0):
		return time.Time{}, fmt.Errorf("invalid time %q: %w", b, strconv.ErrRange)
	default:
		return t, nil
	}
}

// negateSecNano negates a Unix timestamp, where nsec must be within [0, 1e9).
func negateSecNano(sec, nsec int64) (int64, int64) {
	sec = ^sec               // twos-complement negation (i.e., -1*sec + 1)
	nsec = -nsec + 1e9       // negate nsec and add 1e9 (which is the extra +1 from sec negation)
	sec += int64(nsec / 1e9) // handle possible overflow of nsec if it started as zero
	nsec %= 1e9              // ensure nsec stays within [0, 1e9)
	return sec, nsec
}

// appendFracBase10 appends the fraction of n/max10,
// where max10 is a power-of-10 that is larger than n.
func appendFracBase10(b []byte, n, max10 uint64) []byte {
	if n == 0 {
		return b
	}
	return bytes.TrimRight(appendPaddedBase10(append(b, '.'), n, max10), "0")
}

// parseFracBase10 parses the fraction of n/max10,
// where max10 is a power-of-10 that is larger than n.
func parseFracBase10(b []byte, max10 uint64) (n uint64, ok bool) {
	switch {
	case len(b) == 0:
		return 0, true
	case len(b) < len(".0") || b[0] != '.':
		return 0, false
	}
	return parsePaddedBase10(b[len("."):], max10)
}

// appendPaddedBase10 appends a zero-padded encoding of n,
// where max10 is a power-of-10 that is larger than n.
func appendPaddedBase10(b []byte, n, max10 uint64) []byte {
	if n < max10/10 {
		// Formatting of n is shorter than log10(max10),
		// so add max10/10 to ensure the length is equal to log10(max10).
		i := len(b)
		b = strconv.AppendUint(b, n+max10/10, 10)
		b[i]-- // subtract the addition of max10/10
		return b
	}
	return strconv.AppendUint(b, n, 10)
}

// parsePaddedBase10 parses b as the zero-padded encoding of n,
// where max10 is a power-of-10 that is larger than n.
// Truncated suffix is treated as implicit zeros.
// Extended suffix is ignored, but verified to contain only digits.
func parsePaddedBase10(b []byte, max10 uint64) (n uint64, ok bool) {
	pow10 := uint64(1)
	for pow10 < max10 {
		n *= 10
		if len(b) > 0 {
			if b[0] < '0' || '9' < b[0] {
				return n, false
			}
			n += uint64(b[0] - '0')
			b = b[1:]
		}
		pow10 *= 10
	}
	if len(b) > 0 && len(bytes.TrimRight(b, "0123456789")) > 0 {
		return n, false // trailing characters are not digits
	}
	return n, true
}

// consumeSign consumes an optional leading negative or positive sign.
func consumeSign(b []byte, allowPlus bool) ([]byte, bool) {
	if len(b) > 0 {
		if b[0] == '-' {
			return b[len("-"):], true
		} else if b[0] == '+' && allowPlus {
			return b[len("+"):], false
		}
	}
	return b, false
}

// bytesCutByte is similar to bytes.Cut(b, []byte{c}),
// except c may optionally be included as part of the suffix.
func bytesCutByte(b []byte, c byte, include bool) ([]byte, []byte) {
	if i := bytes.IndexByte(b, c); i >= 0 {
		if include {
			return b[:i], b[i:]
		}
		return b[:i], b[i+1:]
	}
	return b, nil
}

// parseTimeRFC3339 parses b as an RFC 3339 timestamp: time.Time.UnmarshalText
// plus the extra grammar checks the standard library applies to the RFC3339
// and RFC3339Nano formats.
func parseTimeRFC3339(b []byte) (time.Time, error) {
	var t time.Time
	// Use time.Time.UnmarshalText to avoid possible string allocation.
	if err := t.UnmarshalText(b); err != nil {
		return time.Time{}, err
	}
	// TODO(https://go.dev/issue/57912):
	// RFC 3339 specifies the grammar for a valid timestamp.
	// However, the parsing functionality in "time" is too loose and
	// incorrectly accepts invalid timestamps as valid.
	// Remove these manual checks when "time" checks it for us.
	newParseError := func(layout, value, layoutElem, valueElem, message string) error {
		return &time.ParseError{Layout: layout, Value: value, LayoutElem: layoutElem, ValueElem: valueElem, Message: message}
	}
	switch {
	case b[len("2006-01-02T")+1] == ':': // hour must be two digits
		return time.Time{}, newParseError(time.RFC3339, string(b), "15", string(b[len("2006-01-02T"):][:1]), "")
	case b[len("2006-01-02T15:04:05")] == ',': // sub-second separator must be a period
		return time.Time{}, newParseError(time.RFC3339, string(b), ".", ",", "")
	case b[len(b)-1] != 'Z':
		switch {
		case parseDec2(b[len(b)-len("07:00"):]) >= 24: // timezone hour must be in range
			return time.Time{}, newParseError(time.RFC3339, string(b), "Z07:00", string(b[len(b)-len("Z07:00"):]), ": timezone hour out of range")
		case parseDec2(b[len(b)-len("00"):]) >= 60: // timezone minute must be in range
			return time.Time{}, newParseError(time.RFC3339, string(b), "Z07:00", string(b[len(b)-len("Z07:00"):]), ": timezone minute out of range")
		}
	}
	return t, nil
}

// parseDec2 parses b as an unsigned, base-10, 2-digit number.
// The result is undefined if digits are not base-10.
func parseDec2(b []byte) byte {
	if len(b) < 2 {
		return 0
	}
	return 10*(b[0]-'0') + (b[1] - '0')
}

// parseUint parses b as a decimal unsigned integer according to
// a strict subset of the JSON number grammar, returning the value if valid.
// It returns (0, false) if there is a syntax error and
// returns (math.MaxUint64, false) if there is an overflow.
func parseUint(b []byte) (v uint64, ok bool) {
	const unsafeWidth = 20 // len(fmt.Sprint(uint64(math.MaxUint64)))
	var n int
	for ; len(b) > n && ('0' <= b[n] && b[n] <= '9'); n++ {
		v = 10*v + uint64(b[n]-'0')
	}
	switch {
	case n == 0 || len(b) != n || (b[0] == '0' && string(b) != "0"):
		return 0, false
	case n >= unsafeWidth && (b[0] != '1' || v < 1e19 || n > unsafeWidth):
		return math.MaxUint64, false
	}
	return v, true
}
