package postgis

import (
	"context"
	"errors"
	"net/url"
	"os"
	"reflect"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alexeydott/tegola/dict"
	"github.com/alexeydott/tegola/provider"
	"github.com/alexeydott/tegola/provider/internal/testrelay"
)

func TestFeatureReliabilityLiveDriverCancellationAndReconnect(t *testing.T) {
	admin, schema := featureLiveDatabase(t)
	qualified := pgQuoteIdent(schema) + `."reliability_items"`
	setup, stop := context.WithTimeout(context.Background(), 5*time.Second)
	_, err := admin.Exec(setup, "CREATE TABLE "+qualified+" (id BIGINT PRIMARY KEY,geom TEXT,name TEXT);INSERT INTO "+qualified+" SELECT n,'POINT(1 2)','owned' FROM generate_series(1,300) n")
	stop()
	if err != nil {
		t.Fatal("owned fixture setup failed")
	}
	tiler, err := NewTileProvider(dict.Dict{"name": "phase10_reliability", "uri": os.Getenv("PGURI"), ConfigKeyPoolMaxConns: 1, "layers": []map[string]any{{"name": "items", "tablename": qualified, "id_fieldname": "id", "geometry_fieldname": "geom", "geometry_format": "wkt", "geometry_type": "point", "srid": 4326, "fields": []string{"name"}}}}, nil)
	if err != nil {
		t.Fatal("owned provider registration failed")
	}
	p := tiler.(*Provider)
	defer p.pool.Close()
	if p.pool.Config().MaxConns != 1 {
		t.Fatal("finite provider pool not configured")
	}
	catalogCtx, catalogStop := context.WithTimeout(context.Background(), 5*time.Second)
	meta, _, catalogErr := collectMapplGISMeta(catalogCtx, p.pool, schema, "reliability_items")
	catalogStop()
	if catalogErr != nil || !reflect.DeepEqual(meta.PrimaryKeyColumns, []string{"id"}) || len(meta.Indexes) != 1 || !reflect.DeepEqual(meta.Indexes[0].Columns, []string{"id"}) {
		t.Fatalf("pool1 metadata ordering/key regression error%T", catalogErr)
	}
	catalogCtx, catalogStop = context.WithCancel(context.Background())
	catalogStop()
	_, _, catalogErr = collectMapplGISMeta(catalogCtx, p.pool, schema, "reliability_items")
	if !errors.Is(catalogErr, context.Canceled) {
		t.Fatalf("metadata cancellation chain lost error%T", catalogErr)
	}
	c, s := context.WithTimeout(context.Background(), 5*time.Second)
	var ownPID uint32
	err = p.pool.QueryRow(c, "SELECT pg_backend_pid()").Scan(&ownPID)
	s()
	if err != nil {
		t.Fatal("owned provider PID identification failed")
	}
	c, s = context.WithTimeout(context.Background(), 5*time.Second)
	lock, err := admin.Begin(c)
	s()
	if err != nil {
		t.Fatal("owned lock transaction failed")
	}
	released := false
	release := func() {
		if released {
			return
		}
		released = true
		c, s := context.WithTimeout(context.Background(), 5*time.Second)
		defer s()
		if err := lock.Rollback(c); err != nil {
			t.Error("owned lock rollback failed")
		}
	}
	defer release()
	c, s = context.WithTimeout(context.Background(), 5*time.Second)
	_, err = lock.Exec(c, "LOCK TABLE ONLY "+qualified+" IN ACCESS EXCLUSIVE MODE")
	s()
	if err != nil {
		t.Fatal("owned source lock failed")
	}
	queryCtx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	finished := make(chan struct{})
	var callbacks atomic.Int64
	go func() {
		defer close(finished)
		_, e := p.QueryFeatures(queryCtx, "items", provider.FeatureQuery{Limit: 1}, func(*provider.Feature) error { callbacks.Add(1); return nil })
		done <- e
	}()
	defer func() {
		cancel()
		release()
		select {
		case <-finished:
		case <-time.After(5 * time.Second):
			t.Error("actual driver worker not joined")
		}
	}()
	observed := false
	poll, ps := context.WithTimeout(context.Background(), 5*time.Second)
	for poll.Err() == nil {
		var wait, statement string
		e := admin.QueryRow(poll, "SELECT coalesce(wait_event_type,''),query FROM pg_catalog.pg_stat_activity WHERE pid=$1", ownPID).Scan(&wait, &statement)
		if e == nil && wait == "Lock" && strings.Contains(statement, qualified) && strings.HasPrefix(statement, "LOCK TABLE ONLY") {
			observed = true
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	ps()
	if !observed {
		t.Fatal("owned source guard lock wait not observed; no driver cancellation proof")
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
		t.Fatal("callback before protected source cancellation")
	}
	release()
	healthy := func() {
		c, s := context.WithTimeout(context.Background(), 5*time.Second)
		defer s()
		count := 0
		_, e := p.QueryFeatures(c, "items", provider.FeatureQuery{Limit: 1}, func(*provider.Feature) error { count++; return nil })
		if e != nil || count != 1 {
			t.Fatalf("same finite pool did not recover count%d error%T", count, e)
		}
	}
	healthy()
	c, s = context.WithTimeout(context.Background(), 5*time.Second)
	err = p.pool.QueryRow(c, "SELECT pg_backend_pid()").Scan(&ownPID)
	s()
	if err != nil {
		t.Fatal("replacement owned PID identification failed")
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
		var statement string
		if e := admin.QueryRow(verify, "SELECT query FROM pg_catalog.pg_stat_activity WHERE pid=$1", ownPID).Scan(&statement); e != nil || !strings.Contains(statement, qualified) {
			return errors.New("owned PID/source association verification failed")
		}
		if e := admin.QueryRow(verify, "SELECT pg_terminate_backend($1)", ownPID).Scan(&killed); e != nil || !killed {
			return errors.New("owned provider connection termination failed")
		}
		return nil
	})
	s()
	if !killed || err == nil || count > featureCandidateChunk {
		t.Fatalf("source loss retried or succeeded killed%v count%d error%T", killed, count, err)
	}
	healthy()
}

func TestFeatureReliabilityLiveRowTransferCancellation(t *testing.T) {
	admin, schema := featureLiveDatabase(t)
	qualified := pgQuoteIdent(schema) + `."row_transfer_items"`
	ctx, stop := context.WithTimeout(context.Background(), 5*time.Second)
	_, err := admin.Exec(ctx, "CREATE TABLE "+qualified+" (id BIGINT PRIMARY KEY,geom TEXT,name TEXT);INSERT INTO "+qualified+" VALUES (1,'POINT(1 2)','small'),(2,'POINT(1 2)',repeat('x',2097152))")
	stop()
	if err != nil {
		t.Fatal("owned payload fixture setup failed")
	}
	u, err := url.Parse(os.Getenv("PGURI"))
	if err != nil {
		t.Fatal("explicit fixture URI invalid")
	}
	relay, err := testrelay.Start(u.Host, qualified)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := relay.Close(); err != nil {
			t.Error("owned relay cleanup failed")
		}
	}()
	u.Host = relay.Address
	// Keep the complete fixture URI authoritative. Setting only PGHOST/PGPORT
	// selects environment mode, which discards the URI credentials/database.
	// Ambient connection variables must not redirect the owned relay either.
	for _, key := range connModeEnvTriggers {
		t.Setenv(key, "")
	}
	tiler, err := NewTileProvider(dict.Dict{"name": "phase10_row_transfer", "uri": u.String(), ConfigKeyPoolMaxConns: 1, "layers": []map[string]any{{"name": "items", "tablename": qualified, "id_fieldname": "id", "geometry_fieldname": "geom", "geometry_format": "wkt", "geometry_type": "point", "srid": 4326, "fields": []string{"name"}}}}, nil)
	if err != nil {
		t.Fatal("owned relay provider registration failed")
	}
	p := tiler.(*Provider)
	defer p.pool.Close()
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
		stack := make([]byte, 128*1024)
		// The relay sends Stage after its socket write, before the provider
		// goroutine necessarily consumes those bytes. Wait for the driver to
		// reach Rows.Next instead of sampling that independent goroutine once.
		observedNext := false
		deadline := time.Now().Add(time.Second)
		for time.Now().Before(deadline) {
			n := runtime.Stack(stack, true)
			if strings.Contains(string(stack[:n]), "pgx/v5.(*baseRows).Next") {
				observedNext = true
				break
			}
			time.Sleep(time.Millisecond)
		}
		if !observedNext {
			t.Fatal("partial transfer not observed inside Rows.Next")
		}
		t.Logf("actual Rows.Next partial payload bytes%d", stage.Bytes)
		beforeCancel := callbacks.Load()
		if beforeCancel > 1 {
			t.Fatal("incomplete payload reached callback")
		}
		cancelStarted := time.Now()
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
		t.Logf("request cancellation returned after %s callbacks%d", time.Since(cancelStarted), callbacks.Load())
		// pgconn.asyncClose separately drains pending wire messages with its
		// own 15s deadline. Request cancellation does not wait for that cleanup.
		select {
		case <-stage.Disconnected:
			t.Logf("driver socket cleanup completed after %s", time.Since(cancelStarted))
		case <-time.After(time.Until(cancelStarted.Add(20 * time.Second))):
			t.Fatal("owned cancelled socket remained connected")
		}
		availabilityCtx, availabilityStop := context.WithDeadline(context.Background(), cancelStarted.Add(20*time.Second))
		available, availabilityErr := p.pool.Acquire(availabilityCtx)
		availabilityStop()
		if availabilityErr != nil {
			t.Fatal("finite pool unavailable within aggregate cleanup bound")
		}
		available.Release()
		t.Logf("finite pool available after %s", time.Since(cancelStarted))
	case <-time.After(10 * time.Second):
		t.Fatal("known source SELECT payload stage not observed")
	}
	ctx, stop = context.WithTimeout(context.Background(), 5*time.Second)
	defer stop()
	count := 0
	_, err = p.QueryFeatures(ctx, "items", provider.FeatureQuery{Limit: 1}, func(*provider.Feature) error { count++; return nil })
	if err != nil || count != 1 || p.pool.Config().MaxConns != 1 {
		t.Fatalf("finite pool recovery error%T count%d", err, count)
	}
}
