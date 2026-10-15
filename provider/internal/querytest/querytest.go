// Package querytest runs backend-independent raw-feature contract checks.
// A reference adapter proves this harness works; backend adapters provide parity evidence.
package querytest

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alexeydott/geom"
	"github.com/alexeydott/tegola/provider"
)

// CountMode declares whether an adapter promises exact or unknown matched totals.
type CountMode uint8

const (
	ExactCount CountMode = iota
	UnknownCount
	// OptionalCount permits unknown totals; any supplied count must still be exact.
	OptionalCount
)

// Row is an immutable fixture descriptor, not a provider query extension.
// EmptyGeometry requires storage's empty geometry to decode to nil. MalformedGeometry
// requires corrupt explicitly selected WKB/native storage and a decode failure.
// MissingID and Metadata rows must not become invented features.
type Row struct {
	Feature       provider.Feature
	Start, End    *time.Time
	EmptyGeometry bool
	// RawEmptyGeometry is literal raw source wire to encode, independently of
	// Feature.Geometry's expected nil after whole-geometry empty normalization.
	RawEmptyGeometry geom.Geometry
	// RawTemporal is literal source ticks and requires declared TemporalStorage.
	RawTemporal       *RawTemporalIntegers
	MalformedGeometry string
	MissingID         bool
	Metadata          bool
	// ExcludedBySelection marks a stored row outside the declared custom domain.
	ExcludedBySelection bool
}

// Fixture describes isolated storage and temporal mapping for one contract case.
// InvalidTemporalMapping must be rejected at registration with a typed invalid-query
// error through Instance.SetupError, or by the query where registration cannot inspect it.
type Fixture struct {
	// FilterFields declares the literal public filter schema only for filter suites.
	FilterFields           []provider.FeatureQueryable
	NativeProfileCase      NativeProfileCase
	Rows                   []Row
	InvalidTemporalMapping bool
	Profile                FixtureProfile
	// Spatial is explicit for dimensional fixtures; zero preserves old XY setup.
	Spatial provider.SpatialMetadata
	// TemporalStorage is explicit for exact-time suites; zero preserves legacy setup.
	TemporalStorage TemporalPropertyStorage
	// PublicFields is the fixture-wide selected public property output set.
	// Non-nil empty explicitly selects none; nil is legacy unspecified metadata.
	PublicFields []string
}

// Instance owns one fresh backend fixture. Cleanup is optional and registered by Run.
type Instance struct {
	// QueryableLayer must be the actual registration-frozen provider layer.
	QueryableLayer                  provider.FeatureQueryableLayerInfo
	NativeProfileOutcome            *NativeProfileEvidence
	nativeRingOrientationEquivalent bool
	// SetupError exposes constructor rejection without a fake querier.
	SetupError error
	Querier    provider.FeatureQuerier
	Layer      string
	Cleanup    func()
	CountMode  CountMode
	// StorageRejectedNativeCorruption describes an actual native storage attempt
	// checked by the backend adapter. It never replaces the raw query oracle.
	StorageRejectedNativeCorruption *NativeStorageEvidence
}

// Factory materializes an isolated fixture with explicit IDs, CRS84 coordinates,
// properties and optional temporal geometry. Register provider resources via Cleanup
// or t.Cleanup. A backend must not omit cases by substituting a mock adapter.
type Factory func(t *testing.T, fixture Fixture) Instance

// Run runs the required feature contract cases against a real or declared reference adapter.
func Run(t *testing.T, factory Factory) {
	RunWithOptions(t, factory, Options{})
}

// RunWithOptions declares fixture property expectations independently of the
// provider under test. Run's ordinary private temporal-read oracle is unchanged.
func RunWithOptions(t *testing.T, factory Factory, options Options) {
	runProfile(t, factory, options, OrdinaryTable)
}

func runProfile(t *testing.T, factory Factory, options Options, fixtureProfile FixtureProfile) {
	t.Helper()
	factory = withNativeRingOrientation(withPublicFieldDeclaration(factory), options.NativeRingOrientationEquivalent)
	profile, err := copyTemporalPropertyProfile(options.PublicTemporalProperties)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range profileCases(fixtureProfile) {
		t.Run(c.name, func(t *testing.T) {
			if profile != nil {
				fixture, err := withPublicTemporalProperties(c.fixture, *profile)
				if err != nil {
					t.Fatal(err)
				}
				c.fixture = fixture
			}
			instance := factory(t, cloneFixture(c.fixture))
			registerInstanceCleanup(t, instance)
			if runNativeStorageOutcome(t, instance, c) {
				return
			}
			if setupIssues, handled := checkSetup(instance, c); handled {
				for _, issue := range setupIssues {
					t.Error(issue)
				}
				return
			}
			if instance.CountMode != ExactCount && instance.CountMode != UnknownCount && instance.CountMode != OptionalCount {
				t.Fatal("invalid count mode")
			}
			for _, issue := range checkCase(instance, c) {
				t.Error(issue)
			}
		})
	}
	t.Run("concurrent isolated results", func(t *testing.T) {
		fixture := canonicalFixture()
		fixture.Profile = fixtureProfile
		if fixtureProfile == CustomSelection {
			fixture = customDomainFixture()
		}
		if profile != nil {
			profiled, err := withPublicTemporalProperties(fixture, *profile)
			if err != nil {
				t.Fatal(err)
			}
			fixture = profiled
		}
		instance := factory(t, cloneFixture(fixture))
		registerInstanceCleanup(t, instance)
		if runNativeStorageOutcome(t, instance, caseSpec{fixture: fixture}) {
			return
		}
		if instance.SetupError != nil {
			t.Fatalf("concurrent fixture setup: %v", instance.SetupError)
		}
		if instance.Querier == nil {
			t.Fatal("factory returned nil querier")
		}
		for _, issue := range checkConcurrentWithFixture(instance, fixture) {
			t.Error(issue)
		}

	})
}

func registerInstanceCleanup(t *testing.T, instance Instance) {
	t.Helper()
	if instance.Cleanup != nil {
		t.Cleanup(instance.Cleanup)
	}
}

func checkSetup(instance Instance, c caseSpec) ([]string, bool) {
	if instance.NativeProfileOutcome != nil || c.fixture.NativeProfileCase != 0 {
		return []string{"unsolicited native profile outcome outside dedicated runner"}, true
	}
	if instance.SetupError != nil {
		if c.invalidMapping {
			var invalid provider.InvalidFeatureQueryError
			if errors.As(instance.SetupError, &invalid) {
				return []string{}, true
			}
			return []string{"invalid temporal mapping setup lost typed invalid-query error"}, true
		}
		return []string{fmt.Sprintf("unexpected fixture setup error: %v", instance.SetupError)}, true
	}
	if instance.Querier == nil {
		return []string{"factory returned nil querier without setup error"}, true
	}
	return nil, false
}

func checkConcurrent(instance Instance) []string {
	return checkConcurrentWithFixture(instance, canonicalFixture())
}

func checkConcurrentWithFixture(instance Instance, fixture Fixture) []string {
	var wg sync.WaitGroup
	batches := make(chan []string, 8)
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ids := []uint64{30, 10, 20}
			if fixture.Profile == CustomSelection {
				ids = append(ids, 90)
			}
			c := caseSpec{name: "concurrent", fixture: fixture, query: provider.FeatureQuery{Limit: 2, IDs: ids}, ids: []uint64{10, 20}, matched: 3, more: true}
			batches <- checkCase(instance, c)
		}()
	}
	wg.Wait()
	close(batches)
	issues := []string{}
	for batch := range batches {
		issues = append(issues, batch...)
	}
	return issues
}

type caseSpec struct {
	nativeRingOrientationEquivalent bool
	name                            string
	fixture                         Fixture
	query                           provider.FeatureQuery
	ids                             []uint64
	matched                         uint64
	more                            bool
	preCancel                       bool
	deadline                        bool
	cancelAfterFirst                bool
	callbackFailure                 bool
	nilCallback                     bool
	missingLayer                    bool
	malformed                       bool
	sourceData                      bool
	invalidQuery                    bool
	invalidMapping                  bool
	mutateDelivery                  bool
	serialCallbacks                 bool
}

func canonicalFixture() Fixture {
	start := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	end := start.Add(2 * time.Hour)
	first := start.Add(-2 * time.Hour)
	middle := start.Add(time.Hour)
	return Fixture{Rows: []Row{
		{Feature: provider.Feature{ID: 30, Geometry: geom.Point{2, 2}, SRID: 4326, Tags: map[string]any{"name": "third", "value": int64(30)}}, Start: &middle, End: &end},
		{Feature: provider.Feature{ID: 10, Geometry: geom.Point{0, 0}, SRID: 4326, Tags: map[string]any{"name": "first", "value": int64(10)}}, Start: &first, End: &start},
		{Feature: provider.Feature{ID: 40, Geometry: geom.Point{10, 10}, SRID: 4326, Tags: map[string]any{"name": "fourth", "value": int64(40)}}},
		{Feature: provider.Feature{ID: 20, Geometry: geom.Point{1, 1}, SRID: 4326, Tags: map[string]any{"name": "second", "value": int64(20)}}, Start: &start, End: &middle},
	}}
}

func contractCases() []caseSpec {
	f := canonicalFixture()
	all := []uint64{10, 20, 30, 40}
	cases := []caseSpec{
		{name: "all", ids: all, matched: 4},
		{name: "bbox inside", query: provider.FeatureQuery{Bounds: []geom.Extent{{0.5, 0.5, 1.5, 1.5}}, BoundsSRID: 4326}, ids: []uint64{20}, matched: 1},
		{name: "bbox outside", query: provider.FeatureQuery{Bounds: []geom.Extent{{20, 20, 30, 30}}, BoundsSRID: 4326}, ids: []uint64{}, matched: 0},
		{name: "bbox boundary", query: provider.FeatureQuery{Bounds: []geom.Extent{{0, 0, 1, 1}}, BoundsSRID: 4326}, ids: []uint64{10, 20}, matched: 2},
		{name: "bbox degenerate", query: provider.FeatureQuery{Bounds: []geom.Extent{{1, 1, 1, 1}}, BoundsSRID: 4326}, ids: []uint64{20}, matched: 1},
		{name: "bbox union dedup before paging", query: provider.FeatureQuery{Bounds: []geom.Extent{{0, 0, 2, 2}, {1, 1, 10, 10}}, BoundsSRID: 4326, Limit: 2, Offset: 1}, ids: []uint64{20, 30}, matched: 4, more: true},
		{name: "single ID", query: provider.FeatureQuery{IDs: []uint64{20}}, ids: []uint64{20}, matched: 1},
		{name: "multiple duplicate IDs", query: provider.FeatureQuery{IDs: []uint64{30, 10, 30}}, ids: []uint64{10, 30}, matched: 2},
		{name: "missing ID", query: provider.FeatureQuery{IDs: []uint64{999}}, ids: []uint64{}, matched: 0},
		{name: "limit and lookahead", query: provider.FeatureQuery{Limit: 1}, ids: []uint64{10}, matched: 4, more: true},
		{name: "stable page one", query: provider.FeatureQuery{Limit: 2}, ids: []uint64{10, 20}, matched: 4, more: true},
		{name: "stable page two", query: provider.FeatureQuery{Limit: 2, Offset: 2}, ids: []uint64{30, 40}, matched: 4},
		{name: "offset beyond matches", query: provider.FeatureQuery{Limit: 2, Offset: 10}, ids: []uint64{}, matched: 4},
		{name: "fields preserve identity and geometry", query: provider.FeatureQuery{Fields: []string{"name"}}, ids: all, matched: 4},
		{name: "pre canceled", preCancel: true},
		{name: "expired deadline", deadline: true},
		{name: "cancel during callback", cancelAfterFirst: true},
		{name: "wrapped callback failure", callbackFailure: true},
		{name: "nil callback", nilCallback: true},
		{name: "missing layer", missingLayer: true},
		{name: "serial callbacks", ids: all, matched: 4, serialCallbacks: true},
		{name: "callback ownership", ids: all, matched: 4, mutateDelivery: true},
	}
	for i := range cases {
		cases[i].fixture = f
	}
	cases = append(cases, caseSpec{name: "empty layer known zero or unknown", fixture: Fixture{Rows: []Row{}}, ids: []uint64{}, matched: 0})
	start := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	end := start.Add(time.Hour)
	zoned := start.In(time.FixedZone("+03", 3*3600))
	beforeStart := start.Add(-time.Nanosecond)
	afterStart := start.Add(time.Nanosecond)
	afterEnd := end.Add(time.Nanosecond)
	temporal := []caseSpec{
		{name: "temporal 1ns before source boundary", query: provider.FeatureQuery{Temporal: &provider.TemporalConstraint{Start: &beforeStart, End: &beforeStart}}, ids: []uint64{10, 40}, matched: 2},
		{name: "temporal 1ns after source boundary", query: provider.FeatureQuery{Temporal: &provider.TemporalConstraint{Start: &afterStart, End: &afterStart}}, ids: []uint64{20, 40}, matched: 2},
		{name: "temporal 1ns after second boundary", query: provider.FeatureQuery{Temporal: &provider.TemporalConstraint{Start: &afterEnd, End: &afterEnd}}, ids: []uint64{30, 40}, matched: 2},
		{name: "temporal instant boundary and absence", query: provider.FeatureQuery{Temporal: &provider.TemporalConstraint{Start: &start, End: &start}}, ids: []uint64{10, 20, 40}, matched: 3},
		{name: "temporal bounded interval", query: provider.FeatureQuery{Temporal: &provider.TemporalConstraint{Start: &start, End: &end}}, ids: all, matched: 4},
		{name: "temporal open start", query: provider.FeatureQuery{Temporal: &provider.TemporalConstraint{End: &start}}, ids: []uint64{10, 20, 40}, matched: 3},
		{name: "temporal open end", query: provider.FeatureQuery{Temporal: &provider.TemporalConstraint{Start: &end}}, ids: []uint64{20, 30, 40}, matched: 3},
		{name: "temporal equivalent timezone", query: provider.FeatureQuery{Temporal: &provider.TemporalConstraint{Start: &zoned, End: &zoned}}, ids: []uint64{10, 20, 40}, matched: 3},
		{name: "bbox time ID and paging", query: provider.FeatureQuery{Temporal: &provider.TemporalConstraint{Start: &start, End: &start}, Bounds: []geom.Extent{{0, 0, 2, 2}}, BoundsSRID: 4326, IDs: []uint64{30, 20, 10}, Limit: 1, Offset: 1}, ids: []uint64{20}, matched: 2},
	}
	for i := range temporal {
		temporal[i].fixture = f
	}
	cases = append(cases, temporal...)
	nullRows := []Row{
		{Feature: provider.Feature{ID: 10, SRID: 4326, Tags: map[string]any{"name": "null"}}, Start: &start, End: &start},
		{Feature: provider.Feature{ID: 20, SRID: 4326, Tags: map[string]any{"name": "empty"}}, EmptyGeometry: true},
	}
	cases = append(cases,
		caseSpec{name: "null and decoded empty geometry", fixture: Fixture{Rows: nullRows}, ids: []uint64{10, 20}, matched: 2},
		caseSpec{name: "absent geometry matches nonintersecting bbox", fixture: Fixture{Rows: nullRows}, query: provider.FeatureQuery{Bounds: []geom.Extent{{100, 100, 101, 101}}, BoundsSRID: 4326}, ids: []uint64{10, 20}, matched: 2},
		caseSpec{name: "absent geometry still respects ID time paging", fixture: Fixture{Rows: nullRows}, query: provider.FeatureQuery{Bounds: []geom.Extent{{100, 100, 101, 101}}, BoundsSRID: 4326, Temporal: &provider.TemporalConstraint{Start: &end, End: &end}, IDs: []uint64{10, 20}, Limit: 1}, ids: []uint64{20}, matched: 1},
		caseSpec{name: "metadata and null ID not invented", fixture: Fixture{Rows: []Row{{Feature: provider.Feature{ID: 10}, MissingID: true}, {Feature: provider.Feature{ID: 20}, Metadata: true}}}, ids: []uint64{}, matched: 0},
		caseSpec{name: "GeometryCollection preserved", mutateDelivery: true, fixture: Fixture{Rows: []Row{{Feature: provider.Feature{ID: 10, Geometry: geom.Collection{geom.Point{0, 0}, geom.Point{2, 2}}, SRID: 4326, Tags: map[string]any{"name": "collection"}}}}}, ids: []uint64{10}, matched: 1},
		caseSpec{name: "GeometryCollection bbox boundary", fixture: Fixture{Rows: []Row{{Feature: provider.Feature{ID: 10, Geometry: geom.Collection{geom.Point{0, 0}, geom.Point{2, 2}}, SRID: 4326, Tags: map[string]any{"name": "collection"}}}}}, query: provider.FeatureQuery{Bounds: []geom.Extent{{2, 2, 2, 2}}, BoundsSRID: 4326}, ids: []uint64{10}, matched: 1},
		caseSpec{name: "GeometryCollection bbox gap", fixture: Fixture{Rows: []Row{{Feature: provider.Feature{ID: 10, Geometry: geom.Collection{geom.Point{0, 0}, geom.Point{2, 2}}, SRID: 4326, Tags: map[string]any{"name": "collection"}}}}}, query: provider.FeatureQuery{Bounds: []geom.Extent{{1, 1, 1, 1}}, BoundsSRID: 4326}, ids: []uint64{}, matched: 0},
		caseSpec{name: "nested mutable geometry ownership", mutateDelivery: true, fixture: Fixture{Rows: []Row{{Feature: provider.Feature{ID: 10, SRID: 4326, Tags: map[string]any{"name": "nested"}, Geometry: geom.Collection{geom.Collection{geom.LineString{{0, 0}, {1, 1}}}, geom.Polygon{{{0, 0}, {2, 0}, {2, 2}, {0, 0}}}}}}}}, ids: []uint64{10}, matched: 1},
		caseSpec{name: "malformed explicit WKB", fixture: Fixture{Rows: []Row{{Feature: provider.Feature{ID: 10}, MalformedGeometry: "wkb"}}}, malformed: true},
		caseSpec{name: "malformed explicit native", fixture: Fixture{Rows: []Row{{Feature: provider.Feature{ID: 10}, MalformedGeometry: "native"}}}, malformed: true},
		caseSpec{name: "invalid temporal mapping", fixture: Fixture{Rows: f.Rows, InvalidTemporalMapping: true}, query: provider.FeatureQuery{Temporal: &provider.TemporalConstraint{Start: &start, End: &end}}, invalidMapping: true},
	)
	for i := range cases {
		if cases[i].query.Limit == 0 {
			cases[i].query.Limit = 100
		}
	}
	return cases
}

func checkCase(instance Instance, c caseSpec) []string {
	c.nativeRingOrientationEquivalent = instance.nativeRingOrientationEquivalent
	issues := []string{}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if c.preCancel {
		cancel()
	}
	if c.deadline {
		var stop context.CancelFunc
		ctx, stop = context.WithDeadline(ctx, time.Now().Add(-time.Second))
		defer stop()
	}
	query := cloneQuery(c.query)
	before := cloneQuery(query)
	delivered := []provider.Feature{}
	var callbackMutex sync.Mutex
	var active atomic.Int32
	callbackIssues := []string{}
	callbackErr := errors.New("contract callback sentinel")
	callback := func(f *provider.Feature) error {
		concurrent := active.Add(1) > 1
		defer active.Add(-1)
		if c.serialCallbacks {
			time.Sleep(time.Millisecond * 5)
		}
		callbackMutex.Lock()
		defer callbackMutex.Unlock()
		if concurrent {
			callbackIssues = append(callbackIssues, "callbacks must be serial")
		}
		if f == nil {
			callbackIssues = append(callbackIssues, "nil feature callback")
			return errors.New("nil feature")
		}
		delivered = append(delivered, cloneFeature(*f))
		if c.mutateDelivery {
			f.Tags["name"] = "mutated by callback"
			mutateGeometry(f.Geometry)
		}
		if c.cancelAfterFirst {
			cancel()
		}
		if c.callbackFailure {
			return fmt.Errorf("callback context: %w", callbackErr)
		}
		return nil
	}
	if c.nilCallback {
		callback = nil
	}
	layer := instance.Layer
	if c.missingLayer {
		layer = "querytest_missing_layer"
	}
	result, err, lifecycleIssues := observeQuery(ctx, instance.Querier, layer, query, callback)
	issues = append(issues, lifecycleIssues...)
	callbackMutex.Lock()
	got := append([]provider.Feature{}, delivered...)
	issues = append(issues, callbackIssues...)
	callbackMutex.Unlock()
	if !reflect.DeepEqual(query, before) {
		issues = append(issues, "query input mutated")
	}
	switch {
	case c.preCancel || c.cancelAfterFirst:
		if !errors.Is(err, context.Canceled) {
			issues = append(issues, "cancellation error chain lost")
		}
		want := 0
		if c.cancelAfterFirst {
			want = 1
		}
		if len(got) != want {
			issues = append(issues, "callbacks continued after cancellation")
		}
	case c.deadline:
		if !errors.Is(err, context.DeadlineExceeded) || len(got) != 0 {
			issues = append(issues, "deadline contract violated")
		}
	case c.callbackFailure:
		if !errors.Is(err, callbackErr) || len(got) != 1 {
			issues = append(issues, "callback error chain or immediate stop lost")
		}
	case c.missingLayer:
		var missing provider.FeatureLayerNotFoundError
		if !errors.As(err, &missing) || len(got) != 0 {
			issues = append(issues, "missing layer must fail before callbacks with typed error")
		}
	case c.nilCallback || c.invalidMapping || c.invalidQuery:
		var invalid provider.InvalidFeatureQueryError
		if !errors.As(err, &invalid) || len(got) != 0 {
			issues = append(issues, "invalid callback or mapping must fail before callbacks with typed error")
		}
	case c.malformed:
		if err == nil || len(got) != 0 {
			issues = append(issues, "malformed geometry accepted or emitted")
		}
		if c.sourceData {
			var data provider.FeatureDataError
			if !errors.As(err, &data) {
				issues = append(issues, "invalid source geometry must have data error classification")
			}
		}
	default:
		if err != nil {
			issues = append(issues, fmt.Sprintf("unexpected query error: %v", err))
			return issues
		}
		issues = append(issues, checkResponse(result, got, c, instance.CountMode)...)
		// Reusing the same query checks deterministic paging and callback ownership.
		repeatDelivered := []provider.Feature{}
		var repeatedMutex sync.Mutex
		secondResult, secondErr, repeatIssues := observeQuery(context.Background(), instance.Querier, instance.Layer, query, func(f *provider.Feature) error {
			repeatedMutex.Lock()
			defer repeatedMutex.Unlock()
			if f == nil {
				return errors.New("nil repeated feature")
			}
			repeatDelivered = append(repeatDelivered, cloneFeature(*f))
			return nil
		})
		repeatedMutex.Lock()
		again := append([]provider.Feature{}, repeatDelivered...)
		repeatedMutex.Unlock()
		issues = append(issues, repeatIssues...)
		if secondErr != nil {
			issues = append(issues, fmt.Sprintf("repeated query: %v", secondErr))
		} else {
			issues = append(issues, checkResponse(secondResult, again, c, instance.CountMode)...)
			if !reflect.DeepEqual(got, again) {
				issues = append(issues, "unstable result or callback mutated shared feature")
			}
		}
	}
	return issues
}

func checkResponse(result provider.FeatureQueryResult, got []provider.Feature, c caseSpec, mode CountMode) []string {
	issues := []string{}
	ids := make([]uint64, 0, len(got))
	for _, f := range got {
		ids = append(ids, f.ID)
	}
	if !reflect.DeepEqual(ids, c.ids) {
		issues = append(issues, fmt.Sprintf("IDs: want %v got %v", c.ids, ids))
	}
	if result.NumberReturned != uint64(len(got)) {
		issues = append(issues, "NumberReturned disagrees with successful callbacks")
	}
	if result.HasMore != c.more {
		issues = append(issues, "HasMore disagrees with lookahead")
	}
	if mode == ExactCount {
		if result.NumberMatched == nil || *result.NumberMatched != c.matched {
			issues = append(issues, "exact NumberMatched wrong or reported unknown")
		}
	} else if mode == OptionalCount {
		if result.NumberMatched != nil && *result.NumberMatched != c.matched {
			issues = append(issues, "optional NumberMatched is not exact")
		}
	} else if result.NumberMatched != nil {
		issues = append(issues, "unknown NumberMatched replaced with fabricated count")
	}
	for _, gotFeature := range got {
		for _, row := range c.fixture.Rows {
			if row.Feature.ID != gotFeature.ID {
				continue
			}
			expected := cloneFeature(row.Feature)
			if len(c.query.Fields) > 0 {
				expected.Tags = map[string]any{}
				for _, field := range c.query.Fields {
					if value, ok := row.Feature.Tags[field]; ok {
						expected.Tags[field] = value
					}
				}
			}
			actual := cloneFeature(gotFeature)
			if c.nativeRingOrientationEquivalent {
				actual.Geometry = normalizeNativeRingOrientation(actual.Geometry)
				expected.Geometry = normalizeNativeRingOrientation(expected.Geometry)
			} else {
				actual.Geometry = normalizeRingClosure(actual.Geometry)
				expected.Geometry = normalizeRingClosure(expected.Geometry)
			}
			if !reflect.DeepEqual(actual, expected) {
				issues = append(issues, fmt.Sprintf("feature %d values/geometry/CRS/properties changed", gotFeature.ID))
			}
		}
	}
	return issues
}

func cloneQuery(q provider.FeatureQuery) provider.FeatureQuery {
	if q.Filter != nil {
		copy := q.Filter.Clone()
		q.Filter = &copy
	}
	if q.Bounds != nil {
		q.Bounds = append([]geom.Extent{}, q.Bounds...)
	}
	if q.Bounds3D != nil {
		q.Bounds3D = append([]provider.Extent3D{}, q.Bounds3D...)
	}
	if q.IDs != nil {
		q.IDs = append([]uint64{}, q.IDs...)
	}
	if q.Fields != nil {
		q.Fields = append([]string{}, q.Fields...)
	}
	if q.Temporal != nil {
		temporal := *q.Temporal
		temporal.Start = cloneTime(temporal.Start)
		temporal.End = cloneTime(temporal.End)
		q.Temporal = &temporal
	}
	return q
}
func cloneTime(t *time.Time) *time.Time {
	if t == nil {
		return nil
	}
	copy := *t
	return &copy
}

// Snapshots support the concrete fixture geometries and scalar properties below.
// They make no promise of arbitrary nested property-object support.
func cloneFeature(f provider.Feature) provider.Feature {
	if f.Tags != nil {
		tags := map[string]any{}
		for key, value := range f.Tags {
			tags[key] = value
		}
		f.Tags = tags
	}
	f.Geometry = cloneGeometry(f.Geometry)
	return f
}
func cloneGeometry(g geom.Geometry) geom.Geometry {
	switch value := g.(type) {
	case geom.Collection:
		out := make(geom.Collection, len(value))
		for i, child := range value {
			out[i] = cloneGeometry(child)
		}
		return out
	case geom.LineString:
		out := make(geom.LineString, len(value))
		copy(out, value)
		return out
	case geom.Polygon:
		out := make(geom.Polygon, len(value))
		for i, ring := range value {
			out[i] = append([][2]float64{}, ring...)
		}
		return out
	default:
		if copy, ok := cloneDimensionalGeometry(g); ok {
			return copy
		}
		return g
	}
}
func mutateGeometry(g geom.Geometry) {
	switch value := g.(type) {
	case geom.Collection:
		for _, child := range value {
			mutateGeometry(child)
		}
	case geom.LineString:
		if len(value) > 0 {
			value[0][0] = 999
		}
	case geom.Polygon:
		if len(value) > 0 && len(value[0]) > 0 {
			value[0][0][0] = 999
		}
	default:
		mutateDimensionalGeometry(g)
	}
}

// queryReturnedHook is implemented only by deterministic negative controls, to
// release and join their late callbacks after the return marker is published.
type queryReturnedHook interface{ queryReturned() }

// observeQuery checks both callback entry and completion against return. Its
// two-millisecond observation window detects observed violations, not absence
// of arbitrarily delayed callbacks forever. It creates no helper goroutines.
func observeQuery(ctx context.Context, q provider.FeatureQuerier, layer string, query provider.FeatureQuery, fn func(*provider.Feature) error) (provider.FeatureQueryResult, error, []string) {
	var returned atomic.Bool
	var mutex sync.Mutex
	issues := []string{}
	record := func() {
		mutex.Lock()
		issues = append(issues, "callback observed after QueryFeatures returned")
		mutex.Unlock()
	}
	wrapped := fn
	if fn != nil {
		wrapped = func(f *provider.Feature) error {
			if returned.Load() {
				record()
				return errors.New("late feature callback")
			}
			err := fn(f)
			if returned.Load() {
				record()
			}
			return err
		}
	}
	result, err := q.QueryFeatures(ctx, layer, query, wrapped)
	returned.Store(true)
	if hook, ok := q.(queryReturnedHook); ok {
		hook.queryReturned()
	}
	timer := time.NewTimer(2 * time.Millisecond)
	<-timer.C
	mutex.Lock()
	defer mutex.Unlock()
	return result, err, append([]string{}, issues...)
}

func cloneFixture(f Fixture) Fixture {
	if f.FilterFields != nil {
		fields := make([]provider.FeatureQueryable, len(f.FilterFields))
		copy(fields, f.FilterFields)
		f.FilterFields = fields
	}
	if f.PublicFields != nil {
		fields := make([]string, len(f.PublicFields))
		copy(fields, f.PublicFields)
		f.PublicFields = fields
	}
	rows := make([]Row, len(f.Rows))
	for i, row := range f.Rows {
		row.Feature = cloneFeature(row.Feature)
		row.RawEmptyGeometry = cloneGeometry(row.RawEmptyGeometry)
		if row.RawTemporal != nil {
			copy := *row.RawTemporal
			if copy.Start != nil {
				value := *copy.Start
				copy.Start = &value
			}
			if copy.End != nil {
				value := *copy.End
				copy.End = &value
			}
			row.RawTemporal = &copy
		}
		row.Start = cloneTime(row.Start)
		row.End = cloneTime(row.End)
		rows[i] = row
	}
	f.Rows = rows
	return f
}

// normalizeRingClosure changes only an exact terminal duplicate of a polygon
// ring's first coordinate. It works on detached snapshots and preserves holes,
// nesting, dimensions, order and all other values without a tolerance.
func normalizeRingClosure(g geom.Geometry) geom.Geometry {
	g = cloneGeometry(g)
	switch value := g.(type) {
	case geom.Collection:
		for i, child := range value {
			value[i] = normalizeRingClosure(child)
		}
	case geom.Polygon:
		for i, ring := range value {
			if len(ring) > 1 && ring[0] == ring[len(ring)-1] {
				value[i] = ring[:len(ring)-1]
			}
		}
	case geom.PolygonZ:
		for i, ring := range value {
			if len(ring) > 1 && ring[0] == ring[len(ring)-1] {
				value[i] = ring[:len(ring)-1]
			}
		}
	}
	return g
}
