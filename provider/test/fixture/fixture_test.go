package fixture_test

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"reflect"
	"sync/atomic"
	"testing"

	"github.com/alexeydott/geom"
	"github.com/alexeydott/tegola/provider"
	"github.com/alexeydott/tegola/provider/test/fixture"
)

func TestConfig(t *testing.T) {
	existing := make([]map[string]any, 1, 2)
	existing[0] = map[string]any{"name": "base"}
	base := map[string]any{"name": "provider", "layers": existing}
	added := []map[string]any{{"name": "added"}}
	got := fixture.Config(base, map[string]any{"name": "override"}, added)
	want := map[string]any{"name": "override", "layers": []map[string]any{existing[0], added[0]}}
	if !reflect.DeepEqual(map[string]any(got), want) {
		t.Fatalf("Config = %#v, want %#v", got, want)
	}
	if base["name"] != "provider" || existing[:2][1] != nil {
		t.Fatal("Config mutated the base map or layer backing array")
	}
	override := []map[string]any{{"name": "replacement"}}
	got = fixture.Config(base, map[string]any{"layers": override}, added)
	if !reflect.DeepEqual(got["layers"], append(override, added...)) {
		t.Fatalf("layers override must precede append: %#v", got)
	}
	for _, value := range []any{nil, "invalid", []map[string]any{}} {
		got = fixture.Config(map[string]any{"layers": value}, nil, nil)
		if !reflect.DeepEqual(got["layers"], value) {
			t.Fatalf("no added layers must preserve %#v, got %#v", value, got)
		}
	}
	if got := fixture.Config(nil, nil, nil); got == nil || len(got) != 0 {
		t.Fatalf("nil inputs = %#v, want writable empty config", got)
	}
}

func TestTile(t *testing.T) {
	bounds := &geom.Extent{1, 2, 3, 4}
	buffered := &geom.Extent{0, 1, 4, 5}
	var tile provider.Tile = fixture.Tile{Z: 14, X: 8, Y: 5, SRID: 4326, Bounds: bounds, BufferedBounds: buffered}
	if z, x, y := tile.ZXY(); z != 14 || x != 8 || y != 5 {
		t.Fatalf("ZXY = %v/%v/%v", z, x, y)
	}
	if got, srid := tile.Extent(); got != bounds || srid != 4326 {
		t.Fatalf("Extent = %v, %v", got, srid)
	}
	if got, srid := tile.BufferedExtent(); got != buffered || srid != 4326 {
		t.Fatalf("BufferedExtent = %v, %v", got, srid)
	}
	zero := fixture.Tile{}
	if got, srid := zero.Extent(); got != nil || srid != 0 {
		t.Fatalf("zero extent = %v, %v", got, srid)
	}
	if got, _ := zero.BufferedExtent(); got != nil {
		t.Fatal("nil buffered extent must not fall back to the unbuffered extent")
	}
}

func TestDriverRows(t *testing.T) {
	input := [][]any{{1, int64(2), nil, "text", []byte{0xff}, 1.5, true}, {}}
	want := [][]driver.Value{{int64(1), int64(2), nil, "text", []byte{0xff}, 1.5, true}, {}}
	got := fixture.DriverRows(input)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("DriverRows = %#v, want %#v", got, want)
	}
	got[0][0] = int64(99)
	if input[0][0] != 1 {
		t.Fatal("DriverRows mutated source rows")
	}
}

func TestSQLRows(t *testing.T) {
	var db *sql.DB
	var closes int32
	var queries []string
	var contexts []context.Context
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	t.Run("scan and replay", func(t *testing.T) {
		db = fixture.OpenSQLRows(t, fixture.SQLRows{
			Columns: []string{"id", "geom"}, Rows: [][]driver.Value{{int64(7), []byte{1, 2}}, {int64(8), nil}},
			TypeNames: []string{"BIGINT", "BLOB"}, QueryLog: &queries, ContextLog: &contexts, Closes: &closes,
		})
		for _, query := range []string{"first query", "different query"} {
			rows, err := db.QueryContext(ctx, query)
			if err != nil {
				t.Fatal(err)
			}
			func() {
				defer func() {
					if err := rows.Close(); err != nil {
						t.Error(err)
					}
				}()
				columns, err := rows.Columns()
				if err != nil || !reflect.DeepEqual(columns, []string{"id", "geom"}) {
					t.Fatalf("columns = %v, %v", columns, err)
				}
				types, err := rows.ColumnTypes()
				if err != nil {
					t.Fatal(err)
				}
				if types[0].DatabaseTypeName() != "BIGINT" || types[1].DatabaseTypeName() != "BLOB" {
					t.Fatal("database type names lost")
				}
				count := 0
				for rows.Next() {
					var id int64
					var blob []byte
					if err := rows.Scan(&id, &blob); err != nil {
						t.Fatal(err)
					}
					if id != int64(7+count) || (count == 0 && !reflect.DeepEqual(blob, []byte{1, 2})) || (count == 1 && blob != nil) {
						t.Fatalf("row %d = %d, %v", count, id, blob)
					}
					count++
				}
				if err := rows.Err(); err != nil {
					t.Fatal(err)
				}
				if count != 2 {
					t.Fatalf("row count = %d, want 2", count)
				}
			}()
		}
		if stmt, err := db.Prepare("unsupported"); err == nil {
			defer func() {
				if err := stmt.Close(); err != nil {
					t.Error(err)
				}
			}()
			t.Fatal("Prepare must fail")
		}
		if _, err := db.Begin(); err == nil {
			t.Fatal("Begin must fail")
		}
	})
	if err := db.Ping(); err == nil {
		t.Fatal("database must be closed by test cleanup")
	}
	if atomic.LoadInt32(&closes) != 2 {
		t.Fatalf("rows closed %d times, want 2", closes)
	}
	if !reflect.DeepEqual(queries, []string{"first query", "different query"}) || len(contexts) != 2 || contexts[0] != ctx || contexts[1] != ctx {
		t.Fatalf("request recording: queries=%v contexts=%v", queries, contexts)
	}
}

func TestSQLRowsEmptyAndIsolated(t *testing.T) {
	empty := fixture.OpenSQLRows(t, fixture.SQLRows{Columns: []string{"id"}})
	full := fixture.OpenSQLRows(t, fixture.SQLRows{Columns: []string{"id"}, Rows: [][]driver.Value{{int64(1)}}})
	var id int64
	if err := empty.QueryRow("any").Scan(&id); err != sql.ErrNoRows {
		t.Fatalf("empty scan = %v, want ErrNoRows", err)
	}
	if err := full.QueryRow("any").Scan(&id); err != nil || id != 1 {
		t.Fatalf("independent scan = %d, %v", id, err)
	}
	rows, err := empty.Query("any")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := rows.Close(); err != nil {
			t.Error(err)
		}
	}()
	types, err := rows.ColumnTypes()
	if err != nil || len(types) != 1 || types[0].DatabaseTypeName() != "" {
		t.Fatalf("unconfigured types = %v, %v", types, err)
	}
	if rows.Next() {
		t.Fatal("empty result must not produce a row")
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
}
