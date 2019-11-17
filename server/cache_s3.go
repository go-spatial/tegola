// +build !noS3Cache

package server

// The point of this file is to load and register the s3 cache backend.
// the s3 cache can be excluded during the build with the `noS3Cache` build flag
// for example from the cmd/tegola directory:
//
// go build -tags 'noS3Cache'
import (
	_ "github.com/go-spatial/tegola/cache/s3"
)
