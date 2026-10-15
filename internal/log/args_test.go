package log_test

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"

	"github.com/alexeydott/tegola/internal/log"
)

// captureLogs runs fn with slog's default logger writing text output to the
// returned buffer, restoring the previous default logger afterwards.
func captureLogs(t *testing.T, fn func()) string {
	t.Helper()
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})))
	defer slog.SetDefault(prev)
	fn()
	return buf.String()
}

// TestLogArgsNonStringFirstArg ensures the variadic logging helpers do not
// panic when the first argument is not a string (part13 P6-30).
func TestLogArgsNonStringFirstArg(t *testing.T) {
	out := captureLogs(t, func() {
		log.Warn(42)
		log.Info(true)
		log.Debug([]int{1, 2})
		log.Error(3.14)
	})
	if !strings.Contains(out, "42") {
		t.Errorf("expected warn output for non-string first arg, got: %q", out)
	}
	if !strings.Contains(out, "true") {
		t.Errorf("expected info output for non-string first arg, got: %q", out)
	}
}

// TestLogArgsEmpty ensures the variadic logging helpers do not panic when
// called with no arguments at all (part13 P6-30).
func TestLogArgsEmpty(t *testing.T) {
	captureLogs(t, func() {
		log.Warn()
		log.Info()
		log.Debug()
		log.Error()
	})
}

// TestLogArgsNoBadKey ensures well-formed calls render the message once and
// pass the remaining arguments as attributes without !BADKEY noise
// (part13 P6-30).
func TestLogArgsNoBadKey(t *testing.T) {
	out := captureLogs(t, func() {
		log.Warn("hello", "k", "v")
		log.Info("hello", "k", "v")
		log.Debug("hello", "k", "v")
		log.Error("hello", "k", "v")
	})
	if strings.Contains(out, "!BADKEY") {
		t.Errorf("log output contains !BADKEY noise: %q", out)
	}
	if strings.Contains(out, "hello=") {
		t.Errorf("log message duplicated as attribute: %q", out)
	}
	if got := strings.Count(out, "k=v"); got != 4 {
		t.Errorf("expected 4 k=v attributes (one per level), got %d: %q", got, out)
	}
	if got := strings.Count(out, "msg=hello"); got != 4 {
		t.Errorf("expected 4 msg=hello (one per level), got %d: %q", got, out)
	}
}
