package postgis

import (
	"context"
	"fmt"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/alexeydott/geom"
	"github.com/alexeydott/tegola/basic"
	"github.com/alexeydott/tegola/dict"
	"github.com/alexeydott/tegola/provider"
	"github.com/alexeydott/tegola/provider/crsconfig"
	"github.com/alexeydott/tegola/provider/internal/querytest"
)

func TestFeatureCRSFrozenLive(t *testing.T) {
	for _, srid := range []uint64{4326, 3857, 32633} {
		for _, custom := range []bool{false, true} {
			t.Run(fmt.Sprintf("%d/custom=%t", srid, custom), func(t *testing.T) {
				db, schema := featureLiveDatabase(t)
				ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
				defer cancel()
				qualified := pgQuoteIdent(schema) + `."crs_items"`
				if _, err := db.Exec(ctx, fmt.Sprintf("CREATE TABLE %s (id bigint PRIMARY KEY,geom geometry(Point,%d),included integer)", qualified, srid)); err != nil {
					t.Fatal(err)
				}
				points := map[uint64][2][2]float64{4326: {{10, 20}, {30, 40}}, 3857: {{1113194.9079327357, 2273030.9269876895}, {3339584.723798207, 4865942.279503175}}, 32633: {{500000, 0}, {700000, 1000000}}}[srid]
				for i, point := range points {
					if _, err := db.Exec(ctx, "INSERT INTO "+qualified+" VALUES ($1,ST_SetSRID(ST_MakePoint($2,$3),$4),1)", i+1, point[0], point[1], int(srid)); err != nil {
						t.Fatal(err)
					}
				}
				layer := map[string]any{"name": "items", "tablename": qualified, "id_fieldname": "id", "geometry_fieldname": "geom", "geometry_type": "point", "srid": int(srid), "fields": []string{"id"}}
				if custom {
					layer["feature_sql"] = "SELECT id,geom FROM " + qualified + " WHERE included=1"
				}
				tiler, err := NewTileProvider(dict.Dict{"name": "live_crs_features", "uri": os.Getenv("PGURI"), "layers": []map[string]any{layer}}, nil)
				if err != nil {
					t.Fatal(err)
				}
				p := tiler.(*Provider)
				t.Cleanup(p.pool.Close)
				proof, err := p.layers["items"].FeatureCRSDefinition()
				if err != nil || proof.CanonicalCode != fmt.Sprint(srid) {
					t.Fatalf("source proof %+v %v", proof, err)
				}
				target := uint64(4326)
				bounds := geom.Extent{9.9, 19.9, 10.1, 20.1}
				if srid == 4326 {
					target = 3857
					bounds = geom.Extent{1100000, 2200000, 1120000, 2300000}
				}
				if srid == 32633 {
					bounds = geom.Extent{14.9, -0.1, 15.1, 0.1}
				}
				definition, _ := crsconfig.CanonicalFeatureDefinition(target)
				q := provider.FeatureQuery{Limit: 1, BoundsSRID: srid, BoundsCRSDefinition: definition, Bounds: []geom.Extent{bounds}}
				var ids []uint64
				result, err := p.QueryFeatures(ctx, "items", q, func(f *provider.Feature) error { ids = append(ids, f.ID); return nil })
				if err != nil || !reflect.DeepEqual(ids, []uint64{1}) || result.NumberReturned != 1 {
					t.Fatalf("queryframe %+v %v %v", result, ids, err)
				}
				q.Offset = 1
				calls := 0
				_, err = p.QueryFeatures(ctx, "items", q, func(*provider.Feature) error { calls++; return nil })
				if err != nil || calls != 0 {
					t.Fatal("offset applied before exact match", calls, err)
				}
				oldSource, _ := basic.EffectiveProj4Definition(srid)
				oldTarget, _ := basic.EffectiveProj4Definition(target)
				t.Cleanup(func() {
					if err := basic.RegisterProj4SRID(srid, oldSource); err != nil {
						t.Error(err)
					}
					if err := basic.RegisterProj4SRID(target, oldTarget); err != nil {
						t.Error(err)
					}
				})
				if err := basic.RegisterProj4SRID(srid, oldTarget); err != nil {
					t.Fatal(err)
				}
				if err := basic.RegisterProj4SRID(target, oldSource); err != nil {
					t.Fatal(err)
				}
				q.Offset = 0
				ids = nil
				_, err = p.QueryFeatures(ctx, "items", q, func(f *provider.Feature) error { ids = append(ids, f.ID); return nil })
				if err != nil || !reflect.DeepEqual(ids, []uint64{1}) {
					t.Fatal("mutable registry changed frozen source/query math", ids, err)
				}
				t.Logf("frozen source EPSG:%d authoritative query profile %d", srid, target)
			})
		}
	}
}

func TestFeatureCRSDimensionalFrozenLive(t *testing.T) {
	for _, format := range []string{"native", "wkb", "wkt"} {
		for _, mixed := range []bool{false, true} {
			if format == "native" && mixed {
				continue
			} // PostGIS fixed native typmod is uniform.
			t.Run(format+fmt.Sprintf("/mixed=%t", mixed), func(t *testing.T) {
				db, schema := featureLiveDatabase(t)
				ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
				defer cancel()
				qualified := pgQuoteIdent(schema) + `."dimensional_items"`
				typeName := map[string]string{"native": "geometry(PointZ,4326)", "wkb": "bytea", "wkt": "text"}[format]
				if _, err := db.Exec(ctx, "CREATE TABLE "+qualified+" (id bigint PRIMARY KEY,geom "+typeName+")"); err != nil {
					t.Fatal(err)
				}
				geometries := []geom.Geometry{geom.PointZ{10, 20, 7}, geom.PointZ{10.01, 20.01, 200}}
				if mixed {
					geometries[1] = geom.Point{10.01, 20.01}
				}
				for i, g := range geometries {
					var value any
					if format == "wkt" {
						if point, ok := g.(geom.PointZ); ok {
							value = fmt.Sprintf("POINT Z(%g %g %g)", point[0], point[1], point[2])
						} else {
							point := g.(geom.Point)
							value = fmt.Sprintf("POINT(%g %g)", point[0], point[1])
						}
					} else {
						wire, err := querytest.EncodeFixtureWKB(g)
						if err != nil {
							t.Fatal(err)
						}
						value = wire
					}
					insert := "INSERT INTO " + qualified + " VALUES ($1,$2)"
					if format == "native" {
						insert = "INSERT INTO " + qualified + " VALUES ($1,ST_GeomFromWKB($2::bytea,4326))"
					}
					if _, err := db.Exec(ctx, insert, i+1, value); err != nil {
						t.Fatal(err)
					}
				}
				layer := map[string]any{"name": "items", "tablename": qualified, "id_fieldname": "id", "geometry_fieldname": "geom", "geometry_type": "point", "srid": 4326, "fields": []string{"id"}, "spatial_dimension": "xyz", "vertical_crs": provider.CRS84h}
				if format != "native" {
					layer["geometry_format"] = format
				}
				if mixed {
					layer["spatial_dimension"] = "mixed_xy_xyz"
				}
				tiler, err := NewTileProvider(dict.Dict{"name": "dimensional_crs", "uri": os.Getenv("PGURI"), "layers": []map[string]any{layer}}, nil)
				if err != nil {
					t.Fatal(err)
				}
				p := tiler.(*Provider)
				t.Cleanup(p.pool.Close)
				definition, _ := crsconfig.CanonicalFeatureDefinition(3857)
				q := provider.FeatureQuery{Limit: 10, BoundsSRID: 4326, BoundsCRSDefinition: definition, BoundsVerticalCRS: provider.CRS84h, Bounds3D: []provider.Extent3D{{1100000, 2200000, 6, 1120000, 2300000, 8}}}
				var ids []uint64
				_, err = p.QueryFeatures(ctx, "items", q, func(f *provider.Feature) error { ids = append(ids, f.ID); return nil })
				want := []uint64{1}
				if mixed {
					want = []uint64{1, 2}
				}
				if err != nil || !reflect.DeepEqual(ids, want) {
					t.Fatal("frozen 3D query/missing-height policy", ids, err)
				}
			})
		}
	}
}
