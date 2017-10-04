package server_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/dimfeld/httptreemux"
	"github.com/terranodo/tegola/server"
)

func TestHandleCapabilities(t *testing.T) {
	//	setup a new provider
	testcases := []struct {
		handler    http.Handler
		hostname   string
		uri        string
		uriPattern string
		reqMethod  string
		expected   server.Capabilities
	}{
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
						Capabilities: "http://localhost:8080/capabilities/test-map.json",
						Tiles: []string{
							"http://localhost:8080/maps/test-map/{z}/{x}/{y}.pbf",
						},
						Layers: []server.CapabilitiesLayer{
							{
								Name: "test-layer-1",
								Tiles: []string{
									"http://localhost:8080/maps/test-map/test-layer-1/{z}/{x}/{y}.pbf",
								},
								MinZoom: 4,
								MaxZoom: 9,
							},
							{
								Name: "test-layer-2-name",
								Tiles: []string{
									"http://localhost:8080/maps/test-map/test-layer-2-name/{z}/{x}/{y}.pbf",
								},
								MinZoom: 10,
								MaxZoom: 20,
							},
						},
					},
				},
			},
		},
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
						Capabilities: "http://cdn.tegola.io/capabilities/test-map.json?debug=true",
						Tiles: []string{
							"http://cdn.tegola.io/maps/test-map/{z}/{x}/{y}.pbf?debug=true",
						},
						Layers: []server.CapabilitiesLayer{

							{
								Name: "test-layer-1",
								Tiles: []string{
									"http://cdn.tegola.io/maps/test-map/test-layer-1/{z}/{x}/{y}.pbf?debug=true",
								},
								MinZoom: 4,
								MaxZoom: 9,
							},
							{
								Name: "test-layer-2-name",
								Tiles: []string{
									"http://cdn.tegola.io/maps/test-map/test-layer-2-name/{z}/{x}/{y}.pbf?debug=true",
								},
								MinZoom: 10,
								MaxZoom: 20,
							},
							{
								Name: "debug-tile-outline",
								Tiles: []string{
									"http://cdn.tegola.io/maps/test-map/debug-tile-outline/{z}/{x}/{y}.pbf?debug=true",
								},
								MinZoom: 0,
								MaxZoom: server.MaxZoom,
							},
							{
								Name: "debug-tile-center",
								Tiles: []string{
									"http://cdn.tegola.io/maps/test-map/debug-tile-center/{z}/{x}/{y}.pbf?debug=true",
								},
								MinZoom: 0,
								MaxZoom: server.MaxZoom,
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
