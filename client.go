package httpx

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/textproto"
	"time"
)

// DefaultClient is a simple wrapper around http.DefaultClient
var DefaultClient Client = http.DefaultClient

// Client performs an http request and returns a response and error.
//
// When implementing, if the error is not nil, the response may or may not be nil.
// If the error is nil then the response should not be nil
type Client interface {
	Do(req *http.Request) (*http.Response, error)
}

// ClientFunc is an adapter to allow the use of ordinary functions as HTTP Clients.
//
// If f is a function with the appropriate signature, ClientFunc(f) is a Client that calls f.
type ClientFunc func(req *http.Request) (*http.Response, error)

// Do calls f(req) and returns the result
func (f ClientFunc) Do(req *http.Request) (*http.Response, error) {
	return f(req)
}

// nilClientCheck returns the DefaultClient if c is not set
func nilClientCheck(c Client) Client {
	if c == nil {
		c = DefaultClient
	}
	return c
}

// nilRequestCheck returns an error when there is no request set
func nilRequestCheck(c ClientFunc) ClientFunc {
	return func(req *http.Request) (*http.Response, error) {
		if req == nil {
			return nil, fmt.Errorf("missing request in call to (Client).Do")
		}
		return c.Do(req)
	}
}

// SetRequest adds a request to the client to perform when the client calls Do.
//
// This overrides any existing request. Generally it should be the last decoration before calling (Client).Do
func SetRequest(c Client, method string, url string) ClientFunc {
	return SetRequestWithContext(context.Background(), c, method, url)
}

// SetRequestWithContext adds a request with context to the client to perform when the client calls Do.
//
// This overrides any existing request. Generally it should be the last decoration before calling (Client).Do
func SetRequestWithContext(ctx context.Context, c Client, method string, url string) ClientFunc {
	return func(_ *http.Request) (*http.Response, error) {
		req, err := http.NewRequestWithContext(ctx, method, url, nil)
		if err != nil {
			return nil, err
		}
		return c.Do(req)
	}
}

// RequireResponseBody returns a non-nil error if the response body is nil
func RequireResponseBody(c Client) ClientFunc {
	c = nilClientCheck(c)
	return nilRequestCheck(func(req *http.Request) (*http.Response, error) {
		resp, err := c.Do(req)
		if err != nil {
			return resp, err
		}
		if resp.Body == nil {
			return resp, fmt.Errorf("expected non-nil response body")
		}
		return resp, nil
	})
}

// RequireResponseStatus returns a non-nil error if the response status does not match one of the statuses given
func RequireResponseStatus(c Client, status ...int) ClientFunc {
	c = nilClientCheck(c)
	if len(status) == 0 {
		status = []int{http.StatusOK}
	}
	valid := make(map[int]bool, len(status))
	for _, s := range status {
		valid[s] = true
	}
	return nilRequestCheck(func(req *http.Request) (*http.Response, error) {
		resp, err := c.Do(req)
		if err != nil {
			return resp, err
		}
		if !valid[resp.StatusCode] {
			return resp, fmt.Errorf("received invalid satus code: %d", resp.StatusCode)
		}
		return resp, nil
	})
}

// SetHeader sets a header value on the request before the request is executed
func SetHeader(c Client, key string, value ...string) ClientFunc {
	c = nilClientCheck(c)
	key = textproto.CanonicalMIMEHeaderKey(key)
	return nilRequestCheck(func(req *http.Request) (*http.Response, error) {
		if req.Header == nil {
			req.Header = make(http.Header)
		}
		req.Header[key] = value
		return c.Do(req)
	})
}

// AddHeader appends a header value on the request before the request is executed
func AddHeader(c Client, key string, value ...string) ClientFunc {
	c = nilClientCheck(c)
	key = textproto.CanonicalMIMEHeaderKey(key)
	return nilRequestCheck(func(req *http.Request) (*http.Response, error) {
		if req.Header == nil {
			req.Header = make(http.Header)
		}
		req.Header[key] = append(req.Header[key], value...)
		return c.Do(req)
	})
}

// Marshaller accepts a single parameter and returns a byte slice and error
type Marshaller func(v any) ([]byte, error)

// Unmarshaller decodes the byte array into the given pointer
type Unmarshaller func(b []byte, v any) error

// SetRequestBody sets the value v to the request body using the given Marshaller
func SetRequestBody(c Client, m Marshaller, v any) ClientFunc {
	c = nilClientCheck(c)
	return nilRequestCheck(func(req *http.Request) (*http.Response, error) {
		if m == nil {
			switch t := v.(type) {
			case []byte:
				req.Body = io.NopCloser(bytes.NewReader(t))
			case io.ReadCloser:
				req.Body = t
			case io.Reader:
				req.Body = io.NopCloser(t)
			default:
				return nil, fmt.Errorf("could not marshal body type %T", v)
			}
		} else {
			b, err := m(v)
			if err != nil {
				return nil, fmt.Errorf("could not marshal request body: %w", err)
			}
			req.Body = io.NopCloser(bytes.NewReader(b))
		}
		return c.Do(req)
	})
}

// SetRequestBodyJSON is a helper function around SetHeader and SetRequestBody for json specific encoding
func SetRequestBodyJSON(c Client, v any) ClientFunc {
	c = SetHeader(c, "Content-Type", "application/json")
	return SetRequestBody(c, json.Marshal, v)
}

// SetResponseBodyHandler adds a function to unmarshal the response body into a given pointer ptr
func SetResponseBodyHandler(c Client, u Unmarshaller, ptr any) ClientFunc {
	return RequireResponseBody(ClientFunc(
		func(req *http.Request) (*http.Response, error) {
			resp, err := c.Do(req)
			if err != nil {
				return resp, err
			}
			b, err := io.ReadAll(resp.Body)
			closeErr := resp.Body.Close()
			if err != nil {
				return resp, err
			}
			resp.Body = io.NopCloser(bytes.NewBuffer(b))
			if err = u(b, ptr); err != nil {
				return resp, err
			}
			if closeErr != nil {
				return resp, errBodyCloser{next: closeErr}
			}
			return resp, nil
		}),
	)
}

// SetResponseJSONReader performs the request and attempts to unmarshal the response body as json
func SetResponseBodyHandlerJSON(c Client, ptr any) ClientFunc {
	c = SetHeader(c, "Accept", "application/json")
	return SetResponseBodyHandler(c, json.Unmarshal, ptr)
}

// SetTimeout sets a time limit on the entire lifetime of the request including connection and header reads
func SetTimeout(c Client, d time.Duration) ClientFunc {
	c = nilClientCheck(c)
	return nilRequestCheck(func(req *http.Request) (*http.Response, error) {
		ctx, cancel := context.WithTimeout(req.Context(), d)
		defer cancel()
		req = req.Clone(ctx)
		return c.Do(req)
	})
}

// AddCookie adds a cookie to the request
func AddCookies(c Client, cookie ...*http.Cookie) ClientFunc {
	c = nilClientCheck(c)
	if len(cookie) == 0 {
		return c.Do
	}
	return nilRequestCheck(func(req *http.Request) (*http.Response, error) {
		for _, cookie := range cookie {
			req.AddCookie(cookie)
		}
		return c.Do(req)
	})
}

// SetCookies clears any existing cookies on the request and sets the value to the cookies given
//
// if the underlying Client implements a cookie jar those cookies in the jar are not removed
func SetCookies(c Client, cookie ...*http.Cookie) ClientFunc {
	// clear previous Cookie header and add any new ones
	return SetHeader(AddCookies(c, cookie...), "Cookie", "")
}
