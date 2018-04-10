package server_test

import (
	"net/http"
	"reflect"
	"testing"

	"github.com/go-spatial/tegola/server"
)

func TestCORSHandler(t *testing.T) {
	type tcase struct {
		CORSAllowedOrigin string
		uri               string
		expected          http.Header
	}
	fn := func(t *testing.T, tc tcase) {

		var err error

		server.CORSAllowedOrigin = tc.CORSAllowedOrigin

		w, _, err := doRequest(nil, "OPTIONS", tc.uri, nil)
		if err != nil {
			t.Fatal(err)
		}

		if w.Code != http.StatusOK {
			t.Errorf("status code, expected %v got %v", http.StatusOK, w.Code)
		}

		if !reflect.DeepEqual(tc.expected, w.Header()) {
			t.Errorf("response headers,\n  expected %+v\n  got %+v", tc.expected, w.Header())
		}
	}
	tests := map[string]tcase{

		"1": {
			CORSAllowedOrigin: "*",
			uri:               "http://localhost:8080/capabilities",
			expected: http.Header{
				"Access-Control-Allow-Origin":  []string{"*"},
				"Access-Control-Allow-Methods": []string{"GET, OPTIONS"},
			},
		},
		"2": {
			CORSAllowedOrigin: "tegola.io",
			uri:               "http://localhost:8080/capabilities",
			expected: http.Header{
				"Access-Control-Allow-Origin":  []string{"tegola.io"},
				"Access-Control-Allow-Methods": []string{"GET, OPTIONS"},
			},
		},
	}
	for name, tc := range tests {
		tc := tc
		t.Run(name, func(t *testing.T) { fn(t, tc) })
	}
}
