//go:build noViewer
// +build noViewer

package server

import (
	"github.com/dimfeld/httptreemux"

	"github.com/go-spatial/tegola/observability"
)

// setupViewer in this file is used for removing the viewer routes when the
// build flag `noViewer` is set
func setupViewer(o observability.Interface, group *httptreemux.Group) {}
