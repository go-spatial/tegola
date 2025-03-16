package log

import (
	"bytes"
	"context"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestStackTraceHandler_TableDriven(t *testing.T) {
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
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			baseHandler := slog.NewTextHandler(&buf, &slog.HandlerOptions{
				Level:     slog.LevelDebug,
				AddSource: true,
			})
			handler := &Handler{handler: baseHandler}
			logger := slog.New(handler)

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
				require.Contains(t, output, "stack", "Error log should include a stack trace")
			} else {
				require.NotContains(t, output, "stack", "Non-error log should not include a stack trace")
			}
		})
	}
}
