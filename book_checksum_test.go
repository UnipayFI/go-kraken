package kraken

import (
	"errors"
	"hash/crc32"
	"testing"

	"github.com/shopspring/decimal"
)

func lvl(price, qty string) WsBookLevel {
	return WsBookLevel{Price: decimal.RequireFromString(price), Qty: decimal.RequireFromString(qty)}
}

// TestBookChecksumGolden anchors the algorithm against an independently
// hand-built pre-image string (so the test does not merely re-run the
// implementation): top-10 asks ascending then top-10 bids descending, each
// level's price (StringFixed(priceDec)) then qty (StringFixed(qtyDec)), with the
// decimal point and leading zeros stripped, all concatenated, then CRC32-IEEE.
func TestBookChecksumGolden(t *testing.T) {
	const priceDec, qtyDec = 2, 4
	c := NewBookChecksummer(priceDec, qtyDec)
	// Deliberately pass the sides unsorted to exercise internal sorting.
	c.ApplySnapshot(WsBook{
		Asks: []WsBookLevel{lvl("100.70", "0.0050"), lvl("100.50", "1.2000")},
		Bids: []WsBookLevel{lvl("99.00", "0.1000"), lvl("100.40", "2.0000")},
	})

	// asks ascending: 100.50/1.2000 then 100.70/0.0050
	// bids descending: 100.40/2.0000 then 99.00/0.1000
	preimage := "10050" + "12000" + // ask 100.50 / 1.2000
		"10070" + "50" + // ask 100.70 / 0.0050
		"10040" + "20000" + // bid 100.40 / 2.0000
		"9900" + "1000" // bid 99.00 / 0.1000
	want := crc32.ChecksumIEEE([]byte(preimage))

	if got := c.Checksum(); got != want {
		t.Fatalf("Checksum() = %d, want %d (preimage %q)", got, want, preimage)
	}
}

// TestBookChecksumFixedPrecisionTrap guards the trailing-zero trap documented in
// the backend: a price/qty that trims to fewer digits via String() must still be
// formatted to the pair's fixed precision. 1562.60 at priceDec=2 must contribute
// "156260", not "15626".
func TestBookChecksumFixedPrecisionTrap(t *testing.T) {
	const priceDec, qtyDec = 2, 8
	c := NewBookChecksummer(priceDec, qtyDec)
	c.ApplySnapshot(WsBook{
		Asks: []WsBookLevel{lvl("1562.60", "0.00026198")},
		Bids: []WsBookLevel{lvl("1562.50", "1.00000000")},
	})

	// ask 1562.60 -> "156260", 0.00026198 -> "26198"
	// bid 1562.50 -> "156250", 1.00000000 -> "100000000" (no leading zeros to strip)
	preimage := "156260" + "26198" + "156250" + "100000000"
	want := crc32.ChecksumIEEE([]byte(preimage))
	if got := c.Checksum(); got != want {
		t.Fatalf("Checksum() = %d, want %d (preimage %q)", got, want, preimage)
	}

	// A decimal that already trims (1562.6) must produce the same checksum as the
	// trailing-zero form (1562.60) at the same precision.
	c2 := NewBookChecksummer(priceDec, qtyDec)
	c2.ApplySnapshot(WsBook{
		Asks: []WsBookLevel{lvl("1562.6", "0.00026198")},
		Bids: []WsBookLevel{lvl("1562.5", "1")},
	})
	if c.Checksum() != c2.Checksum() {
		t.Fatalf("trailing-zero forms disagree: %d vs %d", c.Checksum(), c2.Checksum())
	}
}

// TestBookChecksumTopTen verifies only the best 10 levels per side feed the
// checksum: levels beyond the top 10 must not change it.
func TestBookChecksumTopTen(t *testing.T) {
	const priceDec, qtyDec = 1, 1
	asks := make([]WsBookLevel, 0, 12)
	bids := make([]WsBookLevel, 0, 12)
	for i := 0; i < 12; i++ {
		asks = append(asks, WsBookLevel{Price: decimal.New(int64(100+i), 0), Qty: decimal.New(1, 0)})
		bids = append(bids, WsBookLevel{Price: decimal.New(int64(99-i), 0), Qty: decimal.New(1, 0)})
	}
	full := NewBookChecksummer(priceDec, qtyDec)
	full.ApplySnapshot(WsBook{Asks: asks, Bids: bids})

	top := NewBookChecksummer(priceDec, qtyDec)
	top.ApplySnapshot(WsBook{Asks: asks[:10], Bids: bids[:10]})

	if full.Checksum() != top.Checksum() {
		t.Fatalf("levels beyond top-10 affected checksum: full=%d top10=%d", full.Checksum(), top.Checksum())
	}
}

// TestBookChecksumUpdate checks that an incremental update (level change and
// qty<=0 removal) yields the same state as a snapshot of the final book.
func TestBookChecksumUpdate(t *testing.T) {
	const priceDec, qtyDec = 1, 2
	c := NewBookChecksummer(priceDec, qtyDec)
	c.ApplySnapshot(WsBook{
		Asks: []WsBookLevel{lvl("100.0", "1.00"), lvl("101.0", "2.00")},
		Bids: []WsBookLevel{lvl("99.0", "3.00")},
	})
	// Remove ask 101.0, change ask 100.0 qty, add bid 98.0.
	c.ApplyUpdate(WsBook{
		Asks: []WsBookLevel{lvl("101.0", "0"), lvl("100.0", "1.50")},
		Bids: []WsBookLevel{lvl("98.0", "4.00")},
	})

	// Oracle: a fresh checksummer built directly from the expected final state.
	want := NewBookChecksummer(priceDec, qtyDec)
	want.ApplySnapshot(WsBook{
		Asks: []WsBookLevel{lvl("100.0", "1.50")},
		Bids: []WsBookLevel{lvl("99.0", "3.00"), lvl("98.0", "4.00")},
	})
	if c.Checksum() != want.Checksum() {
		t.Fatalf("update state mismatch: got=%d want=%d", c.Checksum(), want.Checksum())
	}
}

func TestBookChecksumVerify(t *testing.T) {
	c := NewBookChecksummer(2, 4)
	c.ApplySnapshot(WsBook{
		Asks: []WsBookLevel{lvl("100.50", "1.2000")},
		Bids: []WsBookLevel{lvl("100.40", "2.0000")},
	})
	sum := c.Checksum()

	if err := c.Verify(int64(sum)); err != nil {
		t.Fatalf("Verify(matching) = %v, want nil", err)
	}
	if err := c.Verify(0); err != nil {
		t.Fatalf("Verify(0) = %v, want nil (absent checksum)", err)
	}
	err := c.Verify(int64(sum) ^ 0x1)
	if !errors.Is(err, ErrBookChecksumMismatch) {
		t.Fatalf("Verify(wrong) = %v, want ErrBookChecksumMismatch", err)
	}
}

func TestBookChecksumReset(t *testing.T) {
	c := NewBookChecksummer(1, 1)
	c.ApplySnapshot(WsBook{Asks: []WsBookLevel{lvl("100.0", "1.0")}})
	c.Reset()
	empty := NewBookChecksummer(1, 1)
	if c.Checksum() != empty.Checksum() {
		t.Fatalf("after Reset checksum=%d, want empty=%d", c.Checksum(), empty.Checksum())
	}
}
