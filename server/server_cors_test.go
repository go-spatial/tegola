package server_test

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"testing"

	"github.com/go-spatial/tegola/server"
)

const (
	DefaultCORSAllowedOrigin  = "*"
	DefaultCORSAllowedMethods = "GET, OPTIONS"
)

type CORSTestCase struct {
	hostname string
	port     string
	uri      string
}

func CORSTest(tc CORSTestCase) func(*testing.T) {
	return func(t *testing.T) {
		var err error

		server.HostName = &url.URL{
			Host: tc.hostname,
		}
		server.Port = tc.port
		server.URIPrefix = "/"

		// setup a new router. this handles parsing our URL wildcards (i.e. :map_name, :z, :x, :y)
		router := server.NewRouter(nil)

		r, err := http.NewRequest(http.MethodOptions, tc.uri, nil)
		if err != nil {
			t.Fatal(err)
			return
		}

		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)

		if w.Code != http.StatusOK {
			t.Errorf("wrong status code: expected %v, got %v", http.StatusOK, w.Code)
			return
		}

		headers := w.Header()

		if !reflect.DeepEqual(headers["Access-Control-Allow-Origin"], []string{DefaultCORSAllowedOrigin}) {
			t.Errorf("wrong header for Access-Control-Allow-Origin. expected %v got %v", DefaultCORSAllowedOrigin, headers["Access-Control-Allow-Origin"])
			return
		}

		if !reflect.DeepEqual(headers["Access-Control-Allow-Methods"], []string{"GET, OPTIONS"}) {
			t.Errorf("wrong header for Access-Control-Allow-Methods. expected %v got %v", []string{"GET", "OPTIONS"}, headers["Access-Control-Allow-Methods"])
			return
		}
	}
}
