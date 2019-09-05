package algnhsa

import (
	"encoding/base64"
	"net/http/httptest"
)

const acceptAllContentType = "*/*"

type lambdaResponse struct {
	StatusCode        int                 `json:"statusCode"`
	Headers           map[string]string   `json:"headers"`
	MultiValueHeaders map[string][]string `json:"multiValueHeaders"`
	Body              string              `json:"body"`
	IsBase64Encoded   bool                `json:"isBase64Encoded,omitempty"`
}

func newLambdaResponse(w *httptest.ResponseRecorder, binaryContentTypes map[string]bool) (lambdaResponse, error) {
	event := lambdaResponse{}

	// Set status code.
	event.StatusCode = w.Code

	// Set headers.
	event.MultiValueHeaders = w.Result().Header

	// Set body.
	contentType := w.Header().Get("Content-Type")
	if binaryContentTypes[acceptAllContentType] || binaryContentTypes[contentType] {
		event.Body = base64.StdEncoding.EncodeToString(w.Body.Bytes())
		event.IsBase64Encoded = true
	} else {
		event.Body = w.Body.String()
	}

	return event, nil
}
