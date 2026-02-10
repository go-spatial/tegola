package server_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"testing"

	"github.com/dimfeld/httptreemux"
	"github.com/go-spatial/geom"
	"github.com/go-spatial/tegola/atlas"
	"github.com/go-spatial/tegola/mapbox/tilejson"
	"github.com/go-spatial/tegola/provider/test"
	"github.com/go-spatial/tegola/server"
)

// layerFielderProvider is a mock provider that implements LayerFielder for testing TileJSON v3.0.0
type layerFielderProvider struct {
	*test.TileProvider
	fields map[string]map[string]any // layerName -> fields
}

func (p *layerFielderProvider) LayerFields(ctx context.Context, layerName string) (map[string]any, error) {
	if fields, ok := p.fields[layerName]; ok {
		return fields, nil
	}
	return make(map[string]any), nil
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

type TileJSONVersionTestCase struct {
	mapName          string
	setupMap         func() atlas.Map
	expectedVersion  string
	expectedFields   map[string]map[string]any // layer ID -> expected fields
	shouldHaveFields bool
}

func TestHandleMapCapabilitiesTileJSONVersion(t *testing.T) {
	tests := map[string]TileJSONVersionTestCase{
		"TileJSON v2.0.0 without LayerFielder": {
			mapName: "test-map-v2",
			setupMap: func() atlas.Map {
				// return default test map without LayerFielder
				testMap := atlas.NewWebMercatorMap("test-map-v2")
				testMap.Attribution = testMapAttribution
				testMap.Center = testMapCenter
				testMap.Layers = append(testMap.Layers, atlas.Layer{
					Name:              "test-layer",
					ProviderLayerName: "test-layer-provider",
					MinZoom:           4,
					MaxZoom:           9,
					Provider:          &test.TileProvider{}, // No LayerFielder
					GeomType:          geom.Point{},
				})
				return testMap
			},
			expectedVersion:  tilejson.Version2,
			shouldHaveFields: false,
		},
		"TileJSON v3.0.0 with LayerFielder": {
			mapName: "test-map-v3",
			setupMap: func() atlas.Map {
				layerFielderLayer1 := atlas.Layer{
					Name:              "test-layer",
					ProviderLayerName: "test-layer-1",
					MinZoom:           4,
					MaxZoom:           9,
					Provider: &layerFielderProvider{
						TileProvider: &test.TileProvider{},
						fields: map[string]map[string]any{
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
						fields: map[string]map[string]any{
							"test-layer-2-provider-layer-name": {
								"description": "String",
								"count":       "Number",
							},
						},
					},
					GeomType: geom.Line{},
				}

				testMapV3 := atlas.NewWebMercatorMap("test-map-v3")
				testMapV3.Attribution = testMapAttribution
				testMapV3.Center = testMapCenter
				testMapV3.Layers = append(testMapV3.Layers, layerFielderLayer1, layerFielderLayer2)
				return testMapV3
			},
			expectedVersion:  tilejson.Version3,
			shouldHaveFields: true,
			expectedFields: map[string]map[string]any{
				"test-layer": {
					"name":      "String",
					"age":       "Number",
					"is_active": "Boolean",
				},
				"test-layer-2-name": {
					"description": "String",
					"count":       "Number",
				},
			},
		},
		"TileJSON v2.0.0 with mixed providers (one without LayerFielder)": {
			mapName: "test-map-mixed",
			setupMap: func() atlas.Map {
				// one layer with LayerFielder support
				layerWithFielder := atlas.Layer{
					Name:              "layer-with-fielder",
					ProviderLayerName: "layer-with-fielder-provider",
					MinZoom:           4,
					MaxZoom:           9,
					Provider: &layerFielderProvider{
						TileProvider: &test.TileProvider{},
						fields: map[string]map[string]any{
							"layer-with-fielder-provider": {
								"name": "String",
								"type": "String",
							},
						},
					},
					GeomType: geom.Point{},
				}

				// and another layer without LayerFielder support
				layerWithoutFielder := atlas.Layer{
					Name:              "layer-without-fielder",
					ProviderLayerName: "layer-without-fielder-provider",
					MinZoom:           10,
					MaxZoom:           15,
					Provider:          &test.TileProvider{}, // No LayerFielder
					GeomType:          geom.Polygon{},
				}

				testMapMixed := atlas.NewWebMercatorMap("test-map-mixed")
				testMapMixed.Attribution = testMapAttribution
				testMapMixed.Center = testMapCenter
				testMapMixed.Layers = append(testMapMixed.Layers, layerWithFielder, layerWithoutFielder)
				return testMapMixed
			},
			expectedVersion:  tilejson.Version2,
			shouldHaveFields: false, // we want this to fall back to v2.0.0, so no fields should be populated
		},
	}

	fn := func(t *testing.T, tc TileJSONVersionTestCase) {
		server.HostName = nil
		server.Port = ""

		// create handler with injected GetMap function
		// to avoid requiring an Atlas passed to HandleMapCapabitilies struct
		testMap := tc.setupMap()
		handler := server.HandleMapCapabilities{
			GetMap: func(mapName string) (atlas.Map, error) {
				if mapName == tc.mapName {
					return testMap, nil
				}
				return atlas.Map{}, fmt.Errorf("map not found: %s", mapName)
			},
		}

		uri := fmt.Sprintf("http://localhost:8080/capabilities/%s.json", tc.mapName)
		r, err := http.NewRequest(http.MethodGet, uri, nil)
		if err != nil {
			t.Fatal(err)
		}

		// set the context params that the handler expects
		params := map[string]string{
			"map_name": fmt.Sprintf("%s.json", tc.mapName),
		}
		ctx := httptreemux.AddParamsToContext(r.Context(), params)
		r = r.WithContext(ctx)

		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)

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

		// verify TileJSON version
		if tileJSON.TileJSON != tc.expectedVersion {
			t.Errorf("TileJSON version mismatch: got (%v) expected (%v)", tileJSON.TileJSON, tc.expectedVersion)
			return
		}

		// verify Fields presence/absence
		if tc.shouldHaveFields {
			if len(tileJSON.VectorLayers) == 0 {
				t.Errorf("expected VectorLayers, got none")
				return
			}

			// ckeck Fields for each layer
			for _, layer := range tileJSON.VectorLayers {
				expectedFields, ok := tc.expectedFields[layer.ID]
				if !ok {
					t.Errorf("unexpected layer ID: %v", layer.ID)
					continue
				}

				if !reflect.DeepEqual(layer.Fields, expectedFields) {
					t.Errorf("Fields mismatch for layer %v: got %v expected %v", layer.ID, layer.Fields, expectedFields)
				}
			}
		} else {
			// v2.0.0 should not have populated Fields
			for _, layer := range tileJSON.VectorLayers {
				if len(layer.Fields) > 0 {
					t.Errorf("TileJSON v2.0.0 layer %v should not have Fields, got %v", layer.ID, layer.Fields)
				}
			}
		}
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			fn(t, tc)
		})
	}
}

func TestHandleMapCapabilitiesErrorNotCached(t *testing.T) {
	server.HostName = nil
	server.Port = ""

	callCount := 0

	testMap := atlas.NewWebMercatorMap("test-map-flaky")
	testMap.Attribution = testMapAttribution
	testMap.Center = testMapCenter
	testMap.Layers = append(testMap.Layers, atlas.Layer{
		Name:              "test-layer",
		ProviderLayerName: "test-layer-provider",
		MinZoom:           4,
		MaxZoom:           9,
		Provider:          &test.TileProvider{},
		GeomType:          geom.Point{},
	})

	testAtlas := &atlas.Atlas{}
	testAtlas.AddMap(testMap)

	// mock a GetMap that fails first time, but succeeds the second time
	handler := &server.HandleMapCapabilities{
		GetMap: func(mapName string) (atlas.Map, error) {
			callCount++
			if callCount == 1 {
				return atlas.Map{}, fmt.Errorf("temporary database error")
			}
			return testMap, nil
		},
	}

	uri := "http://localhost:8080/capabilities/test-map-flaky.json"

	// first request will fail
	r1, _ := http.NewRequest(http.MethodGet, uri, nil)
	params := map[string]string{"map_name": "test-map-flaky.json"}
	ctx := httptreemux.AddParamsToContext(r1.Context(), params)
	r1 = r1.WithContext(ctx)

	w1 := httptest.NewRecorder()
	handler.ServeHTTP(w1, r1)

	if w1.Code != http.StatusInternalServerError {
		t.Errorf("first request should fail: got %v expected %v", w1.Code, http.StatusInternalServerError)
	}

	// second request - should succeed because errors are not being cached but retried
	r2, _ := http.NewRequest(http.MethodGet, uri, nil)
	r2 = r2.WithContext(httptreemux.AddParamsToContext(r2.Context(), params))

	w2 := httptest.NewRecorder()
	handler.ServeHTTP(w2, r2)

	if w2.Code != http.StatusOK {
		t.Errorf("second request should succeed: got %v expected %v", w2.Code, http.StatusOK)
	}

	if callCount != 2 {
		t.Errorf("GetMap should be called twice (no error caching), was called %d times", callCount)
	}
}
