package log_test

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"

	"github.com/alexeydott/tegola/internal/log"
)

// TestLoggerAccessor ensures Logger returns the live slog default logger, so
// new code and libraries can log through slog and still hit the tegola
// backend.
func TestLoggerAccessor(t *testing.T) {
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))
	defer slog.SetDefault(prev)

	log.Logger().Info("via accessor")
	if !strings.Contains(buf.String(), "via accessor") {
		t.Fatalf("expected accessor logger to write through the default handler, got: %q", buf.String())
	}
}

// TestNewHandlerNilSafety ensures logging never panics on nil handlers: both
// NewHandler(nil) and the zero value of Handler must be usable.
func TestNewHandlerNilSafety(t *testing.T) {
	h := log.NewHandler(nil)
	if h == nil {
		t.Fatal("NewHandler(nil) returned nil")
	}
	// Must not panic.
	_ = h.WithAttrs([]slog.Attr{slog.String("k", "v")}).WithGroup("g").Handle(context.Background(), slog.Record{
		Level:   slog.LevelWarn,
		Message: "nil handler fallback",
	})

	var zh log.Handler
	// The zero value must be usable without panicking as well.
	if zh.WithGroup("g").WithAttrs(nil) == nil {
		t.Fatal("zero value handler derived a nil handler")
	}
	_ = zh.Handle(context.Background(), slog.Record{
		Level:   slog.LevelWarn,
		Message: "zero value handler fallback",
	})
	_ = zh.Enabled(context.Background(), slog.LevelInfo)
}

// TestNewLoggerToNilWriter ensures a nil writer falls back to stderr instead
// of panicking, and that a custom writer receives the records.
func TestNewLoggerToNilWriter(t *testing.T) {
	// Must not panic.
	log.NewLoggerTo(nil, slog.LevelInfo).Info("nil writer fallback")

	var buf bytes.Buffer
	lg := log.NewLoggerTo(&buf, slog.LevelDebug)
	lg.Debug("dbg")
	lg.Info("inf")
	out := buf.String()
	if !strings.Contains(out, "dbg") || !strings.Contains(out, "inf") {
		t.Fatalf("expected records on the custom writer, got: %q", out)
	}
}

// TestNewLoggerToLevelWordsGreppable ensures the level words stay greppable in
// the output; operational tooling counts WARN/ERROR lines.
func TestNewLoggerToLevelWordsGreppable(t *testing.T) {
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(log.NewLoggerTo(&buf, slog.LevelDebug))
	defer slog.SetDefault(prev)

	log.Warnf("warn word check")
	log.Errorf("error word check")
	out := buf.String()
	for _, want := range []string{
		"level=WARN",
		"warn word check",
		"level=ERROR",
		"error word check",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("expected output to contain %q, got: %q", want, out)
		}
	}
}
