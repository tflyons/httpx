package httpx_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/tflyons/httpx"
)

var echoHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	h := w.Header()
	for k, v := range r.Header {
		for i := range v {
			h.Add(k, v[i])
		}
	}
	for _, cookie := range r.Cookies() {
		http.SetCookie(w, cookie)
	}
	io.Copy(w, r.Body)
})

func TestClient_JSON(t *testing.T) {
	srv := httptest.NewServer(echoHandler)
	defer srv.Close()
	var c httpx.Client = srv.Client()

	input := map[string]string{
		"hello": "world",
	}
	output := make(map[string]string)
	c = httpx.SetRequestBodyJSON(c, input)
	c = httpx.SetResponseBodyHandlerJSON(c, &output)

	c = httpx.SetRequest(c, http.MethodPost, srv.URL)
	if _, err := c.Do(nil); err != nil {
		t.Fatal(err)
	}
	if output["hello"] != "world" {
		t.Fatal(output)
	}
}

func TestClient_SetCookies(t *testing.T) {
	srv := httptest.NewServer(echoHandler)
	defer srv.Close()
	var c httpx.Client = srv.Client()

	c = httpx.AddCookies(c, &http.Cookie{
		Name:   "my_cookie",
		Domain: "",
		Path:   "",
	})
	c = httpx.SetRequest(c, http.MethodGet, srv.URL)
	resp, err := c.Do(nil)
	if err != nil {
		t.Fatal(err)
	}
	cookie := resp.Cookies()
	if cookie[0].Name != "my_cookie" {
		t.Fatal(cookie[0])
	}
}

func TestRateLimit(t *testing.T) {
	srv := httptest.NewServer(echoHandler)
	defer srv.Close()
	var c httpx.Client = srv.Client()

	rateLimit := 20
	d := time.Millisecond * 50
	n := 10
	c = httpx.SetRateLimit(c, rateLimit, d)
	c = httpx.SetRequest(c, http.MethodGet, srv.URL)

	// make enough requests to get rate limited n times
	start := time.Now()
	for i := 0; i < rateLimit*n; i++ {
		if _, err := c.Do(nil); err != nil {
			t.Fatal(err)
		}
	}
	end := time.Now()

	// the time difference should be greater than or equal to n durations
	if end.Sub(start) < d*time.Duration(n) {
		t.Fatal("expected time delay due to rate limit")
	}
}

func ExampleClient() {
	c := httpx.DefaultClient
	// set a header to be sent on every request
	c = httpx.SetHeader(c, "some-header", "1234")
	// limit the total request volume to 100 per minute
	c = httpx.SetRateLimit(c, 100, time.Minute)
	// limit every request to 30 seconds round trip
	c = httpx.SetTimeout(c, time.Second*30)
	// set an initializer to load a token from a file into the header prior to doing calls
	c = httpx.SetInitializer(c, func(next httpx.Client) (httpx.ClientFunc, error) {
		token, err := os.ReadFile("mytoken.txt")
		if err != nil {
			return nil, err
		}
		return httpx.SetHeader(next, "SOME-TOKEN", string(token)), nil
	})
}
