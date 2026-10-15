package hana_test

import (
	"context"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/alexeydott/geom"
	"github.com/alexeydott/tegola/basic"
	"github.com/alexeydott/tegola/provider"
	"github.com/alexeydott/tegola/provider/crsconfig"
	"github.com/alexeydott/tegola/provider/hana"
	"github.com/alexeydott/tegola/provider/internal/querytest"
)

func TestFeatureCRSFrozenLive(t *testing.T) {
	if os.Getenv(TESTENV) != "yes" || GetConnectionURI() == "" {
		t.Skip("live HANA CRS proof requires explicit private connection")
	}
	db, err := hana.OpenDB(GetConnectionURI())
	if err != nil {
		t.Fatal("open version proof")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	var version string
	err = db.QueryRowContext(ctx, "SELECT VERSION FROM SYS.M_DATABASE").Scan(&version)
	cancel()
	closeErr := db.Close()
	if err != nil || closeErr != nil {
		t.Fatal("version proof failed")
	}
	t.Logf("HANA version %s", version)
	for _, format := range []string{"native", "wkb", "wkt"} {
		for _, custom := range []bool{false, true} {
			t.Run(format+map[bool]string{false: "/ordinary", true: "/custom"}[custom], func(t *testing.T) {
				fixture := querytest.Fixture{PublicFields: []string{}, Rows: []querytest.Row{
					{Feature: provider.Feature{ID: 1, Geometry: geom.Point{10, 20}}},
					{Feature: provider.Feature{ID: 2, Geometry: geom.Point{30, 40}}},
					{Feature: provider.Feature{ID: 3, Geometry: geom.Point{10.01, 20.01}}},
				}}
				if custom {
					fixture.Profile = querytest.CustomSelection
				}
				instance := hanaFeatureFactory(format)(t, fixture)
				if instance.SetupError != nil {
					t.Fatal(instance.SetupError)
				}
				p := instance.Querier.(*hana.Provider)
				layers, err := p.Layers()
				if err != nil {
					t.Fatal(err)
				}
				proof, err := layers[0].(provider.FeatureCRSLayerInfo).FeatureCRSDefinition()
				if err != nil || proof.HorizontalSRID != 4326 || proof.CanonicalCode != "4326" {
					t.Fatalf("source proof: %+v %v", proof, err)
				}
				definition, _ := crsconfig.CanonicalFeatureDefinition(3857)
				q := provider.FeatureQuery{Limit: 1, Offset: 1, BoundsSRID: 4326, BoundsCRSDefinition: definition, Bounds: []geom.Extent{{1100000, 2200000, 1120000, 2300000}}}
				var ids []uint64
				result, err := instance.Querier.QueryFeatures(context.Background(), instance.Layer, q, func(f *provider.Feature) error { ids = append(ids, f.ID); return nil })
				if err != nil || !reflect.DeepEqual(ids, []uint64{3}) || result.NumberReturned != 1 {
					t.Fatalf("exact-before-page %v %+v %v", ids, result, err)
				}
				if result.NumberMatched != nil && *result.NumberMatched != 2 {
					t.Fatal("coarse count leaked")
				}
				again, err := layers[0].(provider.FeatureCRSLayerInfo).FeatureCRSDefinition()
				if err != nil || proof != again {
					t.Fatal("source proof mutated during query")
				}
				oldSource, _ := basic.EffectiveProj4Definition(4326)
				oldTarget, _ := basic.EffectiveProj4Definition(3857)
				t.Cleanup(func() {
					if err := basic.RegisterProj4SRID(4326, oldSource); err != nil {
						t.Error(err)
					}
					if err := basic.RegisterProj4SRID(3857, oldTarget); err != nil {
						t.Error(err)
					}
				})
				if err := basic.RegisterProj4SRID(4326, definition); err != nil {
					t.Fatal(err)
				}
				if err := basic.RegisterProj4SRID(3857, oldSource); err != nil {
					t.Fatal(err)
				}
				ids = nil
				_, err = instance.Querier.QueryFeatures(context.Background(), instance.Layer, q, func(f *provider.Feature) error { ids = append(ids, f.ID); return nil })
				if err != nil || !reflect.DeepEqual(ids, []uint64{3}) {
					t.Fatal("mutable registry changed frozen query/source math", ids, err)
				}
			})
		}
	}
}

func TestFeatureCRSDimensionalFrozenLive(t *testing.T) {
	if os.Getenv(TESTENV) != "yes" || GetConnectionURI() == "" {
		t.Skip("live HANA dimensional CRS proof requires private connection")
	}
	for _, format := range []string{"native", "wkb", "wkt"} {
		for _, dimension := range []provider.CoordinateDimension{provider.DimensionXYZ, provider.DimensionMixedXYXYZ} {
			t.Run(format+map[provider.CoordinateDimension]string{provider.DimensionXYZ: "/xyz", provider.DimensionMixedXYXYZ: "/mixed"}[dimension], func(t *testing.T) {
				second := geom.Geometry(geom.PointZ{10.01, 20.01, 200})
				if dimension == provider.DimensionMixedXYXYZ {
					second = geom.Point{10.01, 20.01}
				}
				fixture := querytest.Fixture{PublicFields: []string{}, Spatial: provider.SpatialMetadata{Dimension: dimension, VerticalCRS: provider.CRS84h}, Rows: []querytest.Row{
					{Feature: provider.Feature{ID: 1, Geometry: geom.PointZ{10, 20, 7}}},
					{Feature: provider.Feature{ID: 2, Geometry: second}},
				}}
				instance := hanaFeatureFactory(format)(t, fixture)
				if instance.SetupError != nil {
					t.Fatal(instance.SetupError)
				}
				definition, _ := crsconfig.CanonicalFeatureDefinition(3857)
				q := provider.FeatureQuery{Limit: 10, BoundsSRID: 4326, BoundsCRSDefinition: definition, BoundsVerticalCRS: provider.CRS84h, Bounds3D: []provider.Extent3D{{1100000, 2200000, 6, 1120000, 2300000, 8}}}
				var ids []uint64
				_, err := instance.Querier.QueryFeatures(context.Background(), instance.Layer, q, func(f *provider.Feature) error {
					ids = append(ids, f.ID)
					if f.ID == 1 && f.Geometry != (geom.PointZ{10, 20, 7}) {
						t.Fatal("source height changed")
					}
					return nil
				})
				want := []uint64{1}
				if dimension == provider.DimensionMixedXYXYZ {
					want = []uint64{1, 2}
				}
				if err != nil || !reflect.DeepEqual(ids, want) {
					t.Fatal("frozen 3D query/missing-height policy", ids, err)
				}
			})
		}
	}
}
