package algnhsa

import (
	"context"
	"encoding/base64"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
)

type lambdaRequest struct {
	HTTPMethod                      string              `json:"httpMethod"`
	Path                            string              `json:"path"`
	QueryStringParameters           map[string]string   `json:"queryStringParameters,omitempty"`
	MultiValueQueryStringParameters map[string][]string `json:"multiValueQueryStringParameters,omitempty"`
	Headers                         map[string]string   `json:"headers,omitempty"`
	MultiValueHeaders               map[string][]string `json:"multiValueHeaders,omitempty"`
	IsBase64Encoded                 bool                `json:"isBase64Encoded"`
	Body                            string              `json:"body"`
	SourceIP                        string
	Context                         context.Context
}

func newLambdaRequest(ctx context.Context, payload []byte, opts *Options) (lambdaRequest, error) {
	switch opts.RequestType {
	case RequestTypeAPIGateway:
		return newAPIGatewayRequest(ctx, payload, opts)
	case RequestTypeALB:
		return newALBRequest(ctx, payload, opts)
	}

	// The request type wasn't specified.
	// Try to decode the payload as APIGatewayProxyRequest, if it fails try ALBTargetGroupRequest.
	req, err := newAPIGatewayRequest(ctx, payload, opts)
	if err != nil && err != errAPIGatewayUnexpectedRequest {
		return lambdaRequest{}, err
	}
	if err == nil {
		return req, nil
	}

	req, err = newALBRequest(ctx, payload, opts)
	if err != nil && err != errALBUnexpectedRequest {
		return lambdaRequest{}, err
	}
	if err == nil {
		return req, nil
	}

	return lambdaRequest{}, errors.New("neither APIGatewayProxyRequest nor ALBTargetGroupRequest received")
}

func newHTTPRequest(event lambdaRequest) (*http.Request, error) {
	// Build request URL.
	params := url.Values{}
	for k, v := range event.QueryStringParameters {
		params.Set(k, v)
	}
	for k, vals := range event.MultiValueQueryStringParameters {
		params[k] = vals
	}

	// Set headers.
	// https://docs.aws.amazon.com/apigateway/latest/developerguide/set-up-lambda-proxy-integrations.html
	// If you specify values for both headers and multiValueHeaders, API Gateway merges them into a single list.
	// If the same key-value pair is specified in both, only the values from multiValueHeaders will appear
	// the merged list.
	headers := make(http.Header)
	for k, v := range event.Headers {
		headers.Set(k, v)
	}
	for k, vals := range event.MultiValueHeaders {
		headers[http.CanonicalHeaderKey(k)] = vals
	}

	u := url.URL{
		Host:     headers.Get("host"),
		RawPath:  event.Path,
		RawQuery: params.Encode(),
	}

	// Unescape request path
	p, err := url.PathUnescape(u.RawPath)
	if err != nil {
		return nil, err
	}
	u.Path = p

	if u.Path == u.RawPath {
		u.RawPath = ""
	}

	// Handle base64 encoded body.
	var body io.Reader = strings.NewReader(event.Body)
	if event.IsBase64Encoded {
		body = base64.NewDecoder(base64.StdEncoding, body)
	}

	// Create a new request.
	r, err := http.NewRequest(event.HTTPMethod, u.String(), body)
	if err != nil {
		return nil, err
	}

	// Set remote IP address.
	r.RemoteAddr = event.SourceIP

	// Set request URI
	r.RequestURI = u.RequestURI()

	r.Header = headers

	return r.WithContext(event.Context), nil
}
