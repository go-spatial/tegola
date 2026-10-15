package provider

import (
	"context"
	"errors"
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/alexeydott/geom"
)

func TestFeatureQueryValidate(t *testing.T) {
	tests := []struct {
		name  string
		query FeatureQuery
		field string
	}{
		{name: "positive limit", query: FeatureQuery{Limit: 1}},
		{name: "zero limit", field: "limit"},
		{name: "largest safe offset", query: FeatureQuery{Limit: 1, Offset: math.MaxUint64 - 1}},
		{name: "larger limit safe offset", query: FeatureQuery{Limit: 50, Offset: math.MaxUint64 - 50}},
		{name: "larger limit overflow", query: FeatureQuery{Limit: 50, Offset: math.MaxUint64 - 49}, field: "offset"},
		{name: "offset overflow", query: FeatureQuery{Limit: 1, Offset: math.MaxUint64}, field: "offset"},
		{name: "srid without bounds", query: FeatureQuery{Limit: 1, BoundsSRID: 4326}, field: "bounds_srid"},
		{name: "bounds without srid", query: FeatureQuery{Limit: 1, Bounds: []geom.Extent{{0, 0, 1, 1}}}, field: "bounds_srid"},
		{name: "ordinary bounds", query: FeatureQuery{Limit: 1, BoundsSRID: 4326, Bounds: []geom.Extent{{-1, -2, 3, 4}}}},
		{name: "degenerate bounds", query: FeatureQuery{Limit: 1, BoundsSRID: 4326, Bounds: []geom.Extent{{1, 2, 1, 2}}}},
		{name: "bounds union", query: FeatureQuery{Limit: 1, BoundsSRID: 4326, Bounds: []geom.Extent{{-180, -90, -170, 90}, {170, -90, 180, 90}}}},
		{name: "reversed x", query: FeatureQuery{Limit: 1, BoundsSRID: 4326, Bounds: []geom.Extent{{3, 0, 1, 1}}}, field: "bounds"},
		{name: "reversed y", query: FeatureQuery{Limit: 1, BoundsSRID: 4326, Bounds: []geom.Extent{{0, 3, 1, 1}}}, field: "bounds"},
		{name: "empty fields", query: FeatureQuery{Limit: 1, Fields: []string{}}},
		{name: "nonempty fields", query: FeatureQuery{Limit: 1, Fields: []string{"name", "population"}}},
		{name: "blank field", query: FeatureQuery{Limit: 1, Fields: []string{" \t"}}, field: "fields"},
		{name: "empty field", query: FeatureQuery{Limit: 1, Fields: []string{""}}, field: "fields"},
		{name: "duplicate field", query: FeatureQuery{Limit: 1, Fields: []string{"name", "name"}}, field: "fields"},
	}
	for _, value := range []float64{math.NaN(), math.Inf(1), math.Inf(-1)} {
		for coordinate := range 4 {
			bounds := geom.Extent{0, 0, 1, 1}
			bounds[coordinate] = value
			tests = append(tests, struct {
				name  string
				query FeatureQuery
				field string
			}{
				name:  fmt.Sprintf("nonfinite %v coordinate %d", value, coordinate),
				query: FeatureQuery{Limit: 1, BoundsSRID: 4326, Bounds: []geom.Extent{bounds}}, field: "bounds",
			})
		}
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.query.Validate()
			if tc.field == "" {
				if err != nil {
					t.Fatal(err)
				}
				return
			}
			var invalid InvalidFeatureQueryError
			if !errors.As(err, &invalid) || invalid.Field != tc.field {
				t.Fatalf("want invalid field %q, got %v", tc.field, err)
			}
		})
	}
}

func TestTemporalConstraintValidate(t *testing.T) {
	instant := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	later := instant.Add(time.Second)
	otherZone := instant.In(time.FixedZone("offset", 3*60*60))
	var zero time.Time
	tests := []struct {
		name     string
		temporal TemporalConstraint
		invalid  bool
	}{
		{name: "instant", temporal: TemporalConstraint{Start: &instant, End: &instant}},
		{name: "equal different timezone", temporal: TemporalConstraint{Start: &instant, End: &otherZone}},
		{name: "bounded", temporal: TemporalConstraint{Start: &instant, End: &later}},
		{name: "reversed", temporal: TemporalConstraint{Start: &later, End: &instant}, invalid: true},
		{name: "open start", temporal: TemporalConstraint{End: &instant}},
		{name: "open end", temporal: TemporalConstraint{Start: &instant}},
		{name: "both open", invalid: true},
		{name: "zero time is a concrete bound", temporal: TemporalConstraint{Start: &zero, End: &zero}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			copyBound := func(bound *time.Time) *time.Time {
				if bound == nil {
					return nil
				}
				value := *bound
				return &value
			}
			temporal := TemporalConstraint{Start: copyBound(tc.temporal.Start), End: copyBound(tc.temporal.End)}
			startSnapshot := copyBound(temporal.Start)
			endSnapshot := copyBound(temporal.End)
			assertBoundUnchanged := func(name string, got, snapshot *time.Time) {
				t.Helper()
				if (got == nil) != (snapshot == nil) {
					t.Fatalf("%s pointer presence changed", name)
				}
				if got != nil && *got != *snapshot {
					t.Fatalf("%s timestamp changed", name)
				}
			}
			for _, validate := range []struct {
				name string
				run  func() error
			}{
				{name: "temporal", run: temporal.Validate},
				{name: "query", run: func() error { return (FeatureQuery{Limit: 1, Temporal: &temporal}).Validate() }},
			} {
				err := validate.run()
				assertBoundUnchanged("start", temporal.Start, startSnapshot)
				assertBoundUnchanged("end", temporal.End, endSnapshot)
				if !tc.invalid {
					if err != nil {
						t.Fatalf("%s validation: %v", validate.name, err)
					}
					continue
				}
				var invalid InvalidFeatureQueryError
				if !errors.As(err, &invalid) || invalid.Field != "temporal" {
					t.Fatalf("%s: want temporal validation error, got %v", validate.name, err)
				}
			}
		})
	}
}

func TestFeatureQueryErrors(t *testing.T) {
	var invalid InvalidFeatureQueryError
	if !errors.As(fmt.Errorf("outer: %w", InvalidFeatureQueryError{Field: "limit", Reason: "must be positive"}), &invalid) {
		t.Fatal("invalid query type lost through wrapping")
	}
	var missing FeatureLayerNotFoundError
	if !errors.As(fmt.Errorf("outer: %w", FeatureLayerNotFoundError{Layer: "roads"}), &missing) || missing.Layer != "roads" {
		t.Fatal("missing layer type lost through wrapping")
	}
	if !errors.Is(fmt.Errorf("outer: %w", ErrFeatureQueryUnsupported), ErrUnsupported) {
		t.Fatal("unsupported query must match ErrUnsupported")
	}
}

// These fixtures expose interfaces only; streaming behavior is tested by the reusable contract suite.
type queryCapableProvider struct {
	fakeStdPlain
	fakeMVTPlain
}

func (*queryCapableProvider) Layers() ([]LayerInfo, error) { return []LayerInfo{}, nil }
func (*queryCapableProvider) QueryFeatures(context.Context, string, FeatureQuery, func(*Feature) error) (FeatureQueryResult, error) {
	return FeatureQueryResult{}, ErrFeatureQueryUnsupported
}

func TestTilerUnionFeatureQuerier(t *testing.T) {
	capable := &queryCapableProvider{}
	tests := []struct {
		name      string
		union     TilerUnion
		supported bool
	}{
		{name: "standard capable", union: TilerUnion{Std: capable}, supported: true},
		{name: "ordinary standard", union: TilerUnion{Std: fakeStdPlain{}}},
		{name: "mvt only capable", union: TilerUnion{Mvt: capable}},
		{name: "mvt only ordinary", union: TilerUnion{Mvt: fakeMVTPlain{}}},
		{name: "nil union"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := tc.union.FeatureQuerier()
			if tc.supported {
				if err != nil || got != capable {
					t.Fatalf("want standard capability identity, got %v, %v", got, err)
				}
				return
			}
			if got != nil || !errors.Is(err, ErrUnsupported) {
				t.Fatalf("want unsupported nil capability, got %v, %v", got, err)
			}
		})
	}
}
