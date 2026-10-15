package features

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"reflect"
	"sync"
	"testing"

	"github.com/alexeydott/geom"
	"github.com/alexeydott/tegola/provider"
)

type serviceCRSLayer struct {
	testLayer
	spatial    provider.SpatialMetadata
	proof      provider.FeatureCRSDefinition
	proofError error
}

func (l *serviceCRSLayer) SpatialMetadata() (provider.SpatialMetadata, error) { return l.spatial, nil }
func (l *serviceCRSLayer) FeatureCRSDefinition() (provider.FeatureCRSDefinition, error) {
	return l.proof, l.proofError
}
func serviceCRSFixture(t *testing.T, dimension provider.CoordinateDimension, geometry geom.Geometry) (*Service, *provider.Feature) {
	t.Helper()
	descriptor, err := ResolveCRS(CRS84)
	if err != nil {
		t.Fatal(err)
	}
	spatial := provider.SpatialMetadata{Dimension: dimension}
	if dimension != provider.DimensionXY {
		spatial.VerticalCRS = provider.CRS84h
	}
	layer := &serviceCRSLayer{testLayer: testLayer{name: "source", srid: 4326}, spatial: spatial, proof: provider.FeatureCRSDefinition{HorizontalSRID: 4326, Definition: descriptor.Definition().Definition, CanonicalAuthority: "EPSG", CanonicalCode: "4326", Spatial: spatial}}
	source := &provider.Feature{ID: 7, SRID: 4326, Geometry: geometry, Tags: map[string]any{"null": nil, "nested": []any{"original"}}}
	service, err := NewService([]CollectionSource{{ID: "public", Layer: layer, Querier: featureQuerier(source)}})
	if err != nil {
		t.Fatal(err)
	}
	return service, source
}
func TestServiceCRSOutputAxesAndProjection(t *testing.T) {
	service, source := serviceCRSFixture(t, provider.DimensionXY, geom.Point{15, 4})
	for _, tc := range []struct {
		uri      string
		expected [2]float64
	}{{CRS84, [2]float64{15, 4}}, {"http://www.opengis.net/def/crs/EPSG/0/4326", [2]float64{4, 15}}, {"http://www.opengis.net/def/crs/EPSG/0/3857", [2]float64{1669792.3618991035, 445640.10965602624}}} {
		value, err := service.QueryFeatureWithOptions(context.Background(), "public", 7, QueryOptions{OutputCRS: tc.uri})
		if err != nil {
			t.Fatal(err)
		}
		var geometry struct{ Coordinates [2]float64 }
		if err := json.Unmarshal(value.Geometry, &geometry); err != nil {
			t.Fatal(err)
		}
		for i := range tc.expected {
			if math.Abs(geometry.Coordinates[i]-tc.expected[i]) > 1e-7 {
				t.Fatalf("%s: %v", tc.uri, geometry.Coordinates)
			}
		}
		value.Properties["nested"].([]any)[0] = "changed"
	}
	if !reflect.DeepEqual(source.Geometry, geom.Point{15, 4}) || source.Tags["nested"].([]any)[0] != "original" {
		t.Fatal("source mutated")
	}
	catalog, _ := service.CollectionCRS("public")
	values := catalog.URIs()
	values[0] = "changed"
	again, _ := service.CollectionCRS("public")
	if again.URIs()[0] != CRS84 {
		t.Fatal("catalog mutated")
	}
}
func TestServiceCRSMixedAndUnsupportedOutput(t *testing.T) {
	service, _ := serviceCRSFixture(t, provider.DimensionMixedXYXYZ, geom.Collection{geom.Point{15, 4}, geom.PointZ{15, 4, 29}})
	value, err := service.QueryFeatureWithOptions(context.Background(), "public", 7, QueryOptions{OutputCRS: "http://www.opengis.net/def/crs/EPSG/0/4979"})
	if err != nil {
		t.Fatal(err)
	}
	var collection struct {
		Geometries []struct{ Coordinates []float64 }
	}
	if err := json.Unmarshal(value.Geometry, &collection); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(collection.Geometries[0].Coordinates, []float64{4, 15}) || !reflect.DeepEqual(collection.Geometries[1].Coordinates, []float64{4, 15, 29}) {
		t.Fatalf("height/axis changed: %s", value.Geometry)
	}
	_, err = service.QueryFeatureWithOptions(context.Background(), "public", 7, QueryOptions{OutputCRS: CRS84})
	var invalid provider.InvalidFeatureQueryError
	if !errors.As(err, &invalid) {
		t.Fatalf("2D mixed output: %v", err)
	}
}
func TestServiceCRSMissingProofPreservesCore(t *testing.T) {
	service := newTestService(t, 4326, featureQuerier(&provider.Feature{ID: 7, SRID: 4326, Geometry: geom.Point{15, 4}}))
	if _, err := service.QueryFeature(context.Background(), "public", 7); err != nil {
		t.Fatal(err)
	}
	if _, err := service.CollectionCRS("public"); !errors.Is(err, provider.ErrUnsupported) {
		t.Fatalf("missing proof: %v", err)
	}
	if uri, err := service.DefaultCRSURI("public"); err != nil || uri != CRS84 {
		t.Fatalf("default %s %v", uri, err)
	}
	_, err := service.QueryFeatureWithOptions(context.Background(), "public", 7, QueryOptions{OutputCRS: CRS84})
	var invalid provider.InvalidFeatureQueryError
	if !errors.As(err, &invalid) {
		t.Fatalf("explicit unavailable: %v", err)
	}
}

func TestServiceCRSSourceErrorsAndConcurrency(t *testing.T) {
	service, _ := serviceCRSFixture(t, provider.DimensionXY, geom.Point{15, 4})
	var workers sync.WaitGroup
	for range 8 {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for range 8 {
				value, err := service.QueryFeatureWithOptions(context.Background(), "public", 7, QueryOptions{OutputCRS: "http://www.opengis.net/def/crs/EPSG/0/4326"})
				if err != nil {
					t.Error(err)
					return
				}
				value.Properties["nested"].([]any)[0] = "detached"
			}
		}()
	}
	workers.Wait()
	malformed, _ := serviceCRSFixture(t, provider.DimensionXYZ, geom.PointZ{15, 4, math.NaN()})
	_, err := malformed.QueryFeatureWithOptions(context.Background(), "public", 7, QueryOptions{OutputCRS: CRS84h})
	var data provider.FeatureDataError
	if !errors.As(err, &data) {
		t.Fatalf("malformed source must be data error: %v", err)
	}
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := service.QueryFeatureWithOptions(canceled, "public", 7, QueryOptions{OutputCRS: CRS84}); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancel chain %v", err)
	}
}
func TestServiceCRSRejectsClaimedMetadataMismatch(t *testing.T) {
	descriptor, err := ResolveCRS(CRS84)
	if err != nil {
		t.Fatal(err)
	}
	spatial := provider.SpatialMetadata{Dimension: provider.DimensionXY}
	layer := &serviceCRSLayer{testLayer: testLayer{name: "source", srid: 4326}, spatial: spatial, proof: provider.FeatureCRSDefinition{HorizontalSRID: 3857, Definition: descriptor.Definition().Definition, Spatial: spatial}}
	if _, err := NewService([]CollectionSource{{ID: "public", Layer: layer, Querier: testQuerier(emptyQuerier)}}); err == nil {
		t.Fatal("inconsistent source proof admitted")
	}
}

func TestServiceCRSRejectsFalseAuthorityAndInvalidSourceDomain(t *testing.T) {
	descriptor, err := ResolveCRS("http://www.opengis.net/def/crs/EPSG/0/3857")
	if err != nil {
		t.Fatal(err)
	}
	spatial := provider.SpatialMetadata{Dimension: provider.DimensionXY}
	layer := &serviceCRSLayer{testLayer: testLayer{name: "source", srid: 4326}, spatial: spatial, proof: provider.FeatureCRSDefinition{HorizontalSRID: 4326, Definition: descriptor.Definition().Definition, CanonicalAuthority: "EPSG", CanonicalCode: "4326", Spatial: spatial}}
	if _, err := NewService([]CollectionSource{{ID: "public", Layer: layer, Querier: testQuerier(emptyQuerier)}}); err == nil {
		t.Fatal("false authority admitted")
	}
	service, _ := serviceCRSFixture(t, provider.DimensionXY, geom.Point{15, 91})
	_, err = service.QueryFeatureWithOptions(context.Background(), "public", 7, QueryOptions{OutputCRS: CRS84})
	var data provider.FeatureDataError
	if !errors.As(err, &data) {
		t.Fatalf("invalid source domain %v", err)
	}
}

func TestServiceCRSRequestedNonplanarityIsClientError(t *testing.T) {
	polygon := geom.PolygonZ{{{0, 0, 0}, {1, 0, 0}, {1, 1, 1}, {0.5, 2, 2}, {0, 1, 1}, {0, 0, 0}}}
	service, source := serviceCRSFixture(t, provider.DimensionXYZ, polygon)
	catalog, err := service.CollectionCRS("public")
	if err != nil {
		t.Fatal(err)
	}
	target := ""
	for _, uri := range catalog.URIs() {
		descriptor, err := catalog.Resolve(uri)
		if err != nil {
			t.Fatal(err)
		}
		if descriptor.InternalSRID() == 3857 {
			target = uri
		}
	}
	if target == "" {
		t.Fatal("projected XYZ target missing")
	}
	_, err = service.QueryFeatureWithOptions(context.Background(), "public", 7, QueryOptions{OutputCRS: target})
	var invalid provider.InvalidFeatureQueryError
	var data provider.FeatureDataError
	if !errors.As(err, &invalid) || errors.As(err, &data) {
		t.Fatalf("requested nonplanarity classification %v", err)
	}
	if !reflect.DeepEqual(source.Geometry, polygon) {
		t.Fatal("source polygon mutated")
	}
}

func TestServiceCRSInvalidSourcePreservesUnsupportedChain(t *testing.T) {
	bad := geom.PolygonZ{{{0, 0, 0}, {1, 0, 0}, {1, 1, 1}, {0, 1, 0}}}
	for _, geometry := range []geom.Geometry{bad, geom.Collection{geom.Point{0, 0}, bad}} {
		service, _ := serviceCRSFixture(t, provider.DimensionMixedXYXYZ, geometry)
		_, err := service.QueryFeature(context.Background(), "public", 7)
		var data provider.FeatureDataError
		if !errors.As(err, &data) || !errors.Is(err, provider.ErrUnsupported) {
			t.Fatalf("source classification or sentinel lost: %v", err)
		}
	}
}
