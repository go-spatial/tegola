package querytest

import (
	"math"
	"testing"
	"time"

	"github.com/alexeydott/geom"
	"github.com/alexeydott/tegola/provider"
)

// RawTemporalIntegers supplies literal source storage, not query endpoints.
type RawTemporalIntegers struct{ Start, End *int64 }

// RunExactTemporalProfiles checks integer source storage against exact query
// endpoints. Expected membership is literal, never a production quantization.
func RunExactTemporalProfiles(t *testing.T, factory Factory, options ProfileOptions) {
	t.Helper()
	factory = withPublicFieldDeclaration(factory)
	for _, unit := range []struct {
		name    string
		storage TemporalPropertyStorage
	}{
		{"seconds", UnixSeconds}, {"milliseconds", UnixMilliseconds},
		{"microseconds", UnixMicroseconds}, {"nanoseconds", UnixNanoseconds},
	} {
		t.Run(unit.name, func(t *testing.T) {
			for _, profile := range []struct {
				name      string
				selection FixtureProfile
				options   Options
			}{
				{"ordinary exact temporal", OrdinaryTable, options.Ordinary},
				{"custom exact temporal", CustomSelection, options.Custom},
			} {
				t.Run(profile.name, func(t *testing.T) {
					factory := withNativeRingOrientation(factory, profile.options.NativeRingOrientationEquivalent)
					properties, err := copyTemporalPropertyProfile(profile.options.PublicTemporalProperties)
					if err != nil {
						t.Fatal(err)
					}
					if properties != nil {
						properties.Storage = unit.storage
					}
					for _, c := range exactTemporalCases(unit.storage) {
						t.Run(c.name, func(t *testing.T) {
							c.fixture.Profile = profile.selection
							if properties != nil {
								c.fixture, err = withPublicTemporalProperties(c.fixture, *properties)
								if err != nil {
									t.Fatal(err)
								}
							}
							instance := factory(t, cloneFixture(c.fixture))
							registerInstanceCleanup(t, instance)
							if issues, handled := checkSetup(instance, c); handled {
								for _, issue := range issues {
									t.Error(issue)
								}
								return
							}
							if instance.CountMode > OptionalCount {
								t.Fatal("invalid count mode")
							}
							for _, issue := range checkCase(instance, c) {
								t.Error(issue)
							}
						})
					}
				})
			}
		})
	}
}

func exactTimeFixture(storage TemporalPropertyStorage, values ...time.Time) Fixture {
	fixture := Fixture{TemporalStorage: storage}
	for i, value := range values {
		start, end := value, value
		fixture.Rows = append(fixture.Rows, Row{
			Feature: provider.Feature{ID: uint64((i + 1) * 10), SRID: 4326, Geometry: geom.Point{0, 0},
				Tags: map[string]any{"name": "exact", "value": int64((i + 1) * 10)}}, Start: &start, End: &end,
		})
	}
	fixture.Rows = append(fixture.Rows, Row{Feature: provider.Feature{ID: 90, SRID: 4326, Geometry: geom.Point{0, 0},
		Tags: map[string]any{"name": "absent time", "value": int64(90)}}})
	return fixture
}

// This conversion defines literal source fixtures, not query-bound rounding.
func timeFromFixtureInteger(value int64, storage TemporalPropertyStorage) time.Time {
	factor := int64(1)
	switch storage {
	case UnixMilliseconds:
		factor = 1000
	case UnixMicroseconds:
		factor = 1000000
	case UnixNanoseconds:
		factor = 1000000000
	}
	seconds, remainder := value/factor, value%factor
	if remainder < 0 {
		seconds--
		remainder += factor
	}
	return time.Unix(seconds, remainder*(1000000000/factor)).UTC()
}

func exactTemporalCases(storage TemporalPropertyStorage) []caseSpec {
	epoch, before, after := time.Unix(0, 0).UTC(), time.Unix(-1, 0).UTC(), time.Unix(1, 0).UTC()
	fixture := exactTimeFixture(storage, before, epoch, after)
	instant := func(t time.Time, tail string, leap bool) *provider.TemporalConstraint {
		start, end := t, t
		return &provider.TemporalConstraint{Start: &start, End: &end, StartSubNanosecond: tail,
			EndSubNanosecond: tail, StartLeapSecond: leap, EndLeapSecond: leap}
	}
	cases := []caseSpec{}
	add := func(name string, f Fixture, temporal *provider.TemporalConstraint, ids []uint64) {
		cases = append(cases, caseSpec{name: name, fixture: f, query: provider.FeatureQuery{Limit: 100, Temporal: temporal},
			ids: ids, matched: uint64(len(ids))})
	}
	add("exact epoch inclusive", fixture, instant(epoch, "", false), []uint64{20, 90})
	add("one ns before epoch", fixture, instant(time.Unix(-1, 999999999).UTC(), "", false), []uint64{90})
	add("one ns after epoch", fixture, instant(time.Unix(0, 1).UTC(), "", false), []uint64{90})
	add("nonzero tenth decimal digit", fixture, instant(epoch, "1", false), []uint64{90})
	add("all zero long tail", fixture, instant(epoch, "00000000000000000000", false), []uint64{20, 90})
	add("nonzero far decimal tail", fixture, instant(epoch, "00000000000000000001", false), []uint64{90})
	add("negative epoch inclusive", fixture, instant(before, "", false), []uint64{10, 90})
	add("timezone equivalent epoch", fixture, instant(epoch.In(time.FixedZone("fixture offset", 10800)), "", false), []uint64{20, 90})
	tick := timeFromFixtureInteger(1, storage)
	tickFixture := exactTimeFixture(storage, epoch, tick)
	add("declared source tick inclusive", tickFixture, instant(tick, "", false), []uint64{20, 90})
	beforeTickIDs := []uint64{90}
	if storage == UnixNanoseconds {
		beforeTickIDs = []uint64{10, 90}
	}
	add("one ns before declared source tick", tickFixture, instant(tick.Add(-time.Nanosecond), "", false), beforeTickIDs)
	add("one ns after declared source tick", tickFixture, instant(tick.Add(time.Nanosecond), "", false), []uint64{90})
	openAfter := &provider.TemporalConstraint{Start: &epoch, StartSubNanosecond: "1"}
	add("open start after epoch exact tail", fixture, openAfter, []uint64{30, 90})
	openBefore := &provider.TemporalConstraint{End: &epoch, EndSubNanosecond: "1"}
	add("open end after epoch exact tail", fixture, openBefore, []uint64{10, 20, 90})
	reversed := instant(epoch, "2", false)
	reversed.EndSubNanosecond = "1"
	cases = append(cases, caseSpec{name: "same nanosecond reversed tails invalid", fixture: fixture,
		query: provider.FeatureQuery{Limit: 100, Temporal: reversed}, invalidQuery: true})
	leapBefore, leapAfter := time.Unix(1483228799, 0).UTC(), time.Unix(1483228800, 0).UTC()
	leapFixture := exactTimeFixture(storage, leapBefore, leapAfter)
	add("announced leap instant excludes ordinary endpoints", leapFixture, instant(leapBefore, "", true), []uint64{90})
	add("announced leap fraction excludes ordinary endpoints", leapFixture, instant(leapBefore, "123", true), []uint64{90})
	add("interval spanning leap includes ordinary endpoints", leapFixture,
		&provider.TemporalConstraint{Start: &leapBefore, End: &leapAfter}, []uint64{10, 20, 90})
	falseLeap := instant(epoch, "", true)
	cases = append(cases, caseSpec{name: "unannounced leap request invalid", fixture: fixture,
		query: provider.FeatureQuery{Limit: 100, Temporal: falseLeap}, invalidQuery: true})
	minimum, maximum := int64(math.MinInt64), int64(math.MaxInt64)
	edges := exactTimeFixture(storage, epoch, epoch)
	edges.Rows[0].Start, edges.Rows[0].End = nil, nil
	edges.Rows[1].Start, edges.Rows[1].End = nil, nil
	edges.Rows[0].RawTemporal = &RawTemporalIntegers{Start: &minimum, End: &minimum}
	edges.Rows[1].RawTemporal = &RawTemporalIntegers{Start: &maximum, End: &maximum}
	add("raw int64 extremes exclude epoch", edges, instant(epoch, "", false), []uint64{90})
	add("raw int64 minimum in open past", edges, &provider.TemporalConstraint{End: &epoch}, []uint64{10, 90})
	add("raw int64 maximum in open future", edges, &provider.TemporalConstraint{Start: &epoch}, []uint64{20, 90})
	if storage == UnixMicroseconds || storage == UnixNanoseconds {
		for _, edge := range []struct {
			name  string
			value int64
		}{{"source int64 minimum", math.MinInt64}, {"source int64 maximum", math.MaxInt64}} {
			value := timeFromFixtureInteger(edge.value, storage)
			edgeFixture := exactTimeFixture(storage, value)
			add(edge.name+" inclusive", edgeFixture, instant(value, "", false), []uint64{10, 90})
			add(edge.name+" nonzero subnano", edgeFixture, instant(value, "1", false), []uint64{90})
		}
	}
	// A reversed stored interval is source data corruption, never a bad request.
	corrupt := exactTimeFixture(storage, epoch)
	corrupt.Rows[0].Start = &after
	corrupt.Rows[0].End = &before
	cases = append(cases, caseSpec{name: "reversed stored temporal interval is source error", fixture: corrupt,
		query: provider.FeatureQuery{Limit: 100}, malformed: true, sourceData: true})
	return cases
}
