package server_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/terranodo/tegola/server"
)

func TestCapabilities(t *testing.T) {
	//	setup a new provider
	testcases := []struct {
		handler  http.Handler
		mapName  string
		layers   []server.Layer
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
			expected: server.Capabilities{
				Version: "0.3.0",
				Maps: []server.CapabilitiesMap{
					{
						Name: "test-map",
						URI:  "/maps/test-map",
						Layers: []server.CapabilitiesLayer{
							{
								Name:    "test-layer",
								URI:     "/maps/test-map/test-layer",
								MinZoom: 10,
								MaxZoom: 20,
							},
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
