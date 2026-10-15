package lambda_test

import (
	"context"
	"encoding/base64"
	"encoding/json"

	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/alexeydott/geom"
	"github.com/alexeydott/geom/encoding/mvt"
	"github.com/alexeydott/tegola/ogc/features"
	"github.com/alexeydott/tegola/provider"
	"github.com/alexeydott/tegola/server"
	lambdaserver "github.com/alexeydott/tegola/server/lambda"
)

type testContextKey struct{}

type layer struct{}

func (layer) Name() string                 { return "source" }
func (layer) SRID() uint64                 { return 4326 }
func (layer) GeomType() geom.Geometry      { return geom.Point{} }
func (layer) FeatureQuerySupported() error { return nil }
func (layer) TemporalMapping() (provider.TemporalMapping, error) {
	return provider.TemporalMapping{}, nil
}
func (layer) SpatialMetadata() (provider.SpatialMetadata, error) {
	return provider.SpatialMetadata{Dimension: provider.DimensionXY}, nil
}

type querier struct{ panic bool }

func (q querier) QueryFeatures(ctx context.Context, _ string, query provider.FeatureQuery, fn func(*provider.Feature) error) (provider.FeatureQueryResult, error) {
	if q.panic {
		panic("must be contained by actual router")
	}
	if err := ctx.Err(); err != nil {
		return provider.FeatureQueryResult{}, err
	}
	if len(query.IDs) > 0 && query.IDs[0] != 7 {
		return provider.FeatureQueryResult{}, nil
	}
	if err := fn(&provider.Feature{ID: 7, SRID: 4326, Geometry: geom.Point{15, 30}, Tags: map[string]any{"name": nil}}); err != nil {
		return provider.FeatureQueryResult{}, err
	}
	n := uint64(1)
	return provider.FeatureQueryResult{NumberReturned: 1, NumberMatched: &n}, nil
}
func actualRouter(t *testing.T, q querier) http.Handler {
	t.Helper()
	oldRoot, oldPrefix := server.URLRoot, server.URIPrefix
	server.URLRoot = lambdaserver.URLRoot
	server.URIPrefix = "/"
	t.Cleanup(func() { server.URLRoot, server.URIPrefix = oldRoot, oldPrefix })
	s, err := features.NewService([]features.CollectionSource{{ID: "places", Layer: layer{}, Querier: q}})
	if err != nil {
		t.Fatal(err)
	}
	api, err := server.NewFeatureAPI(s, server.FeatureAPIConfig{BasePath: "/features", DefaultLimit: 10, MaxLimit: 100})
	if err != nil {
		t.Fatal(err)
	}
	r, err := server.NewRouterWithOptions(nil, server.RouterOptions{Features: api})
	if err != nil {
		t.Fatal(err)
	}
	return r
}
func publicURL(t *testing.T) *url.URL {
	t.Helper()
	u, err := url.Parse("https://public.example/mapping")
	if err != nil {
		t.Fatal(err)
	}
	return u
}
func event(mode, method, path string) map[string]any {
	headers := map[string]string{"Host": "attacker.example", "X-Forwarded-Proto": "javascript:", "X-Forwarded-Host": "attacker.example"}
	e := map[string]any{"httpMethod": method, "path": path, "headers": headers,
		"requestContext": map[string]any{"accountId": "account", "domainName": "gateway.example", "stage": "stage"}}
	if mode == "v2" || mode == "function_url" {
		stage := "stage"
		if mode == "function_url" {
			stage = "$default"
		}
		delete(e, "httpMethod")
		delete(e, "path")
		e["version"] = "2.0"
		e["rawPath"] = path
		e["rawQueryString"] = ""
		e["requestContext"] = map[string]any{"domainName": "gateway.example", "stage": stage, "http": map[string]string{"method": method}}
	}
	if mode == "alb" {
		delete(e, "headers")
		e["multiValueHeaders"] = map[string][]string{"host": {"attacker.example"}}
		e["requestContext"] = map[string]any{"elb": map[string]string{"targetGroupArn": "arn:aws:elasticloadbalancing:region:account:targetgroup/owned/id"}}
	}
	return e
}

type response struct {
	Status      int                 `json:"statusCode"`
	Description string              `json:"statusDescription"`
	Headers     map[string]string   `json:"headers"`
	Multi       map[string][]string `json:"multiValueHeaders"`
	Cookies     []string            `json:"cookies"`
	Body        string              `json:"body"`
	Binary      bool                `json:"isBase64Encoded"`
}

func invoke(t *testing.T, h *lambdaserver.Handler, ctx context.Context, e map[string]any) response {
	t.Helper()
	payload, err := json.Marshal(e)
	if err != nil {
		t.Fatal(err)
	}
	output, err := h.Invoke(ctx, payload)
	if err != nil {
		t.Fatal(err)
	}
	var result response
	if err := json.Unmarshal(output, &result); err != nil {
		t.Fatal(err)
	}
	return result
}
func (r response) header(key string) string {
	for k, v := range r.Headers {
		if strings.EqualFold(k, key) {
			return v
		}
	}
	for k, v := range r.Multi {
		if strings.EqualFold(k, key) && len(v) > 0 {
			return v[0]
		}
	}
	return ""
}
func TestActualRouterFourModes(t *testing.T) {
	r := actualRouter(t, querier{})
	for _, mode := range []string{"v1", "v2", "function_url", "alb"} {
		t.Run(mode, func(t *testing.T) {
			h, err := lambdaserver.New(r, lambdaserver.Options{PublicURL: publicURL(t), UseProxyPath: true})
			if err != nil {
				t.Fatal(err)
			}
			for _, path := range []string{"/features", "/features/collections", "/features/collections/places/items", "/features/collections/places/items/7", "/features/collections/places/items/99", "/features/api"} {
				get := invoke(t, h, context.Background(), event(mode, "GET", path))
				want := 200
				if strings.HasSuffix(path, "/99") {
					want = 404
				}
				if get.Status != want || get.header("Cache-Control") != "no-store" || get.Binary {
					t.Fatalf("%s %#v", path, get)
				}
				head := invoke(t, h, context.Background(), event(mode, "HEAD", path))
				if head.Status != get.Status || head.Body != "" || head.header("Content-Length") != get.header("Content-Length") {
					t.Fatalf("HEAD mismatch %#v/%#v", get, head)
				}
				if want == 200 && !strings.Contains(get.Body, "https://public.example/mapping/features") {
					t.Fatalf("missing configured URL: %s", get.Body)
				}
				if strings.Contains(path, "items") && want == 200 {
					if get.header("Content-Crs") != "<http://www.opengis.net/def/crs/OGC/1.3/CRS84>" {
						t.Fatalf("CRS %q", get.header("Content-Crs"))
					}
				}
			}
			options := invoke(t, h, context.Background(), event(mode, "OPTIONS", "/features/collections/places/items"))
			if options.Status != 200 || options.header("Allow") == "" || options.header("Access-Control-Allow-Origin") != "*" {
				t.Fatalf("OPTIONS %#v", options)
			}
			bad := event(mode, "GET", "/features/collections/places/items")
			if mode == "v2" || mode == "function_url" {
				bad["rawQueryString"] = "limit=1&limit=2"
			} else if mode == "alb" {
				bad["multiValueQueryStringParameters"] = map[string][]string{"limit": {"1", "2"}}
			} else {
				bad["multiValueQueryStringParameters"] = map[string][]string{"limit": {"1", "2"}}
			}
			if got := invoke(t, h, context.Background(), bad); got.Status != 400 {
				t.Fatalf("repeated query accepted %#v", got)
			}
			if mode == "alb" && getDescription(t, h) != "200 OK" {
				t.Fatal("ALB statusDescription missing")
			}
		})
	}
}
func getDescription(t *testing.T, h *lambdaserver.Handler) string {
	return invoke(t, h, context.Background(), event("alb", "GET", "/features")).Description
}

func TestStageAndEscapedPath(t *testing.T) {
	for _, mode := range []string{"v1", "v2", "function_url", "alb"} {
		for _, path := range []string{"/a%2Fb/%25", "/a//b/../c", "/a/%252F"} {
			t.Run(mode+path, func(t *testing.T) {
				e := event(mode, "GET", path)
				var root, escaped, uri string
				h, err := lambdaserver.New(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					root = lambdaserver.URLRoot(r).String()
					escaped = r.URL.EscapedPath()
					uri = r.RequestURI
				}), lambdaserver.Options{PublicURL: publicURL(t)})
				if err != nil {
					t.Fatal(err)
				}
				invoke(t, h, context.Background(), e)
				if escaped != path || uri != path || root != "https://public.example/mapping" {
					t.Fatalf("%q %q %q", root, escaped, uri)
				}
			})
		}
	}
	for _, mode := range []string{"v1", "v2", "function_url"} {
		var root string
		h, err := lambdaserver.New(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { root = lambdaserver.URLRoot(r).String() }), lambdaserver.Options{})
		if err != nil {
			t.Fatal(err)
		}
		invoke(t, h, context.Background(), event(mode, "GET", "/"))
		want := "https://gateway.example/stage"
		if mode == "function_url" {
			want = "https://gateway.example"
		}
		if root != want {
			t.Fatalf("%s %q", mode, root)
		}
	}
}

func TestCookiesBinaryContextAndConcurrency(t *testing.T) {
	for _, mode := range []string{"v1", "v2", "function_url", "alb"} {
		t.Run(mode, func(t *testing.T) {
			var cookies []*http.Cookie
			ctxKey := testContextKey{}
			ctx := context.WithValue(context.Background(), ctxKey, "present")
			h, err := lambdaserver.New(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Context().Value(ctxKey) != "present" {
					t.Error("context lost")
				}
				cookies = r.Cookies()
				w.Header().Add("Set-Cookie", "a=1")
				w.Header().Add("Set-Cookie", "b=2")
				w.Header().Set("Content-Type", mvt.MimeType)
				if _, err := w.Write([]byte{0, 255, 1, 2}); err != nil {
					t.Error(err)
				}
			}), lambdaserver.Options{PublicURL: publicURL(t)})
			if err != nil {
				t.Fatal(err)
			}
			e := event(mode, "GET", "/")
			if mode == "v2" || mode == "function_url" {
				e["cookies"] = []string{"a=1", "b=2"}
			} else {
				e["multiValueHeaders"] = map[string][]string{"Cookie": {"a=1", "b=2"}}
			}
			got := invoke(t, h, ctx, e)
			body, err := base64.StdEncoding.DecodeString(got.Body)
			if err != nil || !got.Binary || !reflect.DeepEqual(body, []byte{0, 255, 1, 2}) || len(cookies) != 2 {
				t.Fatalf("binary/cookies %#v %#v", got, cookies)
			}
			if mode == "v2" || mode == "function_url" {
				if len(got.Cookies) != 2 {
					t.Fatal("response cookies lost")
				}
			} else if len(got.Multi["Set-Cookie"]) != 2 {
				t.Fatal("response cookies lost")
			}
		})
	}
	h, err := lambdaserver.New(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := w.Write([]byte("ok")); err != nil {
			t.Error(err)
		}
	}), lambdaserver.Options{})
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); invoke(t, h, context.Background(), event("v2", "GET", "/")) }()
	}
	wg.Wait()
}

func TestActualRouterPanicAndCancellation(t *testing.T) {
	for _, panicProvider := range []bool{false, true} {
		r := actualRouter(t, querier{panic: panicProvider})
		h, err := lambdaserver.New(r, lambdaserver.Options{PublicURL: publicURL(t)})
		if err != nil {
			t.Fatal(err)
		}
		ctx := context.Background()
		if !panicProvider {
			var cancel context.CancelFunc
			ctx, cancel = context.WithCancel(ctx)
			cancel()
		}
		got := invoke(t, h, ctx, event("v2", "GET", "/features/collections/places/items"))
		want := 408
		if panicProvider {
			want = 500
		}
		if got.Status != want || got.header("Cache-Control") != "no-store" || strings.Contains(got.Body, "must be contained") {
			t.Fatalf("%#v", got)
		}
	}
}

func TestTransportRejectsBeforeHTTP(t *testing.T) {
	for _, name := range []string{"origin", "base64", "headers", "header_case", "shape", "path", "body", "event", "alb_single", "alb_query"} {
		t.Run(name, func(t *testing.T) {
			called := false
			h, err := lambdaserver.New(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { called = true }), lambdaserver.Options{MaxEventBytes: 2048, MaxRequestBodyBytes: 1024})
			if err != nil {
				t.Fatal(err)
			}
			e := event("v2", "GET", "/")
			switch name {
			case "origin":
				e["requestContext"] = map[string]any{"http": map[string]string{"method": "GET"}}
			case "base64":
				e["body"] = "!!"
				e["isBase64Encoded"] = true
			case "headers":
				e["headers"] = map[string]string{"X-Test": "a\r\nb"}
			case "header_case":
				e["headers"] = map[string]string{"Accept": "a", "accept": "b"}
			case "shape":
				e["httpMethod"] = "GET"
			case "path":
				e["rawPath"] = "/%zz"
			case "body":
				e["body"] = strings.Repeat("x", 1025)
			case "event":
				e["unused"] = strings.Repeat("x", 3000)
			case "alb_single":
				e = event("alb", "GET", "/")
				delete(e, "multiValueHeaders")
			case "alb_query":
				e = event("alb", "GET", "/")
				e["multiValueQueryStringParameters"] = map[string][]string{"%zz": {"a"}}
			}
			payload, err := json.Marshal(e)
			if err != nil {
				t.Fatal(err)
			}
			if _, err = h.Invoke(context.Background(), payload); err == nil || called {
				t.Fatalf("err=%v called=%v", err, called)
			}
		})
	}
	h, err := lambdaserver.New(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { t.Error("duplicate JSON reached handler") }), lambdaserver.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = h.Invoke(context.Background(), []byte(`{"version":"2.0","version":"1.0"}`)); err == nil {
		t.Fatal("duplicate JSON accepted")
	}
}

func TestALBQueryDecodedKeyCollisions(t *testing.T) {
	var query url.Values
	h, err := lambdaserver.New(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { query = r.URL.Query() }), lambdaserver.Options{PublicURL: publicURL(t)})
	if err != nil {
		t.Fatal(err)
	}
	e := event("alb", "GET", "/")
	e["multiValueQueryStringParameters"] = map[string][]string{"a": {"x%252Fy"}, "%61": {"z%20q"}}
	invoke(t, h, context.Background(), e)
	if len(query) != 1 || len(query["a"]) != 2 || !contains(query["a"], "x%2Fy") || !contains(query["a"], "z q") {
		t.Fatalf("%#v", query)
	}
}
func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func TestTransportResponseLimitsAndOptions(t *testing.T) {
	for _, mode := range []string{"v2", "alb"} {
		for _, kind := range []string{"body", "headers", "bad_headers", "utf8", "envelope"} {
			t.Run(mode+kind, func(t *testing.T) {
				h, err := lambdaserver.New(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					switch kind {
					case "headers":
						w.Header().Set("X-Too-Large", strings.Repeat("x", 65537))
					case "bad_headers":
						w.Header().Set("X-Test", "a\r\nb")
					case "utf8":
						w.Header().Set("Content-Type", "application/geo+json")
						_, err := w.Write([]byte{255})
						if err != nil && !strings.Contains(err.Error(), "transport limit") {
							t.Error(err)
						}
						return
					}
					body := strings.Repeat("x", 2048)
					if kind == "envelope" {
						body = strings.Repeat("\x00", 900)
					}
					if _, err := w.Write([]byte(body)); err != nil && !strings.Contains(err.Error(), "transport limit") {
						t.Error(err)
					}
				}), lambdaserver.Options{PublicURL: publicURL(t), MaxResponseBytes: 1024, MaxEventBytes: 2048})
				if err != nil {
					t.Fatal(err)
				}
				got := invoke(t, h, context.Background(), event(mode, "GET", "/"))
				if got.Status != 502 || got.header("Cache-Control") != "no-store" {
					t.Fatalf("%#v", got)
				}
			})
		}
	}
	for _, options := range []lambdaserver.Options{{MaxEventBytes: -1}, {MaxResponseBytes: 1}, {PublicURL: &url.URL{Scheme: "javascript", Host: "a"}}} {
		if _, err := lambdaserver.New(http.NotFoundHandler(), options); err == nil {
			t.Fatal("bad options accepted")
		}
	}
	u := publicURL(t)
	h, err := lambdaserver.New(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if lambdaserver.URLRoot(r).Host != "public.example" {
			t.Error("root mutated")
		}
	}), lambdaserver.Options{PublicURL: u})
	if err != nil {
		t.Fatal(err)
	}
	u.Host = "evil.example"
	invoke(t, h, context.Background(), event("v2", "GET", "/"))
	if _, err := lambdaserver.New(nil, lambdaserver.Options{}); err == nil {
		t.Fatal("nil handler accepted")
	}
}

// Direct transport comparison excludes only gateway envelope shape; actual
// handler bytes, headers and status remain the ordinary HTTP implementation.
func TestDirectHTTPTransportParity(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(201)
		if _, err := w.Write([]byte(`{"ok":true}`)); err != nil {
			t.Error(err)
		}
	})
	direct := httptest.NewRecorder()
	handler.ServeHTTP(direct, httptest.NewRequest("GET", "https://public.example/", nil))
	h, err := lambdaserver.New(handler, lambdaserver.Options{})
	if err != nil {
		t.Fatal(err)
	}
	got := invoke(t, h, context.Background(), event("v2", "GET", "/"))
	if got.Status != direct.Code || got.Body != direct.Body.String() || got.header("Content-Type") != direct.Header().Get("Content-Type") {
		t.Fatalf("%#v", got)
	}
}

func TestModeSpecificEnvelopeAndBodyBoundaries(t *testing.T) {
	body := strings.Repeat("x", 1<<20)
	h, err := lambdaserver.New(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		if _, err := w.Write([]byte(body)); err != nil {
			t.Error(err)
		}
	}), lambdaserver.Options{PublicURL: publicURL(t)})
	if err != nil {
		t.Fatal(err)
	}
	if got := invoke(t, h, context.Background(), event("v2", "GET", "/")); got.Status != 200 || len(got.Body) != len(body) {
		t.Fatal("gateway envelope prematurely limited")
	}
	if got := invoke(t, h, context.Background(), event("alb", "GET", "/")); got.Status != 502 {
		t.Fatal("ALB serialized envelope exceeds one MiB")
	}
	called := 0
	h, err = lambdaserver.New(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { called++ }),
		lambdaserver.Options{MaxRequestBodyBytes: 1024})
	if err != nil {
		t.Fatal(err)
	}
	for _, length := range []int{1024, 1025} {
		e := event("v2", "GET", "/")
		e["body"] = base64.StdEncoding.EncodeToString([]byte(strings.Repeat("x", length)))
		e["isBase64Encoded"] = true
		payload, err := json.Marshal(e)
		if err != nil {
			t.Fatal(err)
		}
		_, err = h.Invoke(context.Background(), payload)
		if (length == 1024 && err != nil) || (length == 1025 && err == nil) {
			t.Fatalf("length=%d err=%v", length, err)
		}
	}
	if called != 1 {
		t.Fatalf("body preflight calls=%d", called)
	}
}

func TestProxyRawQueryAndConfiguredMapping(t *testing.T) {
	for _, mode := range []string{"v1", "v2"} {
		var path string
		h, err := lambdaserver.New(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { path = r.URL.EscapedPath() }),
			lambdaserver.Options{UseProxyPath: true})
		if err != nil {
			t.Fatal(err)
		}
		e := event(mode, "GET", "/original%2Fpath")
		invoke(t, h, context.Background(), e)
		if path != "/original%2Fpath" {
			t.Fatalf("absent proxy rewrote %q", path)
		}
		e["pathParameters"] = map[string]string{"proxy": "a%2Fb/../%25"}
		invoke(t, h, context.Background(), e)
		if path != "/a%2Fb/../%25" {
			t.Fatalf("proxy cleaned %q", path)
		}
	}
	for _, raw := range []string{"a=%zz", "a=%ff", "a=1;b=2"} {
		h, err := lambdaserver.New(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { t.Error("malformed raw query reached HTTP") }), lambdaserver.Options{})
		if err != nil {
			t.Fatal(err)
		}
		e := event("v2", "GET", "/")
		e["rawQueryString"] = raw
		payload, err := json.Marshal(e)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := h.Invoke(context.Background(), payload); err == nil {
			t.Fatalf("raw query accepted %q", raw)
		}
	}
	var query string
	h, err := lambdaserver.New(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { query = r.URL.RawQuery }), lambdaserver.Options{})
	if err != nil {
		t.Fatal(err)
	}
	e := event("v2", "GET", "/")
	e["queryStringParameters"] = map[string]string{"injected": "value"}
	invoke(t, h, context.Background(), e)
	if query != "" {
		t.Fatalf("empty raw query replaced by collapsed map %q", query)
	}
	for _, mapping := range []string{"https://public.example/a%2Fb", "https://public.example/a%5cb"} {
		u, err := url.Parse(mapping)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := lambdaserver.New(http.NotFoundHandler(), lambdaserver.Options{PublicURL: u}); err == nil {
			t.Fatal("encoded mapping separator admitted")
		}
	}
}

func TestCookiesPreflightAndHostNoDisclosure(t *testing.T) {
	for _, cookies := range [][]string{{"a=1\r\nInjected: x"}, {strings.Repeat("x", 65537)}, {strings.Repeat("x", 40000), strings.Repeat("y", 40000)}} {
		called := false
		h, err := lambdaserver.New(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true }), lambdaserver.Options{})
		if err != nil {
			t.Fatal(err)
		}
		e := event("v2", "GET", "/")
		e["cookies"] = cookies
		payload, err := json.Marshal(e)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := h.Invoke(context.Background(), payload); err == nil || called {
			t.Fatalf("cookies reached router: %v", err)
		}
	}
	for _, mode := range []string{"v1", "v2", "function_url", "alb"} {
		var host string
		h, err := lambdaserver.New(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { host = r.Host }),
			lambdaserver.Options{PublicURL: publicURL(t)})
		if err != nil {
			t.Fatal(err)
		}
		e := event(mode, "GET", "/")
		if mode == "alb" {
			e["multiValueHeaders"] = map[string][]string{"Host": {"%invalid-private-host"}}
		} else {
			e["headers"] = map[string]string{"Host": "%invalid-private-host"}
		}
		if mode == "v2" || mode == "function_url" {
			e["rawQueryString"] = "token=private-marker"
		} else {
			e["queryStringParameters"] = map[string]string{"token": "private-marker"}
		}
		payload, err := json.Marshal(e)
		if err != nil {
			t.Fatal(err)
		}
		_, err = h.Invoke(context.Background(), payload)
		if err != nil || host != "public.example" {
			t.Fatalf("trusted Host conversion failed: %v", err)
		}
		e["pathParameters"] = nil
		if mode == "v2" || mode == "function_url" {
			e["rawQueryString"] = "token=private-marker%zz"
		} else {
			e["path"] = "/%zz-private-marker"
		}
		payload, err = json.Marshal(e)
		if err != nil {
			t.Fatal(err)
		}
		_, err = h.Invoke(context.Background(), payload)
		if err == nil || strings.Contains(err.Error(), "private-marker") || strings.Contains(err.Error(), "private-host") {
			t.Fatalf("transport value disclosure: %v", err)
		}
	}
}

func TestCaseAmbiguousConsumedFieldsRejectBeforeHTTP(t *testing.T) {
	h, err := lambdaserver.New(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Error("case ambiguous event reached HTTP")
	}), lambdaserver.Options{})
	if err != nil {
		t.Fatal(err)
	}
	for _, extra := range []string{
		`"cookies":["bad\r\nprivate"],"Cookies":["safe"]`,
		`"body":"` + strings.Repeat("x", 1<<20) + `","Body":"safe"`,
		`"Cookieſ":["bad\r\nprivate"]`,
	} {
		payload := []byte(`{"version":"2.0","rawPath":"/","requestContext":{"domainName":"gateway.example","http":{"method":"GET"}},` + extra + `}`)
		if _, err := h.Invoke(context.Background(), payload); err == nil {
			t.Fatal("ambiguous consumed field accepted")
		}
	}
	var values url.Values
	h, err = lambdaserver.New(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { values = r.URL.Query() }), lambdaserver.Options{})
	if err != nil {
		t.Fatal(err)
	}
	e := event("v1", "GET", "/")
	e["multiValueQueryStringParameters"] = map[string][]string{"Name": {"upper"}, "name": {"lower"}, "ſ": {"unicode"}}
	invoke(t, h, context.Background(), e)
	if values.Get("Name") != "upper" || values.Get("name") != "lower" || values.Get("ſ") != "unicode" {
		t.Fatalf("query key identity lost %#v", values)
	}
}

func TestHEADTransportFailureHasNoBody(t *testing.T) {
	for _, mode := range []string{"v1", "v2", "function_url", "alb"} {
		for _, kind := range []string{"body", "headers", "logical_length", "envelope"} {
			t.Run(mode+kind, func(t *testing.T) {
				h, err := lambdaserver.New(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if kind == "headers" {
						w.Header().Set("X-Large", strings.Repeat("x", 65537))
					}
					if kind == "logical_length" {
						w.Header().Set("Content-Length", "1025")
					}
					if kind == "envelope" {
						w.Header().Set("X-Encoded", strings.Repeat("<", 800))
					}
					if kind == "body" {
						_, err := w.Write([]byte(strings.Repeat("x", 1025)))
						if err == nil {
							t.Error("large write accepted")
						}
					}
				}), lambdaserver.Options{PublicURL: publicURL(t), MaxResponseBytes: 1024, MaxEventBytes: 2048})
				if err != nil {
					t.Fatal(err)
				}
				got := invoke(t, h, context.Background(), event(mode, "HEAD", "/"))
				if got.Status != 502 || got.Body != "" || got.Binary || got.header("Content-Length") == "" {
					t.Fatalf("HEAD transport failure %#v", got)
				}
			})
		}
	}
}
