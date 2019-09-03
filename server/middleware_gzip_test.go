package server_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-spatial/tegola/cache/memory"
	"github.com/go-spatial/tegola/server"
)

func TestMiddlewareGzipHandler(t *testing.T) {
	type tcase struct {
		uri                     string
		requestHeaders          map[string]string
		expectedResponseHeaders map[string]string
	}

	fn := func(tc tcase) func(t *testing.T) {
		return func(t *testing.T) {
			t.Parallel()

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
