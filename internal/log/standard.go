package log

import (
	"fmt"
	"io"
	goLog "log"
	"os"
	"runtime"
	"strings"
	"time"
)

var (
	gologstderr   = goLog.New(os.Stderr, "", 0)
	gologstdout   = goLog.New(os.Stdout, "", 0)
	gologiowriter io.Writer

	standard Standard
)

type Standard struct{}

func (_ Standard) SetOutput(w io.Writer) {
	gologiowriter = w
	goLog.SetOutput(gologiowriter)
}

func StdErrOutput(level string, args ...interface{}) {
	_, file, line, _ := runtime.Caller(4)
	pkgs := strings.Split(file, "/")

	logMsg := fmt.Sprint(args...)
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	if gologiowriter != nil { //give preference to gologiowriter
		// "\r" Eliminates the default message prefix so we can format as we like.
		goLog.Printf("\r%v [%v] %v:%v: %v", timestamp, level, pkgs[len(pkgs)-1], line, logMsg)
	} else {
		gologstderr.Printf("%v [%v] %v:%v: %v", timestamp, level, pkgs[len(pkgs)-1], line, logMsg)
	}
}

func StdOutOutput(level string, args ...interface{}) {
	_, file, line, _ := runtime.Caller(4)
	pkgs := strings.Split(file, "/")

	logMsg := fmt.Sprint(args...)
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	if gologiowriter != nil { //give preference to gologiowriter
		// "\r" Eliminates the default message prefix so we can format as we like.
		goLog.Printf("\r%v [%v] %v:%v: %v", timestamp, level, pkgs[len(pkgs)-1], line, logMsg)
	} else {
		gologstdout.Printf("%v [%v] %v:%v: %v", timestamp, level, pkgs[len(pkgs)-1], line, logMsg)
	}
}

func (_ Standard) Fatal(args ...interface{}) {
	StdErrOutput("FATAL", args...)
	os.Exit(1)
}

func (_ Standard) Error(args ...interface{}) {
	StdErrOutput("ERROR", args...)
}

func (_ Standard) Warn(args ...interface{}) {
	StdOutOutput("WARN", args...)
}

func (_ Standard) Info(args ...interface{}) {
	StdOutOutput("INFO", args...)
}

func (_ Standard) Debug(args ...interface{}) {
	StdOutOutput("DEBUG", args...)
}

func (_ Standard) Trace(args ...interface{}) {
	StdOutOutput("TRACE", args...)
}
