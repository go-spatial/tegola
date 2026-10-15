package mysql

import (
	"bytes"
	"context"
	"fmt"
	"testing"

	"github.com/alexeydott/geom"
	"github.com/alexeydott/geom/encoding/wkb"
	"github.com/alexeydott/tegola/mos"
	"github.com/alexeydott/tegola/provider"
	"github.com/alexeydott/tegola/provider/crsconfig"
	codec "github.com/alexeydott/tegola/provider/geometrycodec"
)

func TestNativeMOSBoundsMutation(t *testing.T) {
	w, db := followupMySQL(t, true)
	ctx := context.Background()
	if _, err := db.Exec("ALTER TABLE items ADD MINX BIGINT, ADD MAXX BIGINT, ADD MINY BIGINT, ADD MAXY BIGINT"); err != nil {
		t.Fatal(err)
	}
	layer := w.provider.layers["items"]
	layer.geometryFormat = "mos"
	layer.mosConfig = codec.MOSConfig{Precision: 0, UnitFactor: 0.001}
	layer.bboxFields = codec.DefaultBBoxFields()
	w.provider.layers["items"] = layer
	if _, err := w.DescribeWritable(ctx, "items"); err != nil {
		t.Fatalf("MOS bounds admission: %v", err)
	}
	geometry := func(g geom.Geometry) []byte {
		b, err := wkb.EncodeBytes(g)
		if err != nil {
			t.Fatal(err)
		}
		return b
	}
	tx, err := w.BeginFeatureTx(ctx, provider.TxOptions{})
	if err != nil {
		t.Fatal(err)
	}
	outcome, err := tx.Apply(ctx, provider.Mutation{Op: provider.MutationInsert, Collection: "items", GeometryWKB: geometry(geom.LineString{{1.2344, 2.3456}, {3.4567, 4.5678}}), GeometrySRID: 4326})
	if err != nil {
		_ = tx.Rollback(ctx)
		t.Fatal(err)
	}
	if _, err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	var blob []byte
	var got [4]float64
	read := func() {
		t.Helper()
		if err := db.QueryRow("SELECT geom,MINX,MAXX,MINY,MAXY FROM items WHERE id=?", outcome.FeatureID).Scan(&blob, &got[0], &got[1], &got[2], &got[3]); err != nil {
			t.Fatal(err)
		}
	}
	read()
	if got != [4]float64{1234, 3457, 2346, 4568} {
		t.Fatalf("quantized bounds %v", got)
	}
	if _, err = mos.Decode(blob, mos.Options{UnitFactor: 0.001}); err != nil {
		t.Fatal(err)
	}
	tail := append(append([]byte(nil), blob...), []byte("private style tail")...)
	if _, err = db.Exec("UPDATE items SET geom=?,MINX=MINX-1,MAXX=MAXX+1,MINY=MINY-1,MAXY=MAXY+1 WHERE id=?", tail, outcome.FeatureID); err != nil {
		t.Fatal(err)
	}
	tx, err = w.BeginFeatureTx(ctx, provider.TxOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Apply(ctx, provider.Mutation{Op: provider.MutationUpdate, Collection: "items", FeatureID: outcome.FeatureID, Properties: map[string]provider.MutationValue{"name": {Kind: provider.MutationValueString, String: "attribute"}}}); err != nil {
		_ = tx.Rollback(ctx)
		t.Fatal(err)
	}
	if _, err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	read()
	if !bytes.Equal(blob, tail) || got != [4]float64{1233, 3458, 2345, 4569} {
		t.Fatal("attribute update changed geometry or bounds")
	}
	tx, err = w.BeginFeatureTx(ctx, provider.TxOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Apply(ctx, provider.Mutation{Op: provider.MutationUpdate, Collection: "items", FeatureID: outcome.FeatureID, GeometryWKB: geometry(geom.Point{5.4321, 6.5432}), GeometrySRID: 4326}); err != nil {
		_ = tx.Rollback(ctx)
		t.Fatal(err)
	}
	if err = tx.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	read()
	if !bytes.Equal(blob, tail) || got != [4]float64{1233, 3458, 2345, 4569} {
		t.Fatal("rollback changed geometry or bounds")
	}

	for _, op := range []provider.MutationOp{provider.MutationUpdate, provider.MutationReplace} {
		tx, err = w.BeginFeatureTx(ctx, provider.TxOptions{})
		if err != nil {
			t.Fatal(err)
		}
		if _, err = tx.Apply(ctx, provider.Mutation{Op: op, Collection: "items", FeatureID: outcome.FeatureID, GeometryWKB: geometry(geom.Point{-5.4321, 6.5432}), GeometrySRID: 4326}); err != nil {
			_ = tx.Rollback(ctx)
			t.Fatal(err)
		}
		if _, err = tx.Commit(ctx); err != nil {
			t.Fatal(err)
		}
		read()
		if got != [4]float64{-5432, -5432, 6543, 6543} {
			t.Fatalf("%s bounds %v", op, got)
		}
	}
	tx, err = w.BeginFeatureTx(ctx, provider.TxOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Apply(ctx, provider.Mutation{Op: provider.MutationUpdate, Collection: "items", FeatureID: outcome.FeatureID, GeometryAbsent: true}); err != nil {
		_ = tx.Rollback(ctx)
		t.Fatal(err)
	}
	if _, err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	var allNull bool
	if err = db.QueryRow("SELECT geom IS NULL AND MINX IS NULL AND MAXX IS NULL AND MINY IS NULL AND MAXY IS NULL FROM items WHERE id=?", outcome.FeatureID).Scan(&allNull); err != nil {
		t.Fatal(err)
	}
	if !allNull {
		t.Fatal("geometry clear left derived bounds")
	}
	tx, err = w.BeginFeatureTx(ctx, provider.TxOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Apply(ctx, provider.Mutation{Op: provider.MutationDelete, Collection: "items", FeatureID: outcome.FeatureID}); err != nil {
		_ = tx.Rollback(ctx)
		t.Fatal(err)
	}
	if _, err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	var remaining int
	if err = db.QueryRow("SELECT COUNT(*) FROM items WHERE id=?", outcome.FeatureID).Scan(&remaining); err != nil {
		t.Fatal(err)
	}
	if remaining != 0 {
		t.Fatal("delete retained feature and bounds")
	}
}

func TestNativeMOSBoundsAdmission(t *testing.T) {
	w, db := followupMySQL(t, true)
	for i, tc := range []struct {
		name, columns, format string
		fields                codec.BBoxFields
		allowed               bool
	}{
		{"integer", "MINX INT,MAXX INT,MINY INT,MAXY INT", "mos", codec.DefaultBBoxFields(), true},
		{"casefold", "MINX BIGINT,MAXX BIGINT,MINY BIGINT,MAXY BIGINT", "mos", codec.BBoxFields{"minx", "maxx", "miny", "maxy"}, true},
		{"double", "MINX DOUBLE,MAXX DOUBLE,MINY DOUBLE,MAXY DOUBLE", "mos", codec.DefaultBBoxFields(), true},
		{"nullable_mismatch", "MINX BIGINT NOT NULL,MAXX BIGINT,MINY BIGINT,MAXY BIGINT", "mos", codec.DefaultBBoxFields(), false},
		{"partial", "MINX BIGINT", "mos", codec.DefaultBBoxFields(), false},
		{"text", "MINX TEXT,MAXX TEXT,MINY TEXT,MAXY TEXT", "mos", codec.DefaultBBoxFields(), false},
		{"narrow", "MINX SMALLINT,MAXX SMALLINT,MINY SMALLINT,MAXY SMALLINT", "mos", codec.DefaultBBoxFields(), false},
		{"unsigned", "MINX INT UNSIGNED,MAXX INT,MINY INT,MAXY INT", "mos", codec.DefaultBBoxFields(), false},
		{"decimal", "MINX DECIMAL(12,2),MAXX DECIMAL(12,2),MINY DECIMAL(12,2),MAXY DECIMAL(12,2)", "mos", codec.DefaultBBoxFields(), true},
		{"decimal_narrow", "MINX DECIMAL(10,2),MAXX DECIMAL(10,2),MINY DECIMAL(10,2),MAXY DECIMAL(10,2)", "mos", codec.DefaultBBoxFields(), false},
		{"duplicate", "MINX INT,MAXX INT,MINY INT,MAXY INT", "mos", codec.BBoxFields{"MINX", "MINX", "MINY", "MAXY"}, false},
		{"nonmos", "MINX INT,MAXX INT,MINY INT,MAXY INT", "wkb", codec.DefaultBBoxFields(), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			table := fmt.Sprintf("bounds_%d", i)
			if _, err := db.Exec("CREATE TABLE " + table + "(id BIGINT PRIMARY KEY AUTO_INCREMENT,geom BLOB," + tc.columns + ") ENGINE=InnoDB"); err != nil {
				t.Fatal(err)
			}
			layer := w.provider.layers["items"]
			layer.tablename = table
			layer.geometryFormat = tc.format
			layer.bboxFields = tc.fields
			layer.mosConfig = codec.MOSConfig{UnitFactor: 0.001}
			mapping, err := admitLayer(context.Background(), w.provider, &layer)
			if (err == nil) != tc.allowed {
				t.Fatalf("admission allowed=%v err=%v", tc.allowed, err)
			}
			if err == nil {
				for _, field := range mapping.bboxFields {
					if _, ok := mapping.writable[field]; ok {
						t.Fatalf("derived field %s is client writable", field)
					}
				}
			}
		})
	}
}

func TestNativeMOSCustomCRSBounds(t *testing.T) {
	w, db := followupMySQL(t, true)
	const definition = "+proj=etmerc +ellps=bessel +towgs84=1,2,3,0,0,0,0 +lon_0=9 +lat_0=50 +k_0=1 +x_0=0 +y_0=0 +units=m +no_defs"
	projection, err := crsconfig.NewFeatureProjection(definition)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec("ALTER TABLE items ADD MINX INT,ADD MAXX INT,ADD MINY INT,ADD MAXY INT"); err != nil {
		t.Fatal(err)
	}
	layer := w.provider.layers["items"]
	layer.geometryFormat = "mos"
	layer.srid = 999999
	layer.feature = &featureProfile{crsProjection: projection}
	layer.mosConfig = codec.MOSConfig{Precision: 0, UnitFactor: 0.001}
	layer.bboxFields = codec.DefaultBBoxFields()
	w.provider.layers["items"] = layer
	ctx := context.Background()
	if _, err = w.DescribeWritable(ctx, "items"); err != nil {
		t.Fatal(err)
	}
	b, err := wkb.EncodeBytes(geom.Point{9.01, 50.01})
	if err != nil {
		t.Fatal(err)
	}
	tx, err := w.BeginFeatureTx(ctx, provider.TxOptions{})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	result, err := tx.Apply(ctx, provider.Mutation{Op: provider.MutationInsert, Collection: "items", GeometryWKB: b, GeometrySRID: 4326})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	var bounds [4]int32
	if err = db.QueryRow("SELECT MINX,MAXX,MINY,MAXY FROM items WHERE id=?", result.FeatureID).Scan(&bounds[0], &bounds[1], &bounds[2], &bounds[3]); err != nil {
		t.Fatal(err)
	}
	// Independent PROJ/pyproj oracle, rounded to MOS millimetres.
	if bounds != [4]int32{714912, 714912, 1045772, 1045772} {
		t.Fatalf("custom CRS raw bounds %v", bounds)
	}
}

func TestNativeMOSSystemInfoReadOnly(t *testing.T) {
	w, db := followupMySQL(t, true)
	layer := w.provider.layers["items"]
	layer.geometryFormat = "mos"
	layer.mosConfig = codec.MOSConfig{UnitFactor: 0.001}
	w.provider.layers["items"] = layer
	ctx := context.Background()
	if _, err := w.DescribeWritable(ctx, "items"); err != nil {
		t.Fatal(err)
	}
	metadata := append([]byte{5}, []byte("Ver 1")...)
	metadata = append(metadata, make([]byte, 58)...)
	if _, err := db.Exec("INSERT INTO items(id,geom,name) VALUES(1,?,'metadata')", metadata); err != nil {
		t.Fatal(err)
	}
	for _, op := range []provider.MutationOp{provider.MutationUpdate, provider.MutationReplace, provider.MutationDelete} {
		tx, err := w.BeginFeatureTx(ctx, provider.TxOptions{})
		if err != nil {
			t.Fatal(err)
		}
		_, err = tx.Apply(ctx, provider.Mutation{Op: op, Collection: "items", FeatureID: 1, Properties: map[string]provider.MutationValue{"name": {Kind: provider.MutationValueString, String: "changed"}}})
		if me, ok := provider.AsMutationError(err); !ok || me.Kind != provider.MutationErrNotFound {
			t.Errorf("%s modified system metadata: %v", op, err)
		}
		if err := tx.Rollback(ctx); err != nil {
			t.Fatal(err)
		}
	}
	var retained []byte
	if err := db.QueryRow("SELECT geom FROM items WHERE id=1").Scan(&retained); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(metadata, retained) {
		t.Fatal("system metadata changed")
	}
}

func TestNativeMOSCanonicalMetadata(t *testing.T) {
	w, db := followupMySQL(t, true)
	if _, err := db.Exec("ALTER TABLE items CHANGE id OKEY BIGINT NOT NULL AUTO_INCREMENT,CHANGE geom LINE BLOB,ADD MUID BIGINT,ADD ObjectType INT,ADD ObjectStyle TEXT,ADD MINX INT,ADD MAXX INT,ADD MINY INT,ADD MAXY INT"); err != nil {
		t.Fatal(err)
	}
	layer := w.provider.layers["items"]
	layer.idFieldname = "OKEY"
	layer.geomFieldname = "LINE"
	layer.geometryFormat = "mos"
	layer.isMapplGIS = true
	layer.mosConfig = codec.MOSConfig{UnitFactor: 0.001}
	layer.bboxFields = codec.DefaultBBoxFields()
	w.provider.layers["items"] = layer
	ctx := context.Background()
	descriptor, err := w.DescribeWritable(ctx, "items")
	if err != nil {
		t.Fatal(err)
	}
	if descriptor.CreateUnsupportedReason == "" {
		t.Fatal("canonical create advertised without metadata initialization")
	}
	raw, err := mos.Encode(geom.Point{1, 2}, mos.Options{UnitFactor: 0.001})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec("INSERT INTO items(OKEY,LINE,MUID,ObjectType,ObjectStyle,MINX,MAXX,MINY,MAXY) VALUES(2,?,900,7,'style',1000,1000,2000,2000)", raw); err != nil {
		t.Fatal(err)
	}
	point, err := wkb.EncodeBytes(geom.Point{3, 4})
	if err != nil {
		t.Fatal(err)
	}
	line, err := wkb.EncodeBytes(geom.LineString{{3, 4}, {5, 6}})
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name string
		m    provider.Mutation
		fail bool
	}{
		{"attribute", provider.Mutation{Op: provider.MutationUpdate, Properties: map[string]provider.MutationValue{"name": {Kind: provider.MutationValueString, String: "changed"}}}, false},
		{"metadata", provider.Mutation{Op: provider.MutationUpdate, Properties: map[string]provider.MutationValue{"MUID": {Kind: provider.MutationValueInteger, Integer: 901}}}, true},
		{"family", provider.Mutation{Op: provider.MutationUpdate, GeometryWKB: line}, true},
		{"replace", provider.Mutation{Op: provider.MutationReplace, GeometryWKB: point}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tx, err := w.BeginFeatureTx(ctx, provider.TxOptions{})
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = tx.Rollback(ctx) }()
			tc.m.Collection = "items"
			tc.m.FeatureID = 2
			_, err = tx.Apply(ctx, tc.m)
			if (err != nil) != tc.fail {
				t.Fatalf("want failure=%v err=%v", tc.fail, err)
			}
			if err == nil {
				if _, err = tx.Commit(ctx); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
	var muid, objectType int
	var style string
	if err = db.QueryRow("SELECT MUID,ObjectType,ObjectStyle FROM items WHERE OKEY=2").Scan(&muid, &objectType, &style); err != nil {
		t.Fatal(err)
	}
	if muid != 900 || objectType != 7 || style != "style" {
		t.Fatal("canonical metadata changed")
	}
	tail := append(raw, []byte("opaque tail")...)
	if _, err = db.Exec("UPDATE items SET LINE=? WHERE OKEY=2", tail); err != nil {
		t.Fatal(err)
	}
	tx, err := w.BeginFeatureTx(ctx, provider.TxOptions{})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	_, err = tx.Apply(ctx, provider.Mutation{Op: provider.MutationUpdate, Collection: "items", FeatureID: 2, GeometryWKB: point})
	if me, ok := provider.AsMutationError(err); !ok || me.Kind != provider.MutationErrUnsupportedCapability {
		t.Fatalf("opaque canonical MOS rewrite accepted: %v", err)
	}
}
