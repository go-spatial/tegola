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

func Output(level string, args ...interface{}) {
	_, file, line, _ := runtime.Caller(4)
	logMsg := fmt.Sprint(args...)
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	// "\r" Eliminates the default message prefix so we can format as we like.
	goLog.Printf("\r%v•%v•%v•%v•%v", timestamp, level, file, line, logMsg)
}

func (_ Standard) Fatal(args ...interface{}) {
	Output("FATAL", args...)
	os.Exit(1)
}

func (_ Standard) Error(args ...interface{}) {
	Output("ERROR", args...)
}

func (_ Standard) Warn(args ...interface{}) {
	Output("WARN", args...)
}

func (_ Standard) Info(args ...interface{}) {
	Output("INFO", args...)
}

func (_ Standard) Debug(args ...interface{}) {
	Output("DEBUG", args...)
}

func (_ Standard) Trace(args ...interface{}) {
	Output("TRACE", args...)
}
