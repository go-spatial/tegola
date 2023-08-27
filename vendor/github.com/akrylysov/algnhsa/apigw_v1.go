package algnhsa

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"path"

	"github.com/aws/aws-lambda-go/events"
)

/*
AWS Documentation:

- https://docs.aws.amazon.com/apigateway/latest/developerguide/http-api-develop-integrations-lambda.html
*/

var (
	errAPIGatewayV1UnexpectedRequest = errors.New("expected APIGatewayProxyRequest event")
)

func newAPIGatewayV1Request(ctx context.Context, payload []byte, opts *Options) (lambdaRequest, error) {
	var event events.APIGatewayProxyRequest
	if err := json.Unmarshal(payload, &event); err != nil {
		return lambdaRequest{}, err
	}
	if event.RequestContext.AccountID == "" {
		return lambdaRequest{}, errAPIGatewayV1UnexpectedRequest
	}

	req := lambdaRequest{
		HTTPMethod:                      event.HTTPMethod,
		Path:                            event.Path,
		QueryStringParameters:           event.QueryStringParameters,
		MultiValueQueryStringParameters: event.MultiValueQueryStringParameters,
		Headers:                         event.Headers,
		MultiValueHeaders:               event.MultiValueHeaders,
		Body:                            event.Body,
		IsBase64Encoded:                 event.IsBase64Encoded,
		SourceIP:                        event.RequestContext.Identity.SourceIP,
		Context:                         context.WithValue(ctx, RequestTypeAPIGatewayV1, event),
		requestType:                     RequestTypeAPIGatewayV1,
	}

	if opts.UseProxyPath {
		req.Path = path.Join("/", event.PathParameters["proxy"])
	}

	return req, nil
}

func newAPIGatewayV1Response(r *http.Response) (lambdaResponse, error) {
	resp := lambdaResponse{
		MultiValueHeaders: r.Header,
	}
	return resp, nil
}

// APIGatewayV1RequestFromContext extracts the APIGatewayProxyRequest event from ctx.
func APIGatewayV1RequestFromContext(ctx context.Context) (events.APIGatewayProxyRequest, bool) {
	val := ctx.Value(RequestTypeAPIGatewayV1)
	if val == nil {
		return events.APIGatewayProxyRequest{}, false
	}
	event, ok := val.(events.APIGatewayProxyRequest)
	return event, ok
}
