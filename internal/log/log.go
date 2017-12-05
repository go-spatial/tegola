package log

import (
	"io"
	"sync"
)

type Interface interface {
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
	logger = standard
	SetLogLevel(INFO)
}

type Level int

// Allows the ordering of severity to be checked
const (
	TRACE = Level(iota - 2)
	DEBUG
	INFO
	WARN
	ERROR
	FATAL
)

var (
	logger Interface
	level  Level
	lock   sync.Mutex
	// FATAL level is never disabled
	IsError bool
	IsWarn  bool
	IsInfo  bool
	IsDebug bool
	IsTrace bool
)

func SetLogLevel(lvl Level) {
	lock.Lock()
	IsTrace = false
	IsDebug = false
	IsInfo = false
	IsWarn = false
	IsError = false
	IsTrace = false
	if lvl != TRACE && lvl != DEBUG && lvl != INFO && lvl != WARN && lvl != ERROR && lvl != FATAL {
		lvl = INFO
	}
	level = lvl
	switch level {
	case TRACE:
		IsTrace = true
		fallthrough
	case DEBUG:
		IsDebug = true
		fallthrough
	case INFO:
		IsInfo = true
		fallthrough
	case WARN:
		IsWarn = true
		fallthrough
	case ERROR:
		IsError = true
	}
	lock.Unlock()
}

// Output format should be: "timestamp•LOG_LEVEL•filename.go•linenumber•output"
func Fatal(format string, args ...interface{}) {
	logger.Fatal(format, args...)
}

func Error(format string, args ...interface{}) {
	if !IsError {
		return
	}
	logger.Error(format, args...)
}

func Warn(format string, args ...interface{}) {
	if !IsWarn {
		return
	}
	logger.Warn(format, args...)
}

func Info(format string, args ...interface{}) {
	if !IsInfo {
		return
	}
	logger.Info(format, args...)
}

func Debug(format string, args ...interface{}) {
	if !IsDebug {
		return
	}
	logger.Debug(format, args...)
}

func Trace(format string, args ...interface{}) {
	if !IsTrace {
		return
	}
	logger.Trace(format, args...)
}
