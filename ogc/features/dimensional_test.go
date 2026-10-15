package features

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"reflect"
	"testing"

	"github.com/alexeydott/geom"
	"github.com/alexeydott/proj"
	"github.com/alexeydott/tegola/provider"
	codec "github.com/alexeydott/tegola/provider/geometrycodec"
)

type dimensionalLayer struct {
	testLayer
	spatial      provider.SpatialMetadata
	spatialError error
}

func (l *dimensionalLayer) SpatialMetadata() (provider.SpatialMetadata, error) {
	return l.spatial, l.spatialError
}

type temporalOnlyLayer struct{ provider.LayerInfo }

func (temporalOnlyLayer) FeatureQuerySupported() error { return nil }
func (temporalOnlyLayer) TemporalMapping() (provider.TemporalMapping, error) {
	return provider.TemporalMapping{}, nil
}

func dimensionalService(t *testing.T, srid uint64, dimension provider.CoordinateDimension, g geom.Geometry) *Service {
	t.Helper()
	layer := &dimensionalLayer{testLayer: testLayer{name: "source", srid: srid}, spatial: provider.SpatialMetadata{Dimension: dimension, VerticalCRS: provider.CRS84h}}
	s, err := NewService([]CollectionSource{{ID: "public", Layer: layer, Querier: featureQuerier(&provider.Feature{ID: 1, SRID: srid, Geometry: g})}})
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestDimensionalCatalogAdmissionAndSnapshot(t *testing.T) {
	layer := &dimensionalLayer{testLayer: testLayer{name: "source", srid: 4326}, spatial: provider.SpatialMetadata{Dimension: provider.DimensionXYZ, VerticalCRS: provider.CRS84h}}
	source := CollectionSource{ID: "public", Layer: layer, Querier: testQuerier(emptyQuerier)}
	s, err := NewService([]CollectionSource{source})
	if err != nil {
		t.Fatal(err)
	}
	layer.spatial.Dimension = provider.DimensionXY
	layer.spatial.VerticalCRS = ""
	if s.collections["public"].spatial.Dimension != provider.DimensionXYZ || s.collections["public"].spatial.VerticalCRS != provider.CRS84h {
		t.Fatal("spatial metadata retained")
	}
	source.Layer = temporalOnlyLayer{&testLayer{name: "source", srid: 4326}}
	if _, err := NewService([]CollectionSource{source}); !errors.Is(err, provider.ErrUnsupported) {
		t.Fatalf("unknown spatial capability admitted: %v", err)
	}
	for _, metadata := range []provider.SpatialMetadata{{}, {Dimension: provider.DimensionXY, VerticalCRS: provider.CRS84h}, {Dimension: provider.DimensionXYZ}} {
		layer.spatial = metadata
		source.Layer = layer
		if _, err := NewService([]CollectionSource{source}); err == nil {
			t.Fatal("invalid metadata admitted")
		}
	}
	layer.spatial = provider.SpatialMetadata{Dimension: provider.DimensionXYZ, VerticalCRS: provider.CRS84h}
	layer.srid = 4269
	if _, err := NewService([]CollectionSource{source}); !errors.Is(err, provider.ErrUnsupported) {
		t.Fatalf("unsupported datum profile admitted: %v", err)
	}
	sentinel := errors.New("source metadata failure")
	layer.spatialError = sentinel
	if _, err := NewService([]CollectionSource{source}); !errors.Is(err, sentinel) {
		t.Fatalf("metadata error lost: %v", err)
	}
}

func TestDimensionalCanonicalOutputAndRegistryIsolation(t *testing.T) {
	// Independent spherical Mercator equation at longitude10, latitude0.
	x := 6378137.0 * math.Pi / 18
	input := geom.Collection{geom.PointZ{x, 0, 12.125}, geom.Point{x, 0}}
	s := dimensionalService(t, 3857, provider.DimensionMixedXYXYZ, input)
	proj.CustomProjection(3857, "+proj=longlat +ellps=krass +towgs84=10,20,30")
	// EPSG3857 is a built-in registry entry; deleting it would affect later
	// XY regression tests. Restore its package-default definition explicitly.
	t.Cleanup(func() {
		proj.CustomProjection(3857, "+proj=merc +a=6378137 +b=6378137 +lat_ts=0.0 +lon_0=0.0 +x_0=0.0 +y_0=0 +k=1.0")
	})
	value, err := s.QueryFeature(context.Background(), "public", 1)
	if err != nil {
		t.Fatal(err)
	}
	var g struct {
		Geometries []struct{ Coordinates []float64 }
	}
	if err := json.Unmarshal(value.Geometry, &g); err != nil {
		t.Fatal(err)
	}
	if len(g.Geometries) != 2 || len(g.Geometries[0].Coordinates) != 3 || len(g.Geometries[1].Coordinates) != 2 {
		t.Fatalf("dimension lost: %s", value.Geometry)
	}
	if math.Abs(g.Geometries[0].Coordinates[0]-10) > 1e-12 || g.Geometries[0].Coordinates[1] != 0 || g.Geometries[0].Coordinates[2] != 12.125 {
		t.Fatalf("canonical output: %s", value.Geometry)
	}
	if input[0].(geom.PointZ)[0] != x || input[0].(geom.PointZ)[2] != 12.125 {
		t.Fatal("source mutated")
	}
}

func TestDimensionalPolygonFamiliesClosureAndVerticalWinding(t *testing.T) {
	vertical := geom.PolygonZ{{{5, 0, 0}, {5, 0, 10}, {5, 10, 10}, {5, 10, 0}}, {{5, 3, 3}, {5, 7, 3}, {5, 7, 7}, {5, 3, 7}}}
	before, _ := json.Marshal(vertical)
	for _, g := range []geom.Geometry{vertical, codec.MultiPolygonZ{vertical}, geom.Collection{codec.MultiPolygonZ{vertical}}} {
		s := dimensionalService(t, 4326, provider.DimensionXYZ, g)
		value, err := s.QueryFeature(context.Background(), "public", 1)
		if err != nil {
			t.Fatal(err)
		}
		var shape map[string]any
		if err := json.Unmarshal(value.Geometry, &shape); err != nil {
			t.Fatal(err)
		}
		if shape["type"] == "GeometryCollection" {
			shape = shape["geometries"].([]any)[0].(map[string]any)
		}
		coordinates := shape["coordinates"].([]any)
		if shape["type"] == "MultiPolygon" {
			coordinates = coordinates[0].([]any)
		}
		if len(coordinates) != 2 {
			t.Fatal("hole lost")
		}
		for i, v := range coordinates {
			ring := v.([]any)
			if len(ring) != 5 || !reflect.DeepEqual(ring[0], ring[4]) {
				t.Fatalf("closure lost: %v", ring)
			}
			var area float64
			for j := 1; j < len(ring); j++ {
				a, b := ring[j-1].([]any), ring[j].([]any)
				if len(a) != 3 || a[0].(float64) != 5 {
					t.Fatal("XYZ altered")
				}
				area += a[1].(float64)*b[2].(float64) - b[1].(float64)*a[2].(float64)
			}
			if i == 0 && area <= 0 || i != 0 && area >= 0 {
				t.Fatalf("in-plane winding: %v", ring)
			}
		}
	}
	after, _ := json.Marshal(vertical)
	if string(before) != string(after) {
		t.Fatal("normalization mutated source")
	}
}

func TestDimensionalAbsenceAndMixedEmptyChildren(t *testing.T) {
	for _, g := range []geom.Geometry{nil, geom.LineStringZ{}, geom.Collection{nil, codec.MultiPolygonZ{nil}}} {
		value, err := dimensionalService(t, 4326, provider.DimensionMixedXYXYZ, g).QueryFeature(context.Background(), "public", 1)
		if err != nil || string(value.Geometry) != "null" {
			t.Fatalf("absence: %s %v", value.Geometry, err)
		}
	}
	g := geom.Collection{nil, geom.MultiLineStringZ{nil, {{0, 0, 5}, {1, 1, 6}}}, geom.Collection{geom.LineString{}, geom.Point{2, 3}}, codec.MultiPolygonZ{nil, {{{0, 0, 7}, {1, 0, 7}, {0, 1, 7}}}}}
	value, err := dimensionalService(t, 4326, provider.DimensionMixedXYXYZ, g).QueryFeature(context.Background(), "public", 1)
	if err != nil {
		t.Fatal(err)
	}
	var shape struct {
		Geometries []struct {
			Type        string
			Coordinates json.RawMessage
			Geometries  []struct{ Type string }
		}
	}
	if err := json.Unmarshal(value.Geometry, &shape); err != nil {
		t.Fatal(err)
	}
	if len(shape.Geometries) != 3 || shape.Geometries[0].Type != "MultiLineString" || shape.Geometries[1].Type != "GeometryCollection" || len(shape.Geometries[1].Geometries) != 1 || shape.Geometries[2].Type != "MultiPolygon" {
		t.Fatalf("nonempty family/order lost: %s", value.Geometry)
	}
	if len(g) != 4 || len(g[1].(geom.MultiLineStringZ)) != 2 || len(g[2].(geom.Collection)) != 2 {
		t.Fatal("source containers mutated")
	}
}

func TestDimensionalIntegrityAndQueryPreflight(t *testing.T) {
	bad := geom.PolygonZ{{{0, 0, 0}, {1, 0, 0}, {1, 1, 1}, {0, 1, 0}}}
	for _, g := range []geom.Geometry{bad, geom.Collection{geom.Point{0, 0}, bad}, geom.PointZ{0, 0, math.Inf(1)}} {
		_, err := dimensionalService(t, 4326, provider.DimensionMixedXYXYZ, g).QueryFeature(context.Background(), "public", 1)
		if err == nil {
			t.Fatal("invalid geometry emitted")
		}
		if _, ok := g.(geom.PointZ); !ok && !errors.Is(err, provider.ErrUnsupported) {
			t.Fatalf("unsupported chain lost: %v", err)
		}
	}
	for _, tc := range []struct {
		dimension provider.CoordinateDimension
		g         geom.Geometry
	}{{provider.DimensionXYZ, geom.Point{0, 0}}, {provider.DimensionXY, geom.PointZ{0, 0, 1}}} {
		layer := &dimensionalLayer{testLayer: testLayer{name: "source", srid: 4326}, spatial: provider.SpatialMetadata{Dimension: tc.dimension}}
		if tc.dimension == provider.DimensionXYZ {
			layer.spatial.VerticalCRS = provider.CRS84h
		}
		s, err := NewService([]CollectionSource{{ID: "public", Layer: layer, Querier: featureQuerier(&provider.Feature{ID: 1, SRID: 4326, Geometry: tc.g})}})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := s.QueryFeature(context.Background(), "public", 1); err == nil {
			t.Fatal("dimension mismatch without bounds accepted")
		}
	}
	calls := 0
	layer := &dimensionalLayer{testLayer: testLayer{name: "source", srid: 4326}, spatial: provider.SpatialMetadata{Dimension: provider.DimensionXYZ, VerticalCRS: provider.CRS84h}}
	s, err := NewService([]CollectionSource{{ID: "public", Layer: layer, Querier: testQuerier(func(context.Context, string, provider.FeatureQuery, func(*provider.Feature) error) (provider.FeatureQueryResult, error) {
		calls++
		return provider.FeatureQueryResult{}, nil
	})}})
	if err != nil {
		t.Fatal(err)
	}
	for _, q := range []provider.FeatureQuery{{Limit: 1, Bounds: []geom.Extent{{0, 0, 1, 1}}, BoundsSRID: 4269}, {Limit: 1, Bounds3D: []provider.Extent3D{{0, 0, 0, 1, 1, 1}}, BoundsSRID: 4326, BoundsVerticalCRS: "other"}} {
		if _, err := s.QueryCollectionPage(context.Background(), "public", q); !errors.Is(err, provider.ErrUnsupported) {
			t.Fatalf("unsupported preflight: %v", err)
		}
	}
	if calls != 0 {
		t.Fatal("unsupported query reached provider")
	}
	// z=y defines a source plane, but inverse Mercator curves latitude; four
	// transformed vertices with three y values do not define one exact plane.
	g := geom.PolygonZ{{{0, 0, 0}, {1e6, 0, 0}, {1e6, 1e6, 1e6}, {0, 2e6, 2e6}}}
	if _, err := dimensionalService(t, 3857, provider.DimensionXYZ, g).QueryFeature(context.Background(), "public", 1); !errors.Is(err, provider.ErrUnsupported) {
		t.Fatalf("nonplanar output emitted: %v", err)
	}
}

func TestDimensionalFamiliesAndHeightBits(t *testing.T) {
	z := math.Copysign(0, -1)
	for _, g := range []geom.Geometry{geom.PointZ{1, 2, z}, geom.MultiPointZ{{1, 2, z}}, geom.LineStringZ{{1, 2, z}, {3, 4, 5}}, geom.MultiLineStringZ{{{1, 2, z}, {3, 4, 5}}}} {
		value, err := dimensionalService(t, 4326, provider.DimensionXYZ, g).QueryFeature(context.Background(), "public", 1)
		if err != nil {
			t.Fatal(err)
		}
		var shape struct {
			Type        string
			Coordinates any
		}
		if err := json.Unmarshal(value.Geometry, &shape); err != nil {
			t.Fatal(err)
		}
		var position []any
		switch shape.Type {
		case "Point":
			position = shape.Coordinates.([]any)
		case "MultiPoint", "LineString":
			position = shape.Coordinates.([]any)[0].([]any)
		case "MultiLineString":
			position = shape.Coordinates.([]any)[0].([]any)[0].([]any)
		default:
			t.Fatalf("family lost: %s", value.Geometry)
		}
		if len(position) != 3 || math.Float64bits(position[2].(float64)) != math.Float64bits(z) {
			t.Fatalf("height bits lost: %s", value.Geometry)
		}
	}
}

func TestDimensionalQueryOwnershipAndErrorChains(t *testing.T) {
	q := provider.FeatureQuery{Limit: 2, Offset: 3, Bounds3D: []provider.Extent3D{{0, 0, 100, 1, 1, 101}}, BoundsSRID: 4326, BoundsVerticalCRS: provider.CRS84h}
	layer := &dimensionalLayer{testLayer: testLayer{name: "source", srid: 4326}, spatial: provider.SpatialMetadata{Dimension: provider.DimensionMixedXYXYZ, VerticalCRS: provider.CRS84h}}
	sentinel := errors.New("callback failed")
	s, err := NewService([]CollectionSource{{ID: "public", Layer: layer, Querier: testQuerier(func(ctx context.Context, _ string, got provider.FeatureQuery, fn func(*provider.Feature) error) (provider.FeatureQueryResult, error) {
		if !reflect.DeepEqual(got, q) {
			t.Fatal("dimensional query changed")
		}
		if err := ctx.Err(); err != nil {
			return provider.FeatureQueryResult{}, err
		}
		return provider.FeatureQueryResult{}, fn(&provider.Feature{ID: 1, SRID: 4326, Geometry: geom.PointZ{0, 0, 100}})
	})}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.QueryCollection(context.Background(), "public", q, func(Feature) error { return sentinel }); !errors.Is(err, sentinel) {
		t.Fatalf("callback chain lost: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := s.QueryCollectionPage(ctx, "public", q); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancel chain lost: %v", err)
	}
	if q.Bounds3D[0][2] != 100 || q.Offset != 3 {
		t.Fatal("query input mutated")
	}
}
