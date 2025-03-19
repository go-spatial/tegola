package log

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"runtime/debug"
	"strings"
)

// LevelSilent is a custom log level that will not
// generate any logs.
const LevelSilent = -8

// NewLogger returns a new tegola JSON logger.
func NewLogger(lvl slog.Level, options ...func(opts *slog.HandlerOptions)) *slog.Logger {
	handlerOptions := &slog.HandlerOptions{
		Level: lvl,
		// TODO: enable once we switch to slog.Default
		// instead of internal/log methods
		AddSource: false,
	}

	for _, opt := range options {
		opt(handlerOptions)
	}

	// Create a base handler that outputs to stderr.
	// The AddSource option includes file and line info in each log record.
	baseHandler := slog.NewJSONHandler(os.Stderr, handlerOptions)

	// Wrap the base handler with our custom handler to add stack traces for errors.
	handler := NewHandler(baseHandler)
	logger := slog.New(handler)

	return logger
}

// NewHandler returns a new custom slog.Handler that wraps the provided baseHandler.
// The returned handler augments error-level logs by appending a stack trace.
func NewHandler(baseHandler slog.Handler) slog.Handler {
	return &Handler{
		handler: baseHandler,
	}
}

// Handler is a custom slog.Handler wrapper that adds a stack trace to error logs.
// It wraps an underlying slog.Handler and delegates all log handling, augmenting
// the log record when the log level is error or higher.
type Handler struct {
	handler slog.Handler
}

// Enabled reports whether the underlying handler is enabled for the provided log level.
// It delegates the check to the wrapped handler.
func (h *Handler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.handler.Enabled(ctx, level)
}

// Handle processes the log record r. If the log level is error or higher,
// it adds a "stack" attribute containing the current stack trace to the record.
// The modified record is then passed to the underlying handler for output.
func (h *Handler) Handle(ctx context.Context, r slog.Record) error {
	// For errors and more severe logs, include the current stack trace.
	if r.Level >= slog.LevelError {
		r.Add("stack", string(debug.Stack()))
	}
	return h.handler.Handle(ctx, r)
}

// WithAttrs returns a new Handler that includes the specified attributes with every log record.
// It derives a new underlying handler with the extra attributes.
func (h *Handler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &Handler{handler: h.handler.WithAttrs(attrs)}
}

// WithGroup returns a new Handler that associates log records with the specified group name.
// It derives a new underlying handler with the group context applied.
func (h *Handler) WithGroup(name string) slog.Handler {
	return &Handler{handler: h.handler.WithGroup(name)}
}

// ParseLogLevel converts the provided log level string to the corresponding slog.Level.
// Supported values are "debug", "info", "warn", "error" and "silent". If the input does not match
// any supported level, the function defaults to slog.LevelInfo.
func ParseLogLevel(level string) slog.Level {
	switch strings.ToLower(level) {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	case "silent":
		return LevelSilent
	default:
		return slog.LevelInfo
	}
}

// TODO: remove those methods and use slog straight up
func Errorf(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	slog.Error(msg)
}

func Error(args ...any) {
	slog.Error(args[0].(string), args...)
}

func Warnf(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	slog.Warn(msg)
}

func Warn(args ...any) {
	slog.Warn(args[0].(string), args...)
}

func Infof(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	slog.Info(msg)
}

func Info(args ...any) {
	slog.Info(args[0].(string), args...)
}

func Debugf(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	slog.Debug(msg)
}

func Debug(args ...any) {
	slog.Debug(args[0].(string), args...)
}
