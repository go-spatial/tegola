# algnhsa [![GoDoc](https://godoc.org/github.com/akrylysov/algnhsa?status.svg)](https://godoc.org/github.com/akrylysov/algnhsa) [![Build Status](https://travis-ci.org/akrylysov/algnhsa.svg?branch=master)](https://travis-ci.org/akrylysov/algnhsa)

algnhsa is an AWS Lambda Go `net/http` server adapter.

algnhsa enables running Go web applications on AWS Lambda/API Gateway without changing the existing HTTP handlers:

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

func main() {
    http.HandleFunc("/", indexHandler)
    http.HandleFunc("/add", addHandler)
    algnhsa.ListenAndServe(http.DefaultServeMux, nil)
}
```

Plug in a third-party HTTP router:

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

More details at http://artem.krylysov.com/blog/2018/01/18/porting-go-web-applications-to-aws-lambda/.
