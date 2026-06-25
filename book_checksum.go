package kraken

import (
	"errors"
	"fmt"
	"hash/crc32"
	"sort"
	"strings"

	"github.com/shopspring/decimal"
)

// ErrBookChecksumMismatch is returned by BookChecksummer.Verify when the locally
// computed CRC32 disagrees with the value the server attached to the frame,
// signalling the maintained book has drifted (the caller should resubscribe to
// fetch a fresh snapshot).
var ErrBookChecksumMismatch = errors.New("kraken: book checksum mismatch")

// BookChecksummer maintains the top-of-book state required to verify the CRC32
// checksum Kraken sends with every level-2 "book" channel frame
// (WsBook.Checksum).
//
// Kraken's WS v2 book channel carries no sequence numbers, so the per-frame CRC32
// is the only way to detect a desynchronised local book. Kraken transmits the
// checksum but provides no way to validate it: a consumer must reconstruct the
// top-10 book in the exact fixed-precision wire representation and re-run the
// algorithm. BookChecksummer owns that representation and the (easy-to-get-wrong)
// algorithm so consumers need not reimplement it.
//
// It deliberately keeps only what the checksum needs — the price/quantity levels
// in fixed-precision form — and is NOT a general-purpose order book. Feed it the
// same WsBook frames you receive (ApplySnapshot for a "snapshot" frame,
// ApplyUpdate for an "update" frame) and call Verify (or Checksum) afterwards.
//
// A BookChecksummer is not safe for concurrent use; callers serialise frame
// handling (the book channel is a single ordered stream).
type BookChecksummer struct {
	priceDecimals int32
	qtyDecimals   int32
	// Sides keyed by price.StringFixed(priceDecimals): the fixed-precision string
	// is both a collision-free key and the exact form the checksum consumes.
	// Keying on the decimal value directly would split economically-equal prices
	// that differ only in trailing zeros (1562.60 vs 1562.6) into two levels.
	bids map[string]decimal.Decimal
	asks map[string]decimal.Decimal
}

// NewBookChecksummer returns a checksummer for a pair whose price and quantity
// are formatted to priceDecimals and qtyDecimals respectively. For Kraken spot
// these are AssetPair.PairDecimals and AssetPair.LotDecimals.
func NewBookChecksummer(priceDecimals, qtyDecimals int32) *BookChecksummer {
	return &BookChecksummer{
		priceDecimals: priceDecimals,
		qtyDecimals:   qtyDecimals,
		bids:          make(map[string]decimal.Decimal),
		asks:          make(map[string]decimal.Decimal),
	}
}

// Reset clears the maintained book. Call it before applying a fresh snapshot
// after a reconnect so stale levels do not leak into the next checksum.
func (b *BookChecksummer) Reset() {
	b.bids = make(map[string]decimal.Decimal)
	b.asks = make(map[string]decimal.Decimal)
}

// ApplySnapshot replaces the maintained book with a snapshot frame's full depth.
func (b *BookChecksummer) ApplySnapshot(book WsBook) {
	b.bids = make(map[string]decimal.Decimal, len(book.Bids))
	b.asks = make(map[string]decimal.Decimal, len(book.Asks))
	b.applyLevels(b.bids, book.Bids)
	b.applyLevels(b.asks, book.Asks)
}

// ApplyUpdate merges an update frame's changed levels into the maintained book.
// A level with quantity <= 0 removes that price.
func (b *BookChecksummer) ApplyUpdate(book WsBook) {
	b.applyLevels(b.bids, book.Bids)
	b.applyLevels(b.asks, book.Asks)
}

func (b *BookChecksummer) applyLevels(side map[string]decimal.Decimal, levels []WsBookLevel) {
	for _, lv := range levels {
		key := lv.Price.StringFixed(b.priceDecimals)
		if lv.Qty.Sign() <= 0 {
			delete(side, key)
			continue
		}
		side[key] = lv.Qty
	}
}

// Checksum computes the CRC32 (IEEE) of the current top-10 book per Kraken's
// WS v2 spec: the top 10 asks (ascending price) followed by the top 10 bids
// (descending price); for each level the price (formatted to priceDecimals) then
// the quantity (formatted to qtyDecimals), each with its decimal point and
// leading zeros removed, all concatenated.
//
// See https://docs.kraken.com/api/docs/guides/spot-ws-book-v2.
func (b *BookChecksummer) Checksum() uint32 {
	const top = 10
	askKeys := sortedBookKeys(b.asks, true)  // ascending price
	bidKeys := sortedBookKeys(b.bids, false) // descending price
	if len(askKeys) > top {
		askKeys = askKeys[:top]
	}
	if len(bidKeys) > top {
		bidKeys = bidKeys[:top]
	}
	var sb strings.Builder
	for _, k := range askKeys {
		sb.WriteString(checksumField(k))
		sb.WriteString(checksumField(b.asks[k].StringFixed(b.qtyDecimals)))
	}
	for _, k := range bidKeys {
		sb.WriteString(checksumField(k))
		sb.WriteString(checksumField(b.bids[k].StringFixed(b.qtyDecimals)))
	}
	return crc32.ChecksumIEEE([]byte(sb.String()))
}

// Verify compares the locally computed checksum against the value carried on the
// frame (WsBook.Checksum, a uint32 widened to int64). A non-positive server value
// is treated as "no checksum on this frame" and returns nil; a mismatch returns
// an error wrapping ErrBookChecksumMismatch.
func (b *BookChecksummer) Verify(serverChecksum int64) error {
	if serverChecksum <= 0 {
		return nil
	}
	local := b.Checksum()
	if local != uint32(serverChecksum) {
		return fmt.Errorf("%w: server=%d local=%d", ErrBookChecksumMismatch, uint32(serverChecksum), local)
	}
	return nil
}

// sortedBookKeys returns side's price keys ordered by numeric value (ascending
// when asc, else descending).
func sortedBookKeys(side map[string]decimal.Decimal, asc bool) []string {
	keys := make([]string, 0, len(side))
	for k := range side {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		a, _ := decimal.NewFromString(keys[i])
		c, _ := decimal.NewFromString(keys[j])
		if asc {
			return a.LessThan(c)
		}
		return a.GreaterThan(c)
	})
	return keys
}

// checksumField removes the decimal point and any leading zeros, e.g.
// "0.00026198" -> "000026198" -> "26198".
func checksumField(s string) string {
	return strings.TrimLeft(strings.ReplaceAll(s, ".", ""), "0")
}
