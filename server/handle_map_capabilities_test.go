package server_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/dimfeld/httptreemux"

	"github.com/terranodo/tegola/server"
	"github.com/terranodo/tegola/tilejson"
)

func TestHandleMapCapabilities(t *testing.T) {
	//	setup a new provider
	testcases := []struct {
		handler    http.Handler
		hostName   string
		uri        string
		uriPattern string
		reqMethod  string
		expected   tilejson.TileJSON
	}{
		{
			handler:    server.HandleCapabilities{},
			hostName:   "",
			uri:        "http://localhost:8080/capabilities/test-map.json",
			uriPattern: "/capabilities/:map_name",
			reqMethod:  "GET",
			expected: tilejson.TileJSON{
				Attribution: &testMap.Attribution,
				Bounds:      [4]float64{0, 0, 0, 0},
				Center:      testMap.Center,
				Format:      "pbf",
				MinZoom:     4,
				MaxZoom:     20,
				Name:        &testMap.Name,
				Description: nil,
				Scheme:      tilejson.SchemeXYZ,
				TileJSON:    tilejson.Version,
				Tiles: []string{
					"http://localhost:8080/maps/test-map/{z}/{x}/{y}.pbf",
				},
				Grids:    []string{},
				Data:     []string{},
				Version:  "1.0.0",
				Template: nil,
				Legend:   nil,
				VectorLayers: []tilejson.VectorLayer{
					{
						Version: 2,
						Extent:  4096,
						ID:      testLayer1.Name,
						Name:    testLayer1.Name,
						//FeatureTags: nil,
						GeometryType: tilejson.GeomTypePoint,
						MinZoom:      testLayer1.MinZoom,
						MaxZoom:      testLayer1.MaxZoom,
						Tiles: []string{
							"http://localhost:8080/maps/test-map/test-layer-1/{z}/{x}/{y}.pbf",
						},
					},
					{
						Version: 2,
						Extent:  4096,
						ID:      testLayer2.Name,
						Name:    testLayer2.Name,
						//FeatureTags: nil,
						GeometryType: tilejson.GeomTypeLine,
						MinZoom:      testLayer2.MinZoom,
						MaxZoom:      testLayer2.MaxZoom,
						Tiles: []string{
							"http://localhost:8080/maps/test-map/test-layer-2/{z}/{x}/{y}.pbf",
						},
					},
				},
			},
		},
		{
			handler:    server.HandleCapabilities{},
			hostName:   "cdn.tegola.io",
			uri:        "http://localhost:8080/capabilities/test-map.json?debug=true",
			uriPattern: "/capabilities/:map_name",
			reqMethod:  "GET",
			expected: tilejson.TileJSON{
				Attribution: &testMap.Attribution,
				Bounds:      [4]float64{0, 0, 0, 0},
				Center:      testMap.Center,
				Format:      "pbf",
				MinZoom:     4,
				MaxZoom:     20,
				Name:        &testMap.Name,
				Description: nil,
				Scheme:      tilejson.SchemeXYZ,
				TileJSON:    tilejson.Version,
				Tiles: []string{
					"http://cdn.tegola.io/maps/test-map/{z}/{x}/{y}.pbf?debug=true",
				},
				Grids:    []string{},
				Data:     []string{},
				Version:  "1.0.0",
				Template: nil,
				Legend:   nil,
				VectorLayers: []tilejson.VectorLayer{
					{
						Version:      2,
						Extent:       4096,
						ID:           testLayer1.Name,
						Name:         testLayer1.Name,
						GeometryType: tilejson.GeomTypePoint,
						MinZoom:      testLayer1.MinZoom,
						MaxZoom:      testLayer1.MaxZoom,
						Tiles: []string{
							"http://cdn.tegola.io/maps/test-map/test-layer-1/{z}/{x}/{y}.pbf?debug=true",
						},
					},
					{
						Version:      2,
						Extent:       4096,
						ID:           testLayer2.Name,
						Name:         testLayer2.Name,
						GeometryType: tilejson.GeomTypeLine,
						MinZoom:      testLayer2.MinZoom,
						MaxZoom:      testLayer2.MaxZoom,
						Tiles: []string{
							"http://cdn.tegola.io/maps/test-map/test-layer-2/{z}/{x}/{y}.pbf?debug=true",
						},
					},
					{
						Version:      2,
						Extent:       4096,
						ID:           "debug-tile-outline",
						Name:         "debug-tile-outline",
						GeometryType: tilejson.GeomTypeLine,
						MinZoom:      0,
						MaxZoom:      server.MaxZoom,
						Tiles: []string{
							"http://cdn.tegola.io/maps/test-map/debug-tile-outline/{z}/{x}/{y}.pbf?debug=true",
						},
					},
					{
						Version:      2,
						Extent:       4096,
						ID:           "debug-tile-center",
						Name:         "debug-tile-center",
						GeometryType: tilejson.GeomTypePoint,
						MinZoom:      0,
						MaxZoom:      server.MaxZoom,
						Tiles: []string{
							"http://cdn.tegola.io/maps/test-map/debug-tile-center/{z}/{x}/{y}.pbf?debug=true",
						},
					},
				},
			},
		},
	}

	for i, test := range testcases {
		var err error

		server.HostName = test.hostName

		//	setup a new router. this handles parsing our URL wildcards (i.e. :map_name, :z, :x, :y)
		router := httptreemux.New()
		//	setup a new router group
		group := router.NewGroup("/")
		group.UsingContext().Handler(test.reqMethod, test.uriPattern, server.HandleMapCapabilities{})

		r, err := http.NewRequest(test.reqMethod, test.uri, nil)
		if err != nil {
			t.Fatal(err)
		}

		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)

		if w.Code != http.StatusOK {
			t.Errorf("Failed test %v. handler returned wrong status code: got (%v) expected (%v)", i, w.Code, http.StatusOK)
		}

		var tileJSON tilejson.TileJSON
		//	read the respons body
		if err := json.NewDecoder(w.Body).Decode(&tileJSON); err != nil {
			t.Errorf("Failed test %v. Unable to decode JSON response body", i)
		}

		if !reflect.DeepEqual(test.expected, tileJSON) {
			t.Errorf("Failed test %v. Response body and expected do not match \n%+v\n%+v", i, test.expected, tileJSON)
		}
	}
}
