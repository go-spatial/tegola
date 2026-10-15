//go:build cgo

package fixture_test

import (
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/alexeydott/tegola/provider/test/fixture"
	_ "github.com/mattn/go-sqlite3"
)

func TestOpenDB(t *testing.T) {
	var db *sql.DB
	t.Run("ordered setup", func(t *testing.T) {
		db = fixture.OpenDB(t, "sqlite3", filepath.Join(t.TempDir(), "fixture.gpkg"),
			"CREATE TABLE items (id INTEGER)", "INSERT INTO items VALUES (7)")
		var id int
		if err := db.QueryRow("SELECT id FROM items").Scan(&id); err != nil || id != 7 {
			t.Fatalf("fixture row = %d, %v", id, err)
		}
	})
	if err := db.Ping(); err == nil {
		t.Fatal("database must close before temporary directory cleanup")
	}
}
