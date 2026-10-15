//go:build cgo

package gpkg_test

import (
	"context"
	"database/sql"
	"encoding/binary"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alexeydott/geom"
	"github.com/alexeydott/geom/encoding/wkb"
	"github.com/alexeydott/tegola/dict"
	"github.com/alexeydott/tegola/mos"
	"github.com/alexeydott/tegola/provider"
	"github.com/alexeydott/tegola/provider/geometrycodec"
	"github.com/alexeydott/tegola/provider/gpkg"
	"github.com/alexeydott/tegola/provider/test/fixture"
)

// openSQLite opens the fixture database; the sqlite3 driver is registered by
// the gpkg package import.
func openSQLite(path string) (*sql.DB, error) {
	return sql.Open("sqlite3", path)
}

// rawFixture builds a SQLite file holding a plain table of raw-format
// geometries plus optional GPKG metadata tables, so raw-format layers can be
// exercised without gpkg_geometry_columns support.
type rawFixture struct {
	path string
}

// newRawFixture creates an empty SQLite database with the given tables.
func newRawFixture(t *testing.T, tables []string) rawFixture {
	t.Helper()

	path := filepath.Join(t.TempDir(), "raw.gpkg")
	db := fixture.OpenDB(t, "sqlite3", path, tables...)
	if err := db.Close(); err != nil {
		t.Fatalf("close sqlite fixture: %v", err)
	}
	return rawFixture{path: path}
}

// mosSystemInfoBlob assembles a MapplGIS LayerInfo blob (packed record,
// mirrors mos/systeminfo_test.go systemInfoBlobBuilder).
func mosSystemInfoBlob(precision int32, projection string, mapUnits byte, unitsDefined bool) []byte {
	buf := make([]byte, 64+len(projection))
	buf[0] = byte(len("Ver 1"))
	copy(buf[1:], "Ver 1")
	binary.LittleEndian.PutUint32(buf[11:15], uint32(precision))
	if projection != "" {
		buf[15] = 1
	}
	binary.LittleEndian.PutUint32(buf[26:30], uint32(1))
	buf[52] = mapUnits
	if unitsDefined {
		buf[53] = 1
	}
	binary.LittleEndian.PutUint32(buf[60:64], uint32(len(projection)))
	copy(buf[64:], projection)
	return buf
}

// mosPolylineBlob assembles a minimal MOS polyline blob (native 10-byte
// header + per-subobject point count, quantized coordinates,
// precision 2 => hundredths of a unit).
func mosPolylineBlob(points [][2]int32) []byte {
	buf := make([]byte, 0, 14+8*len(points))
	buf = append(buf, mos.TypePolyline, 0)
	buf = binary.LittleEndian.AppendUint16(buf, 0) // add flag
	buf = binary.LittleEndian.AppendUint16(buf, 1) // sub-object count
	buf = binary.LittleEndian.AppendUint32(buf, uint32(len(points)))
	buf = binary.LittleEndian.AppendUint32(buf, uint32(len(points))) // per-subobject point count
	for _, p := range points {
		buf = binary.LittleEndian.AppendUint32(buf, uint32(p[0]))
		buf = binary.LittleEndian.AppendUint32(buf, uint32(p[1]))
	}
	return buf
}

func wkbGeomBytes(t *testing.T, g geom.Geometry) []byte {
	t.Helper()
	b, err := wkb.EncodeBytes(g)
	if err != nil {
		t.Fatalf("encode wkb: %v", err)
	}
	return b
}

// TestRawFormatTableLayer covers the R3-02 audit items: raw-format (wkb /
// wkt / mos) layers backed by plain SQLite tables that are NOT registered in
// gpkg_geometry_columns.
// TestMOSPointAndBBox covers the R3-08 e2e contract items for the gpkg
// provider: Point geometries survive the decode pipeline, bbox inside/outside
// filtering holds, and a malformed MOS blob yields a controlled error rather
// than a panic.
func TestMOSPointAndBBox(t *testing.T) {
	t.Run("mos point inside bbox is emitted", func(t *testing.T) {
		fx := newRawFixture(t, []string{
			"CREATE TABLE mos_layer (id INTEGER PRIMARY KEY AUTOINCREMENT, geom BLOB)",
		})
		insertRows(t, fx.path, "mos_layer", []string{"geom"}, [][]interface{}{
			{mosSystemInfoBlob(2, "", byte(mos.UnitsMetres), true)},
			// point object: type 2, one sub-object with one point (200, 400)
			// quantized => (2, 4) metres with precision 2
			{mosPointBlob([][2]int32{{200, 400}})},
		})

		conf := dict.Dict{
			"filepath": fx.path,
			"layers": []map[string]interface{}{
				{
					"name":            "mos_layer",
					"tablename":       "mos_layer",
					"geometry_format": "mos",
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
				[2]float64{10, 10},
			),
		}
		var count int
		err = p.TileFeatures(context.TODO(), "mos_layer", &tile, nil, func(f *provider.Feature) error {
			count++
			pt, ok := f.Geometry.(geom.Point)
			if !ok {
				t.Errorf("geometry type = %T, want geom.Point", f.Geometry)
				return nil
			}
			if pt[0] != 2 || pt[1] != 4 {
				t.Errorf("point = %v, want [2 4]", pt)
			}
			return nil
		})
		if err != nil {
			t.Fatalf("TileFeatures: %v", err)
		}
		if count != 1 {
			t.Errorf("feature count = %v, want 1", count)
		}
	})

	t.Run("mos point outside bbox is filtered", func(t *testing.T) {
		fx := newRawFixture(t, []string{
			"CREATE TABLE mos_layer (id INTEGER PRIMARY KEY AUTOINCREMENT, geom BLOB)",
		})
		insertRows(t, fx.path, "mos_layer", []string{"geom"}, [][]interface{}{
			{mosSystemInfoBlob(2, "", byte(mos.UnitsMetres), true)},
			// (9000, 9000) metres, far outside the tile window below
			{mosPointBlob([][2]int32{{900000, 900000}})},
		})

		conf := dict.Dict{
			"filepath": fx.path,
			"layers": []map[string]interface{}{
				{
					"name":            "mos_layer",
					"tablename":       "mos_layer",
					"geometry_format": "mos",
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
				[2]float64{10, 10},
			),
		}
		var count int
		err = p.TileFeatures(context.TODO(), "mos_layer", &tile, nil, func(f *provider.Feature) error {
			count++
			return nil
		})
		if err != nil {
			t.Fatalf("TileFeatures: %v", err)
		}
		if count != 0 {
			t.Errorf("feature count = %v, want 0 (outside bbox)", count)
		}
	})

	t.Run("malformed mos blob yields controlled error", func(t *testing.T) {
		// a truncated polyline blob (no point coordinates) must produce a
		// controlled error at provider registration, never a panic.
		fx := newRawFixture(t, []string{
			"CREATE TABLE mos_layer (id INTEGER PRIMARY KEY AUTOINCREMENT, geom BLOB)",
		})
		insertRows(t, fx.path, "mos_layer", []string{"geom"}, [][]interface{}{
			{mosSystemInfoBlob(2, "", byte(mos.UnitsMetres), true)},
			// polyline header claiming one sub-object but with no points:
			// native 10-byte header truncated after the count fields
			{[]byte{mos.TypePolyline, 0, 0, 0, 1, 0, 1, 0, 0, 0, 0, 0}},
		})

		conf := dict.Dict{
			"filepath": fx.path,
			"layers": []map[string]interface{}{
				{
					"name":            "mos_layer",
					"tablename":       "mos_layer",
					"geometry_format": "mos",
				},
			},
		}
		_, err := gpkg.NewTileProvider(conf, nil)
		if err == nil {
			t.Fatal("expected NewTileProvider to fail on a malformed MOS blob, got nil")
		}
		if !strings.Contains(err.Error(), "invalid geometry prefix") {
			t.Fatalf("unexpected error for malformed blob: %v", err)
		}
		t.Logf("got expected controlled error: %v", err)
	})
}

// mosPointBlob assembles a minimal MOS point blob (native 10-byte header,
// one sub-object, one quantized point).
func mosPointBlob(points [][2]int32) []byte {
	buf := make([]byte, 0, 14+8*len(points))
	buf = append(buf, mos.TypePoint, 0)
	buf = binary.LittleEndian.AppendUint16(buf, 0) // add flag
	buf = binary.LittleEndian.AppendUint16(buf, 1) // sub-object count
	buf = binary.LittleEndian.AppendUint32(buf, uint32(len(points)))
	buf = binary.LittleEndian.AppendUint32(buf, uint32(len(points))) // per-subobject point count
	for _, p := range points {
		buf = binary.LittleEndian.AppendUint32(buf, uint32(p[0]))
		buf = binary.LittleEndian.AppendUint32(buf, uint32(p[1]))
	}
	return buf
}

func TestRawFormatTableLayer(t *testing.T) {
	t.Run("wkb plain table no rtree", func(t *testing.T) {
		// table is NOT in gpkg_geometry_columns; registration must succeed
		// via SQLite inspection and TileFeatures must filter in memory.
		fx := newRawFixture(t, []string{
			"CREATE TABLE roads (id INTEGER PRIMARY KEY AUTOINCREMENT, geom BLOB, name TEXT)",
		})
		insertRows(t, fx.path, "roads", []string{"geom", "name"}, [][]interface{}{
			{wkbGeomBytes(t, geom.LineString{{-100, -100}, {100, 100}}), "a"},
			{wkbGeomBytes(t, geom.LineString{{-1000, -1000}, {-500, -500}}), "b"},
		})

		conf := dict.Dict{
			"filepath": fx.path,
			"layers": []map[string]interface{}{
				{
					"name":            "raw_layer",
					"tablename":       "roads",
					"geometry_format": "wkb",
					"srid":            3857,
				},
			},
		}

		p, err := gpkg.NewTileProvider(conf, nil)
		if err != nil {
			t.Fatalf("NewTileProvider: %v", err)
		}
		// providers keep their SQLite connection open until Cleanup;
		// registered here so the file is released before t.TempDir's
		// RemoveAll runs (t.Cleanup is LIFO within the subtest).
		t.Cleanup(gpkg.Cleanup)

		tile := fixture.Tile{
			SRID: 3857,
			BufferedBounds: geom.NewExtent(
				[2]float64{-200, -200},
				[2]float64{200, 200},
			),
		}

		var count int
		err = p.TileFeatures(context.TODO(), "raw_layer", &tile, nil, func(f *provider.Feature) error {
			count++
			if f.SRID != 3857 {
				t.Errorf("feature SRID = %v, want 3857", f.SRID)
			}
			return nil
		})
		if err != nil {
			t.Fatalf("TileFeatures: %v", err)
		}
		if count != 1 {
			t.Errorf("feature count = %v, want 1 (in-memory bbox filter)", count)
		}
	})

	t.Run("wkt plain table", func(t *testing.T) {
		fx := newRawFixture(t, []string{
			"CREATE TABLE pois (id INTEGER PRIMARY KEY AUTOINCREMENT, geom TEXT)",
		})
		insertRows(t, fx.path, "pois", []string{"geom"}, [][]interface{}{
			{"POINT(10 10)"},
			{"POINT(9000 9000)"},
		})

		conf := dict.Dict{
			"filepath": fx.path,
			"layers": []map[string]interface{}{
				{
					"name":            "raw_layer",
					"tablename":       "pois",
					"geometry_format": "wkt",
					"srid":            3857,
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
		var count int
		err = p.TileFeatures(context.TODO(), "raw_layer", &tile, nil, func(f *provider.Feature) error {
			count++
			return nil
		})
		if err != nil {
			t.Fatalf("TileFeatures: %v", err)
		}
		if count != 1 {
			t.Errorf("feature count = %v, want 1", count)
		}
	})

	t.Run("mos table with system info first", func(t *testing.T) {
		// The system-info blob precedes geometries: registration must
		// consume it (precision 2, metre units), infer the geometry type
		// from the first real geometry, and never emit the metadata row.
		// Projection registers a synthetic CRS and shifts the feature SRID
		// away from the explicit layer value.
		fx := newRawFixture(t, []string{
			"CREATE TABLE mos_layer (id INTEGER PRIMARY KEY AUTOINCREMENT, geom BLOB)",
		})
		insertRows(t, fx.path, "mos_layer", []string{"geom"}, [][]interface{}{
			// system info: precision 2, +proj=merc (metres), units defined
			{mosSystemInfoBlob(2, "+proj=merc +ellps=WGS84 +datum=WGS84 +units=m +no_defs", byte(mos.UnitsMetres), true)},
			// quantized 1 => 0.01 with precision 2
			{mosPolylineBlob([][2]int32{{100, 100}, {300, 300}})},
		})

		conf := dict.Dict{
			"filepath": fx.path,
			"layers": []map[string]interface{}{
				{
					"name":            "raw_layer",
					"tablename":       "mos_layer",
					"geometry_format": "mos",
				},
			},
		}
		p, err := gpkg.NewTileProvider(conf, nil)
		if err != nil {
			t.Fatalf("NewTileProvider: %v", err)
		}
		t.Cleanup(gpkg.Cleanup)

		// decoded coords are 0.01-scale metres (1..3 metres); a small
		// WebMercator window around the origin must contain them.
		tile := fixture.Tile{
			SRID: 3857,
			BufferedBounds: geom.NewExtent(
				[2]float64{-10, -10},
				[2]float64{10, 10},
			),
		}
		var count int
		var firstGeom geom.Geometry
		err = p.TileFeatures(context.TODO(), "raw_layer", &tile, nil, func(f *provider.Feature) error {
			count++
			if firstGeom == nil {
				firstGeom = f.Geometry
			}
			return nil
		})
		if err != nil {
			t.Fatalf("TileFeatures: %v", err)
		}
		if count != 1 {
			t.Fatalf("feature count = %v, want 1 (system-info row must be skipped)", count)
		}
		ls, ok := firstGeom.(geom.LineString)
		if !ok {
			t.Fatalf("geometry type = %T, want geom.LineString", firstGeom)
		}
		if len(ls) != 2 || ls[0][0] != 1 || ls[1][0] != 3 {
			t.Errorf("linestring = %v, want [[1 1] [3 3]] (precision 2 applied)", ls)
		}
	})

	t.Run("mos explicit precision overrides system info", func(t *testing.T) {
		// explicit mos_precision must win over the system-info value
		fx := newRawFixture(t, []string{
			"CREATE TABLE mos_layer (id INTEGER PRIMARY KEY AUTOINCREMENT, geom BLOB)",
		})
		insertRows(t, fx.path, "mos_layer", []string{"geom"}, [][]interface{}{
			{mosSystemInfoBlob(3, "", byte(mos.UnitsMillimetres), true)},
			{mosPolylineBlob([][2]int32{{1000, 1000}, {3000, 3000}})},
		})

		conf := dict.Dict{
			"filepath": fx.path,
			"layers": []map[string]interface{}{
				{
					"name":            "raw_layer",
					"tablename":       "mos_layer",
					"geometry_format": "mos",
					"mos_precision":   2, // explicit, overrides system-info's 3
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
				[2]float64{-10, -10},
				[2]float64{10, 10},
			),
		}
		var count int
		err = p.TileFeatures(context.TODO(), "raw_layer", &tile, nil, func(f *provider.Feature) error {
			count++
			ls, ok := f.Geometry.(geom.LineString)
			if !ok {
				t.Errorf("geometry type = %T, want geom.LineString", f.Geometry)
				return nil
			}
			// explicit precision 2 => 1000/100 = 10; system-info millimetre
			// units (not overridden) scale it by 0.001 => 0.01. Before the
			// PrecisionSet merge fix the system-info precision 3 won and
			// produced 0.001.
			if ls[0][0] != 0.01 {
				t.Errorf("linestring start x = %v, want 0.01 (explicit precision 2, mm units)", ls[0][0])
			}
			return nil
		})
		if err != nil {
			t.Fatalf("TileFeatures: %v", err)
		}
		if count != 1 {
			t.Errorf("feature count = %v, want 1", count)
		}
	})

	t.Run("negative gpkg format on plain table fails", func(t *testing.T) {
		// geometry_format=gpkg against a table that is not registered in
		// gpkg_geometry_columns must fail with a clear error, not silently
		// succeed or panic.
		fx := newRawFixture(t, []string{
			"CREATE TABLE plain (id INTEGER PRIMARY KEY AUTOINCREMENT, geom BLOB)",
		})
		insertRows(t, fx.path, "plain", []string{"geom"}, [][]interface{}{
			{mosPolylineBlob([][2]int32{{1, 1}, {2, 2}})},
		})

		conf := dict.Dict{
			"filepath": fx.path,
			"layers": []map[string]interface{}{
				{
					"name":            "raw_layer",
					"tablename":       "plain",
					"geometry_format": "gpkg",
				},
			},
		}
		_, err := gpkg.NewTileProvider(conf, nil)
		if err == nil {
			t.Fatal("expected NewTileProvider to fail with geometry_format=gpkg on a plain table, got nil")
		}
		t.Logf("got expected error: %v", err)
	})

	t.Run("raw custom sql mos system info first", func(t *testing.T) {
		// custom SQL against a MOS table where the system-info blob is the
		// first row of the probe query: it must be skipped (SQL layers
		// never apply system info) and the probe must still count the
		// decodable geometry row. The layer carries bounds columns and the
		// bounds-backed !BBOX! token per the MOS custom-SQL contract.
		fx := newRawFixture(t, []string{
			"CREATE TABLE mos_layer (id INTEGER PRIMARY KEY AUTOINCREMENT, geom BLOB, MINX DOUBLE, MAXX DOUBLE, MINY DOUBLE, MAXY DOUBLE)",
		})
		insertRows(t, fx.path, "mos_layer", []string{"geom", "MINX", "MAXX", "MINY", "MAXY"}, [][]interface{}{
			{mosSystemInfoBlob(2, "", byte(mos.UnitsMetres), true), nil, nil, nil, nil},
			{mosPolylineBlob([][2]int32{{100, 100}, {300, 300}}), 1.0, 3.0, 1.0, 3.0},
		})

		conf := dict.Dict{
			"filepath": fx.path,
			"layers": []map[string]interface{}{
				{
					"name":            "raw_layer",
					"sql":             "SELECT id, geom, MINX, MAXX, MINY, MAXY FROM mos_layer WHERE !BBOX!",
					"geometry_format": "mos",
					// MOS custom SQL requires an explicit CRS (A11)
					"srid": 3857,
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
				[2]float64{-10, -10},
				[2]float64{10, 10},
			),
		}
		var count int
		err = p.TileFeatures(context.TODO(), "raw_layer", &tile, nil, func(f *provider.Feature) error {
			count++
			return nil
		})
		if err != nil {
			t.Fatalf("TileFeatures: %v", err)
		}
		if count != 1 {
			t.Errorf("feature count = %v, want 1", count)
		}
	})

	t.Run("raw custom sql mos system info not applied", func(t *testing.T) {
		// custom SQL against a MOS table where the system-info blob carries
		// a non-empty projection: it must be skipped without applying the
		// projection (SQL layers must configure srid/crs_defn explicitly —
		// the sample probe never decodes LayerInfo projection), and the
		// next row decoded.
		fx := newRawFixture(t, []string{
			"CREATE TABLE mos_layer (id INTEGER PRIMARY KEY AUTOINCREMENT, geom BLOB, MINX DOUBLE, MAXX DOUBLE, MINY DOUBLE, MAXY DOUBLE)",
		})
		proj4 := "+proj=merc +lat_ts=56.5 +ellps=clrk66 +type=crs"
		insertRows(t, fx.path, "mos_layer", []string{"geom", "MINX", "MAXX", "MINY", "MAXY"}, [][]interface{}{
			{mosSystemInfoBlob(2, proj4, byte(mos.UnitsMetres), true), nil, nil, nil, nil},
			{mosPolylineBlob([][2]int32{{100, 100}, {300, 300}}), 1.0, 3.0, 1.0, 3.0},
		})

		conf := dict.Dict{
			"filepath": fx.path,
			"layers": []map[string]interface{}{
				{
					"name":            "raw_layer",
					"sql":             "SELECT id, geom, MINX, MAXX, MINY, MAXY FROM mos_layer WHERE !BBOX!",
					"geometry_format": "mos",
					// MOS custom SQL requires an explicit CRS (A11)
					"srid": 3857,
				},
			},
		}
		p, err := gpkg.NewTileProvider(conf, nil)
		if err != nil {
			t.Fatalf("NewTileProvider: %v", err)
		}
		t.Cleanup(gpkg.Cleanup)

		// the system info projection must NOT be applied: the layer SRID
		// stays the provider default (3857)
		lyrs, lerr := p.Layers()
		if lerr != nil {
			t.Fatalf("Layers: %v", lerr)
		}
		if len(lyrs) != 1 {
			t.Fatalf("layer count = %v, want 1", len(lyrs))
		}
		if srid := lyrs[0].SRID(); srid != 3857 {
			t.Errorf("layer srid = %v, want 3857 (system info projection must not be applied to SQL layers)", srid)
		}

		tile := fixture.Tile{
			SRID: 3857,
			BufferedBounds: geom.NewExtent(
				[2]float64{-10, -10},
				[2]float64{10, 10},
			),
		}
		var count int
		err = p.TileFeatures(context.TODO(), "raw_layer", &tile, nil, func(f *provider.Feature) error {
			count++
			return nil
		})
		if err != nil {
			t.Fatalf("TileFeatures: %v", err)
		}
		if count != 1 {
			t.Errorf("feature count = %v, want 1", count)
		}
	})

	t.Run("raw custom sql empty result registers", func(t *testing.T) {
		// custom SQL returning no rows must register a placeholder layer
		// rather than fail startup.
		fx := newRawFixture(t, []string{
			"CREATE TABLE mos_layer (id INTEGER PRIMARY KEY AUTOINCREMENT, geom BLOB, MINX DOUBLE, MAXX DOUBLE, MINY DOUBLE, MAXY DOUBLE)",
		})
		conf := dict.Dict{
			"filepath": fx.path,
			"layers": []map[string]interface{}{
				{
					"name":            "raw_layer",
					"sql":             "SELECT id, geom, MINX, MAXX, MINY, MAXY FROM mos_layer WHERE id = -1 AND !BBOX!",
					"geometry_format": "mos",
					// MOS custom SQL requires an explicit CRS (A11)
					"srid": 3857,
				},
			},
		}
		p, err := gpkg.NewTileProvider(conf, nil)
		if err != nil {
			t.Fatalf("NewTileProvider: %v", err)
		}
		t.Cleanup(gpkg.Cleanup)

		layers, err := p.Layers()
		if err != nil {
			t.Fatalf("Layers: %v", err)
		}
		if len(layers) != 1 {
			t.Fatalf("layer count = %v, want 1", len(layers))
		}
	})

	t.Run("raw custom sql with bbox token rejected", func(t *testing.T) {
		// Raw custom-SQL contract: raw formats (wkb/wkt/mos) cannot use the
		// native-spatial !BBOX! token (its expansion assumes a native spatial
		// column); startup must fail with a descriptive error instead of
		// generating invalid per-tile SQL. Matching is case-insensitive,
		// mirroring uppercaseTokens normalization at tile time.
		for _, tc := range []struct {
			name string
			sql  string
		}{
			{"upper", "SELECT id, geom FROM wkb_layer WHERE geom && !BBOX!"},
			{"lower", "SELECT id, geom FROM wkb_layer WHERE geom && !bbox!"},
		} {
			fx := newRawFixture(t, []string{
				"CREATE TABLE wkb_layer (id INTEGER PRIMARY KEY AUTOINCREMENT, geom BLOB)",
			})
			insertRows(t, fx.path, "wkb_layer", []string{"geom"}, [][]interface{}{
				{wkbGeomBytes(t, geom.Point{10, 20})},
			})

			conf := dict.Dict{
				"filepath": fx.path,
				"layers": []map[string]interface{}{
					{
						"name":            "raw_layer",
						"sql":             tc.sql,
						"geometry_format": "wkb",
					},
				},
			}
			_, err := gpkg.NewTileProvider(conf, nil)
			t.Cleanup(gpkg.Cleanup)
			if err == nil {
				t.Fatalf("%v: expected NewTileProvider to reject !BBOX! in raw custom SQL", tc.name)
			}
			if !strings.Contains(err.Error(), "raw_layer") {
				t.Errorf("%v: error must name the layer, got: %v", tc.name, err)
			}
			if !strings.Contains(err.Error(), "in-memory") {
				t.Errorf("%v: error must suggest the in-memory filter, got: %v", tc.name, err)
			}
		}
	})

	t.Run("missing geometry column fails", func(t *testing.T) {
		fx := newRawFixture(t, []string{
			"CREATE TABLE plain (id INTEGER PRIMARY KEY AUTOINCREMENT, data BLOB)",
		})
		conf := dict.Dict{
			"filepath": fx.path,
			"layers": []map[string]interface{}{
				{
					"name":            "raw_layer",
					"tablename":       "plain",
					"geometry_format": "wkb",
				},
			},
		}
		_, err := gpkg.NewTileProvider(conf, nil)
		if err == nil {
			t.Fatal("expected NewTileProvider to fail with missing geometry column, got nil")
		}
		t.Logf("got expected error: %v", err)
	})
}

// TestMOSDeferredCustomSQLPreQuery covers the contract for tile-dependent
// custom-SQL MOS layers: registration skips startup geometry inspection for
// statements that cannot be neutralized permissively (position tokens), and
// the runtime tile query decodes rows with the configured quantization. SQL
// layers never apply system info, so a runtime system-info blob yields a
// controlled decode error instead of silently configuring the layer.
func TestMOSDeferredCustomSQLPreQuery(t *testing.T) {
	fx := newRawFixture(t, []string{
		"CREATE TABLE mos_layer (id INTEGER PRIMARY KEY AUTOINCREMENT, geom BLOB, MINX DOUBLE, MAXX DOUBLE, MINY DOUBLE, MAXY DOUBLE)",
	})
	// the table is empty at registration: tile-dependent custom SQL defers
	// startup geometry inspection entirely

	conf := dict.Dict{
		"filepath": fx.path,
		"layers": []map[string]interface{}{
			{
				"name":            "mos_layer",
				"geometry_format": "mos",
				// MOS custom SQL requires an explicit CRS (A11)
				"srid": 3857,
				// tile-dependent SQL exercising the position tokens plus
				// the bounds-backed !BBOX! required for MOS custom SQL
				"sql": "SELECT id, geom, MINX, MAXX, MINY, MAXY FROM mos_layer WHERE !ZOOM! >= 0 AND !X! >= 0 AND !Y! >= 0 AND !BBOX!",
			},
		},
	}
	p, err := gpkg.NewTileProvider(conf, nil)
	if err != nil {
		t.Fatalf("NewTileProvider: %v", err)
	}
	t.Cleanup(gpkg.Cleanup)

	// data appears after registration; the quantized blob decodes with the
	// default MOS precision for metres (2), no system info is ever applied
	// to SQL layers
	insertRows(t, fx.path, "mos_layer", []string{"geom", "MINX", "MAXX", "MINY", "MAXY"}, [][]interface{}{
		// quantized (200, 400) with default precision 2 => (2.0, 4.0);
		// bounds columns cover the tile window so the !BBOX! predicate
		// selects the row
		{mosPointBlob([][2]int32{{200, 400}}), 0.0, 10.0, 0.0, 10.0},
	})

	bounds := geom.NewExtent([2]float64{0, 0}, [2]float64{10, 10})
	tile := &fixture.Tile{
		Z: 5, X: 0, Y: 0, SRID: 3857,
		Bounds: bounds, BufferedBounds: bounds,
	}
	var count int
	err = p.TileFeatures(context.TODO(), "mos_layer", tile, nil, func(f *provider.Feature) error {
		count++
		pt, ok := f.Geometry.(geom.Point)
		if !ok {
			t.Errorf("geometry type = %T, want geom.Point", f.Geometry)
			return nil
		}
		if pt[0] != 2.0 || pt[1] != 4.0 {
			t.Errorf("point = %v, want [2 4] (default MOS precision 2 must be used)", pt)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("TileFeatures: %v", err)
	}
	if count != 1 {
		t.Errorf("feature count = %v, want 1", count)
	}
}

// insertRows inserts rows into the given table with named columns (the id
// column is autoincrement and is omitted). nil values become SQL NULL.
func insertRows(t *testing.T, path, table string, cols []string, rows [][]interface{}) {
	t.Helper()

	db, err := openSQLite(path)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer func() { _ = db.Close() }()

	for _, row := range rows {
		if len(row) != len(cols) {
			t.Fatalf("insert: %v values for %v columns", len(row), len(cols))
		}
		ph := make([]string, 0, len(row))
		args := make([]interface{}, 0, len(row))
		for _, v := range row {
			if v == nil {
				// keep column alignment with a NULL literal: positional
				// placeholders cannot express a skipped column
				ph = append(ph, "NULL")
				continue
			}
			ph = append(ph, "?")
			args = append(args, v)
		}
		q := fmt.Sprintf("INSERT INTO %v (%v) VALUES (%v)", table, joinStrings(cols, ", "), joinStrings(ph, ", "))
		if _, err := db.Exec(q, args...); err != nil {
			t.Fatalf("insert %v: %v", q, err)
		}
	}
}

func joinStrings(parts []string, sep string) string {
	out := ""
	for i, p := range parts {
		if i > 0 {
			out += sep
		}
		out += p
	}
	return out
}

func TestMOSLayerInfoPositionInvariant(t *testing.T) {
	// Sampling invariant (docs/provider-contract.md): the MOS system-info
	// blob must configure a *table* layer identically wherever it sits
	// inside the shared 16-row sample window — first row, middle or last.
	// For custom-SQL layers system info is never applied (explicit
	// srid/crs_defn contract), so the SRID stays the provider default.
	proj4 := "+proj=merc +lat_ts=56.5 +ellps=clrk66 +type=crs"
	geomBlob := mosPolylineBlob([][2]int32{{100, 100}, {300, 300}})

	for _, pos := range []int{1, 5, geometrycodec.InspectionSampleLimit} {
		for _, mode := range []string{"table", "custom sql"} {
			pos, mode := pos, mode
			t.Run(fmt.Sprintf("%s position %d", mode, pos), func(t *testing.T) {
				fx := newRawFixture(t, []string{
					"CREATE TABLE mos_layer (id INTEGER PRIMARY KEY AUTOINCREMENT, geom BLOB, MINX DOUBLE, MAXX DOUBLE, MINY DOUBLE, MAXY DOUBLE)",
				})
				rows := make([][]interface{}, 0, geometrycodec.InspectionSampleLimit)
				for i := 1; i <= geometrycodec.InspectionSampleLimit; i++ {
					if i == pos {
						rows = append(rows, []interface{}{mosSystemInfoBlob(2, proj4, byte(mos.UnitsMetres), true), nil, nil, nil, nil})
						continue
					}
					rows = append(rows, []interface{}{geomBlob, 1.0, 3.0, 1.0, 3.0})
				}
				insertRows(t, fx.path, "mos_layer", []string{"geom", "MINX", "MAXX", "MINY", "MAXY"}, rows)

				layerConf := map[string]interface{}{
					"name":            "raw_layer",
					"geometry_format": "mos",
				}
				if mode == "custom sql" {
					layerConf["sql"] = "SELECT id, geom, MINX, MAXX, MINY, MAXY FROM mos_layer WHERE !BBOX!"
					// MOS custom SQL requires an explicit CRS (A11); the
					// system-info projection must never fill it in
					layerConf["srid"] = 3857
				} else {
					layerConf["tablename"] = "mos_layer"
				}
				conf := dict.Dict{
					"filepath": fx.path,
					"layers":   []map[string]interface{}{layerConf},
				}
				p, err := gpkg.NewTileProvider(conf, nil)
				if err != nil {
					t.Fatalf("NewTileProvider: %v", err)
				}
				t.Cleanup(gpkg.Cleanup)

				lyrs, lerr := p.Layers()
				if lerr != nil {
					t.Fatalf("Layers: %v", lerr)
				}
				if len(lyrs) != 1 {
					t.Fatalf("layer count = %v, want 1", len(lyrs))
				}
				if mode == "table" {
					// the projection must register the synthetic SRID in
					// every position, never the provider default (3857)
					if srid := lyrs[0].SRID(); srid == 3857 {
						t.Errorf("layer srid = %v, want the synthetic SRID from the system info projection", srid)
					}
				} else if srid := lyrs[0].SRID(); srid != 3857 {
					// custom SQL layers never apply system info: the SRID
					// stays the provider default
					t.Errorf("layer srid = %v, want 3857 (SQL layers must not apply system info)", srid)
				}

				tile := fixture.Tile{
					SRID: 3857,
					BufferedBounds: geom.NewExtent(
						[2]float64{-10, -10},
						[2]float64{10, 10},
					),
				}
				var count int
				err = p.TileFeatures(context.TODO(), "raw_layer", &tile, nil, func(f *provider.Feature) error {
					count++
					return nil
				})
				if err != nil {
					t.Fatalf("TileFeatures: %v", err)
				}
				if count != geometrycodec.InspectionSampleLimit-1 {
					t.Errorf("feature count = %v, want %v (system-info row skipped)", count, geometrycodec.InspectionSampleLimit-1)
				}
			})
		}
	}
}

// TestConfiguredBoundsFilterAndPlainMinxTag covers two audit items end to
// end on a raw wkb table:
//   - N6: explicitly configured bbox_*_fieldname columns (non-standard
//     names) drive the SQL bounds filter. The out-of-window row carries a
//     corrupt geometry blob, so it only ever reaches the WKB decoder when
//     the SQL filter failed to exclude it — a full table scan fails the
//     tile query with a decode error.
//   - 7.2.5: a plain tag column merely named minx (not one of the layer's
//     bounds columns) is an ordinary tag, while real bounds columns are
//     never tags.
func TestConfiguredBoundsFilterAndPlainMinxTag(t *testing.T) {
	t.Run("configured bounds columns filter in SQL and plain minx is a tag", func(t *testing.T) {
		fx := newRawFixture(t, []string{
			"CREATE TABLE parcels (id INTEGER PRIMARY KEY AUTOINCREMENT, geom BLOB, name TEXT, x0 REAL, x1 REAL, y0 REAL, y1 REAL, minx TEXT)",
		})
		insertRows(t, fx.path, "parcels", []string{"geom", "name", "x0", "x1", "y0", "y1", "minx"}, [][]interface{}{
			{wkbGeomBytes(t, geom.Point{5, 5}), "a", 0.0, 10.0, 0.0, 10.0, "keep-me"},
			// out-of-window row with a corrupt geometry blob: it is
			// decoded only if the SQL bounds filter did not exclude it
			{[]byte{0x01, 0x02}, "b", 400.0, 600.0, 400.0, 600.0, "far"},
		})

		conf := dict.Dict{
			"filepath": fx.path,
			"layers": []map[string]interface{}{
				{
					"name":            "raw_layer",
					"tablename":       "parcels",
					"geometry_format": "wkb",
					"geometry_type":   "Point",
					"srid":            3857,
					// tag columns are config-driven for tablename layers:
					// include the plain minx column and the configured
					// bounds columns to pin the exclusion contract
					"fields":              []string{"name", "minx", "x0", "x1", "y0", "y1"},
					"bbox_minx_fieldname": "x0",
					"bbox_maxx_fieldname": "x1",
					"bbox_miny_fieldname": "y0",
					"bbox_maxy_fieldname": "y1",
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
				[2]float64{-10, -10},
				[2]float64{10, 10},
			),
		}
		var count int
		err = p.TileFeatures(context.TODO(), "raw_layer", &tile, nil, func(f *provider.Feature) error {
			count++
			if f.Tags["minx"] != "keep-me" {
				t.Errorf("column minx is not a bounds field and must be an ordinary tag, got %v", f.Tags["minx"])
			}
			if f.Tags["name"] != "a" {
				t.Errorf("tag name = %v, want a", f.Tags["name"])
			}
			for _, col := range []string{"x0", "x1", "y0", "y1"} {
				if _, ok := f.Tags[col]; ok {
					t.Errorf("bounds column %s must never be a tag", col)
				}
			}
			return nil
		})
		if err != nil {
			t.Fatalf("TileFeatures: %v", err)
		}
		if count != 1 {
			t.Errorf("feature count = %v, want 1 (SQL bounds filter over the configured columns)", count)
		}
	})

	t.Run("legacy bounds columns stay excluded from tags", func(t *testing.T) {
		fx := newRawFixture(t, []string{
			"CREATE TABLE legacy (id INTEGER PRIMARY KEY AUTOINCREMENT, geom BLOB, name TEXT, minx REAL, maxx REAL, miny REAL, maxy REAL, min_zoom INTEGER, max_zoom INTEGER)",
		})
		insertRows(t, fx.path, "legacy", []string{"geom", "name", "minx", "maxx", "miny", "maxy", "min_zoom", "max_zoom"}, [][]interface{}{
			{wkbGeomBytes(t, geom.Point{5, 5}), "a", 0.0, 10.0, 0.0, 10.0, 1, 22},
		})

		conf := dict.Dict{
			"filepath": fx.path,
			"layers": []map[string]interface{}{
				{
					"name":            "raw_layer",
					"tablename":       "legacy",
					"geometry_format": "wkb",
					"geometry_type":   "Point",
					"srid":            3857,
					"fields":          []string{"name", "minx", "min_zoom", "max_zoom"},
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
				[2]float64{-10, -10},
				[2]float64{10, 10},
			),
		}
		err = p.TileFeatures(context.TODO(), "raw_layer", &tile, nil, func(f *provider.Feature) error {
			if f.Tags["name"] != "a" {
				t.Errorf("tag name = %v, want a", f.Tags["name"])
			}
			for _, col := range []string{"minx", "maxx", "miny", "maxy", "min_zoom", "max_zoom"} {
				if _, ok := f.Tags[col]; ok {
					t.Errorf("bounds/zoom column %s must never be a tag", col)
				}
			}
			return nil
		})
		if err != nil {
			t.Fatalf("TileFeatures: %v", err)
		}
	})
}

// SQLite raises integer overflow if the geometry expression is evaluated.
// Explicit MOS plus a geometry type must obtain metadata without doing that.
func TestExplicitMOSCustomSQLDoesNotEvaluateGeometry(t *testing.T) {
	for _, providerFormat := range []bool{false, true} {
		t.Run(fmt.Sprintf("providerFormat=%v", providerFormat), func(t *testing.T) {
			fx := newRawFixture(t, []string{"CREATE TABLE parcels (id INTEGER PRIMARY KEY, geom BLOB)", "INSERT INTO parcels VALUES (1, X'00')"})
			layer := map[string]interface{}{
				"name": "raw_layer", "geometry_fieldname": "geom", "id_fieldname": "id",
				"geometry_type": "Point", "srid": 3857,
				"sql": "SELECT id, abs(-9223372036854775808) AS geom, 0 AS MINX, 1 AS MAXX, 0 AS MINY, 1 AS MAXY FROM parcels WHERE !BBOX!",
			}
			conf := dict.Dict{"filepath": fx.path, "layers": []map[string]interface{}{layer}}
			if providerFormat {
				conf["geometry_format"] = "mos"
			} else {
				layer["geometry_format"] = "mos"
			}
			_, err := gpkg.NewTileProvider(conf, nil)
			if err != nil {
				t.Fatalf("explicit MOS evaluated geometry: %v", err)
			}
			t.Cleanup(gpkg.Cleanup)
		})
	}
}

func TestAutoMOSShortSQLRegistersAndRenders(t *testing.T) {
	for _, badPosition := range []int{-1, 0, 1} {
		t.Run(fmt.Sprintf("badPosition=%d", badPosition), func(t *testing.T) {
			fx := newRawFixture(t, []string{"CREATE TABLE parcels (id INTEGER PRIMARY KEY AUTOINCREMENT, geom BLOB, MINX REAL, MAXX REAL, MINY REAL, MAXY REAL)"})
			good := []interface{}{mosPointBlob([][2]int32{{200, 400}}), 0, 1000, 0, 1000}
			bad := []interface{}{[]byte{255, 0}, 0, 1000, 0, 1000}
			rows := [][]interface{}{good}
			if badPosition == 0 {
				rows = [][]interface{}{bad, good}
			} else if badPosition == 1 {
				rows = append(rows, bad)
			}
			insertRows(t, fx.path, "parcels", []string{"geom", "MINX", "MAXX", "MINY", "MAXY"}, rows)
			conf := dict.Dict{"filepath": fx.path, "layers": []map[string]interface{}{{"name": "short", "geometry_fieldname": "geom", "id_fieldname": "id", "srid": 3857, "sql": "SELECT id, geom, MINX, MAXX, MINY, MAXY FROM parcels WHERE !BBOX! ORDER BY id"}}}
			p, err := gpkg.NewTileProvider(conf, nil)
			if err != nil {
				t.Fatalf("registration: %v", err)
			}
			t.Cleanup(gpkg.Cleanup)
			bounds := geom.NewExtent([2]float64{0, 0}, [2]float64{10, 10})
			tile := &fixture.Tile{SRID: 3857, Bounds: bounds, BufferedBounds: bounds}
			count := 0
			err = p.TileFeatures(context.Background(), "short", tile, nil, func(f *provider.Feature) error { count++; return nil })
			if err != nil || count != 1 {
				t.Fatalf("render count=%d err=%v", count, err)
			}
		})
	}
}
