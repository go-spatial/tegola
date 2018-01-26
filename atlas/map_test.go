package atlas_test

import (
	"context"
	"log"
	"reflect"
	"testing"

	"github.com/gdey/tegola"
	"github.com/terranodo/tegola/atlas"
	"github.com/terranodo/tegola/geom/slippy"
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

func TestEncode(t *testing.T) {
	testcases := []struct {
		grid     atlas.Map
		tile     *slippy.Tile
		expected []byte
	}{
		{
			grid: atlas.Map{
				Layers: []atlas.Layer{
					{
						Name:     "layer1",
						MinZoom:  0,
						MaxZoom:  2,
						Disabled: true,
						Provider: &testTileProvider{},
					},
					{
						Name:     "layer2",
						MinZoom:  1,
						MaxZoom:  5,
						Disabled: true,
						Provider: &testTileProvider{},
					},
				},
			},
			tile:     slippy.NewTile(2, 3, 4, 64, tegola.WebMercator),
			expected: []byte{},
		},
	}

	for i, tc := range testcases {
		grid := tc.grid.EnableAllLayers()

		out, err := grid.Encode(context.Background(), tc.tile)
		if err != nil {
			log.Println("[%v] err: %v", i, err)
		}

		log.Println(out)
	}
}
