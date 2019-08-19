package errors

import (
	"fmt"
)

// Canceled is used as a sentinel error to allow things to
// state that the operation was cancelled.
const ErrCanceled = String("cancelled")

// NilObject is used as a sentinel error to allow things to
// state that the receiver object of the method is nil
const ErrNilObject = String("nil receiver object")

type Err interface {
	// Error is the human readable version of the error.
	// the main use should be for log files and printing
	// on screen. To compare errors use IsEqual.
	Error() string
	// Cause should return the error that this error wraps
	// if it wraps an error, or nil.
	Cause() error
}

// String is an error type that can be a constant.
type String string

func (str String) Error() string { return string(str) }
func (str String) Cause() error  { return nil }

type wrapped struct {
	Description string
	Err         error
}

func (w wrapped) Error() string {
	return fmt.Sprintf("%s : %s", w.Description, w.Err)
}

func (w wrapped) Cause() error { return w.Err }

// Wrap wraps the error with the given description and returns a new Err object
func Wrap(err error, description string) Err {
	return wrapped{
		Err:         err,
		Description: description,
	}
}

// Wrapf wraps the error with the given description and returns a new Err object
func Wrapf(err error, description string, data ...interface{}) Err {
	return Wrap(err, fmt.Sprintf(description, data...))
}

// Root will walk the error graph returning the top
// Most error and count.
func Root(err error) (nerr error, count int) {
	if err == nil {
		return nil, 0
	}
	for {
		e, ok := err.(Err)
		if !ok {
			return err, count
		}
		nerr = e.Cause()
		if nerr == nil || nerr == err {
			return err, count
		}
		count++
		err = nerr
	}
	panic("should not get here")
}

// Walk each error in the list of error calling fn. If fn returns false
// stop the walk.
func Walk(err error, fn func(err error) bool) {
	var nerr error
	for {
		if !fn(err) {
			return
		}
		e, ok := err.(Err)
		if !ok {
			return
		}
		if nerr = e.Cause(); nerr == nil || nerr == err {
			return
		}
		err = nerr
	}
	panic("should not get here")
}
