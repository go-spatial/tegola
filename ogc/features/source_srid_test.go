package features

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/alexeydott/geom"
	"github.com/alexeydott/tegola/provider"
)

type sourceSRIDLayer struct {
	tile, source           uint64
	tileCalls, sourceCalls int
	xyz                    bool
}

func (*sourceSRIDLayer) Name() string            { return "source" }
func (l *sourceSRIDLayer) SRID() uint64          { l.tileCalls++; return l.tile }
func (*sourceSRIDLayer) GeomType() geom.Geometry { return geom.Point{} }
func (*sourceSRIDLayer) TemporalMapping() (provider.TemporalMapping, error) {
	return provider.TemporalMapping{}, nil
}
func (*sourceSRIDLayer) FeatureQuerySupported() error { return nil }
func (l *sourceSRIDLayer) SpatialMetadata() (provider.SpatialMetadata, error) {
	if l.xyz {
		return provider.SpatialMetadata{Dimension: provider.DimensionXYZ, VerticalCRS: provider.CRS84h}, nil
	}
	return provider.SpatialMetadata{Dimension: provider.DimensionXY}, nil
}
func (l *sourceSRIDLayer) FeatureSourceSRID() uint64 { l.sourceCalls++; return l.source }

// Deliberately exposes only the old interface, without embedding the new getter.
type legacySourceSRIDLayer struct{ layer *sourceSRIDLayer }

func (l legacySourceSRIDLayer) Name() string            { return l.layer.Name() }
func (l legacySourceSRIDLayer) SRID() uint64            { return l.layer.SRID() }
func (l legacySourceSRIDLayer) GeomType() geom.Geometry { return l.layer.GeomType() }
func (l legacySourceSRIDLayer) TemporalMapping() (provider.TemporalMapping, error) {
	return l.layer.TemporalMapping()
}
func (l legacySourceSRIDLayer) FeatureQuerySupported() error { return l.layer.FeatureQuerySupported() }
func (l legacySourceSRIDLayer) SpatialMetadata() (provider.SpatialMetadata, error) {
	return l.layer.SpatialMetadata()
}

type sourceSRIDQuerier struct{ feature provider.Feature }

func (q sourceSRIDQuerier) QueryFeatures(_ context.Context, _ string, _ provider.FeatureQuery, callback func(*provider.Feature) error) (provider.FeatureQueryResult, error) {
	f := q.feature
	if err := callback(&f); err != nil {
		return provider.FeatureQueryResult{}, err
	}
	return provider.FeatureQueryResult{NumberReturned: 1}, nil
}

func TestFeatureSourceSRIDFrozenIndependently(t *testing.T) {
	for _, xyz := range []bool{false, true} {
		t.Run(map[bool]string{false: "XY", true: "XYZ"}[xyz], func(t *testing.T) {
			layer := &sourceSRIDLayer{tile: 999999, source: 4326, xyz: xyz}
			var geometry geom.Geometry = geom.Point{15, 30}
			if xyz {
				geometry = geom.PointZ{15, 30, 7}
			}
			service, err := NewService([]CollectionSource{{ID: "public", Layer: layer, Querier: sourceSRIDQuerier{provider.Feature{ID: 1, SRID: 4326, Geometry: geometry}}}})
			if err != nil {
				t.Fatal(err)
			}
			if layer.tileCalls != 0 || layer.sourceCalls != 1 {
				t.Fatalf("wrong metadata getter calls tile=%d source=%d", layer.tileCalls, layer.sourceCalls)
			}
			layer.source = 0
			layer.tile = 3857
			f, err := service.QueryFeature(context.Background(), "public", 1)
			if err != nil {
				t.Fatal(err)
			}
			var decoded struct{ Coordinates []float64 }
			if err := json.Unmarshal(f.Geometry, &decoded); err != nil {
				t.Fatal(err)
			}
			if len(decoded.Coordinates) != map[bool]int{false: 2, true: 3}[xyz] || decoded.Coordinates[0] != 15 || decoded.Coordinates[1] != 30 || (xyz && decoded.Coordinates[2] != 7) {
				t.Fatal("source coordinates changed", decoded)
			}
			if service.collections["public"].srid != 4326 || layer.sourceCalls != 1 || layer.tileCalls != 0 {
				t.Fatal("source metadata was reread")
			}
		})
	}
}
func TestFeatureSourceSRIDZeroNilAndLegacyFallback(t *testing.T) {
	q := sourceSRIDQuerier{provider.Feature{ID: 1, SRID: 4326, Geometry: geom.Point{15, 30}}}
	for _, xyz := range []bool{false, true} {
		layer := &sourceSRIDLayer{tile: 4326, source: 0, xyz: xyz}
		_, err := NewService([]CollectionSource{{ID: "public", Layer: layer, Querier: q}})
		if err == nil || !errors.Is(err, provider.ErrUnsupported) || layer.tileCalls != 0 {
			t.Fatalf("zero source silently fell back: %v", err)
		}
	}
	var typedNil *sourceSRIDLayer
	for _, layer := range []provider.LayerInfo{nil, typedNil} {
		if _, err := NewService([]CollectionSource{{ID: "public", Layer: layer, Querier: q}}); err == nil {
			t.Fatal("nil layer accepted")
		}
	}
	legacy := &sourceSRIDLayer{tile: 4326}
	service, err := NewService([]CollectionSource{{ID: "public", Layer: legacySourceSRIDLayer{legacy}, Querier: q}})
	if err != nil {
		t.Fatal(err)
	}
	legacy.tile = 0
	if service.collections["public"].srid != 4326 || legacy.tileCalls != 1 || legacy.sourceCalls != 0 {
		t.Fatal("legacy fallback not frozen once")
	}
}
