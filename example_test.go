package httpx_test

import (
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/tflyons/httpx"
)

func NewClient() httpx.Client {
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
	return c
}

func Example() {
	c := NewClient()
	thing, err := GetThing(c)
	if err != nil {
		panic(err)
	}
	fmt.Println(thing)
}

type Thing struct {
	Foo string `json:"foo"`
	Bar int    `json:"bar"`
}

func GetThing(baseClient httpx.Client) (Thing, error) {
	c := httpx.SetHeader(baseClient, "ThingSpecificHeader", "abcd")
	c = httpx.RequireResponseStatus(c, http.StatusOK)
	var thing Thing
	c = httpx.SetResponseBodyHandlerJSON(c, &thing)
	c = httpx.SetRequest(c, http.MethodGet, "http://example.com/things")
	_, err := c.Do(nil)
	return thing, err
}
