package server_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/dimfeld/httptreemux"
	"github.com/go-spatial/tegola/server"
)

func TestMiddlewareTileCacheHandler(t *testing.T) {
	//	setup a new provider
	testcases := []struct {
		uri          string
		uriPattern   string
		reqMethod    string
		reqHandler   http.Handler
		expectedCode int
	}{
		{
			uri:        "/maps/test-map/10/2/3.pbf",
			uriPattern: "/maps/:map_name/:z/:x/:y",
			reqHandler: server.HandleMapZXY{},
		},
		{
			uri:        "/maps/test-map/test-layer/4/2/3.pbf",
			uriPattern: "/maps/:map_name/:layer_name/:z/:x/:y",
			reqHandler: server.HandleMapLayerZXY{},
		},
	}

	for i, test := range testcases {
		var err error

		//	setup a new router. this handles parsing our URL wildcards (i.e. :map_name, :z, :x, :y)
		router := httptreemux.New()
		//	setup a new router group
		group := router.NewGroup("/")
		group.UsingContext().Handler("GET", test.uriPattern, server.TileCacheHandler(test.reqHandler))

		r, err := http.NewRequest("GET", test.uri, nil)
		if err != nil {
			t.Errorf("[%v] error, expected nil got %v", i, err)
			continue
		}

		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)

		//	first response we expect the cache to MISS
		if w.Header().Get("Tegola-Cache") != "MISS" {
			t.Errorf("[%v] header Tegola-Cache, expected MISS got %v", i, w.Header().Get("Tegola-Cache"))
			continue
		}

		//	play the request again to get a HIT
		r, err = http.NewRequest("GET", test.uri, nil)
		if err != nil {
			t.Errorf("[%v] GET error, expected nil got %v", i, err)
			continue
		}

		w = httptest.NewRecorder()
		router.ServeHTTP(w, r)

		if w.Header().Get("Tegola-Cache") != "HIT" {
			t.Errorf("[%v] Tegoal-Cache, expected HIT got %v", i, w.Header().Get("Tegola-Cache"))
			continue
		}
	}
}
