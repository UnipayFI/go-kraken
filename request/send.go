package request

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/UnipayFI/go-kraken/client"
	"github.com/UnipayFI/go-kraken/common"
	"github.com/go-json-experiment/json/jsontext"
)

// apiResponse is Kraken's uniform REST envelope. "error" is empty on success
// and otherwise holds "ECategory:Detail" strings; "result" carries the
// endpoint-specific payload.
type apiResponse[T any] struct {
	Error  []string `json:"error"`
	Result T        `json:"result"`
}

// Do executes the request and decodes the envelope's result field into *T. A
// non-empty error array is returned as a *client.APIError.
func Do[T any](r *Request) (resp *T, err error) {
	if r.err != nil {
		return nil, r.err
	}
	if err = r.prepare(); err != nil {
		return nil, err
	}

	r.client.GetLogger().Debugf("request: %s %s", r.method, r.r.URL)
	defer func() {
		if err != nil {
			r.client.GetLogger().Errorf("request %s %s failed: %s", r.method, r.r.URL, err)
		}
	}()

	response, err := r.r.Send()
	if err != nil {
		return nil, err
	}
	body := response.Body()
	r.client.GetLogger().Debugf("response: %s", common.BytesToString(body))

	var out apiResponse[T]
	if uerr := r.client.GetHttpClient().JSONUnmarshal(body, &out); uerr != nil {
		// The body was not a well-formed envelope (gateway error, HTML, ...).
		return nil, fmt.Errorf("request failed (status %d): %s", response.StatusCode(), common.BytesToString(body))
	}
	if len(out.Error) > 0 {
		return nil, &client.APIError{Errors: out.Error}
	}
	return &out.Result, nil
}

// DoRawResult executes the request and returns the raw JSON bytes of the
// envelope's "result" field (after verifying the error array is empty). Tests
// use it to diff the real response shape against the typed structs.
func DoRawResult(r *Request) ([]byte, error) {
	if r.err != nil {
		return nil, r.err
	}
	if err := r.prepare(); err != nil {
		return nil, err
	}
	response, err := r.r.Send()
	if err != nil {
		return nil, err
	}
	body := response.Body()
	var env struct {
		Error  []string       `json:"error"`
		Result jsontext.Value `json:"result"`
	}
	if uerr := r.client.GetHttpClient().JSONUnmarshal(body, &env); uerr != nil {
		return nil, fmt.Errorf("request failed (status %d): %s", response.StatusCode(), common.BytesToString(body))
	}
	if len(env.Error) > 0 {
		return nil, &client.APIError{Errors: env.Error}
	}
	return env.Result, nil
}

// DoRaw executes the request and returns the raw, undecoded response body.
func DoRaw(r *Request) ([]byte, error) {
	if r.err != nil {
		return nil, r.err
	}
	if err := r.prepare(); err != nil {
		return nil, err
	}
	response, err := r.r.Send()
	if err != nil {
		return nil, err
	}
	return response.Body(), nil
}

// prepare finalizes the URL, body and (when private) the API-Key / API-Sign
// signing headers. For a signed request the nonce is injected into the body and
// the signature is computed over uriPath + SHA256(nonce + postData), using the
// exact bytes that go on the wire.
func (r *Request) prepare() error {
	r.r.URL = r.fullURL()
	r.r.Method = r.method

	if !r.needSign {
		// Public POST endpoints still carry their params as a form body.
		if r.method == http.MethodPost && len(r.params) > 0 {
			r.r.SetHeader("Content-Type", "application/x-www-form-urlencoded")
			r.r.SetBody(r.params.Encode())
		}
		return nil
	}

	apiKey := r.client.GetAPIKey()
	secret := r.client.GetAPISecret()
	if apiKey == "" || secret == "" {
		return errors.New("missing credentials: configure client.WithAuth(apiKey, apiSecret)")
	}

	nonce := strconv.FormatInt(r.client.Nonce(), 10)
	r.params.Set("nonce", nonce)
	postData := r.params.Encode()

	var (
		sign string
		err  error
	)
	if fn := r.client.GetSignFn(); fn != nil {
		sign, err = fn(secret, r.path, nonce, postData)
	} else {
		sign, err = HMACSign(secret, r.path, nonce, postData)
	}
	if err != nil {
		return err
	}

	r.r.SetHeader("Content-Type", "application/x-www-form-urlencoded")
	r.r.SetHeader("API-Key", apiKey)
	r.r.SetHeader("API-Sign", sign)
	r.r.SetBody(postData)
	return nil
}
