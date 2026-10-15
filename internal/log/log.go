package log

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"runtime/debug"
	"strings"
)

// LevelSilent is a custom log level that will not
// generate any logs.
const LevelSilent = -8

// NewLogger returns a new tegola text logger writing to stderr at the given
// level. See NewLoggerTo for the details.
func NewLogger(lvl slog.Level, options ...func(opts *slog.HandlerOptions)) *slog.Logger {
	return NewLoggerTo(os.Stderr, lvl, options...)
}

// NewLoggerTo returns a new tegola logger writing single-line, CLI friendly
// text (e.g. `level=WARN msg="…"`), at the given level to w. A nil writer
// falls back to stderr, so logging can never panic on a missing writer. The
// options customise the handler (its Level defaults to lvl). The handler is
// wrapped so that error-level and higher records carry a stack trace in a
// "stack" attribute. Message text and attributes are identical to the JSON
// handler; only the serialisation format differs.
func NewLoggerTo(w io.Writer, lvl slog.Level, options ...func(opts *slog.HandlerOptions)) *slog.Logger {
	handlerOptions := &slog.HandlerOptions{
		Level: lvl,
		// Source attribution stays off by default; it can be enabled
		// through the options when needed.
		AddSource: false,
	}

	for _, opt := range options {
		opt(handlerOptions)
	}

	if w == nil {
		w = os.Stderr
	}

	// Create a base handler that outputs text to w.
	baseHandler := slog.NewTextHandler(w, handlerOptions)

	// Wrap the base handler with our custom handler to add stack traces for errors.
	handler := NewHandler(baseHandler)
	logger := slog.New(handler)

	return logger
}

// Logger returns the package's slog logger: the logger configured via
// slog.SetDefault (see cmd/tegola), or slog's built in default before any
// configuration has happened. Use it when a *slog.Logger is needed instead of
// the package level helpers; it always logs through the same backend as the
// helpers.
func Logger() *slog.Logger {
	return slog.Default()
}

// NewHandler returns a new custom slog.Handler that wraps the provided baseHandler.
// The returned handler augments error-level logs by appending a stack trace.
// A nil baseHandler falls back to a stderr text handler, so the result is
// always safe to use.
func NewHandler(baseHandler slog.Handler) slog.Handler {
	if baseHandler == nil {
		baseHandler = slog.NewTextHandler(os.Stderr, nil)
	}
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

// inner returns the underlying handler, falling back to a stderr text handler
// when h or its handler is nil, so that the zero value of Handler is usable and
// logging can never panic on a missing handler.
func (h *Handler) inner() slog.Handler {
	if h == nil || h.handler == nil {
		return slog.NewTextHandler(os.Stderr, nil)
	}
	return h.handler
}

// Enabled reports whether the underlying handler is enabled for the provided log level.
// It delegates the check to the wrapped handler.
func (h *Handler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.inner().Enabled(ctx, level)
}

// Handle processes the log record r. If the log level is error or higher,
// it adds a "stack" attribute containing the current stack trace to the record.
// The modified record is then passed to the underlying handler for output.
func (h *Handler) Handle(ctx context.Context, r slog.Record) error {
	handler := h.inner()
	// For errors and more severe logs, include the current stack trace.
	if r.Level >= slog.LevelError {
		r.Add("stack", string(debug.Stack()))
	}
	return handler.Handle(ctx, r)
}

// WithAttrs returns a new Handler that includes the specified attributes with every log record.
// It derives a new underlying handler with the extra attributes.
func (h *Handler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &Handler{handler: h.inner().WithAttrs(attrs)}
}

// WithGroup returns a new Handler that associates log records with the specified group name.
// It derives a new underlying handler with the group context applied.
func (h *Handler) WithGroup(name string) slog.Handler {
	return &Handler{handler: h.inner().WithGroup(name)}
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

// splitLogArgs derives the log message and slog attributes from a print-style
// argument list: a string first argument is the message and the remaining
// arguments are passed through as slog attributes. Any other shape (empty
// argument list, non-string first argument) falls back to fmt.Sprint of the
// whole list with no attributes so that odd call shapes cannot panic and the
// message is never duplicated into the attributes (part13 P6-30).
func splitLogArgs(args []any) (msg string, attrs []any) {
	if len(args) == 0 {
		return "", nil
	}
	if s, ok := args[0].(string); ok {
		return s, args[1:]
	}
	return fmt.Sprint(args...), nil
}

func Errorf(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	slog.Error(msg)
}

func Error(args ...any) {
	msg, attrs := splitLogArgs(args)
	slog.Error(msg, attrs...)
}

func Warnf(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	slog.Warn(msg)
}

func Warn(args ...any) {
	msg, attrs := splitLogArgs(args)
	slog.Warn(msg, attrs...)
}

func Infof(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	slog.Info(msg)
}

func Info(args ...any) {
	msg, attrs := splitLogArgs(args)
	slog.Info(msg, attrs...)
}

func Debugf(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	slog.Debug(msg)
}

func Debug(args ...any) {
	msg, attrs := splitLogArgs(args)
	slog.Debug(msg, attrs...)
}
