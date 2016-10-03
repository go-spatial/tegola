package server_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/terranodo/tegola/server"
)

func TestCapabilities(t *testing.T) {
	//	setup a new provider
	testcases := []struct {
		handler http.Handler
		mapName string
		layers  []server.Layer
		//	built during our test as we need the dynamically generated
		//	host and port from httptest for comparing various endpoints
		expected server.Capabilities
	}{
		{
			handler: server.HandleCapabilities{},
			mapName: "test-map",
			layers: []server.Layer{
				server.Layer{
					Name:     "test-layer",
					MinZoom:  10,
					MaxZoom:  20,
					Provider: &testMVTProvider{},
					DefaultTags: map[string]interface{}{
						"foo": "bar",
					},
				},
			},
		},
	}

	for i, test := range testcases {
		var err error

		//	setup a test server
		ts := httptest.NewServer(test.handler)
		defer ts.Close()

		//	build out layer capabilities
		var layers []server.CapabilitiesLayer
		for _, layer := range test.layers {
			layers = append(layers, server.CapabilitiesLayer{
				Name: layer.Name,
				Tiles: []string{
					fmt.Sprintf("%v/maps/%v/%v/{z}/{x}/{y}.pbf", ts.Listener.Addr(), test.mapName, layer.Name),
				},
				MinZoom: layer.MinZoom,
				MaxZoom: layer.MaxZoom,
			})
		}

		//	build our expected capabilities
		test.expected = server.Capabilities{
			Version: serverVersion,
			Maps: []server.CapabilitiesMap{
				{
					Name:         test.mapName,
					Capabilities: fmt.Sprintf("%v/capabilities/%v.json", ts.Listener.Addr(), test.mapName),
					Tiles: []string{
						fmt.Sprintf("%v/maps/%v/{z}/{x}/{y}.pbf", ts.Listener.Addr(), test.mapName),
					},
					Layers: layers,
				},
			},
		}

		//	register a map with layers
		if err := server.RegisterMap(test.mapName, test.layers); err != nil {
			t.Errorf("Failed test %v. Unable to register map (%v)", i, test.mapName)
		}

		//	fetch the URL
		res, err := http.Get(ts.URL)
		if err != nil {
			t.Errorf("Failed test %v. Unable to GET URL (%v)", i, ts.URL)
		}

		var capabilities server.Capabilities
		//	read the respons body
		if err := json.NewDecoder(res.Body).Decode(&capabilities); err != nil {
			t.Errorf("Failed test %v. Unable to decode JSON response body", i)
		}
		defer res.Body.Close()

		if !reflect.DeepEqual(test.expected, capabilities) {
			t.Errorf("Failed test %v. Response body and expected do not match", i)
		}
	}
}
