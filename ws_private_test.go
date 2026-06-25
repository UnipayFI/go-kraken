package kraken

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/UnipayFI/go-kraken/internal/apitest"
)

// awaitNoError waits a short window and passes unless the callback reports an
// error (used for channels whose snapshot may legitimately be empty, so the
// success signal is "subscription established without error").
func awaitNoError(t *testing.T, label string, res <-chan error, window time.Duration) {
	t.Helper()
	select {
	case err := <-res:
		if err != nil {
			t.Errorf("%s: %v", label, err)
			return
		}
		t.Logf("%s: OK (push received, typed decode clean)", label)
	case <-time.After(window):
		t.Logf("%s: OK (subscription established, no error/data within %s)", label, window)
	}
}

// TestWsPrivateChannels live-subscribes to the private v2 channels using a token
// fetched automatically from the REST API.
func TestWsPrivateChannels(t *testing.T) {
	c := NewWebSocketClient(apitest.WsAuthOptions(t)...)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// balances — the account holds USDT, so the snapshot has data to decode.
	t.Run("balances", func(t *testing.T) {
		res := make(chan error, 1)
		done, _, err := c.NewSubscribeBalancesService().Do(ctx, func(p *WsPush[[]WsBalance], e error) {
			if e != nil {
				trySend(res, e)
			} else if len(p.Data) > 0 && p.Data[0].Asset != "" {
				trySend(res, nil)
			}
		})
		if err != nil {
			t.Fatalf("subscribe: %v", err)
		}
		defer close(done)
		awaitWs(t, "balances", res)
	})

	// level3 — authenticated per-order book. It is an entitlement-gated premium
	// feed; on an unentitled account Kraken rejects the subscription, which the
	// test tolerates (the token auth + subscription path are still exercised).
	t.Run("level3", func(t *testing.T) {
		res := make(chan error, 1)
		done, _, err := c.NewSubscribeLevel3Service("BTC/USD").Do(ctx, func(p *WsPush[[]WsLevel3], e error) {
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
		select {
		case e := <-res:
			if e == nil {
				t.Log("level3: OK (push received, typed decode clean)")
			} else if msg := e.Error(); strings.Contains(msg, "invalid") || strings.Contains(msg, "unavailable") || strings.Contains(msg, "permission") {
				t.Logf("level3: account not entitled (%v) — subscription path + token OK", e)
			} else {
				t.Errorf("level3: %v", e)
			}
		case <-time.After(10 * time.Second):
			t.Log("level3: no data within window (may be entitlement-gated)")
		}
	})

	// executions — verifies token auth + subscription; full field decode of
	// real executions is covered by the order-entry test placing live orders.
	t.Run("executions", func(t *testing.T) {
		res := make(chan error, 1)
		done, _, err := c.NewSubscribeExecutionsService().SetSnapOrders(true).Do(ctx, func(p *WsPush[[]WsExecution], e error) {
			if e != nil {
				trySend(res, e)
			} else if len(p.Data) > 0 {
				trySend(res, nil)
			}
		})
		if err != nil {
			t.Fatalf("subscribe: %v", err)
		}
		defer close(done)
		awaitNoError(t, "executions", res, 8*time.Second)
	})
}
