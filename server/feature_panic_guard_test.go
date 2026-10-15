package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/dimfeld/httptreemux"
)

type featurePanicSecret struct{ inspected *int }

func (p featurePanicSecret) Error() string  { *p.inspected++; panic("Error must not run") }
func (p featurePanicSecret) String() string { *p.inspected++; panic("String must not run") }

func TestFeaturePanicTransactionNoPartialResponse(t *testing.T) {
	api := &FeatureAPI{cfg: FeatureAPIConfig{MaxResponseBytes: 1024}}
	for _, method := range []string{"GET", "HEAD"} {
		for _, phase := range []string{"before", "headers", "body", "nil", "canceled"} {
			t.Run(method+phase, func(t *testing.T) {
				inspected := 0
				request := httptest.NewRequest(method, "/features?f=html", nil)
				if phase == "canceled" {
					ctx, cancel := context.WithCancel(request.Context())
					cancel()
					request = request.WithContext(ctx)
				}
				response := httptest.NewRecorder()
				response.Header().Set("Access-Control-Allow-Origin", "https://allowed.example")
				response.Header().Set("X-Configured", "retained")
				api.serveFeatureBuffered(response, request, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					w.Header().Set("Content-Crs", "private-source")
					w.Header().Set("ETag", "stale")
					w.Header().Set("Content-Encoding", "gzip")
					w.Header().Set("Location", "https://private.example/")
					w.Header().Set("Trailer", "Secret")
					w.Header().Set("Secret", "private")
					w.Header().Set(http.TrailerPrefix+"Secret", "private")
					if phase == "headers" || phase == "body" {
						w.WriteHeader(200)
					}
					if phase == "body" {
						_, _ = w.Write([]byte("partial private success"))
					}
					if phase == "nil" {
						panic(nil)
					}
					panic(featurePanicSecret{&inspected})
				}))
				if inspected != 0 || response.Code != 500 || response.Header().Get("Content-Type") != "application/json" || response.Header().Get("Cache-Control") != "no-store" {
					t.Fatal(response.Code, response.Header(), inspected)
				}
				for _, name := range []string{"Content-Crs", "ETag", "Content-Encoding", "Location", "Trailer", "Secret", http.TrailerPrefix + "Secret"} {
					if response.Header().Get(name) != "" {
						t.Fatal("failure retained header", name)
					}
				}
				if response.Header().Get("Access-Control-Allow-Origin") != "https://allowed.example" || response.Header().Get("X-Configured") != "retained" {
					t.Fatal("configured headers lost")
				}
				const body = `{"code":"InternalError","description":"Request did not complete"}`
				if response.Header().Get("Content-Length") != strconv.Itoa(len(body)) || (method == "GET" && response.Body.String() != body) || (method == "HEAD" && response.Body.Len() != 0) {
					t.Fatal("partial response or HEAD parity", response.Body.String())
				}
			})
		}
	}
}

func TestFeaturePanicTransactionNormalHeadersAndOverflow(t *testing.T) {
	api := &FeatureAPI{cfg: FeatureAPIConfig{MaxResponseBytes: 1024}}
	response := httptest.NewRecorder()
	response.Header().Set("X-Removed", "outer")
	api.serveFeatureBuffered(response, httptest.NewRequest("GET", "/features", nil), http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Del("X-Removed")
		w.Header().Set("X-Kept", "inner")
		w.Header().Set("Content-Length", "999")
		_, _ = w.Write([]byte("complete"))
	}))
	if response.Header().Get("X-Removed") != "" || response.Header().Get("X-Kept") != "inner" || response.Header().Get("Content-Length") != "8" || response.Body.String() != "complete" {
		t.Fatal("normal transaction changed", response.Header(), response.Body.String())
	}
	response = httptest.NewRecorder()
	api.serveFeatureBuffered(response, httptest.NewRequest("GET", "/features", nil), http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(strings.Repeat("x", 1025))) }))
	if response.Code != 400 || !strings.Contains(response.Body.String(), "ResponseTooLarge") || strings.Contains(response.Body.String(), "xxx") {
		t.Fatal("capture overflow leaked output")
	}
}

func TestFeaturePanicAbortAndLegacyBoundary(t *testing.T) {
	api := &FeatureAPI{cfg: FeatureAPIConfig{MaxResponseBytes: 1024}}
	func() {
		defer func() {
			if recover() != http.ErrAbortHandler {
				t.Fatal("abort sentinel not preserved")
			}
		}()
		api.serveFeatureBuffered(httptest.NewRecorder(), httptest.NewRequest("GET", "/features", nil), http.HandlerFunc(func(http.ResponseWriter, *http.Request) { panic(http.ErrAbortHandler) }))
	}()
	mux := httptreemux.New()
	mux.GET("/tile", func(http.ResponseWriter, *http.Request, map[string]string) { panic("legacy") })
	router := &Router{TreeMux: mux, featureBasePath: "/features", featureAPI: api}
	defer func() {
		if recover() != "legacy" {
			t.Fatal("legacy panic was contained")
		}
	}()
	router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/tile", nil))
}
