package server_test

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/go-spatial/tegola/atlas"
	"github.com/go-spatial/tegola/mapbox/tilejson"
	"github.com/go-spatial/tegola/server"
)

func TestHandleMapCapabilities(t *testing.T) {
	// setup a new provider
	testcases := []struct {
		handler   http.Handler
		hostName  string
		port      string
		uri       string
		reqMethod string
		expected  tilejson.TileJSON
	}{
		{
			handler:   server.HandleCapabilities{},
			hostName:  "",
			uri:       "http://localhost:8080/capabilities/test-map.json",
			reqMethod: "GET",
			expected: tilejson.TileJSON{
				Attribution: &testMapAttribution,
				Bounds:      [4]float64{-180.0, -85.0511, 180.0, 85.0511},
				Center:      testMapCenter,
				Format:      "pbf",
				MinZoom:     testLayer1.MinZoom,
				MaxZoom:     testLayer3.MaxZoom, //	the max zoom for the test group is in layer 3
				Name:        &testMapName,
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
						Version:      2,
						Extent:       4096,
						ID:           testLayer1.MVTName(),
						Name:         testLayer1.MVTName(),
						Description:  testLayer1.MVTName(),
						GeometryType: tilejson.GeomTypePoint,
						MinZoom:      testLayer1.MinZoom,
						MaxZoom:      testLayer3.MaxZoom, // layer 1 and layer 3 share a name in our test so the zoom range includes the entire zoom range
						Tiles: []string{
							fmt.Sprintf("http://localhost:8080/maps/test-map/%v/{z}/{x}/{y}.pbf", testLayer1.MVTName()),
						},
					},
					{
						Version:      2,
						Extent:       4096,
						ID:           testLayer2.MVTName(),
						Name:         testLayer2.MVTName(),
						Description:  testLayer2.MVTName(),
						GeometryType: tilejson.GeomTypeLine,
						MinZoom:      testLayer2.MinZoom,
						MaxZoom:      testLayer2.MaxZoom,
						Tiles: []string{
							fmt.Sprintf("http://localhost:8080/maps/test-map/%v/{z}/{x}/{y}.pbf", testLayer2.MVTName()),
						},
					},
				},
			},
		},
		{
			handler:   server.HandleCapabilities{},
			hostName:  "cdn.tegola.io",
			port:      "none",
			uri:       "http://localhost:8080/capabilities/test-map.json?debug=true",
			reqMethod: "GET",
			expected: tilejson.TileJSON{
				Attribution: &testMapAttribution,
				Bounds:      [4]float64{-180.0, -85.0511, 180.0, 85.0511},
				Center:      testMapCenter,
				Format:      "pbf",
				MinZoom:     0,
				MaxZoom:     atlas.MaxZoom,
				Name:        &testMapName,
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
						ID:           testLayer1.MVTName(),
						Name:         testLayer1.MVTName(),
						Description:  testLayer1.MVTName(),
						GeometryType: tilejson.GeomTypePoint,
						MinZoom:      testLayer1.MinZoom,
						MaxZoom:      testLayer3.MaxZoom, // layer 1 and layer 3 share a name in our test so the zoom range includes the entire zoom range
						Tiles: []string{
							fmt.Sprintf("http://cdn.tegola.io/maps/test-map/%v/{z}/{x}/{y}.pbf?debug=true", testLayer1.MVTName()),
						},
					},
					{
						Version:      2,
						Extent:       4096,
						ID:           testLayer2.MVTName(),
						Name:         testLayer2.MVTName(),
						Description:  testLayer2.MVTName(),
						GeometryType: tilejson.GeomTypeLine,
						MinZoom:      testLayer2.MinZoom,
						MaxZoom:      testLayer2.MaxZoom,
						Tiles: []string{
							fmt.Sprintf("http://cdn.tegola.io/maps/test-map/%v/{z}/{x}/{y}.pbf?debug=true", testLayer2.MVTName()),
						},
					},
					{
						Version:      2,
						Extent:       4096,
						ID:           "debug-tile-outline",
						Name:         "debug-tile-outline",
						Description:  "debug-tile-outline",
						GeometryType: tilejson.GeomTypeLine,
						MinZoom:      0,
						MaxZoom:      atlas.MaxZoom,
						Tiles: []string{
							"http://cdn.tegola.io/maps/test-map/debug-tile-outline/{z}/{x}/{y}.pbf?debug=true",
						},
					},
					{
						Version:      2,
						Extent:       4096,
						ID:           "debug-tile-center",
						Name:         "debug-tile-center",
						Description:  "debug-tile-center",
						GeometryType: tilejson.GeomTypePoint,
						MinZoom:      0,
						MaxZoom:      atlas.MaxZoom,
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
		server.Port = test.port

		// setup a new router. this handles parsing our URL wildcards (i.e. :map_name, :z, :x, :y)
		router := server.NewRouter(nil)

		r, err := http.NewRequest(test.reqMethod, test.uri, nil)
		if err != nil {
			t.Fatal(err)
			continue
		}

		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)

		if w.Code != http.StatusOK {
			t.Errorf("[%v] handler returned wrong status code: got (%v) expected (%v)", i, w.Code, http.StatusOK)
			continue
		}

		bytes, err := ioutil.ReadAll(w.Body)
		if err != nil {
			t.Errorf("[%v] err reading response body: %v", i, err)
			continue
		}

		var tileJSON tilejson.TileJSON
		// read the response body
		if err := json.Unmarshal(bytes, &tileJSON); err != nil {
			t.Errorf("[%v] unable to unmarshal JSON response body: %v", i, err)
			continue
		}

		if !reflect.DeepEqual(testMapName, *tileJSON.Name) {
			t.Errorf("[%v] response map name and expected do not match \n%+v\n%+v", i, testMapName, *tileJSON.Name)
			continue
		}

		if !reflect.DeepEqual(testMapAttribution, *tileJSON.Attribution) {
			t.Errorf("[%v] response map attribution and expected do not match \n%+v\n%+v", i, testMapAttribution, *tileJSON.Attribution)
			continue
		}

		if !reflect.DeepEqual(test.expected, tileJSON) {
			t.Errorf("[%v] response body and expected do not match \n%+v\n%+v", i, test.expected, tileJSON)
			continue
		}
	}
}

func TestHandleMapCapabilitiesCORS(t *testing.T) {
	tests := map[string]CORSTestCase{
		"1": {
			uri: "/capabilities/test-map.json",
		},
	}

	for name, tc := range tests {
		tc := tc
		t.Run(name, func(t *testing.T) { CORSTest(t, tc) })
	}
}
