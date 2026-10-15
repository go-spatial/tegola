// Package fixture supplies provider test setup without registering providers
// or importing concrete provider implementations.
package fixture

import (
	"database/sql"
	"maps"
	"testing"

	"github.com/alexeydott/geom"
	"github.com/alexeydott/geom/slippy"
	"github.com/alexeydott/tegola/dict"
)

// Config shallow-copies base, applies overrides, and appends layers to the
// resulting "layers" entry. Nested maps remain shared with the caller.
func Config(base, overrides map[string]any, layers []map[string]any) dict.Dict {
	config := make(dict.Dict, len(base))
	maps.Copy(config, base)
	maps.Copy(config, overrides)
	if len(layers) > 0 {
		existing, _ := config["layers"].([]map[string]any)
		// Do not overwrite the base slice's spare capacity.
		combined := make([]map[string]any, 0, len(existing)+len(layers))
		combined = append(combined, existing...)
		config["layers"] = append(combined, layers...)
	}
	return config
}

// Tile implements provider.Tile with explicit, independent extents. Nil and
// non-finite bounds are passed through unchanged for error-path tests.
type Tile struct {
	Z              slippy.Zoom
	X, Y           uint
	SRID           uint64
	Bounds         *geom.Extent
	BufferedBounds *geom.Extent
}

func (t Tile) ZXY() (slippy.Zoom, uint, uint)         { return t.Z, t.X, t.Y }
func (t Tile) Extent() (*geom.Extent, uint64)         { return t.Bounds, t.SRID }
func (t Tile) BufferedExtent() (*geom.Extent, uint64) { return t.BufferedBounds, t.SRID }

// OpenDB opens an already-registered driver, executes fixture DDL in order,
// and closes the database at test cleanup. It does not select a driver,
// create a temporary directory, or gate live database tests.
func OpenDB(t testing.TB, driverName, dsn string, statements ...string) *sql.DB {
	t.Helper()
	db, err := sql.Open(driverName, dsn)
	if err != nil {
		t.Fatalf("open fixture database: %v", err)
	}
	closeDB(t, db)
	for _, statement := range statements {
		if _, err := db.Exec(statement); err != nil {
			t.Fatalf("exec fixture %q: %v", statement, err)
		}
	}
	return db
}

func closeDB(t testing.TB, db *sql.DB) {
	t.Helper()
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Errorf("close fixture database: %v", err)
		}
	})
}
