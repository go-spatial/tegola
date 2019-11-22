// Copyright (C) 2018, Michael P. Gerlek (Flaxen Consulting)
//
// Portions of this code were derived from the PROJ.4 software
// In keeping with the terms of the PROJ.4 project, this software
// is provided under the MIT-style license in `LICENSE.md` and may
// additionally be subject to the copyrights of the PROJ.4 authors.

package merror

import (
	"fmt"
	"runtime"
	"strings"

	"github.com/go-spatial/proj/mlog"
)

// ShowSource determines whther to include source file and line information in
// the Error() string
var ShowSource = false

// Error captures the type of error, where it occurred, inner errors, etc.
// Error implements the error interface
type Error struct {
	Message  string
	Function string
	Line     int
	File     string
	Inner    error
}

// New creates a new Error object
func New(format string, v ...interface{}) error {
	file, line, function := stackinfo(2)

	err := Error{
		Message:  fmt.Sprintf(format, v...),
		Function: function,
		Line:     line,
		File:     file,
	}

	mlog.Error(err)

	return err
}

// Wrap returns a new Error object which contains the given error object
//
// If v is used, v[0] must be a format string.
func Wrap(inner error, v ...interface{}) error {
	file, line, function := stackinfo(2)

	format := "wrapped error"
	if len(v) > 0 {
		v0, ok := v[0].(string)
		if !ok {
			mlog.Printf("malformed log message at %s:%d", file, line)
			v = nil
		} else {
			format = v0
			v = v[1:]
		}
	}

	err := Error{
		Message:  fmt.Sprintf(format, v...),
		Function: function,
		Line:     line,
		File:     file,
		Inner:    inner,
	}

	mlog.Error(err)

	return err
}

// Pass just returns the error you give it, with the side effect of logging
// that the event occurred
func Pass(err error) error {
	mlog.Printf("PASSTHROUGH ERROR: %s", err.Error())
	return err
}

func (e Error) Error() string {
	s := e.Message
	if ShowSource {
		s += fmt.Sprintf(" (from %s at %s:%d)", e.Function, e.File, e.Line)
	}
	if e.Inner != nil {
		s += " // Inner: " + e.Inner.Error()
	}

	return s
}

// stackinfo returns (file, line, function)
func stackinfo(depth int) (string, int, string) {

	pc, file, line, ok := runtime.Caller(depth)
	if !ok {
		pc = 0
		file = ""
		line = 0
	}

	function := ""
	if pc != 0 {
		function = runtime.FuncForPC(pc).Name()
	}

	i := strings.LastIndex(file, "/")
	if i >= 0 {
		file = file[i+1:]
	}

	i = strings.LastIndex(function, "/")
	if i >= 0 {
		function = function[i+1:]
	}

	return file, line, function
}
