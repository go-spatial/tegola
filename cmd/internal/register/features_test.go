package register

import (
	"context"
	"errors"
	"testing"

	"github.com/alexeydott/geom"
	"github.com/alexeydott/tegola/config"
	"github.com/alexeydott/tegola/provider"
)

type featureLayer struct {
	name        string
	eligibility error
}

func (l featureLayer) Name() string            { return l.name }
func (l featureLayer) SRID() uint64            { return 4326 }
func (l featureLayer) GeomType() geom.Geometry { return geom.Point{} }
func (l featureLayer) TemporalMapping() (provider.TemporalMapping, error) {
	return provider.TemporalMapping{}, nil
}
func (l featureLayer) FeatureQuerySupported() error { return l.eligibility }
func (featureLayer) SpatialMetadata() (provider.SpatialMetadata, error) {
	return provider.SpatialMetadata{Dimension: provider.DimensionXY}, nil
}

type featureProvider struct {
	layers []provider.LayerInfo
	err    error
}

func (p *featureProvider) Layers() ([]provider.LayerInfo, error) { return p.layers, p.err }
func (*featureProvider) TileFeatures(context.Context, string, provider.Tile, provider.Params, func(*provider.Feature) error) error {
	panic("tile path must not run")
}
func (*featureProvider) QueryFeatures(context.Context, string, provider.FeatureQuery, func(*provider.Feature) error) (provider.FeatureQueryResult, error) {
	return provider.FeatureQueryResult{}, nil
}

// Embed only the Tiler interface to hide the raw optional capability.
type rawIneligible struct{ provider.Tiler }

type featureMVT struct{ *featureProvider }

func (*featureMVT) MVTForLayers(context.Context, provider.Tile, provider.Params, []provider.Layer) ([]byte, error) {
	panic("MVT path must not run")
}

func TestFeatureBinding(t *testing.T) {
	cfg := config.FeaturesConfig{Enabled: true, Collections: []config.FeatureCollectionConfig{{ID: "roads", ProviderLayer: "data.source", Title: "Public roads", Description: "Published"}}}
	p := &featureProvider{layers: []provider.LayerInfo{featureLayer{name: "source"}}}
	providers := map[string]provider.TilerUnion{"data": {Std: p}}
	s, err := Features(cfg, providers)
	if err != nil {
		t.Fatal(err)
	}
	metadata, err := s.Collection("roads")
	if err != nil || metadata.Title != "Public roads" || metadata.Description != "Published" {
		t.Fatalf("metadata %v %v", metadata, err)
	}
	cfg.Collections[0].Title = "changed"
	metadata, _ = s.Collection("roads")
	if metadata.Title != "Public roads" {
		t.Fatal("configuration retained")
	}
	cfg.Enabled = false
	if s, err := Features(cfg, nil); s != nil || err != nil {
		t.Fatalf("disabled resolved: %v %v", s, err)
	}
	cfg.BasePath = "/maps"
	if _, err := Features(cfg, nil); err == nil {
		t.Fatal("disabled invalid syntax accepted")
	}
}

func TestFeatureBindingFailures(t *testing.T) {
	cfg := config.FeaturesConfig{Enabled: true, Collections: []config.FeatureCollectionConfig{{ID: "roads", ProviderLayer: "data.source"}}}
	sentinel := errors.New("metadata read failed")
	var nilProvider *featureProvider
	cases := map[string]map[string]provider.TilerUnion{
		"missing provider":   nil,
		"empty union":        {"data": {}},
		"typed nil provider": {"data": {Std: nilProvider}},
		"MVT provider":       {"data": {Mvt: &featureMVT{&featureProvider{}}}},
		"tile only":          {"data": {Std: rawIneligible{&featureProvider{}}}},
		"missing layer":      {"data": {Std: &featureProvider{}}},
		"duplicate layer":    {"data": {Std: &featureProvider{layers: []provider.LayerInfo{featureLayer{name: "source"}, featureLayer{name: "source"}}}}},
		"unknown metadata":   {"data": {Std: &featureProvider{layers: []provider.LayerInfo{plainFeatureLayer{}}}}},
		"ineligible":         {"data": {Std: &featureProvider{layers: []provider.LayerInfo{featureLayer{name: "source", eligibility: provider.ErrUnsupported}}}}},
		"metadata error":     {"data": {Std: &featureProvider{err: sentinel}}},
	}
	for name, providers := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := Features(cfg, providers); err == nil {
				t.Fatal("invalid publication accepted")
			}
		})
	}
	if _, err := Features(cfg, cases["metadata error"]); !errors.Is(err, sentinel) {
		t.Fatalf("lost error chain: %v", err)
	}
	if _, err := Features(cfg, cases["ineligible"]); !errors.Is(err, provider.ErrUnsupported) {
		t.Fatalf("lost eligibility: %v", err)
	}
}

type plainFeatureLayer struct{}

func (plainFeatureLayer) Name() string            { return "source" }
func (plainFeatureLayer) SRID() uint64            { return 4326 }
func (plainFeatureLayer) GeomType() geom.Geometry { return geom.Point{} }
