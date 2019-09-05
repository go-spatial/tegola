# algnhsa [![GoDoc](https://godoc.org/github.com/akrylysov/algnhsa?status.svg)](https://godoc.org/github.com/akrylysov/algnhsa) [![Build Status](https://travis-ci.org/akrylysov/algnhsa.svg?branch=master)](https://travis-ci.org/akrylysov/algnhsa)

algnhsa is an AWS Lambda Go `net/http` server adapter.

algnhsa enables running Go web applications on AWS Lambda and API Gateway or ALB without changing the existing HTTP handlers:

```go
package main

import (
    "fmt"
    "net/http"
    "strconv"

    "github.com/akrylysov/algnhsa"
)

func indexHandler(w http.ResponseWriter, r *http.Request) {
    w.Write([]byte("index"))
}

func addHandler(w http.ResponseWriter, r *http.Request) {
    f, _ := strconv.Atoi(r.FormValue("first"))
    s, _ := strconv.Atoi(r.FormValue("second"))
    w.Header().Set("X-Hi", "foo")
    fmt.Fprintf(w, "%d", f+s)
}

func contextHandler(w http.ResponseWriter, r *http.Request) {
    proxyReq, ok := algnhsa.ProxyRequestFromContext(r.Context())
    if ok {
        fmt.Fprint(w, proxyReq.RequestContext.AccountID)
    }
}

func main() {
    http.HandleFunc("/", indexHandler)
    http.HandleFunc("/add", addHandler)
    http.HandleFunc("/context", contextHandler)
    algnhsa.ListenAndServe(http.DefaultServeMux, nil)
}
```

## Plug in a third-party HTTP router

```go
package main

import (
    "net/http"

    "github.com/akrylysov/algnhsa"
    "github.com/go-chi/chi"
)

func main() {
    r := chi.NewRouter()
    r.Get("/", func(w http.ResponseWriter, r *http.Request) {
        w.Write([]byte("index"))
    })
    algnhsa.ListenAndServe(r, nil)
}
```

## Setting up API Gateway 

1. Create a new REST API.

2. In the "Resources" section create a new `ANY` method to handle requests to `/` (check "Use Lambda Proxy Integration").

    ![API Gateway index](https://akrylysov.github.io/algnhsa/apigateway-index.png)

3. Add a catch-all `{proxy+}` resource to handle requests to every other path (check "Configure as proxy resource").

    ![API Gateway catch-all](https://akrylysov.github.io/algnhsa/apigateway-catchall.png)

## Setting up ALB

1. Create a new ALB and point it to your Lambda function.

2. In the target group settings enable "Multi value headers".
