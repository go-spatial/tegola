package geometrycodec

import (
	"errors"
	"math"
	"reflect"
	"testing"

	"github.com/alexeydott/geom"
)

func TestTransformFeatureSpatialGeometry(t *testing.T) {
	input := geom.Collection{geom.Point{1, 2}, geom.LineStringZ{{0, 0, 7}, {1, 1, 9}}, MultiPolygonZ{{{{0, 0, 5}, {1, 0, 5}, {0, 1, 5}}}}}
	transform := func(p [2]float64) ([2]float64, error) { return [2]float64{p[0] + 10, p[1] - 20}, nil }
	out, err := TransformFeatureSpatialGeometry(input, transform)
	if err != nil {
		t.Fatal(err)
	}
	want := geom.Collection{geom.Point{11, -18}, geom.LineStringZ{{10, -20, 7}, {11, -19, 9}}, MultiPolygonZ{{{{10, -20, 5}, {11, -20, 5}, {10, -19, 5}}}}}
	if !reflect.DeepEqual(out, want) {
		t.Fatalf("got %v want %v", out, want)
	}
	out.(geom.Collection)[1].(geom.LineStringZ)[0][2] = 77
	out.(geom.Collection)[2].(MultiPolygonZ)[0][0][0][0] = 88
	if input[1].(geom.LineStringZ)[0][2] != 7 || input[2].(MultiPolygonZ)[0][0][0][0] != 0 {
		t.Fatal("transformed output aliases source")
	}
}

func TestTransformFeatureSpatialValidation(t *testing.T) {
	identity := func(p [2]float64) ([2]float64, error) { return p, nil }
	input := geom.PolygonZ{{{0, 0, 0}, {1, 0, 1}, {1, 1, 2}, {0, 1, 1}}}
	nonlinear := func(p [2]float64) ([2]float64, error) { return [2]float64{p[0], p[1] * (p[0] + 1)}, nil }
	if _, err := TransformFeatureSpatialGeometry(input, nonlinear); !errors.Is(err, ErrUnsupportedFeatureSpatialGeometry) {
		t.Fatalf("accepted transformed nonplanar surface: %v", err)
	}
	if _, err := TransformFeatureSpatialGeometry(geom.Collection{geom.Point{0, 0}, geom.PointZ{0, 0, math.NaN()}}, identity); err == nil {
		t.Fatal("accepted invalid child")
	}
	badOutput := func(p [2]float64) ([2]float64, error) { return [2]float64{math.Inf(1), p[1]}, nil }
	if _, err := TransformFeatureSpatialGeometry(geom.PointZ{0, 0, 1}, badOutput); err == nil {
		t.Fatal("accepted nonfinite output")
	}
	sentinel := errors.New("callback failure")
	fail := func([2]float64) ([2]float64, error) { return [2]float64{}, sentinel }
	if _, err := TransformFeatureSpatialGeometry(geom.Point{0, 0}, fail); !errors.Is(err, sentinel) {
		t.Fatalf("lost error chain: %v", err)
	}
	if _, err := TransformFeatureSpatialGeometry(nil, nil); err == nil {
		t.Fatal("accepted nil transform")
	}
}
