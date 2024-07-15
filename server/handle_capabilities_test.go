package server_test

import (
	"encoding/json"
	"net/http"
	"net/url"
	"testing"

	"github.com/go-test/deep"

	"github.com/go-spatial/tegola"
	"github.com/go-spatial/tegola/atlas"
	"github.com/go-spatial/tegola/server"
)

func TestHandleCapabilities(t *testing.T) {

	type tcase struct {
		hostname *url.URL
		port     string
		uri      string
		expected server.Capabilities
	}

	fn := func(tc tcase) func(*testing.T) {
		return func(t *testing.T) {
			server.HostName = tc.hostname
			server.Port = tc.port

			a := newTestMapWithLayers(testLayer1, testLayer2, testLayer3)

			resp, _, err := doRequest(t, a, http.MethodGet, tc.uri, nil)
			if err != nil {
				t.Fatal(err)
			}
			if resp.Code != http.StatusOK {
				t.Errorf("status code, expected %v got %v", http.StatusOK, resp.Code)
				return
			}

			// read the response body
			var capabilities server.Capabilities
			if err := json.NewDecoder(resp.Body).Decode(&capabilities); err != nil {
				t.Errorf("unexpected error decoding response body: %s", err)
				return
			}

			if diff := deep.Equal(tc.expected, capabilities); diff != nil {
				t.Errorf("expected does not match output. diff: %v", diff)
			}
		}
	}

	tests := map[string]tcase{
		"empty host port": {
			//  With empty hostname and no port specified in config, urls should have host:port matching request uri.
			uri: "http://localhost:8080/capabilities",
			expected: server.Capabilities{
				Version: serverVersion,
				Maps: []server.CapabilitiesMap{
					{
						Name:         "test-map",
						Attribution:  "test attribution",
						Center:       [3]float64{1.0, 2.0, 3.0},
						Bounds:       tegola.WGS84Bounds,
						Capabilities: "http://localhost:8080/capabilities/test-map.json",
						Tiles: []server.TileURLTemplate{
							{
								Scheme:  "http",
								Host:    "localhost:8080",
								MapName: "test-map",
							},
						},
						Layers: []server.CapabilitiesLayer{
							{
								Name: testLayer1.MVTName(),
								Tiles: []server.TileURLTemplate{
									{
										Scheme:    "http",
										Host:      "localhost:8080",
										MapName:   "test-map",
										LayerName: testLayer1.MVTName(),
									},
								},
								MinZoom: testLayer1.MinZoom,
								MaxZoom: testLayer3.MaxZoom, // layer 1 and layer 3 share a name in our test so the zoom range includes the entire zoom range
							},
							{
								Name: testLayer2.MVTName(),
								Tiles: []server.TileURLTemplate{
									{
										Scheme:    "http",
										Host:      "localhost:8080",
										MapName:   "test-map",
										LayerName: testLayer2.MVTName(),
									},
								},
								MinZoom: testLayer2.MinZoom,
								MaxZoom: testLayer2.MaxZoom,
							},
						},
					},
				},
			},
		},
		"none port cdn host": {
			// With hostname set and port set to "none" in config, urls should have host "cdn.tegola.io"
			// debug layers turned on
			hostname: &url.URL{
				Host: "cdn.tegola.io",
			},
			port: "none", // Set to none or port 8080 from uri will be used.
			uri:  "http://localhost:8080/capabilities?debug=true",
			expected: server.Capabilities{
				Version: serverVersion,
				Maps: []server.CapabilitiesMap{
					{
						Name:         "test-map",
						Attribution:  "test attribution",
						Center:       [3]float64{1.0, 2.0, 3.0},
						Bounds:       tegola.WGS84Bounds,
						Capabilities: "http://cdn.tegola.io/capabilities/test-map.json?debug=true",
						Tiles: []server.TileURLTemplate{
							{
								Scheme:  "http",
								Host:    "cdn.tegola.io",
								MapName: "test-map",
								Query: url.Values{
									server.QueryKeyDebug: []string{"true"},
								},
							},
						},
						Layers: []server.CapabilitiesLayer{
							{
								Name: testLayer1.MVTName(),
								Tiles: []server.TileURLTemplate{
									{
										Scheme:    "http",
										Host:      "cdn.tegola.io",
										MapName:   "test-map",
										LayerName: testLayer1.MVTName(),
										Query: url.Values{
											server.QueryKeyDebug: []string{"true"},
										},
									},
								},
								MinZoom: testLayer1.MinZoom,
								MaxZoom: testLayer3.MaxZoom, // layer 1 and layer 3 share a name in our test so the zoom range includes the entire zoom range
							},
							{
								Name: "test-layer-2-name",
								Tiles: []server.TileURLTemplate{
									{
										Scheme:    "http",
										Host:      "cdn.tegola.io",
										MapName:   "test-map",
										LayerName: testLayer2.MVTName(),
										Query: url.Values{
											server.QueryKeyDebug: []string{"true"},
										},
									},
								},
								MinZoom: testLayer2.MinZoom,
								MaxZoom: testLayer2.MaxZoom,
							},
							{
								Name: "debug-tile-outline",
								Tiles: []server.TileURLTemplate{
									{
										Scheme:    "http",
										Host:      "cdn.tegola.io",
										MapName:   "test-map",
										LayerName: "debug-tile-outline",
										Query: url.Values{
											server.QueryKeyDebug: []string{"true"},
										},
									},
								},
								MinZoom: 0,
								MaxZoom: atlas.MaxZoom,
							},
							{
								Name: "debug-tile-center",
								Tiles: []server.TileURLTemplate{
									{
										Scheme:    "http",
										Host:      "cdn.tegola.io",
										MapName:   "test-map",
										LayerName: "debug-tile-center",
										Query: url.Values{
											server.QueryKeyDebug: []string{"true"},
										},
									},
								},
								MinZoom: 0,
								MaxZoom: atlas.MaxZoom,
							},
						},
					},
				},
			},
		},
		"std": {
			uri: "http://localhost:8080/capabilities",
			expected: server.Capabilities{
				Version: serverVersion,
				Maps: []server.CapabilitiesMap{
					{
						Name:         "test-map",
						Attribution:  "test attribution",
						Center:       [3]float64{1.0, 2.0, 3.0},
						Bounds:       tegola.WGS84Bounds,
						Capabilities: "http://localhost:8080/capabilities/test-map.json",
						Tiles: []server.TileURLTemplate{
							{
								Scheme:  "http",
								Host:    "localhost:8080",
								MapName: "test-map",
							},
						},
						Layers: []server.CapabilitiesLayer{
							{
								Name: testLayer1.MVTName(),
								Tiles: []server.TileURLTemplate{
									{
										Scheme:    "http",
										Host:      "localhost:8080",
										MapName:   "test-map",
										LayerName: testLayer1.MVTName(),
									},
								},
								MinZoom: testLayer1.MinZoom,
								MaxZoom: testLayer3.MaxZoom, // layer 1 and layer 3 share a name in our test so the zoom range includes the entire zoom range
							},
							{
								Name: testLayer2.MVTName(),
								Tiles: []server.TileURLTemplate{
									{
										Scheme:    "http",
										Host:      "localhost:8080",
										MapName:   "test-map",
										LayerName: testLayer2.MVTName(),
									},
								},
								MinZoom: testLayer2.MinZoom,
								MaxZoom: testLayer2.MaxZoom,
							},
						},
					},
				},
			},
		},
		"unset port set host": {
			// With hostname set in config, port unset in config, and no port in request uri,
			// urls should have host from config and no port: "cdn.tegola.io"
			hostname: &url.URL{
				Host: "cdn.tegola.io",
			},
			port: "none", // Set to none or port 8080 from uri will be used.
			uri:  "http://localhost/capabilities?debug=true",
			expected: server.Capabilities{
				Version: serverVersion,
				Maps: []server.CapabilitiesMap{
					{
						Name:         "test-map",
						Attribution:  "test attribution",
						Center:       [3]float64{1.0, 2.0, 3.0},
						Bounds:       tegola.WGS84Bounds,
						Capabilities: "http://cdn.tegola.io/capabilities/test-map.json?debug=true",
						Tiles: []server.TileURLTemplate{
							{
								Scheme:  "http",
								Host:    "cdn.tegola.io",
								MapName: "test-map",
								Query: url.Values{
									server.QueryKeyDebug: []string{
										"true",
									},
								},
							},
						},
						Layers: []server.CapabilitiesLayer{
							{
								Name: testLayer1.MVTName(),
								Tiles: []server.TileURLTemplate{
									{
										Scheme:    "http",
										Host:      "cdn.tegola.io",
										MapName:   "test-map",
										LayerName: testLayer1.MVTName(),
										Query: url.Values{
											server.QueryKeyDebug: []string{
												"true",
											},
										},
									},
								},
								MinZoom: testLayer1.MinZoom,
								MaxZoom: testLayer3.MaxZoom, // layer 1 and layer 3 share a name in our test so the zoom range includes the entire zoom range
							},
							{
								Name: "test-layer-2-name",
								Tiles: []server.TileURLTemplate{
									{
										Scheme:    "http",
										Host:      "cdn.tegola.io",
										MapName:   "test-map",
										LayerName: testLayer2.MVTName(),
										Query: url.Values{
											server.QueryKeyDebug: []string{
												"true",
											},
										},
									},
								},
								MinZoom: testLayer2.MinZoom,
								MaxZoom: testLayer2.MaxZoom,
							},
							{
								Name: "debug-tile-outline",
								Tiles: []server.TileURLTemplate{
									{
										Scheme:    "http",
										Host:      "cdn.tegola.io",
										MapName:   "test-map",
										LayerName: "debug-tile-outline",
										Query: url.Values{
											server.QueryKeyDebug: []string{
												"true",
											},
										},
									},
								},
								MinZoom: 0,
								MaxZoom: atlas.MaxZoom,
							},
							{
								Name: "debug-tile-center",
								Tiles: []server.TileURLTemplate{
									{
										Scheme:    "http",
										Host:      "cdn.tegola.io",
										MapName:   "test-map",
										LayerName: "debug-tile-center",
										Query: url.Values{
											server.QueryKeyDebug: []string{
												"true",
											},
										},
									},
								},
								MinZoom: 0,
								MaxZoom: atlas.MaxZoom,
							},
						},
					},
				},
			},
		},
		"config set hostname unset port": {
			// With hostname set and port unset in config, urls should have host from config and
			//  port from uri: "cdn.tegola.io:8080"
			hostname: &url.URL{
				Host: "cdn.tegola.io",
			},
			uri: "http://localhost:8080/capabilities?debug=true",
			expected: server.Capabilities{
				Version: serverVersion,
				Maps: []server.CapabilitiesMap{
					{
						Name:         "test-map",
						Attribution:  "test attribution",
						Center:       [3]float64{1.0, 2.0, 3.0},
						Bounds:       tegola.WGS84Bounds,
						Capabilities: "http://cdn.tegola.io/capabilities/test-map.json?debug=true",
						Tiles: []server.TileURLTemplate{
							{
								Scheme:  "http",
								Host:    "cdn.tegola.io",
								MapName: "test-map",
								Query: url.Values{
									server.QueryKeyDebug: []string{"true"},
								},
							},
						},
						Layers: []server.CapabilitiesLayer{
							{
								Name: testLayer1.MVTName(),
								Tiles: []server.TileURLTemplate{
									{
										Scheme:    "http",
										Host:      "cdn.tegola.io",
										MapName:   "test-map",
										LayerName: testLayer1.MVTName(),
										Query: url.Values{
											server.QueryKeyDebug: []string{"true"},
										},
									},
								},
								MinZoom: testLayer1.MinZoom,
								MaxZoom: testLayer3.MaxZoom, // layer 1 and layer 3 share a name in our test so the zoom range includes the entire zoom range
							},
							{
								Name: "test-layer-2-name",
								Tiles: []server.TileURLTemplate{
									{
										Scheme:    "http",
										Host:      "cdn.tegola.io",
										MapName:   "test-map",
										LayerName: testLayer2.MVTName(),
										Query: url.Values{
											server.QueryKeyDebug: []string{"true"},
										},
									},
								},
								MinZoom: testLayer2.MinZoom,
								MaxZoom: testLayer2.MaxZoom,
							},
							{
								Name: "debug-tile-outline",
								Tiles: []server.TileURLTemplate{
									{
										Scheme:    "http",
										Host:      "cdn.tegola.io",
										MapName:   "test-map",
										LayerName: "debug-tile-outline",
										Query: url.Values{
											server.QueryKeyDebug: []string{"true"},
										},
									},
								},
								MinZoom: 0,
								MaxZoom: atlas.MaxZoom,
							},
							{
								Name: "debug-tile-center",
								Tiles: []server.TileURLTemplate{
									{
										Scheme:    "http",
										Host:      "cdn.tegola.io",
										MapName:   "test-map",
										LayerName: "debug-tile-center",
										Query: url.Values{
											server.QueryKeyDebug: []string{"true"},
										},
									},
								},
								MinZoom: 0,
								MaxZoom: atlas.MaxZoom,
							},
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

func TestHandleCapabilitiesCORS(t *testing.T) {
	tests := map[string]CORSTestCase{
		"1": {
			uri: "/capabilities",
		},
		"hostname": {
			uri:      "/capabilities",
			hostname: "tegola.io",
		},
	}

	for name, tc := range tests {
		t.Run(name, CORSTest(tc))
	}
}
