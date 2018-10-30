// +build pprof

package main

// The point of this file is to load the Go profiler.
// You need to compile Tegola with `go build -tags 'pprof'` and you need to
// enabled it by setting the TEGOLA_HTTP_PPROF_BIND environment to a
// hostname:port combination (e.g. TEGOLA_HTTP_PPROF_BIND=localhost:6060).

// To show 30s CPU profile:
//   % go tool pprof -web http://localhost:6060/debug/pprof/profile
// To show all allocated space:
//   % go tool pprof -alloc_space -web http://localhost:6060/debug/pprof/heap

// The profiler can be completely disabled during the build with the `noPprof` build flag
// for example from the cmd/tegola direcotry:
//
// go build -tags 'pprof'

import (
	"net/http"
	_ "net/http/pprof"
	"os"
	"runtime"
	"strconv"
	"strings"

	"github.com/go-spatial/tegola/internal/log"
)

func init() {
	if bind := os.Getenv("TEGOLA_HTTP_PPROF_BIND"); bind != "" {
		go func() {
			log.Infof("Starting up profiler on %v", bind)
			err := http.ListenAndServe(bind, nil)
			log.Infof("Failed to start up profiler on %v : %v", bind, err)
		}()
		if mutexrate := os.Getenv("TEGOLA_PPROF_MUTEX_RATE"); mutexrate != "" {
			rate, _ := strconv.Atoi(strings.TrimSpace(mutexrate))
			if rate > 0 {
				log.Infof("Setting Mutex Profile Fraction rate to %v", rate)
				runtime.SetMutexProfileFraction(rate)
			}
		}
	}
}
