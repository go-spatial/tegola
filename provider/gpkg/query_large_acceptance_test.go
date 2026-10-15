//go:build cgo

package gpkg

import (
	"context"
	"database/sql"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/alexeydott/tegola/dict"
	"github.com/alexeydott/tegola/provider"
)

func TestFeatureQueryMillionRowBoundedAccess(t *testing.T) {
	started := time.Now()
	path := filepath.Join(t.TempDir(), "million.gpkg")
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		t.Fatal(err)
	}
	func() {
		defer func() {
			if err := db.Close(); err != nil {
				t.Error(err)
			}
		}()
		if _, err := db.Exec("CREATE TABLE items(id INTEGER PRIMARY KEY,geom TEXT);INSERT INTO items VALUES(1,'POINT (15 30)')"); err != nil {
			t.Fatal(err)
		}
		for size := 1; size < 1000000; size *= 2 {
			if _, err := db.Exec("INSERT INTO items SELECT id+?,NULL FROM items WHERE id+?<=1000000", size, size); err != nil {
				t.Fatal(err)
			}
		}
		if _, err := db.Exec("UPDATE items SET geom='POINT (not-a-number)' WHERE id=1000000"); err != nil {
			t.Fatal(err)
		}
		var count int
		if err := db.QueryRow("SELECT COUNT(*) FROM items").Scan(&count); err != nil || count != 1000000 {
			t.Fatalf("fixture count %d %v", count, err)
		}
	}()
	tiler, err := NewTileProvider(dict.Dict{"filepath": path, "layers": []map[string]any{{"name": "source", "tablename": "items", "id_fieldname": "id", "geometry_fieldname": "geom", "geometry_format": "wkt", "srid": 4326}}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	p := tiler.(*Provider)
	t.Cleanup(func() {
		if err := p.Close(); err != nil {
			t.Error(err)
		}
	})
	setup := time.Since(started)
	for _, tc := range []struct {
		name string
		q    provider.FeatureQuery
		want []uint64
	}{{"singleID", provider.FeatureQuery{Limit: 1, IDs: []uint64{1}}, []uint64{1}}, {"limit10", provider.FeatureQuery{Limit: 10}, []uint64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}}} {
		t.Run(tc.name, func(t *testing.T) {
			ids := []uint64{}
			begin := time.Now()
			result, err := p.QueryFeatures(context.Background(), "source", tc.q, func(f *provider.Feature) error { ids = append(ids, f.ID); return nil })
			if err != nil || !reflect.DeepEqual(ids, tc.want) {
				t.Fatalf("bounded query %v %v", ids, err)
			}
			if tc.name == "limit10" && !result.HasMore {
				t.Fatal("lookahead absent")
			}
			if result.NumberMatched != nil && tc.name == "limit10" {
				t.Fatal("unscanned exact count claimed")
			}
			t.Logf("fixture=1000000 setup=%s query=%s IDs=%v (timings informational; no threshold)", setup, time.Since(begin), ids)
			layer := p.layers["source"]
			where, args, impossible, err := featureCandidatePredicate(layer, tc.q)
			if err != nil || impossible {
				t.Fatalf("candidate %v %v", impossible, err)
			}
			statement := "SELECT l.id,l.geom FROM items l WHERE " + where + " ORDER BY l.id LIMIT 256"
			func() {
				rows, err := p.db.Query("EXPLAIN QUERY PLAN "+statement, args...)
				if err != nil {
					t.Fatal(err)
				}
				defer func() {
					if err := rows.Close(); err != nil {
						t.Error(err)
					}
				}()
				details := []string{}
				for rows.Next() {
					var id, parent, unused int
					var detail string
					if err := rows.Scan(&id, &parent, &unused, &detail); err != nil {
						t.Fatal(err)
					}
					details = append(details, detail)
				}
				if err := rows.Err(); err != nil {
					t.Fatal(err)
				}
				t.Logf("SQL %s; plan %v", statement, details)
				if tc.name == "singleID" && !strings.Contains(strings.Join(details, " "), "SEARCH l USING INTEGER PRIMARY KEY") {
					t.Fatal("ID lacks PK search", details)
				}
			}()
		})
	}
	// Selecting the far corrupt row must fail, proving it is not a tolerated or
	// silently discarded fixture. Successful small requests therefore did not
	// decode the whole million-row source. LIMIT10 still uses an ordered scan.
	_, err = p.QueryFeatures(context.Background(), "source", provider.FeatureQuery{Limit: 1, IDs: []uint64{1000000}}, func(*provider.Feature) error { return nil })
	if err == nil {
		t.Fatal("far malformed sentinel tolerated")
	}
}
