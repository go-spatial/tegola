package atlas_test

import (
	"bytes"
	"compress/gzip"
	"context"
	"io"
	"reflect"
	"testing"

	"github.com/golang/protobuf/proto"

	vectorTile "github.com/go-spatial/geom/encoding/mvt/vector_tile"
	"github.com/go-spatial/geom/slippy"
	"github.com/go-spatial/tegola/atlas"
	"github.com/go-spatial/tegola/internal/p"
	"github.com/go-spatial/tegola/provider/test"
	"github.com/go-spatial/tegola/provider/test/emptycollection"
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
		{
			atlasMap: atlas.Map{
				Layers: []atlas.Layer{
					{
						Name:    "layer1",
						MinZoom: 0,
						MaxZoom: 0,
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
						MaxZoom: 0,
					},
					{
						Name:    "layer2",
						MinZoom: 1,
						MaxZoom: 5,
					},
				},
			},
			zoom: 0,
			expected: atlas.Map{
				Layers: []atlas.Layer{
					{
						Name:    "layer1",
						MinZoom: 0,
						MaxZoom: 0,
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

	type tcase struct {
		grid     atlas.Map
		tile     *slippy.Tile
		expected vectorTile.Tile
	}

	fn := func(tc tcase) func(t *testing.T) {
		return func(t *testing.T) {
			out, err := tc.grid.Encode(context.Background(), tc.tile)
			if err != nil {
				t.Errorf("err: %v", err)
				return
			}

			// decompress our output
			var buf bytes.Buffer
			r, err := gzip.NewReader(bytes.NewReader(out))
			if err != nil {
				t.Errorf("err: %v", err)
				return
			}

			_, err = io.Copy(&buf, r)
			if err != nil {
				t.Errorf("err: %v", err)
				return
			}

			var tile vectorTile.Tile

			if err = proto.Unmarshal(buf.Bytes(), &tile); err != nil {
				t.Errorf("error unmarshalling output: %v", err)
				return
			}

			// check the layer lengths match
			if len(tile.Layers) != len(tc.expected.Layers) {
				t.Errorf("expected (%d) layers, got (%d)", len(tc.expected.Layers), len(tile.Layers))
				return
			}

			for j, tileLayer := range tile.Layers {
				expectedLayer := tc.expected.Layers[j]

				if *tileLayer.Version != *expectedLayer.Version {
					t.Errorf("expected %v got %v", *tileLayer.Version, *expectedLayer.Version)
					return
				}

				if *tileLayer.Name != *expectedLayer.Name {
					t.Errorf("expected %v got %v", *tileLayer.Name, *expectedLayer.Name)
					return
				}

				// features check
				for k, tileLayerFeature := range tileLayer.Features {
					expectedTileLayerFeature := expectedLayer.Features[k]

					if *tileLayerFeature.Id != *expectedTileLayerFeature.Id {
						t.Errorf("expected %v got %v", *tileLayerFeature.Id, *expectedTileLayerFeature.Id)
						return
					}

					// the vector tile layer tags output is not always consistent since it's generated from a map.
					// because of that we're going to check everything but the tags values

					// if !reflect.DeepEqual(tileLayerFeature.Tags, expectedTileLayerFeature.Tags) {
					//  t.Errorf("expected %v got %v", tileLayerFeature.Tags, expectedTileLayerFeature.Tags)
					// 	return
					// }

					if *tileLayerFeature.Type != *expectedTileLayerFeature.Type {
						t.Errorf("expected %v got %v", *tileLayerFeature.Type, *expectedTileLayerFeature.Type)
						return
					}

					if !reflect.DeepEqual(tileLayerFeature.Geometry, expectedTileLayerFeature.Geometry) {
						t.Errorf("expected %v got %v", tileLayerFeature.Geometry, expectedTileLayerFeature.Geometry)
						return
					}
				}

				if len(tileLayer.Keys) != len(expectedLayer.Keys) {
					t.Errorf("layer keys length, expected %v got %v", len(expectedLayer.Keys), len(tileLayer.Keys))
					return
				}
				{
					var keysmaps = make(map[string]struct{})
					for _, k := range expectedLayer.Keys {
						keysmaps[k] = struct{}{}
					}
					var ferr bool
					for _, k := range tileLayer.Keys {
						if _, ok := keysmaps[k]; !ok {
							t.Errorf("did not find key, expected %v got nil", k)
							ferr = true
						}
					}
					if ferr {
						return
					}
				}

				if *tileLayer.Extent != *expectedLayer.Extent {
					t.Errorf("expected %v got %v", *tileLayer.Extent, *expectedLayer.Extent)
					return
				}

				if len(expectedLayer.Keys) != len(tileLayer.Keys) {
					t.Errorf("key len expected %v got %v", len(expectedLayer.Keys), len(tileLayer.Keys))
					return

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
					t.Errorf("constructed map expected %v got %v", expmap, gotmap)
				}
			}
		}
	}

	tests := map[string]tcase{
		"test_provider": {
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
		"empty_collection": {
			grid: atlas.Map{
				Layers: []atlas.Layer{
					{
						Name:     "empty_geom_collection",
						MinZoom:  0,
						MaxZoom:  2,
						Provider: &emptycollection.TileProvider{},
					},
				},
			},
			tile: slippy.NewTile(2, 3, 4),
			expected: vectorTile.Tile{
				Layers: []*vectorTile.Tile_Layer{
					{
						Version:  p.Uint32(2),
						Name:     p.String("empty_geom_collection"),
						Features: []*vectorTile.Tile_Feature{},
						Keys:     []string{},
						Values:   []*vectorTile.Tile_Value{},
						Extent:   p.Uint32(vectorTile.Default_Tile_Layer_Extent),
					},
				},
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, fn(tc))
	}
}
