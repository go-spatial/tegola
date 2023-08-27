package algnhsa

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"path"
	"strings"

	"github.com/aws/aws-lambda-go/events"
)

/*
AWS Documentation:

- https://docs.aws.amazon.com/apigateway/latest/developerguide/http-api-develop-integrations-lambda.html
- https://docs.aws.amazon.com/lambda/latest/dg/lambda-urls.html
*/

var (
	errAPIGatewayV2UnexpectedRequest = errors.New("expected APIGatewayV2HTTPRequest event")
)

func newAPIGatewayV2Request(ctx context.Context, payload []byte, opts *Options) (lambdaRequest, error) {
	var event events.APIGatewayV2HTTPRequest
	if err := json.Unmarshal(payload, &event); err != nil {
		return lambdaRequest{}, err
	}
	if event.Version != "2.0" {
		return lambdaRequest{}, errAPIGatewayV2UnexpectedRequest
	}

	req := lambdaRequest{
		HTTPMethod:      event.RequestContext.HTTP.Method,
		Path:            event.RawPath,
		RawQueryString:  event.RawQueryString,
		Headers:         event.Headers,
		Body:            event.Body,
		IsBase64Encoded: event.IsBase64Encoded,
		SourceIP:        event.RequestContext.HTTP.SourceIP,
		Context:         context.WithValue(ctx, RequestTypeAPIGatewayV2, event),
		requestType:     RequestTypeAPIGatewayV2,
	}

	// APIGatewayV2 doesn't support multi-value headers.
	// For cookies there is a workaround - Cookie headers are assigned to the event Cookies slice.
	// All other multi-value headers are joined into a single value with a comma.
	// It would be unsafe to split such values on a comma - it's impossible to distinguish a multi-value header
	// joined with a comma and a single-value header that contains a comma.
	if len(event.Cookies) > 0 {
		if req.MultiValueHeaders == nil {
			req.MultiValueHeaders = make(map[string][]string)
		}
		req.MultiValueHeaders["Cookie"] = event.Cookies
	}

	if opts.UseProxyPath {
		req.Path = path.Join("/", event.PathParameters["proxy"])
	}

	return req, nil
}

func newAPIGatewayV2Response(r *http.Response) (lambdaResponse, error) {
	resp := lambdaResponse{
		Headers: make(map[string]string, len(r.Header)),
	}
	// APIGatewayV2 doesn't support multi-value headers.
	for key, values := range r.Header {
		// For cookies there is a workaround - Set-Cookie headers are assigned to the response Cookies slice.
		if key == canonicalSetCookieHeaderKey {
			resp.Cookies = values
			continue
		}
		// All other multi-value headers are joined into a single value with a comma.
		resp.Headers[key] = strings.Join(values, ",")
	}
	return resp, nil
}

// APIGatewayV2RequestFromContext extracts the APIGatewayV2HTTPRequest event from ctx.
func APIGatewayV2RequestFromContext(ctx context.Context) (events.APIGatewayV2HTTPRequest, bool) {
	val := ctx.Value(RequestTypeAPIGatewayV2)
	if val == nil {
		return events.APIGatewayV2HTTPRequest{}, false
	}
	event, ok := val.(events.APIGatewayV2HTTPRequest)
	return event, ok
}
