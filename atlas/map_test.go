package atlas_test

import (
	"context"
	"reflect"
	"testing"

	"github.com/golang/protobuf/proto"

	"github.com/go-spatial/geom/slippy"
	"github.com/go-spatial/tegola/atlas"
	"github.com/go-spatial/tegola/internal/p"
	"github.com/go-spatial/tegola/mvt/vector_tile"
	"github.com/go-spatial/tegola/provider/test"
)

func TestMapFilterLayersByZoom(t *testing.T) {
	testcases := []struct {
		atlasMap atlas.Map
		zoom     uint
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
						Provider: &test.TileProvider{},
						DefaultTags: map[string]interface{}{
							"foo": "bar",
						},
					},
					{
						Name:     "layer2",
						MinZoom:  1,
						MaxZoom:  5,
						Provider: &test.TileProvider{},
					},
				},
			},
			tile: slippy.NewTile(2, 3, 4),
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

		for j, tileLayer := range tile.Layers {
			expectedLayer := tc.expected.Layers[j]

			if *tileLayer.Version != *expectedLayer.Version {
				t.Errorf("[%v] expected %v got %v", i, *tileLayer.Version, *expectedLayer.Version)
				continue
			}

			if *tileLayer.Name != *expectedLayer.Name {
				t.Errorf("[%v] expected %v got %v", i, *tileLayer.Name, *expectedLayer.Name)
				continue
			}

			// features check
			for k, tileLayerFeature := range tileLayer.Features {
				expectedTileLayerFeature := expectedLayer.Features[k]

				if *tileLayerFeature.Id != *expectedTileLayerFeature.Id {
					t.Errorf("[%v] expected %v got %v", i, *tileLayerFeature.Id, *expectedTileLayerFeature.Id)
					continue
				}

				/*
					// the vector tile layer tags output is not always consistent since it's generated from a map.
					// because of that we're going to check everything but the tags values

					if !reflect.DeepEqual(tileLayerFeature.Tags, expectedTileLayerFeature.Tags) {
						t.Errorf("[%v] expected %v got %v", i, tileLayerFeature.Tags, expectedTileLayerFeature.Tags)
						continue
					}
				*/

				if *tileLayerFeature.Type != *expectedTileLayerFeature.Type {
					t.Errorf("[%v] expected %v got %v", i, *tileLayerFeature.Type, *expectedTileLayerFeature.Type)
					continue
				}

				if !reflect.DeepEqual(tileLayerFeature.Geometry, expectedTileLayerFeature.Geometry) {
					t.Errorf("[%v] expected %v got %v", i, tileLayerFeature.Geometry, expectedTileLayerFeature.Geometry)
					continue
				}
			}

			if len(tileLayer.Keys) != len(expectedLayer.Keys) {
				t.Errorf("[%v] layer keys length, expected %v got %v", i, len(expectedLayer.Keys), len(tileLayer.Keys))
				continue
			}
			{
				var keysmaps = make(map[string]struct{})
				for _, k := range expectedLayer.Keys {
					keysmaps[k] = struct{}{}
				}
				var ferr bool
				for _, k := range tileLayer.Keys {
					if _, ok := keysmaps[k]; !ok {
						t.Errorf("[%v] did not find key, expected %v got nil", i, k)
						ferr = true
					}
				}
				if ferr {
					continue
				}
			}

			if *tileLayer.Extent != *expectedLayer.Extent {
				t.Errorf("[%v] expected %v got %v", i, *tileLayer.Extent, *expectedLayer.Extent)
				continue
			}

			if len(expectedLayer.Keys) != len(tileLayer.Keys) {
				t.Errorf("[%v] key len expected %v got %v", i, len(expectedLayer.Keys), len(tileLayer.Keys))
				continue

			}

			var gotmap = make(map[string]interface{})
			var expmap = make(map[string]interface{})
			for i, k := range tileLayer.Keys {
				gotmap[k] = tileLayer.Values[i]
			}
			for i, k := range expectedLayer.Keys {
				expmap[k] = expectedLayer.Values[i]
			}
			if !reflect.DeepEqual(expmap, gotmap) {
				t.Errorf("[%v] constructed map expected %v got %v", i, expmap, gotmap)
			}

		}
	}
}
