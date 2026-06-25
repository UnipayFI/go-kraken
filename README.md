# go-kraken

[![Go Reference](https://pkg.go.dev/badge/github.com/UnipayFI/go-kraken.svg)](https://pkg.go.dev/github.com/UnipayFI/go-kraken)
[![Go 1.26+](https://img.shields.io/badge/Go-1.26%2B-00ADD8?logo=go)](go.mod)
[![License: MIT](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)

A Go SDK for the [Kraken](https://docs.kraken.com/api/) spot exchange, covering the full Spot REST API.

| API | Aligned to |
|---|---|
| **Spot REST&Websocket**  | [2026-06-22](https://docs.kraken.com/exchange/changelog) |

Response structs are reconciled against the live API (not just the docs): every endpoint is tested by diffing the real JSON keys against the struct, so they stay in sync with the date above.

## Install

```bash
go get github.com/UnipayFI/go-kraken@latest
```

## Highlights

- One signing/transport core shared by every endpoint (`client` + `request` + `common`).
- Fluent per-endpoint API: `NewXxxService(...).SetFoo(...).Do(ctx)`.
- Amounts as `decimal.Decimal`, timestamps as `time.Time` — Kraken's string-encoded numbers, UNIX-seconds (and the occasional RFC3339 / nanosecond) times, and `false`/`""`/`"0"` "not set" sentinels are decoded for you.
- Kraken's positional-array payloads (OHLC, order book, trades, spreads, ticker fields, fee tiers) are decoded into named struct fields.
- Every endpoint is tested against the live API; trading is exercised with tiny, reversible, large-cap orders.

## Quick start

```go
package main

import (
	"context"
	"fmt"

	kraken "github.com/UnipayFI/go-kraken"
	"github.com/UnipayFI/go-kraken/client"
	"github.com/shopspring/decimal"
)

func main() {
	ctx := context.Background()

	c := kraken.NewClient(
		client.WithAuth("apiKey", "apiSecret"),
		// client.WithProxy("socks5://127.0.0.1:7890"),
	)

	// Public market data (no auth).
	srv, _ := c.NewGetServerTimeService().Do(ctx)
	fmt.Println("server time:", srv.UnixTime)

	ticker, _ := c.NewGetTickerService().SetPair("XBTUSD").Do(ctx)
	fmt.Println("BTC last:", ticker["XXBTZUSD"].Last.Price)

	// Private account data.
	balances, _ := c.NewGetAccountBalanceService().Do(ctx)
	fmt.Println("USDT:", balances["USDT"])

	// Place a post-only limit order.
	ref, err := c.NewAddOrderService("XBTUSDT", kraken.OrderSideBuy, kraken.OrderTypeLimit,
		decimal.RequireFromString("0.0001")).
		SetPrice(decimal.RequireFromString("30000")).
		SetOrderFlags(kraken.OrderFlagPost).
		Do(ctx)
	if err != nil {
		panic(err)
	}
	fmt.Println("txid:", ref.TxID)
}
```

## Authentication

Pass credentials from the Kraken [API-management page](https://www.kraken.com/u/security/api):

```go
c := kraken.NewClient(client.WithAuth(apiKey, apiSecret))
```

`apiSecret` is the base64-encoded **Private Key** shown when the key was created. Private requests are signed with

```
API-Sign = base64( HMAC-SHA512( base64decode(secret), uriPath + SHA256(nonce + postData) ) )
```

placed in the `API-Sign` header alongside `API-Key`. Each request carries an always-increasing **nonce** in its url-encoded body; the SDK generates a strictly-monotonic nonce for you (nanosecond-based, so it survives restarts). For an external signer pass `client.WithSignFn(fn)`; to share or override the nonce sequence pass `client.WithNonce(fn)`.

Other options: `WithProxy` (http/https/socks5), `WithBaseURL`, `WithNetwork`, `WithLogger`, `WithHTTPClient`.

## WebSocket

The v2 streams (`wss://ws.kraken.com/v2` public, `wss://ws-auth.kraken.com/v2` private) are exposed with the same `NewXxxService(...).Do(ctx, cb)` shape. Private channels and order entry need credentials; the short-lived auth token is fetched from the REST API and cached for you.

```go
ws := kraken.NewWebSocketClient(
    client.WithWebSocketAuth(apiKey, apiSecret), // private channels + order entry only
    // client.WithWebSocketProxy("socks5://127.0.0.1:7890"),
)

// Public ticker.
done, _, _ := ws.NewSubscribeTickerService("BTC/USD").
    Do(ctx, func(p *kraken.WsPush[[]kraken.WsTicker], err error) {
        if err != nil {
            return
        }
        fmt.Println(p.Type, p.Data[0].Last) // "snapshot"/"update", last price
    })
close(done) // unsubscribe + close

// Private balances (token fetched + cached automatically).
ws.NewSubscribeBalancesService().Do(ctx, func(p *kraken.WsPush[[]kraken.WsBalance], err error) {
    // p.Data[0].Asset, p.Data[0].Balance, ...
})
```

Each `Do` returns `(done chan<- struct{}, stop <-chan struct{}, err error)`: close `done` to unsubscribe and close; `stop` closes when the reader exits. A `{"method":"ping"}` keepalive is sent automatically.

Orders can also be placed over a persistent, authenticated connection — a low-latency alternative to the REST trade endpoints:

```go
tc, _ := ws.DialTrade(ctx)
defer tc.Close()
ack, _ := tc.AddOrder(ctx, kraken.WsAddOrder{
    Symbol: "BTC/USDT", Side: kraken.OrderSideBuy, OrderType: kraken.OrderTypeLimit,
    OrderQty: decimal.RequireFromString("0.0001"), LimitPrice: decimal.RequireFromString("30000"),
    PostOnly: true,
})
fmt.Println(ack.Result.OrderID)
// tc.AmendOrder / tc.EditOrder / tc.CancelOrder / tc.BatchAdd / tc.CancelAll / ...
```

Public channels: `ticker`, `book`, `ohlc`, `trade`, `instrument`. Private channels: `executions`, `balances`, `level3` (per-order book; entitlement-gated).

## Packages

**Spot REST** (root package `kraken`)

| Area | File | Endpoints |
|------|------|-----------|
| Market data | `market.go` | Time, SystemStatus, Assets, AssetPairs, Ticker, OHLC, Depth (order book), Trades, Spread, GroupedBook, Level3 (authenticated) |
| Account data | `account.go` | Balance, BalanceEx, TradeBalance, Open/Closed/Query Orders, OrderAmends, Trades History, Query Trades, Open Positions, Ledgers, Query Ledgers, Trade Volume, export reports (Add/Status/Retrieve/Remove), CreditLines, GetApiKeyInfo |
| Trading | `trade.go` | AddOrder, AddOrderBatch, AmendOrder, EditOrder, CancelOrder, CancelAll, CancelAllOrdersAfter, CancelOrderBatch |
| Funding | `funding.go` | Deposit Methods/Addresses/Status, Withdraw Methods/Addresses/Info, Withdraw, Withdraw Status/Cancel, WalletTransfer |
| Subaccounts | `subaccount.go` | CreateSubaccount, AccountTransfer (institutional) |
| Earn | `earn.go` | Allocate, Deallocate, Allocate/Deallocate Status, Strategies, Allocations |
| Transparency | `transparency.go` | PreTrade, PostTrade (MiFID pre/post-trade data) |
| WebSocket auth | `websocket.go` | GetWebSocketsToken (REST) |
| Shared types | `types.go` | order enums, `NanoTime`, `MethodLimit` |

**WebSocket v2** (`ws*.go`)

| Area | File | Channels / methods |
|------|------|--------------------|
| Entry | `ws.go` | `NewWebSocketClient`, `WsPush`, `SubscribeRaw` |
| Public channels | `ws_public.go` | ticker, book, ohlc, trade, instrument |
| Private channels | `ws_private.go` | executions, balances, level3 |
| Order entry | `ws_trade.go` | add/amend/edit/cancel order, cancel-all, cancel-all-after, batch add/cancel |

**Core**

| Package | Scope |
|---------|-------|
| `kraken.go` | entry point: `NewClient` (REST) + `NewWebSocketClient` (in `ws.go`) |
| `client/` | REST + WebSocket clients, options, nonce generator, HMAC-SHA512 signer config, WS token cache, `APIError` |
| `request/` | request builder (form-urlencoded), generic `Do[T]` envelope decode, signer, WS subscribe/order-entry framework |
| `common/` | constants, global `time.Time` (UNIX-seconds/RFC3339) + `decimal.Decimal` JSON codec |
| `internal/apitest/` | test-only field-coverage helpers |
| `cmd/kraw/` | dev tool: sign + dump any endpoint's raw response |

## Testing

Tests hit the live API and read credentials from the environment, skipping when unset:

```bash
export KRAKEN_API_KEY=...  KRAKEN_API_SECRET=...
export KRAKEN_PROXY=socks5://127.0.0.1:7890   # optional

go test ./ -run TestMarketData -v                 # one module at a time
go test ./ -run TestWsPublicChannels -v           # live WebSocket channels
KRAKEN_TEST_WRITE=1 go test ./ -run TestTradeLifecycle -v     # live REST order tests (tiny, reversible)
KRAKEN_TEST_WRITE=1 go test ./ -run TestWsTradeLifecycle -v   # live WebSocket order entry
```

- Run **per module** (`-run TestXxx`). Kraken enforces a per-key rate counter, so firing the whole suite at once can trip `EAPI:Rate limit exceeded`; individual modules stay under the limit.
- REST tests diff the real response JSON against the typed structs (`AssertCovers`), so a missing or renamed field fails the test. WebSocket tests subscribe live and verify each push decodes cleanly into the typed struct (a wrong field type surfaces as a decode error).
- Capability-gated endpoints (subaccounts, some funding/earn operations, the level3 stream) are tolerated when the account lacks the capability — signing and the request path are still exercised.
- State-changing trade/earn tests are gated behind `KRAKEN_TEST_WRITE=1` and use minimal amounts on large-cap pairs (a post-only order far from market never fills; one tiny market round-trip is immediately unwound). The fund-moving **Withdraw** endpoint is implemented but never executed.

The `cmd/kraw` helper signs and dumps any endpoint's raw response:

```bash
go run ./cmd/kraw GET  /0/public/Ticker "pair=XBTUSD"
go run ./cmd/kraw POST /0/private/Balance
go run ./cmd/kraw POST /0/private/Ledgers "asset=USDT&type=deposit"
```

## CHANGE_LOG

### 2026-06-25 (WebSocket v2)

- Added the full Kraken Spot **WebSocket v2** surface, in the same `NewXxxService(...).Do(ctx, cb)` style:
  - Public channels (5): ticker, book, ohlc, trade, instrument.
  - Private channels (3): executions, balances, level3 (entitlement-gated).
  - Order entry (8 methods): add_order, amend_order, edit_order, cancel_order, cancel_all, cancel_all_orders_after, batch_add, batch_cancel — over a persistent `DialTrade` connection.
- Token-based auth: the WebSocket token is fetched from REST `GetWebSocketsToken` and cached (auto-refresh near expiry); proxy is shared with the stream dial. Automatic `{"method":"ping"}` keepalive.
- All channels verified against the live stream (push received + clean typed decode); WebSocket order entry verified with tiny reversible orders and live `executions` events.

### 2026-06-25

- Initial release. Full Kraken Spot REST API coverage (**59 endpoints**) across all categories: market data, account data, trading, funding, subaccounts, Earn, Transparency, and the WebSockets auth token.
  - Market data (11): Time, SystemStatus, Assets, AssetPairs, Ticker, OHLC, Depth, Trades, Spread, GroupedBook, Level3.
  - Account data (19): balances, orders, order amends, trades, positions, ledgers, trade volume, export reports, credit lines, API-key info.
  - Trading (8), Funding (10), Subaccounts (2), Earn (6), Transparency (2: PreTrade/PostTrade), WebSockets auth (1).
- Signing core: HMAC-SHA512 over `uriPath + SHA256(nonce + postData)`, strictly-monotonic nanosecond nonce, form-urlencoded bodies (with bracketed `orders[i][...]` arrays for batch endpoints).
- Global JSON codec: `decimal.Decimal` for amounts, `time.Time` for UNIX-seconds/RFC3339 timestamps, `NanoTime` for nanosecond timestamps, `MethodLimit` for the `false`-or-amount union.
- All public and private endpoints reconciled against the live API; trading verified with tiny reversible orders and one large-cap market round-trip, Earn with a reversible allocate/deallocate round-trip.

## License

[MIT](LICENSE)
