package mvt

import (
	"context"
	"fmt"
	"testing"

	"github.com/go-spatial/tegola"
	"github.com/go-spatial/tegola/basic"
	"github.com/go-spatial/tegola/internal/p"
	"github.com/go-spatial/tegola/mvt/vector_tile"
)

func newTileLayer(name string, keys []string, values []*vectorTile.Tile_Value, features []*vectorTile.Tile_Feature) *vectorTile.Tile_Layer {
	return &vectorTile.Tile_Layer{
		Version:  p.Uint32(Version),
		Name:     &name,
		Features: features,
		Keys:     keys,
		Values:   values,
		Extent:   p.Uint32(DefaultExtent),
	}
}

func TestLayer(t *testing.T) {
	tile := tegola.NewTile(0, 0, 0)

	// TODO: gdey — think of a better way to build out features for a layer.
	fromPixel := func(x, y float64) *basic.Point {
		pt, err := tile.FromPixel(tegola.WebMercator, [2]float64{x, y})
		if err != nil {
			panic(fmt.Sprintf("error trying to convert %v,%v to WebMercator. %v", x, y, err))
		}

		bpt := basic.Point(pt)
		return &bpt
	}

	type tcase struct {
		layer   *Layer
		vtlayer *vectorTile.Tile_Layer
		eerr    error
	}

	fn := func(tc tcase) func(t *testing.T) {
		return func(t *testing.T) {
			vt, err := tc.layer.VTileLayer(context.Background(), tile)
			if err != tc.eerr {
				t.Errorf("unexpected error, Expected %v Got %v", tc.eerr, err)
			}
			if tc.vtlayer == nil {
				if vt != nil {
					t.Errorf("expected nil value, Got non-nil")
				}
				return
			}
			if vt == nil {
				t.Errorf("for a Vector Tile, Expected non-nil Got nil")
				return
			}
			if *tc.vtlayer.Version != *vt.Version {
				t.Errorf("versions do not match, Expected %v Got %v", *tc.vtlayer.Version, *vt.Version)
			}
			if *tc.vtlayer.Name != *vt.Name {
				t.Errorf("names do not match, Expected %v Got %v", *tc.vtlayer.Name, *vt.Name)
			}
			if *tc.vtlayer.Extent != *vt.Extent {
				t.Errorf("extent do not match, Expected %v Got %v", *tc.vtlayer.Extent, *vt.Extent)
			}
			if len(tc.vtlayer.Features) != len(vt.Features) {
				t.Errorf("features do not have the same length, Expected %v Got %v", len(tc.vtlayer.Features), len(vt.Features))
			}

			// TODO: Should check to see if the features are equal.
			if len(tc.vtlayer.Values) != len(vt.Values) {
				t.Errorf("values do not have the same length, Expected %v Got %v", len(tc.vtlayer.Values), len(vt.Values))
			}

			// TODO: Should check that the Values are equal.
		}
	}

	tests := map[string]tcase{
		"1": tcase{
			layer: &Layer{
				Name: "nofeatures",
			},
			vtlayer: newTileLayer("nofeatures", nil, nil, nil),
		},
		"2": tcase{
			layer: &Layer{
				Name: "onefeature",
				features: []Feature{
					{
						Geometry: fromPixel(1, 1),
						Tags: map[string]interface{}{
							"tag1": "tag",
							"tag2": "tag",
						},
					},
				},
			},
			// features should not be nil, when we start comparing features this will fail.
			// But for now it's okay.
			vtlayer: newTileLayer("onefeature", []string{"tag1", "tag2"}, []*vectorTile.Tile_Value{vectorTileValue("tag")}, []*vectorTile.Tile_Feature{nil}),
		},
		"3": tcase{
			layer: &Layer{
				Name: "twofeature",
				features: []Feature{
					{
						Geometry: &basic.Polygon{
							basic.Line{
								*fromPixel(3, 6),
								*fromPixel(8, 12),
								*fromPixel(20, 34),
							},
						},
						Tags: map[string]interface{}{
							"tag1": "tag",
							"tag2": "tag",
						},
					},
					{
						Geometry: fromPixel(1, 1),
						Tags: map[string]interface{}{
							"tag1": "tag",
							"tag2": "tag",
						},
					},
				},
			},
			// features should not be nil, when we start comparing features this will fail.
			// But for now it's okay.
			vtlayer: newTileLayer("twofeature", []string{"tag1", "tag2"}, []*vectorTile.Tile_Value{vectorTileValue("tag1")}, []*vectorTile.Tile_Feature{nil, nil}),
		},
	}

	for name, tc := range tests {
		t.Run(name, fn(tc))
	}
}
