package main

import (
	"fmt"
	"net/http"
	urlpkg "net/url"
	"time"

	"github.com/tflyons/httpx"
)

type MyClient struct {
	c   httpx.Client
	url string
}

func NewMyClient(client httpx.Client, url string) (*MyClient, error) {
	_, err := urlpkg.Parse(url)
	if err != nil {
		return nil, fmt.Errorf("invalid url: %w", err)
	}
	if client == nil {
		client = http.DefaultClient
	}

	m := &MyClient{
		c:   client,
		url: url,
	}
	// all http requests include the login token, a rate limiter and require a 200 response with a non nil body
	m.c = m.decorateLogin(m.c, "tom", "password1") // don't hardcode credentials
	m.c = httpx.SetRateLimit(m.c, 100, time.Minute)
	m.c = httpx.RequireResponseStatus(m.c, http.StatusOK)
	m.c = httpx.RequireResponseBody(m.c)
	return m, nil
}

func (m *MyClient) Foo() (*Foo, error) {
	req, err := http.NewRequest(http.MethodGet, m.url+"/foo", nil)
	if err != nil {
		return nil, err
	}

	c := m.decorateSetSomeHeader(m.c, "hello!")

	var foo Foo
	c = httpx.SetResponseBodyHandlerJSON(c, &foo)
	if _, err = c.Do(req); err != nil {
		return nil, err
	}
	return &foo, nil
}

func (m *MyClient) Bar() (*Bar, error) {
	req, err := http.NewRequest(http.MethodGet, m.url+"/bar", nil)
	if err != nil {
		return nil, err
	}

	var bar Bar
	c := httpx.SetResponseBodyHandlerJSON(m.c, &bar)
	if _, err = c.Do(req); err != nil {
		return nil, err
	}
	return &bar, nil
}

func (m *MyClient) decorateSetSomeHeader(c httpx.Client, v string) httpx.ClientFunc {
	return func(req *http.Request) (*http.Response, error) {
		req.Header.Set("SOME-HEADER", v)
		return c.Do(req)
	}
}

func (m *MyClient) decorateLogin(c httpx.Client, user, pass string) httpx.ClientFunc {
	ch := make(chan struct{}, 1)
	ch <- struct{}{}
	var token string
	return func(req *http.Request) (*http.Response, error) {
		select {
		case <-req.Context().Done():
			return nil, req.Context().Err()
		case _, ok := <-ch:
			if ok {
				req, err := http.NewRequest(http.MethodGet, m.url+"/login", nil)
				if err != nil {
					ch <- struct{}{}
					return nil, err
				}
				req.SetBasicAuth(user, pass)
				resp, err := httpx.RequireResponseStatus(http.DefaultClient, 200).Do(req)
				if err != nil {
					ch <- struct{}{}
					return nil, err
				}
				token = resp.Header.Get("TOKEN")
				close(ch)
			}
		}
		req.Header.Set("TOKEN", token)
		return c.Do(req)
	}
}
