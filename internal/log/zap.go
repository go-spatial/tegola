package log

import (
	"io"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var zapLogger *ZapLogger

type ZapLogger struct {
	*zap.SugaredLogger
}

// SetOutput...
// To satisfy log.Interface
func (z *ZapLogger) SetOutput(io.Writer) {
	return
}

// Trace ...
// Trace is not a supported log level by uber/zap
// To satisfy log.Interface
func (z *ZapLogger) Trace(args ...interface{}) {
	z.Debug(args)
}

func newZapDefaultConfig() zap.Config {
	return zap.Config{
		Level:       zap.NewAtomicLevelAt(zap.DebugLevel),
		Development: false,
		Sampling: &zap.SamplingConfig{
			Initial:    100,
			Thereafter: 100,
		},
		Encoding: "json",
		EncoderConfig: zapcore.EncoderConfig{
			TimeKey:        "timestamp",
			LevelKey:       "level",
			NameKey:        "logger",
			CallerKey:      "caller",
			MessageKey:     "message",
			StacktraceKey:  "stacktrace",
			LineEnding:     zapcore.DefaultLineEnding,
			EncodeLevel:    zapcore.LowercaseLevelEncoder,
			EncodeTime:     zapcore.ISO8601TimeEncoder,
			EncodeDuration: zapcore.SecondsDurationEncoder,
			EncodeCaller:   zapcore.ShortCallerEncoder,
		},
		OutputPaths:      []string{"stdout"},
		ErrorOutputPaths: []string{"stderr"},
	}
}

func buildZapLogger() error {
	zapconfig := newZapDefaultConfig()

	zapconfig.DisableCaller = true
	core, err := zapconfig.Build()
	if err != nil {
		return err
	}

	defer core.Sync()
	sl := core.Sugar()

	zapLogger = &ZapLogger{sl}
	return nil
}
