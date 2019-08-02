package mvt

import (
	"context"
	"testing"

	"github.com/go-spatial/geom"
	"github.com/go-spatial/tegola/internal/p"
	vectorTile "github.com/go-spatial/tegola/mvt/vector_tile"
)

func TestLayer(t *testing.T) {
	type tcase struct {
		layer   *Layer
		vtlayer *vectorTile.Tile_Layer
		eerr    error
	}

	fn := func(tc tcase) func(t *testing.T) {
		return func(t *testing.T) {
			vt, err := tc.layer.VTileLayer(context.Background())
			if err != tc.eerr {
				t.Errorf("unexpected error, expected %v got %v", tc.eerr, err)
			}
			if tc.vtlayer == nil {
				if vt != nil {
					t.Errorf("expected nil value, got non-nil")
				}
				return
			}
			if vt == nil {
				t.Errorf("for a Vector Tile, Expected non-nil Got nil")
				return
			}
			if *tc.vtlayer.Version != *vt.Version {
				t.Errorf("versions do not match, expected %v got %v", *tc.vtlayer.Version, *vt.Version)
			}
			if *tc.vtlayer.Name != *vt.Name {
				t.Errorf("names do not match, expected %v got %v", *tc.vtlayer.Name, *vt.Name)
			}
			if *tc.vtlayer.Extent != *vt.Extent {
				t.Errorf("extent do not match, expected %v got %v", *tc.vtlayer.Extent, *vt.Extent)
			}
			if len(tc.vtlayer.Features) != len(vt.Features) {
				t.Errorf("features do not have the same length, expected %v got %v", len(tc.vtlayer.Features), len(vt.Features))
			}

			// TODO: Should check to see if the features are equal.
			if len(tc.vtlayer.Values) != len(vt.Values) {
				t.Errorf("values do not have the same length, expected %v got %v", len(tc.vtlayer.Values), len(vt.Values))
			}

			// TODO: Should check that the Values are equal.
		}
	}

	tests := map[string]tcase{
		"no features": {
			layer: &Layer{
				Name: "nofeatures",
			},
			vtlayer: &vectorTile.Tile_Layer{
				Version:  p.Uint32(Version),
				Name:     p.String("nofeatures"),
				Features: nil,
				Keys:     nil,
				Values:   nil,
				Extent:   p.Uint32(DefaultExtent),
			},
		},
		"one feature": {
			layer: &Layer{
				Name: "onefeature",
				features: []Feature{
					{
						Geometry: geom.Point{1, 1},
						Tags: map[string]interface{}{
							"tag1": "tag",
							"tag2": "tag",
						},
					},
				},
			},
			// features should not be nil, when we start comparing features this will fail.
			// But for now it's okay.
			vtlayer: &vectorTile.Tile_Layer{
				Version:  p.Uint32(Version),
				Name:     p.String("onefeature"),
				Features: []*vectorTile.Tile_Feature{nil},
				Keys:     []string{"tag1", "tag2"},
				Values:   []*vectorTile.Tile_Value{vectorTileValue("tag")},
				Extent:   p.Uint32(DefaultExtent),
			},
		},
		"two features": {
			layer: &Layer{
				Name: "twofeatures",
				features: []Feature{
					{
						Geometry: geom.Polygon{
							geom.LineString{
								geom.Point{3, 6},
								geom.Point{8, 12},
								geom.Point{20, 34},
							},
						},
						Tags: map[string]interface{}{
							"tag1": "tag",
							"tag2": "tag",
						},
					},
					{
						Geometry: geom.Point{1, 1},
						Tags: map[string]interface{}{
							"tag1": "tag",
							"tag2": "tag",
						},
					},
				},
			},
			// features should not be nil, when we start comparing features this will fail.
			// But for now it's okay.
			vtlayer: &vectorTile.Tile_Layer{
				Version:  p.Uint32(Version),
				Name:     p.String("twofeatures"),
				Features: []*vectorTile.Tile_Feature{nil, nil},
				Keys:     []string{"tag1", "tag2"},
				Values:   []*vectorTile.Tile_Value{vectorTileValue("tag1")},
				Extent:   p.Uint32(DefaultExtent),
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, fn(tc))
	}
}
