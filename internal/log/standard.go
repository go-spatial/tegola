package log

import (
	"fmt"
	"io"
	goLog "log"
	"os"
	"runtime"
	"time"
)

var standard Standard

type Standard struct{}

func (_ Standard) SetOutput(w io.Writer) {
	goLog.SetOutput(w)
}

func Output(level string, format string, args ...interface{}) {
	logMsg := fmt.Sprintf(format, args...)
	_, file, line, _ := runtime.Caller(4)
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	// "\r" Eliminates the default message prefix so we can format as we like.
	goLog.Printf("\r%v•%v•%v•%v•%v", timestamp, level, file, line, logMsg)
}

func (_ Standard) Fatal(format string, args ...interface{}) {
	Output("FATAL", format, args...)
	os.Exit(1)
}

func (_ Standard) Error(format string, args ...interface{}) {
	Output("ERROR", format, args...)
}

func (_ Standard) Warn(format string, args ...interface{}) {
	Output("WARN", format, args...)
}

func (_ Standard) Info(format string, args ...interface{}) {
	Output("INFO", format, args...)
}

func (_ Standard) Debug(format string, args ...interface{}) {
	Output("DEBUG", format, args...)
}

func (_ Standard) Trace(format string, args ...interface{}) {
	Output("TRACE", format, args...)
}
