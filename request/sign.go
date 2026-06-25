package request

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"fmt"

	"github.com/UnipayFI/go-kraken/common"
)

// SignFn mirrors client.SignFn: it turns the request components into the
// API-Sign header value, given the configured (base64-encoded) secret.
type SignFn = func(secret, uriPath, nonce, postData string) (signature string, err error)

// HMACSign is Kraken's default request signer:
//
//	API-Sign = base64( HMAC-SHA512( base64decode(secret), uriPath + SHA256(nonce + postData) ) )
//
// where uriPath is the full path (e.g. "/0/private/AddOrder"), postData is the
// url-encoded POST body (which itself begins with "nonce=..."), and nonce is
// that same nonce value as a string. The secret is the base64 "Private Key"
// from the Kraken API-management page and is decoded to raw bytes before use as
// the HMAC key.
func HMACSign(secret, uriPath, nonce, postData string) (string, error) {
	decoded, err := base64.StdEncoding.DecodeString(secret)
	if err != nil {
		return "", fmt.Errorf("kraken: api secret is not valid base64: %w", err)
	}
	sha := sha256.Sum256(common.StringToBytes(nonce + postData))
	mac := hmac.New(sha512.New, decoded)
	mac.Write(common.StringToBytes(uriPath))
	mac.Write(sha[:])
	return base64.StdEncoding.EncodeToString(mac.Sum(nil)), nil
}
