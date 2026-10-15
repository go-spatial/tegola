package geometrycodec_test

import (
	"math"
	"testing"

	"github.com/alexeydott/geom"
	"github.com/alexeydott/tegola/provider/geometrycodec"
)

func TestFeatureSpatialGeometryEmpty(t *testing.T) {
	cases := []struct {
		name     string
		geometry geom.Geometry
		empty    bool
	}{
		{name: "nil", empty: true},
		{name: "XY multipoint", geometry: geom.MultiPoint{}, empty: true},
		{name: "XYZ multipoint", geometry: geom.MultiPointZ{}, empty: true},
		{name: "XY line", geometry: geom.LineString{}, empty: true},
		{name: "XYZ line", geometry: geom.LineStringZ{}, empty: true},
		{name: "XY multiline", geometry: geom.MultiLineString{{}, {}}, empty: true},
		{name: "XYZ multiline", geometry: geom.MultiLineStringZ{{}, {}}, empty: true},
		{name: "XY polygon", geometry: geom.Polygon{}, empty: true},
		{name: "XYZ polygon", geometry: geom.PolygonZ{}, empty: true},
		{name: "XY multipolygon", geometry: geom.MultiPolygon{{}, {}}, empty: true},
		{name: "XYZ multipolygon", geometry: geometrycodec.MultiPolygonZ{{}, {}}, empty: true},
		{name: "typed nil", geometry: geom.Collection(nil), empty: true},
		{name: "nested empty", geometry: geom.Collection{nil, geom.Collection{geom.LineStringZ{}, geom.Polygon{}}}, empty: true},
		{name: "XY zero point", geometry: geom.Point{}},
		{name: "XYZ zero point", geometry: geom.PointZ{}},
		{name: "mixed populated", geometry: geom.Collection{geom.LineString{}, geom.Collection{geom.PointZ{1, 2, 3}}}},
		{name: "populated multiline", geometry: geom.MultiLineStringZ{{}, {{1, 2, 3}, {4, 5, 6}}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := geometrycodec.ValidateFeatureSpatialGeometry(tc.geometry); err != nil {
				t.Fatalf("fixture must be valid: %v", err)
			}
			if got := geometrycodec.FeatureSpatialGeometryEmpty(tc.geometry); got != tc.empty {
				t.Fatalf("empty = %v, want %v", got, tc.empty)
			}
		})
	}
}

func TestFeatureSpatialGeometryEmptyDoesNotValidate(t *testing.T) {
	invalid := geom.Collection{geom.LineStringZ{}, geom.PointZ{math.NaN(), 2, 3}}
	if geometrycodec.FeatureSpatialGeometryEmpty(invalid) {
		t.Fatal("populated malformed child must not be hidden as empty")
	}
	if err := geometrycodec.ValidateFeatureSpatialGeometry(invalid); err == nil {
		t.Fatal("validation must independently reject malformed child")
	}
	invalidRing := geom.PolygonZ{{}}
	if geometrycodec.FeatureSpatialGeometryEmpty(invalidRing) {
		t.Fatal("malformed populated polygon container must not be hidden")
	}
	if err := geometrycodec.ValidateFeatureSpatialGeometry(invalidRing); err == nil {
		t.Fatal("validation must independently reject empty ring")
	}
}
