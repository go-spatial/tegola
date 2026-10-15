package features

import (
	"context"
	"reflect"
	"sort"
	"sync"
	"testing"

	"github.com/alexeydott/tegola/provider"
)

func TestConformanceClassesAdmissionAndOwnership(t *testing.T) {
	base := []string{
		"http://www.opengis.net/spec/ogcapi-features-1/1.0/conf/core",
		"http://www.opengis.net/spec/ogcapi-features-1/1.0/conf/geojson",
		"http://www.opengis.net/spec/ogcapi-features-1/1.0/conf/html",
		"http://www.opengis.net/spec/ogcapi-features-1/1.0/conf/oas30",
	}
	crsURI := "http://www.opengis.net/spec/ogcapi-features-2/1.0/conf/crs"
	for _, service := range []*Service{nil, {}, {collections: map[string]resolvedCollection{}}} {
		if classes := service.ConformanceClasses(); classes == nil || len(classes) != 0 {
			t.Fatal("empty service manufactured classes", classes)
		}
	}
	projection, err := ResolveCRS(CRS84)
	if err != nil {
		t.Fatal(err)
	}
	spatial := provider.SpatialMetadata{Dimension: provider.DimensionXY}
	capable := &serviceCRSLayer{testLayer: testLayer{name: "source", srid: 4326}, spatial: spatial, proof: provider.FeatureCRSDefinition{HorizontalSRID: 4326, Definition: projection.Definition().Definition, CanonicalAuthority: "EPSG", CanonicalCode: "4326", Spatial: spatial}}
	core := &testLayer{name: "source", srid: 4326}
	noIO := testQuerier(func(context.Context, string, provider.FeatureQuery, func(*provider.Feature) error) (provider.FeatureQueryResult, error) {
		t.Error("conformance performed provider I/O")
		return provider.FeatureQueryResult{}, nil
	})
	for _, tc := range []struct {
		name   string
		layers []provider.LayerInfo
		crs    bool
	}{
		{"core", []provider.LayerInfo{core}, false},
		{"all-capable", []provider.LayerInfo{capable, capable}, true},
		{"mixed-publication", []provider.LayerInfo{capable, core}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sources := make([]CollectionSource, len(tc.layers))
			for i, layer := range tc.layers {
				sources[i] = CollectionSource{ID: string(rune('a' + i)), Layer: layer, Querier: noIO}
			}
			service, err := NewService(sources)
			if err != nil {
				t.Fatal(err)
			}
			want := append([]string(nil), base...)
			if tc.crs {
				want = append(want, crsURI)
			}
			sort.Strings(want)
			classes := service.ConformanceClasses()
			if !reflect.DeepEqual(classes, want) {
				t.Fatal(classes, want)
			}
			classes[0] = "untrusted"
			if !reflect.DeepEqual(service.ConformanceClasses(), want) {
				t.Fatal("caller mutated registry")
			}
			var group sync.WaitGroup
			for i := 0; i < 16; i++ {
				group.Add(1)
				go func() {
					defer group.Done()
					if !reflect.DeepEqual(service.ConformanceClasses(), want) {
						t.Error("unstable declaration")
					}
				}()
			}
			group.Wait()
		})
	}
}

func TestConformanceClassesFrozenCRSAndHeightProfiles(t *testing.T) {
	for _, dimension := range []provider.CoordinateDimension{provider.DimensionXYZ, provider.DimensionMixedXYXYZ} {
		service, _ := serviceCRSFixture(t, dimension, nil)
		if classes := service.ConformanceClasses(); len(classes) != 5 || classes[4] != ConformanceCRS {
			t.Fatal("accepted height catalog lost admission", classes)
		}
	}
	service, _ := serviceCRSFixture(t, provider.DimensionXY, nil)
	collection := service.collections["public"]
	collection.crs = CollectionCRS{}
	service.collections["public"] = collection
	if len(service.ConformanceClasses()) != 4 {
		t.Fatal("uninitialized CRS catalog manufactured extension")
	}
}

type conformanceMetadataLayer struct {
	serviceCRSLayer
	forbid bool
}

func (layer *conformanceMetadataLayer) FeatureCRSDefinition() (provider.FeatureCRSDefinition, error) {
	if layer.forbid {
		panic("conformance fetched live provider metadata")
	}
	return layer.proof, nil
}

func TestConformanceClassesDoNotReadLiveMetadata(t *testing.T) {
	descriptor, err := ResolveCRS(CRS84)
	if err != nil {
		t.Fatal(err)
	}
	spatial := provider.SpatialMetadata{Dimension: provider.DimensionXY}
	layer := &conformanceMetadataLayer{serviceCRSLayer: serviceCRSLayer{testLayer: testLayer{name: "source", srid: 4326}, spatial: spatial, proof: provider.FeatureCRSDefinition{HorizontalSRID: 4326, Definition: descriptor.Definition().Definition, CanonicalAuthority: "EPSG", CanonicalCode: "4326", Spatial: spatial}}}
	service, err := NewService([]CollectionSource{{ID: "public", Layer: layer, Querier: testQuerier(emptyQuerier)}})
	if err != nil {
		t.Fatal(err)
	}
	layer.forbid = true
	layer.proof = provider.FeatureCRSDefinition{}
	if len(service.ConformanceClasses()) != 5 {
		t.Fatal("live source mutation changed publication admission")
	}
}
