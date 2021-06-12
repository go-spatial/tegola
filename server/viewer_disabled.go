// +build noViewer

package server

import (
	"github.com/dimfeld/httptreemux"
	"github.com/go-spatial/tegola/internal/build"
	"github.com/go-spatial/tegola/observability"
)

func init() {
	// add ourself to the build
	build.Tags = append(build.Tags, "noViewer")
}

// setupViewer in this file is used for removing the viewer routes when the
// build flag `noViewer` is set
func setupViewer(o observability.Interface, group *httptreemux.Group) {}
