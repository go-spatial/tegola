//go:build cgo

package gpkg

import (
	"github.com/alexeydott/tegola/provider"
	codec "github.com/alexeydott/tegola/provider/geometrycodec"
	"github.com/alexeydott/tegola/provider/test/fixture"
	_ "github.com/mattn/go-sqlite3"
	"testing"
)

func TestSQLiteDialectExecutesBackslashAndProbe(t *testing.T) {
	db := fixture.OpenDB(t, "sqlite3", ":memory:")
	tile := provider.NewTile(3, 0, 0, 0, 3857)
	sql, err := replaceTokens(`SELECT '\', !ZOOM!`, &Layer{}, tile, nil)
	if err != nil {
		t.Fatal(err)
	}
	var slash string
	var z int
	if err := db.QueryRow(sql).Scan(&slash, &z); err != nil || slash != `\` || z != 3 {
		t.Fatalf("SQL %s: slash=%q zoom=%d err=%v", sql, slash, z, err)
	}
	probe := codec.WrapProbeSQL(codec.SQLite.PrepareProbeSQL(`SELECT '\', !X! -- trailing comment`, "geom", "id", ""))
	if err := db.QueryRow(probe).Scan(&slash, &z); err != nil || slash != `\` || z != 0 {
		t.Fatalf("probe %s: slash=%q x=%d err=%v", probe, slash, z, err)
	}
}
