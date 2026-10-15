//go:build cgo

package gpkg_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"log/slog"
	"reflect"
	"sort"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/alexeydott/geom"
	"github.com/alexeydott/tegola/dict"
	"github.com/alexeydott/tegola/provider"
	"github.com/alexeydott/tegola/provider/gpkg"
	"github.com/alexeydott/tegola/provider/test/fixture"
)

// captureWarns runs f with the default slog logger replaced by one writing
// WARN+ records into a buffer and returns the captured text. internal/log's
// Warnf routes through slog's default logger.
func captureWarns(t *testing.T, f func()) string {
	t.Helper()
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn})))
	defer slog.SetDefault(prev)
	f()
	return buf.String()
}

// captureDebug runs f with the default slog logger replaced by one writing
// DEBUG+ records into a buffer and returns the captured text (WARN lines
// are captured too). Used to assert the tile-query SQL shape via the
// "qtext:" debug record.
func captureDebug(t *testing.T, f func()) string {
	t.Helper()
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	defer slog.SetDefault(prev)
	f()
	return buf.String()
}

// TestNullFeatureIDSkipped covers the gpkg part of audit P6-11: a NULL
// feature id must not silently become ID 0 (duplicate IDs collapse in the
// MVT). The documented strategy is to skip the row and warn, since
// provider.Feature.ID is a plain uint64 and cannot encode "no id".
func TestNullFeatureIDSkipped(t *testing.T) {
	// id is deliberately NOT a rowid alias so NULLs can be stored.
	fx := newRawFixture(t, []string{
		"CREATE TABLE places (id INTEGER, geom BLOB, name TEXT)",
	})
	insertRows(t, fx.path, "places", []string{"id", "geom", "name"}, [][]interface{}{
		{7, wkbGeomBytes(t, geom.Point{50, 50}), "kept"},
		{nil, wkbGeomBytes(t, geom.Point{60, 60}), "dropped"},
	})

	conf := dict.Dict{
		"filepath": fx.path,
		"layers": []map[string]interface{}{
			{
				"name":               "raw_layer",
				"tablename":          "places",
				"id_fieldname":       "id",
				"geometry_fieldname": "geom",
				"geometry_format":    "wkb",
				"srid":               3857,
				"fields":             []string{"name"},
			},
		},
	}

	p, err := gpkg.NewTileProvider(conf, nil)
	if err != nil {
		t.Fatalf("NewTileProvider: %v", err)
	}
	t.Cleanup(gpkg.Cleanup)

	tile := fixture.Tile{
		SRID: 3857,
		BufferedBounds: geom.NewExtent(
			[2]float64{0, 0},
			[2]float64{100, 100},
		),
	}

	var ids []uint64
	var names []interface{}
	err = p.TileFeatures(context.TODO(), "raw_layer", &tile, nil, func(f *provider.Feature) error {
		ids = append(ids, f.ID)
		names = append(names, f.Tags["name"])
		return nil
	})
	if err != nil {
		t.Fatalf("TileFeatures: %v", err)
	}

	if len(ids) != 1 {
		t.Fatalf("feature count = %v (%v), want 1 (NULL-id row skipped)", len(ids), ids)
	}
	if ids[0] != 7 {
		t.Errorf("feature id = %v, want 7", ids[0])
	}
	if names[0] != "kept" {
		t.Errorf("feature tag name = %v, want kept", names[0])
	}
}

// TestBlobTagBase64 covers audit P6-18: BLOB tag values are arbitrary
// binary and must not be string()ed into the MVT as invalid UTF-8. The
// chosen strategy is base64-encoding the raw bytes (lossless, cheap).
func TestBlobTagBase64(t *testing.T) {
	blob := []byte{0x00, 0xff, 0x41, 0x7f, 0xfe}
	fx := newRawFixture(t, []string{
		"CREATE TABLE items (id INTEGER PRIMARY KEY, geom BLOB, data BLOB)",
	})
	insertRows(t, fx.path, "items", []string{"geom", "data"}, [][]interface{}{
		{wkbGeomBytes(t, geom.Point{50, 50}), blob},
	})

	conf := dict.Dict{
		"filepath": fx.path,
		"layers": []map[string]interface{}{
			{
				"name":               "raw_layer",
				"tablename":          "items",
				"id_fieldname":       "id",
				"geometry_fieldname": "geom",
				"geometry_format":    "wkb",
				"srid":               3857,
				"fields":             []string{"data"},
			},
		},
	}

	p, err := gpkg.NewTileProvider(conf, nil)
	if err != nil {
		t.Fatalf("NewTileProvider: %v", err)
	}
	t.Cleanup(gpkg.Cleanup)

	tile := fixture.Tile{
		SRID: 3857,
		BufferedBounds: geom.NewExtent(
			[2]float64{0, 0},
			[2]float64{100, 100},
		),
	}

	var tags []map[string]interface{}
	err = p.TileFeatures(context.TODO(), "raw_layer", &tile, nil, func(f *provider.Feature) error {
		tags = append(tags, f.Tags)
		return nil
	})
	if err != nil {
		t.Fatalf("TileFeatures: %v", err)
	}
	if len(tags) != 1 {
		t.Fatalf("feature count = %v, want 1", len(tags))
	}

	want := base64.StdEncoding.EncodeToString(blob)
	got, ok := tags[0]["data"].(string)
	if !ok {
		t.Fatalf("blob tag = %#v (%T), want a string", tags[0]["data"], tags[0]["data"])
	}
	if got != want {
		t.Errorf("blob tag = %q, want %q (standard base64 of raw bytes)", got, want)
	}
	if !utf8.ValidString(got) {
		t.Errorf("blob tag %q is not valid UTF-8", got)
	}
}

// TestMixedCaseColumnNames covers audit P6-17: column-name lookups must be
// case-insensitive because SQLite column names can differ in case from the
// configured names. Two seams: the registration colSet lookup (tablename
// layers) and the id/geom scan dispatch (custom SQL layers, where the result
// columns carry the table's own casing).
func TestMixedCaseColumnNames(t *testing.T) {
	// table columns deliberately differ in case from the configured names
	fx := newRawFixture(t, []string{
		"CREATE TABLE mixed (ID INTEGER, Geom BLOB, Name TEXT)",
	})
	insertRows(t, fx.path, "mixed", []string{"ID", "Geom", "Name"}, [][]interface{}{
		{7, wkbGeomBytes(t, geom.Point{50, 50}), "kept"},
	})

	t.Run("tablename layer registration", func(t *testing.T) {
		conf := dict.Dict{
			"filepath": fx.path,
			"layers": []map[string]interface{}{
				{
					"name":               "raw_layer",
					"tablename":          "mixed",
					"id_fieldname":       "id",
					"geometry_fieldname": "geom",
					"geometry_format":    "wkb",
					"srid":               3857,
					"fields":             []string{"name"},
				},
			},
		}

		p, err := gpkg.NewTileProvider(conf, nil)
		if err != nil {
			// pre-fix the case-sensitive colSet lookup fails with
			// "table ... has no geometry column ..."
			t.Fatalf("NewTileProvider: %v", err)
		}
		t.Cleanup(gpkg.Cleanup)

		f := fetchOneFeature(t, p, "raw_layer")
		if f.ID != 7 {
			t.Errorf("feature id = %v, want 7 (id column matched case-insensitively)", f.ID)
		}
		if f.Geometry == nil {
			t.Errorf("feature geometry = nil, want decoded point (geom column matched case-insensitively)")
		}
		// the result columns carry the table's own casing
		if f.Tags["Name"] != "kept" {
			t.Errorf("feature tags = %v, want Name=kept", f.Tags)
		}
	})

	t.Run("custom sql layer scan dispatch", func(t *testing.T) {
		conf := dict.Dict{
			"filepath": fx.path,
			"layers": []map[string]interface{}{
				{
					"name":               "raw_layer",
					"sql":                "SELECT * FROM mixed",
					"id_fieldname":       "id",
					"geometry_fieldname": "geom",
					"geometry_format":    "wkb",
					"srid":               3857,
					"fields":             []string{"Name"},
				},
			},
		}

		p, err := gpkg.NewTileProvider(conf, nil)
		if err != nil {
			t.Fatalf("NewTileProvider: %v", err)
		}
		t.Cleanup(gpkg.Cleanup)

		f := fetchOneFeature(t, p, "raw_layer")
		// SELECT * returns the table's own column casing, so the id/geom
		// dispatch must compare case-insensitively (pre-fix the id lands
		// in tags and the geometry is nil)
		if f.ID != 7 {
			t.Errorf("feature id = %v, want 7 (id column matched case-insensitively)", f.ID)
		}
		if f.Geometry == nil {
			t.Errorf("feature geometry = nil, want decoded point (geom column matched case-insensitively)")
		}
		if f.Tags["Name"] != "kept" {
			t.Errorf("feature tags = %v, want Name=kept", f.Tags)
		}
	})
}

// fetchOneFeature returns the single feature produced for a layer covering
// the fixture point at (50, 50).
func fetchOneFeature(t *testing.T, p provider.Tiler, layer string) provider.Feature {
	t.Helper()

	tile := fixture.Tile{
		SRID: 3857,
		BufferedBounds: geom.NewExtent(
			[2]float64{0, 0},
			[2]float64{100, 100},
		),
	}

	var got []provider.Feature
	err := p.TileFeatures(context.TODO(), layer, &tile, nil, func(f *provider.Feature) error {
		got = append(got, *f)
		return nil
	})
	if err != nil {
		t.Fatalf("TileFeatures: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("feature count = %v, want 1", len(got))
	}
	return got[0]
}

// gpkgPointBlob crafts a GeoPackage binary geometry blob: 8-byte header
// (magic 'GP', version 0, flags 0x01 = little-endian + no envelope) plus a
// little-endian WKB point body.
func gpkgPointBlob(t *testing.T, srid int32, x, y float64) []byte {
	t.Helper()
	hdr := make([]byte, 8)
	hdr[0], hdr[1], hdr[2], hdr[3] = 'G', 'P', 0, 0x01
	binary.LittleEndian.PutUint32(hdr[4:8], uint32(srid))
	return append(hdr, wkbGeomBytes(t, geom.Point{x, y})...)
}

// TestGeometryColumnSelectedByConfig exercises audit P6-13 end to end: a
// table with multiple geometry columns must serve the column named by
// geometry_fieldname (with its own SRID), not whichever column won the old
// last-one-wins metadata map, and an explicit unknown geometry column must
// fail registration with a clear error.
func TestGeometryColumnSelectedByConfig(t *testing.T) {
	ddl := `
		CREATE TABLE dual (fid INTEGER PRIMARY KEY, geom_a BLOB, geom_b BLOB, note TEXT);
		CREATE TABLE gpkg_contents (table_name TEXT NOT NULL, data_type TEXT NOT NULL, min_x DOUBLE, min_y DOUBLE, max_x DOUBLE, max_y DOUBLE, srs_id INTEGER);
		CREATE TABLE gpkg_geometry_columns (table_name TEXT NOT NULL, column_name TEXT NOT NULL, geometry_type_name TEXT NOT NULL, srs_id INTEGER NOT NULL, z TINYINT NOT NULL, m TINYINT NOT NULL);
		CREATE TABLE rtree_dual_geom_a (id INTEGER, minx DOUBLE, maxx DOUBLE, miny DOUBLE, maxy DOUBLE);
		CREATE TABLE rtree_dual_geom_b (id INTEGER, minx DOUBLE, maxx DOUBLE, miny DOUBLE, maxy DOUBLE);`
	fx := newRawFixture(t, []string{ddl})
	insertRows(t, fx.path, "dual",
		[]string{"fid", "geom_a", "geom_b", "note"},
		[][]interface{}{
			{1, gpkgPointBlob(t, 3857, 1, 1), gpkgPointBlob(t, 4326, 2, 2), "note"},
		})
	insertRows(t, fx.path, "gpkg_contents",
		[]string{"table_name", "data_type", "min_x", "min_y", "max_x", "max_y", "srs_id"},
		[][]interface{}{{"dual", "features", 0.0, 0.0, 10.0, 10.0, nil}})
	insertRows(t, fx.path, "gpkg_geometry_columns",
		[]string{"table_name", "column_name", "geometry_type_name", "srs_id", "z", "m"},
		[][]interface{}{
			// inserted out of name order: pre-fix the map kept geom_b
			{"dual", "geom_b", "POINT", 4326, 0, 0},
			{"dual", "geom_a", "POINT", 3857, 0, 0},
		})
	for _, rt := range []string{"rtree_dual_geom_a", "rtree_dual_geom_b"} {
		insertRows(t, fx.path, rt,
			[]string{"id", "minx", "maxx", "miny", "maxy"},
			[][]interface{}{{1, 0.0, 10.0, 0.0, 10.0}})
	}

	conf := dict.Dict{
		"filepath": fx.path,
		"srid":     3857,
		"layers": []map[string]interface{}{
			{
				"name":               "dual_layer",
				"tablename":          "dual",
				"id_fieldname":       "fid",
				"geometry_fieldname": "geom_a",
				"fields":             []string{"note"},
			},
		},
	}
	p, err := gpkg.NewTileProvider(conf, nil)
	if err != nil {
		t.Fatalf("NewTileProvider: %v", err)
	}
	t.Cleanup(gpkg.Cleanup)

	f := fetchOneFeature(t, p, "dual_layer")
	want := geom.Point{1, 1}
	if fmt.Sprintf("%v", f.Geometry) != fmt.Sprintf("%v", want) {
		t.Errorf("geometry = %v (%T), expected %v (geom_a blob; pre-P6-13 the last gpkg_geometry_columns row won and geom_b was served)",
			f.Geometry, f.Geometry, want)
	}

	// explicit unknown geometry column: clear registration error
	conf["layers"] = []map[string]interface{}{
		{
			"name":               "bad_layer",
			"tablename":          "dual",
			"id_fieldname":       "fid",
			"geometry_fieldname": "geom_no_such",
			"fields":             []string{"note"},
		},
	}
	if _, err := gpkg.NewTileProvider(conf, nil); err == nil {
		t.Fatal("NewTileProvider with unknown geometry_fieldname errored = false, expected true")
	} else if !strings.Contains(err.Error(), "no geometry column") {
		t.Errorf("error = %q, expected it to name the missing geometry column", err.Error())
	}
}

// TestSRSFallbackWarnings covers audit P6-14: GeoPackage SRS ids 0
// (undefined) and -1 (cartesian engineering CRS) as well as NULL srs_id
// values must not silently fall back to web mercator - registration warns
// with a hint (0: configure srid 4326 for lon/lat data). An explicitly
// configured srid keeps its documented override semantics and must not warn.
// Pre-fix: none of these warnings were emitted.
func TestSRSFallbackWarnings(t *testing.T) {
	const srsDDL = `CREATE TABLE gpkg_contents (table_name TEXT, data_type TEXT, srs_id INTEGER, min_x DOUBLE, min_y DOUBLE, max_x DOUBLE, max_y DOUBLE);
CREATE TABLE gpkg_geometry_columns (table_name TEXT, column_name TEXT, geometry_type_name TEXT, srs_id INTEGER, z TINYINT, m TINYINT);
CREATE TABLE rtree_t1_geom (id INTEGER, minx DOUBLE, maxx DOUBLE, miny DOUBLE, maxy DOUBLE);`

	mkFixture := func(t *testing.T, gcSrsID interface{}) string {
		t.Helper()
		fx := newRawFixture(t, []string{
			"CREATE TABLE t1 (fid INTEGER PRIMARY KEY, geom BLOB, note TEXT)",
			srsDDL,
		})
		insertRows(t, fx.path, "gpkg_contents", []string{"table_name", "data_type", "srs_id"}, [][]interface{}{
			{"t1", "features", nil},
		})
		insertRows(t, fx.path, "gpkg_geometry_columns", []string{"table_name", "column_name", "geometry_type_name", "srs_id", "z", "m"}, [][]interface{}{
			{"t1", "geom", "GEOMETRY", gcSrsID, 0, 0},
		})
		insertRows(t, fx.path, "t1", []string{"fid", "geom", "note"}, [][]interface{}{
			{1, wkbGeomBytes(t, geom.Point{1, 1}), "a"},
		})
		return fx.path
	}
	baseConf := func(path string) dict.Dict {
		return dict.Dict{
			"filepath": path,
			"layers": []map[string]interface{}{
				{
					"name":               "t1",
					"tablename":          "t1",
					"id_fieldname":       "fid",
					"geometry_fieldname": "geom",
					"fields":             []string{"note"},
				},
			},
		}
	}

	t.Run("srs_id 0 suggests 4326", func(t *testing.T) {
		path := mkFixture(t, 0)
		out := captureWarns(t, func() {
			p, err := gpkg.NewTileProvider(baseConf(path), nil)
			if err != nil {
				t.Fatalf("NewTileProvider errored = %v", err)
			}
			t.Cleanup(gpkg.Cleanup)
			_ = p
		})
		for _, want := range []string{"undefined per the GeoPackage spec", "configure srid 4326"} {
			if !strings.Contains(out, want) {
				t.Errorf("warning %q missing from logs: %s", want, out)
			}
		}
	})

	t.Run("srs_id -1 cartesian", func(t *testing.T) {
		path := mkFixture(t, -1)
		out := captureWarns(t, func() {
			p, err := gpkg.NewTileProvider(baseConf(path), nil)
			if err != nil {
				t.Fatalf("NewTileProvider errored = %v", err)
			}
			t.Cleanup(gpkg.Cleanup)
			_ = p
		})
		for _, want := range []string{"cartesian engineering CRS", "falling back to srid"} {
			if !strings.Contains(out, want) {
				t.Errorf("warning %q missing from logs: %s", want, out)
			}
		}
	})

	t.Run("srs_id NULL", func(t *testing.T) {
		path := mkFixture(t, nil)
		out := captureWarns(t, func() {
			p, err := gpkg.NewTileProvider(baseConf(path), nil)
			if err != nil {
				t.Fatalf("NewTileProvider errored = %v", err)
			}
			t.Cleanup(gpkg.Cleanup)
			_ = p
		})
		for _, want := range []string{"is NULL", "falling back to srid"} {
			if !strings.Contains(out, want) {
				t.Errorf("warning %q missing from logs: %s", want, out)
			}
		}
	})

	t.Run("explicit srid suppresses the warning", func(t *testing.T) {
		path := mkFixture(t, 0)
		conf := baseConf(path)
		conf["srid"] = 4326
		out := captureWarns(t, func() {
			p, err := gpkg.NewTileProvider(conf, nil)
			if err != nil {
				t.Fatalf("NewTileProvider errored = %v", err)
			}
			t.Cleanup(gpkg.Cleanup)
			_ = p
		})
		if strings.Contains(out, "falling back") {
			t.Errorf("explicit srid must keep override semantics without warnings, got: %s", out)
		}
	})

	t.Run("custom sql header srs_id 0", func(t *testing.T) {
		fx := newRawFixture(t, []string{
			"CREATE TABLE t1 (fid INTEGER PRIMARY KEY, geom BLOB, note TEXT)",
		})
		insertRows(t, fx.path, "t1", []string{"fid", "geom", "note"}, [][]interface{}{
			{1, gpkgPointBlob(t, 0, 1, 1), "a"},
		})
		conf := dict.Dict{
			"filepath": fx.path,
			"layers": []map[string]interface{}{
				{
					"name":               "custom",
					"sql":                "SELECT * FROM t1",
					"id_fieldname":       "fid",
					"geometry_fieldname": "geom",
					"fields":             []string{"note"},
				},
			},
		}
		out := captureWarns(t, func() {
			p, err := gpkg.NewTileProvider(conf, nil)
			if err != nil {
				t.Fatalf("NewTileProvider errored = %v", err)
			}
			t.Cleanup(gpkg.Cleanup)
			_ = p
		})
		for _, want := range []string{"geometry header SRS ID is 0", "configure srid 4326"} {
			if !strings.Contains(out, want) {
				t.Errorf("warning %q missing from logs: %s", want, out)
			}
		}
	})
}

// TestRawLayerWithoutBoundsWarns covers audit P6-19: a raw-format layer
// whose table has no bounds columns full-table-scans per tile request -
// registration must warn, naming the layer and the perf cost. Pre-fix: no
// warning was emitted.
func TestRawLayerWithoutBoundsWarns(t *testing.T) {
	mkConf := func(path string) dict.Dict {
		return dict.Dict{
			"filepath": path,
			"layers": []map[string]interface{}{
				{
					"name":               "raw_layer",
					"tablename":          "places",
					"id_fieldname":       "id",
					"geometry_fieldname": "geom",
					"geometry_format":    "wkb",
					"srid":               3857,
					"fields":             []string{"name"},
				},
			},
		}
	}

	t.Run("warns without bounds columns", func(t *testing.T) {
		fx := newRawFixture(t, []string{
			"CREATE TABLE places (id INTEGER, geom BLOB, name TEXT)",
		})
		insertRows(t, fx.path, "places", []string{"id", "geom", "name"}, [][]interface{}{
			{1, wkbGeomBytes(t, geom.Point{50, 50}), "a"},
		})
		out := captureWarns(t, func() {
			p, err := gpkg.NewTileProvider(mkConf(fx.path), nil)
			if err != nil {
				t.Fatalf("NewTileProvider errored = %v", err)
			}
			t.Cleanup(gpkg.Cleanup)
			_ = p
		})
		for _, want := range []string{"raw_layer", "places", "full-table-scan"} {
			if !strings.Contains(out, want) {
				t.Errorf("warning %q missing from logs: %s", want, out)
			}
		}
	})

	t.Run("no warning with bounds columns", func(t *testing.T) {
		fx := newRawFixture(t, []string{
			"CREATE TABLE places (id INTEGER, geom BLOB, name TEXT, minx DOUBLE, maxx DOUBLE, miny DOUBLE, maxy DOUBLE)",
		})
		insertRows(t, fx.path, "places", []string{"id", "geom", "name", "minx", "maxx", "miny", "maxy"}, [][]interface{}{
			{1, wkbGeomBytes(t, geom.Point{50, 50}), "a", 50.0, 50.0, 50.0, 50.0},
		})
		out := captureWarns(t, func() {
			p, err := gpkg.NewTileProvider(mkConf(fx.path), nil)
			if err != nil {
				t.Fatalf("NewTileProvider errored = %v", err)
			}
			t.Cleanup(gpkg.Cleanup)
			_ = p
		})
		if strings.Contains(out, "full-table-scan") {
			t.Errorf("bounds columns present, expected no scan-cost warning, got: %s", out)
		}
	})
}

// TestRTreeAndIDChecks covers audit P5-5 and the UPSTREAM debt 0.6b
// closure: registration picks and caches the tile-query plan - the
// rtree_<table>_<column> shadow table is checked once via sqlite_master,
// and when it is missing the layer registers with a WARN and a fallback
// plan (never a per-request probe, never a cryptic "no such table" per
// tile request) - and the configured id column is expected to be the
// rowid alias (INTEGER PRIMARY KEY), warn otherwise. The tile query joins
// the RTree on rowid, so the join stays deterministic even when the id
// column is not the rowid alias. Pre-fix: no registration checks existed
// and the join matched the configured id column against the RTree's
// rowid, silently dropping rows when the two differ.
func TestRTreeAndIDChecks(t *testing.T) {
	const metaDDL = `CREATE TABLE gpkg_contents (table_name TEXT, data_type TEXT, srs_id INTEGER, min_x DOUBLE, min_y DOUBLE, max_x DOUBLE, max_y DOUBLE);
CREATE TABLE gpkg_geometry_columns (table_name TEXT, column_name TEXT, geometry_type_name TEXT, srs_id INTEGER, z TINYINT, m TINYINT);
CREATE TABLE rtree_t1_geom (id INTEGER, minx DOUBLE, maxx DOUBLE, miny DOUBLE, maxy DOUBLE);`
	const metaDDLNoRTree = `CREATE TABLE gpkg_contents (table_name TEXT, data_type TEXT, srs_id INTEGER, min_x DOUBLE, min_y DOUBLE, max_x DOUBLE, max_y DOUBLE);
CREATE TABLE gpkg_geometry_columns (table_name TEXT, column_name TEXT, geometry_type_name TEXT, srs_id INTEGER, z TINYINT, m TINYINT);`

	mkNativeFixture := func(t *testing.T, ddl string, idCol string, ids []interface{}) string {
		t.Helper()
		// "fid" fixtures model the GeoPackage rowid alias (INTEGER
		// PRIMARY KEY); "id" fixtures are plain non-alias columns.
		idDef := fmt.Sprintf("%v INTEGER", idCol)
		if idCol == "fid" {
			idDef += " PRIMARY KEY"
		}
		fx := newRawFixture(t, []string{
			fmt.Sprintf("CREATE TABLE t1 (%v, geom BLOB, note TEXT)", idDef),
			ddl,
		})
		insertRows(t, fx.path, "gpkg_contents", []string{"table_name", "data_type", "srs_id"}, [][]interface{}{
			{"t1", "features", 3857},
		})
		insertRows(t, fx.path, "gpkg_geometry_columns", []string{"table_name", "column_name", "geometry_type_name", "srs_id", "z", "m"}, [][]interface{}{
			{"t1", "geom", "GEOMETRY", 3857, 0, 0},
		})
		geomPts := []geom.Point{{50, 50}, {60, 60}}
		rows := make([][]interface{}, len(ids))
		for i := range ids {
			rows[i] = []interface{}{ids[i], gpkgPointBlob(t, 3857, geomPts[i][0], geomPts[i][1]), "n"}
		}
		insertRows(t, fx.path, "t1", []string{idCol, "geom", "note"}, rows)
		// RTree entries key on rowid (insertion order 1..n).
		if strings.Contains(ddl, "rtree_t1_geom") {
			insertRows(t, fx.path, "rtree_t1_geom", []string{"id", "minx", "maxx", "miny", "maxy"}, [][]interface{}{
				{1, 40.0, 60.0, 40.0, 60.0},
				{2, 50.0, 70.0, 50.0, 70.0},
			})
		}
		return fx.path
	}
	mkConf := func(path, idField string) dict.Dict {
		return dict.Dict{
			"filepath": path,
			"layers": []map[string]interface{}{
				{
					"name":               "t1",
					"tablename":          "t1",
					"id_fieldname":       idField,
					"geometry_fieldname": "geom",
					"fields":             []string{"note"},
				},
			},
		}
	}
	fetchIDs := func(t *testing.T, p provider.Tiler) []uint64 {
		t.Helper()
		tile := fixture.Tile{
			SRID:           3857,
			BufferedBounds: geom.NewExtent([2]float64{0, 0}, [2]float64{100, 100}),
		}
		var ids []uint64
		err := p.TileFeatures(context.TODO(), "t1", &tile, nil, func(f *provider.Feature) error {
			ids = append(ids, f.ID)
			return nil
		})
		if err != nil {
			t.Fatalf("TileFeatures: %v", err)
		}
		sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
		return ids
	}

	t.Run("missing rtree warns and registers with fallback", func(t *testing.T) {
		// DELIBERATE change (UPSTREAM debt 0.6b closure): the former
		// registration ERROR is now a WARN + fallback plan. This fixture
		// has no bounds columns either, so the layer degrades to the
		// full-scan plan and still serves in-tile features through the
		// mandatory in-memory exact filter.
		path := mkNativeFixture(t, metaDDLNoRTree, "fid", []interface{}{1, 2})
		var ids []uint64
		out := captureWarns(t, func() {
			p, err := gpkg.NewTileProvider(mkConf(path, "fid"), nil)
			if err != nil {
				t.Fatalf("NewTileProvider errored = %v, want WARN + fallback registration", err)
			}
			t.Cleanup(gpkg.Cleanup)
			ids = fetchIDs(t, p)
		})
		for _, want := range []string{"rtree_t1_geom", "CreateRTreeIndex", "t1", "full-table-scan"} {
			if !strings.Contains(out, want) {
				t.Errorf("warning %q missing %q", out, want)
			}
		}
		if want := []uint64{1, 2}; !reflect.DeepEqual(ids, want) {
			t.Errorf("feature ids = %v, want %v (fallback plan must still serve in-tile features)", ids, want)
		}
	})

	t.Run("missing id column errors", func(t *testing.T) {
		path := mkNativeFixture(t, metaDDL, "fid", []interface{}{1, 2})
		_, err := gpkg.NewTileProvider(mkConf(path, "nope"), nil)
		if err == nil || !strings.Contains(err.Error(), `has no id column "nope"`) {
			t.Errorf("error = %v, want clear missing-id-column error", err)
		}
	})

	t.Run("non-alias id warns and joins on rowid", func(t *testing.T) {
		// id values deliberately differ from rowids: pre-fix the join
		// l.id = si.id matched nothing (0 features).
		path := mkNativeFixture(t, metaDDL, "id", []interface{}{10, 20})
		var ids []uint64
		out := captureWarns(t, func() {
			p, err := gpkg.NewTileProvider(mkConf(path, "id"), nil)
			if err != nil {
				t.Fatalf("NewTileProvider errored = %v", err)
			}
			t.Cleanup(gpkg.Cleanup)
			ids = fetchIDs(t, p)
		})
		if !strings.Contains(out, "not an INTEGER PRIMARY KEY") || !strings.Contains(out, "t1") {
			t.Errorf("expected rowid-alias warning naming table t1, got: %s", out)
		}
		if want := []uint64{10, 20}; !reflect.DeepEqual(ids, want) {
			t.Errorf("feature ids = %v, want %v (rowid join must match rows regardless of the id column)", ids, want)
		}
	})

	t.Run("rowid alias id is silent and deterministic", func(t *testing.T) {
		path := mkNativeFixture(t, metaDDL, "fid", []interface{}{1, 2})
		var ids []uint64
		out := captureWarns(t, func() {
			p, err := gpkg.NewTileProvider(mkConf(path, "fid"), nil)
			if err != nil {
				t.Fatalf("NewTileProvider errored = %v", err)
			}
			t.Cleanup(gpkg.Cleanup)
			ids = fetchIDs(t, p)
		})
		if strings.Contains(out, "not an INTEGER PRIMARY KEY") {
			t.Errorf("fid is the rowid alias, expected no warning, got: %s", out)
		}
		if want := []uint64{1, 2}; !reflect.DeepEqual(ids, want) {
			t.Errorf("feature ids = %v, want %v", ids, want)
		}
	})
}

// TestTileQueryPlanSelection covers the UPSTREAM debt 0.6b closure: the
// RTree existence check happens ONCE at layer registration (a sqlite_master
// probe for the rtree_<table>_<column> shadow table), the result is cached
// as the layer's tile-query plan, and every plan keeps exactly one SQL
// round trip per tile request - never a per-request probe. planRTree keeps
// the upstream RTree JOIN SQL; without the shadow table the layer registers
// with a WARN and falls back to planBBox (bounds columns on the table
// itself) or, when there are no bounds columns either, planScan (full scan
// with the mandatory in-memory exact filter).
func TestTileQueryPlanSelection(t *testing.T) {
	const metaDDL = `CREATE TABLE gpkg_contents (table_name TEXT, data_type TEXT, srs_id INTEGER, min_x DOUBLE, min_y DOUBLE, max_x DOUBLE, max_y DOUBLE);
CREATE TABLE gpkg_geometry_columns (table_name TEXT, column_name TEXT, geometry_type_name TEXT, srs_id INTEGER, z TINYINT, m TINYINT);`

	// row 3 is outside the (0,0)-(100,100) test tile: every plan must
	// drop it and serve exactly [1,2].
	pts := []geom.Point{{50, 50}, {60, 60}, {200, 200}}
	mkFixture := func(t *testing.T, withRTree, withBounds bool) string {
		t.Helper()
		t1 := "CREATE TABLE t1 (fid INTEGER PRIMARY KEY, geom BLOB, note TEXT"
		if withBounds {
			t1 += ", minx DOUBLE, maxx DOUBLE, miny DOUBLE, maxy DOUBLE"
		}
		t1 += ")"
		ddl := metaDDL
		if withRTree {
			ddl += "\nCREATE TABLE rtree_t1_geom (id INTEGER, minx DOUBLE, maxx DOUBLE, miny DOUBLE, maxy DOUBLE);"
		}
		fx := newRawFixture(t, []string{t1, ddl})
		insertRows(t, fx.path, "gpkg_contents", []string{"table_name", "data_type", "srs_id"}, [][]interface{}{
			{"t1", "features", 3857},
		})
		insertRows(t, fx.path, "gpkg_geometry_columns", []string{"table_name", "column_name", "geometry_type_name", "srs_id", "z", "m"}, [][]interface{}{
			{"t1", "geom", "GEOMETRY", 3857, 0, 0},
		})
		cols := []string{"fid", "geom", "note"}
		if withBounds {
			cols = append(cols, "minx", "maxx", "miny", "maxy")
		}
		rows := make([][]interface{}, len(pts))
		for i, pt := range pts {
			row := []interface{}{i + 1, gpkgPointBlob(t, 3857, pt[0], pt[1]), "n"}
			if withBounds {
				row = append(row, pt[0], pt[0], pt[1], pt[1])
			}
			rows[i] = row
		}
		insertRows(t, fx.path, "t1", cols, rows)
		if withRTree {
			// RTree entries key on rowid and cover every row, like a
			// real shadow table.
			entries := make([][]interface{}, len(pts))
			for i, pt := range pts {
				entries[i] = []interface{}{i + 1, pt[0], pt[0], pt[1], pt[1]}
			}
			insertRows(t, fx.path, "rtree_t1_geom", []string{"id", "minx", "maxx", "miny", "maxy"}, entries)
		}
		return fx.path
	}
	conf := func(path string) dict.Dict {
		return dict.Dict{
			"filepath": path,
			"layers": []map[string]interface{}{
				{
					"name":               "t1",
					"tablename":          "t1",
					"id_fieldname":       "fid",
					"geometry_fieldname": "geom",
					"fields":             []string{"note"},
				},
			},
		}
	}
	fetchIDs := func(t *testing.T, p provider.Tiler) []uint64 {
		t.Helper()
		tile := fixture.Tile{
			SRID:           3857,
			BufferedBounds: geom.NewExtent([2]float64{0, 0}, [2]float64{100, 100}),
		}
		var ids []uint64
		err := p.TileFeatures(context.TODO(), "t1", &tile, nil, func(f *provider.Feature) error {
			ids = append(ids, f.ID)
			return nil
		})
		if err != nil {
			t.Fatalf("TileFeatures: %v", err)
		}
		sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
		return ids
	}
	run := func(t *testing.T, path string) ([]uint64, string) {
		t.Helper()
		var ids []uint64
		out := captureDebug(t, func() {
			p, err := gpkg.NewTileProvider(conf(path), nil)
			if err != nil {
				t.Fatalf("NewTileProvider errored = %v", err)
			}
			t.Cleanup(gpkg.Cleanup)
			ids = fetchIDs(t, p)
		})
		return ids, out
	}
	// qtextLine pins the "exactly one SQL round trip per tile request"
	// contract and returns the tile-query SQL line (the shape the plan
	// controls); warn lines may legitimately mention bounds column names.
	qtextLine := func(t *testing.T, out string) string {
		t.Helper()
		n := 0
		var line string
		for _, l := range strings.Split(out, "\n") {
			if strings.Contains(l, "qtext:") {
				n++
				line = l
			}
		}
		if n != 1 {
			t.Errorf("tile request logged %d SQL round trips (qtext lines), want exactly 1", n)
		}
		return line
	}
	wantIDs := []uint64{1, 2}

	t.Run("rtree shadow table present uses the upstream join plan", func(t *testing.T) {
		ids, out := run(t, mkFixture(t, true, false))
		q := qtextLine(t, out)
		if !strings.Contains(q, "JOIN") || !strings.Contains(q, "rtree_t1_geom") || !strings.Contains(q, "si ON l.rowid = si.id") {
			t.Errorf("planRTree query must keep the upstream RTree JOIN, got: %s", q)
		}
		if strings.Contains(out, "CreateRTreeIndex") {
			t.Errorf("rtree present, expected no fallback warning, got: %s", out)
		}
		if !reflect.DeepEqual(ids, wantIDs) {
			t.Errorf("feature ids = %v, want %v", ids, wantIDs)
		}
	})

	t.Run("missing rtree with bounds columns falls back to plain-bbox plan", func(t *testing.T) {
		ids, out := run(t, mkFixture(t, false, true))
		for _, want := range []string{"rtree_t1_geom", "CreateRTreeIndex", "plain-bbox"} {
			if !strings.Contains(out, want) {
				t.Errorf("fallback warning missing %q, got: %s", want, out)
			}
		}
		q := qtextLine(t, out)
		if strings.Contains(q, "JOIN") {
			t.Errorf("planBBox query must not join the rtree shadow table, got: %s", q)
		}
		if !strings.Contains(q, "minx") || !strings.Contains(q, "maxy") {
			t.Errorf("planBBox query must filter on the table's bounds columns, got: %s", q)
		}
		if !reflect.DeepEqual(ids, wantIDs) {
			t.Errorf("feature ids = %v, want %v (bbox predicate must drop out-of-tile rows)", ids, wantIDs)
		}
	})

	t.Run("missing rtree and bounds columns falls back to full-scan plan", func(t *testing.T) {
		ids, out := run(t, mkFixture(t, false, false))
		for _, want := range []string{"rtree_t1_geom", "CreateRTreeIndex", "full-table-scan"} {
			if !strings.Contains(out, want) {
				t.Errorf("fallback warning missing %q, got: %s", want, out)
			}
		}
		q := qtextLine(t, out)
		if strings.Contains(q, "JOIN") {
			t.Errorf("planScan query must not join the rtree shadow table, got: %s", q)
		}
		if strings.Contains(q, "minx") || strings.Contains(q, "maxy") {
			t.Errorf("planScan query must carry no bounds predicate (the in-memory filter does it), got: %s", q)
		}
		if !reflect.DeepEqual(ids, wantIDs) {
			t.Errorf("feature ids = %v, want %v (in-memory filter must drop out-of-tile rows)", ids, wantIDs)
		}
	})
}
