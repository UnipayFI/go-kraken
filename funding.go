package kraken

import (
	"context"
	"time"

	"github.com/UnipayFI/go-kraken/request"
	"github.com/shopspring/decimal"
)

// ===========================================================================
// 1. Get Deposit Methods -- POST /0/private/DepositMethods
// ===========================================================================

// GetDepositMethodsService lists the deposit methods available for an asset.
type GetDepositMethodsService struct {
	c      *Client
	params map[string]string
}

// NewGetDepositMethodsService lists deposit methods for the given asset.
func (c *Client) NewGetDepositMethodsService(asset string) *GetDepositMethodsService {
	return &GetDepositMethodsService{c: c, params: map[string]string{"asset": asset}}
}

// SetAssetClass filters by asset class (default currency).
func (s *GetDepositMethodsService) SetAssetClass(aclass string) *GetDepositMethodsService {
	s.params["aclass"] = aclass
	return s
}

func (s *GetDepositMethodsService) Do(ctx context.Context) ([]DepositMethod, error) {
	resp, err := request.Do[[]DepositMethod](request.Post(ctx, s.c, "/0/private/DepositMethods", s.params).WithSign())
	if err != nil {
		return nil, err
	}
	return *resp, nil
}

// DepositMethod is one available deposit method.
type DepositMethod struct {
	Method     string          `json:"method"`      // name of the deposit method
	Limit      MethodLimit     `json:"limit"`       // deposit limit (false if none)
	Fee        decimal.Decimal `json:"fee"`         // deposit fee, if any
	GenAddress bool            `json:"gen-address"` // whether new addresses can be generated
	Minimum    decimal.Decimal `json:"minimum"`     // minimum deposit amount
}

// ===========================================================================
// 2. Get Deposit Addresses -- POST /0/private/DepositAddresses
// ===========================================================================

// GetDepositAddressesService lists (or generates) deposit addresses for an
// asset and method.
type GetDepositAddressesService struct {
	c      *Client
	params map[string]string
}

// NewGetDepositAddressesService lists addresses for asset/method.
func (c *Client) NewGetDepositAddressesService(asset, method string) *GetDepositAddressesService {
	return &GetDepositAddressesService{c: c, params: map[string]string{
		"asset":  asset,
		"method": method,
	}}
}

// SetNew generates a new address (instead of returning existing ones).
func (s *GetDepositAddressesService) SetNew(newAddr bool) *GetDepositAddressesService {
	s.params["new"] = formatBool(newAddr)
	return s
}

// SetAmount requests an address for a Lightning deposit of the given amount.
func (s *GetDepositAddressesService) SetAmount(amount decimal.Decimal) *GetDepositAddressesService {
	s.params["amount"] = amount.String()
	return s
}

func (s *GetDepositAddressesService) Do(ctx context.Context) ([]DepositAddress, error) {
	resp, err := request.Do[[]DepositAddress](request.Post(ctx, s.c, "/0/private/DepositAddresses", s.params).WithSign())
	if err != nil {
		return nil, err
	}
	return *resp, nil
}

// DepositAddress is one deposit address.
type DepositAddress struct {
	Address    string    `json:"address"`  // deposit address
	ExpireTime time.Time `json:"expiretm"` // expiration time (zero if it does not expire)
	New        bool      `json:"new"`      // whether the address was newly generated
	Memo       string    `json:"memo"`     // memo for the deposit (some assets)
	Tag        string    `json:"tag"`      // destination tag (some assets)
}

// ===========================================================================
// 3. Get Status of Recent Deposits -- POST /0/private/DepositStatus
// ===========================================================================

// GetDepositStatusService lists recent deposits.
type GetDepositStatusService struct {
	c      *Client
	params map[string]string
}

func (c *Client) NewGetDepositStatusService() *GetDepositStatusService {
	return &GetDepositStatusService{c: c, params: map[string]string{}}
}

// SetAsset filters by asset.
func (s *GetDepositStatusService) SetAsset(asset string) *GetDepositStatusService {
	s.params["asset"] = asset
	return s
}

// SetAssetClass filters by asset class (default currency).
func (s *GetDepositStatusService) SetAssetClass(aclass string) *GetDepositStatusService {
	s.params["aclass"] = aclass
	return s
}

// SetMethod filters by deposit method.
func (s *GetDepositStatusService) SetMethod(method string) *GetDepositStatusService {
	s.params["method"] = method
	return s
}

func (s *GetDepositStatusService) Do(ctx context.Context) ([]TransferStatus, error) {
	resp, err := request.Do[[]TransferStatus](request.Post(ctx, s.c, "/0/private/DepositStatus", s.params).WithSign())
	if err != nil {
		return nil, err
	}
	return *resp, nil
}

// TransferStatus is one deposit or withdrawal record. Withdrawal-only fields
// (key, network) are empty for deposits.
type TransferStatus struct {
	Method     string          `json:"method"`      // transfer method
	AssetClass string          `json:"aclass"`      // asset class
	Asset      string          `json:"asset"`       // asset
	RefID      string          `json:"refid"`       // reference id
	TxID       string          `json:"txid"`        // on-chain transaction id
	Info       string          `json:"info"`        // address / info
	Amount     decimal.Decimal `json:"amount"`      // amount
	Fee        decimal.Decimal `json:"fee"`         // fee
	Time       time.Time       `json:"time"`        // unix timestamp of the transfer
	Status     string          `json:"status"`      // status (Success, Failure, Pending, ...)
	StatusProp string          `json:"status-prop"` // additional status property (return, onhold, ...)
	Key        string          `json:"key"`         // withdrawal key name (withdrawals only)
	Network    string          `json:"network"`     // network (withdrawals only)
}

// ===========================================================================
// 4. Get Withdrawal Methods -- POST /0/private/WithdrawMethods
// ===========================================================================

// GetWithdrawMethodsService lists the withdrawal methods available to the user.
type GetWithdrawMethodsService struct {
	c      *Client
	params map[string]string
}

func (c *Client) NewGetWithdrawMethodsService() *GetWithdrawMethodsService {
	return &GetWithdrawMethodsService{c: c, params: map[string]string{}}
}

// SetAsset filters by asset.
func (s *GetWithdrawMethodsService) SetAsset(asset string) *GetWithdrawMethodsService {
	s.params["asset"] = asset
	return s
}

// SetAssetClass filters by asset class (default currency).
func (s *GetWithdrawMethodsService) SetAssetClass(aclass string) *GetWithdrawMethodsService {
	s.params["aclass"] = aclass
	return s
}

// SetNetwork filters by network name.
func (s *GetWithdrawMethodsService) SetNetwork(network string) *GetWithdrawMethodsService {
	s.params["network"] = network
	return s
}

func (s *GetWithdrawMethodsService) Do(ctx context.Context) ([]WithdrawMethod, error) {
	resp, err := request.Do[[]WithdrawMethod](request.Post(ctx, s.c, "/0/private/WithdrawMethods", s.params).WithSign())
	if err != nil {
		return nil, err
	}
	return *resp, nil
}

// WithdrawMethod is one available withdrawal method.
type WithdrawMethod struct {
	Asset     string            `json:"asset"`      // asset
	Method    string            `json:"method"`     // withdrawal method name
	MethodID  string            `json:"method_id"`  // method identifier
	Network   string            `json:"network"`    // network name
	NetworkID string            `json:"network_id"` // network identifier
	Minimum   decimal.Decimal   `json:"minimum"`    // minimum withdrawal amount
	Fee       WithdrawMethodFee `json:"fee"`        // withdrawal fee
	Limits    []WithdrawLimit   `json:"limits"`     // withdrawal limits per dimension
}

// WithdrawMethodFee is the fee charged for a withdrawal method.
type WithdrawMethodFee struct {
	AssetClass string          `json:"aclass"` // asset class of the fee
	Asset      string          `json:"asset"`  // asset the fee is charged in
	Fee        decimal.Decimal `json:"fee"`    // fee amount
}

// WithdrawLimit is one withdrawal-limit dimension (e.g. by USD-equivalent or by
// asset amount), keyed by rolling-window length in seconds.
type WithdrawLimit struct {
	Description string                         `json:"description"` // human description
	LimitType   string                         `json:"limit_type"`  // equiv_amount | amount
	Limits      map[string]WithdrawLimitWindow `json:"limits"`      // window-seconds -> usage
}

// WithdrawLimitWindow is the usage of one rolling limit window.
type WithdrawLimitWindow struct {
	Maximum   decimal.Decimal `json:"maximum"`   // maximum allowed in the window
	Remaining decimal.Decimal `json:"remaining"` // remaining allowance
	Used      decimal.Decimal `json:"used"`      // amount already used
}

// ===========================================================================
// 5. Get Withdrawal Addresses -- POST /0/private/WithdrawAddresses
// ===========================================================================

// GetWithdrawAddressesService lists the withdrawal addresses set up on the
// account.
type GetWithdrawAddressesService struct {
	c      *Client
	params map[string]string
}

func (c *Client) NewGetWithdrawAddressesService() *GetWithdrawAddressesService {
	return &GetWithdrawAddressesService{c: c, params: map[string]string{}}
}

// SetAsset filters by asset.
func (s *GetWithdrawAddressesService) SetAsset(asset string) *GetWithdrawAddressesService {
	s.params["asset"] = asset
	return s
}

// SetAssetClass filters by asset class (default currency).
func (s *GetWithdrawAddressesService) SetAssetClass(aclass string) *GetWithdrawAddressesService {
	s.params["aclass"] = aclass
	return s
}

// SetMethod filters by withdrawal method.
func (s *GetWithdrawAddressesService) SetMethod(method string) *GetWithdrawAddressesService {
	s.params["method"] = method
	return s
}

// SetKey filters by withdrawal key name.
func (s *GetWithdrawAddressesService) SetKey(key string) *GetWithdrawAddressesService {
	s.params["key"] = key
	return s
}

// SetVerified filters by whether the address is verified.
func (s *GetWithdrawAddressesService) SetVerified(verified bool) *GetWithdrawAddressesService {
	s.params["verified"] = formatBool(verified)
	return s
}

func (s *GetWithdrawAddressesService) Do(ctx context.Context) ([]WithdrawAddress, error) {
	resp, err := request.Do[[]WithdrawAddress](request.Post(ctx, s.c, "/0/private/WithdrawAddresses", s.params).WithSign())
	if err != nil {
		return nil, err
	}
	return *resp, nil
}

// WithdrawAddress is one saved withdrawal address.
type WithdrawAddress struct {
	Address  string `json:"address"`  // withdrawal address
	Asset    string `json:"asset"`    // asset
	Method   string `json:"method"`   // withdrawal method
	Key      string `json:"key"`      // address key name
	Memo     string `json:"memo"`     // memo (some assets)
	Tag      string `json:"tag"`      // destination tag (some assets)
	Network  string `json:"network"`  // network (if applicable)
	Verified bool   `json:"verified"` // whether the address is verified
}

// ===========================================================================
// 6. Get Withdrawal Information -- POST /0/private/WithdrawInfo
// ===========================================================================

// GetWithdrawInfoService returns fee and limit information for a prospective
// withdrawal, without performing it.
type GetWithdrawInfoService struct {
	c      *Client
	params map[string]string
}

// NewGetWithdrawInfoService queries withdrawal info for the asset, withdrawal
// key name and amount.
func (c *Client) NewGetWithdrawInfoService(asset, key string, amount decimal.Decimal) *GetWithdrawInfoService {
	return &GetWithdrawInfoService{c: c, params: map[string]string{
		"asset":  asset,
		"key":    key,
		"amount": amount.String(),
	}}
}

func (s *GetWithdrawInfoService) Do(ctx context.Context) (*WithdrawInfo, error) {
	return request.Do[WithdrawInfo](request.Post(ctx, s.c, "/0/private/WithdrawInfo", s.params).WithSign())
}

// WithdrawInfo is the fee/limit preview for a withdrawal.
type WithdrawInfo struct {
	Method string          `json:"method"` // withdrawal method that will be used
	Limit  decimal.Decimal `json:"limit"`  // maximum net amount withdrawable now
	Amount decimal.Decimal `json:"amount"` // net amount that will be sent, after fees
	Fee    decimal.Decimal `json:"fee"`    // fees that will be paid
}

// ===========================================================================
// 7. Withdraw Funds -- POST /0/private/Withdraw  (implemented; not tested)
// ===========================================================================

// WithdrawService submits a withdrawal to a pre-configured withdrawal address
// (identified by its key name). This is a fund-moving endpoint; it is provided
// for completeness and is intentionally never exercised by the test suite.
type WithdrawService struct {
	c      *Client
	params map[string]string
}

// NewWithdrawService withdraws amount of asset to the withdrawal key.
func (c *Client) NewWithdrawService(asset, key string, amount decimal.Decimal) *WithdrawService {
	return &WithdrawService{c: c, params: map[string]string{
		"asset":  asset,
		"key":    key,
		"amount": amount.String(),
	}}
}

// SetAssetClass sets the asset class of the asset being withdrawn.
func (s *WithdrawService) SetAssetClass(aclass string) *WithdrawService {
	s.params["aclass"] = aclass
	return s
}

// SetAddress confirms the withdrawal address matches the key.
func (s *WithdrawService) SetAddress(address string) *WithdrawService {
	s.params["address"] = address
	return s
}

// SetMaxFee caps the acceptable fee; the withdrawal fails if exceeded.
func (s *WithdrawService) SetMaxFee(maxFee decimal.Decimal) *WithdrawService {
	s.params["max_fee"] = maxFee.String()
	return s
}

func (s *WithdrawService) Do(ctx context.Context) (*WithdrawRef, error) {
	return request.Do[WithdrawRef](request.Post(ctx, s.c, "/0/private/Withdraw", s.params).WithSign())
}

// WithdrawRef references a submitted withdrawal.
type WithdrawRef struct {
	RefID string `json:"refid"` // reference id of the withdrawal
}

// ===========================================================================
// 8. Get Status of Recent Withdrawals -- POST /0/private/WithdrawStatus
// ===========================================================================

// GetWithdrawStatusService lists recent withdrawals.
type GetWithdrawStatusService struct {
	c      *Client
	params map[string]string
}

func (c *Client) NewGetWithdrawStatusService() *GetWithdrawStatusService {
	return &GetWithdrawStatusService{c: c, params: map[string]string{}}
}

// SetAsset filters by asset.
func (s *GetWithdrawStatusService) SetAsset(asset string) *GetWithdrawStatusService {
	s.params["asset"] = asset
	return s
}

// SetAssetClass filters by asset class (default currency).
func (s *GetWithdrawStatusService) SetAssetClass(aclass string) *GetWithdrawStatusService {
	s.params["aclass"] = aclass
	return s
}

// SetMethod filters by withdrawal method.
func (s *GetWithdrawStatusService) SetMethod(method string) *GetWithdrawStatusService {
	s.params["method"] = method
	return s
}

func (s *GetWithdrawStatusService) Do(ctx context.Context) ([]TransferStatus, error) {
	resp, err := request.Do[[]TransferStatus](request.Post(ctx, s.c, "/0/private/WithdrawStatus", s.params).WithSign())
	if err != nil {
		return nil, err
	}
	return *resp, nil
}

// ===========================================================================
// 9. Request Withdrawal Cancellation -- POST /0/private/WithdrawCancel
// ===========================================================================

// WithdrawCancelService requests cancellation of a recently-requested
// withdrawal that has not yet been fully processed.
type WithdrawCancelService struct {
	c      *Client
	params map[string]string
}

// NewWithdrawCancelService cancels the withdrawal identified by asset and refid.
func (c *Client) NewWithdrawCancelService(asset, refID string) *WithdrawCancelService {
	return &WithdrawCancelService{c: c, params: map[string]string{
		"asset": asset,
		"refid": refID,
	}}
}

// Do reports whether cancellation was accepted. Kraken returns a bare boolean.
func (s *WithdrawCancelService) Do(ctx context.Context) (bool, error) {
	resp, err := request.Do[bool](request.Post(ctx, s.c, "/0/private/WithdrawCancel", s.params).WithSign())
	if err != nil {
		return false, err
	}
	return *resp, nil
}

// ===========================================================================
// 10. Request Wallet Transfer -- POST /0/private/WalletTransfer
// ===========================================================================

// WalletTransferService transfers funds from the Kraken Spot wallet to the
// Kraken Futures wallet. (Reverse transfers use the Futures API.)
type WalletTransferService struct {
	c      *Client
	params map[string]string
}

// NewWalletTransferService transfers amount of asset from "Spot Wallet" to
// "Futures Wallet".
func (c *Client) NewWalletTransferService(asset string, amount decimal.Decimal) *WalletTransferService {
	return &WalletTransferService{c: c, params: map[string]string{
		"asset":  asset,
		"amount": amount.String(),
		"from":   "Spot Wallet",
		"to":     "Futures Wallet",
	}}
}

// SetFrom overrides the source wallet (default "Spot Wallet").
func (s *WalletTransferService) SetFrom(from string) *WalletTransferService {
	s.params["from"] = from
	return s
}

// SetTo overrides the destination wallet (default "Futures Wallet").
func (s *WalletTransferService) SetTo(to string) *WalletTransferService {
	s.params["to"] = to
	return s
}

func (s *WalletTransferService) Do(ctx context.Context) (*WalletTransferRef, error) {
	return request.Do[WalletTransferRef](request.Post(ctx, s.c, "/0/private/WalletTransfer", s.params).WithSign())
}

// WalletTransferRef references a wallet transfer.
type WalletTransferRef struct {
	RefID string `json:"refid"` // reference id of the transfer
}
