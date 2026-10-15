package features

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/alexeydott/geom"
	"github.com/alexeydott/tegola/provider"
)

func TestServiceDatetimeKnownAbsenceKeepsOtherConditions(t *testing.T) {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	end := start.Add(time.Hour)
	for name, temporal := range map[string]*provider.TemporalConstraint{
		"instant":  {Start: &start, End: &start},
		"interval": {Start: &start, End: &end},
		"open":     {Start: &start},
		"exact":    {Start: &start, End: &end, StartSubNanosecond: "123"},
	} {
		t.Run(name, func(t *testing.T) {
			query := provider.FeatureQuery{Limit: 1, Offset: 1, IDs: []uint64{2, 3}, Fields: []string{"name"}, Bounds: []geom.Extent{{10, 20, 30, 40}}, BoundsSRID: 4326, Temporal: temporal}
			original := query
			originalTemporal := *temporal
			constant, err := provider.NewFilterExpression(provider.FilterNode{Kind: provider.FilterBooleanConstant, Boolean: true})
			if err != nil {
				t.Fatal(err)
			}
			query.Filter = &constant
			original.Filter = &constant
			calls := 0
			s := newTestService(t, 4326, testQuerier(func(_ context.Context, layer string, received provider.FeatureQuery, fn func(*provider.Feature) error) (provider.FeatureQueryResult, error) {
				calls++
				expected := query
				expected.Temporal = nil
				if layer != "source" || !reflect.DeepEqual(received, expected) {
					t.Fatalf("non-temporal conditions changed: %+v", received)
				}
				if err := fn(&provider.Feature{ID: 3, SRID: 4326, Geometry: geom.Point{15, 30}, Tags: map[string]any{"name": "kept"}}); err != nil {
					return provider.FeatureQueryResult{}, err
				}
				count := uint64(2)
				return provider.FeatureQueryResult{NumberReturned: 1, NumberMatched: &count}, nil
			}))
			catalog, _ := provider.NewFeatureQueryables(nil)
			collection := s.collections["public"]
			collection.queryables, collection.queryablesAvailable = catalog, true
			s.collections["public"] = collection
			page, err := s.QueryCollectionPage(context.Background(), "public", query)
			if err != nil || calls != 1 || page.NumberReturned != 1 || page.NumberMatched == nil || *page.NumberMatched != 2 || page.Features[0].ID != 3 {
				t.Fatal(page, calls, err)
			}
			if !reflect.DeepEqual(query, original) || query.Temporal != temporal || !reflect.DeepEqual(*temporal, originalTemporal) {
				t.Fatal("caller temporal query mutated")
			}
		})
	}
}

func TestServiceDatetimeKnownAbsenceValidatesBeforeProvider(t *testing.T) {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	end := start.Add(-time.Second)
	calls := 0
	s := newTestService(t, 4326, testQuerier(func(context.Context, string, provider.FeatureQuery, func(*provider.Feature) error) (provider.FeatureQueryResult, error) {
		calls++
		return provider.FeatureQueryResult{}, nil
	}))
	for name, temporal := range map[string]*provider.TemporalConstraint{
		"both-open":       {},
		"reversed":        {Start: &start, End: &end},
		"orphan-fraction": {End: &end, StartSubNanosecond: "1"},
		"bad-fraction":    {Start: &start, StartSubNanosecond: "x"},
		"unknown-leap":    {Start: &start, StartLeapSecond: true},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := s.QueryCollectionPage(context.Background(), "public", provider.FeatureQuery{Limit: 1, Temporal: temporal})
			var invalid provider.InvalidFeatureQueryError
			if !errors.As(err, &invalid) || calls != 0 {
				t.Fatal("invalid datetime bypassed validation", calls, err)
			}
		})
	}
}
