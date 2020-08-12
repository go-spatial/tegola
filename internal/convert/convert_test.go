package convert

import (
	"testing"

	"github.com/go-spatial/geom"
	"github.com/go-spatial/geom/cmp"
)

func TestToTegolaToGeom(t *testing.T) {
	type tcase struct {
		geom geom.Geometry
		err  error
	}
	fn := func(tc tcase) func(*testing.T) {
		return func(t *testing.T) {
			t.Parallel()
			gott, err := ToTegola(tc.geom)
			if tc.err == nil && err != nil {
				t.Errorf("Unexpected error, expected nil got %v", err)
				return
			}
			if tc.err != nil && err == nil {
				t.Errorf("Unexpected error, expected %v got nil", tc.err)
				return
			}
			if tc.err != nil && err != nil && tc.err != err {
				t.Errorf("Unexpected error, expected %v got %v", tc.err, err)
				return
			}
			if tc.err != nil {
				return
			}
			t.Logf("Tegola geometry %v", gott)
			got, err := ToGeom(gott)
			if tc.err == nil && err != nil {
				t.Logf("Tegola geometry %#v", gott)
				t.Errorf("Unexpected error, expected nil got %v", err)
				return
			}

			if !cmp.GeometryEqual(tc.geom, got) {
				t.Errorf("unequal geometry, expected %v got %v", tc.geom, got)
			}
		}
	}
	tests := map[string]tcase{
		"Point Empty":           {geom: geom.Point{}},
		"MultiPoint Empty":      {geom: geom.MultiPoint{}},
		"LineString Empty":      {geom: geom.LineString{}},
		"MultiLineString Empty": {geom: geom.MultiLineString{}},
		"Polygon Empty":         {geom: geom.Polygon{}},
		"MultiPolygon Empty":    {geom: geom.MultiPolygon{}},
		"Point":                 {geom: geom.Point{10, 10}},
		"MultiPoint":            {geom: geom.MultiPoint{{10, 10}, {90, 90}, {20, 30}}},
		"LineString":            {geom: geom.LineString{{10, 10}, {90, 90}, {20, 30}}},
		"MultiLineString": {
			geom: geom.MultiLineString{
				{{10, 10}, {90, 90}, {20, 30}},
				{{10, 15}, {95, 90}, {25, 30}},
			},
		},
		"Polygon": {
			geom: geom.Polygon{
				{{10, 10}, {90, 90}, {20, 30}},
				{{10, 15}, {95, 90}, {25, 30}},
			},
		},
		"MultiPolygon": {
			geom: geom.MultiPolygon{
				{
					{{10, 10}, {90, 90}, {20, 30}},
					{{10, 15}, {95, 90}, {25, 30}},
				},
			},
		},
		/*
			// Collection support is broken. :(
			"Collection Empty":      tcase{geom: geom.Collection{}},
		*/
	}
	for name, tc := range tests {
		t.Run(name, fn(tc))
	}
}
