//go:build cgo

package gpkg_test

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/alexeydott/geom"
	"github.com/alexeydott/tegola/mos"
	"path/filepath"
	"testing"

	"github.com/alexeydott/tegola/dict"
	pa "github.com/alexeydott/tegola/provider/audit"
	"github.com/alexeydott/tegola/provider/gpkg"
	"github.com/alexeydott/tegola/provider/internal/querytest"
)

func TestNativeMOSMutationMatrix(t *testing.T) {
	t.Run("plain", func(t *testing.T) { runNativeMOSMutationMatrix(t, false) })
	t.Run("bounds", func(t *testing.T) { runNativeMOSMutationMatrix(t, true) })
}
func runNativeMOSMutationMatrix(t *testing.T, withBounds bool) {
	querytest.RunMOSMutations(t, func(t *testing.T, kind string) querytest.MOSMutationInstance {
		path := filepath.Join(t.TempDir(), "mos.sqlite")
		db, err := sql.Open("sqlite3", path)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := db.Close(); err != nil {
				t.Error(err)
			}
		})
		ddl := "CREATE TABLE items(id INTEGER PRIMARY KEY,geom BLOB,name TEXT,value INTEGER"
		if withBounds {
			ddl += ",MINX INTEGER,MAXX INTEGER,MINY INTEGER,MAXY INTEGER"
		}
		if _, err := db.Exec(ddl + ")"); err != nil {
			t.Fatal(err)
		}
		if err := pa.Migrate(context.Background(), db, "sqlite"); err != nil {
			t.Fatal(err)
		}
		tiler, err := gpkg.NewTileProvider(dict.Dict{"filepath": path, "layers": []map[string]interface{}{{
			"name": "items", "tablename": "items", "id_fieldname": "id", "geometry_fieldname": "geom",
			"geometry_format": "mos", "geometry_type": kind, "srid": 3857, "mos_precision": 2, "mos_units": "m", "fields": []string{"name", "value"},
		}}}, nil)
		if err != nil {
			t.Fatal(err)
		}
		p := tiler.(*gpkg.Provider)
		t.Cleanup(func() {
			if err := p.Close(); err != nil {
				t.Error(err)
			}
		})
		if _, err := p.DescribeWritable(context.Background(), "items"); err != nil {
			t.Fatal(err)
		}
		return querytest.MOSMutationInstance{Writer: p, Querier: p, Raw: func(id uint64) []byte {
			t.Helper()
			var raw []byte
			if err := db.QueryRow("SELECT geom FROM items WHERE id=?", id).Scan(&raw); err != nil {
				t.Fatal(err)
			}
			if withBounds && raw != nil {
				g, err := mos.Decode(raw, mos.Options{UnitFactor: 1})
				if err != nil {
					t.Fatal(err)
				}
				points, err := geom.GetCoordinates(g)
				if err != nil || len(points) == 0 {
					t.Fatalf("coordinates: %v", err)
				}
				want := [4]float64{points[0][0], points[0][0], points[0][1], points[0][1]}
				for _, pt := range points {
					want[0] = min(want[0], pt[0])
					want[1] = max(want[1], pt[0])
					want[2] = min(want[2], pt[1])
					want[3] = max(want[3], pt[1])
				}
				var got [4]float64
				if err := db.QueryRow("SELECT MINX,MAXX,MINY,MAXY FROM items WHERE id=?", id).Scan(&got[0], &got[1], &got[2], &got[3]); err != nil {
					t.Fatal(err)
				}
				if got != want {
					t.Fatalf("native bbox=%v want=%v", got, want)
				}
			}
			return raw
		}, SetRaw: func(id uint64, raw []byte) {
			t.Helper()
			if _, err := db.Exec("UPDATE items SET geom=? WHERE id=?", raw, id); err != nil {
				t.Fatal(err)
			}
		}}
	})
}

func TestNativeMOSBoundsProfile(t *testing.T) {
	for _, custom := range []bool{false, true} {
		for _, units := range []string{"m", "cm"} {
			t.Run(fmt.Sprintf("custom=%v/units=%s", custom, units), func(t *testing.T) {
				path := filepath.Join(t.TempDir(), "bounds.sqlite")
				db, err := sql.Open("sqlite3", path)
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() {
					if err := db.Close(); err != nil {
						t.Error(err)
					}
				})
				if _, err := db.Exec("CREATE TABLE items(id INTEGER PRIMARY KEY,geom BLOB,name TEXT,MINX INTEGER,MAXX INTEGER,MINY INTEGER,MAXY INTEGER)"); err != nil {
					t.Fatal(err)
				}
				for index, pt := range [][2]int32{{123, 235}, {100000, 200000}} {
					if _, err := db.Exec("INSERT INTO items VALUES(?,?,?,?,?,?,?)", index+1, querytest.NativeMOSPoint(pt[0], pt[1]), "source", pt[0], pt[0], pt[1], pt[1]); err != nil {
						t.Fatal(err)
					}
				}
				precision := 2
				if units == "cm" {
					precision = 0
				}
				layer := map[string]interface{}{"name": "items", "tablename": "items", "id_fieldname": "id", "geometry_fieldname": "geom", "geometry_format": "mos", "geometry_type": "point", "srid": 3857, "mos_precision": precision, "mos_units": units, "fields": []string{"name"}}
				if custom {
					delete(layer, "tablename")
					layer["sql"] = "SELECT id,geom,name FROM items WHERE !BBOX!"
				}
				tiler, err := gpkg.NewTileProvider(dict.Dict{"filepath": path, "layers": []map[string]interface{}{layer}}, nil)
				if err != nil {
					t.Fatal(err)
				}
				p := tiler.(*gpkg.Provider)
				t.Cleanup(func() {
					if err := p.Close(); err != nil {
						t.Error(err)
					}
				})
				querytest.AssertMOSBoundsProfile(t, p, p, p, custom, true)
			})
		}
	}
}
