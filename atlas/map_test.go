package atlas_test

import (
	"reflect"
	"testing"

	"github.com/terranodo/tegola/atlas"
)

func TestMapEnableLayersByZoom(t *testing.T) {
	testcases := []struct {
		atlasMap atlas.Map
		zoom     int
		expected atlas.Map
	}{
		{
			atlasMap: atlas.Map{
				Layers: []atlas.Layer{
					{
						Name:     "layer1",
						MinZoom:  0,
						MaxZoom:  2,
						Disabled: false,
					},
					{
						Name:     "layer2",
						MinZoom:  1,
						MaxZoom:  5,
						Disabled: false,
					},
				},
			},
			zoom: 5,
			expected: atlas.Map{
				Layers: []atlas.Layer{
					{
						Name:     "layer1",
						MinZoom:  0,
						MaxZoom:  2,
						Disabled: true,
					},
					{
						Name:     "layer2",
						MinZoom:  1,
						MaxZoom:  5,
						Disabled: false,
					},
				},
			},
		},
		{
			atlasMap: atlas.Map{
				Layers: []atlas.Layer{
					{
						Name:     "layer1",
						MinZoom:  0,
						MaxZoom:  2,
						Disabled: false,
					},
					{
						Name:     "layer2",
						MinZoom:  1,
						MaxZoom:  5,
						Disabled: false,
					},
				},
			},
			zoom: 2,
			expected: atlas.Map{
				Layers: []atlas.Layer{
					{
						Name:     "layer1",
						MinZoom:  0,
						MaxZoom:  2,
						Disabled: false,
					},
					{
						Name:     "layer2",
						MinZoom:  1,
						MaxZoom:  5,
						Disabled: false,
					},
				},
			},
		},
	}

	for i, tc := range testcases {
		output := tc.atlasMap.EnableLayersByZoom(tc.zoom)

		if !reflect.DeepEqual(output, tc.expected) {
			t.Errorf("testcase (%v) failed. output \n\n%+v\n\n does not match expected \n\n%+v", i, output, tc.expected)
		}
	}
}

func TestMapEnableLayersByName(t *testing.T) {
	testcases := []struct {
		atlasMap atlas.Map
		name     string
		expected atlas.Map
	}{
		{
			atlasMap: atlas.Map{
				Layers: []atlas.Layer{
					{
						Name:     "layer1",
						MinZoom:  0,
						MaxZoom:  2,
						Disabled: true,
					},
					{
						Name:     "layer2",
						MinZoom:  1,
						MaxZoom:  5,
						Disabled: true,
					},
				},
			},
			name: "layer1",
			expected: atlas.Map{
				Layers: []atlas.Layer{
					{
						Name:     "layer1",
						MinZoom:  0,
						MaxZoom:  2,
						Disabled: false,
					},
					{
						Name:     "layer2",
						MinZoom:  1,
						MaxZoom:  5,
						Disabled: true,
					},
				},
			},
		},
	}

	for i, tc := range testcases {
		output := tc.atlasMap.EnableLayersByName(tc.name)

		if !reflect.DeepEqual(output, tc.expected) {
			t.Errorf("testcase (%v) failed. output \n\n%+v\n\n does not match expected \n\n%+v", i, output, tc.expected)
		}
	}
}
