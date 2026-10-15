package hana_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/SAP/go-hdb/driver"
	"github.com/alexeydott/tegola/dict"
	"github.com/alexeydott/tegola/provider"
	"github.com/alexeydott/tegola/provider/hana"
)

func hanaGuardFixture(t *testing.T) (*hana.Provider, *sql.DB, string) {
	t.Helper()
	if os.Getenv(TESTENV) != "yes" || GetConnectionURI() == "" {
		t.Skip("requires live HANA")
	}
	db, e := hana.OpenDB(GetConnectionURI())
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { featureFixtureClose(t, db) })
	table := featureFixtureTable(t, "TEGOLA_GUARD")
	q := `"` + table + `"`
	if _, e = db.Exec(`CREATE ROW TABLE ` + q + ` ("id" BIGINT PRIMARY KEY,"geom" NVARCHAR(100),"name" NVARCHAR(30))`); e != nil {
		t.Fatal(e)
	}
	for _, sql := range []string{`INSERT INTO ` + q + ` VALUES(1,'POINT(1 2)','first')`, `INSERT INTO ` + q + ` VALUES(2,'POINT(3 4)','second')`} {
		if _, e = db.Exec(sql); e != nil {
			t.Fatal(e)
		}
	}
	t.Cleanup(func() {
		if _, e := db.Exec(`DROP TABLE ` + q); e != nil {
			t.Error(e)
		}
	})
	p, e := hana.CreateProvider(dict.Dict{hana.ConfigKeyName: "guard", hana.ConfigKeyURI: GetConnectionURI(), hana.ConfigKeyMaxConn: 1, "layers": []map[string]any{{"name": "items", "tablename": q, "id_fieldname": "id", "geometry_fieldname": "geom", "geometry_format": "wkt", "geometry_type": "point", "srid": 4326, "fields": []string{"name"}}}}, nil, hana.ProviderType)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(p.Close)
	return p, db, table
}

func TestFeatureLiveSourceLockAdversarial(t *testing.T) {
	for _, kind := range []string{"ALTER", "DROP", "RENAME", "writer", "loss", "cancel callback"} {
		t.Run(kind, func(t *testing.T) {
			p, db, table := hanaGuardFixture(t)
			q := `"` + table + `"`
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			entered := make(chan struct{})
			release := make(chan struct{})
			done := make(chan error, 1)
			joined := make(chan struct{})
			var releaseOnce sync.Once
			unblock := func() { releaseOnce.Do(func() { close(release) }) }
			t.Cleanup(func() {
				unblock()
				select {
				case <-joined:
				case <-time.After(5 * time.Second):
					t.Error("guard query goroutine did not finish during cleanup")
				}
			})
			calls := 0
			go func() {
				defer close(joined)
				_, e := p.QueryFeatures(ctx, "items", provider.FeatureQuery{Limit: 10}, func(*provider.Feature) error {
					calls++
					if calls == 1 {
						close(entered)
						<-release
					}
					return nil
				})
				done <- e
			}()
			select {
			case <-entered:
			case e := <-done:
				t.Fatalf("beforecallback %v", e)
			case <-ctx.Done():
				t.Fatal(ctx.Err())
			}
			admin, e := db.Conn(context.Background())
			if e != nil {
				t.Fatal(e)
			}
			defer featureFixtureClose(t, admin)
			if _, e = admin.ExecContext(context.Background(), "SET TRANSACTION LOCK WAIT TIMEOUT 50"); e != nil {
				t.Fatal(e)
			}
			switch kind {
			case "loss":
				var id int
				e = admin.QueryRowContext(context.Background(), `SELECT t.CONNECTION_ID FROM SYS.M_TRANSACTIONS t JOIN SYS.M_OBJECT_LOCKS l ON t.TRANSACTION_ID=l.LOCK_OWNER_TRANSACTION_ID WHERE l.OBJECT_NAME=? AND l.LOCK_MODE='EXCLUSIVE'`, table).Scan(&id)
				if e != nil {
					t.Fatal(e)
				}
				_, e = admin.ExecContext(context.Background(), fmt.Sprintf("ALTER SYSTEM DISCONNECT SESSION '%d'", id))
				if e != nil {
					t.Fatal(e)
				}
			case "cancel callback":
				cancel()
				time.Sleep(50 * time.Millisecond)
				_, e = admin.ExecContext(context.Background(), "ALTER TABLE "+q+" ADD (AFTER_CANCEL INTEGER)")
				if e != nil {
					t.Fatalf("cancel did not release serverlock %v", e)
				}
			default:
				statement := map[string]string{"ALTER": "ALTER TABLE " + q + " ADD (ADDED INTEGER)", "DROP": "DROP TABLE " + q, "RENAME": "RENAME TABLE " + q + " TO " + `"` + table + `_RENAMED"`, "writer": "UPDATE " + q + ` SET "name"='mutated' WHERE "id"=1`}[kind]
				_, e = admin.ExecContext(context.Background(), statement)
				var typed driver.Error
				if !errors.As(e, &typed) || typed.Code() != 131 {
					t.Fatalf("guard should block %s131, got %v", kind, e)
				}
			}
			unblock()
			e = <-done
			if kind == "loss" && e == nil {
				t.Fatal("losttransaction succeeded")
			}
			if kind == "cancel callback" && !errors.Is(e, context.Canceled) {
				t.Fatalf("cancel error %v", e)
			}
			if kind != "loss" && kind != "cancel callback" && e != nil {
				t.Fatal(e)
			}
			if (kind == "loss" || kind == "cancel callback") && calls != 1 {
				t.Fatalf("callback continued %d", calls)
			}
			if kind != "cancel callback" {
				_, e = admin.ExecContext(context.Background(), "ALTER TABLE "+q+" ADD (AFTER_RELEASE INTEGER)")
				if e != nil {
					t.Fatalf("lock/session leaked %v", e)
				}
			}
		})
	}
}

func TestFeatureLiveSourceABAAndFinitePool(t *testing.T) {
	p, db, table := hanaGuardFixture(t)
	q := `"` + table + `"`
	if _, e := db.Exec("DROP TABLE " + q); e != nil {
		t.Fatal(e)
	}
	if _, e := db.Exec(`CREATE ROW TABLE ` + q + ` ("id" BIGINT PRIMARY KEY,"geom" NVARCHAR(100),"name" NVARCHAR(30))`); e != nil {
		t.Fatal(e)
	}
	called := false
	_, e := p.QueryFeatures(context.Background(), "items", provider.FeatureQuery{Limit: 1}, func(*provider.Feature) error { called = true; return nil })
	var data provider.FeatureDataError
	if called || !errors.As(e, &data) {
		t.Fatalf("ABA guard %v callback%v", e, called)
	}
	p2, _, _ := hanaGuardFixture(t)
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_, e := p2.QueryFeatures(ctx, "items", provider.FeatureQuery{Limit: 1}, func(*provider.Feature) error { return nil })
			if e != nil {
				t.Error(e)
			}
		}()
	}
	wg.Wait()
}

func TestFeatureLiveCancelWaitingForSourceLock(t *testing.T) {
	p, db, table := hanaGuardFixture(t)
	tx, err := db.BeginTx(context.Background(), &sql.TxOptions{Isolation: sql.LevelRepeatableRead})
	if err != nil {
		t.Fatal(err)
	}
	defer featureFixtureRollback(t, tx)
	if _, err := tx.Exec(`LOCK TABLE "` + table + `" IN EXCLUSIVE MODE`); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	calls := 0
	_, err = p.QueryFeatures(ctx, "items", provider.FeatureQuery{Limit: 1}, func(*provider.Feature) error { calls++; return nil })
	if calls != 0 || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("waiting lock cancellation calls=%d: %v", calls, err)
	}
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	ctx2, cancel2 := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel2()
	_, err = p.QueryFeatures(ctx2, "items", provider.FeatureQuery{Limit: 1}, func(*provider.Feature) error { calls++; return nil })
	if err != nil || calls != 1 {
		t.Fatalf("single-connection pool recovery calls=%d: %v", calls, err)
	}
}

func TestFeatureLiveConcurrentLayersSingleConnection(t *testing.T) {
	_, db, table := hanaGuardFixture(t)
	second := table + "_B"
	q := `"` + second + `"`
	if _, err := db.Exec(`CREATE ROW TABLE ` + q + ` ("id" BIGINT PRIMARY KEY,"geom" NVARCHAR(100),"name" NVARCHAR(30))`); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if _, err := db.Exec("DROP TABLE " + q); err != nil {
			t.Error(err)
		}
	}()
	if _, err := db.Exec(`INSERT INTO ` + q + ` VALUES(1,'POINT(10 20)','other layer')`); err != nil {
		t.Fatal(err)
	}
	layers := []map[string]any{}
	for i, source := range []string{table, second} {
		layers = append(layers, map[string]any{"name": fmt.Sprintf("items%d", i), "tablename": `"` + source + `"`, "id_fieldname": "id", "geometry_fieldname": "geom", "geometry_format": "wkt", "geometry_type": "point", "srid": 4326, "fields": []string{"name"}})
	}
	p, err := hana.CreateProvider(dict.Dict{hana.ConfigKeyName: "layers", hana.ConfigKeyURI: GetConnectionURI(), hana.ConfigKeyMaxConn: 1, "layers": layers}, nil, hana.ProviderType)
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	var group sync.WaitGroup
	for i := 0; i < 8; i++ {
		group.Add(1)
		go func(i int) {
			defer group.Done()
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			name := fmt.Sprintf("items%d", i%2)
			calls := 0
			_, err := p.QueryFeatures(ctx, name, provider.FeatureQuery{Limit: 1}, func(f *provider.Feature) error {
				calls++
				want := "first"
				if i%2 == 1 {
					want = "other layer"
				}
				if f.Tags["name"] != want {
					t.Errorf("layer isolation %s: %#v", name, f)
				}
				return nil
			})
			if err != nil || calls != 1 {
				t.Errorf("layer%s callbacks%d: %v", name, calls, err)
			}
		}(i)
	}
	group.Wait()
}
