package mvt_test

import (
	"reflect"
	"testing"

	"github.com/go-spatial/tegola/basic"
	"github.com/go-spatial/tegola/internal/p"
	"github.com/go-spatial/tegola/mvt"
)

func TestLayerAddFeatures(t *testing.T) {
	type tcase struct {
		features []mvt.Feature
		expected []mvt.Feature // nil means that it's the same as the features.
	}

	fn := func(tc tcase) func(t *testing.T) {
		return func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("did not expect AddFeatures to panic: recovered: %v", r)
				}
			}()

			// creat a new layer to add features to
			l := new(mvt.Layer)
			l.AddFeatures(tc.features...)

			gotFeatures := l.Features()

			expectedFeatures := tc.expected
			if expectedFeatures == nil {
				expectedFeatures = tc.features
			}

			if len(gotFeatures) != len(expectedFeatures) {
				t.Errorf("number of features incorrect. expected: %v got: %v", len(expectedFeatures), len(gotFeatures))
			}

			for i := range expectedFeatures {
				if !reflect.DeepEqual(expectedFeatures[i], gotFeatures[i]) {
					t.Errorf("expected feature %v to match. expected: %v got: %v", i, expectedFeatures[i], gotFeatures[i])
				}
			}
		}
	}

	tests := map[string]tcase{
		"nil id test 1": tcase{
			features: []mvt.Feature{
				{
					Tags:     map[string]interface{}{"btag": "tag"},
					Geometry: basic.Point{12.0, 15.0},
				},
				{
					ID:       p.Uint64(1),
					Tags:     map[string]interface{}{"atag": "tag"},
					Geometry: basic.Point{12.0, 15.0},
				},
			},
		},
		"nil id test 2": tcase{
			features: []mvt.Feature{
				{
					ID:       p.Uint64(1),
					Tags:     map[string]interface{}{"atag": "tag"},
					Geometry: basic.Point{12.0, 15.0},
				},
				{
					Tags:     map[string]interface{}{"btag": "tag"},
					Geometry: basic.Point{12.0, 15.0},
				},
			},
		},
		"same feature test": tcase{
			features: []mvt.Feature{
				{
					ID:       p.Uint64(1),
					Tags:     map[string]interface{}{"atag": "tag"},
					Geometry: basic.Point{12.0, 15.0},
				},
				{
					ID:       p.Uint64(1),
					Tags:     map[string]interface{}{"atag": "tag"},
					Geometry: basic.Point{12.0, 15.0},
				},
			},
			expected: []mvt.Feature{
				{
					ID:       p.Uint64(1),
					Tags:     map[string]interface{}{"atag": "tag"},
					Geometry: basic.Point{12.0, 15.0},
				},
				{
					ID:       p.Uint64(1),
					Tags:     map[string]interface{}{"atag": "tag"},
					Geometry: basic.Point{12.0, 15.0},
				},
			},
		},
		"different feature test": tcase{
			features: []mvt.Feature{
				{
					ID:       p.Uint64(1),
					Tags:     map[string]interface{}{"atag": "tag"},
					Geometry: basic.Point{12.0, 15.0},
				},
				{
					ID:       p.Uint64(2),
					Tags:     map[string]interface{}{"atag": "tag"},
					Geometry: basic.Point{12.0, 15.0},
				},
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, fn(tc))
	}
}
