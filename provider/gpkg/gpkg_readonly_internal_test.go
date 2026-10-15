//go:build cgo

package gpkg

import (
	"database/sql"
	"path/filepath"
	"testing"
)

// P6-16: databases opened through sqliteReadOnlyDSN must be read-only —
// writes have to be rejected by SQLite, while reads keep working.
func TestReadOnlyOpenRejectsWrites(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ro.gpkg")

	// build the fixture with a plain (writable) connection
	wdb, err := sql.Open("sqlite3", path)
	if err != nil {
		t.Fatalf("open writable: %v", err)
	}
	for _, stmt := range []string{
		`CREATE TABLE t (id INTEGER)`,
		`INSERT INTO t VALUES (1)`,
	} {
		if _, err := wdb.Exec(stmt); err != nil {
			t.Fatalf("fixture %q: %v", stmt, err)
		}
	}
	if err := wdb.Close(); err != nil {
		t.Fatalf("close writable: %v", err)
	}

	db, err := sql.Open("sqlite3", sqliteReadOnlyDSN(path))
	if err != nil {
		t.Fatalf("open read-only: %v", err)
	}
	defer func() { _ = db.Close() }()

	var n int
	if err := db.QueryRow("SELECT count(*) FROM t").Scan(&n); err != nil {
		t.Fatalf("read through read-only DSN: %v", err)
	}
	if n != 1 {
		t.Fatalf("read through read-only DSN: count = %d, want 1", n)
	}

	if _, err := db.Exec("INSERT INTO t VALUES (2)"); err == nil {
		t.Fatalf("write through read-only DSN must fail (P6-16)")
	}
}
