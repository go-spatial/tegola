package querytest

import (
	"math"
	"sync"
	"testing"
	"time"

	"github.com/alexeydott/geom"
	"github.com/alexeydott/tegola/provider"
	"github.com/alexeydott/tegola/provider/geometrycodec"
)

// RunDimensionalProfiles requires real raw WKB/WKT factories in both selection
// profiles. It makes no native XYZ claim for backends admitting native XY only.
func RunDimensionalProfiles(t *testing.T, factory Factory, options ProfileOptions) {
	t.Helper()
	factory = withPublicFieldDeclaration(factory)
	for _, p := range []struct {
		name    string
		profile FixtureProfile
		options Options
	}{
		{"ordinary raw dimensional", OrdinaryTable, options.Ordinary},
		{"custom raw dimensional", CustomSelection, options.Custom},
	} {
		t.Run(p.name, func(t *testing.T) {
			factory := withNativeRingOrientation(factory, p.options.NativeRingOrientationEquivalent)
			nativeProfile, err := copyNativeProfileOptions(p.options.NativeProfileOutcomes)
			if err != nil {
				t.Fatal(err)
			}
			profile, err := copyTemporalPropertyProfile(p.options.PublicTemporalProperties)
			if err != nil {
				t.Fatal(err)
			}
			for _, c := range dimensionalCases() {
				t.Run(c.name, func(t *testing.T) {
					c.fixture.Profile = p.profile
					if profile != nil {
						fixture, err := withPublicTemporalProperties(c.fixture, *profile)
						if err != nil {
							t.Fatal(err)
						}
						c.fixture = fixture
					}
					c.fixture.NativeProfileCase = nativeProfileCaseFor(nativeProfile, c)
					instance := factory(t, cloneFixture(c.fixture))
					registerInstanceCleanup(t, instance)
					if c.fixture.NativeProfileCase != 0 {
						runNativeProfileOutcome(t, factory, instance, c, nativeProfile)
						return
					}
					if runNativeStorageOutcome(t, instance, c) {
						return
					}
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
					if c.mutateDelivery {
						for _, issue := range checkDimensionalConcurrent(instance, c) {
							t.Error(issue)
						}
					}
				})
			}
		})
	}
}

func checkDimensionalConcurrent(instance Instance, c caseSpec) []string {
	var group sync.WaitGroup
	var mutex sync.Mutex
	issues := []string{}
	for i := 0; i < 8; i++ {
		group.Add(1)
		go func() {
			defer group.Done()
			found := checkCase(instance, c)
			mutex.Lock()
			issues = append(issues, found...)
			mutex.Unlock()
		}()
	}
	group.Wait()
	return issues
}

func dimensionalFixture(g geom.Geometry, mixed bool) Fixture {
	dimension := provider.DimensionXYZ
	if mixed {
		dimension = provider.DimensionMixedXYXYZ
	}
	return Fixture{Spatial: provider.SpatialMetadata{Dimension: dimension, VerticalCRS: provider.CRS84h}, Rows: []Row{{
		Feature: provider.Feature{ID: 10, SRID: 4326, Geometry: g, Tags: map[string]any{"name": "dimensional", "value": int64(10)}},
	}}}
}
func dimensionalBox(box provider.Extent3D) provider.FeatureQuery {
	return provider.FeatureQuery{
		Limit: 100, Bounds3D: []provider.Extent3D{box}, BoundsSRID: 4326, BoundsVerticalCRS: provider.CRS84h,
	}
}
func dimensionalCases() []caseSpec {
	line := geom.LineStringZ{{0, 0, 0}, {10, 0, 10}}
	tilted := geom.PolygonZ{
		{{0, 0, 0}, {10, 0, 10}, {10, 10, 10}, {0, 10, 0}, {0, 0, 0}},
		{{4, 4, 4}, {6, 4, 6}, {6, 6, 6}, {4, 6, 4}, {4, 4, 4}},
	}
	constant := geom.PolygonZ{
		{{0, 0, 7}, {2, 0, 7}, {2, 2, 7}, {0, 2, 7}, {0, 0, 7}},
		{{0.5, 0.5, 7}, {1.5, 0.5, 7}, {1.5, 1.5, 7}, {0.5, 1.5, 7}, {0.5, 0.5, 7}},
	}
	vertical := geom.PolygonZ{
		{{3, 0, 0}, {3, 10, 0}, {3, 10, 10}, {3, 0, 10}, {3, 0, 0}},
		{{3, 4, 4}, {3, 6, 4}, {3, 6, 6}, {3, 4, 6}, {3, 4, 4}},
	}
	cases := []caseSpec{
		{name: "XYZ point preserved", fixture: dimensionalFixture(geom.PointZ{15, 30, 7}, false), ids: []uint64{10}, matched: 1},
		{name: "XYZ correlated segment range false positive", fixture: dimensionalFixture(line, false), query: dimensionalBox(provider.Extent3D{0, -1, 9, 1, 1, 10}), ids: []uint64{}, matched: 0},
		{name: "XYZ exact degenerate segment boundary", fixture: dimensionalFixture(line, false), query: dimensionalBox(provider.Extent3D{5, 0, 5, 5, 0, 5}), ids: []uint64{10}, matched: 1},
		{name: "XYZ polyline global ranges insufficient", fixture: dimensionalFixture(geom.LineStringZ{{0, 0, 0}, {10, 0, 10}, {0, 0, 20}}, false), query: dimensionalBox(provider.Extent3D{0, -1, 9, 1, 1, 11}), ids: []uint64{}, matched: 0},
		{name: "tilted polygon hole interior excluded", fixture: dimensionalFixture(tilted, false), query: dimensionalBox(provider.Extent3D{4.5, 4.5, 4.5, 5.5, 5.5, 5.5}), ids: []uint64{}, matched: 0},
		{name: "tilted polygon XY versus Z false positive", fixture: dimensionalFixture(tilted, false), query: dimensionalBox(provider.Extent3D{1, 1, 8, 2, 2, 9}), ids: []uint64{}, matched: 0},
		{name: "tilted polygon hole boundary included", fixture: dimensionalFixture(tilted, false), query: dimensionalBox(provider.Extent3D{4, 5, 4, 4, 5, 4}), ids: []uint64{10}, matched: 1},
		{name: "tilted polygon outer boundary included", fixture: dimensionalFixture(tilted, false), query: dimensionalBox(provider.Extent3D{10, 5, 10, 10, 5, 10}), ids: []uint64{10}, matched: 1},
		{name: "constant height hole excluded", fixture: dimensionalFixture(constant, false), query: dimensionalBox(provider.Extent3D{1, 1, 7, 1, 1, 7}), ids: []uint64{}, matched: 0},
		{name: "constant height hole boundary", fixture: dimensionalFixture(constant, false), query: dimensionalBox(provider.Extent3D{0.5, 1, 7, 0.5, 1, 7}), ids: []uint64{10}, matched: 1},
		{name: "vertical polygon hole excluded", fixture: dimensionalFixture(vertical, false), query: dimensionalBox(provider.Extent3D{3, 5, 5, 3, 5, 5}), ids: []uint64{}, matched: 0},
		{name: "vertical polygon hole boundary", fixture: dimensionalFixture(vertical, false), query: dimensionalBox(provider.Extent3D{3, 4, 5, 3, 4, 5}), ids: []uint64{10}, matched: 1},
		{name: "mixed XY has unconstrained vertical", fixture: dimensionalFixture(geom.Collection{geom.Point{0, 0}, geom.PointZ{5, 5, 100}}, true), query: dimensionalBox(provider.Extent3D{0, 0, 500, 0, 0, 501}), ids: []uint64{10}, matched: 1},
		{name: "empty child does not imply absent collection", fixture: dimensionalFixture(geom.Collection{geom.LineStringZ{}, geom.PointZ{5, 5, 5}}, false), query: dimensionalBox(provider.Extent3D{100, 100, 100, 101, 101, 101}), ids: []uint64{}, matched: 0},
		{name: "NULL spatial geometry matches XYZ bbox", fixture: dimensionalFixture(nil, false), query: dimensionalBox(provider.Extent3D{100, 100, 100, 101, 101, 101}), ids: []uint64{10}, matched: 1},
		{name: "XYZ MultiPoint family preserved", fixture: dimensionalFixture(geom.MultiPointZ{{1, 2, 3}, {4, 5, 6}}, false), ids: []uint64{10}, matched: 1},
		{name: "XYZ MultiLineString family preserved", fixture: dimensionalFixture(geom.MultiLineStringZ{{{0, 0, 0}, {1, 1, 1}}, {{2, 2, 2}, {3, 3, 3}}}, false), ids: []uint64{10}, matched: 1},
		{name: "XYZ MultiPolygon family preserved", fixture: dimensionalFixture(geometrycodec.MultiPolygonZ{[][][3]float64(tilted)}, false), ids: []uint64{10}, matched: 1},
		{name: "nested XYZ mutable callback ownership", fixture: dimensionalFixture(geom.Collection{geom.Collection{geom.LineStringZ{{0, 0, 0}, {1, 1, 1}}}, tilted}, false), ids: []uint64{10}, matched: 1, mutateDelivery: true},
		{name: "later invalid XYZ child cannot hide behind intersection", fixture: dimensionalFixture(geom.Collection{geom.PointZ{0, 0, 0}, geom.PointZ{1, math.NaN(), 2}}, false), query: dimensionalBox(provider.Extent3D{0, 0, 0, 0, 0, 0}), malformed: true, sourceData: true},
		{name: "nonplanar XYZ source surface invalid", fixture: dimensionalFixture(geom.PolygonZ{{{0, 0, 0}, {1, 0, 0}, {1, 1, 1}, {0, 1, 0}, {0, 0, 0}}}, false), malformed: true, sourceData: true},
	}
	for i := range cases {
		if cases[i].query.Limit == 0 {
			cases[i].query.Limit = 100
		}
	}
	for _, empty := range []struct {
		name   string
		source geom.Geometry
	}{
		{"wholly empty XYZ line becomes null", geom.LineStringZ{}},
		{"nested wholly empty XYZ collection becomes null", geom.Collection{geom.Collection{geom.LineStringZ{}, geom.PolygonZ{}}}},
	} {
		fixture := dimensionalFixture(nil, false)
		fixture.Rows[0].RawEmptyGeometry = empty.source
		cases = append(cases, caseSpec{
			name: empty.name, fixture: fixture,
			query: dimensionalBox(provider.Extent3D{100, 100, 100, 101, 101, 101}), ids: []uint64{10}, matched: 1,
		})
	}
	epoch, next := time.Unix(0, 0).UTC(), time.Unix(1, 0).UTC()
	hit := geom.LineStringZ{{0, 0, 9}, {1, 0, 10}}
	fixture := Fixture{Spatial: provider.SpatialMetadata{Dimension: provider.DimensionXYZ, VerticalCRS: provider.CRS84h}, Rows: []Row{
		{Feature: provider.Feature{ID: 10, SRID: 4326, Geometry: line, Tags: map[string]any{"name": "false", "value": int64(10)}}, Start: &epoch, End: &epoch},
		{Feature: provider.Feature{ID: 20, SRID: 4326, Geometry: hit, Tags: map[string]any{"name": "hit", "value": int64(20)}}, Start: &epoch, End: &epoch},
		{Feature: provider.Feature{ID: 30, SRID: 4326, Geometry: hit, Tags: map[string]any{"name": "later", "value": int64(30)}}, Start: &next, End: &next},
		{Feature: provider.Feature{ID: 40, SRID: 4326, Tags: map[string]any{"name": "absent", "value": int64(40)}}},
		{Feature: provider.Feature{ID: 50, SRID: 4326, Geometry: geom.LineStringZ{{1, 0, 9}, {1, 0, 10}}, Tags: map[string]any{"name": "boundary", "value": int64(50)}}, Start: &epoch, End: &epoch},
	}}
	for _, page := range []struct {
		name   string
		limit  uint
		offset uint64
		ids    []uint64
		more   bool
	}{
		{"XYZ bbox time exact set", 100, 0, []uint64{20, 40, 50}, false},
		{"XYZ bbox time page middle", 1, 1, []uint64{40}, true},
		{"XYZ bbox time page last", 1, 2, []uint64{50}, false},
	} {
		query := dimensionalBox(provider.Extent3D{0, -1, 9, 1, 1, 10})
		query.Temporal = &provider.TemporalConstraint{Start: &epoch, End: &epoch}
		query.Limit = page.limit
		query.Offset = page.offset
		cases = append(cases, caseSpec{name: page.name, fixture: fixture, query: query, ids: page.ids, matched: 3, more: page.more})
	}
	return cases
}
