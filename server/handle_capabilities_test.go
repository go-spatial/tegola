package server_test

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"reflect"
	"testing"

	"github.com/go-spatial/geom/slippy"

	"github.com/go-spatial/tegola/atlas"
	"github.com/go-spatial/tegola/server"
)

func TestHandleCapabilities(t *testing.T) {

	type tcase struct {
		hostname string
		port     string
		uri      string
		method   string
		expected server.Capabilities
	}

	fn := func(t *testing.T, tc tcase) {
		var err error

		server.HostName = tc.hostname
		server.Port = tc.port
		a := newTestMapWithLayers(testLayer1, testLayer2, testLayer3)

		w, _, err := doRequest(a, tc.method, tc.uri, nil)
		if err != nil {
			t.Fatal(err)
		}

		if w.Code != http.StatusOK {
			t.Errorf("status code, expected %v got %v", http.StatusOK, w.Code)
			return
		}

		bytes, err := ioutil.ReadAll(w.Body)
		if err != nil {
			t.Errorf("error response body, expected nil got %v", err)
			return
		}

		var capabilities server.Capabilities

		// read the respons body
		if err := json.Unmarshal(bytes, &capabilities); err != nil {
			t.Errorf("error unmarshal JSON, expected nil got %v", err)
			return
		}

		if !reflect.DeepEqual(tc.expected, capabilities) {
			t.Errorf("response body, \n  expected %+v\n  got %+v", tc.expected, capabilities)
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
						Bounds:       slippy.SupportedProjections[3857].WGS84Extents,
						Capabilities: "http://localhost:8080/capabilities/test-map.json",
						Tiles: []string{
							"http://localhost:8080/maps/test-map/{z}/{x}/{y}.pbf",
						},
						Layers: []server.CapabilitiesLayer{
							{
								Name: testLayer1.MVTName(),
								Tiles: []string{
									fmt.Sprintf("http://localhost:8080/maps/test-map/%v/{z}/{x}/{y}.pbf", testLayer1.MVTName()),
								},
								MinZoom: testLayer1.MinZoom,
								MaxZoom: testLayer3.MaxZoom, // layer 1 and layer 3 share a name in our test so the zoom range includes the entire zoom range
							},
							{
								Name: testLayer2.MVTName(),
								Tiles: []string{
									fmt.Sprintf("http://localhost:8080/maps/test-map/%v/{z}/{x}/{y}.pbf", testLayer2.MVTName()),
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
			hostname: "cdn.tegola.io",
			port:     "none", // Set to none or port 8080 from uri will be used.
			uri:      "http://localhost:8080/capabilities?debug=true",
			expected: server.Capabilities{
				Version: serverVersion,
				Maps: []server.CapabilitiesMap{
					{
						Name:         "test-map",
						Attribution:  "test attribution",
						Center:       [3]float64{1.0, 2.0, 3.0},
						Bounds:       slippy.SupportedProjections[3857].WGS84Extents,
						Capabilities: "http://cdn.tegola.io/capabilities/test-map.json?debug=true",
						Tiles: []string{
							"http://cdn.tegola.io/maps/test-map/{z}/{x}/{y}.pbf?debug=true",
						},
						Layers: []server.CapabilitiesLayer{
							{
								Name: testLayer1.MVTName(),
								Tiles: []string{
									fmt.Sprintf("http://cdn.tegola.io/maps/test-map/%v/{z}/{x}/{y}.pbf?debug=true", testLayer1.MVTName()),
								},
								MinZoom: testLayer1.MinZoom,
								MaxZoom: testLayer3.MaxZoom, // layer 1 and layer 3 share a name in our test so the zoom range includes the entire zoom range
							},
							{
								Name: "test-layer-2-name",
								Tiles: []string{
									fmt.Sprintf("http://cdn.tegola.io/maps/test-map/%v/{z}/{x}/{y}.pbf?debug=true", testLayer2.MVTName()),
								},
								MinZoom: testLayer2.MinZoom,
								MaxZoom: testLayer2.MaxZoom,
							},
							{
								Name: "debug-tile-outline",
								Tiles: []string{
									"http://cdn.tegola.io/maps/test-map/debug-tile-outline/{z}/{x}/{y}.pbf?debug=true",
								},
								MinZoom: 0,
								MaxZoom: atlas.MaxZoom,
							},
							{
								Name: "debug-tile-center",
								Tiles: []string{
									"http://cdn.tegola.io/maps/test-map/debug-tile-center/{z}/{x}/{y}.pbf?debug=true",
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
						Bounds:       slippy.SupportedProjections[3857].WGS84Extents,
						Capabilities: "http://localhost:8080/capabilities/test-map.json",
						Tiles: []string{
							"http://localhost:8080/maps/test-map/{z}/{x}/{y}.pbf",
						},
						Layers: []server.CapabilitiesLayer{
							{
								Name: testLayer1.MVTName(),
								Tiles: []string{
									fmt.Sprintf("http://localhost:8080/maps/test-map/%v/{z}/{x}/{y}.pbf", testLayer1.MVTName()),
								},
								MinZoom: testLayer1.MinZoom,
								MaxZoom: testLayer3.MaxZoom, // layer 1 and layer 3 share a name in our test so the zoom range includes the entire zoom range
							},
							{
								Name: testLayer2.MVTName(),
								Tiles: []string{
									fmt.Sprintf("http://localhost:8080/maps/test-map/%v/{z}/{x}/{y}.pbf", testLayer2.MVTName()),
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
			hostname: "cdn.tegola.io",
			port:     "none", // Set to none or port 8080 from uri will be used.
			uri:      "http://localhost/capabilities?debug=true",
			expected: server.Capabilities{
				Version: serverVersion,
				Maps: []server.CapabilitiesMap{
					{
						Name:         "test-map",
						Attribution:  "test attribution",
						Center:       [3]float64{1.0, 2.0, 3.0},
						Bounds:       slippy.SupportedProjections[3857].WGS84Extents,
						Capabilities: "http://cdn.tegola.io/capabilities/test-map.json?debug=true",
						Tiles: []string{
							"http://cdn.tegola.io/maps/test-map/{z}/{x}/{y}.pbf?debug=true",
						},
						Layers: []server.CapabilitiesLayer{
							{
								Name: testLayer1.MVTName(),
								Tiles: []string{
									fmt.Sprintf("http://cdn.tegola.io/maps/test-map/%v/{z}/{x}/{y}.pbf?debug=true", testLayer1.MVTName()),
								},
								MinZoom: testLayer1.MinZoom,
								MaxZoom: testLayer3.MaxZoom, // layer 1 and layer 3 share a name in our test so the zoom range includes the entire zoom range
							},
							{
								Name: "test-layer-2-name",
								Tiles: []string{
									fmt.Sprintf("http://cdn.tegola.io/maps/test-map/%v/{z}/{x}/{y}.pbf?debug=true", testLayer2.MVTName()),
								},
								MinZoom: testLayer2.MinZoom,
								MaxZoom: testLayer2.MaxZoom,
							},
							{
								Name: "debug-tile-outline",
								Tiles: []string{
									"http://cdn.tegola.io/maps/test-map/debug-tile-outline/{z}/{x}/{y}.pbf?debug=true",
								},
								MinZoom: 0,
								MaxZoom: atlas.MaxZoom,
							},
							{
								Name: "debug-tile-center",
								Tiles: []string{
									"http://cdn.tegola.io/maps/test-map/debug-tile-center/{z}/{x}/{y}.pbf?debug=true",
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
			hostname: "cdn.tegola.io",
			uri:      "http://localhost:8080/capabilities?debug=true",
			expected: server.Capabilities{
				Version: serverVersion,
				Maps: []server.CapabilitiesMap{
					{
						Name:         "test-map",
						Attribution:  "test attribution",
						Center:       [3]float64{1.0, 2.0, 3.0},
						Bounds:       slippy.SupportedProjections[3857].WGS84Extents,
						Capabilities: "http://cdn.tegola.io/capabilities/test-map.json?debug=true",
						Tiles: []string{
							"http://cdn.tegola.io/maps/test-map/{z}/{x}/{y}.pbf?debug=true",
						},
						Layers: []server.CapabilitiesLayer{
							{
								Name: testLayer1.MVTName(),
								Tiles: []string{
									fmt.Sprintf("http://cdn.tegola.io/maps/test-map/%v/{z}/{x}/{y}.pbf?debug=true", testLayer1.MVTName()),
								},
								MinZoom: testLayer1.MinZoom,
								MaxZoom: testLayer3.MaxZoom, // layer 1 and layer 3 share a name in our test so the zoom range includes the entire zoom range
							},
							{
								Name: "test-layer-2-name",
								Tiles: []string{
									fmt.Sprintf("http://cdn.tegola.io/maps/test-map/%v/{z}/{x}/{y}.pbf?debug=true", testLayer2.MVTName()),
								},
								MinZoom: testLayer2.MinZoom,
								MaxZoom: testLayer2.MaxZoom,
							},
							{
								Name: "debug-tile-outline",
								Tiles: []string{
									"http://cdn.tegola.io/maps/test-map/debug-tile-outline/{z}/{x}/{y}.pbf?debug=true",
								},
								MinZoom: 0,
								MaxZoom: atlas.MaxZoom,
							},
							{
								Name: "debug-tile-center",
								Tiles: []string{
									"http://cdn.tegola.io/maps/test-map/debug-tile-center/{z}/{x}/{y}.pbf?debug=true",
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
		tc := tc
		t.Run(name, func(t *testing.T) { fn(t, tc) })
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
		tc := tc
		t.Run(name, func(t *testing.T) { CORSTest(t, tc) })
	}
}
