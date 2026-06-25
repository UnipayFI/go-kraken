package kraken

import (
	"context"

	"github.com/UnipayFI/go-kraken/request"
	"github.com/shopspring/decimal"
)

// Subaccount endpoints are available to institutional clients only; on a
// standard account they return a permission error (the request path and signing
// are still exercised).

// ===========================================================================
// 1. Create Subaccount -- POST /0/private/CreateSubaccount
// ===========================================================================

// CreateSubaccountService creates a trading subaccount (institutional accounts
// only).
type CreateSubaccountService struct {
	c      *Client
	params map[string]string
}

// NewCreateSubaccountService creates a subaccount with the given username and
// email.
func (c *Client) NewCreateSubaccountService(username, email string) *CreateSubaccountService {
	return &CreateSubaccountService{c: c, params: map[string]string{
		"username": username,
		"email":    email,
	}}
}

// Do reports whether the subaccount was created (Kraken returns a bare boolean).
func (s *CreateSubaccountService) Do(ctx context.Context) (bool, error) {
	resp, err := request.Do[bool](request.Post(ctx, s.c, "/0/private/CreateSubaccount", s.params).WithSign())
	if err != nil {
		return false, err
	}
	return *resp, nil
}

// ===========================================================================
// 2. Account Transfer -- POST /0/private/AccountTransfer
// ===========================================================================

// AccountTransferService transfers funds between the master account and a
// subaccount, or between subaccounts (institutional accounts only). The API key
// must belong to the master account.
type AccountTransferService struct {
	c      *Client
	params map[string]string
}

// NewAccountTransferService transfers amount of asset from the source account id
// to the destination account id.
func (c *Client) NewAccountTransferService(asset string, amount decimal.Decimal, from, to string) *AccountTransferService {
	return &AccountTransferService{c: c, params: map[string]string{
		"asset":  asset,
		"amount": amount.String(),
		"from":   from,
		"to":     to,
	}}
}

// SetAssetClass sets the asset class of the asset being transferred.
func (s *AccountTransferService) SetAssetClass(aclass string) *AccountTransferService {
	s.params["asset_class"] = aclass
	return s
}

func (s *AccountTransferService) Do(ctx context.Context) (*AccountTransferResult, error) {
	return request.Do[AccountTransferResult](request.Post(ctx, s.c, "/0/private/AccountTransfer", s.params).WithSign())
}

// AccountTransferResult is the AccountTransfer response.
type AccountTransferResult struct {
	TransferID string `json:"transfer_id"` // transfer id
	Status     string `json:"status"`      // pending or complete
}
