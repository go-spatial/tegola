package server_test

import (
	"reflect"
	"testing"

	"github.com/terranodo/tegola/server"
)

func TestMapFilterLayersByZoom(t *testing.T) {
	testcases := []struct {
		serverMap server.Map
		zoom      int
		expected  []server.Layer
	}{
		{
			serverMap: server.Map{
				Layers: []server.Layer{
					{
						Name:    "layer1",
						MinZoom: 0,
						MaxZoom: 2,
					},
					{
						Name:    "layer2",
						MinZoom: 1,
						MaxZoom: 5,
					},
				},
			},
			zoom: 2,
			expected: []server.Layer{
				{
					Name:    "layer1",
					MinZoom: 0,
					MaxZoom: 2,
				},
				{
					Name:    "layer2",
					MinZoom: 1,
					MaxZoom: 5,
				},
			},
		},
	}

	for i, tc := range testcases {
		output := tc.serverMap.FilterLayersByZoom(tc.zoom)

		if !reflect.DeepEqual(output, tc.expected) {
			t.Errorf("testcase (%v) failed. output \n\n%+v\n\n does not match expected \n\n%+v", i, output, tc.expected)
		}
	}
}

func TestMapFilterLayersByName(t *testing.T) {
	testcases := []struct {
		serverMap server.Map
		name      string
		expected  []server.Layer
	}{
		{
			serverMap: server.Map{
				Layers: []server.Layer{
					{
						Name:    "layer1",
						MinZoom: 0,
						MaxZoom: 2,
					},
					{
						Name:    "layer2",
						MinZoom: 1,
						MaxZoom: 5,
					},
				},
			},
			name: "layer1",
			expected: []server.Layer{
				{
					Name:    "layer1",
					MinZoom: 0,
					MaxZoom: 2,
				},
			},
		},
		{
			serverMap: server.Map{
				Layers: []server.Layer{
					{
						ProviderLayerName: "layer1",
						MinZoom:           0,
						MaxZoom:           2,
					},
					{
						ProviderLayerName: "layer2",
						MinZoom:           1,
						MaxZoom:           5,
					},
				},
			},
			name: "layer2",
			expected: []server.Layer{
				{
					ProviderLayerName: "layer2",
					MinZoom:           1,
					MaxZoom:           5,
				},
			},
		},
	}

	for i, tc := range testcases {
		output := tc.serverMap.FilterLayersByName(tc.name)

		if !reflect.DeepEqual(output, tc.expected) {
			t.Errorf("testcase (%v) failed. output \n\n%+v\n\n does not match expected \n\n%+v", i, output, tc.expected)
		}
	}
}
