package algnhsa

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"

	"github.com/aws/aws-lambda-go/events"
)

/*
AWS Documentation:

- https://docs.aws.amazon.com/lambda/latest/dg/services-alb.html
*/

var (
	errALBUnexpectedRequest         = errors.New("expected ALBTargetGroupRequest event")
	errALBExpectedMultiValueHeaders = errors.New("expected multi value headers; enable Multi value headers in target group settings")
)

func getALBSourceIP(event events.ALBTargetGroupRequest) string {
	if xff, ok := event.MultiValueHeaders["x-forwarded-for"]; ok && len(xff) > 0 {
		ips := strings.SplitN(xff[0], ",", 2)
		if len(ips) > 0 {
			return ips[0]
		}
	}
	return ""
}

func newALBRequest(ctx context.Context, payload []byte, opts *Options) (lambdaRequest, error) {
	var event events.ALBTargetGroupRequest
	if err := json.Unmarshal(payload, &event); err != nil {
		return lambdaRequest{}, err
	}
	if event.RequestContext.ELB.TargetGroupArn == "" {
		return lambdaRequest{}, errALBUnexpectedRequest
	}
	if len(event.MultiValueHeaders) == 0 {
		return lambdaRequest{}, errALBExpectedMultiValueHeaders
	}

	for _, vals := range event.MultiValueQueryStringParameters {
		for i, v := range vals {
			unescaped, err := url.QueryUnescape(v)
			if err != nil {
				return lambdaRequest{}, err
			}
			vals[i] = unescaped
		}
	}

	req := lambdaRequest{
		HTTPMethod:                      event.HTTPMethod,
		Path:                            event.Path,
		MultiValueQueryStringParameters: event.MultiValueQueryStringParameters,
		MultiValueHeaders:               event.MultiValueHeaders,
		Body:                            event.Body,
		IsBase64Encoded:                 event.IsBase64Encoded,
		SourceIP:                        getALBSourceIP(event),
		Context:                         context.WithValue(ctx, RequestTypeALB, event),
		requestType:                     RequestTypeALB,
	}

	return req, nil
}

func newALBResponse(r *http.Response) (lambdaResponse, error) {
	resp := lambdaResponse{
		MultiValueHeaders: r.Header,
	}
	return resp, nil
}

// ALBRequestFromContext extracts the ALBTargetGroupRequest event from ctx.
func ALBRequestFromContext(ctx context.Context) (events.ALBTargetGroupRequest, bool) {
	val := ctx.Value(RequestTypeALB)
	if val == nil {
		return events.ALBTargetGroupRequest{}, false
	}
	event, ok := val.(events.ALBTargetGroupRequest)
	return event, ok
}
