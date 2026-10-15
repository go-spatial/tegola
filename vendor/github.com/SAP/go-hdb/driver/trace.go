package driver

import (
	"context"
	"database/sql/driver"
	"flag"
	"fmt"
	"log/slog"
	"strconv"
	"sync/atomic"
	"time"
)

var (
	protTrace atomic.Bool
	sqlTrace  atomic.Bool
)

func setTrace(b *atomic.Bool, s string) error {
	v, err := strconv.ParseBool(s)
	if err == nil {
		b.Store(v)
	}
	return err
}

func init() {
	flag.BoolFunc("hdb.protTrace", "enabling hdb protocol trace", func(s string) error { return setTrace(&protTrace, s) })
	flag.BoolFunc("hdb.sqlTrace", "enabling hdb sql trace", func(s string) error { return setTrace(&sqlTrace, s) })
}

// ProtTrace returns true if protocol tracing output is active, false otherwise.
func ProtTrace() bool { return protTrace.Load() }

// SetProtTrace sets protocol tracing output active or inactive.
// Credential fields in protocol trace output are masked by default and only
// shown in full if redaction is explicitly disabled. If the application cannot
// fully control command-line flags (e.g. in shared or managed environments),
// call SetProtTrace(false) at startup to override the -hdb.protTrace flag.
func SetProtTrace(on bool) { protTrace.Store(on) }

// SQLTrace returns true if sql tracing output is active, false otherwise.
func SQLTrace() bool { return sqlTrace.Load() }

// SetSQLTrace sets sql tracing output active or inactive.
func SetSQLTrace(on bool) { sqlTrace.Store(on) }

const (
	tracePing     = "ping"
	tracePrepare  = "prepare"
	traceQuery    = "query"
	traceExec     = "exec"
	traceExecCall = "call"
)

type sqlTracer struct {
	logger *slog.Logger
	maxArg int
}

const defSQLTracerMaxArg = 5 // default limit of number of arguments

func newSQLTracer(logger *slog.Logger, maxArg int) *sqlTracer {
	if maxArg <= 0 {
		maxArg = defSQLTracerMaxArg
	}
	return &sqlTracer{logger: logger, maxArg: maxArg}
}

func (t *sqlTracer) log(ctx context.Context, startTime time.Time, traceKind string, query string, nvargs ...driver.NamedValue) {
	duration := time.Since(startTime).Milliseconds()
	l := len(nvargs)

	attrs := []slog.Attr{ //nolint:prealloc
		slog.String(traceKind, query),
		slog.Int64("ms", duration),
	}

	if l == 0 {
		t.logger.LogAttrs(ctx, slog.LevelInfo, "SQL", attrs...)
		return
	}

	numArg := min(l, t.maxArg)
	argAttrs := make([]slog.Attr, 0, numArg)
	for i := range numArg {
		name := nvargs[i].Name
		if name == "" {
			name = strconv.Itoa(nvargs[i].Ordinal)
		}
		argAttrs = append(argAttrs, slog.String(name, fmt.Sprintf("%v", nvargs[i].Value)))
	}
	if l > t.maxArg {
		argAttrs = append(argAttrs, slog.Int("numArgSkip", l-t.maxArg))
	}
	attrs = append(attrs, slog.Any("arg", slog.GroupValue(argAttrs...)))

	t.logger.LogAttrs(ctx, slog.LevelInfo, "SQL", attrs...)
}
