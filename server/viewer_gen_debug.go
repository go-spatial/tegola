// +build bindataDebug
//go:generate go run ../ui/scripts/build/ --bindata-run yes -bindata-debug

package server

import "github.com/go-spatial/tegola/internal/build"

func init() {
	// add ourself to the build
	build.Tags = append(build.Tags, "bindataDebug")
}
