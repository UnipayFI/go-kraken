package kraken

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/UnipayFI/go-kraken/internal/apitest"
	"github.com/shopspring/decimal"
)

// TestEarn exercises the Earn read endpoints unconditionally. The fund-moving
// allocate/deallocate round-trip (which verifies the allocation item fields
// against a real allocation) is gated behind KRAKEN_TEST_WRITE=1.
func TestEarn(t *testing.T) {
	c := NewClient(apitest.AuthOptions(t)...)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	// 1. List Earn Strategies (and find an allocatable one).
	var allocatableID string
	{
		raw := apitest.FetchRawPost(t, c, ctx, "/0/private/Earn/Strategies", map[string]string{"asset": "USDT"})
		resp, err := c.NewListEarnStrategiesService().SetAsset("USDT").Do(ctx)
		if err != nil {
			t.Fatalf("Earn/Strategies: %v", err)
		}
		apitest.AssertCovers(t, "Earn/Strategies", raw, resp)
		if len(resp.Items) == 0 {
			t.Fatal("Earn/Strategies: no strategies")
		}
		// Only pick a strategy with no bonding/unbonding/exit-queue period, so the
		// write round-trip is fully reversible and never strands funds.
		for _, s := range resp.Items {
			if s.CanAllocate &&
				s.LockType.BondingPeriod == 0 &&
				s.LockType.UnbondingPeriod == 0 &&
				s.LockType.ExitQueuePeriod == 0 {
				allocatableID = s.ID
				break
			}
		}
		t.Logf("Earn/Strategies: %d strategies, reversible-allocatable=%q", len(resp.Items), allocatableID)
		pace()
	}

	// Fall back to any strategy id for the status-endpoint checks.
	statusID := allocatableID
	if statusID == "" {
		resp, _ := c.NewListEarnStrategiesService().SetAsset("USDT").Do(ctx)
		if resp != nil && len(resp.Items) > 0 {
			statusID = resp.Items[0].ID
		}
	}

	// 2. List Earn Allocations.
	{
		raw := apitest.FetchRawPost(t, c, ctx, "/0/private/Earn/Allocations", nil)
		resp, err := c.NewListEarnAllocationsService().Do(ctx)
		if err != nil {
			t.Fatalf("Earn/Allocations: %v", err)
		}
		apitest.AssertCovers(t, "Earn/Allocations", raw, resp)
		t.Logf("Earn/Allocations: %d item(s) total_allocated=%s", len(resp.Items), resp.TotalAllocated)
		pace()
	}

	// 3. Get Allocation Status.
	if statusID != "" {
		raw := apitest.FetchRawPost(t, c, ctx, "/0/private/Earn/AllocateStatus", map[string]string{"strategy_id": statusID})
		resp, err := c.NewGetAllocationStatusService(statusID).Do(ctx)
		if err != nil {
			t.Fatalf("Earn/AllocateStatus: %v", err)
		}
		apitest.AssertCovers(t, "Earn/AllocateStatus", raw, resp)
		pace()
	}

	// 4. Get Deallocation Status.
	if statusID != "" {
		raw := apitest.FetchRawPost(t, c, ctx, "/0/private/Earn/DeallocateStatus", map[string]string{"strategy_id": statusID})
		resp, err := c.NewGetDeallocationStatusService(statusID).Do(ctx)
		if err != nil {
			t.Fatalf("Earn/DeallocateStatus: %v", err)
		}
		apitest.AssertCovers(t, "Earn/DeallocateStatus", raw, resp)
		pace()
	}

	// 5-6. Allocate / Deallocate round-trip (write-gated; verifies allocation
	// item fields against a real allocation, then returns the funds).
	if os.Getenv("KRAKEN_TEST_WRITE") != "1" {
		t.Log("Earn/Allocate+Deallocate: set KRAKEN_TEST_WRITE=1 to run the live round-trip")
		return
	}
	if allocatableID == "" {
		t.Skip("no allocatable USDT strategy available")
	}
	earnRoundTrip(t, c, ctx, allocatableID)
}

func earnRoundTrip(t *testing.T, c *Client, ctx context.Context, strategyID string) {
	amount := decimal.RequireFromString("2")

	// Allocate.
	ok, err := c.NewAllocateEarnFundsService(strategyID, amount).Do(ctx)
	if err != nil {
		if apitest.Tolerable(t, "Earn/Allocate", err, "minimum", "Insufficient", "cap", "restricted") {
			return
		}
		t.Fatalf("Earn/Allocate: %v", err)
	}
	t.Logf("Earn/Allocate: accepted=%v", ok)

	// Wait for the allocation to finish processing.
	waitEarnSettled(t, c, ctx, strategyID, true)

	// Allocations now has a row for this strategy — verify item fields.
	allocRaw := apitest.FetchRawPost(t, c, ctx, "/0/private/Earn/Allocations",
		map[string]string{"hide_zero_allocations": "true"})
	allocs, err := c.NewListEarnAllocationsService().SetHideZeroAllocations(true).Do(ctx)
	if err != nil {
		t.Fatalf("Earn/Allocations (post-allocate): %v", err)
	}
	apitest.AssertCovers(t, "Earn/Allocations(item)", allocRaw, allocs)
	if len(allocs.Items) > 0 {
		t.Logf("Earn allocation item: strategy=%s total=%s", allocs.Items[0].StrategyID, allocs.Items[0].AmountAllocated.Total.Native)
	}

	// Deallocate to return the funds.
	okD, err := c.NewDeallocateEarnFundsService(strategyID, amount).Do(ctx)
	if err != nil {
		t.Fatalf("Earn/Deallocate: %v — manual cleanup may be needed for strategy %s", err, strategyID)
	}
	t.Logf("Earn/Deallocate: accepted=%v", okD)
	waitEarnSettled(t, c, ctx, strategyID, false)
}

// waitEarnSettled polls the (de)allocation status until it is no longer pending.
func waitEarnSettled(t *testing.T, c *Client, ctx context.Context, strategyID string, allocate bool) {
	t.Helper()
	for attempt := 0; attempt < 15; attempt++ {
		var pending bool
		if allocate {
			st, err := c.NewGetAllocationStatusService(strategyID).Do(ctx)
			if err != nil {
				return
			}
			pending = st.Pending
		} else {
			st, err := c.NewGetDeallocationStatusService(strategyID).Do(ctx)
			if err != nil {
				return
			}
			pending = st.Pending
		}
		if !pending {
			return
		}
		time.Sleep(2 * time.Second)
	}
}
