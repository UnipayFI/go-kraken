package kraken

import "errors"

// MaxClOrdIDLen is the maximum length of the free-text form of a client order id
// (cl_ord_id) accepted by Kraken's add_order endpoint.
//
// Kraken accepts a cl_ord_id in one of three forms (verified empirically with
// add_order's validate mode):
//   - free text up to MaxClOrdIDLen characters, restricted to lowercase letters
//     and digits — uppercase or over-length text is rejected with
//     "EGeneral:Invalid arguments:cl_ord_id";
//   - a standard UUID (8-4-4-4-12 hexadecimal, any case);
//   - a 32-character hexadecimal string (no hyphens).
const MaxClOrdIDLen = 18

// ErrInvalidClOrdID reports a cl_ord_id Kraken's add_order will reject. See
// MaxClOrdIDLen for the accepted forms.
var ErrInvalidClOrdID = errors.New("kraken: cl_ord_id must be <=18 lowercase alphanumerics, a standard UUID, or 32 hex characters")

// ValidateClOrdID reports whether s is a client order id Kraken's add_order will
// accept, returning ErrInvalidClOrdID otherwise. An empty string is valid (the
// field is optional). See MaxClOrdIDLen for the accepted forms.
func ValidateClOrdID(s string) error {
	switch {
	case s == "":
		return nil
	case isClOrdUUID(s):
		return nil
	case len(s) == 32 && isHexString(s):
		return nil
	case len(s) <= MaxClOrdIDLen && isLowerAlphanumeric(s):
		return nil
	default:
		return ErrInvalidClOrdID
	}
}

func isLowerAlphanumeric(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if !((c >= 'a' && c <= 'z') || (c >= '0' && c <= '9')) {
			return false
		}
	}
	return true
}

func isHexString(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if !isHexDigit(s[i]) {
			return false
		}
	}
	return true
}

// isClOrdUUID reports whether s is a standard 8-4-4-4-12 hexadecimal UUID.
func isClOrdUUID(s string) bool {
	if len(s) != 36 {
		return false
	}
	for i := 0; i < len(s); i++ {
		switch i {
		case 8, 13, 18, 23:
			if s[i] != '-' {
				return false
			}
		default:
			if !isHexDigit(s[i]) {
				return false
			}
		}
	}
	return true
}

func isHexDigit(c byte) bool {
	return (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')
}
