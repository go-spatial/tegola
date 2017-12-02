package log

import (
	"fmt"
	"io"
	goLog "log"
	"os"
	"runtime"
)

var stdLogger StdLogger

type StdLogger struct {
	Logger
}

func SetOutput(w io.Writer) {
	goLog.SetOutput(w)
}

func Output(level string, format string, args ...interface{}) {
	logMsg := fmt.Sprintf(format, args...)
	_, file, line, _ := runtime.Caller(3)
	// timestamp will be provided by goLog
	goLog.Printf("•%v•%v•%v•%v", level, file, line, logMsg)
}

func (l *StdLogger) Fatal(format string, args ...interface{}) {
	Output("FATAL", format, args...)
	os.Exit(1)
}

func (l *StdLogger) Error(format string, args ...interface{}) {
	Output("ERROR", format, args...)
}

func (l *StdLogger) Warn(format string, args ...interface{}) {
	Output("WARN", format, args...)
}

func (l *StdLogger) Info(format string, args ...interface{}) {
	Output("INFO", format, args...)
}

func (l *StdLogger) Debug(format string, args ...interface{}) {
	Output("DEBUG", format, args...)
}

func (l *StdLogger) Trace(format string, args ...interface{}) {
	Output("TRACE", format, args...)
}
