package features

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alexeydott/geom"
	"github.com/alexeydott/proj"
	"github.com/alexeydott/tegola/basic"
	"github.com/alexeydott/tegola/provider"
	"github.com/alexeydott/tegola/provider/crsconfig"
)

func featureQuerier(feature *provider.Feature) testQuerier {
	return func(ctx context.Context, _ string, _ provider.FeatureQuery, fn func(*provider.Feature) error) (provider.FeatureQueryResult, error) {
		if err := ctx.Err(); err != nil {
			return provider.FeatureQueryResult{}, err
		}
		if err := fn(feature); err != nil {
			return provider.FeatureQueryResult{}, err
		}
		one := uint64(1)
		return provider.FeatureQueryResult{NumberReturned: 1, NumberMatched: &one}, nil
	}
}

func TestServiceCRS84Sources(t *testing.T) {
	basic.RegisterBuiltinProj4SRIDs()
	custom, err := basic.RegisterProj4Defn("+proj=utm +zone=41 +datum=WGS84 +units=m +no_defs")
	if err != nil {
		t.Fatal(err)
	}
	datum, err := basic.RegisterProj4Defn("+proj=longlat +ellps=krass +towgs84=23.92,-141.27,-80.9,0,0.35,0.82,-0.12 +no_defs")
	if err != nil {
		t.Fatal(err)
	}
	mos, changed, err := crsconfig.ApplySystemInfoCRS(3857, false, "+proj=utm +zone=41 +datum=WGS84 +units=m +no_defs")
	if err != nil || !changed {
		t.Fatalf("MOS resolution: %v %v", changed, err)
	}
	expected := []float64{61.4, 55.1}
	for name, srid := range map[string]uint64{"4326": 4326, "3857": 3857, "UTM": 32641, "custom": custom, "towgs84": datum, "MOS resolved": uint64(mos)} {
		t.Run(name, func(t *testing.T) {
			xy := append([]float64{}, expected...)
			if srid != 4326 {
				var err error
				xy, err = proj.Convert(proj.EPSGCode(srid), expected)
				if err != nil {
					t.Fatal(err)
				}
			}
			source := &provider.Feature{ID: 18446744073709551615, SRID: srid, Geometry: geom.Collection{geom.Point{xy[0], xy[1]}}, Tags: map[string]any{"large": uint64(18446744073709551615), "nested": map[string]any{"values": []any{"original"}}}}
			s := newTestService(t, srid, featureQuerier(source))
			value, err := s.QueryFeature(context.Background(), "public", source.ID)
			if err != nil {
				t.Fatal(err)
			}
			var geometry struct {
				Type       string
				Geometries []struct {
					Type        string
					Coordinates [2]float64
				}
			}
			if err := json.Unmarshal(value.Geometry, &geometry); err != nil {
				t.Fatal(err)
			}
			if geometry.Type != "GeometryCollection" || len(geometry.Geometries) != 1 {
				t.Fatalf("collection flattened: %s", value.Geometry)
			}
			for i, v := range geometry.Geometries[0].Coordinates {
				if math.Abs(v-expected[i]) > 1e-6 {
					t.Fatalf("CRS84 mismatch: %v", geometry)
				}
			}
			if value.ID != source.ID || value.Properties["large"].(json.Number).String() != "18446744073709551615" {
				t.Fatal("identity/property precision lost")
			}
			value.Properties["nested"].(map[string]any)["values"].([]any)[0] = "modified"
			if source.Tags["nested"].(map[string]any)["values"].([]any)[0] != "original" {
				t.Fatal("property alias retained")
			}
			if source.Geometry.(geom.Collection)[0].(geom.Point) != (geom.Point{xy[0], xy[1]}) {
				t.Fatal("source geometry mutated")
			}
		})
	}
}

func TestServiceForwardsQueryUnchanged(t *testing.T) {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	query := provider.FeatureQuery{Limit: 3, Offset: 2, IDs: []uint64{7}, Fields: []string{"name"}, Bounds: []geom.Extent{{10, 20, 30, 40}}, BoundsSRID: 4326, Temporal: &provider.TemporalConstraint{Start: &start}}
	original := query
	s, err := NewService([]CollectionSource{{ID: "public", Layer: &testLayer{name: "source", srid: 3857, mapping: provider.TemporalMapping{InstantField: "time"}}, Querier: testQuerier(func(_ context.Context, layer string, received provider.FeatureQuery, _ func(*provider.Feature) error) (provider.FeatureQueryResult, error) {
		if layer != "source" || !reflect.DeepEqual(received, query) {
			t.Fatalf("query changed: %+v", received)
		}
		return provider.FeatureQueryResult{}, nil
	})}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.QueryCollectionPage(context.Background(), "public", query); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(query, original) {
		t.Fatal("caller query mutated")
	}
}

func TestServiceErrorsAndBoundedPages(t *testing.T) {
	callbackError := errors.New("caller stopped")
	source := &provider.Feature{ID: 7, SRID: 4326, Geometry: geom.Point{10, 20}}
	s := newTestService(t, 4326, featureQuerier(source))
	if _, err := s.QueryCollection(context.Background(), "public", provider.FeatureQuery{Limit: 1}, func(Feature) error { return callbackError }); !errors.Is(err, callbackError) {
		t.Fatalf("callback chain: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := s.QueryFeature(ctx, "public", 7); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if _, err := s.QueryCollection(context.Background(), "public", provider.FeatureQuery{Limit: 1}, nil); err == nil {
		t.Fatal("nil callback accepted")
	}
	if _, err := s.QueryCollectionPage(context.Background(), "public", provider.FeatureQuery{}); err == nil {
		t.Fatal("invalid query accepted")
	}
	if _, err := s.QueryFeature(context.Background(), "missing", 7); err == nil {
		t.Fatal("missing collection accepted")
	} else {
		var typed CollectionNotFoundError
		if !errors.As(err, &typed) {
			t.Fatal(err)
		}
	}
	empty := newTestService(t, 4326, testQuerier(emptyQuerier))
	if _, err := empty.QueryFeature(context.Background(), "public", 7); err == nil {
		t.Fatal("missing feature accepted")
	} else {
		var typed FeatureNotFoundError
		if !errors.As(err, &typed) {
			t.Fatal(err)
		}
	}
	cases := map[string]testQuerier{
		"overdelivery": func(_ context.Context, _ string, _ provider.FeatureQuery, fn func(*provider.Feature) error) (provider.FeatureQueryResult, error) {
			if err := fn(source); err != nil {
				return provider.FeatureQueryResult{}, err
			}
			return provider.FeatureQueryResult{}, fn(source)
		},
		"count mismatch": func(_ context.Context, _ string, _ provider.FeatureQuery, fn func(*provider.Feature) error) (provider.FeatureQueryResult, error) {
			return provider.FeatureQueryResult{}, fn(source)
		},
		"ignored callback": func(_ context.Context, _ string, _ provider.FeatureQuery, fn func(*provider.Feature) error) (provider.FeatureQueryResult, error) { // Deliberately broken provider: service must retain an ignored callback failure.
			_ = fn(nil)
			return provider.FeatureQueryResult{}, nil
		},
		"SRID mismatch": featureQuerier(&provider.Feature{ID: 7, SRID: 3857, Geometry: geom.Point{0, 0}}),
		"nonfinite":     featureQuerier(&provider.Feature{ID: 7, SRID: 4326, Geometry: geom.Point{math.NaN(), 0}}),
		"wrong ID":      featureQuerier(&provider.Feature{ID: 8, SRID: 4326}),
	}
	for name, q := range cases {
		t.Run(name, func(t *testing.T) {
			s := newTestService(t, 4326, q)
			if name == "wrong ID" {
				if _, err := s.QueryFeature(context.Background(), "public", 7); err == nil {
					t.Fatal("wrong ID accepted")
				}
				return
			}
			page, err := s.QueryCollectionPage(context.Background(), "public", provider.FeatureQuery{Limit: 1})
			if err == nil || len(page.Features) != 0 {
				t.Fatalf("successful partial page: %+v %v", page, err)
			}
		})
	}
}

func TestServiceConcurrentImmutableResponses(t *testing.T) {
	source := &provider.Feature{ID: 7, SRID: 4326, Geometry: geom.LineString{{10, 20}, {11, 21}}, Tags: map[string]any{"nested": []any{uint64(7)}}}
	s := newTestService(t, 4326, featureQuerier(source))
	var wg sync.WaitGroup
	for range 16 {
		wg.Go(func() {
			page, err := s.QueryCollectionPage(context.Background(), "public", provider.FeatureQuery{Limit: 1})
			if err != nil {
				t.Error(err)
				return
			}
			page.Features[0].Properties["nested"].([]any)[0] = "changed"
			page.Features[0].Geometry[0] = ' '
		})
	}
	wg.Wait()
	if source.Tags["nested"].([]any)[0] != uint64(7) {
		t.Fatal("shared properties mutated")
	}
}

func TestServiceCancellationAndProviderErrorChains(t *testing.T) {
	sentinel := errors.New("database stopped")
	s := newTestService(t, 4326, testQuerier(func(context.Context, string, provider.FeatureQuery, func(*provider.Feature) error) (provider.FeatureQueryResult, error) {
		return provider.FeatureQueryResult{}, sentinel
	}))
	if _, err := s.QueryCollectionPage(context.Background(), "public", provider.FeatureQuery{Limit: 1}); !errors.Is(err, sentinel) {
		t.Fatalf("provider chain lost: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	s = newTestService(t, 4326, testQuerier(func(_ context.Context, _ string, _ provider.FeatureQuery, fn func(*provider.Feature) error) (provider.FeatureQueryResult, error) {
		if err := fn(&provider.Feature{ID: 1, SRID: 4326}); err != nil {
			return provider.FeatureQueryResult{}, err
		}
		cancel()
		return provider.FeatureQueryResult{NumberReturned: 1}, nil
	}))
	page, err := s.QueryCollectionPage(ctx, "public", provider.FeatureQuery{Limit: 1})
	if !errors.Is(err, context.Canceled) || len(page.Features) != 0 {
		t.Fatalf("cancelled partial page returned: %+v %v", page, err)
	}
}

func TestServiceRejectsUnencodablePayloads(t *testing.T) {
	cyclic := map[string]any{}
	cyclic["self"] = cyclic
	for name, feature := range map[string]*provider.Feature{
		"unsupported geometry": {SRID: 4326, Geometry: struct{}{}},
		"cyclic properties":    {SRID: 4326, Tags: cyclic},
		"invalid properties":   {SRID: 4326, Tags: map[string]any{"value": math.Inf(1)}},
		"invalid polygon":      {SRID: 4326, Geometry: geom.Polygon{{{0, 0}, {1, 1}, {2, 2}}}},
		"longitude domain":     {SRID: 4326, Geometry: geom.Point{181, 0}},
	} {
		t.Run(name, func(t *testing.T) {
			s := newTestService(t, 4326, featureQuerier(feature))
			if _, err := s.QueryCollectionPage(context.Background(), "public", provider.FeatureQuery{Limit: 1}); err == nil {
				t.Fatal("unencodable feature accepted")
			}
		})
	}
}

type cancellationDuringTraversal struct {
	context.Context
	checks atomic.Int32
}

func (c *cancellationDuringTraversal) Err() error {
	if c.checks.Add(1) >= 5 {
		return context.Canceled
	}
	return nil
}

func TestNestedGeometryCancellationPreservesSentinel(t *testing.T) {
	ctx := &cancellationDuringTraversal{Context: context.Background()}
	source := &provider.Feature{ID: 1, SRID: 4326, Geometry: geom.Collection{geom.LineString{{0, 0}, {1, 1}, {2, 2}, {3, 3}}}}
	s := newTestService(t, 4326, featureQuerier(source))
	if _, err := s.QueryCollectionPage(ctx, "public", provider.FeatureQuery{Limit: 1}); !errors.Is(err, context.Canceled) {
		t.Fatalf("nested traversal lost cancellation chain: %v", err)
	}
}
