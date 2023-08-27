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

func addHandler(w http.ResponseWriter, r *http.Request) {
    f, _ := strconv.Atoi(r.FormValue("first"))
    s, _ := strconv.Atoi(r.FormValue("second"))
    w.Header().Set("X-Hi", "foo")
    fmt.Fprintf(w, "%d", f+s)
}

func contextHandler(w http.ResponseWriter, r *http.Request) {
    lambdaEvent, ok := algnhsa.APIGatewayV2RequestFromContext(r.Context())
    if ok {
        fmt.Fprint(w, lambdaEvent.RequestContext.AccountID)
    }
}

func main() {
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
        w.Write([]byte("hi"))
    })
    algnhsa.ListenAndServe(r, nil)
}
```

## Deployment

First, build your Go application for Linux and zip it:

```bash
GOOS=linux GOARCH=amd64 go build -o handler
zip handler.zip handler
```

AWS provides plenty of ways to expose a Lambda function to the internet.

### Lambda Function URL

This is the easier way to deploy your Lambda function as an HTTP endpoint.
It only requires going to the "Function URL" section of the Lambda function configuration and clicking "Configure Function URL".

### API Gateway

#### HTTP API

1. Create a new HTTP API.

2. Configure a catch-all `$default` route.

#### REST API

1. Create a new REST API.

2. In the "Resources" section create a new `ANY` method to handle requests to `/` (check "Use Lambda Proxy Integration").

3. Add a catch-all `{proxy+}` resource to handle requests to every other path (check "Configure as proxy resource").

### ALB

1. Create a new ALB and point it to your Lambda function.

2. In the target group settings in the "Attributes" section enable "Multi value headers".

[AWS Documentation](https://docs.aws.amazon.com/lambda/latest/dg/services-alb.html)
