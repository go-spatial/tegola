//go:build cgo

// gpkg_contract_test.go pins the bounds-backed custom-SQL registration
// contract matrix (audit part10 A02/A03/A05/A06/A08/A09/A11 and the part10
// section-4 regression matrix rows) for the gpkg provider, the only one of
// the four providers whose registration path runs fully offline. The A01
// probe rows are covered by the identical mosfixture contract tests in all
// four providers.
package gpkg_test

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"

	"github.com/alexeydott/geom"
	"github.com/alexeydott/tegola/dict"
	"github.com/alexeydott/tegola/provider"
	"github.com/alexeydott/tegola/provider/gpkg"
	"github.com/alexeydott/tegola/provider/test/fixture"
)

const (
	contractDDLUpper = "CREATE TABLE mos_layer (id INTEGER PRIMARY KEY AUTOINCREMENT, geom BLOB, MINX DOUBLE, MAXX DOUBLE, MINY DOUBLE, MAXY DOUBLE)"
	contractDDLLower = "CREATE TABLE mos_layer (id INTEGER PRIMARY KEY AUTOINCREMENT, geom BLOB, minx DOUBLE, maxx DOUBLE, miny DOUBLE, maxy DOUBLE)"
	contractSQLUpper = "SELECT id, geom, MINX, MAXX, MINY, MAXY FROM mos_layer WHERE !BBOX!"
	contractSQLLower = "SELECT id, geom, minx, maxx, miny, maxy FROM mos_layer WHERE !BBOX!"
)

// contractRows are three decodable MOS rows (>= MinValidMOSRows) with
// consistent bounds columns (MOS default precision 2: 100 -> 1.0).
func contractRows() [][]interface{} {
	return [][]interface{}{
		{mosPolylineBlob([][2]int32{{100, 100}, {300, 300}}), 1.0, 3.0, 1.0, 3.0},
		{mosPolylineBlob([][2]int32{{100, 300}, {300, 300}}), 1.0, 3.0, 1.0, 3.0},
		{mosPolylineBlob([][2]int32{{300, 100}, {300, 300}}), 1.0, 3.0, 1.0, 3.0},
	}
}

func contractConf(path string, layers ...map[string]interface{}) dict.Dict {
	return dict.Dict{"filepath": path, "layers": layers}
}

// contractFeatures runs the tile query over the fixture extent that catches
// the decoded rows (1.0..3.0 at the paired default precision 2).
func contractFeatures(t *testing.T, p provider.Tiler, name string) int {
	t.Helper()
	tile := fixture.Tile{
		SRID:           3857,
		BufferedBounds: geom.NewExtent([2]float64{-10, -10}, [2]float64{10, 10}),
	}
	var count int
	if err := p.TileFeatures(context.TODO(), name, &tile, nil, func(f *provider.Feature) error {
		count++
		return nil
	}); err != nil {
		t.Fatalf("TileFeatures: %v", err)
	}
	return count
}

func TestMOSCustomSQLRegistrationContract(t *testing.T) {
	t.Run("auto format switches to mos on a valid MOS sample", func(t *testing.T) {
		// regression matrix row 1: "MOS SQL + bounds + 3 valid rows + auto
		// type -> SQL-sample detected" (A04/A05 positive half).
		fx := newRawFixture(t, []string{contractDDLUpper})
		insertRows(t, fx.path, "mos_layer", []string{"geom", "MINX", "MAXX", "MINY", "MAXY"}, contractRows())
		conf := contractConf(fx.path, map[string]interface{}{
			"name": "raw_layer",
			"sql":  contractSQLUpper,
			"srid": 3857,
		})
		p, err := gpkg.NewTileProvider(conf, nil)
		if err != nil {
			t.Fatalf("NewTileProvider: %v", err)
		}
		t.Cleanup(gpkg.Cleanup)
		if count := contractFeatures(t, p, "raw_layer"); count != 3 {
			t.Errorf("feature count = %v, want 3 (auto format must detect the MOS sample, switch to mos and decode it)", count)
		}
	})

	t.Run("auto format without bbox token errors after MOS detection", func(t *testing.T) {
		// A05: the format must resolve first and the !BBOX! requirement must
		// then hold for the effective format (mos).
		fx := newRawFixture(t, []string{contractDDLUpper})
		insertRows(t, fx.path, "mos_layer", []string{"geom", "MINX", "MAXX", "MINY", "MAXY"}, contractRows())
		conf := contractConf(fx.path, map[string]interface{}{
			"name": "raw_layer",
			"sql":  "SELECT id, geom, MINX, MAXX, MINY, MAXY FROM mos_layer",
			"srid": 3857,
		})
		_, err := gpkg.NewTileProvider(conf, nil)
		if err == nil {
			t.Fatal("expected startup error: auto->MOS custom SQL without !BBOX! must fail the post-resolution contract")
		}
		if !strings.Contains(err.Error(), "BBOX") {
			t.Errorf("error must name the missing token, got: %v", err)
		}
	})

	t.Run("missing bounds column warns and registers", func(t *testing.T) {
		// regression matrix row 2 (b5ad0979 regression fix): the !BBOX!
		// predicate resolves the bounds columns in the query's own scope
		// (the table's columns), so bounds columns missing from the SELECT
		// list are a registration warning, never a startup error - and the
		// layer must still register and serve.
		fx := newRawFixture(t, []string{contractDDLUpper})
		insertRows(t, fx.path, "mos_layer", []string{"geom", "MINX", "MAXX", "MINY", "MAXY"}, contractRows())
		conf := contractConf(fx.path, map[string]interface{}{
			"name":            "raw_layer",
			"sql":             "SELECT id, geom, MINX, MAXX, MINY FROM mos_layer WHERE !BBOX!",
			"geometry_format": "mos",
			"srid":            3857,
		})
		var buf bytes.Buffer
		prev := slog.Default()
		slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
		defer slog.SetDefault(prev)
		p, err := gpkg.NewTileProvider(conf, nil)
		if err != nil {
			t.Fatalf("NewTileProvider: missing bounds column in the SELECT list must warn, not fail: %v", err)
		}
		t.Cleanup(gpkg.Cleanup)
		if !strings.Contains(buf.String(), "bounds columns") {
			t.Errorf("registration must warn about the missing bounds result columns, got log: %q", buf.String())
		}
		if count := contractFeatures(t, p, "raw_layer"); count != 3 {
			t.Errorf("feature count = %v, want 3 (the !BBOX! predicate must resolve against the table's bounds columns)", count)
		}
	})

	t.Run("missing bbox token errors", func(t *testing.T) {
		// regression matrix row 3: "missing BBOX -> startup error".
		fx := newRawFixture(t, []string{contractDDLUpper})
		insertRows(t, fx.path, "mos_layer", []string{"geom", "MINX", "MAXX", "MINY", "MAXY"}, contractRows())
		conf := contractConf(fx.path, map[string]interface{}{
			"name":            "raw_layer",
			"sql":             "SELECT id, geom, MINX, MAXX, MINY, MAXY FROM mos_layer",
			"geometry_format": "mos",
			"srid":            3857,
		})
		_, err := gpkg.NewTileProvider(conf, nil)
		if err == nil {
			t.Fatal("expected startup error: MOS custom SQL without !BBOX! must fail")
		}
		if !strings.Contains(err.Error(), "BBOX") {
			t.Errorf("error must name the missing token, got: %v", err)
		}
	})

	t.Run("explicit geometry type with missing bound warns and registers", func(t *testing.T) {
		// A03: an explicit geometry_type must never skip the structural
		// validation - the probe still runs and warns; the layer registers.
		fx := newRawFixture(t, []string{contractDDLUpper})
		insertRows(t, fx.path, "mos_layer", []string{"geom", "MINX", "MAXX", "MINY", "MAXY"}, contractRows())
		conf := contractConf(fx.path, map[string]interface{}{
			"name":            "raw_layer",
			"sql":             "SELECT id, geom, MINX, MAXX, MINY FROM mos_layer WHERE !BBOX!",
			"geometry_format": "mos",
			"geometry_type":   "LineString",
			"srid":            3857,
		})
		var buf bytes.Buffer
		prev := slog.Default()
		slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
		defer slog.SetDefault(prev)
		p, err := gpkg.NewTileProvider(conf, nil)
		if err != nil {
			t.Fatalf("NewTileProvider: explicit geometry_type must not turn the bounds warning into an error: %v", err)
		}
		t.Cleanup(gpkg.Cleanup)
		if !strings.Contains(buf.String(), "bounds columns") {
			t.Errorf("the structural probe must still run and warn under an explicit geometry_type, got log: %q", buf.String())
		}
		if count := contractFeatures(t, p, "raw_layer"); count != 3 {
			t.Errorf("feature count = %v, want 3", count)
		}
	})

	t.Run("explicit geometry type with empty data registers", func(t *testing.T) {
		// A03: explicit types skip only inference and the >=3-sample-row
		// requirement; empty result sets are allowed.
		fx := newRawFixture(t, []string{contractDDLUpper})
		conf := contractConf(fx.path, map[string]interface{}{
			"name":            "raw_layer",
			"sql":             contractSQLUpper,
			"geometry_format": "mos",
			"geometry_type":   "LineString",
			"srid":            3857,
		})
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
			t.Errorf("layer count = %v, want 1", len(lyrs))
		}
	})

	t.Run("lower-case bounds columns work end to end", func(t *testing.T) {
		// regression matrix row 6: lower-case bounds with uppercase default
		// config -> the actual result-column names are persisted (A09) and
		// the runtime predicate filters correctly.
		fx := newRawFixture(t, []string{contractDDLLower})
		insertRows(t, fx.path, "mos_layer", []string{"geom", "minx", "maxx", "miny", "maxy"}, contractRows())
		conf := contractConf(fx.path, map[string]interface{}{
			"name":            "raw_layer",
			"sql":             contractSQLLower,
			"geometry_format": "mos",
			"srid":            3857,
		})
		p, err := gpkg.NewTileProvider(conf, nil)
		if err != nil {
			t.Fatalf("NewTileProvider: %v", err)
		}
		t.Cleanup(gpkg.Cleanup)
		if count := contractFeatures(t, p, "raw_layer"); count != 3 {
			t.Errorf("feature count = %v, want 3 (bounds predicate must quote the actual result-column names)", count)
		}
	})

	t.Run("tile dependent sql with missing bound warns and registers", func(t *testing.T) {
		// regression matrix row 7 / A08: tile-dependent SQL still runs the
		// structural result-column check at registration - it warns and the
		// layer registers; it never skips the probe and never fails.
		fx := newRawFixture(t, []string{contractDDLUpper})
		insertRows(t, fx.path, "mos_layer", []string{"geom", "MINX", "MAXX", "MINY", "MAXY"}, contractRows())
		conf := contractConf(fx.path, map[string]interface{}{
			"name":            "raw_layer",
			"sql":             "SELECT id, geom, MINX, MINY, MAXY FROM mos_layer WHERE !ZOOM! >= 0 AND !X! >= 0 AND !Y! >= 0 AND !BBOX!",
			"geometry_format": "mos",
			"srid":            3857,
		})
		var buf bytes.Buffer
		prev := slog.Default()
		slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
		defer slog.SetDefault(prev)
		p, err := gpkg.NewTileProvider(conf, nil)
		if err != nil {
			t.Fatalf("NewTileProvider: tile-dependent custom SQL must warn about missing bounds columns, not fail: %v", err)
		}
		t.Cleanup(gpkg.Cleanup)
		if !strings.Contains(buf.String(), "bounds columns") {
			t.Errorf("tile-dependent custom SQL must still run the structural probe and warn, got log: %q", buf.String())
		}
		if count := contractFeatures(t, p, "raw_layer"); count != 3 {
			t.Errorf("feature count = %v, want 3", count)
		}
	})

	t.Run("gpkg format custom sql requires bbox token", func(t *testing.T) {
		// A06: custom geometry_format=gpkg carries the same structural
		// contract as mos.
		fx := newRawFixture(t, []string{contractDDLUpper})
		insertRows(t, fx.path, "mos_layer", []string{"geom", "MINX", "MAXX", "MINY", "MAXY"}, contractRows())
		conf := contractConf(fx.path, map[string]interface{}{
			"name":            "raw_layer",
			"sql":             "SELECT id, geom, MINX, MAXX, MINY, MAXY FROM mos_layer",
			"geometry_format": "gpkg",
			"geometry_type":   "LineString",
			"srid":            3857,
		})
		_, err := gpkg.NewTileProvider(conf, nil)
		if err == nil {
			t.Fatal("expected startup error: gpkg custom SQL without !BBOX! must fail")
		}
		if !strings.Contains(err.Error(), "BBOX") {
			t.Errorf("error must name the missing token, got: %v", err)
		}
	})

	t.Run("gpkg format custom sql with missing bounds column warns and registers", func(t *testing.T) {
		// A06: geometry_format=gpkg carries the same contract as mos -
		// missing bounds result columns warn and register (the predicate
		// resolves in the table's scope).
		fx := newRawFixture(t, []string{contractDDLUpper})
		insertRows(t, fx.path, "mos_layer", []string{"geom", "MINX", "MAXX", "MINY", "MAXY"}, [][]interface{}{
			{gpkgPointBlob(t, 3857, 1, 1), 1.0, 3.0, 1.0, 3.0},
			{gpkgPointBlob(t, 3857, 2, 2), 1.0, 3.0, 1.0, 3.0},
			{gpkgPointBlob(t, 3857, 3, 3), 1.0, 3.0, 1.0, 3.0},
		})
		conf := contractConf(fx.path, map[string]interface{}{
			"name":            "raw_layer",
			"sql":             "SELECT id, geom, MINX, MAXX, MINY FROM mos_layer WHERE !BBOX!",
			"geometry_format": "gpkg",
			"geometry_type":   "Point",
			"srid":            3857,
		})
		var buf bytes.Buffer
		prev := slog.Default()
		slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
		defer slog.SetDefault(prev)
		p, err := gpkg.NewTileProvider(conf, nil)
		if err != nil {
			t.Fatalf("NewTileProvider: gpkg custom SQL with a missing bounds column must warn, not fail: %v", err)
		}
		t.Cleanup(gpkg.Cleanup)
		if !strings.Contains(buf.String(), "bounds columns") {
			t.Errorf("registration must warn about the missing bounds result columns, got log: %q", buf.String())
		}
		if count := contractFeatures(t, p, "raw_layer"); count != 3 {
			t.Errorf("feature count = %v, want 3", count)
		}
	})

	t.Run("gpkg format valid custom sql registers", func(t *testing.T) {
		// A06 positive half (predicate mode BoundsSourceCRS is pinned by
		// TestBuildBBoxPredicateContract).
		fx := newRawFixture(t, []string{contractDDLUpper})
		insertRows(t, fx.path, "mos_layer", []string{"geom", "MINX", "MAXX", "MINY", "MAXY"}, [][]interface{}{
			{wkbGeomBytes(t, geom.Point{1, 1}), 1.0, 3.0, 1.0, 3.0},
			{wkbGeomBytes(t, geom.Point{2, 2}), 1.0, 3.0, 1.0, 3.0},
			{wkbGeomBytes(t, geom.Point{3, 3}), 1.0, 3.0, 1.0, 3.0},
		})
		conf := contractConf(fx.path, map[string]interface{}{
			"name":            "raw_layer",
			"sql":             contractSQLUpper,
			"geometry_format": "gpkg",
			"geometry_type":   "LineString",
			"srid":            3857,
		})
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
			t.Errorf("layer count = %v, want 1", len(lyrs))
		}
	})

	t.Run("mos custom sql without explicit CRS errors", func(t *testing.T) {
		// A11: MOS custom SQL requires srid or crs_defn.
		fx := newRawFixture(t, []string{contractDDLUpper})
		insertRows(t, fx.path, "mos_layer", []string{"geom", "MINX", "MAXX", "MINY", "MAXY"}, contractRows())
		conf := contractConf(fx.path, map[string]interface{}{
			"name":            "raw_layer",
			"sql":             contractSQLUpper,
			"geometry_format": "mos",
		})
		_, err := gpkg.NewTileProvider(conf, nil)
		if err == nil {
			t.Fatal("expected startup error: MOS custom SQL without srid and crs_defn must fail")
		}
		if !strings.Contains(err.Error(), "srid or crs_defn") {
			t.Errorf("error must point at the explicit CRS configuration, got: %v", err)
		}
	})

	t.Run("mos custom sql without mos precision uses paired defaults", func(t *testing.T) {
		// A11: mos_precision/mos_units are optional; the paired defaults
		// (precision 2) decode the fixture rows into the tile extent.
		fx := newRawFixture(t, []string{contractDDLUpper})
		insertRows(t, fx.path, "mos_layer", []string{"geom", "MINX", "MAXX", "MINY", "MAXY"}, contractRows())
		conf := contractConf(fx.path, map[string]interface{}{
			"name":            "raw_layer",
			"sql":             contractSQLUpper,
			"geometry_format": "mos",
			"srid":            3857,
		})
		p, err := gpkg.NewTileProvider(conf, nil)
		if err != nil {
			t.Fatalf("NewTileProvider: %v", err)
		}
		t.Cleanup(gpkg.Cleanup)
		if count := contractFeatures(t, p, "raw_layer"); count != 3 {
			t.Errorf("feature count = %v, want 3 (paired MOS defaults must decode the sample rows)", count)
		}
	})
}
