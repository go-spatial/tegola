package mvt

import (
	"testing"

	"github.com/terranodo/tegola/basic"
	"github.com/terranodo/tegola/mvt/vector_tile"
)

func newTileLayer(name string, keys []string, values []*vectorTile.Tile_Value, features []*vectorTile.Tile_Feature) *vectorTile.Tile_Layer {
	version := uint32(2)
	extent := uint32(4096)
	return &vectorTile.Tile_Layer{
		Version:  &version,
		Name:     &name,
		Features: features,
		Keys:     keys,
		Values:   values,
		Extent:   &extent,
	}
}

func TestLayer(t *testing.T) {
	testcases := []struct {
		layer   *Layer
		vtlayer *vectorTile.Tile_Layer
		eerr    error
	}{
		{
			layer: &Layer{
				Name: "nofeatures",
			},
			vtlayer: newTileLayer("nofeatures", nil, nil, nil),
		},
		{
			layer: &Layer{
				Name: "onefeature",
				features: []Feature{
					{
						Geometry: &basic.Point{1, 1},
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
	}
	for i, tcase := range testcases {
		vt, err := tcase.layer.VTileLayer()
		if err != tcase.eerr {
			t.Errorf("For Test %v: Got unexpected error. Expected %v Got %v", i, tcase.eerr, err)
		}
		if tcase.vtlayer == nil {
			if vt != nil {
				t.Errorf("For Test %v: Got a non-nil value when we expected a nil value.", i)
			}
			continue
		}
		if vt == nil {
			t.Errorf("For Test %v: Expected to get a Vector Tile, got nil instead.", i)
			continue
		}
		if *tcase.vtlayer.Version != *vt.Version {
			t.Errorf("For Test %v: Versions do not match, Expected %v Got %v.", i, *tcase.vtlayer.Version, *vt.Version)
		}
		if *tcase.vtlayer.Name != *vt.Name {
			t.Errorf("For Test %v: Names do not match, Expected %v Got %v.", i, *tcase.vtlayer.Name, *vt.Name)
		}
		if *tcase.vtlayer.Extent != *vt.Extent {
			t.Errorf("For Test %v: Extent do not match, Expected %v Got %v.", i, *tcase.vtlayer.Extent, *vt.Extent)
		}
		if len(tcase.vtlayer.Features) != len(vt.Features) {
			t.Errorf("For Test %v: Features do not have the same length, Expected %v Got %v.", i, len(tcase.vtlayer.Features), len(vt.Features))
		}
		// TODO: Should check to see if the features are equal.
		if len(tcase.vtlayer.Values) != len(vt.Values) {
			t.Errorf("For Test %v: Values do not have the same length, Expected %v Got %v.", i, len(tcase.vtlayer.Values), len(vt.Values))
		}
		// TODO: Should check that the Values are equal.

	}
}
