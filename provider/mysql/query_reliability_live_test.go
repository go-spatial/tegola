package mysql

import (
	"context"
	"database/sql"
	"errors"
	"net"
	"runtime"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alexeydott/tegola/provider"
	"github.com/alexeydott/tegola/provider/internal/testrelay"
)

func TestFeatureReliabilityLiveDriverCancellationAndReconnect(t *testing.T) {
	for _, flavor := range []string{GeometryFormatMySQL, GeometryFormatMariaDB} {
		t.Run(flavor, func(t *testing.T) {
			admin, config := featureLiveDatabase(t, flavor)
			table := featureLiveTable(t, admin, "id BIGINT PRIMARY KEY, geom TEXT, name LONGTEXT")
			for id := 1; id <= 300; id++ {
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				_, err := admin.ExecContext(ctx, "INSERT INTO "+featureQuoteIdentifier(table)+" VALUES (?, 'POINT(1 2)', ?)", id, strings.Repeat("x", 64*1024))
				cancel()
				if err != nil {
					t.Fatal("owned fixture insertion failed")
				}
			}
			p := featureLiveProvider(t, config, map[string]any{"name": "items", "tablename": table, "id_fieldname": "id", "geometry_fieldname": "geom", "geometry_format": "wkt", "geometry_type": "point", "srid": 4326, "fields": []string{"name"}})
			p.db.SetMaxOpenConns(1)
			p.db.SetMaxIdleConns(1)
			ctx, stop := context.WithTimeout(context.Background(), 5*time.Second)
			var ownID uint64
			err := p.db.QueryRowContext(ctx, "SELECT CONNECTION_ID()").Scan(&ownID)
			stop()
			if err != nil {
				t.Fatal("owned provider connection identification failed")
			}
			lockCtx, lockStop := context.WithTimeout(context.Background(), 5*time.Second)
			lock, err := admin.Conn(lockCtx)
			lockStop()
			if err != nil {
				t.Fatal("owned lock connection failed")
			}
			defer func() {
				if err := lock.Close(); err != nil {
					t.Error("owned lock connection close failed")
				}
			}()
			lockCtx, lockStop = context.WithTimeout(context.Background(), 5*time.Second)
			_, err = lock.ExecContext(lockCtx, "LOCK TABLES "+featureQuoteIdentifier(table)+" WRITE")
			lockStop()
			if err != nil {
				t.Fatal("owned source lock failed")
			}
			queryCtx, cancel := context.WithCancel(context.Background())
			done := make(chan error, 1)
			finished := make(chan struct{})
			var callbacks atomic.Int64
			go func() {
				defer close(finished)
				_, err := p.QueryFeatures(queryCtx, "items", provider.FeatureQuery{Limit: 1}, func(*provider.Feature) error { callbacks.Add(1); return nil })
				done <- err
			}()
			released := false
			release := func() {
				if released {
					return
				}
				released = true
				c, s := context.WithTimeout(context.Background(), 5*time.Second)
				defer s()
				if _, e := lock.ExecContext(c, "UNLOCK TABLES"); e != nil {
					t.Error("owned source unlock failed")
				}
			}
			defer func() {
				cancel()
				release()
				select {
				case <-finished:
				case <-time.After(5 * time.Second):
					t.Error("driver worker not joined")
				}
			}()
			observed := false
			pollCtx, pollStop := context.WithTimeout(context.Background(), 5*time.Second)
			for pollCtx.Err() == nil {
				var state, statement sql.NullString
				e := admin.QueryRowContext(pollCtx, "SELECT STATE,INFO FROM information_schema.PROCESSLIST WHERE ID=?", ownID).Scan(&state, &statement)
				if e == nil && strings.Contains(statement.String, table) && strings.HasPrefix(strings.ToUpper(strings.TrimSpace(statement.String)), "SELECT") && strings.Contains(strings.ToLower(state.String), "lock") {
					observed = true
					break
				}
				time.Sleep(5 * time.Millisecond)
			}
			pollStop()
			if !observed {
				t.Fatal("owned source SELECT lock wait was not observed; no driver cancellation proof")
			}
			cancel()
			select {
			case e := <-done:
				if !errors.Is(e, context.Canceled) {
					t.Fatalf("driver cancellation chain missing: %T", e)
				}
			case <-time.After(5 * time.Second):
				t.Fatal("actual driver cancellation did not complete")
			}
			if callbacks.Load() != 0 {
				t.Fatal("callback before source lock/cancellation")
			}
			release()
			healthy := func() {
				c, s := context.WithTimeout(context.Background(), 5*time.Second)
				defer s()
				count := 0
				_, e := p.QueryFeatures(c, "items", provider.FeatureQuery{Limit: 1}, func(*provider.Feature) error { count++; return nil })
				if e != nil || count != 1 {
					t.Fatalf("same finite pool did not recover: count%d error%T", count, e)
				}
			}
			healthy()
			c, s := context.WithTimeout(context.Background(), 5*time.Second)
			err = p.db.QueryRowContext(c, "SELECT CONNECTION_ID()").Scan(&ownID)
			s()
			if err != nil {
				t.Fatal("owned replacement connection identification failed")
			}
			count := 0
			killed := false
			c, s = context.WithTimeout(context.Background(), 10*time.Second)
			_, err = p.QueryFeatures(c, "items", provider.FeatureQuery{Limit: 300}, func(*provider.Feature) error {
				count++
				if count != 1 {
					return nil
				}
				verify, vs := context.WithTimeout(context.Background(), 5*time.Second)
				defer vs()
				var dbName string
				var statement sql.NullString
				if e := admin.QueryRowContext(verify, "SELECT DB,INFO FROM information_schema.PROCESSLIST WHERE ID=?", ownID).Scan(&dbName, &statement); e != nil || dbName != p.layers["items"].feature.schema.database || !statement.Valid || !strings.HasPrefix(strings.TrimSpace(statement.String), "SELECT ") || !strings.Contains(statement.String, featureQuoteIdentifier(table)) {
					return errors.New("owned connection identity verification failed")
				}
				if _, e := admin.ExecContext(verify, "KILL CONNECTION "+strconv.FormatUint(ownID, 10)); e != nil {
					return errors.New("owned connection kill failed")
				}
				killed = true
				return nil
			})
			s()
			if !killed || err == nil || count > featureChunkSize {
				t.Fatalf("source loss retried or succeeded: killed%v count%d error%T", killed, count, err)
			}
			healthy()
			if p.db.Stats().MaxOpenConnections != 1 {
				t.Fatal("finite pool budget changed")
			}
		})
	}
}

func TestFeatureReliabilityLiveRowTransferCancellation(t *testing.T) {
	for _, flavor := range []string{GeometryFormatMySQL, GeometryFormatMariaDB} {
		t.Run(flavor, func(t *testing.T) {
			admin, config := featureLiveDatabase(t, flavor)
			if tls, _ := config.String("tls", nil); tls != "" {
				t.Fatal("fixture relay requires explicit plaintext transport")
			}
			table := featureLiveTable(t, admin, "id BIGINT PRIMARY KEY, geom TEXT, name LONGTEXT")
			ctx, stop := context.WithTimeout(context.Background(), 5*time.Second)
			_, err := admin.ExecContext(ctx, "INSERT INTO "+featureQuoteIdentifier(table)+" VALUES (1,'POINT(1 2)','small'),(2,'POINT(1 2)',?)", strings.Repeat("x", 2*1024*1024))
			stop()
			if err != nil {
				t.Fatal("owned payload insertion failed")
			}
			host, _ := config.String("host", nil)
			port, _ := config.Int("port", nil)
			relay, err := testrelay.Start(net.JoinHostPort(host, strconv.Itoa(port)), table)
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				if err := relay.Close(); err != nil {
					t.Error("owned relay cleanup failed")
				}
			}()
			host, portText, _ := net.SplitHostPort(relay.Address)
			port, _ = strconv.Atoi(portText)
			config["host"] = host
			config["port"] = port
			p := featureLiveProvider(t, config, map[string]any{"name": "items", "tablename": table, "id_fieldname": "id", "geometry_fieldname": "geom", "geometry_format": "wkt", "geometry_type": "point", "srid": 4326, "fields": []string{"name"}})
			p.db.SetMaxOpenConns(1)
			p.db.SetMaxIdleConns(1)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			var callbacks atomic.Int32
			done := make(chan error, 1)
			go func() {
				_, e := p.QueryFeatures(ctx, "items", provider.FeatureQuery{Limit: 2}, func(*provider.Feature) error { callbacks.Add(1); return nil })
				done <- e
			}()
			defer func() {
				cancel()
				select {
				case <-done:
				case <-time.After(5 * time.Second):
					t.Error("owned transfer goroutine cleanup timeout")
				}
			}()
			select {
			case stage := <-relay.Stage:
				// The relay may gate before the race-instrumented consumer is
				// scheduled. Keep the payload blocked until this query's own
				// goroutine is positively observed in Rows.Next.
				witnessDeadline := time.Now().Add(5 * time.Second)
				witness := false
				for !witness && time.Now().Before(witnessDeadline) {
					stack := make([]byte, 128*1024)
					n := runtime.Stack(stack, true)
					for _, goroutine := range strings.Split(string(stack[:n]), "\n\n") {
						if strings.Contains(goroutine, "database/sql.(*Rows).nextLocked") && strings.Contains(goroutine, "provider/mysql.(*Provider).QueryFeatures") {
							witness = true
							break
						}
					}
					if !witness {
						time.Sleep(time.Millisecond)
					}
				}
				if !witness {
					t.Fatal("partial transfer not observed inside owned query Rows.Next")
				}
				t.Logf("actual Rows.Next partial payload bytes%d", stage.Bytes)
				beforeCancel := callbacks.Load()
				if beforeCancel > 1 {
					t.Fatal("incomplete payload reached callback")
				}
				cancel()
				select {
				case err = <-done:
					done <- err
				case <-time.After(5 * time.Second):
					t.Fatal("actual row transfer cancellation unbounded")
				}
				if !errors.Is(err, context.Canceled) || callbacks.Load() != beforeCancel {
					t.Fatalf("row cancellation chain/callbacks error%T count%d", err, callbacks.Load())
				}
				select {
				case <-stage.Disconnected:
				case <-time.After(5 * time.Second):
					t.Fatal("owned cancelled socket remained connected")
				}
			case <-time.After(10 * time.Second):
				t.Fatal("known source SELECT payload stage not observed")
			}
			ctx, stop = context.WithTimeout(context.Background(), 5*time.Second)
			defer stop()
			count := 0
			_, err = p.QueryFeatures(ctx, "items", provider.FeatureQuery{Limit: 1}, func(*provider.Feature) error { count++; return nil })
			if err != nil || count != 1 || p.db.Stats().MaxOpenConnections != 1 {
				t.Fatalf("finite pool recovery error%T count%d", err, count)
			}
		})
	}
}
