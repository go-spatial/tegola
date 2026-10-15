package features

import (
	"context"
	"errors"
	"testing"

	"github.com/alexeydott/geom"
	"github.com/alexeydott/proj"
	"github.com/alexeydott/tegola/provider"
)

type testLayer struct {
	name          string
	srid          uint64
	mapping       provider.TemporalMapping
	eligibility   error
	temporalError error
}

func (l *testLayer) Name() string            { return l.name }
func (l *testLayer) SRID() uint64            { return l.srid }
func (l *testLayer) GeomType() geom.Geometry { return geom.Point{} }
func (l *testLayer) TemporalMapping() (provider.TemporalMapping, error) {
	return l.mapping, l.temporalError
}
func (l *testLayer) FeatureQuerySupported() error { return l.eligibility }
func (*testLayer) SpatialMetadata() (provider.SpatialMetadata, error) {
	return provider.SpatialMetadata{Dimension: provider.DimensionXY}, nil
}

type plainLayer struct{}

func (plainLayer) Name() string            { return "plain" }
func (plainLayer) SRID() uint64            { return 4326 }
func (plainLayer) GeomType() geom.Geometry { return geom.Point{} }

type testQuerier func(context.Context, string, provider.FeatureQuery, func(*provider.Feature) error) (provider.FeatureQueryResult, error)

func (q testQuerier) QueryFeatures(ctx context.Context, layer string, query provider.FeatureQuery, fn func(*provider.Feature) error) (provider.FeatureQueryResult, error) {
	return q(ctx, layer, query, fn)
}
func emptyQuerier(context.Context, string, provider.FeatureQuery, func(*provider.Feature) error) (provider.FeatureQueryResult, error) {
	zero := uint64(0)
	return provider.FeatureQueryResult{NumberMatched: &zero}, nil
}
func newTestService(t *testing.T, srid uint64, query testQuerier) *Service {
	t.Helper()
	s, err := NewService([]CollectionSource{{ID: "public", Layer: &testLayer{name: "source", srid: srid}, Querier: query}})
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestCatalogRejectsInvalidSources(t *testing.T) {
	valid := CollectionSource{ID: "public", Layer: &testLayer{name: "source", srid: 4326}, Querier: testQuerier(emptyQuerier)}
	var nilLayer *testLayer
	var nilQuerier testQuerier
	cases := map[string][]CollectionSource{
		"blank ID":         {{ID: " ", Layer: valid.Layer, Querier: valid.Querier}},
		"duplicate":        {valid, valid},
		"nil layer":        {{ID: "public", Layer: nilLayer, Querier: valid.Querier}},
		"nil querier":      {{ID: "public", Layer: valid.Layer, Querier: nilQuerier}},
		"missing metadata": {{ID: "public", Layer: plainLayer{}, Querier: valid.Querier}},
		"blank layer":      {{ID: "public", Layer: &testLayer{srid: 4326}, Querier: valid.Querier}},
		"unknown CRS":      {{ID: "public", Layer: &testLayer{name: "source", srid: 99999999}, Querier: valid.Querier}},
		"invalid mapping":  {{ID: "public", Layer: &testLayer{name: "source", srid: 4326, mapping: provider.TemporalMapping{StartField: "start"}}, Querier: valid.Querier}},
		"unsupported":      {{ID: "public", Layer: &testLayer{name: "source", srid: 4326, eligibility: provider.ErrUnsupported}, Querier: valid.Querier}},
	}
	for name, sources := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := NewService(sources); err == nil {
				t.Fatal("accepted invalid source")
			}
		})
	}
	_, err := NewService(cases["unsupported"])
	if !errors.Is(err, provider.ErrUnsupported) {
		t.Fatalf("lost unsupported chain: %v", err)
	}
	sentinel := errors.New("temporal metadata")
	valid.Layer = &testLayer{name: "source", srid: 4326, temporalError: sentinel}
	if _, err := NewService([]CollectionSource{valid}); !errors.Is(err, sentinel) {
		t.Fatalf("lost temporal error: %v", err)
	}
}

func TestCatalogSnapshotsMetadata(t *testing.T) {
	layer := &testLayer{name: "original", srid: 4326, mapping: provider.TemporalMapping{InstantField: "time"}}
	source := []CollectionSource{{ID: "public", Layer: layer, Querier: testQuerier(func(_ context.Context, name string, _ provider.FeatureQuery, _ func(*provider.Feature) error) (provider.FeatureQueryResult, error) {
		if name != "original" {
			t.Fatalf("mutable source name retained: %s", name)
		}
		return provider.FeatureQueryResult{}, nil
	})}}
	s, err := NewService(source)
	if err != nil {
		t.Fatal(err)
	}
	source[0].ID = "changed"
	layer.name = "changed"
	layer.srid = 3857
	layer.mapping.InstantField = "changed"
	if s.collections["public"].srid != 4326 || s.collections["public"].temporal.InstantField != "time" {
		t.Fatal("metadata not snapshotted")
	}
	if _, err := s.QueryCollectionPage(context.Background(), "public", provider.FeatureQuery{Limit: 1}); err != nil {
		t.Fatal(err)
	}
}

func TestCatalogRejectsBrokenRegisteredProjection(t *testing.T) {
	const code = 987654321
	proj.CustomProjection(code, "+proj=not_an_operation")
	t.Cleanup(func() { proj.RemoveCustomProjection(code) })
	_, err := NewService([]CollectionSource{{ID: "public", Layer: &testLayer{name: "source", srid: code}, Querier: testQuerier(emptyQuerier)}})
	if !errors.Is(err, provider.ErrUnsupported) {
		t.Fatalf("broken registered projection accepted: %v", err)
	}
}

func TestCatalogAcceptsRestrictedProjectionDomain(t *testing.T) {
	const code = 987654322
	proj.CustomProjection(code, "+proj=etmerc +lon_0=90 +lat_0=0 +ellps=WGS84 +units=m")
	t.Cleanup(func() { proj.RemoveCustomProjection(code) })
	xy, probeError := proj.Convert(code, []float64{0, 0})
	if probeError == nil && finite(xy[0]) && finite(xy[1]) {
		t.Fatalf("fixture does not exclude arbitrary zero location: %v", xy)
	}
	layer := &testLayer{name: "local", srid: code}
	s, err := NewService([]CollectionSource{{ID: "public", Layer: layer, Querier: featureQuerier(&provider.Feature{ID: 1, SRID: code, Geometry: geom.Point{0, 0}})}})
	if err != nil {
		t.Fatalf("valid local domain rejected: %v", err)
	}
	if _, err := s.QueryFeature(context.Background(), "public", 1); err != nil {
		t.Fatalf("local center transform failed: %v", err)
	}
}
