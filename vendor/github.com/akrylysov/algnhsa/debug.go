package algnhsa

import (
	"encoding/json"
	"github.com/aws/aws-lambda-go/events"
	"io"
	"mime"
	"net/http"
)

const maxDumpFormParseMem = 32 << 20 // 32MB

// RequestDebugDump is a dump of the HTTP request including the original Lambda event.
type RequestDebugDump struct {
	Method string
	URL    struct {
		Path    string
		RawPath string
	}
	RequestURI          string
	Host                string
	RemoteAddr          string
	Header              map[string][]string
	Form                map[string][]string
	Body                string
	APIGatewayV1Request *events.APIGatewayProxyRequest  `json:",omitempty"`
	APIGatewayV2Request *events.APIGatewayV2HTTPRequest `json:",omitempty"`
	ALBRequest          *events.ALBTargetGroupRequest   `json:",omitempty"`
}

func parseMediaType(r *http.Request) (string, error) {
	ct := r.Header.Get("Content-Type")
	if ct == "" {
		return "", nil
	}
	mt, _, err := mime.ParseMediaType(ct)
	return mt, err
}

// NewRequestDebugDump creates a new RequestDebugDump from an HTTP request.
func NewRequestDebugDump(r *http.Request) (*RequestDebugDump, error) {
	mt, err := parseMediaType(r)
	if err != nil {
		return nil, err
	}
	if mt == "multipart/form-data" {
		if err := r.ParseMultipartForm(maxDumpFormParseMem); err != nil {
			return nil, err
		}
	} else {
		if err := r.ParseForm(); err != nil {
			return nil, err
		}
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, err
	}

	dump := &RequestDebugDump{
		Method: r.Method,
		URL: struct {
			Path    string
			RawPath string
		}{Path: r.URL.Path, RawPath: r.URL.RawPath},
		RequestURI: r.RequestURI,
		Host:       r.Host,
		RemoteAddr: r.RemoteAddr,
		Header:     r.Header,
		Form:       r.Form,
		Body:       string(body),
	}

	if event, ok := APIGatewayV1RequestFromContext(r.Context()); ok {
		dump.APIGatewayV1Request = &event
	}
	if event, ok := APIGatewayV2RequestFromContext(r.Context()); ok {
		dump.APIGatewayV2Request = &event
	}
	if event, ok := ALBRequestFromContext(r.Context()); ok {
		dump.ALBRequest = &event
	}

	return dump, nil
}

// RequestDebugDumpHandler is an HTTP handler that returns JSON encoded RequestDebugDump.
func RequestDebugDumpHandler(w http.ResponseWriter, r *http.Request) {
	dump, err := NewRequestDebugDump(r)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = io.WriteString(w, err.Error())
		return
	}
	enc := json.NewEncoder(w)
	if err := enc.Encode(dump); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = io.WriteString(w, err.Error())
		return
	}
	return
}
