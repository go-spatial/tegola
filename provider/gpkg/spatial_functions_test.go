//go:build cgo

package gpkg

import (
	"database/sql"
	"math"
	"testing"

	"github.com/alexeydott/geom"
	"github.com/alexeydott/geom/encoding/wkb"
)

func TestGeoPackageTriggerFunctions(t *testing.T) {
	db, err := sql.Open(featureSQLiteDriver, ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			t.Error(err)
		}
	}()
	raw, err := wkb.EncodeBytes(geom.LineString{{-4, 8}, {12, -2}})
	if err != nil {
		t.Fatal(err)
	}
	blob := encodeGPKGGeometry(raw, 4326, nil)
	var empty int
	var minx, maxx, miny, maxy float64
	err = db.QueryRow("SELECT ST_IsEmpty(?1),ST_MinX(?1),ST_MaxX(?1),ST_MinY(?1),ST_MaxY(?1)", blob).Scan(&empty, &minx, &maxx, &miny, &maxy)
	if err != nil {
		t.Fatal(err)
	}
	if empty != 0 || minx != -4 || maxx != 12 || miny != -2 || maxy != 8 {
		t.Fatalf("unexpected geometry result: empty=%d bounds=%v", empty, []float64{minx, maxx, miny, maxy})
	}
	var null sql.NullFloat64
	if err := db.QueryRow("SELECT ST_MinX(NULL)").Scan(&null); err != nil || null.Valid {
		t.Fatalf("NULL bounds=%v error=%v", null, err)
	}
	if err := db.QueryRow("SELECT ST_IsEmpty(?1)", gpkgHeader(0x11, 4326)).Scan(&empty); err != nil || empty != 1 {
		t.Fatalf("empty header=%d error=%v", empty, err)
	}
	if err := db.QueryRow("SELECT ST_MinX(?1)", []byte("broken")).Scan(&null); err == nil {
		t.Fatal("malformed geometry must fail instead of corrupting RTree")
	}
	if err := db.QueryRow("SELECT ST_IsEmpty(?1)", gpkgHeader(0x31, 4326)).Scan(&empty); err == nil {
		t.Fatal("extended geometry must fail instead of silently accepting unsupported storage")
	}
	nonfinite, err := wkb.EncodeBytes(geom.Point{math.Inf(1), 0})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow("SELECT ST_MinX(?1)", encodeGPKGGeometry(nonfinite, 4326, nil)).Scan(&null); err == nil {
		t.Fatal("non-finite coordinate must fail instead of corrupting RTree")
	}
}
