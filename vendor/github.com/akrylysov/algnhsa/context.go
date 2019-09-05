package algnhsa

import (
	"context"

	"github.com/aws/aws-lambda-go/events"
)

type contextKey int

const (
	proxyRequestContextKey contextKey = iota
	albRequestContextKey
)

func newProxyRequestContext(ctx context.Context, event events.APIGatewayProxyRequest) context.Context {
	return context.WithValue(ctx, proxyRequestContextKey, event)
}

// ProxyRequestFromContext extracts the APIGatewayProxyRequest event from ctx.
func ProxyRequestFromContext(ctx context.Context) (events.APIGatewayProxyRequest, bool) {
	val := ctx.Value(proxyRequestContextKey)
	if val == nil {
		return events.APIGatewayProxyRequest{}, false
	}
	event, ok := val.(events.APIGatewayProxyRequest)
	return event, ok
}

func newTargetGroupRequestContext(ctx context.Context, event events.ALBTargetGroupRequest) context.Context {
	return context.WithValue(ctx, albRequestContextKey, event)
}

// TargetGroupRequestFromContext extracts the ALBTargetGroupRequest event from ctx.
func TargetGroupRequestFromContext(ctx context.Context) (events.ALBTargetGroupRequest, bool) {
	val := ctx.Value(albRequestContextKey)
	if val == nil {
		return events.ALBTargetGroupRequest{}, false
	}
	event, ok := val.(events.ALBTargetGroupRequest)
	return event, ok
}
