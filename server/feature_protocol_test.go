package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestFeatureProtocolLimitsAndHeaders(t *testing.T) {
	api := &FeatureAPI{cfg: FeatureAPIConfig{MaxResponseBytes: 1024, QueryTimeout: time.Second}}
	for _, method := range []string{"GET", "HEAD"} {
		for _, tc := range []struct {
			name, query, accept string
			status              int
		}{{"query", strings.Repeat("a", 65537), "", 414}, {"accept", "", strings.Repeat("a", 16385), 431}, {"format", "f=html&f=json", "", 400}} {
			t.Run(method+tc.name, func(t *testing.T) {
				r := httptest.NewRequest(method, "/features?"+tc.query, nil)
				r.Header.Set("Accept", tc.accept)
				w := httptest.NewRecorder()
				api.protocolHandler(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { t.Fatal("reached handler") })).ServeHTTP(w, r)
				if w.Code != tc.status {
					t.Fatalf("status %d", w.Code)
				}
				if method == "HEAD" && w.Body.Len() != 0 {
					t.Fatal("HEAD body")
				}
			})
		}
	}
	h := http.Header{"Vary": []string{"Origin, accept"}, "Access-Control-Expose-Headers": []string{"X-Existing, content-crs"}, "Etag": []string{"unsafe"}, "Last-Modified": []string{"unsafe"}, "Content-Encoding": []string{"gzip"}}
	featureProtocolHeaders(h)
	if h.Get("ETag") != "" || h.Get("Last-Modified") != "" || h.Get("Content-Encoding") != "" || len(h.Values("Vary")) != 1 || len(h.Values("Access-Control-Expose-Headers")) != 1 {
		t.Fatalf("headers %v", h)
	}
}
func TestFeatureRepresentationLimitsHeadAndCancellation(t *testing.T) {
	api := &FeatureAPI{cfg: FeatureAPIConfig{MaxResponseBytes: 1024, QueryTimeout: time.Second}}
	for _, method := range []string{"GET", "HEAD"} {
		r := httptest.NewRequest(method, "/features", nil)
		w := httptest.NewRecorder()
		w.Header().Set("Content-Crs", "private")
		api.writeJSON(w, r, 200, "application/json", strings.Repeat("a", 1024))
		if w.Code != 400 || w.Header().Get("Content-Crs") != "" {
			t.Fatalf("oversize %d %v", w.Code, w.Header())
		}
		if method == "HEAD" && w.Body.Len() != 0 {
			t.Fatal("head body")
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	r := httptest.NewRequest("GET", "/features", nil).WithContext(ctx)
	w := httptest.NewRecorder()
	api.writeJSON(w, r, 200, "application/json", map[string]string{"a": "b"})
	if w.Code != 408 || !strings.Contains(w.Body.String(), "RequestTimeout") {
		t.Fatal(w.Code, w.Body.String())
	}
}
func TestFeatureFormatContextAndNegotiation(t *testing.T) {
	api := &FeatureAPI{cfg: FeatureAPIConfig{BasePath: "/features", QueryTimeout: time.Second, MaxResponseBytes: 1024}, uriPrefix: "/proxy"}
	r := httptest.NewRequest("GET", "/features?f=html&limit=2", nil)
	r.Header.Set("Accept", "application/xml")
	api.protocolHandler(api.negotiate(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if featureSelectedFormat(r) != "html" || r.URL.Query().Get("f") != "" || featureOriginalQuery(r) != "f=html&limit=2" {
			t.Fatal("format context")
		}
		link := api.formatLink(r, "/items", "next", "application/geo+json", url.Values{"offset": []string{"2"}}, featureSelectedFormat(r))
		if !strings.Contains(link.Href, "/proxy/features/items?f=html&offset=2") || !strings.HasPrefix(link.Type, "text/html") {
			t.Fatal(link)
		}
	}), "application/json")).ServeHTTP(httptest.NewRecorder(), r)
	if r.URL.Query().Get("f") != "html" {
		t.Fatal("input mutated")
	}
}
func TestFeatureRepresentationExactBoundaryAndHTML(t *testing.T) {
	api := &FeatureAPI{cfg: FeatureAPIConfig{MaxResponseBytes: 1024, QueryTimeout: time.Second}}
	for _, method := range []string{"GET", "HEAD"} {
		r := httptest.NewRequest(method, "/features", nil)
		w := httptest.NewRecorder()
		api.writeJSON(w, r, 200, "application/json", strings.Repeat("a", 1022))
		if w.Code != 200 || w.Header().Get("Content-Length") != "1024" {
			t.Fatal(w.Code, w.Header())
		}
		if method == "HEAD" && w.Body.Len() != 0 {
			t.Fatal("HEAD body")
		}
	}
	for _, method := range []string{"GET", "HEAD"} {
		r := httptest.NewRequest(method, "/features?f=html", nil)
		w := httptest.NewRecorder()
		api.protocolHandler(api.negotiate(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			api.writeRepresentation(w, r, 200, "application/json", strings.Repeat("a", 2048), nil)
		}), "application/json")).ServeHTTP(w, r)
		if w.Code != 400 || w.Header().Get("Content-Type") != "application/json" {
			t.Fatal(w.Code, w.Header())
		}
		if method == "HEAD" && w.Body.Len() != 0 {
			t.Fatal("HEAD body")
		}
	}
}
func TestFeaturePublicationDeadlineReachesHandler(t *testing.T) {
	api := &FeatureAPI{cfg: FeatureAPIConfig{MaxResponseBytes: 1024, QueryTimeout: time.Millisecond}}
	r := httptest.NewRequest("GET", "/features", nil)
	w := httptest.NewRecorder()
	api.protocolHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
		api.writeJSON(w, r, 200, "application/json", "value")
	})).ServeHTTP(w, r)
	if w.Code != 408 {
		t.Fatal(w.Code, w.Body.String())
	}
}
func TestFeatureNegotiatesAvailableRepresentations(t *testing.T) {
	api := &FeatureAPI{cfg: FeatureAPIConfig{MaxResponseBytes: 4096, QueryTimeout: time.Second}}
	for _, tc := range []struct {
		accept, format string
		status         int
	}{{"", "json", 200}, {"*/*", "json", 200}, {"text/html,application/json", "json", 200}, {"text/html;q=0.8,application/json;q=0.5", "html", 200}, {"application/json;q=0,*/*;q=1", "html", 200}, {"application/json;q=0,text/html;q=0,*/*;q=1", "", 406}} {
		r := httptest.NewRequest("GET", "/features", nil)
		if tc.accept != "" {
			r.Header.Set("Accept", tc.accept)
		}
		w := httptest.NewRecorder()
		api.protocolHandler(api.negotiate(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if featureSelectedFormat(r) != tc.format {
				t.Fatalf("%s -> %s", tc.accept, featureSelectedFormat(r))
			}
			w.WriteHeader(200)
		}), "application/json")).ServeHTTP(w, r)
		if w.Code != tc.status {
			t.Fatalf("%s status %d", tc.accept, w.Code)
		}
	}
}
func TestFeatureHTMLCharsetNegotiation(t *testing.T) {
	api := &FeatureAPI{cfg: FeatureAPIConfig{MaxResponseBytes: 4096, QueryTimeout: time.Second}}
	for _, method := range []string{"GET", "HEAD"} {
		for _, tc := range []struct {
			accept, format string
			status         int
		}{{"text/html;charset=UTF-8", "html", 200}, {"text/html;charset=UTF-8;q=0, text/html;q=1, application/json;q=0", "", 406}, {"text/html;charset=UTF-8;q=0, */*;q=1", "json", 200}} {
			r := httptest.NewRequest(method, "/features", nil)
			r.Header.Set("Accept", tc.accept)
			w := httptest.NewRecorder()
			api.protocolHandler(api.negotiate(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if featureSelectedFormat(r) != tc.format {
					t.Fatal(featureSelectedFormat(r))
				}
				api.writeJSON(w, r, 200, "application/json", map[string]string{"value": "literal"})
			}), "application/json")).ServeHTTP(w, r)
			if w.Code != tc.status {
				t.Fatalf("%s %s status%d", method, tc.accept, w.Code)
			}
			if method == "HEAD" && w.Body.Len() != 0 {
				t.Fatal("HEAD body")
			}
		}
	}
}
