package features

import (
	"context"
	"errors"
	"math"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alexeydott/geom"
	"github.com/alexeydott/tegola/provider"
)

type metricQuerier struct {
	testQuerier
	metadata provider.FeatureQueryExecutionMetadata
	err      error
	reads    atomic.Int32
}

func (q *metricQuerier) FeatureQueryExecutionInfo() (provider.FeatureQueryExecutionMetadata, error) {
	q.reads.Add(1)
	return q.metadata, q.err
}

type metricObserver struct {
	mu     sync.Mutex
	events []QueryObservation
	panic  bool
}

func (o *metricObserver) ObserveFeatureQuery(event QueryObservation) {
	if o.panic {
		panic("private observer value")
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	o.events = append(o.events, event)
}

func TestFeatureQueryMetricsSnapshotAndOriginalClass(t *testing.T) {
	q := &metricQuerier{metadata: provider.FeatureQueryExecutionMetadata{Backend: provider.FeatureQueryBackendGPKG, ScalarFilter: provider.FeatureFilterExecutionSQL}}
	q.testQuerier = func(_ context.Context, _ string, query provider.FeatureQuery, fn func(*provider.Feature) error) (provider.FeatureQueryResult, error) {
		if query.Temporal != nil {
			t.Fatal("known absent temporal predicate forwarded")
		}
		if err := fn(&provider.Feature{ID: 1, SRID: 4326, Geometry: geom.Point{1, 2}}); err != nil {
			return provider.FeatureQueryResult{}, err
		}
		return provider.FeatureQueryResult{NumberReturned: 1}, nil
	}
	s := newMetricService(t, q)
	o := &metricObserver{}
	observed := s.WithQueryObserver(o)
	q.metadata = provider.FeatureQueryExecutionMetadata{}
	instant := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	query := provider.FeatureQuery{Limit: 1, Temporal: &provider.TemporalConstraint{Start: &instant, End: &instant}}
	_, err := observed.QueryCollectionPage(context.Background(), "public", query)
	if err != nil || q.reads.Load() != 1 || s.queryObserver != nil || query.Temporal == nil || len(o.events) != 1 {
		t.Fatal(err, q.reads.Load(), o.events)
	}
	event := o.events[0]
	if event.Class != QueryClassDatetime || event.Backend != provider.FeatureQueryBackendGPKG || event.Pushdown != QueryPushdownNone || event.RowsReturned != 1 || event.Outcome != QueryOutcomeOK || event.Duration < 0 {
		t.Fatal(event)
	}
}

func TestFeatureQueryMetricsPartialErrorsAndObserverIsolation(t *testing.T) {
	sentinel := errors.New("private provider failure")
	for name, failure := range map[string]error{
		"error": sentinel, "canceled": errors.Join(sentinel, context.Canceled), "deadline": errors.Join(sentinel, context.DeadlineExceeded),
		"source": provider.FeatureDataError{Err: sentinel}, "unsupported": provider.ErrUnsupported,
		"invalid": provider.InvalidFeatureQueryError{Field: "source", Reason: "invalid"},
	} {
		t.Run(name, func(t *testing.T) {
			o := &metricObserver{}
			s := newMetricService(t, testQuerier(func(_ context.Context, _ string, _ provider.FeatureQuery, fn func(*provider.Feature) error) (provider.FeatureQueryResult, error) {
				if err := fn(&provider.Feature{ID: 1, SRID: 4326, Geometry: geom.Point{1, 2}}); err != nil {
					return provider.FeatureQueryResult{}, err
				}
				return provider.FeatureQueryResult{NumberReturned: 1}, failure
			})).WithQueryObserver(o)
			_, err := s.QueryCollectionPage(context.Background(), "public", provider.FeatureQuery{Limit: 1})
			if !errors.Is(err, failure) || len(o.events) != 1 || o.events[0].RowsReturned != 1 || o.events[0].Outcome != queryOutcome(failure, false) {
				t.Fatal(err, o.events)
			}
			o.panic = true
			_, err = s.QueryCollectionPage(context.Background(), "public", provider.FeatureQuery{Limit: 1})
			if !errors.Is(err, failure) {
				t.Fatal("observer replaced provider error", err)
			}
		})
	}
}

func TestFeatureQueryMetricsCallbackAndBusinessPanic(t *testing.T) {
	for _, panicProvider := range []bool{false, true} {
		o := &metricObserver{}
		sentinel := errors.New("private callback failure")
		s := newMetricService(t, testQuerier(func(_ context.Context, _ string, _ provider.FeatureQuery, fn func(*provider.Feature) error) (provider.FeatureQueryResult, error) {
			if panicProvider {
				panic(sentinel)
			}
			return provider.FeatureQueryResult{}, fn(&provider.Feature{ID: 1, SRID: 4326, Geometry: geom.Point{1, 2}})
		})).WithQueryObserver(o)
		func() {
			defer func() {
				value := recover()
				if panicProvider && value != sentinel {
					t.Fatal("provider panic changed", value)
				}
			}()
			_, err := s.QueryCollection(context.Background(), "public", provider.FeatureQuery{Limit: 1}, func(Feature) error { return sentinel })
			if panicProvider || !errors.Is(err, sentinel) {
				t.Fatal(err)
			}
		}()
		want := QueryOutcomeCallbackError
		if panicProvider {
			want = QueryOutcomeError
		}
		if len(o.events) != 1 || o.events[0].Outcome != want || o.events[0].RowsReturned != 0 {
			t.Fatal(o.events)
		}
	}
}

func TestFeatureQueryMetricsSourceAndOutputValidationAreNotCallerFailures(t *testing.T) {
	for _, geometry := range []geom.Geometry{
		geom.Point{math.NaN(), 1},
		geom.PolygonZ{{{0, 0, 0}, {1, 0, 0}, {1, 1, 1}, {0, 1, 0}, {0, 0, 0}}},
	} {
		dimension := provider.DimensionXY
		if _, ok := geometry.(geom.PolygonZ); ok {
			dimension = provider.DimensionXYZ
		}
		s, _ := serviceCRSFixture(t, dimension, geometry)
		o := &metricObserver{}
		s = s.WithQueryObserver(o)
		_, err := s.QueryCollectionPage(context.Background(), "public", provider.FeatureQuery{Limit: 1})
		var source provider.FeatureDataError
		if !errors.As(err, &source) || len(o.events) != 1 || o.events[0].Outcome != QueryOutcomeSourceError || o.events[0].RowsReturned != 0 {
			t.Fatal("source failure mislabeled as caller callback", err, o.events)
		}
	}
	// The geographic pole is valid source data but cannot be represented by
	// the requested Mercator profile. The service's representation error is not
	// an error returned by the caller callback.
	s, _ := serviceCRSFixture(t, provider.DimensionXY, geom.Point{0, 90})
	o := &metricObserver{}
	s = s.WithQueryObserver(o)
	_, err := s.QueryCollectionPageWithOptions(context.Background(), "public", provider.FeatureQuery{Limit: 1}, QueryOptions{OutputCRS: "http://www.opengis.net/def/crs/EPSG/0/3857"})
	var invalid provider.InvalidFeatureQueryError
	if !errors.As(err, &invalid) || len(o.events) != 1 || o.events[0].Outcome != QueryOutcomeInvalid {
		t.Fatal(err, o.events)
	}
}

func TestFeatureQueryMetricsInvalidMetadataAndPreIO(t *testing.T) {
	for _, getterError := range []error{nil, provider.ErrUnsupported} {
		q := &metricQuerier{metadata: provider.FeatureQueryExecutionMetadata{Backend: 255, ScalarFilter: 255}, err: getterError, testQuerier: emptyQuerier}
		s := newMetricService(t, q)
		if s.collections["public"].execution != (provider.FeatureQueryExecutionMetadata{}) {
			t.Fatal("invalid optional metadata admitted")
		}
		o := &metricObserver{}
		s = s.WithQueryObserver(o)
		_, _ = s.QueryCollectionPage(context.Background(), "public", provider.FeatureQuery{})
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		_, _ = s.QueryCollectionPage(ctx, "public", provider.FeatureQuery{Limit: 1})
		if len(o.events) != 0 {
			t.Fatal("pre-I/O validation counted", o.events)
		}
	}
}

func TestFeatureQueryMetricsPushdownAndConcurrency(t *testing.T) {
	for _, execution := range []provider.FeatureFilterExecution{provider.FeatureFilterExecutionUnknown, provider.FeatureFilterExecutionSQL} {
		q := &metricQuerier{metadata: provider.FeatureQueryExecutionMetadata{Backend: provider.FeatureQueryBackendMySQL, ScalarFilter: execution}, testQuerier: emptyQuerier}
		s := newMetricService(t, q)
		catalog, _ := provider.NewFeatureQueryables(nil)
		collection := s.collections["public"]
		collection.queryables, collection.queryablesAvailable = catalog, true
		s.collections["public"] = collection
		o := &metricObserver{}
		s = s.WithQueryObserver(o)
		filter, _ := provider.NewFilterExpression(provider.FilterNode{Kind: provider.FilterBooleanConstant, Boolean: false})
		var wg sync.WaitGroup
		for i := 0; i < 16; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_, err := s.QueryCollectionPage(context.Background(), "public", provider.FeatureQuery{Limit: 1, Filter: &filter})
				if err != nil {
					t.Error(err)
				}
			}()
		}
		wg.Wait()
		want := QueryPushdownUnknown
		if execution == provider.FeatureFilterExecutionSQL {
			want = QueryPushdownSQLFilter
		}
		if len(o.events) != 16 || q.reads.Load() != 1 {
			t.Fatal(len(o.events), q.reads.Load())
		}
		for _, event := range o.events {
			if event.Class != QueryClassFilter || event.Pushdown != want {
				t.Fatal(event)
			}
		}
	}
}

func newMetricService(t *testing.T, querier provider.FeatureQuerier) *Service {
	t.Helper()
	service, err := NewService([]CollectionSource{{ID: "public", Layer: &testLayer{name: "source", srid: 4326}, Querier: querier}})
	if err != nil {
		t.Fatal(err)
	}
	return service
}
