package atlas_test

import (
	"context"
	"reflect"
	"testing"

	"github.com/gdey/tegola"
	"github.com/golang/protobuf/proto"
	"github.com/terranodo/tegola/atlas"
	"github.com/terranodo/tegola/geom/slippy"
	"github.com/terranodo/tegola/mvt/vector_tile"
)

func TestMapFilterLayersByZoom(t *testing.T) {
	testcases := []struct {
		atlasMap atlas.Map
		zoom     int
		expected atlas.Map
	}{
		{
			atlasMap: atlas.Map{
				Layers: []atlas.Layer{
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
			zoom: 5,
			expected: atlas.Map{
				Layers: []atlas.Layer{
					{
						Name:    "layer2",
						MinZoom: 1,
						MaxZoom: 5,
					},
				},
			},
		},
		{
			atlasMap: atlas.Map{
				Layers: []atlas.Layer{
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
			expected: atlas.Map{
				Layers: []atlas.Layer{
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
		},
	}

	for i, tc := range testcases {
		output := tc.atlasMap.FilterLayersByZoom(tc.zoom)

		if !reflect.DeepEqual(output, tc.expected) {
			t.Errorf("testcase (%v) failed. output \n\n%+v\n\n does not match expected \n\n%+v", i, output, tc.expected)
		}
	}
}

func TestMapFilterLayersByName(t *testing.T) {
	testcases := []struct {
		grid     atlas.Map
		name     string
		expected atlas.Map
	}{
		{
			grid: atlas.Map{
				Layers: []atlas.Layer{
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
			expected: atlas.Map{
				Layers: []atlas.Layer{
					{
						Name:    "layer1",
						MinZoom: 0,
						MaxZoom: 2,
					},
				},
			},
		},
	}

	for i, tc := range testcases {
		output := tc.grid.FilterLayersByName(tc.name)

		if !reflect.DeepEqual(output, tc.expected) {
			t.Errorf("testcase (%v) failed. output \n\n%+v\n\n does not match expected \n\n%+v", i, output, tc.expected)
		}
	}
}

func TestEncode(t *testing.T) {
	testcases := []struct {
		grid           atlas.Map
		tile           *slippy.Tile
		expectedLayers []string
	}{
		{
			grid: atlas.Map{
				Layers: []atlas.Layer{
					{
						Name:     "layer1",
						MinZoom:  0,
						MaxZoom:  2,
						Provider: &testTileProvider{},
					},
					{
						Name:     "layer2",
						MinZoom:  1,
						MaxZoom:  5,
						Provider: &testTileProvider{},
					},
				},
			},
			tile:           slippy.NewTile(2, 3, 4, 64, tegola.WebMercator),
			expectedLayers: []string{"layer1", "layer2"},
		},
	}

	for i, tc := range testcases {
		out, err := tc.grid.Encode(context.Background(), tc.tile)
		if err != nil {
			t.Errorf("[%v] err: %v", i, err)
			continue
		}

		//	success check
		if len(tc.expectedLayers) > 0 {
			var tile vectorTile.Tile

			if err = proto.Unmarshal(out, &tile); err != nil {
				t.Errorf("[%v] error unmarshalling output: %v", i, err)
				continue
			}

			var tileLayers []string
			//	extract all the layers names in the response
			for i := range tile.Layers {
				tileLayers = append(tileLayers, *tile.Layers[i].Name)
			}

			if !reflect.DeepEqual(tc.expectedLayers, tileLayers) {
				t.Errorf("[%v] expected layers (%v) got (%v)", i, tc.expectedLayers, tileLayers)
				continue
			}
		}
	}
}
