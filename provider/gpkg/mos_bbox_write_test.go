//go:build cgo

package gpkg_test

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"github.com/alexeydott/geom"
	"github.com/alexeydott/tegola/dict"
	"github.com/alexeydott/tegola/mos"
	"github.com/alexeydott/tegola/provider"
	pa "github.com/alexeydott/tegola/provider/audit"
	"github.com/alexeydott/tegola/provider/gpkg"
	"path/filepath"
	"testing"
)

func mosBoundsFixture(t *testing.T, ddl string, overrides map[string]any) (*gpkg.Provider, *sql.DB) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "mos-bounds.sqlite")
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Error(err)
		}
	})
	if _, err := db.Exec(ddl); err != nil {
		t.Fatal(err)
	}
	layer := map[string]any{"name": "items", "tablename": "items", "id_fieldname": "id", "geometry_fieldname": "geom", "geometry_type": "point", "geometry_format": "mos", "srid": 3857, "mos_precision": 2, "mos_units": "m", "fields": []string{"name"}}
	for k, v := range overrides {
		layer[k] = v
	}
	raw, err := gpkg.NewTileProvider(dict.Dict{"filepath": path, "layers": []map[string]any{layer}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	p := raw.(*gpkg.Provider)
	t.Cleanup(func() {
		if err := p.Close(); err != nil {
			t.Error(err)
		}
	})
	return p, db
}

func TestMOSNativeBBoxAdmission(t *testing.T) {
	cases := []struct {
		name, columns string
		accepted      bool
	}{
		{"integer", "MINX INTEGER,MAXX INTEGER,MINY INTEGER,MAXY INTEGER", true},
		{"real", "MINX REAL,MAXX REAL,MINY REAL,MAXY REAL", true},
		{"partial", "MINX INTEGER,MAXX INTEGER", false},
		{"text", "MINX TEXT,MAXX INTEGER,MINY INTEGER,MAXY INTEGER", false},
		{"nonnull bbox nullable geom", "MINX INTEGER NOT NULL,MAXX INTEGER,MINY INTEGER,MAXY INTEGER", false},
		{"generated", "MINX INTEGER GENERATED ALWAYS AS(0),MAXX INTEGER,MINY INTEGER,MAXY INTEGER", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p, _ := mosBoundsFixture(t, "CREATE TABLE items(id INTEGER PRIMARY KEY,geom BLOB,name TEXT,"+tc.columns+")", nil)
			descriptor, err := p.DescribeWritable(context.Background(), "items")
			if (err == nil) != tc.accepted {
				t.Fatalf("accepted=%v descriptor=%+v err=%v", tc.accepted, descriptor, err)
			}
			if tc.accepted {
				for _, name := range []string{"MINX", "MAXX", "MINY", "MAXY"} {
					if _, ok := descriptor.WritableColumns[name]; ok {
						t.Fatalf("derived bbox exposed %s", name)
					}
				}
			}
		})
	}
}

func TestMOSNativeBBoxLifecycle(t *testing.T) {
	p, db := mosBoundsFixture(t, "CREATE TABLE items(id INTEGER PRIMARY KEY,geom BLOB,name TEXT,MINX INTEGER,MAXX INTEGER,MINY INTEGER,MAXY INTEGER)", nil)
	ctx := context.Background()
	if err := pa.Migrate(ctx, db, "sqlite"); err != nil {
		t.Fatal(err)
	}
	apply := func(m provider.Mutation, commit bool) provider.MutationOutcome {
		t.Helper()
		m.Collection = "items"
		tx, err := p.BeginFeatureTx(ctx, provider.TxOptions{})
		if err != nil {
			t.Fatal(err)
		}
		out, err := tx.Apply(ctx, m)
		if err != nil {
			_ = tx.Rollback(ctx)
			t.Fatal(err)
		}
		if commit {
			if _, err := tx.Commit(ctx); err != nil {
				t.Fatal(err)
			}
		} else {
			if err := tx.Rollback(ctx); err != nil {
				t.Fatal(err)
			}
		}
		return out
	}
	out := apply(provider.Mutation{Op: provider.MutationInsert, GeometryWKB: wkbPoint(t, 1.234, -2.346), GeometrySRID: 3857, Properties: map[string]provider.MutationValue{"name": strVal("first")}}, true)
	read := func(want [4]int64) []byte {
		t.Helper()
		var raw []byte
		var got [4]int64
		if err := db.QueryRow("SELECT geom,MINX,MAXX,MINY,MAXY FROM items WHERE id=?", out.FeatureID).Scan(&raw, &got[0], &got[1], &got[2], &got[3]); err != nil {
			t.Fatal(err)
		}
		if got != want {
			t.Fatalf("bbox=%v want=%v", got, want)
		}
		return raw
	}
	initial := read([4]int64{123, 123, -235, -235})
	// Unknown MOS styling tails and existing conservative bounds survive attribute-only changes.
	initial = append(initial, []byte("native-style-tail")...)
	if _, err := db.Exec("UPDATE items SET geom=?,MINX=122,MAXX=124 WHERE id=?", initial, out.FeatureID); err != nil {
		t.Fatal(err)
	}
	apply(provider.Mutation{Op: provider.MutationUpdate, FeatureID: out.FeatureID, Properties: map[string]provider.MutationValue{"name": strVal("attribute")}}, true)
	if !bytes.Equal(initial, read([4]int64{122, 124, -235, -235})) {
		t.Fatal("attribute-only update changed MOS bytes")
	}
	apply(provider.Mutation{Op: provider.MutationUpdate, FeatureID: out.FeatureID, GeometryWKB: wkbPoint(t, 3.456, 4.567), GeometrySRID: 3857}, false)
	if !bytes.Equal(initial, read([4]int64{122, 124, -235, -235})) {
		t.Fatal("rollback changed MOS bytes")
	}
	apply(provider.Mutation{Op: provider.MutationUpdate, FeatureID: out.FeatureID, GeometryWKB: wkbPoint(t, 3.456, 4.567), GeometrySRID: 3857}, true)
	read([4]int64{346, 346, 457, 457})
	apply(provider.Mutation{Op: provider.MutationReplace, FeatureID: out.FeatureID, GeometryWKB: wkbPoint(t, -5.678, 6.789), GeometrySRID: 3857}, true)
	read([4]int64{-568, -568, 679, 679})
	apply(provider.Mutation{Op: provider.MutationUpdate, FeatureID: out.FeatureID, GeometryAbsent: true}, true)
	var n int
	if err := db.QueryRow("SELECT count(*) FROM items WHERE id=? AND geom IS NULL AND MINX IS NULL AND MAXX IS NULL AND MINY IS NULL AND MAXY IS NULL", out.FeatureID).Scan(&n); err != nil || n != 1 {
		t.Fatalf("geometry clear bbox count=%d err=%v", n, err)
	}
	apply(provider.Mutation{Op: provider.MutationDelete, FeatureID: out.FeatureID}, true)
	if err := db.QueryRow("SELECT count(*) FROM items").Scan(&n); err != nil || n != 0 {
		t.Fatalf("delete count=%d err=%v", n, err)
	}
}

func TestMOSNativeCustomCRSWriteRead(t *testing.T) {
	const definition = "+proj=etmerc +lat_0=50 +lon_0=30 +k=1 +x_0=1000 +y_0=2000 +ellps=bessel +towgs84=10,20,30,0.1,-0.2,0.3,1 +units=m"
	p, db := mosBoundsFixture(t, "CREATE TABLE items(id INTEGER PRIMARY KEY,geom BLOB,name TEXT,MINX INTEGER,MAXX INTEGER,MINY INTEGER,MAXY INTEGER)", map[string]any{"crs_defn": definition, "mos_precision": 0, "mos_units": "mm"})
	ctx := context.Background()
	if err := pa.Migrate(ctx, db, "sqlite"); err != nil {
		t.Fatal(err)
	}
	tx, err := p.BeginFeatureTx(ctx, provider.TxOptions{})
	if err != nil {
		t.Fatal(err)
	}
	out, err := tx.Apply(ctx, provider.Mutation{Op: provider.MutationInsert, Collection: "items", GeometryWKB: wkbPoint(t, 30.2, 50.1), GeometrySRID: 4326})
	if err != nil {
		_ = tx.Rollback(ctx)
		t.Fatal(err)
	}
	if _, err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	var got [4]int64
	if err := db.QueryRow("SELECT MINX,MAXX,MINY,MAXY FROM items WHERE id=?", out.FeatureID).Scan(&got[0], &got[1], &got[2], &got[3]); err != nil {
		t.Fatal(err)
	}
	// Independent PROJ 9.5.1 synthetic CRS oracle, quantized to millimetres.
	if want := ([4]int64{15289293, 15289293, 13063614, 13063614}); got != want {
		t.Fatalf("custom native bbox=%v want=%v", got, want)
	}
	var ids []uint64
	_, err = p.QueryFeatures(ctx, "items", provider.FeatureQuery{Limit: 10, BoundsSRID: 4326, Bounds: []geom.Extent{{30.19999, 50.09999, 30.20001, 50.10001}}}, func(f *provider.Feature) error { ids = append(ids, f.ID); return nil })
	if err != nil || len(ids) != 1 || ids[0] != out.FeatureID {
		t.Fatalf("custom projected bounds read IDs=%v err=%v", ids, err)
	}
}

func TestMOSAuxiliaryColumnPreservesGeoPackageMetadata(t *testing.T) {
	for _, registered := range []bool{false, true} {
		t.Run(fmt.Sprintf("registeredMOS=%v", registered), func(t *testing.T) {
			p, db := mosBoundsFixture(t, "CREATE TABLE items(id INTEGER PRIMARY KEY,geom BLOB,native_geom BLOB,name TEXT,MINX INTEGER,MAXX INTEGER,MINY INTEGER,MAXY INTEGER)", nil)
			for _, ddl := range []string{
				"CREATE TABLE gpkg_geometry_columns(table_name TEXT,column_name TEXT,geometry_type_name TEXT,srs_id INTEGER,z INTEGER,m INTEGER)",
				"CREATE TABLE gpkg_contents(table_name TEXT,data_type TEXT,last_change TEXT,min_x DOUBLE,max_x DOUBLE,min_y DOUBLE,max_y DOUBLE,srs_id INTEGER)",
				"INSERT INTO gpkg_contents VALUES('items','features','old',10,20,30,40,3857)",
			} {
				if _, err := db.Exec(ddl); err != nil {
					t.Fatal(err)
				}
			}
			column := "native_geom"
			if registered {
				column = "geom"
			}
			if _, err := db.Exec("INSERT INTO gpkg_geometry_columns VALUES('items',?,'POINT',3857,0,0)", column); err != nil {
				t.Fatal(err)
			}
			ctx := context.Background()
			_, err := p.DescribeWritable(ctx, "items")
			if registered {
				if err == nil {
					t.Fatal("registered GeoPackage binary column admitted as raw MOS")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if err := pa.Migrate(ctx, db, "sqlite"); err != nil {
				t.Fatal(err)
			}
			tx, err := p.BeginFeatureTx(ctx, provider.TxOptions{})
			if err != nil {
				t.Fatal(err)
			}
			_, err = tx.Apply(ctx, provider.Mutation{Op: provider.MutationInsert, Collection: "items", GeometryWKB: wkbPoint(t, 1, 2), GeometrySRID: 3857})
			if err != nil {
				_ = tx.Rollback(ctx)
				t.Fatal(err)
			}
			if _, err := tx.Commit(ctx); err != nil {
				t.Fatal(err)
			}
			var bounds [4]int64
			var changed string
			if err := db.QueryRow("SELECT min_x,max_x,min_y,max_y,last_change FROM gpkg_contents").Scan(&bounds[0], &bounds[1], &bounds[2], &bounds[3], &changed); err != nil {
				t.Fatal(err)
			}
			if bounds != ([4]int64{10, 20, 30, 40}) || changed == "old" {
				t.Fatalf("native metadata bounds=%v last_change=%s", bounds, changed)
			}
		})
	}
}

func TestMOSSystemInfoRowsCannotBeMutated(t *testing.T) {
	p, db := mosBoundsFixture(t, "CREATE TABLE items(id INTEGER PRIMARY KEY,geom BLOB,name TEXT,MINX INTEGER,MAXX INTEGER,MINY INTEGER,MAXY INTEGER)", nil)
	ctx := context.Background()
	if err := pa.Migrate(ctx, db, "sqlite"); err != nil {
		t.Fatal(err)
	}
	raw := make([]byte, 64)
	raw[0] = 5
	copy(raw[1:], "Ver 1")
	raw[11] = 2
	raw[26] = 1
	raw[52] = byte(mos.UnitsMetres)
	raw[53] = 1
	if _, err := db.Exec("INSERT INTO items(id,geom,name) VALUES(1,?,'metadata')", raw); err != nil {
		t.Fatal(err)
	}
	for _, op := range []provider.MutationOp{provider.MutationUpdate, provider.MutationReplace, provider.MutationDelete} {
		tx, err := p.BeginFeatureTx(ctx, provider.TxOptions{})
		if err != nil {
			t.Fatal(err)
		}
		_, applyErr := tx.Apply(ctx, provider.Mutation{Op: op, Collection: "items", FeatureID: 1, Properties: map[string]provider.MutationValue{"name": strVal("changed")}, GeometryWKB: wkbPoint(t, 1, 2), GeometrySRID: 3857})
		if err := tx.Rollback(ctx); err != nil {
			t.Fatal(err)
		}
		if applyErr == nil {
			t.Errorf("system metadata row admitted for %v", op)
		}
	}
	var stored []byte
	var name string
	if err := db.QueryRow("SELECT geom,name FROM items WHERE id=1").Scan(&stored, &name); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(stored, raw) || name != "metadata" {
		t.Fatal("system metadata changed")
	}
}

func TestCanonicalMapplGISMutationProfile(t *testing.T) {
	system := mosSystemInfoBlob(2, "", byte(mos.UnitsMetres), true)
	geometry, err := mos.Encode(geom.Point{1, 2}, mos.Options{Precision: 2, UnitFactor: 1})
	if err != nil {
		t.Fatal(err)
	}
	ddl := "CREATE TABLE items(OKEY INTEGER PRIMARY KEY,LINE BLOB,name TEXT,MUID TEXT,ObjectStyle TEXT,ObjectType INTEGER,MINX INTEGER,MAXX INTEGER,MINY INTEGER,MAXY INTEGER);"
	for _, column := range []string{"MUID", "ObjectType", "MINX", "MAXX", "MINY", "MAXY"} {
		ddl += "CREATE INDEX idx_" + column + " ON items(" + column + ");"
	}
	ddl += fmt.Sprintf("INSERT INTO items(OKEY,LINE) VALUES(1,X'%x');INSERT INTO items VALUES(2,X'%x','feature','identity','style',2,100,100,200,200);", system, geometry)
	p, db := mosBoundsFixture(t, ddl, map[string]any{"id_fieldname": "OKEY", "geometry_fieldname": "LINE", "fields": []string{"name", "MUID", "ObjectStyle", "ObjectType"}})
	ctx := context.Background()
	wd, err := p.DescribeWritable(ctx, "items")
	if err != nil {
		t.Fatal(err)
	}
	if wd.CreateUnsupportedReason == "" {
		t.Fatal("canonical create missing explicit metadata contract limitation")
	}
	for _, column := range []string{"MUID", "ObjectStyle", "ObjectType"} {
		if _, ok := wd.WritableColumns[column]; ok {
			t.Fatalf("bookkeeping column writable: %s", column)
		}
	}
	if err := pa.Migrate(ctx, db, "sqlite"); err != nil {
		t.Fatal(err)
	}
	apply := func(m provider.Mutation, wantError bool) {
		t.Helper()
		m.Collection = "items"
		tx, err := p.BeginFeatureTx(ctx, provider.TxOptions{})
		if err != nil {
			t.Fatal(err)
		}
		_, err = tx.Apply(ctx, m)
		if (err != nil) != wantError {
			_ = tx.Rollback(ctx)
			t.Fatalf("mutation %v err=%v wantError=%v", m.Op, err, wantError)
		}
		if wantError {
			if err := tx.Rollback(ctx); err != nil {
				t.Fatal(err)
			}
		} else {
			if _, err := tx.Commit(ctx); err != nil {
				t.Fatal(err)
			}
		}
	}
	apply(provider.Mutation{Op: provider.MutationInsert, GeometryWKB: wkbPoint(t, 1, 2), GeometrySRID: 3857}, true)
	for _, op := range []provider.MutationOp{provider.MutationUpdate, provider.MutationReplace, provider.MutationDelete} {
		apply(provider.Mutation{Op: op, FeatureID: 1, Properties: map[string]provider.MutationValue{"name": strVal("changed")}}, true)
	}
	for _, op := range []provider.MutationOp{provider.MutationUpdate, provider.MutationReplace} {
		apply(provider.Mutation{Op: op, FeatureID: 2, GeometryWKB: wkbPoint(t, 3, 4), GeometrySRID: 3857, Properties: map[string]provider.MutationValue{"name": strVal("changed")}}, false)
		var muid, style string
		var objectType int
		if err := db.QueryRow("SELECT MUID,ObjectStyle,ObjectType FROM items WHERE OKEY=2").Scan(&muid, &style, &objectType); err != nil {
			t.Fatal(err)
		}
		if muid != "identity" || style != "style" || objectType != 2 {
			t.Fatal("canonical bookkeeping changed")
		}
	}
	for _, kind := range []string{"tail", "flags", "modification", "text", "image"} {
		raw := append([]byte(nil), geometry...)
		switch kind {
		case "tail":
			raw = append(raw, []byte("opaque-style")...)
		case "flags":
			raw[2] = 1
		case "modification":
			raw[1] = 7
		case "text":
			raw[0] = mos.TypeText
		case "image":
			raw[0] = mos.TypeImage
		}
		if _, err := db.Exec("UPDATE items SET LINE=? WHERE OKEY=2", raw); err != nil {
			t.Fatal(err)
		}
		t.Run(kind, func(t *testing.T) {
			tx, err := p.BeginFeatureTx(ctx, provider.TxOptions{})
			if err != nil {
				t.Fatal(err)
			}
			_, applyErr := tx.Apply(ctx, provider.Mutation{Op: provider.MutationUpdate, Collection: "items", FeatureID: 2, GeometryWKB: wkbPoint(t, 3, 4), GeometrySRID: 3857})
			if err := tx.Rollback(ctx); err != nil {
				t.Fatal(err)
			}
			if applyErr == nil {
				t.Error("canonical metadata would be discarded on geometry rewrite")
			}
		})
		apply(provider.Mutation{Op: provider.MutationUpdate, FeatureID: 2, Properties: map[string]provider.MutationValue{"name": strVal("attribute-only")}}, false)
		var preserved []byte
		if err := db.QueryRow("SELECT LINE FROM items WHERE OKEY=2").Scan(&preserved); err != nil || !bytes.Equal(preserved, raw) {
			t.Fatalf("metadata bytes changed: %v", err)
		}
	}
	apply(provider.Mutation{Op: provider.MutationDelete, FeatureID: 2}, false)
	var stored []byte
	if err := db.QueryRow("SELECT LINE FROM items WHERE OKEY=1").Scan(&stored); err != nil || !bytes.Equal(stored, system) {
		t.Fatalf("SystemInfo changed: %v", err)
	}
}
