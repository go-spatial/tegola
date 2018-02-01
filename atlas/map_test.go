package atlas_test

import (
	"context"
	"reflect"
	"testing"

	"github.com/arolek/p"
	"github.com/golang/protobuf/proto"

	"github.com/terranodo/tegola"
	"github.com/terranodo/tegola/atlas"
	"github.com/terranodo/tegola/geom/slippy"
	"github.com/terranodo/tegola/mvt/vector_tile"
	"github.com/terranodo/tegola/provider/test_provider"
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
	// create vars for the vector tile types so we can take their addresses
	// unknown := vectorTile.Tile_UNKNOWN
	// point := vectorTile.Tile_POINT
	// linestring := vectorTile.Tile_LINESTRING
	polygon := vectorTile.Tile_POLYGON

	testcases := []struct {
		grid     atlas.Map
		tile     *slippy.Tile
		expected vectorTile.Tile
	}{
		{
			grid: atlas.Map{
				Layers: []atlas.Layer{
					{
						Name:     "layer1",
						MinZoom:  0,
						MaxZoom:  2,
						Provider: &test_provider.TestTileProvider{},
						DefaultTags: map[string]interface{}{
							"foo": "bar",
						},
					},
					{
						Name:     "layer2",
						MinZoom:  1,
						MaxZoom:  5,
						Provider: &test_provider.TestTileProvider{},
					},
				},
			},
			tile: slippy.NewTile(2, 3, 4, 64, tegola.WebMercator),
			expected: vectorTile.Tile{
				Layers: []*vectorTile.Tile_Layer{
					{
						Version: p.Uint32(2),
						Name:    p.String("layer1"),
						Features: []*vectorTile.Tile_Feature{
							{
								Id:       p.Uint64(0),
								Tags:     []uint32{0, 0, 1, 1},
								Type:     &polygon,
								Geometry: []uint32{9, 0, 0, 26, 8192, 0, 0, 8192, 8191, 0, 15},
							},
						},
						Keys: []string{"type", "foo"},
						Values: []*vectorTile.Tile_Value{
							{
								StringValue: p.String("debug_buffer_outline"),
							},
							{
								StringValue: p.String("bar"),
							},
						},
						Extent: p.Uint32(vectorTile.Default_Tile_Layer_Extent),
					},
					{
						Version: p.Uint32(2),
						Name:    p.String("layer2"),
						Features: []*vectorTile.Tile_Feature{
							{
								Id:       p.Uint64(0),
								Tags:     []uint32{0, 0},
								Type:     &polygon,
								Geometry: []uint32{9, 0, 0, 26, 8192, 0, 0, 8192, 8191, 0, 15},
							},
						},
						Keys: []string{"type"},
						Values: []*vectorTile.Tile_Value{
							{
								StringValue: p.String("debug_buffer_outline"),
							},
						},
						Extent: p.Uint32(vectorTile.Default_Tile_Layer_Extent),
					},
				},
			},
		},
	}

	for i, tc := range testcases {
		out, err := tc.grid.Encode(context.Background(), tc.tile)
		if err != nil {
			t.Errorf("[%v] err: %v", i, err)
			continue
		}

		var tile vectorTile.Tile

		if err = proto.Unmarshal(out, &tile); err != nil {
			t.Errorf("[%v] error unmarshalling output: %v", i, err)
			continue
		}

		if !reflect.DeepEqual(tc.expected, tile) {
			t.Errorf("[%v] expected %v got %v", i, tc.expected, tile)
			continue
		}
	}
}
