package hana_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"

	"github.com/alexeydott/geom"
	"github.com/alexeydott/tegola/dict"
	"github.com/alexeydott/tegola/provider"
	"github.com/alexeydott/tegola/provider/hana"
)

func TestFeatureLiveNativeCatalogProfile(t *testing.T) {
	if os.Getenv(TESTENV) != "yes" || GetConnectionURI() == "" {
		t.Skip("live HANA")
	}
	for _, kind := range []string{"ST_POINT", "ST_GEOMETRY"} {
		for _, custom := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s custom=%v", kind, custom), func(t *testing.T) {
				db, e := hana.OpenDB(GetConnectionURI())
				if e != nil {
					t.Fatal(e)
				}
				t.Cleanup(func() { featureFixtureClose(t, db) })
				table := featureFixtureTable(t, "TEGOLA_NATIVE")
				q := `"` + table + `"`
				if _, e = db.Exec(`CREATE COLUMN TABLE ` + q + ` ("id" BIGINT PRIMARY KEY,"geom" ` + kind + `(4326),"name" NVARCHAR(30))`); e != nil {
					t.Fatal(e)
				}
				t.Cleanup(func() {
					if _, e := db.Exec("DROP TABLE " + q); e != nil {
						t.Error(e)
					}
				})
				for _, sql := range []string{`INSERT INTO ` + q + ` VALUES(1,NEW ST_POINT('POINT(1 2)',4326),'point')`, `INSERT INTO ` + q + ` VALUES(2,NULL,NULL)`} {
					if _, e = db.Exec(sql); e != nil {
						t.Fatal(e)
					}
				}
				layer := map[string]any{"name": "items", "tablename": q, "id_fieldname": "id", "geometry_fieldname": "geom", "geometry_type": "point", "srid": 4326, "fields": []string{"name"}}
				if custom {
					layer["feature_sql"] = `SELECT "id","geom","name" FROM ` + q + ` WHERE "id">=1`
				}
				p, e := hana.CreateProvider(dict.Dict{hana.ConfigKeyName: "native", hana.ConfigKeyURI: GetConnectionURI(), "layers": []map[string]any{layer}}, nil, hana.ProviderType)
				if e != nil {
					t.Fatal(e)
				}
				t.Cleanup(p.Close)
				calls := 0
				r, e := p.QueryFeatures(context.Background(), "items", provider.FeatureQuery{Limit: 10}, func(f *provider.Feature) error {
					calls++
					if f.ID == 1 {
						if f.SRID != 4326 || f.Geometry != (geom.Geometry)(geom.Point{1, 2}) || f.Tags["name"] != "point" {
							t.Fatalf("point %+v", f)
						}
					} else {
						if f.Geometry != nil {
							t.Fatal(f)
						}
						v, exists := f.Tags["name"]
						if !exists || v != nil {
							t.Fatalf("selected NULL lost %+v", f)
						}
					}
					return nil
				})
				if kind == "ST_GEOMETRY" {
					if calls != 0 || !errors.Is(e, provider.ErrUnsupported) {
						t.Fatalf("generic dimension4 admitted %+v %v", r, e)
					}
					return
				}
				if e != nil || calls != 2 || r.NumberReturned != 2 {
					t.Fatalf("native %+v calls%d err%v", r, calls, e)
				}
				_, e = db.Exec(`INSERT INTO ` + q + ` VALUES(3,NEW ST_POINT('POINT Z(1 2 3)',4326),'xyz')`)
				if e == nil {
					t.Fatal("ST_POINT acceptedXYZ")
				}
			})
		}
	}
}

func TestFeatureLiveNativeDimensionRejectsEncounteredBody(t *testing.T) {
	if os.Getenv(TESTENV) != "yes" || GetConnectionURI() == "" {
		t.Skip("live HANA")
	}
	for _, c := range []struct{ dimension, body string }{
		{"xy", "POINT M(1 2 3)"}, {"xy", "POINT ZM(1 2 3 4)"},
		{"xy", "POINT Z(1 2 3)"}, {"xyz", "POINT(1 2)"},
		{"mixed_xy_xyz", "POINT M(1 2 3)"}, {"mixed_xy_xyz", "POINT ZM(1 2 3 4)"},
	} {
		t.Run(c.dimension+" "+c.body, func(t *testing.T) {
			db, err := hana.OpenDB(GetConnectionURI())
			if err != nil {
				t.Fatal(err)
			}
			defer featureFixtureClose(t, db)
			table := featureFixtureTable(t, "TEGOLA_NATIVE_REJECT")
			q := `"` + table + `"`
			if _, err := db.Exec(`CREATE COLUMN TABLE ` + q + ` ("id" BIGINT PRIMARY KEY,"geom" ST_GEOMETRY(4326))`); err != nil {
				t.Fatal(err)
			}
			defer func() {
				if _, err := db.Exec("DROP TABLE " + q); err != nil {
					t.Error(err)
				}
			}()
			if _, err := db.Exec(`INSERT INTO `+q+` VALUES(1,ST_GeomFromText(?,4326))`, c.body); err != nil {
				t.Fatal(err)
			}
			layer := map[string]any{"name": "items", "tablename": q, "id_fieldname": "id", "geometry_fieldname": "geom", "geometry_type": "point", "srid": 4326, "spatial_dimension": c.dimension}
			if c.dimension != "xy" {
				layer["vertical_crs"] = provider.CRS84h
			}
			p, err := hana.CreateProvider(dict.Dict{hana.ConfigKeyName: "reject", hana.ConfigKeyURI: GetConnectionURI(), "layers": []map[string]any{layer}}, nil, hana.ProviderType)
			if err != nil {
				t.Fatal(err)
			}
			defer p.Close()
			calls := 0
			_, err = p.QueryFeatures(context.Background(), "items", provider.FeatureQuery{Limit: 1}, func(*provider.Feature) error { calls++; return nil })
			var data provider.FeatureDataError
			if calls != 0 || !errors.As(err, &data) {
				t.Fatalf("dimension integrity calls=%d: %v", calls, err)
			}
		})
	}
}
