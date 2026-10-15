package mysql

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/alexeydott/geom"
	"github.com/alexeydott/tegola"
	"github.com/alexeydott/tegola/provider"
	"github.com/alexeydott/tegola/provider/test/fixture"
)

type queryDiagnosticCapture struct {
	mu      sync.Mutex
	records []slog.Record
}

func (*queryDiagnosticCapture) Enabled(context.Context, slog.Level) bool { return true }
func (c *queryDiagnosticCapture) Handle(_ context.Context, record slog.Record) error {
	if record.Message == "mysql tile query complete" {
		c.mu.Lock()
		c.records = append(c.records, record.Clone())
		c.mu.Unlock()
	}
	return nil
}
func (c *queryDiagnosticCapture) WithAttrs([]slog.Attr) slog.Handler { return c }
func (c *queryDiagnosticCapture) WithGroup(string) slog.Handler      { return c }
func (c *queryDiagnosticCapture) snapshot() []slog.Record {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]slog.Record{}, c.records...)
}

func captureQueryDiagnostics(t *testing.T) *queryDiagnosticCapture {
	t.Helper()
	capture := &queryDiagnosticCapture{records: []slog.Record{}}
	previous := slog.Default()
	slog.SetDefault(slog.New(capture))
	t.Cleanup(func() { slog.SetDefault(previous) })
	return capture
}

func diagnosticAttributes(record slog.Record) map[string]any {
	attrs := map[string]any{}
	record.Attrs(func(a slog.Attr) bool {
		attrs[a.Key] = a.Value.Any()
		return true
	})
	return attrs
}

func assertDiagnosticPhaseTimes(t *testing.T, attrs map[string]any) {
	t.Helper()
	total := attrs["total_ms"].(float64)
	var sum float64
	for _, key := range []string{"query_ms", "rows_decode_ms", "callback_ms"} {
		elapsed := attrs[key].(float64)
		if elapsed < 0 || elapsed > total {
			t.Fatalf("%s=%v outside total_ms=%v", key, elapsed, total)
		}
		sum += elapsed
	}
	if sum > total {
		t.Fatalf("phase times overlap: sum=%v total=%v", sum, total)
	}
}

func diagnosticProvider(t *testing.T) *Provider {
	t.Helper()
	db := fixture.OpenSQLRows(t, fixture.SQLRows{
		Columns: []string{"id", "geom"},
		Rows:    [][]driver.Value{{int64(1), "POINT (0 0)"}},
	})
	return &Provider{
		db: db,
		layers: map[string]Layer{
			"diagnostics": {
				name: "diagnostics", sql: "SELECT id, geom FROM secret_table WHERE !BBOX!",
				idFieldname: "id", geomFieldname: "geom", geometryFormat: GeometryFormatWKT,
				srid: tegola.WebMercator, geomType: geom.Point{},
			},
		},
	}
}

func TestTileQueryPoolWaitDiagnostics(t *testing.T) {
	capture := captureQueryDiagnostics(t)
	p := diagnosticProvider(t)
	p.db.SetMaxOpenConns(1)
	held, err := p.db.Conn(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := held.Close(); err != nil && !errors.Is(err, sql.ErrConnDone) {
			t.Error(err)
		}
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	tile := provider.NewTile(0, 0, 0, 0, tegola.WebMercator)
	err = p.TileFeatures(ctx, "diagnostics", tile, nil, func(*provider.Feature) error {
		t.Fatal("saturated query must not emit features")
		return nil
	})
	if err != context.DeadlineExceeded {
		t.Fatalf("context error changed: %v", err)
	}
	records := capture.snapshot()
	if len(records) != 1 {
		t.Fatalf("want one attempt, got %d", len(records))
	}
	a := diagnosticAttributes(records[0])
	assertDiagnosticPhaseTimes(t, a)
	if records[0].Level != slog.LevelWarn || a["outcome"] != "deadline" || a["failure_phase"] != "query" {
		t.Fatalf("wrong deadline record: %v", a)
	}
	if a["pool_wait_count_delta_global"] != int64(1) || a["pool_wait_ms_delta_global"].(float64) <= 0 {
		t.Fatalf("missing global pool wait: %v", a)
	}
	if a["pool_in_use"] != int64(1) || a["pool_max_open"] != int64(1) || a["query_ms"].(float64) <= 0 {
		t.Fatalf("missing pool/query observations: %v", a)
	}
	if err := held.Close(); err != nil {
		t.Fatal(err)
	}
	var features int
	err = p.TileFeatures(context.Background(), "diagnostics", tile, nil, func(*provider.Feature) error {
		features++
		return nil
	})
	if err != nil || features != 1 {
		t.Fatalf("query after release: features=%d, err=%v", features, err)
	}
	records = capture.snapshot()
	if len(records) != 2 {
		t.Fatalf("want successful second attempt record, got %d", len(records))
	}
	a = diagnosticAttributes(records[1])
	assertDiagnosticPhaseTimes(t, a)
	if records[1].Level != slog.LevelDebug || a["outcome"] != "ok" || a["failure_phase"] != "none" ||
		a["rows"] != int64(1) || a["features"] != int64(1) || a["attempt"] != int64(1) {
		t.Fatalf("wrong success record: %v", a)
	}
}

func TestTileQueryCallbackDiagnostics(t *testing.T) {
	capture := captureQueryDiagnostics(t)
	p := diagnosticProvider(t)
	wantErr := errors.New("secret callback error")
	err := p.TileFeatures(
		context.Background(), "diagnostics", provider.NewTile(0, 0, 0, 0, tegola.WebMercator), nil,
		func(*provider.Feature) error {
			time.Sleep(2 * time.Millisecond)
			return wantErr
		},
	)
	if err != wantErr {
		t.Fatalf("callback error changed: %v", err)
	}
	records := capture.snapshot()
	if len(records) != 1 {
		t.Fatalf("want one attempt, got %d", len(records))
	}
	a := diagnosticAttributes(records[0])
	assertDiagnosticPhaseTimes(t, a)
	if a["failure_phase"] != "callback" || a["outcome"] != "error" || a["callback_ms"].(float64) < 1 {
		t.Fatalf("wrong callback timing: %v", a)
	}
	if a["rows_decode_ms"].(float64) < 0 || a["query_ms"].(float64) < 0 {
		t.Fatalf("negative timings: %v", a)
	}
	if strings.Contains(fmt.Sprint(a), "secret") {
		t.Fatalf("completion record disclosed SQL or error text: %v", a)
	}
}

func TestTileQueryPanicDiagnostics(t *testing.T) {
	capture := captureQueryDiagnostics(t)
	p := diagnosticProvider(t)
	wantPanic := errors.New("secret panic payload")
	var recovered any
	func() {
		defer func() { recovered = recover() }()
		_ = p.TileFeatures(
			context.Background(),
			"diagnostics",
			provider.NewTile(0, 0, 0, 0, tegola.WebMercator),
			nil,
			func(*provider.Feature) error { panic(wantPanic) },
		)
	}()
	if recovered != wantPanic {
		t.Fatalf("panic payload changed: %v", recovered)
	}
	records := capture.snapshot()
	if len(records) != 1 {
		t.Fatalf("want one panic completion record, got %d", len(records))
	}
	a := diagnosticAttributes(records[0])
	assertDiagnosticPhaseTimes(t, a)
	if records[0].Level != slog.LevelWarn || a["outcome"] != "error" || a["failure_phase"] != "callback" {
		t.Fatalf("panic reported incorrectly: %v", a)
	}
	if strings.Contains(fmt.Sprint(a), "secret") {
		t.Fatalf("completion record disclosed panic payload: %v", a)
	}
}

func TestTileQueryOutcome(t *testing.T) {
	for _, tt := range []struct {
		err     error
		elapsed time.Duration
		outcome string
		level   slog.Level
	}{
		{err: nil, elapsed: time.Millisecond, outcome: "ok", level: slog.LevelDebug},
		{err: nil, elapsed: time.Second, outcome: "ok", level: slog.LevelWarn},
		{err: context.Canceled, elapsed: time.Second, outcome: "canceled", level: slog.LevelDebug},
		{err: fmt.Errorf("wrapped: %w", context.DeadlineExceeded), outcome: "deadline", level: slog.LevelWarn},
		{err: errors.New("failure"), outcome: "error", level: slog.LevelWarn},
	} {
		outcome, level := tileQueryOutcome(tt.err, tt.elapsed)
		if outcome != tt.outcome || level != tt.level {
			t.Errorf("got %s/%v; want %s/%v", outcome, level, tt.outcome, tt.level)
		}
	}
}
