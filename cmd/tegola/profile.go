// +build profile

package main

import (
	"flag"

	"github.com/pkg/profile"
)

var mode = flag.String("profile.mode", "cpu", "enable profiling mode, one of [cpu, mem, mutex, block]")

func setupProfiler() Stopper {
	switch *mode {
	case "cpu":
		return profile.Start(profile.CPUProfile)
	case "mem":
		return profile.Start(profile.MemProfile)
	case "mutex":
		return profile.Start(profile.MutexProfile)
	case "block":
		return profile.Start(profile.BlockProfile)
	default:
		// do nothing
		return __noopt{}
	}
}
