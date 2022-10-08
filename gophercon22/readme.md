# How I Write HTTP Clients After (almost) 7 Years

## Running the example

    cd gophercon22
    go run . server
    # in a separate terminal
    go run . client

## Original Papercall Paper

### Abstract

**HTTP Servers have middleware.**  
**HTTP Clients have ... nothing?**

Inspired by the well known article and previous Gophercon talk "How I Write HTTP Services After 7/8 Years," this talk aims to provide a rival paradigm for HTTP clients. This talk introduces and describes using client decorators for building better HTTP clients with easier testing and greater maintainability.

### Target Audience

This talk is suitable for a general audience but will be most valuable for beginner to intermediate gophers who work with HTTP APIs and clients.

### Why now?

The best time to give this talk was 3 years ago, the second best time is now. This talk builds out a direction of client standards around HTTP clients to help new and seasoned Go developers better build out their API standards and create more robust code going forward.

### Introduction (2 minutes)

This section will cover a brief overview of myself and my background as well as reference the inspirational talk by Mat Ryer in 2019. This talk is heavily code-oriented and will feature a walkthrough of a basic HTTP server and client, and their respective tests.

### Server Middleware  with http.HandlerFunc (5 minutes)

It's standard to use middleware when building out web services. These are chained functions which can be used to segment different pieces of logic in the code. For instance, you may have an endpoint that saves data to a database:

    package app
    
    // Server contains a database interface and all of the API handlers needed
    type Server struct {
        database interface {
            Save(interface{}) error
        }
    }
    
    // Thing is a custom structure that we want our server to act on
    type Thing struct {
        Foo string
        Bar string
    }

    // HandleSaveThing saves an object from the request body to the database
    func (s *Server) HandleSaveThing() http.HandlerFunc {
        return func(w http.ResponseWriter, r *http.Request) {
            if r.Body == nil {
                w.WriteHeader(http.StatusBadRequest)
                return
            }
            var thing Thing
            if err := json.NewDecoder(r.Body).Decode(&thing); err != nil {
                w.WriteHeader(http.StatusBadRequest)
                return
            }
            if err := database.Save(thing); err != nil {
                w.WriteHeader(http.StatusInternalServerError)
                return
            }
        }
    }

It may be necessary for this code to only ever be executed after some filtering; for instance, only if the request contains a custom header. Rather than addding this logic to the `HandleSaveThing` function, we can use a middleware method.

    // MiddlewareCustomHeader validates the incoming request for a valid header
    func (s *Server) MiddlewareCustomHeader(next http.HandlerFunc) http.HandlerFunc {
        return func(w http.ResponseWriter, r *http.Request) {
            v := r.Header.Get("X-MY-CUSTOM-HEADER")
            if v == "" {
                w.WriteHeader(http.StatusBadRequest)
                return
            }
            next.Do(w, r)
        }
    }

When we construct our endpoint router we can add the middleware:

    func (s *Server) Root() http.Handler {
        sm := http.NewServeMux()
        sm.HandleFunc("/save", s.MiddlewareCustomHeader(s.HandleSaveThing()))
    
        return sm
    }

Our main.go file might then look like:

    package main
    
    func main(){
        db, err := NewDatabase()
        if err != nil {
            log.Fatal(err)
        }
        s, err := NewServer()
        if err != nil {
            log.Fatal(err)
        }
        log.Fatal(http.ListenAndServe(":8080", s.Root()))
    }

### DoerFunc for Clients (8 minutes)

Middleware is great for servers, but what about clients? The standard go HTTP client implements:

    type HTTPClient interface{
        Do(*http.Request) (*http.Response, error)
    }

We'll want to be able to abstract this in the same way as the HTTP handlers abstract with `HandlerFunc`. Since the `http` package does not have this as a type we can make our own.

    // DoerFunc is to an HTTP client as HandlerFunc is to an HTTP server
    type DoerFunc func(*http.Request) (*http.Response, error)
    
    // Do calls the parent function, the same way ServeHTTP calls the parent in a HandlerFunc type
    func (f DoerFunc) Do(req *http.Request) (*http.Response, error) {
        return f(req)
    }

We can create a request function that mirrors our handler:

    // HandleSaveThing saves an object from the request body to the database
    func (s *Server) HandleSaveThing() http.HandlerFunc {
        return func(w http.ResponseWriter, r *http.Request) {
            if r.Body == nil {
                w.WriteHeader(http.StatusBadRequest)
                return
            }
            var thing Thing
            if err := json.NewDecoder(r.Body).Decode(&thing); err != nil {
                w.WriteHeader(http.StatusBadRequest)
                return
            }
            if err := database.Save(thing); err != nil {
                w.WriteHeader(http.StatusInternalServerError)
                return
            }
        }
    }
    
    // RequestSaveThing makes a request to save Thing to the HTTP endpoint
    func RequestSaveThing(client HTTPClient, addr string, thing Thing) error {
        b, err := json.Marshal(thing)
        if err != nil {
            return err
        }
        req, err := http.NewRequest(http.MethodPost, addr, bytes.NewBuffer(b))
        if err != nil {
            return err
        }
    
        resp, err := client.Do(req)
        if err != nil {
            return err
        }
        if resp.Body != nil {
            defer resp.Body.Close()
        }
        if resp.StatusCode != http.StatusOK {
            return fmt.Errorf("invalid status code received %d", resp.StatusCode)
        }
        return nil
    }

In the simplest scenario we can make our request with the default HTTP client:

    thing := app.Thing{
        Foo: "foo",
        Bar: "bar",
    }
    
    client := http.DefaultClient
    if err := app.RequestSaveThing(client, addr, thing); err != nil {
        return err
    }

This simple version is ideal for directly testing the logic in the handler.

    func TestSaveThing(t *testing.T){
        db := &mockDB{}
        s := Server{
            database: db,
        }
        handler := s.HandleSaveThing()
        svr := httptest.NewServer(handler)
        defer svr.Close()
    
        // here we'll use the httptest client instead of the http.DefaultClient
        client := svr.Client()
        err := RequestSaveThing(client, svr.URL, Thing{Foo: "foo", Bar: "bar"})
        if err != nil {
            t.Fatal(err)
        }
    }

### Adding Decorators (5 minutes)

Decorators are the client equivalent of a middleware function. We can use decorators to change the behavior of a client without directly affecting the business logic. Take our scenario from before. If we have a server with the `MiddlewareCustomHeader` middleware, then our client needs to contain the correct header in its request.

    // DecorateCustomHeader adds a header to the outgoing request
    func DecorateCustomHeader(client HTTPClient, value string) DoerFunc {
        return func(req *http.Request) (*http.Response, error) {
            req.Header.Set("X-MY-CUSTOM-HEADER", value)
            return client.Do(req)
        }
    }

In our tests, we can test this independently or with our other tests.

    func TestDecorateHeader(t *testing.T) {
        s := Server{}
        // create a blank handler that always returns 200
        handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
        // add middleware that returns an error if no header is present
        handler = s.MiddleWareCustomHeader(handler)
    
        // new test server & blank request
        svr := httptest.NewServer(handler)
        defer svr.Close()
        req, _ := http.NewRequest(http.MethodGet, svr.URL, nil)
    
        client := svr.Client()
        // decorate with the header
        client = DecorateCustomHeader(client, "my header")
        resp, err := client.Do(req)
        if err != nil {
            t.Fatal(err)
        }
        if resp.StatusCode != http.StatusOK {
            t.Fatal(resp.StatusCode)
        }
    }

    func TestSaveThing(t *testing.T){
        db := &mockDB{}
        s := Server{
            database: db,
        }
        handler := s.HandleSaveThing()
        // add our middleware to require the header
        handler = s.MiddlewareCustomHeader(handler)
        svr := httptest.NewServer(handler)
        defer svr.Close()
    
        // here we'll use the httptest client instead of the http.DefaultClient
        client := svr.Client()
        // add our decorator to provide the header
        client = DecorateCustomHeader(client, "my value")
        err := RequestSaveThing(client, svr.URL, Thing{Foo: "foo", Bar: "bar"})
        if err != nil {
            t.Fatal(err)
        }
    }

### Decorators: Format the Request or Check the Response (4 minutes)

Decorators can be used on either side of the request action, either decorating the request with data or checking the response/error. For instance, a simple decorator for checking the status of a response could be:

    func DecorateRequireStatus(client HTTPClient, status int) DoerFunc {
        return func(req *http.Request) (*http.Response, error) {
            resp, err := client.Do(req)
            if err != nil {
                return resp, err
            }
            if resp.StatusCode != status {
                return resp, fmt.Errorf("expected status %d got %d", status, resp.StatusCode)
            }
            return resp, nil
        }
    }

This is especially useful if the server has a common error formatting. For instance, we can check if the server included an error in the response or has a nil body.

    func DecorateCheckErrorsHeader(client HTTPClient) DoerFunc {
        return func(req *http.Request) (*http.Response, error) {
            resp, err := client.Do(req)
            if err != nil {
                return resp, err
            }
            if e := resp.Header.Get("Errors"); e != "" {
                return resp, fmt.Errorf(e)
            }
            return resp, nil
        }
    }
    
    func DecorateCheckNilBody(client HTTPClient) DoerFunc {
        return func(req *http.Request) (*http.Response, error) {
            resp, err := client.Do(req)
            if err != nil {
                return resp, err
            }
            if resp.Body == nil {
                return resp, fmt.Errorf("received nil response body")
            }
            return resp, nil
        }
    }

These can be used across multiple client calls to construct client configurations easily and keep the actual implementation logic small.

    client := http.DefaultClient
    client = DecorateRequireStatus(client, http.StatusOK)
    client = DecorateCheckNilBody(client)
    client = DecorateCheckErrorsHeader(client)
    err := RequestSaveToken(client, url, token)

### Request Rate Limiting (4 minutes)

Decorators can help with client-side request rate limiting. We can initialize a channel that limits the number of requests in flight at any given time:

    func DecorateRateLimit(client HTTPClient, reqPerMin int) DoerFunc {
        ticker := time.NewTicker(time.Second * 60)
        ch := make(chan struct{}, reqPerMin)
        go func() {
            // every minute, fill the channel back up
            for range ticker.C {
                for i := 0; i < reqPerMin; i++ {
                    select {
                    case ch <- struct{}{}:
                    default:
                        break
                    }
                }
            }
        }()
        return func(req *http.Request) (*http.Response, error) {
            select {
            case <-ch:
            case <-req.Context().Done():
                return nil, req.Context().Err()
            }
            return client.Do(req)
        }
    }

### Auth Calls as Part of Initialization (5 min)

We can set up our client as part of our middleware. For instance, let's say we need to add a header token to our request. This token is retrieved using another endpoint and is required for future calls.

    func getToken(authURL string, user string, pass string) ([]byte, error) {
        r, err := http.NewRequest(http.MethodGet, authURL, nil)
        if err != nil {
            return nil, err
        }
        r.SetBasicAuth(user, pass)
        resp, err := http.DefaultClient.Do(r)
        if err != nil {
            return nil, err
        }
        return io.ReadAll(resp.Body)
    }
    
    func DecorateToken(client HTTPClient, authURL string, user string, pass string) DoerFunc {
        token, err := getToken(authURL, user, pass)
        if err != nil {
            log.Fatal(err)
        }
        return func(req *http.Request) (*http.Response, error) {
            req.Header.Set("Authorization", "Bearer "+string(token))
            return client.Do(req)
        }
    }

### Handling Initialization Errors (5 min)

The previous example is adequate, but will cause a fatal log instead of gracefully handling the auth error. We can improve this. Let's start with using `sync.Once`:

    func DecorateToken(client HTTPClient, authURL string, user string, pass string) DoerFunc {
        var token []byte
        var init sync.Once
        var err error
      return func(req *http.Request) (*http.Response, error) {
            init.Do(func(){
                token, err = getToken(authURL, user, pass)
            })
            if err != nil {
                return nil, err
            }
    
            req.Header.Set("Authorization", "Bearer "+string(token))
            return client.Do(req)
        }
    }

That's not bad, but it also means we only ever attempt to get a token once. If there's ever an issue with the server, we won't be able to recover without starting all over. Instead, let's use a channel to block callers. When the channel is full, it will block and attempt to get a token. Once it has a token, it will close the channel, unblocking all other goroutines.

    func DecorateToken(client HTTPClient, authURL string, user string, pass string) DoerFunc {
        var token []byte
        var init = make(chan struct{}, 1)
        init <- struct{}{}
        return func(req *http.Request) (*http.Response, error) {
            // channel ensures we only do the request one at a time, but can retry on subsequent calls
            if _, ok := <-init; ok && len(token) == 0 {
                var err error
                token, err = getToken(authURL, user, pass)
                if err != nil {
                    init <- struct{}{}
                    return nil, err
                }
                close(init)
            }
    
            req.Header.Set("Authorization", "Bearer "+string(token))
            return client.Do(req)
        }
    }

### Conclusion (2 minutes)

All together, these can be used to make streamlined clients with data decoration, authorization and custom error handling baked in from the start.

    type MyClient struct {
        c HTTPClient
    }
    
    func NewClient() *MyClient {
        client := http.DefaultClient
        client = DecorateCheckNilBody(client)
        client = DecorateCheckErrorsHeader(client)
        client = DecorateRateLimit(client, 2000)
        client = DecorateRequireStatus(client, http.StatusOK)
        return &MyClient{
            c: client,
        }
    }
