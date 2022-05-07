package log

import (
	"fmt"
	"io"
	"sync"
)

var TimestampRegex string = `\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}` // Ex: "2006-01-02 15:04:05"

type Interface interface {
	// These all take args the same as calls to fmt.Printf()
	Fatal(...interface{})
	Error(...interface{})
	Warn(...interface{})
	Info(...interface{})
	Debug(...interface{})
	Trace(...interface{})
	SetOutput(io.Writer)
}

func init() {
	SetLogger(STANDARD)
	SetLogLevel(INFO)
}

func SetLogger(n string) {
	switch n {
	case "zap":
		buildZapLogger()
		logger = zapLogger
	case "standard":
		logger = standard
	default:
		logger = standard
	}
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

// Supported Loggers
const (
	STANDARD string = "standard"
	ZAP      string = "zap"
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

func (lvl Level) String() string {
	switch lvl {
	case TRACE:
		return "TRACE"
	case DEBUG:
		return "DEBUG"
	case INFO:
		return "INFO"
	case WARN:
		return "WARN"
	case ERROR:
		return "ERROR"
	case FATAL:
		return "FATAL"
	default:
		return "UNKNOWN LEVEL"
	}
}

func SetOutput(w io.Writer) {
	logger.SetOutput(w)
}

func GetLogLevel() Level {
	return level
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
func Fatalf(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	logger.Fatal(msg)
}

func Errorf(format string, args ...interface{}) {
	if !IsError {
		return
	}
	msg := fmt.Sprintf(format, args...)
	logger.Error(msg)
}

func Warnf(format string, args ...interface{}) {
	if !IsWarn {
		return
	}
	msg := fmt.Sprintf(format, args...)
	logger.Warn(msg)
}

func Infof(format string, args ...interface{}) {
	if !IsInfo {
		return
	}
	msg := fmt.Sprintf(format, args...)
	logger.Info(msg)
}

func Debugf(format string, args ...interface{}) {
	if !IsDebug {
		return
	}
	msg := fmt.Sprintf(format, args...)
	logger.Debug(msg)
}

func Tracef(format string, args ...interface{}) {
	if !IsTrace {
		return
	}
	msg := fmt.Sprintf(format, args...)
	logger.Trace(msg)
}

func Fatal(args ...interface{}) {
	logger.Fatal(args...)
}

func Error(args ...interface{}) {
	if !IsError {
		return
	}
	logger.Error(args...)
}

func Warn(args ...interface{}) {
	if !IsWarn {
		return
	}
	logger.Warn(args...)
}

func Info(args ...interface{}) {
	if !IsInfo {
		return
	}
	logger.Info(args...)
}

func Debug(args ...interface{}) {
	if !IsDebug {
		return
	}
	logger.Debug(args...)
}

func Trace(args ...interface{}) {
	if !IsTrace {
		return
	}
	logger.Trace(args...)
}
