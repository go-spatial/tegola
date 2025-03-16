package log

import (
	"context"
	"fmt"
	"log/slog"
	"runtime/debug"
	"strings"
)

// Handler is a custom slog.Handler wrapper that adds a stack trace to error logs.
type Handler struct {
	handler slog.Handler
}

func (h *Handler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.handler.Enabled(ctx, level)
}

func (h *Handler) Handle(ctx context.Context, r slog.Record) error {
	// For errors and more severe logs, include the current stack trace.
	if r.Level >= slog.LevelError {
		r.Add("stack", string(debug.Stack()))
	}
	return h.handler.Handle(ctx, r)
}

func (h *Handler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &Handler{handler: h.handler.WithAttrs(attrs)}
}

func (h *Handler) WithGroup(name string) slog.Handler {
	return &Handler{handler: h.handler.WithGroup(name)}
}

func ParseLogLevel(level string) slog.Level {
	switch strings.ToLower(level) {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

func Errorf(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	slog.Error(msg)
}

func Error(args ...interface{}) {
	slog.Error(args[0].(string), args...)
}

func Warnf(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	slog.Warn(msg)
}

func Warn(args ...interface{}) {
	slog.Warn(args[0].(string), args...)
}

func Infof(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	slog.Info(msg)
}

func Info(args ...interface{}) {
	slog.Info(args[0].(string), args...)
}

func Debugf(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	slog.Debug(msg)
}

func Debug(args ...interface{}) {
	slog.Debug(args[0].(string), args...)
}
