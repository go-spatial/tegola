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

var errUnsupportedPayloadFormat = errors.New("unsupported payload format; supported formats: APIGatewayV2HTTPRequest, APIGatewayProxyRequest, ALBTargetGroupRequest")

type lambdaRequest struct {
	HTTPMethod                      string
	Path                            string
	QueryStringParameters           map[string]string
	MultiValueQueryStringParameters map[string][]string
	RawQueryString                  string
	Headers                         map[string]string
	MultiValueHeaders               map[string][]string
	IsBase64Encoded                 bool
	Body                            string
	SourceIP                        string
	Context                         context.Context
	requestType                     RequestType
}

func newLambdaRequest(ctx context.Context, payload []byte, opts *Options) (lambdaRequest, error) {
	switch opts.RequestType {
	case RequestTypeAPIGatewayV1:
		return newAPIGatewayV1Request(ctx, payload, opts)
	case RequestTypeAPIGatewayV2:
		return newAPIGatewayV2Request(ctx, payload, opts)
	case RequestTypeALB:
		return newALBRequest(ctx, payload, opts)
	}

	// The request type wasn't specified.
	// Try to decode the payload as APIGatewayV2HTTPRequest, fall back to APIGatewayProxyRequest, then ALBTargetGroupRequest.
	req, err := newAPIGatewayV2Request(ctx, payload, opts)
	if err != nil && err != errAPIGatewayV2UnexpectedRequest {
		return lambdaRequest{}, err
	}
	if err == nil {
		return req, nil
	}

	req, err = newAPIGatewayV1Request(ctx, payload, opts)
	if err != nil && err != errAPIGatewayV1UnexpectedRequest {
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

	return lambdaRequest{}, errUnsupportedPayloadFormat
}

func newHTTPRequest(event lambdaRequest) (*http.Request, error) {
	// Build request URL.
	rawQuery := event.RawQueryString
	if len(rawQuery) == 0 {
		params := url.Values{}
		for k, v := range event.QueryStringParameters {
			params.Set(k, v)
		}
		for k, vals := range event.MultiValueQueryStringParameters {
			params[k] = vals
		}
		rawQuery = params.Encode()
	}

	// https://docs.aws.amazon.com/apigateway/latest/developerguide/set-up-lambda-proxy-integrations.html
	// If you specify values for both headers and multiValueHeaders, API Gateway V1 merges them into a single list.
	// If the same key-value pair is specified in both, only the values from multiValueHeaders will appear
	// in the merged list.
	headers := make(http.Header)
	for k, v := range event.Headers {
		headers.Set(k, v)
	}
	for k, vals := range event.MultiValueHeaders {
		headers[http.CanonicalHeaderKey(k)] = vals
	}

	unescapedPath, err := url.PathUnescape(event.Path)
	if err != nil {
		return nil, err
	}
	u := url.URL{
		Host:     headers.Get("Host"),
		Path:     unescapedPath,
		RawQuery: rawQuery,
	}

	// Handle base64 encoded body.
	var body io.Reader = strings.NewReader(event.Body)
	if event.IsBase64Encoded {
		body = base64.NewDecoder(base64.StdEncoding, body)
	}

	// Create a new request.
	r, err := http.NewRequestWithContext(event.Context, event.HTTPMethod, u.String(), body)
	if err != nil {
		return nil, err
	}

	// Set remote IP address.
	r.RemoteAddr = event.SourceIP

	// Set request URI
	r.RequestURI = u.RequestURI()

	r.Header = headers

	return r, nil
}
