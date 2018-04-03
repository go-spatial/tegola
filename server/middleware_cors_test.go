package server_test

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/dimfeld/httptreemux"
	"github.com/go-spatial/tegola/server"
)

func TestCORSHandler(t *testing.T) {
	testcases := []struct {
		handler           http.Handler
		CORSAllowedOrigin string
		uri               string
		uriPattern        string
		reqMethod         string
		expected          http.Header
	}{
		{
			handler:           server.HandleCapabilities{},
			CORSAllowedOrigin: "*",
			uri:               "http://localhost:8080/capabilities",
			uriPattern:        "/capabilities",
			reqMethod:         "OPTIONS",
			expected: http.Header{
				"Access-Control-Allow-Origin":  []string{"*"},
				"Access-Control-Allow-Methods": []string{"GET, OPTIONS"},
			},
		},
		{
			handler:           server.HandleCapabilities{},
			CORSAllowedOrigin: "tegola.io",
			uri:               "http://localhost:8080/capabilities",
			uriPattern:        "/capabilities",
			reqMethod:         "OPTIONS",
			expected: http.Header{
				"Access-Control-Allow-Origin":  []string{"tegola.io"},
				"Access-Control-Allow-Methods": []string{"GET, OPTIONS"},
			},
		},
	}

	for i, tc := range testcases {
		var err error

		server.CORSAllowedOrigin = tc.CORSAllowedOrigin

		// setup a new router. this handles parsing our URL wildcards (i.e. :map_name, :z, :x, :y)
		router := httptreemux.New()

		// setup a new router group
		group := router.NewGroup("/")
		group.UsingContext().Handler(tc.reqMethod, tc.uriPattern, server.CORSHandler(server.HandleCapabilities{}))

		r, err := http.NewRequest(tc.reqMethod, tc.uri, nil)
		if err != nil {
			t.Fatal(err)
		}

		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)

		if w.Code != http.StatusOK {
			t.Errorf("testcase (%v) failed. handler returned wrong status code: got (%v) expected (%v)", i, w.Code, http.StatusOK)
		}

		if !reflect.DeepEqual(tc.expected, w.Header()) {
			t.Errorf("testcase (%v) failed. response headers and expected do not match \n%+v\n%+v", i, tc.expected, w.Header())
		}
	}
}
