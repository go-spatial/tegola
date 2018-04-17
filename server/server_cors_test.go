package server_test

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/go-spatial/tegola/server"
)

type CORSTestCase struct {
	hostname string
	port     string
	uri      string
}

func CORSTest(t *testing.T, tc CORSTestCase) {
	var err error

	server.HostName = tc.hostname
	server.Port = tc.port

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

	if !reflect.DeepEqual(headers["Access-Control-Allow-Origin"], []string{server.CORSAllowedOrigin}) {
		t.Errorf("wrong header for Access-Control-Allow-Origin. expected %v got %v", server.CORSAllowedOrigin, headers["Access-Control-Allow-Origin"])
		return
	}

	if !reflect.DeepEqual(headers["Access-Control-Allow-Methods"], []string{"GET, OPTIONS"}) {
		t.Errorf("wrong header for Access-Control-Allow-Methods. expected %v got %v", []string{"GET", "OPTIONS"}, headers["Access-Control-Allow-Methods"])
		return
	}
}
