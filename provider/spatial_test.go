package provider

import (
	"errors"
	"math"
	"reflect"
	"testing"

	"github.com/alexeydott/geom"
)

func TestDimensionalFeatureQueryValidation(t *testing.T) {
	valid := FeatureQuery{Bounds3D: []Extent3D{{1, 2, 3, 1, 2, 3}}, BoundsSRID: 4326, BoundsVerticalCRS: CRS84h, Limit: 1}
	if err := valid.Validate(); err != nil {
		t.Fatal(err)
	}
	before := append([]Extent3D(nil), valid.Bounds3D...)
	for _, change := range []func(*FeatureQuery){
		func(q *FeatureQuery) { q.BoundsSRID = 0 },
		func(q *FeatureQuery) { q.BoundsVerticalCRS = "" },
		func(q *FeatureQuery) { q.Bounds3D = nil },
		func(q *FeatureQuery) { q.Bounds3D = []Extent3D{{0, 0, 2, 1, 1, 1}} },
		func(q *FeatureQuery) { q.Bounds3D = []Extent3D{{0, 0, math.NaN(), 1, 1, 2}} },
		func(q *FeatureQuery) { q.Bounds3D = []Extent3D{{math.Inf(1), 0, 0, math.Inf(1), 1, 2}} },
		func(q *FeatureQuery) { q.Bounds3D = []Extent3D{{2, 0, 0, 1, 1, 2}} },
		func(q *FeatureQuery) { q.Bounds3D = []Extent3D{{0, 2, 0, 1, 1, 2}} },
		func(q *FeatureQuery) { q.Bounds = []geom.Extent{{0, 0, 1, 1}} },
	} {
		q := valid
		change(&q)
		err := q.Validate()
		var invalid InvalidFeatureQueryError
		if !errors.As(err, &invalid) {
			t.Fatalf("expected typed invalid query error: %v", err)
		}
	}
	if !reflect.DeepEqual(before, valid.Bounds3D) {
		t.Fatal("validation mutated caller bounds")
	}
	q := FeatureQuery{Bounds: []geom.Extent{{0, 0, 1, 1}}, BoundsSRID: 4326, BoundsVerticalCRS: CRS84h, Limit: 1}
	if err := q.Validate(); err == nil {
		t.Fatal("vertical CRS accepted without XYZ bounds")
	}
}

func TestSpatialMetadataValidation(t *testing.T) {
	for _, m := range []SpatialMetadata{{Dimension: DimensionXY}, {Dimension: DimensionXYZ, VerticalCRS: CRS84h}, {Dimension: DimensionMixedXYXYZ, VerticalCRS: CRS84h}} {
		if err := m.Validate(); err != nil {
			t.Fatal(err)
		}
	}
	for _, m := range []SpatialMetadata{{}, {Dimension: DimensionXY, VerticalCRS: CRS84h}, {Dimension: DimensionXYZ}, {Dimension: DimensionXYZ, VerticalCRS: "orthometric"}} {
		if err := m.Validate(); err == nil {
			t.Fatal("invalid metadata accepted")
		}
	}
}
