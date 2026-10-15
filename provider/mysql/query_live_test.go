package mysql

import (
	"context"
	"database/sql"
	"errors"
	"math"
	"net"
	"os"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alexeydott/geom"
	"github.com/alexeydott/tegola/dict"
	"github.com/alexeydott/tegola/provider"
	mysqlDriver "github.com/go-sql-driver/mysql"
)

var featureLiveSequence atomic.Uint64

func featureLiveDatabase(t *testing.T, flavor string) (*sql.DB, dict.Dict) {
	t.Helper()
	gate, key := "RUN_MYSQL_TESTS", "MYSQL_FEATURE_TEST_DSN"
	if flavor == GeometryFormatMariaDB {
		gate, key = "RUN_MARIADB_TESTS", "MARIADB_FEATURE_TEST_DSN"
	}
	if os.Getenv(gate) != "yes" {
		t.Skip("live feature backend disabled; set " + gate + "=yes and " + key)
	}
	dsn := os.Getenv(key)
	if dsn == "" {
		t.Fatal("live feature tests require explicit DSN; no catalog privileges are granted by tests")
	}
	connection, err := mysqlDriver.ParseDSN(dsn)
	if err != nil {
		t.Fatal("invalid live feature DSN")
	}
	if connection.Net != "tcp" {
		t.Fatal("live constructor fixture requires TCP DSN")
	}
	host, portText, err := net.SplitHostPort(connection.Addr)
	if err != nil {
		t.Fatal("invalid live TCP address")
	}
	port, err := strconv.Atoi(portText)
	if err != nil {
		t.Fatal("invalid live TCP port")
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Error(err)
		}
	})
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		t.Fatal(err)
	}
	actual, err := detectServerFlavor(db)
	if err != nil || actual != flavor {
		t.Fatalf("live server flavor mismatch: %s %v", actual, err)
	}
	return db, dict.Dict{"host": host, "port": port, "database": connection.DBName,
		"user": connection.User, "password": connection.Passwd, "tls": connection.TLSConfig, "timeout": "10s"}
}

func featureLiveTable(t *testing.T, db *sql.DB, columns string) string {
	t.Helper()
	name := "tegola_feature_" + strconv.FormatInt(time.Now().UnixNano(), 10) + "_" + strconv.FormatUint(featureLiveSequence.Add(1), 10)
	if _, err := db.Exec("CREATE TABLE " + featureQuoteIdentifier(name) + " (" + columns + ") ENGINE=InnoDB"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if _, err := db.Exec("DROP TABLE IF EXISTS " + featureQuoteIdentifier(name)); err != nil {
			t.Error(err)
		}
	})
	return name
}

func featureLiveProvider(t *testing.T, connection dict.Dict, layer map[string]any) *Provider {
	t.Helper()
	config := dict.Dict{}
	for key, value := range connection {
		config[key] = value
	}
	config["layers"] = []map[string]any{layer}
	tiler, err := NewTileProvider(config, nil)
	if err != nil {
		t.Fatal(err)
	}
	p := tiler.(*Provider)
	t.Cleanup(func() {
		if err := p.Close(); err != nil {
			t.Error(err)
		}
	})
	if err := p.layers["items"].FeatureQuerySupported(); err != nil {
		t.Fatalf("raw profile not eligible (requires physical catalog visibility): %v", err)
	}
	return p
}

func TestFeatureLiveUnsignedPagingAndCustomDomain(t *testing.T) {
	for _, flavor := range []string{GeometryFormatMySQL, GeometryFormatMariaDB} {
		t.Run(flavor, func(t *testing.T) {
			db, config := featureLiveDatabase(t, flavor)
			table := featureLiveTable(t, db, "id BIGINT UNSIGNED UNIQUE, geom TEXT, name VARCHAR(40) COLLATE utf8mb4_bin, active INT, start_time BIGINT, end_time BIGINT")
			for _, row := range [][]any{{uint64(1), "POINT(50 50)", "outside", 1, int64(0), int64(10)},
				{uint64(2), "POINT(1 1)", "first", 1, int64(0), int64(10)},
				{uint64(3), "POINT(1 1)", "excluded-domain", 0, int64(0), int64(10)},
				{uint64(math.MaxUint64), "POINT(1 1)", "last", 1, nil, nil},
				{nil, "POINT(1 1)", "no-id", 1, nil, nil}} {
				if _, err := db.Exec("INSERT INTO "+featureQuoteIdentifier(table)+" VALUES (?,?,?,?,?,?)", row...); err != nil {
					t.Fatal(err)
				}
			}
			for _, custom := range []bool{false, true} {
				t.Run(strconv.FormatBool(custom), func(t *testing.T) {
					layer := map[string]any{"name": "items", "tablename": table, "id_fieldname": "id", "geometry_fieldname": "geom", "geometry_format": "wkt", "geometry_type": "point", "srid": 4326,
						"temporal_start_field": "start_time", "temporal_end_field": "end_time", "temporal_storage": "unix_nanoseconds"}
					if custom {
						layer["feature_sql"] = "SELECT id,geom,name,active,start_time,end_time FROM " + featureQuoteIdentifier(table) + " WHERE active=1"
					}
					p := featureLiveProvider(t, config, layer)
					query := provider.FeatureQuery{Limit: 1, Offset: 1, BoundsSRID: 4326, Bounds: []geom.Extent{{0, 0, 2, 2}}}
					ids := []uint64{}
					_, err := p.QueryFeatures(context.Background(), "items", query, func(f *provider.Feature) error { ids = append(ids, f.ID); return nil })
					want := []uint64{3}
					if custom {
						want = []uint64{math.MaxUint64}
					}
					if err != nil || !reflect.DeepEqual(ids, want) {
						t.Fatalf("exact paging/domain: %v %v", ids, err)
					}
					query = provider.FeatureQuery{Limit: 10, IDs: []uint64{math.MaxUint64}}
					ids = []uint64{}
					_, err = p.QueryFeatures(context.Background(), "items", query, func(f *provider.Feature) error { ids = append(ids, f.ID); return nil })
					if err != nil || !reflect.DeepEqual(ids, []uint64{math.MaxUint64}) {
						t.Fatalf("unsigned native binding: %v %v", ids, err)
					}
				})
			}
		})
	}
}

func TestFeatureLiveNativeXYAndWrongSRIDIntegrity(t *testing.T) {
	for _, flavor := range []string{GeometryFormatMySQL, GeometryFormatMariaDB} {
		t.Run(flavor, func(t *testing.T) {
			db, config := featureLiveDatabase(t, flavor)
			table := featureLiveTable(t, db, "id BIGINT PRIMARY KEY, geom GEOMETRY, name VARCHAR(40)")
			option := ""
			if flavor == GeometryFormatMySQL {
				option = ",'axis-order=long-lat'"
			}
			if _, err := db.Exec("INSERT INTO "+featureQuoteIdentifier(table)+" VALUES (?,ST_GeomFromText(?,4326"+option+"),?)", int64(1), "POINT(12 55)", "known"); err != nil {
				t.Fatal(err)
			}
			layer := map[string]any{"name": "items", "tablename": table, "id_fieldname": "id", "geometry_fieldname": "geom", "geometry_format": flavor, "geometry_type": "point", "srid": 4326}
			p := featureLiveProvider(t, config, layer)
			var got geom.Geometry
			_, err := p.QueryFeatures(context.Background(), "items", provider.FeatureQuery{Limit: 10}, func(f *provider.Feature) error { got = f.Geometry; return nil })
			if err != nil || !reflect.DeepEqual(got, geom.Point{12, 55}) {
				t.Fatalf("native long-lat export: %#v %v", got, err)
			}
			if _, err := db.Exec("UPDATE "+featureQuoteIdentifier(table)+" SET geom=ST_GeomFromText(?,3857) WHERE id=?", "POINT(12 55)", int64(1)); err != nil {
				t.Fatal(err)
			}
			_, err = p.QueryFeatures(context.Background(), "items", provider.FeatureQuery{Limit: 10}, func(*provider.Feature) error { t.Fatal("wrong-SRID row emitted"); return nil })
			var data provider.FeatureDataError
			if !errors.As(err, &data) {
				t.Fatalf("native source-integrity error: %v", err)
			}
		})
	}
}

func TestFeatureLiveSnapshotAndPhysicalReplacement(t *testing.T) {
	for _, flavor := range []string{GeometryFormatMySQL, GeometryFormatMariaDB} {
		t.Run(flavor, func(t *testing.T) {
			db, config := featureLiveDatabase(t, flavor)
			columns := "id BIGINT PRIMARY KEY, geom TEXT, name VARCHAR(40)"
			table := featureLiveTable(t, db, columns)
			for id := 1; id <= 300; id++ {
				if _, err := db.Exec("INSERT INTO "+featureQuoteIdentifier(table)+" VALUES (?,?,?)", id, "POINT(1 1)", "original"); err != nil {
					t.Fatal(err)
				}
			}
			p := featureLiveProvider(t, config, map[string]any{"name": "items", "tablename": table,
				"id_fieldname": "id", "geometry_fieldname": "geom", "geometry_format": "wkt", "geometry_type": "point", "srid": 4326})
			count := 0
			_, err := p.QueryFeatures(context.Background(), "items", provider.FeatureQuery{Limit: 400}, func(f *provider.Feature) error {
				count++
				if count == 1 {
					// Mutate a row in the next 256-row chunk after the first snapshot
					// read. A fresh per-chunk transaction would expose this mutation.
					if _, err := db.Exec("UPDATE "+featureQuoteIdentifier(table)+" SET name=? WHERE id=?", "changed", 300); err != nil {
						return err
					}
				}
				if f.Tags["name"] != "original" {
					t.Errorf("snapshot changed at ID%d: %#v", f.ID, f.Tags)
				}
				return nil
			})
			if err != nil || count != 300 {
				t.Fatalf("snapshot delivery: count%d %v", count, err)
			}
			if _, err := db.Exec("DROP TABLE " + featureQuoteIdentifier(table)); err != nil {
				t.Fatal(err)
			}
			if _, err := db.Exec("CREATE TABLE " + featureQuoteIdentifier(table) + " (" + columns + ") ENGINE=InnoDB"); err != nil {
				t.Fatal(err)
			}
			_, err = p.QueryFeatures(context.Background(), "items", provider.FeatureQuery{Limit: 10}, func(*provider.Feature) error {
				t.Fatal("same-DDL replacement emitted data")
				return nil
			})
			var data provider.FeatureDataError
			if !errors.As(err, &data) {
				t.Fatalf("physical replacement did not fail closed: %v", err)
			}
		})
	}
}

func TestFeatureLiveDDLWaitsForReadSnapshot(t *testing.T) {
	for _, flavor := range []string{GeometryFormatMySQL, GeometryFormatMariaDB} {
		t.Run(flavor, func(t *testing.T) {
			db, config := featureLiveDatabase(t, flavor)
			table := featureLiveTable(t, db, "id BIGINT PRIMARY KEY, geom TEXT, name VARCHAR(40)")
			if _, err := db.Exec("INSERT INTO " + featureQuoteIdentifier(table) + " VALUES (1,'POINT(1 1)','original')"); err != nil {
				t.Fatal(err)
			}
			p := featureLiveProvider(t, config, map[string]any{"name": "items", "tablename": table,
				"id_fieldname": "id", "geometry_fieldname": "geom", "geometry_format": "wkt", "geometry_type": "point", "srid": 4326})
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			conn, err := db.Conn(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				if err := conn.Close(); err != nil {
					t.Error(err)
				}
			}()
			var connectionID uint64
			if err := conn.QueryRowContext(ctx, "SELECT CONNECTION_ID()").Scan(&connectionID); err != nil {
				t.Fatal(err)
			}
			held, release := make(chan struct{}), make(chan struct{})
			var releaseOnce sync.Once
			unlock := func() { releaseOnce.Do(func() { close(release) }) }
			defer unlock()
			queryDone, ddlDone := make(chan error, 1), make(chan error, 1)
			var workers sync.WaitGroup
			defer func() {
				unlock()
				cancel()
				joined := make(chan struct{})
				go func() { workers.Wait(); close(joined) }()
				select {
				case <-joined:
				case <-time.After(3 * time.Second):
					t.Error("live DDL/query workers did not stop after cancellation")
				}
			}()
			workers.Add(1)
			go func() {
				defer workers.Done()
				_, err := p.QueryFeatures(ctx, "items", provider.FeatureQuery{Limit: 10}, func(*provider.Feature) error {
					close(held)
					select {
					case <-release:
						return nil
					case <-ctx.Done():
						return ctx.Err()
					}
				})
				queryDone <- err
			}()
			select {
			case <-held:
			case err := <-queryDone:
				t.Fatalf("query did not hold the snapshot: %v", err)
			case <-ctx.Done():
				t.Fatal(ctx.Err())
			}
			workers.Add(1)
			go func() {
				defer workers.Done()
				_, err := conn.ExecContext(ctx, "ALTER TABLE "+featureQuoteIdentifier(table)+" ADD COLUMN ddl_probe INT NULL")
				ddlDone <- err
			}()
			// Require an actual server metadata-lock wait, not a sleep-based
			// inference from a goroutine that might not yet have sent ALTER.
			waitCtx, stopWait := context.WithTimeout(ctx, 5*time.Second)
			defer stopWait()
			ticker := time.NewTicker(20 * time.Millisecond)
			defer ticker.Stop()
			observed := false
			for !observed {
				var state sql.NullString
				if err := db.QueryRowContext(waitCtx, "SELECT STATE FROM INFORMATION_SCHEMA.PROCESSLIST WHERE ID=?", connectionID).Scan(&state); err != nil {
					unlock()
					cancel()
					t.Fatalf("cannot observe independent DDL state: %v", err)
				}
				observed = state.Valid && strings.Contains(strings.ToLower(state.String), "waiting for table metadata lock")
				if observed {
					break
				}
				select {
				case err := <-ddlDone:
					unlock()
					t.Fatalf("ALTER completed while feature transaction was held: %v", err)
				case <-waitCtx.Done():
					unlock()
					cancel()
					t.Fatal("server did not report a metadata-lock wait")
				case <-ticker.C:
				}
			}
			select {
			case err := <-ddlDone:
				t.Fatalf("ALTER completed before snapshot release: %v", err)
			default:
			}
			unlock()
			for _, done := range []<-chan error{queryDone, ddlDone} {
				select {
				case err := <-done:
					if err != nil {
						t.Fatal(err)
					}
				case <-ctx.Done():
					t.Fatal(ctx.Err())
				}
			}
		})
	}
}
