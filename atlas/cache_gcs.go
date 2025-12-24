//go:build !noGCSCache
// +build !noGCSCache

package atlas

// The point of this file is to load and register the GCS cache backend.
// the GCS cache can be excluded during the build with the `noGCSCache` build flag
// for example from the cmd/tegola directory:
//
// go build -tags 'noGCSCache'
import (
	_ "github.com/go-spatial/tegola/cache/gcs"
)
