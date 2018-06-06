package algnhsa

import (
	"context"

	"github.com/aws/aws-lambda-go/events"
)

type key int

const requestContextKey key = 0

func newContext(ctx context.Context, event events.APIGatewayProxyRequest) context.Context {
	return context.WithValue(ctx, requestContextKey, event)
}

// ProxyRequestFromContext extracts the APIGatewayProxyRequest event from ctx.
func ProxyRequestFromContext(ctx context.Context) (events.APIGatewayProxyRequest, bool) {
	event, ok := ctx.Value(requestContextKey).(events.APIGatewayProxyRequest)
	return event, ok
}
