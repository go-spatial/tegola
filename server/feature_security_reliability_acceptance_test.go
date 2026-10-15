package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alexeydott/geom"
	"github.com/alexeydott/tegola/ogc/features"
	"github.com/alexeydott/tegola/provider"
)

func TestFeatureSecurityRejectsInjectionBeforeProvider(t *testing.T) {
	observer := &phase09Query{text: "literal"}
	srv := phase09Server(t, []features.CollectionSource{{ID: "items", Layer: phase09FullLayer{}, Querier: observer}}, FeatureAPIConfig{}, "/")
	for _, filter := range []string{"n=1; DROP TABLE items", "n=1 -- comment", `"n; DROP TABLE items" = 1`, `"missing" = 1`, strings.Repeat("NOT ", 256) + "TRUE", strings.Repeat("(", 256) + "TRUE" + strings.Repeat(")", 256), "n = 1e1000000000", "s = 'unterminated"} {
		for _, method := range []string{http.MethodGet, http.MethodHead} {
			t.Run(method+strconv.Itoa(len(filter))+url.QueryEscape(filter[:min(len(filter), 24)]), func(t *testing.T) {
				before := observer.calls.Load()
				r := protocolRequest(t, srv, method, "/features/collections/items/items?filter="+url.QueryEscape(filter))
				if r.status != 400 || r.header.Get("Cache-Control") != "no-store" || r.header.Get("Content-Crs") != "" || observer.calls.Load() != before {
					t.Fatalf("unsafe input reached provider/status%d calls%d", r.status, observer.calls.Load()-before)
				}
				if method == http.MethodHead && len(r.body) != 0 {
					t.Fatal("HEAD error body")
				}
			})
		}
	}
}

type phase10PanicValue struct{ formatted *atomic.Int64 }

func (v phase10PanicValue) Error() string  { v.formatted.Add(1); return "phase10-panic-secret" }
func (v phase10PanicValue) String() string { v.formatted.Add(1); return "phase10-panic-secret" }

type phase10PanicQuery struct {
	panicOnce atomic.Bool
	formatted atomic.Int64
}

func (p *phase10PanicQuery) QueryFeatures(ctx context.Context, _ string, _ provider.FeatureQuery, fn func(*provider.Feature) error) (provider.FeatureQueryResult, error) {
	if p.panicOnce.CompareAndSwap(true, false) {
		panic(phase10PanicValue{&p.formatted})
	}
	if err := ctx.Err(); err != nil {
		return provider.FeatureQueryResult{}, err
	}
	if err := fn(&provider.Feature{ID: 1, SRID: 4326, Geometry: geom.Point{15, 30}, Tags: map[string]any{"name": "healthy"}}); err != nil {
		return provider.FeatureQueryResult{}, err
	}
	return provider.FeatureQueryResult{NumberReturned: 1}, nil
}
func TestFeatureSecurityPanicContainmentAndRecovery(t *testing.T) {
	for _, method := range []string{http.MethodGet, http.MethodHead} {
		for _, format := range []string{"json", "html"} {
			t.Run(method+format, func(t *testing.T) {
				p := &phase10PanicQuery{}
				p.panicOnce.Store(true)
				srv := phase09Server(t, []features.CollectionSource{{ID: "items", Layer: protocolLayer{}, Querier: p}}, FeatureAPIConfig{}, "/")
				response := protocolRequest(t, srv, method, "/features/collections/items/items?f="+format)
				if response.status != 500 || response.header.Get("Content-Type") != "application/json" || response.header.Get("Content-Crs") != "" || response.header.Get("Cache-Control") != "no-store" || p.formatted.Load() != 0 {
					t.Fatalf("panic containment status%d formatted%d headers%v", response.status, p.formatted.Load(), response.header)
				}
				if method == http.MethodHead {
					if len(response.body) != 0 {
						t.Fatal("HEAD body")
					}
				} else if strings.Contains(string(response.body), "phase10-panic-secret") || !strings.Contains(string(response.body), `"code":"InternalError"`) {
					t.Fatal("panic leaked or missing generic error")
				}
				next := protocolRequest(t, srv, http.MethodGet, "/features/collections/items/items?f=json")
				if next.status != 200 || !strings.Contains(string(next.body), "healthy") {
					t.Fatal("server did not recover for next request")
				}
			})
		}
	}
}

type phase10PanicMarshaler struct{ value phase10PanicValue }

func (v phase10PanicMarshaler) MarshalJSON() ([]byte, error) { panic(v.value) }
func TestFeatureSecurityEncoderPanicContainment(t *testing.T) {
	for _, method := range []string{http.MethodGet, http.MethodHead} {
		for _, format := range []string{"json", "html"} {
			t.Run(method+format, func(t *testing.T) {
				var formatted atomic.Int64
				api := &FeatureAPI{cfg: FeatureAPIConfig{MaxResponseBytes: 4096, QueryTimeout: time.Second}}
				r := httptest.NewRequest(method, "/features?f="+format, nil)
				w := httptest.NewRecorder()
				api.serveFeatureBuffered(w, r, api.protocolHandler(api.negotiate(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Crs", "private")
					api.writeRepresentation(w, r, 200, "application/json", phase10PanicMarshaler{phase10PanicValue{&formatted}}, nil)
				}), "application/json")))
				if w.Code != 500 || w.Header().Get("Content-Type") != "application/json" || w.Header().Get("Content-Crs") != "" || w.Header().Get("Cache-Control") != "no-store" || formatted.Load() != 0 {
					t.Fatalf("encoderpanic status%d formatted%d headers%v", w.Code, formatted.Load(), w.Header())
				}
				if method == http.MethodHead && w.Body.Len() != 0 {
					t.Fatal("HEAD body")
				}
				if method == http.MethodGet && strings.Contains(w.Body.String(), "must-never") {
					t.Fatal("partial source")
				}
			})
		}
	}
}
