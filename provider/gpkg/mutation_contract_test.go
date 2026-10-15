//go:build cgo

// mutation_contract_test.go proves the GeoPackage native writer against a
// real database file: admission, Insert/Replace/Update/Delete, rollback,
// multi-action transactions and negative controls (W14, ADR-0014).
package gpkg_test

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/alexeydott/geom"
	"github.com/alexeydott/geom/encoding/wkb"
	"github.com/alexeydott/tegola/dict"
	"github.com/alexeydott/tegola/provider"
	"github.com/alexeydott/tegola/provider/gpkg"
)

const mutationDDL = `CREATE TABLE parcels (
	fid INTEGER PRIMARY KEY,
	geom BLOB,
	name TEXT,
	lots INTEGER NOT NULL,
	price REAL
)`

func newMutationFixture(t *testing.T) (string, provider.MutationProvider) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "mut.gpkg")
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if _, err := db.Exec(mutationDDL); err != nil {
		t.Fatalf("ddl: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	conf := dict.Dict{
		"filepath": path,
		"layers": []map[string]interface{}{
			{
				"name":               "parcels",
				"tablename":          "parcels",
				"id_fieldname":       "fid",
				"geometry_fieldname": "geom",
				"geometry_format":    "wkb",
				"srid":               4326,
				"fields":             []string{"name", "lots", "price"},
			},
		},
	}
	p, err := gpkg.NewTileProvider(conf, nil)
	if err != nil {
		t.Fatalf("NewTileProvider: %v", err)
	}
	t.Cleanup(gpkg.Cleanup)
	gp, ok := p.(*gpkg.Provider)
	if !ok {
		t.Fatalf("not a *gpkg.Provider")
	}
	return path, gp.MutationWriter()
}

func wkbPoint(t *testing.T, x, y float64) []byte {
	t.Helper()
	b, err := wkb.EncodeBytes(geom.Point{x, y})
	if err != nil {
		t.Fatalf("wkb: %v", err)
	}
	return b
}

func strVal(s string) provider.MutationValue {
	return provider.MutationValue{Kind: provider.MutationValueString, String: s}
}
func intVal(n int64) provider.MutationValue {
	return provider.MutationValue{Kind: provider.MutationValueInteger, Integer: n}
}

func countRows(t *testing.T, path string) int {
	t.Helper()
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer func() { _ = db.Close() }()
	var n int
	if err := db.QueryRow("SELECT COUNT(*) FROM parcels").Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	return n
}

func TestMutationAdmission(t *testing.T) {
	_, w := newMutationFixture(t)
	wd, err := w.DescribeWritable(context.Background(), "parcels")
	if err != nil {
		t.Fatalf("DescribeWritable: %v", err)
	}
	if wd.Table != "parcels" || wd.IDColumn != "fid" || wd.GeometryColumn != "geom" {
		t.Fatalf("bad descriptor: %+v", wd)
	}
	if wd.WritableColumns["name"] != "name" || wd.WritableColumns["lots"] != "lots" {
		t.Fatalf("writable columns: %v", wd.WritableColumns)
	}
	if _, err := w.DescribeWritable(context.Background(), "missing"); err == nil {
		t.Fatal("expected error for unknown layer")
	}
}

func TestMutationInsertReadback(t *testing.T) {
	path, w := newMutationFixture(t)
	ctx := context.Background()
	tx, err := w.BeginFeatureTx(ctx, provider.TxOptions{})
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	out, err := tx.Apply(ctx, provider.Mutation{
		Op:         provider.MutationInsert,
		Collection: "parcels",
		Properties: map[string]provider.MutationValue{
			"name": strVal("alpha"),
			"lots": intVal(3),
		},
		GeometryWKB: wkbPoint(t, 10, 20),
	})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if out.FeatureID == 0 || out.Affected != 1 {
		t.Fatalf("bad outcome: %+v", out)
	}
	if _, err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit: %v", err)
	}
	if n := countRows(t, path); n != 1 {
		t.Fatalf("rows = %d, want 1", n)
	}
	// Geometry round-trips as a GPKG binary readable by the read path.
	db, _ := sql.Open("sqlite3", path)
	defer func() { _ = db.Close() }()
	var raw []byte
	var name string
	var lots int64
	if err := db.QueryRow("SELECT geom, name, lots FROM parcels WHERE fid = ?", out.FeatureID).Scan(&raw, &name, &lots); err != nil {
		t.Fatalf("readback: %v", err)
	}
	if name != "alpha" || lots != 3 {
		t.Fatalf("readback values: %q %d", name, lots)
	}
	if len(raw) < 5 || (raw[0] != 0x01 && raw[0] != 0x00) {
		t.Fatal("geometry is not WKB")
	}
	// WKB round-trips through the decoder.
	if _, err := wkb.DecodeBytes(raw); err != nil {
		t.Fatalf("WKB decode: %v", err)
	}
}

func TestMutationUpdateAttributeOnlyPreservesGeometry(t *testing.T) {
	path, w := newMutationFixture(t)
	ctx := context.Background()
	tx, _ := w.BeginFeatureTx(ctx, provider.TxOptions{})
	out, err := tx.Apply(ctx, provider.Mutation{
		Op:          provider.MutationInsert,
		Collection:  "parcels",
		Properties:  map[string]provider.MutationValue{"name": strVal("a"), "lots": intVal(1)},
		GeometryWKB: wkbPoint(t, 1, 2),
	})
	if err != nil {
		t.Fatalf("insert: %v", err)
	}
	if _, err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit: %v", err)
	}
	db, _ := sql.Open("sqlite3", path)
	var before []byte
	if err := db.QueryRow("SELECT geom FROM parcels WHERE fid = ?", out.FeatureID).Scan(&before); err != nil {
		t.Fatalf("readback: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	tx2, _ := w.BeginFeatureTx(ctx, provider.TxOptions{})
	if _, err := tx2.Apply(ctx, provider.Mutation{
		Op:         provider.MutationUpdate,
		Collection: "parcels",
		FeatureID:  out.FeatureID,
		Properties: map[string]provider.MutationValue{"name": strVal("b")},
	}); err != nil {
		t.Fatalf("update: %v", err)
	}
	if _, err := tx2.Commit(ctx); err != nil {
		t.Fatalf("commit: %v", err)
	}
	db2, _ := sql.Open("sqlite3", path)
	defer func() { _ = db2.Close() }()
	var after []byte
	var name string
	if err := db2.QueryRow("SELECT geom, name FROM parcels WHERE fid = ?", out.FeatureID).Scan(&after, &name); err != nil {
		t.Fatalf("readback: %v", err)
	}
	if name != "b" {
		t.Fatalf("name = %q", name)
	}
	if string(after) != string(before) {
		t.Fatal("attribute-only update changed geometry bytes")
	}
	_ = path
}

func TestMutationReplaceAndDelete(t *testing.T) {
	_, w := newMutationFixture(t)
	ctx := context.Background()
	tx, _ := w.BeginFeatureTx(ctx, provider.TxOptions{})
	out, err := tx.Apply(ctx, provider.Mutation{
		Op:          provider.MutationInsert,
		Collection:  "parcels",
		Properties:  map[string]provider.MutationValue{"name": strVal("x"), "lots": intVal(9)},
		GeometryWKB: wkbPoint(t, 5, 5),
	})
	if err != nil {
		t.Fatalf("insert: %v", err)
	}
	// Replace sets every writable property; absent ones become NULL.
	if _, err := tx.Apply(ctx, provider.Mutation{
		Op:          provider.MutationReplace,
		Collection:  "parcels",
		FeatureID:   out.FeatureID,
		Properties:  map[string]provider.MutationValue{"lots": intVal(10)},
		GeometryWKB: wkbPoint(t, 6, 6),
	}); err != nil {
		t.Fatalf("replace: %v", err)
	}
	if _, err := tx.Apply(ctx, provider.Mutation{
		Op:         provider.MutationDelete,
		Collection: "parcels",
		FeatureID:  out.FeatureID,
	}); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit: %v", err)
	}
	// All three actions in one native transaction; delete wins.
	tx2, _ := w.BeginFeatureTx(ctx, provider.TxOptions{})
	_, err = tx2.Apply(ctx, provider.Mutation{
		Op:         provider.MutationUpdate,
		Collection: "parcels",
		FeatureID:  out.FeatureID,
		Properties: map[string]provider.MutationValue{"name": strVal("ghost")},
	})
	if err == nil {
		t.Fatal("expected not-found for deleted feature")
	}
	_ = tx2.Rollback(ctx)
	if me, ok := provider.AsMutationError(err); !ok || me.Kind != provider.MutationErrNotFound {
		t.Fatalf("expected not-found, got %v", err)
	}
}

func TestMutationRollback(t *testing.T) {
	path, w := newMutationFixture(t)
	ctx := context.Background()
	tx, _ := w.BeginFeatureTx(ctx, provider.TxOptions{})
	if _, err := tx.Apply(ctx, provider.Mutation{
		Op:          provider.MutationInsert,
		Collection:  "parcels",
		Properties:  map[string]provider.MutationValue{"name": strVal("tmp"), "lots": intVal(1)},
		GeometryWKB: wkbPoint(t, 0, 0),
	}); err != nil {
		t.Fatalf("insert: %v", err)
	}
	if err := tx.Rollback(ctx); err != nil {
		t.Fatalf("rollback: %v", err)
	}
	if n := countRows(t, path); n != 0 {
		t.Fatalf("rows = %d after rollback, want 0", n)
	}
}

func TestMutationUnknownPropertyRejected(t *testing.T) {
	_, w := newMutationFixture(t)
	ctx := context.Background()
	tx, _ := w.BeginFeatureTx(ctx, provider.TxOptions{})
	defer func() { _ = tx.Rollback(ctx) }()
	_, err := tx.Apply(ctx, provider.Mutation{
		Op:          provider.MutationInsert,
		Collection:  "parcels",
		Properties:  map[string]provider.MutationValue{"nope": strVal("x"), "lots": intVal(1)},
		GeometryWKB: wkbPoint(t, 0, 0),
	})
	if err == nil {
		t.Fatal("expected schema violation for unknown property")
	}
	if me, ok := provider.AsMutationError(err); !ok || me.Kind != provider.MutationErrSchemaViolation {
		t.Fatalf("expected schema violation, got %v", err)
	}
}

func TestMutationNotFound(t *testing.T) {
	_, w := newMutationFixture(t)
	ctx := context.Background()
	tx, _ := w.BeginFeatureTx(ctx, provider.TxOptions{})
	defer func() { _ = tx.Rollback(ctx) }()
	for _, op := range []provider.MutationOp{provider.MutationUpdate, provider.MutationReplace, provider.MutationDelete} {
		_, err := tx.Apply(ctx, provider.Mutation{
			Op:         op,
			Collection: "parcels",
			FeatureID:  9999,
			Properties: map[string]provider.MutationValue{"lots": intVal(1)},
		})
		if me, ok := provider.AsMutationError(err); !ok || me.Kind != provider.MutationErrNotFound {
			t.Fatalf("op %v: expected not-found, got %v", op, err)
		}
	}
}

func TestMutationWriterCached(t *testing.T) {
	_, mp1 := newMutationFixture(t)
	_, mp2 := newMutationFixture(t)
	// Same provider returns the same cached Writer: no sql.DB pool is
	// opened per transaction.
	path := filepath.Join(t.TempDir(), "cached.gpkg")
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if _, err := db.Exec(mutationDDL); err != nil {
		t.Fatalf("ddl: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	conf := dict.Dict{
		"filepath": path,
		"layers": []map[string]interface{}{
			{
				"name":               "parcels",
				"tablename":          "parcels",
				"id_fieldname":       "fid",
				"geometry_fieldname": "geom",
				"geometry_format":    "wkb",
				"srid":               4326,
				"fields":             []string{"name", "lots", "price"},
			},
		},
	}
	p, err := gpkg.NewTileProvider(conf, nil)
	if err != nil {
		t.Fatalf("NewTileProvider: %v", err)
	}
	t.Cleanup(gpkg.Cleanup)
	gp := p.(*gpkg.Provider)
	w1 := gp.MutationWriter()
	w2 := gp.MutationWriter()
	if w1 != w2 {
		t.Fatal("MutationWriter not cached: distinct Writer per call leaks sql.DB pools")
	}
	_ = mp1
	_ = mp2
}

func TestMutationGeneratedColumnReadOnly(t *testing.T) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "gen.gpkg")
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	ddl := `CREATE TABLE genparcels (
		fid INTEGER PRIMARY KEY,
		geom BLOB,
		lots INTEGER NOT NULL,
		double_lots INTEGER GENERATED ALWAYS AS (lots * 2) STORED
	)`
	if _, err := db.Exec(ddl); err != nil {
		t.Fatalf("ddl: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	conf := dict.Dict{
		"filepath": path,
		"layers": []map[string]interface{}{
			{
				"name":               "genparcels",
				"tablename":          "genparcels",
				"id_fieldname":       "fid",
				"geometry_fieldname": "geom",
				"geometry_format":    "wkb",
				"srid":               4326,
				"fields":             []string{"lots", "double_lots"},
			},
		},
	}
	p, err := gpkg.NewTileProvider(conf, nil)
	if err != nil {
		t.Fatalf("NewTileProvider: %v", err)
	}
	t.Cleanup(gpkg.Cleanup)
	mp := p.(*gpkg.Provider).MutationWriter()
	wd, err := mp.DescribeWritable(context.Background(), "genparcels")
	if err != nil {
		t.Fatalf("DescribeWritable: %v", err)
	}
	if _, ok := wd.WritableColumns["double_lots"]; ok {
		t.Fatal("generated column admitted as writable")
	}
	found := false
	for _, c := range wd.ReadOnlyColumns {
		if c == "double_lots" {
			found = true
		}
	}
	if !found {
		t.Fatal("generated column not listed as read-only")
	}
	// Writing the generated column must be rejected.
	tx, err := mp.BeginFeatureTx(context.Background(), provider.TxOptions{})
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	_, err = tx.Apply(context.Background(), provider.Mutation{
		Op:         provider.MutationInsert,
		Collection: "genparcels",
		Properties: map[string]provider.MutationValue{
			"lots":        intVal(3),
			"double_lots": intVal(6),
		},
		GeometryWKB: wkbPoint(t, 1, 2),
	})
	if err == nil {
		_ = tx.Rollback(context.Background())
		t.Fatal("insert into generated column accepted")
	}
	_ = tx.Rollback(context.Background())
}

func TestMutationGPKGMetadataMaintained(t *testing.T) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "meta.gpkg")
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	// Minimal GeoPackage metadata: gpkg_contents + an RTree index table.
	setup := []string{
		`CREATE TABLE parcels (fid INTEGER PRIMARY KEY, geom BLOB, name TEXT)`,
		`CREATE TABLE gpkg_contents (table_name TEXT PRIMARY KEY, data_type TEXT, identifier TEXT, description TEXT, last_change DATETIME NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')), min_x DOUBLE, min_y DOUBLE, max_x DOUBLE, max_y DOUBLE, srs_id INTEGER)`,
		`INSERT INTO gpkg_contents (table_name, data_type) VALUES ('parcels', 'features')`,
		`CREATE TABLE gpkg_geometry_columns (table_name TEXT PRIMARY KEY, column_name TEXT, geometry_type_name TEXT, srs_id INTEGER, z TINYINT, m TINYINT)`,
		`INSERT INTO gpkg_geometry_columns (table_name, column_name, geometry_type_name, srs_id, z, m) VALUES ('parcels', 'geom', 'POINT', 4326, 0, 0)`,
		`CREATE VIRTUAL TABLE rtree_parcels_geom USING rtree(id, minx, maxx, miny, maxy)`,
	}
	for _, q := range setup {
		if _, err := db.Exec(q); err != nil {
			t.Fatalf("setup %q: %v", q, err)
		}
	}
	var before string
	if err := db.QueryRow(`SELECT last_change FROM gpkg_contents WHERE table_name='parcels'`).Scan(&before); err != nil {
		t.Fatalf("last_change: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	conf := dict.Dict{
		"filepath": path,
		"layers": []map[string]interface{}{
			{
				"name":               "parcels",
				"tablename":          "parcels",
				"id_fieldname":       "fid",
				"geometry_fieldname": "geom",
				"geometry_format":    "wkb",
				"srid":               4326,
				"fields":             []string{"name"},
			},
		},
	}
	p, err := gpkg.NewTileProvider(conf, nil)
	if err != nil {
		t.Fatalf("NewTileProvider: %v", err)
	}
	t.Cleanup(gpkg.Cleanup)
	mp := p.(*gpkg.Provider).MutationWriter()

	ctx := context.Background()
	tx, err := mp.BeginFeatureTx(ctx, provider.TxOptions{})
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	out, err := tx.Apply(ctx, provider.Mutation{
		Op:          provider.MutationInsert,
		Collection:  "parcels",
		Properties:  map[string]provider.MutationValue{"name": strVal("meta")},
		GeometryWKB: wkbPoint(t, 10, 20),
	})
	if err != nil {
		t.Fatalf("insert: %v", err)
	}
	if _, err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit: %v", err)
	}

	db2, err := sql.Open("sqlite3", path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer func() { _ = db2.Close() }()
	var after string
	if err := db2.QueryRow(`SELECT last_change FROM gpkg_contents WHERE table_name='parcels'`).Scan(&after); err != nil {
		t.Fatalf("last_change after: %v", err)
	}
	if after == before {
		t.Fatal("gpkg_contents.last_change not bumped by commit")
	}
	var minx, maxx, miny, maxy float64
	if err := db2.QueryRow(`SELECT minx, maxx, miny, maxy FROM rtree_parcels_geom WHERE id = ?`, out.FeatureID).Scan(&minx, &maxx, &miny, &maxy); err != nil {
		t.Fatalf("rtree entry missing: %v", err)
	}
	if minx != 10 || maxx != 10 || miny != 20 || maxy != 20 {
		t.Fatalf("rtree bounds wrong: %v %v %v %v", minx, maxx, miny, maxy)
	}

	// Delete must drop the RTree entry.
	tx2, err := mp.BeginFeatureTx(ctx, provider.TxOptions{})
	if err != nil {
		t.Fatalf("begin2: %v", err)
	}
	if _, err := tx2.Apply(ctx, provider.Mutation{Op: provider.MutationDelete, Collection: "parcels", FeatureID: out.FeatureID}); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := tx2.Commit(ctx); err != nil {
		t.Fatalf("commit2: %v", err)
	}
	var n int
	if err := db2.QueryRow(`SELECT COUNT(*) FROM rtree_parcels_geom WHERE id = ?`, out.FeatureID).Scan(&n); err != nil {
		t.Fatalf("rtree count: %v", err)
	}
	if n != 0 {
		t.Fatal("rtree entry not removed on delete")
	}
}
