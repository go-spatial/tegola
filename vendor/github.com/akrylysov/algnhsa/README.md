# algnhsa [![GoDoc](https://godoc.org/github.com/akrylysov/algnhsa?status.svg)](https://godoc.org/github.com/akrylysov/algnhsa) ![Build Status](https://github.com/akrylysov/algnhsa/actions/workflows/test.yaml/badge.svg)

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

## Plug in a third-party web framework

### Gin

```go
package main

import (
	"net/http"

	"github.com/akrylysov/algnhsa"
	"github.com/gin-gonic/gin"
)

func main() {
	r := gin.Default()
	r.GET("/", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"message": "hi",
		})
	})
	algnhsa.ListenAndServe(r, nil)
}
```

### echo

```go
package main

import (
	"net/http"

	"github.com/akrylysov/algnhsa"
	"github.com/labstack/echo/v4"
)

func main() {
	e := echo.New()
	e.GET("/", func(c echo.Context) error {
		return c.String(http.StatusOK, "hi")
	})
	algnhsa.ListenAndServe(e, nil)
}
```

### chi

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

### Fiber

```go
package main

import (
	"github.com/akrylysov/algnhsa"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/adaptor"
)

func main() {
	app := fiber.New()
	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendString("Hello, World!")
	})
	algnhsa.ListenAndServe(adaptor.FiberApp(app), nil)
}
```

## Deployment

First, build your Go application for Linux and zip it:

```bash
GOOS=linux GOARCH=amd64 go build -tags lambda.norpc -o bootstrap
zip function.zip bootstrap
```

When creating a new function, choose the "Provide your own bootstrap on Amazon Linux 2" runtime or "Custom runtime on Amazon Linux 2" when modifying an existing function. Make sure to use `bootstrap` as the executable name and as the handler name in AWS.

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
