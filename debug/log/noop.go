// +build !debug

// Log package
package log

// All methods here should be a noop.
func Printf(format string, args ...interface{}) {}
func Println(args ...interface{})               {}
