package hana_test

import (
	"context"
	"errors"
	"github.com/alexeydott/geom"
	"github.com/alexeydott/tegola/provider"
	"github.com/alexeydott/tegola/provider/internal/querytest"
	"reflect"
	"testing"
	"time"
)

func TestFeatureLiveMOSSupportedProfiles(t *testing.T) {
	if GetConnectionURI() == "" {
		t.Skip("liveHANA")
	}
	for _, profile := range []querytest.FixtureProfile{querytest.OrdinaryTable, querytest.CustomSelection} {
		for _, unit := range []querytest.TemporalPropertyStorage{querytest.UnixSeconds, querytest.UnixMilliseconds, querytest.UnixMicroseconds, querytest.UnixNanoseconds} {
			t.Run(timeStorageName(unit)+map[bool]string{true: " custom", false: " ordinary"}[profile == querytest.CustomSelection], func(t *testing.T) {
				at := time.Unix(0, 0)
				geometries := []geom.Geometry{geom.Point{0, 0}, geom.MultiPoint{{4, 4}, {5, 5}}, geom.LineString{{-2, -2}, {2, 2}}, geom.MultiLineString{{{-5, -5}, {-4, -4}}, {{4, 4}, {5, 5}}}, geom.Polygon{{{-3, -3}, {3, -3}, {3, 3}, {-3, 3}}, {{-1, -1}, {-1, 1}, {1, 1}, {1, -1}}}, geom.MultiPolygon{{{{-5, -5}, {-4, -5}, {-4, -4}, {-5, -4}}}, {{{4, 4}, {5, 4}, {5, 5}, {4, 5}}}}, nil}
				f := querytest.Fixture{Profile: profile, TemporalStorage: unit}
				for i, g := range geometries {
					id := uint64((i + 1) * 10)
					row := querytest.Row{Feature: provider.Feature{ID: id, SRID: 4326, Geometry: g, Tags: map[string]any{"name": "selected", "value": int64(id)}}, Start: &at, End: &at}
					if g == nil {
						row.Start, row.End = nil, nil
					}
					f.Rows = append(f.Rows, row)
				}
				f.Rows = append(f.Rows, querytest.Row{Feature: provider.Feature{ID: 90, SRID: 4326, Geometry: geom.Point{0, 0}, Tags: map[string]any{"name": "excluded", "value": int64(90)}}, Start: &at, End: &at, ExcludedBySelection: true})
				inst := hanaFeatureFactory("mos")(t, f)
				if inst.SetupError != nil {
					t.Fatal(inst.SetupError)
				}
				check := func(q provider.FeatureQuery, want []uint64) {
					t.Helper()
					ids := []uint64{}
					r, e := inst.Querier.QueryFeatures(context.Background(), inst.Layer, q, func(f *provider.Feature) error { ids = append(ids, f.ID); return nil })
					if e != nil || !reflect.DeepEqual(ids, want) {
						t.Fatalf("ids%v want%v result%+v err%v", ids, want, r, e)
					}
				}
				base := []uint64{10, 30, 70}
				if profile == querytest.OrdinaryTable {
					base = append(base, 90)
				}
				check(provider.FeatureQuery{Limit: 100, Bounds: []geom.Extent{{0, 0, 0, 0}}, BoundsSRID: 4326}, base)
				check(provider.FeatureQuery{Limit: 100, Bounds3D: []provider.Extent3D{{0, 0, 1e9, 0, 0, 1e9 + 1}}, BoundsSRID: 4326, BoundsVerticalCRS: provider.CRS84h}, base)
				check(provider.FeatureQuery{Limit: 1, Offset: 1, Bounds: []geom.Extent{{0, 0, 0, 0}}, BoundsSRID: 4326}, []uint64{30})
				if profile == querytest.CustomSelection {
					check(provider.FeatureQuery{Limit: 100, IDs: []uint64{90}}, []uint64{})
				}
				later := time.Unix(1, 0)
				check(provider.FeatureQuery{Limit: 100, Temporal: &provider.TemporalConstraint{Start: &later}}, []uint64{70})
				ctx, cancel := context.WithCancel(context.Background())
				calls := 0
				_, e := inst.Querier.QueryFeatures(ctx, inst.Layer, provider.FeatureQuery{Limit: 100}, func(*provider.Feature) error { calls++; cancel(); return nil })
				if !errors.Is(e, context.Canceled) || calls != 1 {
					t.Fatalf("cancel %v %d", e, calls)
				}
				for _, id := range []uint64{10, 20, 30, 40, 50, 60} {
					check(provider.FeatureQuery{Limit: 1, IDs: []uint64{id}, Fields: []string{"name"}}, []uint64{id})
				}
			})
		}
	}
	for _, dimension := range []provider.CoordinateDimension{provider.DimensionXYZ, provider.DimensionMixedXYXYZ} {
		inst := hanaFeatureFactory("mos")(t, querytest.Fixture{Spatial: provider.SpatialMetadata{Dimension: dimension, VerticalCRS: provider.CRS84h}, Rows: []querytest.Row{{Feature: provider.Feature{ID: 1, Geometry: geom.Point{0, 0}, SRID: 4326}}}})
		if inst.SetupError != nil {
			continue
		}
		_, e := inst.Querier.QueryFeatures(context.Background(), inst.Layer, provider.FeatureQuery{Limit: 1}, func(*provider.Feature) error { t.Fatal("unsupportedMOSprofilecallback"); return nil })
		if !errors.Is(e, provider.ErrUnsupported) {
			t.Fatal(e)
		}
	}
}
func timeStorageName(v querytest.TemporalPropertyStorage) string {
	return map[querytest.TemporalPropertyStorage]string{querytest.UnixSeconds: "seconds", querytest.UnixMilliseconds: "milliseconds", querytest.UnixMicroseconds: "microseconds", querytest.UnixNanoseconds: "nanoseconds"}[v]
}

func TestFeatureLiveMOSMalformedBodies(t *testing.T) {
	if GetConnectionURI() == "" {
		t.Skip("live HANA")
	}
	for _, c := range []struct {
		name      string
		g         geom.Geometry
		malformed string
	}{
		{"unknown body", nil, "mos"}, {"zero point polyline", geom.LineString{}, ""}, {"zero ring polygon", geom.Polygon{}, ""},
	} {
		t.Run(c.name, func(t *testing.T) {
			inst := hanaFeatureFactory("mos")(t, querytest.Fixture{Rows: []querytest.Row{{Feature: provider.Feature{ID: 1, SRID: 4326, Geometry: c.g}, MalformedGeometry: c.malformed}}})
			if inst.SetupError != nil {
				t.Fatal(inst.SetupError)
			}
			calls := 0
			_, err := inst.Querier.QueryFeatures(context.Background(), inst.Layer, provider.FeatureQuery{Limit: 1}, func(*provider.Feature) error { calls++; return nil })
			var data provider.FeatureDataError
			if calls != 0 || !errors.As(err, &data) {
				t.Fatalf("malformed MOS calls=%d: %v", calls, err)
			}
		})
	}
}
