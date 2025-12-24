//go:build !noPrometheusObserver
// +build !noPrometheusObserver

package atlas

// The point of this file is to load and register the prometheus observer backend.
// The prometheus observer can be excluded during the build with the `noPrometheusObserver` build flag
import (
	_ "github.com/go-spatial/tegola/observability/prometheus"
)
