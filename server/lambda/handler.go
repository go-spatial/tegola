// Package lambda adapts AWS buffered HTTP events to Tegola's existing router.
package lambda

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/akrylysov/algnhsa"
	awslambda "github.com/aws/aws-lambda-go/lambda"

	"github.com/alexeydott/geom/encoding/mvt"
)

const (
	maxEventBytes    = 6 << 20
	maxBodyBytes     = 1 << 20
	maxResponseBytes = 4 << 20
	maxHeaderBytes   = 64 << 10
	albEnvelopeBytes = 1 << 20
)

// Options contains operator-controlled transport settings. PublicURL is the
// complete external origin and mapping path; inferred stages are not appended.
type Options struct {
	PublicURL           *url.URL
	UseProxyPath        bool
	MaxEventBytes       int
	MaxRequestBodyBytes int
	MaxResponseBytes    int
}

// Handler is immutable and supports concurrent buffered invocations.
type Handler struct {
	options Options
	adapter awslambda.Handler
}

type invocationKey struct{}

type invocation struct {
	root     url.URL
	path     string
	head     bool
	overflow bool
	text     []byte
}

// New wraps an already assembled HTTP handler without starting an AWS runtime.
func New(handler http.Handler, options Options) (*Handler, error) {
	if handler == nil {
		return nil, errors.New("lambda handler is required")
	}
	limits := []*int{&options.MaxEventBytes, &options.MaxRequestBodyBytes, &options.MaxResponseBytes}
	ceilings := []int{maxEventBytes, maxBodyBytes, maxResponseBytes}
	for i, limit := range limits {
		if *limit == 0 {
			*limit = ceilings[i]
		}
		if *limit < 1024 || *limit > ceilings[i] {
			return nil, errors.New("invalid lambda transport limit")
		}
	}
	if options.PublicURL != nil {
		root := *options.PublicURL
		if (root.Scheme != "https" && root.Scheme != "http") || !validAuthority(root.Host) ||
			root.User != nil || root.RawQuery != "" || root.Fragment != "" || root.Opaque != "" ||
			!validPath(root.EscapedPath(), true) ||
			strings.Contains(strings.ToLower(root.EscapedPath()), "%2f") ||
			strings.Contains(strings.ToLower(root.EscapedPath()), "%5c") {
			return nil, errors.New("invalid lambda public URL")
		}
		root.Path = strings.TrimSuffix(root.Path, "/")
		root.RawPath = strings.TrimSuffix(root.RawPath, "/")
		options.PublicURL = &root
	}
	h := &Handler{options: options}
	h.adapter = algnhsa.New(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		state := r.Context().Value(invocationKey{}).(*invocation)
		path, err := url.PathUnescape(state.path)
		if err != nil {
			panic("validated lambda path became invalid")
		}
		r.URL.Path, r.URL.RawPath = path, state.path
		r.URL.Scheme, r.URL.Host = state.root.Scheme, state.root.Host
		r.Host = state.root.Host
		r.RequestURI = r.URL.RequestURI()
		writer := &boundedWriter{ResponseWriter: w, state: state, remaining: options.MaxResponseBytes}
		handler.ServeHTTP(writer, r)
		if state.head && w.Header().Get("Content-Length") != "" {
			length, err := strconv.ParseUint(w.Header().Get("Content-Length"), 10, 64)
			if err != nil || length > uint64(options.MaxResponseBytes) {
				state.overflow = true
			}
		}
		if !responseHeadersValid(w.Header()) ||
			(w.Header().Get("Content-Type") != mvt.MimeType && !utf8.Valid(state.text)) {
			state.overflow = true
		}
	}), &algnhsa.Options{BinaryContentTypes: []string{mvt.MimeType}})
	return h, nil
}

// URLRoot returns a detached trusted public root for a Lambda request. Callers
// install this function once at assembly, never mutate it per invocation.
func URLRoot(r *http.Request) *url.URL {
	if state, ok := r.Context().Value(invocationKey{}).(*invocation); ok {
		root := state.root
		return &root
	}
	return &url.URL{}
}

// Invoke preserves the invocation context through the ordinary HTTP router.
// Invalid transport data returns fixed value-free errors before handler I/O.
func (h *Handler) Invoke(ctx context.Context, payload []byte) ([]byte, error) {
	if len(payload) > h.options.MaxEventBytes || !utf8.Valid(payload) {
		return nil, errors.New("invalid lambda event size or encoding")
	}
	event, state, alb, err := h.prepare(payload)
	if err != nil {
		return nil, err
	}
	response, err := h.adapter.Invoke(context.WithValue(ctx, invocationKey{}, state), event)
	if err != nil {
		// Dependency URL errors may contain event URLs and private query values.
		return nil, errors.New("convert lambda HTTP event")
	}
	limit := h.options.MaxEventBytes
	if alb && limit > albEnvelopeBytes {
		limit = albEnvelopeBytes
	}
	var output map[string]json.RawMessage
	if err := json.Unmarshal(response, &output); err != nil {
		return nil, fmt.Errorf("decode lambda HTTP response: %w", err)
	}
	if alb {
		var status int
		if err := json.Unmarshal(output["statusCode"], &status); err != nil {
			return nil, fmt.Errorf("decode lambda response status: %w", err)
		}
		output["statusDescription"], err = json.Marshal(fmt.Sprintf("%d %s", status, http.StatusText(status)))
		if err != nil {
			return nil, err
		}
		if _, ok := output["isBase64Encoded"]; !ok {
			output["isBase64Encoded"] = json.RawMessage("false")
		}
	}
	if state.head {
		output["body"] = json.RawMessage(`""`)
		output["isBase64Encoded"] = json.RawMessage("false")
	}
	response, err = json.Marshal(output)
	if err != nil {
		return nil, fmt.Errorf("encode lambda HTTP response: %w", err)
	}
	if state.overflow || len(response) > limit {
		return transportFailure(alb, state.head), nil
	}
	return response, nil
}

type eventFields struct {
	Version           string              `json:"version"`
	HTTPMethod        string              `json:"httpMethod"`
	Path              string              `json:"path"`
	RawPath           string              `json:"rawPath"`
	RawQueryString    string              `json:"rawQueryString"`
	Body              string              `json:"body"`
	IsBase64Encoded   bool                `json:"isBase64Encoded"`
	Headers           map[string]string   `json:"headers"`
	MultiValueHeaders map[string][]string `json:"multiValueHeaders"`
	MultiValueQuery   map[string][]string `json:"multiValueQueryStringParameters"`
	SingleValueQuery  map[string]string   `json:"queryStringParameters"`
	Cookies           []string            `json:"cookies"`
	PathParameters    map[string]string   `json:"pathParameters"`
	RequestContext    struct {
		AccountID  string `json:"accountId"`
		DomainName string `json:"domainName"`
		Stage      string `json:"stage"`
		HTTP       struct {
			Method string `json:"method"`
		} `json:"http"`
		ELB struct {
			TargetGroupARN string `json:"targetGroupArn"`
		} `json:"elb"`
	} `json:"requestContext"`
}

func (h *Handler) prepare(payload []byte) ([]byte, *invocation, bool, error) {
	if err := validateJSON(payload); err != nil {
		return nil, nil, false, err
	}
	var fields eventFields
	var event map[string]json.RawMessage
	if err := json.Unmarshal(payload, &fields); err != nil {
		return nil, nil, false, errors.New("invalid lambda event JSON")
	}
	if err := json.Unmarshal(payload, &event); err != nil {
		return nil, nil, false, errors.New("invalid lambda event JSON")
	}
	alb := fields.RequestContext.ELB.TargetGroupARN != ""
	v2 := fields.Version == "2.0"
	v1 := (fields.Version == "" || fields.Version == "1.0") && fields.RequestContext.AccountID != ""
	if (alb && (v1 || v2)) || (!alb && !v1 && !v2) ||
		(v2 && fields.HTTPMethod != "") || (alb && fields.Version != "") {
		return nil, nil, false, errors.New("unsupported lambda event shape")
	}
	method, path := fields.HTTPMethod, fields.Path
	if v2 {
		method, path = fields.RequestContext.HTTP.Method, fields.RawPath
	}
	if !validToken(method) || !validPath(path, false) {
		return nil, nil, false, errors.New("invalid lambda method or path")
	}
	if h.options.UseProxyPath && !alb {
		if proxy, ok := fields.PathParameters["proxy"]; ok {
			path = "/" + strings.TrimPrefix(proxy, "/")
			if !validPath(path, false) {
				return nil, nil, false, errors.New("invalid lambda proxy path")
			}
		}
	}
	if err := validateHeaders(fields.Headers, fields.MultiValueHeaders, fields.Cookies); err != nil {
		return nil, nil, false, err
	}
	if v2 {
		query, err := url.ParseQuery(fields.RawQueryString)
		if err != nil || !validQueryValues(query) {
			return nil, nil, false, errors.New("invalid lambda raw query")
		}
		// The v2 raw string is authoritative, including when it is empty.
		delete(event, "queryStringParameters")
		delete(event, "multiValueQueryStringParameters")
	} else if !alb {
		if !validQueryValues(headerValues(fields.SingleValueQuery)) || !validQueryValues(fields.MultiValueQuery) {
			return nil, nil, false, errors.New("invalid lambda query encoding")
		}
	}
	if alb {
		if h.options.PublicURL == nil || len(fields.MultiValueHeaders) == 0 {
			return nil, nil, false, errors.New("ALB requires public URL and multi-value headers")
		}
		query := make(map[string][]string, len(fields.MultiValueQuery))
		for key, values := range fields.MultiValueQuery {
			decoded, err := url.QueryUnescape(key)
			if err != nil || !utf8.ValidString(decoded) {
				return nil, nil, false, errors.New("invalid ALB query encoding")
			}
			for _, value := range values {
				if decoded, err := url.QueryUnescape(value); err != nil || !utf8.ValidString(decoded) {
					return nil, nil, false, errors.New("invalid ALB query encoding")
				}
			}
			query[decoded] = append(query[decoded], values...)
		}
		encoded, err := json.Marshal(query)
		if err != nil {
			return nil, nil, false, err
		}
		event["multiValueQueryStringParameters"] = encoded
	}
	bodyLength := len(fields.Body)
	if fields.IsBase64Encoded {
		if base64.StdEncoding.DecodedLen(len(fields.Body)) > h.options.MaxRequestBodyBytes+2 {
			return nil, nil, false, errors.New("lambda request body too large")
		}
		body, err := base64.StdEncoding.Strict().DecodeString(fields.Body)
		if err != nil {
			return nil, nil, false, errors.New("invalid lambda request body encoding")
		}
		bodyLength = len(body)
	}
	if bodyLength > h.options.MaxRequestBodyBytes {
		return nil, nil, false, errors.New("lambda request body too large")
	}
	root := url.URL{Scheme: "https", Host: fields.RequestContext.DomainName}
	if h.options.PublicURL != nil {
		root = *h.options.PublicURL
	} else {
		if !validAuthority(root.Host) {
			return nil, nil, false, errors.New("lambda trusted public authority required")
		}
		stage := fields.RequestContext.Stage
		if stage != "" && stage != "$default" {
			if !validStage(stage) {
				return nil, nil, false, errors.New("invalid lambda stage")
			}
			root.Path = "/" + stage
		}
	}
	// algnhsa constructs its initial URL from Host before our HTTP wrapper.
	// Replace all casing variants with the proven authority before conversion.
	for key := range fields.Headers {
		if strings.EqualFold(key, "Host") {
			delete(fields.Headers, key)
		}
	}
	if fields.Headers == nil {
		fields.Headers = make(map[string]string)
	}
	fields.Headers["Host"] = root.Host
	for key := range fields.MultiValueHeaders {
		if strings.EqualFold(key, "Host") {
			delete(fields.MultiValueHeaders, key)
		}
	}
	if alb {
		fields.MultiValueHeaders["Host"] = []string{root.Host}
	}
	encodedHeaders, err := json.Marshal(fields.Headers)
	if err != nil {
		return nil, nil, false, err
	}
	event["headers"] = encodedHeaders
	encodedMulti, err := json.Marshal(fields.MultiValueHeaders)
	if err != nil {
		return nil, nil, false, err
	}
	event["multiValueHeaders"] = encodedMulti
	// Do not ask algnhsa to join/clean proxy paths; the exact path is restored
	// on the request after its conversion and the original context is retained.
	encodedPath, err := json.Marshal(path)
	if err != nil {
		return nil, nil, false, err
	}
	if v2 {
		event["rawPath"] = encodedPath
	} else {
		event["path"] = encodedPath
	}
	prepared, err := json.Marshal(event)
	if err != nil {
		return nil, nil, false, err
	}
	return prepared, &invocation{root: root, path: path, head: method == http.MethodHead}, alb, nil
}

func validAuthority(authority string) bool {
	if authority == "" || len(authority) > 253 || strings.ContainsAny(authority, "/\\?#@ \t\r\n") {
		return false
	}
	u, err := url.Parse("https://" + authority)
	return err == nil && u.Host == authority && u.Hostname() != "" && u.User == nil
}

func validQueryValues(values map[string][]string) bool {
	for key, entries := range values {
		if !utf8.ValidString(key) {
			return false
		}
		for _, value := range entries {
			if !utf8.ValidString(value) {
				return false
			}
		}
	}
	return true
}

func validStage(stage string) bool {
	if len(stage) > 128 {
		return false
	}
	for _, c := range stage {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') ||
			(c >= '0' && c <= '9') || c == '_' || c == '-') {
			return false
		}
	}
	return stage != ""
}

func validPath(path string, allowEmpty bool) bool {
	if path == "" {
		return allowEmpty
	}
	if len(path) > 8192 || !strings.HasPrefix(path, "/") || strings.HasPrefix(path, "//") ||
		strings.ContainsAny(path, "?#\\") || !utf8.ValidString(path) {
		return false
	}
	decoded, err := url.PathUnescape(path)
	if err != nil || !utf8.ValidString(decoded) {
		return false
	}
	for _, c := range decoded {
		if c < 32 || c == 127 {
			return false
		}
	}
	return true
}

func validToken(value string) bool {
	if value == "" || len(value) > 256 {
		return false
	}
	for _, c := range value {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') ||
			(c >= '0' && c <= '9') || strings.ContainsRune("!#$%&'*+-.^_`|~", c)) {
			return false
		}
	}
	return true
}

func validateHeaders(single map[string]string, multi map[string][]string, cookies []string) error {
	for _, cookie := range cookies {
		for _, c := range cookie {
			if c < 32 || c == 127 {
				return errors.New("invalid lambda cookie value")
			}
		}
	}
	seen := make(map[string]bool)
	bytes := 0
	entriesList := []map[string][]string{headerValues(single), multi}
	if len(cookies) > 0 {
		entriesList = append(entriesList, map[string][]string{"Cookie": cookies})
	}
	for _, entries := range entriesList {
		clear(seen)
		for key, values := range entries {
			bytes += len(key)
			canonical := http.CanonicalHeaderKey(key)
			if !validToken(key) || seen[canonical] {
				return errors.New("invalid lambda header names")
			}
			seen[canonical] = true
			for _, value := range values {
				bytes += len(key) + len(value)
				for _, c := range value {
					if (c < 32 && c != '\t') || c == 127 {
						return errors.New("invalid lambda header value")
					}
				}
			}
		}
	}
	if bytes > maxHeaderBytes {
		return errors.New("lambda headers too large")
	}
	return nil
}

func headerValues(single map[string]string) map[string][]string {
	result := make(map[string][]string, len(single))
	for key, value := range single {
		result[key] = []string{value}
	}
	return result
}

type boundedWriter struct {
	http.ResponseWriter
	state     *invocation
	remaining int
}

func (w *boundedWriter) WriteHeader(status int) {
	if !responseHeadersValid(w.Header()) {
		w.state.overflow = true
		return
	}
	w.ResponseWriter.WriteHeader(status)
}

func (w *boundedWriter) Write(p []byte) (int, error) {
	if !responseHeadersValid(w.Header()) || len(p) > w.remaining {
		w.state.overflow = true
		return 0, errors.New("lambda response exceeds transport limit")
	}
	w.remaining -= len(p)
	if w.state.head {
		return len(p), nil
	}
	if w.Header().Get("Content-Type") != mvt.MimeType {
		w.state.text = append(w.state.text, p...)
	}
	return w.ResponseWriter.Write(p)
}

func responseHeadersValid(headers http.Header) bool {
	return validateHeaders(nil, headers, nil) == nil
}

// JSON event objects must not depend on last-key-wins or unbounded nesting.
func validateJSON(payload []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.UseNumber()
	nodes := 0
	var readValue func(int, string) error
	readValue = func(depth int, objectPath string) error {
		nodes++
		if depth > 32 || nodes > 65536 {
			return errors.New("lambda event structure exceeds limit")
		}
		token, err := decoder.Token()
		if err != nil {
			return errors.New("invalid lambda event JSON")
		}
		delimiter, ok := token.(json.Delim)
		if !ok {
			return nil
		}
		if delimiter == '{' {
			keys := make(map[string]bool)
			for decoder.More() {
				key, err := decoder.Token()
				name, ok := key.(string)
				// encoding/json matches consumed struct fields case-insensitively.
				// Header/query maps remain case-sensitive and are not structs.
				identity := name
				if objectPath == "" || objectPath == "/requestcontext" ||
					objectPath == "/requestcontext/http" || objectPath == "/requestcontext/elb" {
					for _, c := range name {
						if c > 127 {
							return errors.New("invalid lambda event field name")
						}
					}
					identity = strings.ToLower(name)
				}
				if err != nil || !ok || keys[identity] {
					return errors.New("ambiguous lambda event JSON")
				}
				keys[identity] = true
				if err := readValue(depth+1, objectPath+"/"+strings.ToLower(name)); err != nil {
					return err
				}
			}
		} else if delimiter == '[' {
			for decoder.More() {
				if err := readValue(depth+1, objectPath+"/[]"); err != nil {
					return err
				}
			}
		} else {
			return errors.New("invalid lambda event JSON")
		}
		if _, err := decoder.Token(); err != nil {
			return errors.New("invalid lambda event JSON")
		}
		return nil
	}
	if err := readValue(0, ""); err != nil {
		return err
	}
	if _, err := decoder.Token(); err != io.EOF {
		return errors.New("invalid lambda event JSON")
	}
	return nil
}

func transportFailure(alb, head bool) []byte {
	const body = `{"code":"TransportLimit","description":"Response exceeds Lambda transport limit"}`
	header := map[string][]string{
		"Content-Type": {"application/json"}, "Cache-Control": {"no-store"},
		"Content-Length": {strconv.Itoa(len(body))},
	}
	output := map[string]any{"statusCode": 502, "body": body, "isBase64Encoded": false}
	if head {
		output["body"] = ""
	}
	if alb {
		output["statusDescription"] = "502 Bad Gateway"
		output["multiValueHeaders"] = header
	} else {
		single := make(map[string]string, len(header))
		for key, values := range header {
			single[key] = values[0]
		}
		output["headers"] = single
	}
	// All inputs are fixed scalar values; encoding cannot fail.
	encoded, err := json.Marshal(output)
	if err != nil {
		panic("fixed lambda transport response cannot encode")
	}
	return encoded
}
