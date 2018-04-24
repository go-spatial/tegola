package server_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-spatial/tegola/cache/memory"
)

func TestMiddlewareTileCacheHandler(t *testing.T) {

	tests := []string{
		"/maps/test-map/10/2/3.pbf",
		"/maps/test-map/test-layer/4/2/3.pbf",
	}

	for i, tc := range tests {
		tc := tc
		t.Run(fmt.Sprintf("%v", i), func(t *testing.T) {
			var err error

			a := newTestMapWithLayers(testLayer1, testLayer2, testLayer3)
			a.SetCache(memory.New())

			w, router, err := doRequest(a, "GET", tc, nil)
			if err != nil {
				t.Errorf("error making request, expected nil got %v", err)
				return
			}

			// first response we expect the cache to MISS
			if w.Header().Get("Tegola-Cache") != "MISS" {
				t.Errorf("header Tegola-Cache, expected MISS got %v", w.Header().Get("Tegola-Cache"))
				return
			}

			// play the request again to get a HIT
			r, err := http.NewRequest("GET", tc, nil)
			if err != nil {
				t.Errorf("error making request, expected nil got %v", err)
				return
			}

			w = httptest.NewRecorder()
			router.ServeHTTP(w, r)

			if w.Header().Get("Tegola-Cache") != "HIT" {
				t.Errorf("Tegoal-Cache, expected HIT got %v", w.Header().Get("Tegola-Cache"))
				return
			}
		})
	}
}
