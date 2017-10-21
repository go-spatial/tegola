// +build gotrace

package gotrace

import (
	"log"
	"runtime"
	"time"
)

func init() {
	log.Println("Tracing enabled.")
}

func T() func() {
	var pc [1]uintptr
	runtime.Callers(2, pc[:])
	fn := runtime.FuncForPC(pc[0])
	name := fn.Name()
	return Trace(name)
}

func Trace(name string) func() {
	tracer := time.Now()
	log.Printf("Started %v", name)
	return func() {
		t := time.Now()
		log.Printf(" for %v took eslasped time: %v", name, t.Sub(tracer))
	}
}

func Tracefn(fn func(d time.Duration)) func() {
	tracer := time.Now()
	return func() {
		t := time.Now()
		fn(t.Sub(tracer))
	}

}
