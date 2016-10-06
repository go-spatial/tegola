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
		handler   http.Handler
		serverMap server.Map
		//	built during our test as we need the dynamically generated
		//	host and port from httptest for comparing various endpoints
		expected server.Capabilities
	}{
		{
			handler: server.HandleCapabilities{},
			serverMap: server.Map{
				Name:   "test-map",
				Center: [3]float64{1.0, 2.0, 3.0},
				Layers: []server.Layer{
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
		},
	}

	for i, test := range testcases {
		var err error

		//	setup a test server
		ts := httptest.NewServer(test.handler)
		defer ts.Close()

		//	build out layer capabilities
		var layers []server.CapabilitiesLayer
		for _, layer := range test.serverMap.Layers {
			layers = append(layers, server.CapabilitiesLayer{
				Name: layer.Name,
				Tiles: []string{
					fmt.Sprintf("%v/maps/%v/%v/{z}/{x}/{y}.pbf", ts.Listener.Addr(), test.serverMap.Name, layer.Name),
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
					Name:         test.serverMap.Name,
					Center:       test.serverMap.Center,
					Capabilities: fmt.Sprintf("%v/capabilities/%v.json", ts.Listener.Addr(), test.serverMap.Name),
					Tiles: []string{
						fmt.Sprintf("%v/maps/%v/{z}/{x}/{y}.pbf", ts.Listener.Addr(), test.serverMap.Name),
					},
					Layers: layers,
				},
			},
		}

		//	register a map with layers
		if err := server.RegisterMap(test.serverMap); err != nil {
			t.Errorf("Failed test %v. Unable to register map (%v)", i, test.serverMap.Name)
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
			t.Errorf("Failed test %v. Response body and expected do not match \n%+v\n%+v", i, test.expected, capabilities)
		}
	}
}
