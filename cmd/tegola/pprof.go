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
// go build -tags 'noPprof'

import (
	"log"
	"net/http"
	_ "net/http/pprof"
	"os"
)

func init() {
	if bind := os.Getenv("TEGOLA_HTTP_PPROF_BIND"); bind != "" {
		go func() {
			log.Fatal(http.ListenAndServe(bind, nil))
		}()
	}
}
