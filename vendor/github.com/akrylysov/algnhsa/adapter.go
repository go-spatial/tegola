package algnhsa

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"

	"github.com/aws/aws-lambda-go/lambda"
)

// New returns a new lambda handler for the given http.Handler.
// It is up to the caller of New to run lamdba.Start(handler) with the returned handler.
func New(handler http.Handler, opts *Options) lambda.Handler {
	if handler == nil {
		handler = http.DefaultServeMux
	}
	if opts == nil {
		opts = defaultOptions
	}
	opts.setBinaryContentTypeMap()
	return lambdaHandler{httpHandler: handler, opts: opts}
}

var defaultOptions = &Options{}

type lambdaHandler struct {
	httpHandler http.Handler
	opts        *Options
}

func (handler lambdaHandler) Invoke(ctx context.Context, payload []byte) ([]byte, error) {
	resp, err := handler.handleEvent(ctx, payload)
	if err != nil {
		return nil, err
	}
	if handler.opts.DebugLog {
		fmt.Printf("Response: %+v", resp)
	}
	return json.Marshal(resp)
}

func (handler lambdaHandler) handleEvent(ctx context.Context, payload []byte) (lambdaResponse, error) {
	if handler.opts.DebugLog {
		fmt.Printf("Request: %s", payload)
	}
	eventReq, err := newLambdaRequest(ctx, payload, handler.opts)
	if err != nil {
		return lambdaResponse{}, err
	}
	r, err := newHTTPRequest(eventReq)
	if err != nil {
		return lambdaResponse{}, err
	}
	w := httptest.NewRecorder()
	handler.httpHandler.ServeHTTP(w, r)
	return newLambdaResponse(w, handler.opts.binaryContentTypeMap, eventReq.requestType)
}

// ListenAndServe starts the AWS Lambda runtime (aws-lambda-go lambda.Start) with a given handler.
func ListenAndServe(handler http.Handler, opts *Options) {
	lambdaHandler := New(handler, opts)
	lambda.StartHandler(lambdaHandler)
}
