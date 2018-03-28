package mvt

import (
	"fmt"
	"reflect"
	"testing"

	"context"

	"github.com/gdey/tbltest"
	"github.com/go-spatial/tegola"
	"github.com/go-spatial/tegola/basic"
	"github.com/go-spatial/tegola/mvt/vector_tile"
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

func TestLayerAddFeatures(t *testing.T) {
	type tc struct {
		features []Feature
		expected []Feature // Nil means that it's the same as the features.
		skipped  bool
	}
	fn := func(idx int, tcase tc) {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("[%v] did not expect AddFeatures to panic: recovered: %v", idx, r)
			}
		}()
		// First create a blank layer to add the features to.
		l := new(Layer)
		skipped := l.AddFeatures(tcase.features...)
		if tcase.skipped != skipped {
			t.Errorf("[%v] skipped value; expected: %v got: %v", idx, tcase.skipped, skipped)
		}
		gotFeatures := l.Features()
		expectedFeatures := tcase.expected
		if expectedFeatures == nil {
			expectedFeatures = tcase.features
		}
		if len(gotFeatures) != len(expectedFeatures) {
			t.Errorf("[%v] number of features incorrect. expected: %v got: %v", idx, len(expectedFeatures), len(gotFeatures))
		}
		for i := range expectedFeatures {
			if !reflect.DeepEqual(expectedFeatures[i], gotFeatures[i]) {
				t.Errorf("[%v] expected feature %v to match. expected: %v got: %v", idx, i, expectedFeatures[i], gotFeatures[i])
			}
		}
	}
	newID := func(id uint64) *uint64 { return &id }
	tbltest.Cases(
		// nil id test 1
		tc{
			features: []Feature{
				{
					Tags:     map[string]interface{}{"btag": "tag"},
					Geometry: basic.Point{12.0, 15.0},
				},
				{
					ID:       newID(1),
					Tags:     map[string]interface{}{"atag": "tag"},
					Geometry: basic.Point{12.0, 15.0},
				},
			},
		},
		// nil id test 2
		tc{
			features: []Feature{
				{
					ID:       newID(1),
					Tags:     map[string]interface{}{"atag": "tag"},
					Geometry: basic.Point{12.0, 15.0},
				},
				{
					Tags:     map[string]interface{}{"btag": "tag"},
					Geometry: basic.Point{12.0, 15.0},
				},
			},
		},
		// same feature test
		tc{
			features: []Feature{
				{
					ID:       newID(1),
					Tags:     map[string]interface{}{"atag": "tag"},
					Geometry: basic.Point{12.0, 15.0},
				},
				{
					ID:       newID(1),
					Tags:     map[string]interface{}{"atag": "tag"},
					Geometry: basic.Point{12.0, 15.0},
				},
			},
			expected: []Feature{
				{
					ID:       newID(1),
					Tags:     map[string]interface{}{"atag": "tag"},
					Geometry: basic.Point{12.0, 15.0},
				},
			},
			skipped: true,
		},
		// different feature test
		tc{
			features: []Feature{
				{
					ID:       newID(1),
					Tags:     map[string]interface{}{"atag": "tag"},
					Geometry: basic.Point{12.0, 15.0},
				},
				{
					ID:       newID(2),
					Tags:     map[string]interface{}{"atag": "tag"},
					Geometry: basic.Point{12.0, 15.0},
				},
			},
		},
	).Run(fn)
}

func TestLayer(t *testing.T) {
	tile := tegola.NewTile(0, 0, 0)

	//TODO: gdey — thing of a better way to build out features for a layer.
	fromPixel := func(x, y float64) *basic.Point {
		pt, err := tile.FromPixel(tegola.WebMercator, [2]float64{x, y})
		if err != nil {
			panic(fmt.Sprintf("error trying to convert %v,%v to WebMercator. %v", x, y, err))
		}
		bpt := basic.Point(pt)
		return &bpt
	}
	type tc struct {
		layer   *Layer
		vtlayer *vectorTile.Tile_Layer
		eerr    error
	}
	fn := func(i int, tcase tc) {
		vt, err := tcase.layer.VTileLayer(context.Background(), tile)
		if err != tcase.eerr {
			t.Errorf("[%v] unexpected error, Expected %v Got %v", i, tcase.eerr, err)
		}
		if tcase.vtlayer == nil {
			if vt != nil {
				t.Errorf("[%v] for VTileLayer, Expected nil value Got non-nil", i)
			}
			return
		}
		if vt == nil {
			t.Errorf("[%v] for a Vector Tile, Expected non-nil Got nil", i)
			return
		}
		if *tcase.vtlayer.Version != *vt.Version {
			t.Errorf("[%v] versions do not match, Expected %v Got %v", i, *tcase.vtlayer.Version, *vt.Version)
		}
		if *tcase.vtlayer.Name != *vt.Name {
			t.Errorf("[%v] names do not match, Expected %v Got %v", i, *tcase.vtlayer.Name, *vt.Name)
		}
		if *tcase.vtlayer.Extent != *vt.Extent {
			t.Errorf("[%v] extent do not match, Expected %v Got %v", i, *tcase.vtlayer.Extent, *vt.Extent)
		}
		if len(tcase.vtlayer.Features) != len(vt.Features) {
			t.Errorf("[%v] features do not have the same length, Expected %v Got %v", i, len(tcase.vtlayer.Features), len(vt.Features))
		}
		// TODO: Should check to see if the features are equal.
		if len(tcase.vtlayer.Values) != len(vt.Values) {
			t.Errorf("[%v] values do not have the same length, Expected %v Got %v", i, len(tcase.vtlayer.Values), len(vt.Values))
		}
		// TODO: Should check that the Values are equal.

	}

	tbltest.Cases(
		tc{
			layer: &Layer{
				Name: "nofeatures",
			},
			vtlayer: newTileLayer("nofeatures", nil, nil, nil),
		},
		tc{
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
		tc{
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
	).Run(fn)
}
