package log

import (
	"io"
	"sync"
)

type Logger interface {
	// These all take args the same as calls to fmt.Printf()
	Fatal(string, ...interface{})
	Error(string, ...interface{})
	Warn(string, ...interface{})
	Info(string, ...interface{})
	Debug(string, ...interface{})
	Trace(string, ...interface{})
	SetOutput(io.Writer)
}

func init() {
	defaultLogger = &stdLogger
}

// Allows the ordering of severity to be checked
const (
	FATAL = 60
	ERROR = 50
	WARN  = 40
	INFO  = 30
	DEBUG = 20
	TRACE = 10
)

var (
	defaultLogger Logger
	logLevel      int = INFO
	logLock       sync.Mutex
	IsError       bool = true
	IsWarn        bool = true
	IsInfo        bool = true
	IsDebug       bool = false
	IsTrace       bool = false
)

func SetLogLevel(level int) {
	logLock.Lock()
	switch {
	case level <= TRACE:
		logLevel = TRACE
		IsError = true
		IsWarn = true
		IsInfo = true
		IsDebug = true
		IsTrace = true
	case level <= DEBUG:
		logLevel = DEBUG
		IsError = true
		IsWarn = true
		IsInfo = true
		IsDebug = true
		IsTrace = false
	case level <= INFO:
		logLevel = INFO
		IsError = true
		IsWarn = true
		IsInfo = true
		IsDebug = false
		IsTrace = false
	case level <= WARN:
		logLevel = WARN
		IsError = true
		IsWarn = true
		IsInfo = false
		IsDebug = false
		IsTrace = false
	case level <= ERROR:
		logLevel = ERROR
		IsError = true
		IsWarn = false
		IsInfo = false
		IsDebug = false
		IsTrace = false
	default:
		logLevel = FATAL
		IsError = false
		IsWarn = false
		IsInfo = false
		IsDebug = false
		IsTrace = false
	}
	logLock.Unlock()
}

// Output format should be: "timestamp•LOG_LEVEL•filename.go•linenumber•output"
func Fatal(format string, args ...interface{}) {
	defaultLogger.Fatal(format, args...)
}

func Error(format string, args ...interface{}) {
	if IsError {
		defaultLogger.Error(format, args...)
	}
}

func Warn(format string, args ...interface{}) {
	if IsWarn {
		defaultLogger.Warn(format, args...)
	}
}

func Info(format string, args ...interface{}) {
	if IsInfo {
		defaultLogger.Info(format, args...)
	}
}

func Debug(format string, args ...interface{}) {
	if IsDebug {
		defaultLogger.Debug(format, args...)
	}
}

func Trace(format string, args ...interface{}) {
	if IsTrace {
		defaultLogger.Trace(format, args...)
	}
}
