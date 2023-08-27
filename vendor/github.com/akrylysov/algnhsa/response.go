package algnhsa

import (
	"encoding/base64"
	"net/http"
	"net/http/httptest"
)

const acceptAllContentType = "*/*"

var canonicalSetCookieHeaderKey = http.CanonicalHeaderKey("Set-Cookie")

// lambdaResponse is a combined lambda response.
// It contains common fields from APIGatewayProxyResponse, APIGatewayV2HTTPResponse and ALBTargetGroupResponse.
type lambdaResponse struct {
	StatusCode        int                 `json:"statusCode"`
	Headers           map[string]string   `json:"headers,omitempty"`
	MultiValueHeaders map[string][]string `json:"multiValueHeaders,omitempty"`
	Cookies           []string            `json:"cookies,omitempty"`
	Body              string              `json:"body"`
	IsBase64Encoded   bool                `json:"isBase64Encoded,omitempty"`
}

func newLambdaResponse(w *httptest.ResponseRecorder, binaryContentTypes map[string]bool, requestType RequestType) (lambdaResponse, error) {
	result := w.Result()

	var resp lambdaResponse
	var err error
	switch requestType {
	case RequestTypeAPIGatewayV1:
		resp, err = newAPIGatewayV1Response(result)
	case RequestTypeALB:
		resp, err = newALBResponse(result)
	case RequestTypeAPIGatewayV2:
		resp, err = newAPIGatewayV2Response(result)
	}
	if err != nil {
		return resp, err
	}

	resp.StatusCode = result.StatusCode

	// Set body.
	contentType := result.Header.Get("Content-Type")
	if binaryContentTypes[acceptAllContentType] || binaryContentTypes[contentType] {
		resp.Body = base64.StdEncoding.EncodeToString(w.Body.Bytes())
		resp.IsBase64Encoded = true
	} else {
		resp.Body = w.Body.String()
	}

	return resp, nil
}
