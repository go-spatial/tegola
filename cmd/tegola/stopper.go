package main

// This is for the profiler support.
type Stopper interface {
	Stop()
}
type __noopt struct{}

func (__noopt) Stop() {}
