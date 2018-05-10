package algnhsa

import (
	"context"
	"encoding/base64"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/aws/aws-lambda-go/events"
)

func newHTTPRequest(ctx context.Context, event events.APIGatewayProxyRequest) (*http.Request, error) {
	// Build request URL.
	params := url.Values{}
	for k, v := range event.QueryStringParameters {
		params.Set(k, v)
	}
	u := url.URL{
		Path:     event.Path,
		RawQuery: params.Encode(),
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

	// Set headers.
	for k, v := range event.Headers {
		r.Header.Set(k, v)
	}

	// Set remote IP address.
	r.RemoteAddr = event.RequestContext.Identity.SourceIP

	return r.WithContext(newContext(ctx, event)), nil
}
