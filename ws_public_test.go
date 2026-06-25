package kraken

import (
	"context"
	"testing"
	"time"

	"github.com/UnipayFI/go-kraken/internal/apitest"
)

// awaitWs waits up to 15s for a subscription callback to report either a decode
// error or a successful, non-empty data push.
func awaitWs(t *testing.T, label string, res <-chan error) {
	t.Helper()
	select {
	case err := <-res:
		if err != nil {
			t.Errorf("%s: %v", label, err)
			return
		}
		t.Logf("%s: OK (push received, typed decode clean)", label)
	case <-time.After(15 * time.Second):
		t.Errorf("%s: timed out waiting for a push", label)
	}
}

func trySend(ch chan<- error, v error) {
	select {
	case ch <- v:
	default:
	}
}

// TestWsPublicChannels live-subscribes to every public v2 channel and verifies a
// push arrives AND decodes cleanly into the typed struct (a wrong field type
// surfaces as a decode error in the callback).
func TestWsPublicChannels(t *testing.T) {
	c := NewWebSocketClient(apitest.WsPublicOptions()...)
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	const sym = "BTC/USD"

	t.Run("ticker", func(t *testing.T) {
		res := make(chan error, 1)
		done, _, err := c.NewSubscribeTickerService(sym).Do(ctx, func(p *WsPush[[]WsTicker], e error) {
			if e != nil {
				trySend(res, e)
			} else if len(p.Data) > 0 && !p.Data[0].Bid.IsZero() {
				trySend(res, nil)
			}
		})
		if err != nil {
			t.Fatalf("subscribe: %v", err)
		}
		defer close(done)
		awaitWs(t, "ticker", res)
	})

	t.Run("book", func(t *testing.T) {
		res := make(chan error, 1)
		done, _, err := c.NewSubscribeBookService(sym).SetDepth(10).Do(ctx, func(p *WsPush[[]WsBook], e error) {
			if e != nil {
				trySend(res, e)
			} else if len(p.Data) > 0 && (len(p.Data[0].Bids) > 0 || len(p.Data[0].Asks) > 0) {
				trySend(res, nil)
			}
		})
		if err != nil {
			t.Fatalf("subscribe: %v", err)
		}
		defer close(done)
		awaitWs(t, "book", res)
	})

	t.Run("ohlc", func(t *testing.T) {
		res := make(chan error, 1)
		done, _, err := c.NewSubscribeOHLCService(1, sym).Do(ctx, func(p *WsPush[[]WsOHLC], e error) {
			if e != nil {
				trySend(res, e)
			} else if len(p.Data) > 0 && !p.Data[0].Close.IsZero() {
				trySend(res, nil)
			}
		})
		if err != nil {
			t.Fatalf("subscribe: %v", err)
		}
		defer close(done)
		awaitWs(t, "ohlc", res)
	})

	t.Run("trade", func(t *testing.T) {
		res := make(chan error, 1)
		done, _, err := c.NewSubscribeTradeService(sym).Do(ctx, func(p *WsPush[[]WsTrade], e error) {
			if e != nil {
				trySend(res, e)
			} else if len(p.Data) > 0 && !p.Data[0].Price.IsZero() {
				trySend(res, nil)
			}
		})
		if err != nil {
			t.Fatalf("subscribe: %v", err)
		}
		defer close(done)
		awaitWs(t, "trade", res)
	})

	t.Run("instrument", func(t *testing.T) {
		res := make(chan error, 1)
		done, _, err := c.NewSubscribeInstrumentService().Do(ctx, func(p *WsPush[WsInstrument], e error) {
			if e != nil {
				trySend(res, e)
			} else if len(p.Data.Pairs) > 0 && len(p.Data.Assets) > 0 {
				trySend(res, nil)
			}
		})
		if err != nil {
			t.Fatalf("subscribe: %v", err)
		}
		defer close(done)
		awaitWs(t, "instrument", res)
	})
}
