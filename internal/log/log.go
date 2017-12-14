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

func SetOutput(w io.Writer) {
	logger.SetOutput(w)
}

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
func Fatal(msg interface{}, args ...interface{}) {
	var msgString string
	switch m := msg.(type) {
	case string:
		msgString = m
	case error:
		msgString = m.Error()
	}
	logger.Fatal(msgString, args...)
}

func Error(msg interface{}, args ...interface{}) {
	if !IsError {
		return
	}
	var msgString string
	switch m := msg.(type) {
	case string:
		msgString = m
	case error:
		msgString = m.Error()
	}
	logger.Error(msgString, args...)
}

func Warn(msg interface{}, args ...interface{}) {
	if !IsWarn {
		return
	}
	var msgString string
	switch m := msg.(type) {
	case string:
		msgString = m
	case error:
		msgString = m.Error()
	}
	logger.Warn(msgString, args...)
}

func Info(msg interface{}, args ...interface{}) {
	if !IsInfo {
		return
	}
	var msgString string
	switch m := msg.(type) {
	case string:
		msgString = m
	case error:
		msgString = m.Error()
	}
	logger.Info(msgString, args...)
}

func Debug(msg interface{}, args ...interface{}) {
	if !IsDebug {
		return
	}
	var msgString string
	switch m := msg.(type) {
	case string:
		msgString = m
	case error:
		msgString = m.Error()
	}
	logger.Debug(msgString, args...)
}

func Trace(msg interface{}, args ...interface{}) {
	if !IsTrace {
		return
	}
	var msgString string
	switch m := msg.(type) {
	case string:
		msgString = m
	case error:
		msgString = m.Error()
	}
	logger.Trace(msgString, args...)
}
