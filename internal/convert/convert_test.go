package convert

import (
	"testing"

	"github.com/terranodo/tegola/geom"
	"github.com/terranodo/tegola/geom/cmp"
)

func TestToTegolaToGeom(t *testing.T) {
	type tcase struct {
		geom geom.Geometry
		err  error
	}
	fn := func(t *testing.T, tc tcase) {
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
	tests := map[string]tcase{
		"Point Empty":           tcase{geom: geom.Point{}},
		"MultiPoint Empty":      tcase{geom: geom.MultiPoint{}},
		"LineString Empty":      tcase{geom: geom.LineString{}},
		"MultiLineString Empty": tcase{geom: geom.MultiLineString{}},
		"Polygon Empty":         tcase{geom: geom.Polygon{}},
		"MultiPolygon Empty":    tcase{geom: geom.MultiPolygon{}},
		"Point":                 tcase{geom: geom.Point{10, 10}},
		"MultiPoint":            tcase{geom: geom.MultiPoint{{10, 10}, {90, 90}, {20, 30}}},
		"LineString":            tcase{geom: geom.LineString{{10, 10}, {90, 90}, {20, 30}}},
		"MultiLineString": tcase{
			geom: geom.MultiLineString{
				{{10, 10}, {90, 90}, {20, 30}},
				{{10, 15}, {95, 90}, {25, 30}},
			},
		},
		"Polygon": tcase{
			geom: geom.Polygon{
				{{10, 10}, {90, 90}, {20, 30}},
				{{10, 15}, {95, 90}, {25, 30}},
			},
		},
		"MultiPolygon": tcase{
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
		tc := tc
		t.Run(name, func(t *testing.T) { fn(t, tc) })
	}
}
