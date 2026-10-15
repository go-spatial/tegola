package mysql

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"time"

	"github.com/alexeydott/tegola/internal/log"
	"github.com/alexeydott/tegola/provider"
)

type tileQueryDiagnostics struct {
	started, phaseStarted   time.Time
	phase                   string
	query, decode, callback time.Duration
	rows, features          int
	before                  sql.DBStats
	queried                 bool
}

func (d *tileQueryDiagnostics) enter(phase string) {
	now := time.Now()
	elapsed := now.Sub(d.phaseStarted)
	switch d.phase {
	case "rows_decode":
		d.decode += elapsed
	case "callback":
		d.callback += elapsed
	}
	d.phase, d.phaseStarted = phase, now
}

func (d *tileQueryDiagnostics) finish(
	db *sql.DB,
	layer string,
	tile provider.Tile,
	attempt int,
	err error,
) {
	// Diagnostics must never replace a business error or a propagated panic.
	// Tile implementations and configured logging sinks are external callbacks.
	defer func() { _ = recover() }()
	phase := d.phase
	d.enter("done")
	total := time.Since(d.started)
	outcome, level := tileQueryOutcome(err, total)
	if err == nil {
		phase = "none"
	}
	var z, x, y uint
	if d.queried {
		z, x, y = diagnosticTileCoordinates(tile)
	}
	after := sql.DBStats{}
	if db != nil {
		after = db.Stats()
	}
	if !d.queried {
		d.before = after
	}
	// Pool deltas cover all overlapping operations on this DB. In particular,
	// query_ms includes both pool acquisition and driver execution; neither
	// global delta is an exact per-request pool-wait measurement.
	log.Logger().Log(context.Background(), level, "mysql tile query complete",
		"provider", Name, "layer", layer, "z", z, "x", x, "y", y, "attempt", attempt,
		"outcome", outcome, "failure_phase", phase,
		"total_ms", float64(total)/float64(time.Millisecond),
		"query_ms", float64(d.query)/float64(time.Millisecond),
		"rows_decode_ms", float64(d.decode)/float64(time.Millisecond),
		"callback_ms", float64(d.callback)/float64(time.Millisecond),
		"rows", d.rows, "features", d.features,
		"pool_open", after.OpenConnections, "pool_in_use", after.InUse,
		"pool_idle", after.Idle, "pool_max_open", after.MaxOpenConnections,
		"pool_wait_count_delta_global", after.WaitCount-d.before.WaitCount,
		"pool_wait_ms_delta_global", float64(after.WaitDuration-d.before.WaitDuration)/float64(time.Millisecond),
	)
}

func diagnosticTileCoordinates(tile provider.Tile) (z, x, y uint) {
	defer func() { _ = recover() }()
	if tile != nil {
		zoom, x, y := tile.ZXY()
		return uint(zoom), x, y
	}
	return 0, 0, 0
}

func tileQueryOutcome(err error, elapsed time.Duration) (string, slog.Level) {
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return "deadline", slog.LevelWarn
	case errors.Is(err, context.Canceled):
		return "canceled", slog.LevelDebug
	case err != nil:
		return "error", slog.LevelWarn
	case elapsed >= time.Second:
		return "ok", slog.LevelWarn
	default:
		return "ok", slog.LevelDebug
	}
}
