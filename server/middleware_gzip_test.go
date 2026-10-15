package server_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/alexeydott/tegola/cache/memory"
	"github.com/alexeydott/tegola/server"
)

func TestMiddlewareGzipHandler(t *testing.T) {
	type tcase struct {
		uri                     string
		requestHeaders          map[string]string
		expectedResponseHeaders map[string]string
	}

	fn := func(tc tcase) func(t *testing.T) {
		return func(t *testing.T) {
			// our tests don't use the URIPrefix but our server is a singleton
			// so we set it to the default for this test
			server.URIPrefix = "/"

			var err error

			// setup a new atlas
			a := newTestMapWithLayers(testLayer1, testLayer2, testLayer3)
			cacher, _ := memory.New(nil)
			a.SetCache(cacher)

			// setup a new router
			router := server.NewRouter(a)

			// setup a new request
			r, err := http.NewRequest("GET", tc.uri, nil)
			if err != nil {
				t.Errorf("unexecpted err: %v", err)
				return
			}

			// add test case request headers
			for k, v := range tc.requestHeaders {
				r.Header.Add(k, v)
			}

			// new recorder to capture the response
			w := httptest.NewRecorder()

			// issue the request
			router.ServeHTTP(w, r)

			// check our response for the correct headers
			for k, v := range tc.expectedResponseHeaders {
				h := w.Header().Get(k)
				if h != v {
					t.Errorf("expected header (%v) to have value (%v) got (%v)", k, v, h)
					return
				}
			}

			// handle no requestHeader
			if len(tc.requestHeaders) == 0 {
				encoding := w.Header().Get("Content-Encoding")
				if encoding != "" {
					t.Errorf("expected Content-Encoding to not be set, got (%v)", encoding)
					return
				}
			}
		}
	}

	tests := map[string]tcase{
		"Accept-Encoding: gzip": {
			uri: "/maps/test-map/10/2/3.pbf",
			requestHeaders: map[string]string{
				"Accept-Encoding": "gzip",
			},
			expectedResponseHeaders: map[string]string{
				"Content-Encoding": "gzip",
			},
		},
		"Accept-Encoding: foo, gzip": {
			uri: "/maps/test-map/10/2/3.pbf",
			requestHeaders: map[string]string{
				"Accept-Encoding": "foo, gzip",
			},
			expectedResponseHeaders: map[string]string{
				"Content-Encoding": "gzip",
			},
		},
		"Accept-Encoding: gzip;q=0": {
			uri: "/maps/test-map/10/2/3.pbf",
			requestHeaders: map[string]string{
				"Accept-Encoding": "gzip;q=0",
			},
			expectedResponseHeaders: map[string]string{},
		},
		"Accept-Encoding: *": {
			uri: "/maps/test-map/10/2/3.pbf",
			requestHeaders: map[string]string{
				"Accept-Encoding": "*",
			},
			expectedResponseHeaders: map[string]string{
				"Content-Encoding": "gzip",
			},
		},
		"Accept-Encoding: foo, *": {
			uri: "/maps/test-map/10/2/3.pbf",
			requestHeaders: map[string]string{
				"Accept-Encoding": "foo, *",
			},
			expectedResponseHeaders: map[string]string{
				"Content-Encoding": "gzip",
			},
		},
		"Accept-Encoding: *;q=0": {
			uri: "/maps/test-map/10/2/3.pbf",
			requestHeaders: map[string]string{
				"Accept-Encoding": "*;q=0",
			},
			expectedResponseHeaders: map[string]string{},
		},
		"Accept-Encoding missing": {
			uri:                     "/maps/test-map/10/2/3.pbf",
			requestHeaders:          map[string]string{},
			expectedResponseHeaders: map[string]string{},
		},
	}

	for name, tc := range tests {
		t.Run(name, fn(tc))
	}
}

func TestMiddlewareGzipHandlerTileStatusRemainsCompatible(t *testing.T) {
	handler := server.GZipHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"cached":true}`))
	}))

	request := httptest.NewRequest(
		http.MethodGet,
		"/maps/test-map/water/12/1/2?tile=status",
		nil,
	)
	request.Header.Set("Accept-Encoding", "gzip")
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if got := recorder.Header().Get("Content-Encoding"); got != "gzip" {
		t.Fatalf("expected gzip Content-Encoding, got %q", got)
	}
}

func TestMiddlewareGzipHandlerDoesNotEncodeErrorsOrEmptyResponses(t *testing.T) {
	tests := []struct {
		name       string
		handler    http.Handler
		statusCode int
		body       string
	}{
		{
			name: "bad request",
			handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				http.Error(w, "bad request", http.StatusBadRequest)
			}),
			statusCode: http.StatusBadRequest,
			body:       "bad request\n",
		},
		{
			name: "no content",
			handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Length", "0")
				w.WriteHeader(http.StatusNoContent)
			}),
			statusCode: http.StatusNoContent,
		},
		{
			name: "reset content",
			handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Length", "0")
				w.WriteHeader(http.StatusResetContent)
			}),
			statusCode: http.StatusResetContent,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, "/test", nil)
			request.Header.Set("Accept-Encoding", "gzip")
			recorder := httptest.NewRecorder()

			server.GZipHandler(tc.handler).ServeHTTP(recorder, request)

			if recorder.Code != tc.statusCode {
				t.Fatalf("expected status %d, got %d", tc.statusCode, recorder.Code)
			}
			if got := recorder.Header().Get("Content-Encoding"); got != "" {
				t.Fatalf("expected no Content-Encoding, got %q", got)
			}
			if got := recorder.Body.String(); got != tc.body {
				t.Fatalf("expected body %q, got %q", tc.body, got)
			}
			if (tc.statusCode == http.StatusNoContent || tc.statusCode == http.StatusResetContent) &&
				recorder.Header().Get("Content-Length") != "" {
				t.Fatalf("expected no Content-Length for status %d, got %q",
					tc.statusCode, recorder.Header().Get("Content-Length"))
			}
		})
	}
}
