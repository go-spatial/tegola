// Copyright (C) 2018, Michael P. Gerlek (Flaxen Consulting)
//
// Portions of this code were derived from the PROJ.4 software
// In keeping with the terms of the PROJ.4 project, this software
// is provided under the MIT-style license in `LICENSE.md` and may
// additionally be subject to the copyrights of the PROJ.4 authors.

package mlog

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
)

// DebugEnabled controls whether Debug log messages are generated
var DebugEnabled = false

// InfoEnabled controls whether "regular" log messages are generated
var InfoEnabled = true

// ErrorEnabled controls whether Error log messages are generated
var ErrorEnabled = true

func EnableError() {
	defaultLogger.EnableError()
}

func EnableInfo() {
	defaultLogger.EnableInfo()
}

func EnableDebug() {
	defaultLogger.EnableDebug()
}

func DisableError() {
	defaultLogger.DisableError()
}

func DisableInfo() {
	defaultLogger.DisableInfo()
}

func DisableDebug() {
	defaultLogger.DisableDebug()
}

// Debugf writes a debug message to stderr
func Debugf(format string, v ...interface{}) {
	if !defaultLogger.debug.enabled {
		return
	}
	_ = defaultLogger.debug.Output(2, fmt.Sprintf(format, v...))
}

// Printf writes a regular log message to stderr
func Printf(format string, v ...interface{}) {
	if !defaultLogger.info.enabled {
		return
	}
	_ = defaultLogger.info.Output(2, fmt.Sprintf(format, v...))
}

// Printv writes a variable as a regular log message to stderr
//
// TODO: would be nice if this could print the variable name
// (and ideally the private fields too, if reflection allows
// us access to them)
func Printv(v interface{}) error {
	if !defaultLogger.info.enabled {
		return nil
	}
	b, err := json.MarshalIndent(v, "", "    ")
	if err != nil {
		return fmt.Errorf("failed to marshal indent: %w", err)
	}
	return defaultLogger.info.Output(2, string(b))
}

// Error writes an error message to stderr
func Error(err error) {
	if !defaultLogger.error.enabled {
		return
	}
	_ = defaultLogger.error.Output(2, err.Error())
}

type Outputer interface {
	// Output writes the output for a logging event.
	// The string s contains the text to print after the prefix specified by the flags of the Logger.
	//   A newline is appended if the last character of s is not already a newline.
	// calldepth is used to recover the PC and is provided for generality, although at the moment on
	//   all pre-defined paths it will be 2
	Output(calldepth int, s string) error
}

var defaultLogger = NewLoggerSingleOutput(os.Stderr)

type levelLogger struct {
	enabled bool
	Outputer
}

type Logger struct {
	info  levelLogger
	error levelLogger
	debug levelLogger
}

func (l Logger) Debugf(format string, v ...interface{}) error {
	if !l.debug.enabled {
		return nil
	}
	return l.debug.Output(2, fmt.Sprintf(format, v...))
}
func (l Logger) Printf(format string, v ...interface{}) error {
	if !l.info.enabled {
		return nil
	}
	return l.info.Output(2, fmt.Sprintf(format, v...))
}

func (l Logger) Printv(v interface{}) error {
	if !l.info.enabled {
		return nil
	}
	b, err := json.MarshalIndent(v, "", "    ")
	if err != nil {
		return err
	}
	s := string(b)
	return l.info.Output(2, s)
}

func (l Logger) Error(err error) error {
	if !l.error.enabled {
		return nil
	}
	return l.error.Output(2, err.Error())
}

func (l *Logger) EnableError() {
	l.error.enabled = true
}

func (l *Logger) EnableInfo() {
	l.info.enabled = true
}

func (l *Logger) EnableDebug() {
	l.debug.enabled = true
}

func (l *Logger) DisableError() {
	l.error.enabled = false
}

func (l *Logger) DisableInfo() {
	l.info.enabled = false
}

func (l *Logger) DisableDebug() {
	l.debug.enabled = false
}

func NewLoggerSingleOutput(w io.Writer) Logger {
	return Logger{
		info: levelLogger{
			Outputer: log.New(w, "[LOG] ", log.Lshortfile),
		},
		debug: levelLogger{
			Outputer: log.New(w, "[DEBUG] ", log.Lshortfile),
		},
		error: levelLogger{
			Outputer: log.New(w, "[ERROR] ", 0),
		},
	}
}
