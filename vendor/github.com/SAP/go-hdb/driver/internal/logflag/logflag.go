// Package logflag provides a boolean flag to use with log enablement.
package logflag

import (
	"io"
	"log"
	"os"
	"strconv"
)

// A Flag represents a boolean value to be used as flag to enable or disable logging output.
type Flag struct {
	log *log.Logger
}

// New returns a new Flag instance.
func New(log *log.Logger) *Flag { return &Flag{log: log} }

func (f *Flag) String() string {
	/*
		The flag package does create flags via reflection to determine default values.
		As this is not using the constructor the flag attributes are not set.
	*/
	if f.log == nil {
		return strconv.FormatBool(false) // default value
	}
	return strconv.FormatBool(f.log.Writer() != io.Discard)
}

// IsBoolFlag implements the flag.Value interface.
func (f *Flag) IsBoolFlag() bool { return true }

// Set implements the flag.Value interface.
func (f *Flag) Set(s string) error {
	b, err := strconv.ParseBool(s)
	if err != nil {
		return err
	}
	if b {
		f.log.SetOutput(os.Stderr)
	} else {
		f.log.SetOutput(io.Discard)
	}
	return nil
}
