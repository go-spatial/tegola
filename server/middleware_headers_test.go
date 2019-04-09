package server_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-spatial/tegola/cache/memory"
	"github.com/go-spatial/tegola/server"
)

func TestMiddlewareHeaders(t *testing.T) {
	type tcase struct {
		uri                     string
		httpMethod              string
		customHeaders           map[string]interface{}
		expectedResponseHeaders map[string]string
	}

	fn := func(tc tcase) func(t *testing.T) {
		return func(t *testing.T) {
			var err error

			// set the custom headers in the server package
			server.Headers = tc.customHeaders

			// setup a new atlas
			a := newTestMapWithLayers(testLayer1, testLayer2, testLayer3)
			cacher, _ := memory.New(nil)
			a.SetCache(cacher)

			// setup a new router
			router := server.NewRouter(a)

			// setup a new request
			r, err := http.NewRequest(tc.httpMethod, tc.uri, nil)
			if err != nil {
				t.Errorf("unexecpted err: %v", err)
				return
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
		}
	}

	tests := map[string]tcase{
		"default headers GET": {
			uri:           "/maps/test-map/10/2/3.pbf",
			httpMethod:    http.MethodGet,
			customHeaders: map[string]interface{}{},
			expectedResponseHeaders: map[string]string{
				"Access-Control-Allow-Origin":  DefaultCORSAllowedOrigin,
				"Access-Control-Allow-Methods": DefaultCORSAllowedMethods,
			},
		},
		"user defined headers GET": {
			uri:        "/maps/test-map/10/2/3.pbf",
			httpMethod: http.MethodGet,
			customHeaders: map[string]interface{}{
				"Test-Header": "tegola",
			},
			expectedResponseHeaders: map[string]string{
				"Access-Control-Allow-Origin":  DefaultCORSAllowedOrigin,
				"Access-Control-Allow-Methods": DefaultCORSAllowedMethods,
				"Test-Header":                  "tegola",
			},
		},
		"user defined cors override GET": {
			uri:        "/maps/test-map/10/2/3.pbf",
			httpMethod: http.MethodGet,
			customHeaders: map[string]interface{}{
				"Access-Control-Allow-Origin":  "tegola.io",
				"Access-Control-Allow-Methods": "GET, POST",
			},
			expectedResponseHeaders: map[string]string{
				"Access-Control-Allow-Origin":  "tegola.io",
				"Access-Control-Allow-Methods": "GET, POST",
			},
		},
		"default headers OPTIONS": {
			uri:           "/maps/test-map/10/2/3.pbf",
			httpMethod:    http.MethodOptions,
			customHeaders: map[string]interface{}{},
			expectedResponseHeaders: map[string]string{
				"Access-Control-Allow-Origin":  DefaultCORSAllowedOrigin,
				"Access-Control-Allow-Methods": DefaultCORSAllowedMethods,
			},
		},
		"user defined headers OPTIONS": {
			uri:        "/maps/test-map/10/2/3.pbf",
			httpMethod: http.MethodOptions,
			customHeaders: map[string]interface{}{
				"Test-Header": "tegola",
			},
			expectedResponseHeaders: map[string]string{
				"Access-Control-Allow-Origin":  DefaultCORSAllowedOrigin,
				"Access-Control-Allow-Methods": DefaultCORSAllowedMethods,
				"Test-Header":                  "tegola",
			},
		},
		"user defined cors override OPTIONS": {
			uri:        "/maps/test-map/10/2/3.pbf",
			httpMethod: http.MethodOptions,
			customHeaders: map[string]interface{}{
				"Access-Control-Allow-Origin":  "tegola.io",
				"Access-Control-Allow-Methods": "GET, POST",
			},
			expectedResponseHeaders: map[string]string{
				"Access-Control-Allow-Origin":  "tegola.io",
				"Access-Control-Allow-Methods": "GET, POST",
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, fn(tc))
	}
}
