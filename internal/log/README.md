# internal/log

Consolidated logging for tegola. Every log statement in first party code goes
through this package or through its *slog.Logger accessor; the single backend
is the standard library `log/slog`.

## Backend

`log/slog` with a text handler: one line per record, CLI friendly and easy to
grep. The level word is always present (`level=DEBUG`, `level=INFO`,
`level=WARN`, `level=ERROR`), so operational tooling can keep counting
"WARN"/"ERROR" lines. Records at error level and above additionally carry the
goroutine stack in a `stack` attribute.

	time=2024-01-01T00:00:00.000Z level=WARN msg="cache: invalid file key: ..."

`NewLoggerTo` builds a logger on any `io.Writer` (a nil writer falls back to
stderr and never panics); `NewLogger` writes to stderr. Both wrap the handler
so error records carry stack traces. JSON output is still available for
operators who want structured logs:

	slog.New(log.NewHandler(slog.NewJSONHandler(os.Stderr, nil)))

## Levels

Debug, Info, Warn and Error, plus `log.LevelSilent` for "off".
`ParseLogLevel` maps the CLI strings `debug`, `info`, `warn`, `error` and
`silent` (anything else defaults to info).

## Functions

* `log.Error(vals...)`, `log.Warn(vals...)`, `log.Info(vals...)`,
  `log.Debug(vals...)` and their `...f(format, args...)` variants.
  The variadic helpers take a message plus optional `key, value` attribute
  pairs; odd argument shapes are stringified safely and never panic.

* `log.Logger() *slog.Logger`: the shared slog logger (the configured
  default). Use it when a *slog.Logger is needed instead of the package level
  helpers, e.g. for third party APIs:

	log.Logger().Info("tile is rather large", slog.Int("size_kb", kb))

* At startup `cmd/tegola` configures the logger once via
  `slog.SetDefault(log.NewLogger(...))`, which also routes the standard
  library `log` package through the same backend.