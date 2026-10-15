package querytest

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/alexeydott/geom"
	"github.com/alexeydott/tegola/provider"
)

// Negative controls prove the checker rejects observed bad provider behavior.
func TestCheckerRejectsBrokenImplementations(t *testing.T) {
	tests := []struct {
		name     string
		c        caseSpec
		behavior func(context.Context, provider.FeatureQuery, func(*provider.Feature) error) (provider.FeatureQueryResult, error)
	}{
		{name: "dropped cancellation", c: caseSpec{preCancel: true}, behavior: func(context.Context, provider.FeatureQuery, func(*provider.Feature) error) (provider.FeatureQueryResult, error) {
			return provider.FeatureQueryResult{}, nil
		}},
		{name: "incorrect count", c: caseSpec{ids: []uint64{}, matched: 0}, behavior: func(context.Context, provider.FeatureQuery, func(*provider.Feature) error) (provider.FeatureQueryResult, error) {
			count := uint64(1)
			return provider.FeatureQueryResult{NumberMatched: &count}, nil
		}},
		{name: "false unknown", c: caseSpec{ids: []uint64{}, matched: 0}, behavior: func(context.Context, provider.FeatureQuery, func(*provider.Feature) error) (provider.FeatureQueryResult, error) {
			return provider.FeatureQueryResult{}, nil
		}},
		{name: "duplicate callbacks", c: caseSpec{ids: []uint64{10}, matched: 1}, behavior: func(ctx context.Context, q provider.FeatureQuery, fn func(*provider.Feature) error) (provider.FeatureQueryResult, error) {
			count := uint64(1)
			f := provider.Feature{ID: 10}
			if err := fn(&f); err != nil {
				return provider.FeatureQueryResult{}, err
			}
			if err := fn(&f); err != nil {
				return provider.FeatureQueryResult{}, err
			}
			return provider.FeatureQueryResult{NumberReturned: 2, NumberMatched: &count}, nil
		}},
		{name: "query ownership", c: caseSpec{query: provider.FeatureQuery{IDs: []uint64{10}}, ids: []uint64{}, matched: 0}, behavior: func(ctx context.Context, q provider.FeatureQuery, fn func(*provider.Feature) error) (provider.FeatureQueryResult, error) {
			q.IDs[0] = 999
			count := uint64(0)
			return provider.FeatureQueryResult{NumberMatched: &count}, nil
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tc.c.query.Limit = 100
			instance := Instance{Querier: brokenQuerier{run: tc.behavior}, Layer: "features", CountMode: ExactCount}
			if len(checkCase(instance, tc.c)) == 0 {
				t.Fatal("checker accepted deliberately broken implementation")
			}
		})
	}
}

type brokenQuerier struct {
	run func(context.Context, provider.FeatureQuery, func(*provider.Feature) error) (provider.FeatureQueryResult, error)
}

func (b brokenQuerier) QueryFeatures(ctx context.Context, layer string, q provider.FeatureQuery, fn func(*provider.Feature) error) (provider.FeatureQueryResult, error) {
	return b.run(ctx, q, fn)
}

func TestCheckerRejectsConcurrentCallbacks(t *testing.T) {
	bad := brokenQuerier{run: func(ctx context.Context, q provider.FeatureQuery, fn func(*provider.Feature) error) (provider.FeatureQueryResult, error) {
		start := make(chan struct{})
		var wg sync.WaitGroup
		for _, id := range []uint64{10, 20} {
			wg.Add(1)
			go func(id uint64) {
				defer wg.Done()
				<-start
				if err := fn(&provider.Feature{ID: id}); err != nil {
					return
				}
			}(id)
		}
		close(start)
		wg.Wait()
		count := uint64(2)
		return provider.FeatureQueryResult{NumberReturned: 2, NumberMatched: &count}, nil
	}}
	c := caseSpec{serialCallbacks: true, query: provider.FeatureQuery{Limit: 2}, ids: []uint64{10, 20}, matched: 2}
	issues := checkCase(Instance{Querier: bad, Layer: "features", CountMode: ExactCount}, c)
	for _, issue := range issues {
		if strings.Contains(issue, "callbacks must be serial") {
			return
		}
	}
	t.Fatal("checker did not identify concurrent callbacks")
}

func TestConcurrentCheckerCollectsManyFailures(t *testing.T) {
	bad := brokenQuerier{run: func(context.Context, provider.FeatureQuery, func(*provider.Feature) error) (provider.FeatureQueryResult, error) {
		wrongCount := uint64(999)
		return provider.FeatureQueryResult{NumberReturned: 999, NumberMatched: &wrongCount}, nil
	}}
	issues := checkConcurrent(Instance{Querier: bad, Layer: "features", CountMode: ExactCount})
	if len(issues) <= 32 {
		t.Fatalf("want more than 32 diagnostics without deadlock, got %d", len(issues))
	}
}

func TestOptionalMatchedCount(t *testing.T) {
	zero := uint64(0)
	wrong := uint64(1)
	c := caseSpec{ids: []uint64{}, matched: 0}
	for _, count := range []*uint64{nil, &zero} {
		if issues := checkResponse(provider.FeatureQueryResult{NumberMatched: count}, []provider.Feature{}, c, OptionalCount); len(issues) != 0 {
			t.Fatalf("optional count rejected: %v", issues)
		}
	}
	if issues := checkResponse(provider.FeatureQueryResult{NumberMatched: &wrong}, []provider.Feature{}, c, OptionalCount); len(issues) == 0 {
		t.Fatal("optional count accepted wrong known total")
	}
}

func TestCheckerRejectsLateCallbacks(t *testing.T) {
	for _, lateCall := range []int{1, 2} {
		t.Run(fmt.Sprintf("query %d", lateCall), func(t *testing.T) {
			bad := &lateQuerier{lateCall: lateCall}
			c := caseSpec{query: provider.FeatureQuery{Limit: 1}, ids: []uint64{}, matched: 0}
			issues := checkCase(Instance{Querier: bad, Layer: "features", CountMode: ExactCount}, c)
			if bad.lateError == nil {
				t.Fatal("late callback did not get rejection")
			}
			for _, issue := range issues {
				if strings.Contains(issue, "callback observed after") {
					return
				}
			}
			t.Fatal("checker did not observe deterministic late callback")
		})
	}
}

type lateQuerier struct {
	calls, lateCall int
	release         chan struct{}
	done            chan error
	lateError       error
}

func (q *lateQuerier) QueryFeatures(ctx context.Context, layer string, query provider.FeatureQuery, fn func(*provider.Feature) error) (provider.FeatureQueryResult, error) {
	q.calls++
	q.release = nil
	if q.calls == q.lateCall {
		release := make(chan struct{})
		done := make(chan error, 1)
		q.release = release
		q.done = done
		go func() { <-release; done <- fn(&provider.Feature{ID: 10}) }()
	}
	zero := uint64(0)
	return provider.FeatureQueryResult{NumberMatched: &zero}, nil
}
func (q *lateQuerier) queryReturned() {
	if q.release != nil {
		close(q.release)
		q.lateError = <-q.done
	}
}

func TestCheckerRejectsNestedGeometryOwnership(t *testing.T) {
	fixture := Fixture{Rows: []Row{{Feature: provider.Feature{ID: 10, SRID: 4326, Tags: map[string]any{"name": "nested"}, Geometry: geom.Collection{geom.Collection{geom.LineString{{0, 0}, {1, 1}}}, geom.Polygon{{{0, 0}, {2, 0}, {2, 2}, {0, 0}}}}}}}}
	stored := cloneFeature(fixture.Rows[0].Feature)
	bad := brokenQuerier{run: func(ctx context.Context, q provider.FeatureQuery, fn func(*provider.Feature) error) (provider.FeatureQueryResult, error) {
		f := stored // Deliberately shares nested mutable geometry with stored state.
		f.Tags = map[string]any{"name": "nested"}
		if err := fn(&f); err != nil {
			return provider.FeatureQueryResult{}, err
		}
		one := uint64(1)
		return provider.FeatureQueryResult{NumberReturned: 1, NumberMatched: &one}, nil
	}}
	c := caseSpec{fixture: fixture, query: provider.FeatureQuery{Limit: 1}, ids: []uint64{10}, matched: 1, mutateDelivery: true}
	issues := checkCase(Instance{Querier: bad, Layer: "features", CountMode: ExactCount}, c)
	for _, issue := range issues {
		if strings.Contains(issue, "shared feature") || strings.Contains(issue, "values/geometry") {
			return
		}
	}
	t.Fatal("checker accepted nested shared geometry")
}

func TestSetupErrorsAreClassified(t *testing.T) {
	validFailure := Instance{SetupError: fmt.Errorf("register: %w", provider.InvalidFeatureQueryError{Field: "temporal", Reason: "invalid mapping"})}
	if issues, handled := checkSetup(validFailure, caseSpec{invalidMapping: true}); !handled || len(issues) != 0 {
		t.Fatalf("typed setup rejection not accepted: %v", issues)
	}
	if issues, handled := checkSetup(validFailure, caseSpec{}); !handled || len(issues) == 0 {
		t.Fatal("normal fixture setup error accepted")
	}
	if issues, handled := checkSetup(Instance{SetupError: errors.New("unclassified setup failure")}, caseSpec{invalidMapping: true}); !handled || len(issues) == 0 {
		t.Fatal("invalid mapping setup error lost classification")
	}
	if issues, handled := checkSetup(Instance{}, caseSpec{}); !handled || len(issues) == 0 {
		t.Fatal("missing querier accepted without setup error")
	}
}

func TestGeometryRingClosureComparison(t *testing.T) {
	closed := geom.Collection{geom.Polygon{{{0, 0}, {4, 0}, {4, 4}, {0, 0}}, {{1, 1}, {2, 1}, {2, 2}, {1, 1}}, {{3, 3}, {3.5, 3}, {3.5, 3.5}, {3, 3}}}}
	implicit := geom.Collection{geom.Polygon{{{0, 0}, {4, 0}, {4, 4}}, {{1, 1}, {2, 1}, {2, 2}}, {{3, 3}, {3.5, 3}, {3.5, 3.5}}}}
	before := cloneGeometry(closed)
	if !reflect.DeepEqual(normalizeRingClosure(closed), normalizeRingClosure(implicit)) {
		t.Fatal("equivalent ring closure rejected")
	}
	if !reflect.DeepEqual(closed, before) {
		t.Fatal("ring normalization mutated fixture")
	}
	changed := cloneGeometry(implicit).(geom.Collection)
	changed[0].(geom.Polygon)[0][1][0] = 4.1
	if reflect.DeepEqual(normalizeRingClosure(closed), normalizeRingClosure(changed)) {
		t.Fatal("changed coordinate accepted")
	}
	reordered := cloneGeometry(implicit).(geom.Collection)
	polygon := reordered[0].(geom.Polygon)
	polygon[1], polygon[2] = polygon[2], polygon[1]
	if reflect.DeepEqual(normalizeRingClosure(closed), normalizeRingClosure(reordered)) {
		t.Fatal("changed hole order accepted")
	}
}
