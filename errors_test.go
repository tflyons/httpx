package httpx_test

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/tflyons/httpx"
)

func badClose(c httpx.Client) httpx.ClientFunc {
	return func(req *http.Request) (*http.Response, error) {
		resp, err := c.Do(req)
		resp.Body = badCloser{resp.Body}
		return resp, err
	}
}

type badCloser struct {
	io.Reader
}

func (badCloser) Close() error { return fmt.Errorf("close body error") }

func TestClient_CloseBodyError(t *testing.T) {
	srv := httptest.NewServer(echoHandler)
	defer srv.Close()
	var c httpx.Client = srv.Client()

	// add a function that will cause the call to resp.Body.Close to error
	c = httpx.SetRequestBodyJSON(c, map[string]string{})
	var out map[string]string
	c = badClose(c)
	c = httpx.SetResponseBodyHandlerJSON(c, &out)

	c = httpx.SetRequest(c, http.MethodPost, srv.URL)
	_, err := c.Do(nil)
	if !errors.Is(err, httpx.ErrBodyClose) {
		t.Fatal(err)
	}
}
