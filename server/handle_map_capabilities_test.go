package server_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"testing"

	"github.com/go-spatial/geom"
	"github.com/go-spatial/tegola/atlas"
	"github.com/go-spatial/tegola/mapbox/tilejson"
	"github.com/go-spatial/tegola/provider/test"
	"github.com/go-spatial/tegola/server"
)

// layerFielderProvider is a mock provider that implements LayerFielder for testing TileJSON v3.0.0
type layerFielderProvider struct {
	*test.TileProvider
	fields map[string]map[string]interface{} // layerName -> fields
}

func (p *layerFielderProvider) LayerFields(ctx context.Context, layerName string) (map[string]interface{}, error) {
	if fields, ok := p.fields[layerName]; ok {
		return fields, nil
	}
	return make(map[string]interface{}), nil
}

func TestHandleMapCapabilities(t *testing.T) {
	type tcase struct {
		handler   http.Handler
		hostName  *url.URL
		port      string
		uri       string
		reqMethod string
		expected  tilejson.TileJSON
	}

	fn := func(tc tcase) func(*testing.T) {
		return func(t *testing.T) {
			var err error

			server.HostName = tc.hostName
			server.Port = tc.port

			// setup a new router. this handles parsing our URL wildcards (i.e. :map_name, :z, :x, :y)
			router := server.NewRouter(nil)

			r, err := http.NewRequest(tc.reqMethod, tc.uri, nil)
			if err != nil {
				t.Fatal(err)
				return
			}

			w := httptest.NewRecorder()
			router.ServeHTTP(w, r)

			if w.Code != http.StatusOK {
				t.Errorf("handler returned wrong status code: got (%v) expected (%v)", w.Code, http.StatusOK)
				return
			}

			bytes, err := io.ReadAll(w.Body)
			if err != nil {
				t.Errorf("err reading response body: %v", err)
				return
			}

			var tileJSON tilejson.TileJSON
			// read the response body
			if err := json.Unmarshal(bytes, &tileJSON); err != nil {
				t.Errorf("unable to unmarshal JSON response body: %v", err)
				return
			}

			if !reflect.DeepEqual(tc.expected, tileJSON) {
				t.Errorf("response body and expected do not match \n%+v\n%+v", tc.expected, tileJSON)
				return
			}
		}
	}

	tests := map[string]tcase{
		"happy path": {
			handler:   server.HandleCapabilities{},
			hostName:  nil,
			uri:       "http://localhost:8080/capabilities/test-map.json",
			reqMethod: http.MethodGet,
			expected: tilejson.TileJSON{
				Attribution: &testMapAttribution,
				Bounds:      [4]float64{-180.0, -85.0511, 180.0, 85.0511},
				Center:      testMapCenter,
				Format:      server.TileURLFileFormat,
				MinZoom:     testLayer1.MinZoom,
				MaxZoom:     testLayer3.MaxZoom, //	the max zoom for the test group is in layer 3
				Name:        &testMapName,
				Description: nil,
				Scheme:      tilejson.SchemeXYZ,
				TileJSON:    tilejson.Version2,
				Tiles: []string{
					server.TileURLTemplate{
						Scheme:  "http",
						Host:    "localhost:8080",
						MapName: "test-map",
					}.String(),
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
						GeometryType: tilejson.GeomTypePoint,
						MinZoom:      testLayer1.MinZoom,
						MaxZoom:      testLayer3.MaxZoom, // layer 1 and layer 3 share a name in our test so the zoom range includes the entire zoom range
						Tiles: []string{
							server.TileURLTemplate{
								Scheme:    "http",
								Host:      "localhost:8080",
								MapName:   "test-map",
								LayerName: testLayer1.MVTName(),
							}.String(),
						},
					},
					{
						Version:      2,
						Extent:       4096,
						ID:           testLayer2.MVTName(),
						Name:         testLayer2.MVTName(),
						GeometryType: tilejson.GeomTypeLine,
						MinZoom:      testLayer2.MinZoom,
						MaxZoom:      testLayer2.MaxZoom,
						Tiles: []string{
							server.TileURLTemplate{
								Scheme:    "http",
								Host:      "localhost:8080",
								MapName:   "test-map",
								LayerName: testLayer2.MVTName(),
							}.String(),
						},
					},
				},
			},
		},
		"with hostname": {
			handler: server.HandleCapabilities{},
			hostName: &url.URL{
				Host: "cdn.tegola.io",
			},
			port:      "none",
			uri:       "http://localhost:8080/capabilities/test-map.json?debug=true",
			reqMethod: http.MethodGet,
			expected: tilejson.TileJSON{
				Attribution: &testMapAttribution,
				Bounds:      [4]float64{-180.0, -85.0511, 180.0, 85.0511},
				Center:      testMapCenter,
				Format:      server.TileURLFileFormat,
				MinZoom:     0,
				MaxZoom:     atlas.MaxZoom,
				Name:        &testMapName,
				Description: nil,
				Scheme:      tilejson.SchemeXYZ,
				TileJSON:    tilejson.Version2,
				Tiles: []string{
					server.TileURLTemplate{
						Scheme:  "http",
						Host:    "cdn.tegola.io",
						MapName: "test-map",
						Query: url.Values{
							server.QueryKeyDebug: []string{
								"true",
							},
						},
					}.String(),
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
						GeometryType: tilejson.GeomTypePoint,
						MinZoom:      testLayer1.MinZoom,
						MaxZoom:      testLayer3.MaxZoom, // layer 1 and layer 3 share a name in our test so the zoom range includes the entire zoom range
						Tiles: []string{
							server.TileURLTemplate{
								Scheme:    "http",
								Host:      "cdn.tegola.io",
								MapName:   "test-map",
								LayerName: testLayer1.MVTName(),
								Query: url.Values{
									server.QueryKeyDebug: []string{
										"true",
									},
								},
							}.String(),
						},
					},
					{
						Version:      2,
						Extent:       4096,
						ID:           testLayer2.MVTName(),
						Name:         testLayer2.MVTName(),
						GeometryType: tilejson.GeomTypeLine,
						MinZoom:      testLayer2.MinZoom,
						MaxZoom:      testLayer2.MaxZoom,
						Tiles: []string{
							server.TileURLTemplate{
								Scheme:    "http",
								Host:      "cdn.tegola.io",
								MapName:   "test-map",
								LayerName: testLayer2.MVTName(),
								Query: url.Values{
									server.QueryKeyDebug: []string{
										"true",
									},
								},
							}.String(),
						},
					},
					{
						Version:      2,
						Extent:       4096,
						ID:           "debug-tile-outline",
						Name:         "debug-tile-outline",
						GeometryType: tilejson.GeomTypeLine,
						MinZoom:      0,
						MaxZoom:      atlas.MaxZoom,
						Tiles: []string{
							server.TileURLTemplate{
								Scheme:    "http",
								Host:      "cdn.tegola.io",
								MapName:   "test-map",
								LayerName: "debug-tile-outline",
								Query: url.Values{
									server.QueryKeyDebug: []string{
										"true",
									},
								},
							}.String(),
						},
					},
					{
						Version:      2,
						Extent:       4096,
						ID:           "debug-tile-center",
						Name:         "debug-tile-center",
						GeometryType: tilejson.GeomTypePoint,
						MinZoom:      0,
						MaxZoom:      atlas.MaxZoom,
						Tiles: []string{
							server.TileURLTemplate{
								Scheme:    "http",
								Host:      "cdn.tegola.io",
								MapName:   "test-map",
								LayerName: "debug-tile-center",
								Query: url.Values{
									server.QueryKeyDebug: []string{
										"true",
									},
								},
							}.String(),
						},
					},
				},
			},
		},
		// https://github.com/go-spatial/tegola/issues/994
		"hostname with scheme": {
			handler: server.HandleCapabilities{},
			hostName: &url.URL{
				// The scheme is determined at request time. if the
				// user sets the scheme on the hostname, it will be
				// ignored
				Scheme: "https",
				Host:   "cdn.tegola.io",
			},
			port:      "none",
			uri:       "http://localhost:8080/capabilities/test-map.json?debug=true",
			reqMethod: http.MethodGet,
			expected: tilejson.TileJSON{
				Attribution: &testMapAttribution,
				Bounds:      [4]float64{-180.0, -85.0511, 180.0, 85.0511},
				Center:      testMapCenter,
				Format:      server.TileURLFileFormat,
				MinZoom:     0,
				MaxZoom:     atlas.MaxZoom,
				Name:        &testMapName,
				Description: nil,
				Scheme:      tilejson.SchemeXYZ,
				TileJSON:    tilejson.Version2,
				Tiles: []string{
					server.TileURLTemplate{
						Scheme:  "http",
						Host:    "cdn.tegola.io",
						MapName: "test-map",
						Query: url.Values{
							server.QueryKeyDebug: []string{
								"true",
							},
						},
					}.String(),
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
						GeometryType: tilejson.GeomTypePoint,
						MinZoom:      testLayer1.MinZoom,
						MaxZoom:      testLayer3.MaxZoom, // layer 1 and layer 3 share a name in our test so the zoom range includes the entire zoom range
						Tiles: []string{
							server.TileURLTemplate{
								Scheme:    "http",
								Host:      "cdn.tegola.io",
								MapName:   "test-map",
								LayerName: testLayer1.MVTName(),
								Query: url.Values{
									server.QueryKeyDebug: []string{
										"true",
									},
								},
							}.String(),
						},
					},
					{
						Version:      2,
						Extent:       4096,
						ID:           testLayer2.MVTName(),
						Name:         testLayer2.MVTName(),
						GeometryType: tilejson.GeomTypeLine,
						MinZoom:      testLayer2.MinZoom,
						MaxZoom:      testLayer2.MaxZoom,
						Tiles: []string{
							server.TileURLTemplate{
								Scheme:    "http",
								Host:      "cdn.tegola.io",
								MapName:   "test-map",
								LayerName: testLayer2.MVTName(),
								Query: url.Values{
									server.QueryKeyDebug: []string{
										"true",
									},
								},
							}.String(),
						},
					},
					{
						Version:      2,
						Extent:       4096,
						ID:           "debug-tile-outline",
						Name:         "debug-tile-outline",
						GeometryType: tilejson.GeomTypeLine,
						MinZoom:      0,
						MaxZoom:      atlas.MaxZoom,
						Tiles: []string{
							server.TileURLTemplate{
								Scheme:    "http",
								Host:      "cdn.tegola.io",
								MapName:   "test-map",
								LayerName: "debug-tile-outline",
								Query: url.Values{
									server.QueryKeyDebug: []string{
										"true",
									},
								},
							}.String(),
						},
					},
					{
						Version:      2,
						Extent:       4096,
						ID:           "debug-tile-center",
						Name:         "debug-tile-center",
						GeometryType: tilejson.GeomTypePoint,
						MinZoom:      0,
						MaxZoom:      atlas.MaxZoom,
						Tiles: []string{
							server.TileURLTemplate{
								Scheme:    "http",
								Host:      "cdn.tegola.io",
								MapName:   "test-map",
								LayerName: "debug-tile-center",
								Query: url.Values{
									server.QueryKeyDebug: []string{
										"true",
									},
								},
							}.String(),
						},
					},
				},
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, fn(tc))
	}
}

func TestHandleMapCapabilitiesCORS(t *testing.T) {
	tests := map[string]CORSTestCase{
		"1": {
			uri: "/capabilities/test-map.json",
		},
	}

	for name, tc := range tests {
		t.Run(name, CORSTest(tc))
	}
}

func TestHandleMapCapabilitiesTileJSONVersion(t *testing.T) {
	// Test v2.0.0: provider without LayerFielder (existing test-map)
	t.Run("TileJSON v2.0.0 without LayerFielder", func(t *testing.T) {
		server.HostName = nil
		server.Port = ""

		router := server.NewRouter(nil)
		r, err := http.NewRequest(http.MethodGet, "http://localhost:8080/capabilities/test-map.json", nil)
		if err != nil {
			t.Fatal(err)
		}

		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)

		if w.Code != http.StatusOK {
			t.Errorf("handler returned wrong status code: got (%v) expected (%v)", w.Code, http.StatusOK)
			return
		}

		bytes, err := io.ReadAll(w.Body)
		if err != nil {
			t.Errorf("err reading response body: %v", err)
			return
		}

		var tileJSON tilejson.TileJSON
		if err := json.Unmarshal(bytes, &tileJSON); err != nil {
			t.Errorf("unable to unmarshal JSON response body: %v", err)
			return
		}

		// Should be v2.0.0 since test.TileProvider doesn't implement LayerFielder
		if tileJSON.TileJSON != tilejson.Version2 {
			t.Errorf("TileJSON version mismatch: got (%v) expected (%v)", tileJSON.TileJSON, tilejson.Version2)
			return
		}

		// v2.0.0 should not have Fields (or empty Fields should be omitted)
		for _, layer := range tileJSON.VectorLayers {
			if len(layer.Fields) > 0 {
				t.Errorf("TileJSON v2.0.0 layer %v should not have Fields, got %v", layer.ID, layer.Fields)
			}
		}
	})

	// Test v3.0.0: provider with LayerFielder
	t.Run("TileJSON v3.0.0 with LayerFielder", func(t *testing.T) {
		// Create a new map with LayerFielder provider
		testMapV3Name := "test-map-v3"
		layerFielderLayer1 := atlas.Layer{
			Name:              "test-layer",
			ProviderLayerName: "test-layer-1",
			MinZoom:           4,
			MaxZoom:           9,
			Provider: &layerFielderProvider{
				TileProvider: &test.TileProvider{},
				fields: map[string]map[string]interface{}{
					"test-layer-1": {
						"name":      "String",
						"age":       "Number",
						"is_active": "Boolean",
					},
				},
			},
			GeomType: geom.Point{},
		}

		layerFielderLayer2 := atlas.Layer{
			Name:              "test-layer-2-name",
			ProviderLayerName: "test-layer-2-provider-layer-name",
			MinZoom:           10,
			MaxZoom:           15,
			Provider: &layerFielderProvider{
				TileProvider: &test.TileProvider{},
				fields: map[string]map[string]interface{}{
					"test-layer-2-provider-layer-name": {
						"description": "String",
						"count":       "Number",
					},
				},
			},
			GeomType: geom.Line{},
		}

		testMapV3 := atlas.NewWebMercatorMap(testMapV3Name)
		testMapV3.Attribution = testMapAttribution
		testMapV3.Center = testMapCenter
		testMapV3.Layers = append(testMapV3.Layers, layerFielderLayer1, layerFielderLayer2)

		a := &atlas.Atlas{}
		a.AddMap(testMapV3)

		server.HostName = nil
		server.Port = ""

		router := server.NewRouter(a)
		r, err := http.NewRequest(http.MethodGet, "http://localhost:8080/capabilities/test-map-v3.json", nil)
		if err != nil {
			t.Fatal(err)
		}

		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)

		if w.Code != http.StatusOK {
			t.Errorf("handler returned wrong status code: got (%v) expected (%v)", w.Code, http.StatusOK)
			return
		}

		bytes, err := io.ReadAll(w.Body)
		if err != nil {
			t.Errorf("err reading response body: %v", err)
			return
		}

		var tileJSON tilejson.TileJSON
		if err := json.Unmarshal(bytes, &tileJSON); err != nil {
			t.Errorf("unable to unmarshal JSON response body: %v", err)
			return
		}

		// Should be v3.0.0 since provider implements LayerFielder
		if tileJSON.TileJSON != tilejson.Version {
			t.Errorf("TileJSON version mismatch: got (%v) expected (%v)", tileJSON.TileJSON, tilejson.Version)
			return
		}

		// v3.0.0 should have Fields
		if len(tileJSON.VectorLayers) == 0 {
			t.Errorf("expected VectorLayers, got none")
			return
		}

		expectedFields1 := map[string]interface{}{
			"name":      "String",
			"age":       "Number",
			"is_active": "Boolean",
		}

		expectedFields2 := map[string]interface{}{
			"description": "String",
			"count":       "Number",
		}

		// Check Fields for first layer
		if !reflect.DeepEqual(tileJSON.VectorLayers[0].Fields, expectedFields1) {
			t.Errorf("Fields mismatch for layer %v: got %v expected %v", tileJSON.VectorLayers[0].ID, tileJSON.VectorLayers[0].Fields, expectedFields1)
		}

		// Check Fields for second layer
		if len(tileJSON.VectorLayers) > 1 {
			if !reflect.DeepEqual(tileJSON.VectorLayers[1].Fields, expectedFields2) {
				t.Errorf("Fields mismatch for layer %v: got %v expected %v", tileJSON.VectorLayers[1].ID, tileJSON.VectorLayers[1].Fields, expectedFields2)
			}
		}
	})
}
