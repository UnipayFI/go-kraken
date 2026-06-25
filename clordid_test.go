package kraken

import (
	"errors"
	"strings"
	"testing"
)

func TestValidateClOrdID(t *testing.T) {
	cases := []struct {
		name string
		in   string
		ok   bool
	}{
		{"empty is optional", "", true},
		{"lowercase alnum", "te7zb3z1", true},
		{"max length 18", strings.Repeat("a", 18), true},
		{"over length 19", strings.Repeat("a", 19), false},
		{"uppercase rejected", "ABC123", false},
		{"mixed case rejected", "Abc123", false},
		{"hyphen in free text rejected", "te-7-b3-1", false},
		{"standard uuid", "550e8400-e29b-41d4-a716-446655440000", true},
		{"uppercase uuid", "550E8400-E29B-41D4-A716-446655440000", true},
		{"uuid wrong dashes", "550e8400e29b41d4a716446655440000xxxx", false},
		{"32 hex", "0123456789abcdef0123456789abcdef", true},
		{"32 hex uppercase", "0123456789ABCDEF0123456789ABCDEF", true},
		{"32 non-hex rejected", "0123456789abcdefg123456789abcdef", false},
		{"31 hex rejected", "0123456789abcdef0123456789abcde", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := ValidateClOrdID(c.in)
			if c.ok && err != nil {
				t.Fatalf("ValidateClOrdID(%q) = %v, want nil", c.in, err)
			}
			if !c.ok {
				if err == nil {
					t.Fatalf("ValidateClOrdID(%q) = nil, want error", c.in)
				}
				if !errors.Is(err, ErrInvalidClOrdID) {
					t.Fatalf("ValidateClOrdID(%q) = %v, want ErrInvalidClOrdID", c.in, err)
				}
			}
		})
	}
}

func TestMaxClOrdIDLen(t *testing.T) {
	if MaxClOrdIDLen != 18 {
		t.Fatalf("MaxClOrdIDLen = %d, want 18", MaxClOrdIDLen)
	}
}
