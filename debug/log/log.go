// +build debug

// Log package
package log

import "log"

func Printf(format string, args ...interface{}) {
	log.Printf(format, args...)
}

func Println(args ...interface{}) {
	log.Println(args...)
}
