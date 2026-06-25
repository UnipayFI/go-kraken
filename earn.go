package kraken

import (
	"context"
	"strings"
	"time"

	"github.com/UnipayFI/go-kraken/request"
	"github.com/shopspring/decimal"
)

// ===========================================================================
// 1. Allocate Earn Funds -- POST /0/private/Earn/Allocate
// ===========================================================================

// AllocateEarnFundsService allocates funds to an Earn strategy. The operation is
// asynchronous; poll GetAllocationStatus for completion.
type AllocateEarnFundsService struct {
	c      *Client
	params map[string]string
}

// NewAllocateEarnFundsService allocates amount of the strategy's asset to the
// strategy.
func (c *Client) NewAllocateEarnFundsService(strategyID string, amount decimal.Decimal) *AllocateEarnFundsService {
	return &AllocateEarnFundsService{c: c, params: map[string]string{
		"strategy_id": strategyID,
		"amount":      amount.String(),
	}}
}

// Do reports whether the allocation request was accepted (Kraken returns a bare
// boolean; the allocation itself completes asynchronously).
func (s *AllocateEarnFundsService) Do(ctx context.Context) (bool, error) {
	resp, err := request.Do[bool](request.Post(ctx, s.c, "/0/private/Earn/Allocate", s.params).WithSign())
	if err != nil {
		return false, err
	}
	return *resp, nil
}

// ===========================================================================
// 2. Deallocate Earn Funds -- POST /0/private/Earn/Deallocate
// ===========================================================================

// DeallocateEarnFundsService removes previously allocated funds from an Earn
// strategy. The operation is asynchronous; poll GetDeallocationStatus.
type DeallocateEarnFundsService struct {
	c      *Client
	params map[string]string
}

// NewDeallocateEarnFundsService deallocates amount from the strategy.
func (c *Client) NewDeallocateEarnFundsService(strategyID string, amount decimal.Decimal) *DeallocateEarnFundsService {
	return &DeallocateEarnFundsService{c: c, params: map[string]string{
		"strategy_id": strategyID,
		"amount":      amount.String(),
	}}
}

// Do reports whether the deallocation request was accepted.
func (s *DeallocateEarnFundsService) Do(ctx context.Context) (bool, error) {
	resp, err := request.Do[bool](request.Post(ctx, s.c, "/0/private/Earn/Deallocate", s.params).WithSign())
	if err != nil {
		return false, err
	}
	return *resp, nil
}

// ===========================================================================
// 3. Get Allocation Status -- POST /0/private/Earn/AllocateStatus
// ===========================================================================

// GetAllocationStatusService reports whether an allocation to a strategy is
// still being processed.
type GetAllocationStatusService struct {
	c      *Client
	params map[string]string
}

func (c *Client) NewGetAllocationStatusService(strategyID string) *GetAllocationStatusService {
	return &GetAllocationStatusService{c: c, params: map[string]string{"strategy_id": strategyID}}
}

func (s *GetAllocationStatusService) Do(ctx context.Context) (*EarnAllocationStatus, error) {
	return request.Do[EarnAllocationStatus](request.Post(ctx, s.c, "/0/private/Earn/AllocateStatus", s.params).WithSign())
}

// EarnAllocationStatus reports whether an (de)allocation is pending.
type EarnAllocationStatus struct {
	Pending bool `json:"pending"` // whether the operation is still being processed
}

// ===========================================================================
// 4. Get Deallocation Status -- POST /0/private/Earn/DeallocateStatus
// ===========================================================================

// GetDeallocationStatusService reports whether a deallocation from a strategy is
// still being processed.
type GetDeallocationStatusService struct {
	c      *Client
	params map[string]string
}

func (c *Client) NewGetDeallocationStatusService(strategyID string) *GetDeallocationStatusService {
	return &GetDeallocationStatusService{c: c, params: map[string]string{"strategy_id": strategyID}}
}

func (s *GetDeallocationStatusService) Do(ctx context.Context) (*EarnAllocationStatus, error) {
	return request.Do[EarnAllocationStatus](request.Post(ctx, s.c, "/0/private/Earn/DeallocateStatus", s.params).WithSign())
}

// ===========================================================================
// 5. List Earn Strategies -- POST /0/private/Earn/Strategies
// ===========================================================================

// ListEarnStrategiesService lists the Earn strategies available to the account.
type ListEarnStrategiesService struct {
	c      *Client
	params map[string]string
}

func (c *Client) NewListEarnStrategiesService() *ListEarnStrategiesService {
	return &ListEarnStrategiesService{c: c, params: map[string]string{}}
}

// SetAsset filters strategies by asset.
func (s *ListEarnStrategiesService) SetAsset(asset string) *ListEarnStrategiesService {
	s.params["asset"] = asset
	return s
}

// SetAscending returns strategies in ascending order.
func (s *ListEarnStrategiesService) SetAscending(ascending bool) *ListEarnStrategiesService {
	s.params["ascending"] = formatBool(ascending)
	return s
}

// SetLimit caps the number of strategies returned.
func (s *ListEarnStrategiesService) SetLimit(limit int) *ListEarnStrategiesService {
	s.params["limit"] = formatInt(limit)
	return s
}

// SetCursor sets the pagination cursor.
func (s *ListEarnStrategiesService) SetCursor(cursor string) *ListEarnStrategiesService {
	s.params["cursor"] = cursor
	return s
}

// SetLockType filters by lock type(s) (flex, bonded, timed, instant).
func (s *ListEarnStrategiesService) SetLockType(lockTypes ...string) *ListEarnStrategiesService {
	s.params["lock_type"] = strings.Join(lockTypes, ",")
	return s
}

func (s *ListEarnStrategiesService) Do(ctx context.Context) (*EarnStrategiesResult, error) {
	return request.Do[EarnStrategiesResult](request.Post(ctx, s.c, "/0/private/Earn/Strategies", s.params).WithSign())
}

// EarnStrategiesResult is a page of Earn strategies.
type EarnStrategiesResult struct {
	Items      []EarnStrategy `json:"items"`
	NextCursor string         `json:"next_cursor"`
}

// EarnStrategy describes one Earn strategy.
type EarnStrategy struct {
	ID                        string          `json:"id"`                          // strategy id
	Asset                     string          `json:"asset"`                       // asset that can be allocated
	AssetClass                string          `json:"asset_class"`                 // asset class
	AllocationFee             decimal.Decimal `json:"allocation_fee"`              // fee charged on allocation
	DeallocationFee           decimal.Decimal `json:"deallocation_fee"`            // fee charged on deallocation
	AllocationRestrictionInfo []string        `json:"allocation_restriction_info"` // reasons allocation may be restricted
	APREstimate               EarnAPREstimate `json:"apr_estimate"`                // estimated APR range
	AutoCompound              EarnNamedType   `json:"auto_compound"`               // auto-compound behavior
	CanAllocate               bool            `json:"can_allocate"`                // whether allocation is currently allowed
	CanDeallocate             bool            `json:"can_deallocate"`              // whether deallocation is currently allowed
	LockType                  EarnLockType    `json:"lock_type"`                   // lock/bonding terms
	UserCap                   decimal.Decimal `json:"user_cap"`                    // maximum the user may allocate
	UserMinAllocation         decimal.Decimal `json:"user_min_allocation"`         // minimum the user may allocate
	YieldSource               EarnNamedType   `json:"yield_source"`                // source of the yield
}

// EarnAPREstimate is the estimated APR range for a strategy.
type EarnAPREstimate struct {
	High decimal.Decimal `json:"high"`
	Low  decimal.Decimal `json:"low"`
}

// EarnNamedType is a simple "{type: ...}" descriptor used by auto_compound and
// yield_source.
type EarnNamedType struct {
	Type string `json:"type"`
}

// EarnLockType describes a strategy's lock/bonding terms. Bonding/unbonding
// fields are present only for bonded/timed strategies.
type EarnLockType struct {
	Type                    string `json:"type"`                      // flex, bonded, timed, instant
	PayoutFrequency         int64  `json:"payout_frequency"`          // reward payout frequency (seconds)
	BondingPeriod           int64  `json:"bonding_period"`            // bonding period (seconds)
	BondingPeriodVariable   bool   `json:"bonding_period_variable"`   // whether the bonding period varies
	BondingRewards          bool   `json:"bonding_rewards"`           // whether rewards accrue while bonding
	ExitQueuePeriod         int64  `json:"exit_queue_period"`         // exit-queue period (seconds)
	UnbondingPeriod         int64  `json:"unbonding_period"`          // unbonding period (seconds)
	UnbondingPeriodVariable bool   `json:"unbonding_period_variable"` // whether the unbonding period varies
	UnbondingRewards        bool   `json:"unbonding_rewards"`         // whether rewards accrue while unbonding
}

// ===========================================================================
// 6. List Earn Allocations -- POST /0/private/Earn/Allocations
// ===========================================================================

// ListEarnAllocationsService lists the account's current Earn allocations.
type ListEarnAllocationsService struct {
	c      *Client
	params map[string]string
}

func (c *Client) NewListEarnAllocationsService() *ListEarnAllocationsService {
	return &ListEarnAllocationsService{c: c, params: map[string]string{}}
}

// SetAscending returns allocations in ascending order.
func (s *ListEarnAllocationsService) SetAscending(ascending bool) *ListEarnAllocationsService {
	s.params["ascending"] = formatBool(ascending)
	return s
}

// SetConvertedAsset values allocations in the given asset (default USD).
func (s *ListEarnAllocationsService) SetConvertedAsset(asset string) *ListEarnAllocationsService {
	s.params["converted_asset"] = asset
	return s
}

// SetHideZeroAllocations omits strategies with no current allocation.
func (s *ListEarnAllocationsService) SetHideZeroAllocations(hide bool) *ListEarnAllocationsService {
	s.params["hide_zero_allocations"] = formatBool(hide)
	return s
}

func (s *ListEarnAllocationsService) Do(ctx context.Context) (*EarnAllocationsResult, error) {
	return request.Do[EarnAllocationsResult](request.Post(ctx, s.c, "/0/private/Earn/Allocations", s.params).WithSign())
}

// EarnAllocationsResult is the account's Earn allocation summary.
type EarnAllocationsResult struct {
	ConvertedAsset      string           `json:"converted_asset"`       // asset the converted values are in
	ConvertedAssetClass string           `json:"converted_asset_class"` // class of the converted asset
	TotalAllocated      decimal.Decimal  `json:"total_allocated"`       // total allocated (converted asset)
	TotalRewarded       decimal.Decimal  `json:"total_rewarded"`        // total rewarded (converted asset)
	NextCursor          string           `json:"next_cursor"`           // pagination cursor
	Items               []EarnAllocation `json:"items"`                 // per-strategy allocations
}

// EarnAllocation is the account's allocation to a single strategy.
type EarnAllocation struct {
	StrategyID      string              `json:"strategy_id"`      // strategy id
	NativeAsset     string              `json:"native_asset"`     // asset allocated
	AssetClass      string              `json:"asset_class"`      // asset class of the allocated asset
	IsUtilized      bool                `json:"is_utilized"`      // whether the allocation is currently utilized
	AmountAllocated EarnAmountAllocated `json:"amount_allocated"` // breakdown of allocated amounts
	TotalRewarded   EarnConvertedAmount `json:"total_rewarded"`   // total rewards earned
	Payout          EarnPayout          `json:"payout"`           // current payout period info
}

// EarnConvertedAmount is an amount expressed in both the native and converted
// assets.
type EarnConvertedAmount struct {
	Native    decimal.Decimal `json:"native"`    // amount in the native asset
	Converted decimal.Decimal `json:"converted"` // amount in the converted asset
}

// EarnAmountAllocated breaks an allocation into its lifecycle states.
type EarnAmountAllocated struct {
	Bonding   EarnAllocationState `json:"bonding"`    // amount currently bonding
	ExitQueue EarnAllocationState `json:"exit_queue"` // amount in the exit queue
	Pending   EarnConvertedAmount `json:"pending"`    // amount pending allocation
	Unbonding EarnAllocationState `json:"unbonding"`  // amount currently unbonding
	Total     EarnConvertedAmount `json:"total"`      // total allocated
}

// EarnAllocationState is an allocation lifecycle bucket with its individual
// allocation records.
type EarnAllocationState struct {
	Native          decimal.Decimal        `json:"native"`           // amount in native asset
	Converted       decimal.Decimal        `json:"converted"`        // amount in converted asset
	AllocationCount int                    `json:"allocation_count"` // number of individual allocations
	Allocations     []EarnAllocationDetail `json:"allocations"`      // individual allocation records
}

// EarnAllocationDetail is one individual allocation record within a state.
type EarnAllocationDetail struct {
	Native    decimal.Decimal `json:"native"`     // amount in the native asset
	Converted decimal.Decimal `json:"converted"`  // amount in the converted asset
	CreatedAt time.Time       `json:"created_at"` // when the allocation was created
	Expires   time.Time       `json:"expires"`    // when bonding/unbonding completes
}

// EarnPayout describes the current reward payout period.
type EarnPayout struct {
	AccumulatedReward EarnConvertedAmount `json:"accumulated_reward"` // reward accrued this period
	EstimatedReward   EarnConvertedAmount `json:"estimated_reward"`   // estimated reward for the period
	PeriodStart       time.Time           `json:"period_start"`       // start of the payout period
	PeriodEnd         time.Time           `json:"period_end"`         // end of the payout period
}
