package httpx_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

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
