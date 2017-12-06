package server_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/dimfeld/httptreemux"

	"github.com/terranodo/tegola/atlas"
	"github.com/terranodo/tegola/server"
)

func TestHandleCapabilities(t *testing.T) {
	//	setup a new provider
	testcases := []struct {
		handler    http.Handler
		hostname   string
		port       string
		uri        string
		uriPattern string
		reqMethod  string
		expected   server.Capabilities
	}{
		// With empty hostname and no port specified in config, urls should have host:port matching
		//	request uri.
		{
			handler:    server.HandleCapabilities{},
			hostname:   "",
			uri:        "http://localhost:8080/capabilities",
			uriPattern: "/capabilities",
			reqMethod:  "GET",
			expected: server.Capabilities{
				Version: serverVersion,
				Maps: []server.CapabilitiesMap{
					{
						Name:         "test-map",
						Attribution:  "test attribution",
						Center:       [3]float64{1.0, 2.0, 3.0},
						Bounds:       [4]float64{-180.0, -85.0511, 180.0, 85.0511},
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
								MaxZoom: testLayer3.MaxZoom, //	layer 1 and layer 3 share a name in our test so the zoom range includes the entire zoom range
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
		// With hostname set and port set to "none" in config, urls should have host "cdn.tegola.io"
		{
			handler:    server.HandleCapabilities{},
			hostname:   "cdn.tegola.io",
			port:       "none", // Set to none or port 8080 from uri will be used.
			uri:        "http://localhost:8080/capabilities?debug=true",
			uriPattern: "/capabilities",
			reqMethod:  "GET",
			expected: server.Capabilities{
				Version: serverVersion,
				Maps: []server.CapabilitiesMap{
					{
						Name:         "test-map",
						Attribution:  "test attribution",
						Center:       [3]float64{1.0, 2.0, 3.0},
						Bounds:       [4]float64{-180.0, -85.0511, 180.0, 85.0511},
						Capabilities: "http://cdn.tegola.io/capabilities/test-map.json?debug=true",
						Tiles: []string{
							"http://cdn.tegola.io/maps/test-map/{z}/{x}/{y}.pbf?debug=true",
						},
						Layers: []server.CapabilitiesLayer{
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
							{
								Name: testLayer1.MVTName(),
								Tiles: []string{
									fmt.Sprintf("http://cdn.tegola.io/maps/test-map/%v/{z}/{x}/{y}.pbf?debug=true", testLayer1.MVTName()),
								},
								MinZoom: testLayer1.MinZoom,
								MaxZoom: testLayer3.MaxZoom, //	layer 1 and layer 3 share a name in our test so the zoom range includes the entire zoom range
							},
							{
								Name: "test-layer-2-name",
								Tiles: []string{
									fmt.Sprintf("http://cdn.tegola.io/maps/test-map/%v/{z}/{x}/{y}.pbf?debug=true", testLayer2.MVTName()),
								},
								MinZoom: testLayer2.MinZoom,
								MaxZoom: testLayer2.MaxZoom,
							},
						},
					},
				},
			},
		},
		{
			handler:    server.HandleCapabilities{},
			hostname:   "",
			uri:        "http://localhost:8080/capabilities",
			uriPattern: "/capabilities",
			reqMethod:  "GET",
			expected: server.Capabilities{
				Version: serverVersion,
				Maps: []server.CapabilitiesMap{
					{
						Name:         "test-map",
						Attribution:  "test attribution",
						Center:       [3]float64{1.0, 2.0, 3.0},
						Bounds:       [4]float64{-180.0, -85.0511, 180.0, 85.0511},
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
								MaxZoom: testLayer3.MaxZoom, //	layer 1 and layer 3 share a name in our test so the zoom range includes the entire zoom range
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
		// With hostname set in config, port unset in config, and no port in request uri,
		//	 urls should have host from config and no port: "cdn.tegola.io"
		{
			handler:    server.HandleCapabilities{},
			hostname:   "cdn.tegola.io",
			port:       "none", // Set to none or port 8080 from uri will be used.
			uri:        "http://localhost/capabilities?debug=true",
			uriPattern: "/capabilities",
			reqMethod:  "GET",
			expected: server.Capabilities{
				Version: serverVersion,
				Maps: []server.CapabilitiesMap{
					{
						Name:         "test-map",
						Attribution:  "test attribution",
						Center:       [3]float64{1.0, 2.0, 3.0},
						Bounds:       [4]float64{-180.0, -85.0511, 180.0, 85.0511},
						Capabilities: "http://cdn.tegola.io/capabilities/test-map.json?debug=true",
						Tiles: []string{
							"http://cdn.tegola.io/maps/test-map/{z}/{x}/{y}.pbf?debug=true",
						},
						Layers: []server.CapabilitiesLayer{
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
							{
								Name: testLayer1.MVTName(),
								Tiles: []string{
									fmt.Sprintf("http://cdn.tegola.io/maps/test-map/%v/{z}/{x}/{y}.pbf?debug=true", testLayer1.MVTName()),
								},
								MinZoom: testLayer1.MinZoom,
								MaxZoom: testLayer3.MaxZoom, //	layer 1 and layer 3 share a name in our test so the zoom range includes the entire zoom range
							},
							{
								Name: "test-layer-2-name",
								Tiles: []string{
									fmt.Sprintf("http://cdn.tegola.io/maps/test-map/%v/{z}/{x}/{y}.pbf?debug=true", testLayer2.MVTName()),
								},
								MinZoom: testLayer2.MinZoom,
								MaxZoom: testLayer2.MaxZoom,
							},
						},
					},
				},
			},
		},
		// With hostname set and port unset in config, urls should have host from config and
		// 	port from uri: "cdn.tegola.io:8080"
		{
			handler:    server.HandleCapabilities{},
			hostname:   "cdn.tegola.io",
			uri:        "http://localhost:8080/capabilities?debug=true",
			uriPattern: "/capabilities",
			reqMethod:  "GET",
			expected: server.Capabilities{
				Version: serverVersion,
				Maps: []server.CapabilitiesMap{
					{
						Name:         "test-map",
						Attribution:  "test attribution",
						Center:       [3]float64{1.0, 2.0, 3.0},
						Bounds:       [4]float64{-180.0, -85.0511, 180.0, 85.0511},
						Capabilities: "http://cdn.tegola.io:8080/capabilities/test-map.json?debug=true",
						Tiles: []string{
							"http://cdn.tegola.io:8080/maps/test-map/{z}/{x}/{y}.pbf?debug=true",
						},
						Layers: []server.CapabilitiesLayer{
							{
								Name: "debug-tile-outline",
								Tiles: []string{
									"http://cdn.tegola.io:8080/maps/test-map/debug-tile-outline/{z}/{x}/{y}.pbf?debug=true",
								},
								MinZoom: 0,
								MaxZoom: atlas.MaxZoom,
							},
							{
								Name: "debug-tile-center",
								Tiles: []string{
									"http://cdn.tegola.io:8080/maps/test-map/debug-tile-center/{z}/{x}/{y}.pbf?debug=true",
								},
								MinZoom: 0,
								MaxZoom: atlas.MaxZoom,
							},
							{
								Name: testLayer1.MVTName(),
								Tiles: []string{
									fmt.Sprintf("http://cdn.tegola.io:8080/maps/test-map/%v/{z}/{x}/{y}.pbf?debug=true", testLayer1.MVTName()),
								},
								MinZoom: testLayer1.MinZoom,
								MaxZoom: testLayer3.MaxZoom, //	layer 1 and layer 3 share a name in our test so the zoom range includes the entire zoom range
							},
							{
								Name: "test-layer-2-name",
								Tiles: []string{
									fmt.Sprintf("http://cdn.tegola.io:8080/maps/test-map/%v/{z}/{x}/{y}.pbf?debug=true", testLayer2.MVTName()),
								},
								MinZoom: testLayer2.MinZoom,
								MaxZoom: testLayer2.MaxZoom,
							},
						},
					},
				},
			},
		},
	}

	for i, test := range testcases {
		var err error

		server.HostName = test.hostname
		server.Port = test.port

		//	setup a new router. this handles parsing our URL wildcards (i.e. :map_name, :z, :x, :y)
		router := httptreemux.New()
		//	setup a new router group
		group := router.NewGroup("/")
		group.UsingContext().Handler(test.reqMethod, test.uriPattern, server.HandleCapabilities{})

		r, err := http.NewRequest(test.reqMethod, test.uri, nil)
		if err != nil {
			t.Fatal(err)
		}

		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)

		if w.Code != http.StatusOK {
			t.Errorf("Failed test %v. handler returned wrong status code: got (%v) expected (%v)", i, w.Code, http.StatusOK)
		}

		var capabilities server.Capabilities
		//	read the respons body
		if err := json.NewDecoder(w.Body).Decode(&capabilities); err != nil {
			t.Errorf("Failed test %v. Unable to decode JSON response body", i)
		}

		if !reflect.DeepEqual(test.expected, capabilities) {
			t.Errorf("Failed test %v. Response body and expected do not match \n%+v\n%+v", i, test.expected, capabilities)
		}
	}
}
