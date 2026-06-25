package client

import (
	"fmt"
	"strings"
)

// APIError is the error envelope Kraken returns. Every REST response carries an
// "error" array of strings; it is empty on success and otherwise holds one or
// more error codes of the form "ECategory:Detail" (e.g. "EOrder:Insufficient
// funds", "EAPI:Invalid nonce", "EGeneral:Invalid arguments").
type APIError struct {
	Errors []string `json:"error"`
}

// Error returns the joined Kraken error strings.
func (e *APIError) Error() string {
	return fmt.Sprintf("<APIError> %s", strings.Join(e.Errors, "; "))
}

// IsValid reports whether e represents an actual API-level error (a non-empty
// error array).
func (e *APIError) IsValid() bool {
	return len(e.Errors) > 0
}

// Has reports whether any of the returned error strings contains substr. Kraken
// codes are not stable enough to match exactly across endpoints, so callers
// match on a substring such as "Invalid nonce" or "Insufficient funds".
func (e *APIError) Has(substr string) bool {
	for _, s := range e.Errors {
		if strings.Contains(s, substr) {
			return true
		}
	}
	return false
}

// IsAPIError reports whether err is a Kraken *APIError.
func IsAPIError(err error) bool {
	_, ok := err.(*APIError)
	return ok
}
