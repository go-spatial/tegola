package log_test

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"

	"github.com/go-spatial/tegola/internal/log"
)

func TestStackTraceHandler(t *testing.T) {
	testCases := []struct {
		name        string
		level       slog.Level
		message     string
		expectStack bool
	}{
		{name: "Debug", level: slog.LevelDebug, message: "debug", expectStack: false},
		{name: "Info", level: slog.LevelInfo, message: "info", expectStack: false},
		{name: "Warn", level: slog.LevelWarn, message: "warn", expectStack: false},
		{name: "Error", level: slog.LevelError, message: "error", expectStack: true},
		{name: "Silent", level: log.LevelSilent, message: "error", expectStack: false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			baseHandler := slog.NewTextHandler(&buf, &slog.HandlerOptions{
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
			if tc.expectStack {
				if !strings.Contains(output, "stack") {
					t.Fatal("expect stack in log")
				}

				if tc.level != log.LevelSilent && !strings.Contains(output, "testGroup") {
					t.Fatal("expect testGroup in log")
				}
			} else {
				if strings.Contains(output, "stack") {
					t.Fatal("did not expect stack to be in log")
				}

				if tc.level != log.LevelSilent && !strings.Contains(output, "testGroup") {
					t.Fatal("expect testGroup in log")
				}
			}
		})
	}
}
