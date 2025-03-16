package log_test

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"os"
	"strings"
	"testing"

	"github.com/go-spatial/tegola/internal/log"
)

func TestStackTraceHandler(t *testing.T) {
	type tcase struct {
		level       slog.Level
		message     string
		expectStack bool
	}

	testCases := map[string]tcase{
		"Debug":  {level: slog.LevelDebug, message: "debug", expectStack: false},
		"Info":   {level: slog.LevelInfo, message: "info", expectStack: false},
		"Warn":   {level: slog.LevelWarn, message: "warn", expectStack: false},
		"Error":  {level: slog.LevelError, message: "error", expectStack: true},
		"Silent": {level: log.LevelSilent, message: "error", expectStack: false},
	}

	fn := func(tc tcase) func(t *testing.T) {
		return func(t *testing.T) {
			var buf bytes.Buffer
			writer := io.MultiWriter(&buf, os.Stdout)
			baseHandler := slog.NewTextHandler(writer, &slog.HandlerOptions{
				Level:     slog.LevelDebug,
				AddSource: true,
			})
			handler := log.NewHandler(baseHandler)
			logger := slog.New(handler).WithGroup("testGroup").With("foo", "bar")

			// Log based on level.
			switch tc.level {
			case slog.LevelDebug:
				logger.Debug(tc.message)
			case slog.LevelInfo:
				logger.Info(tc.message)
			case slog.LevelWarn:
				logger.Warn(tc.message)
			case slog.LevelError:
				logger.Error(tc.message)
			default:
				logger.Log(context.Background(), tc.level, tc.message)
			}

			output := buf.String()
			if hasStack := strings.Contains(output, "stack"); tc.expectStack != hasStack {
				t.Fatalf("stack in log, expected %t, got %t", tc.expectStack, hasStack)
			}
		}
	}

	for name, tc := range testCases {
		t.Run(name, fn(tc))
	}
}
