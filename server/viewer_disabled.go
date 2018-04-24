// +build noViewer

package server

import "github.com/dimfeld/httptreemux"

// setupViewer in this file is used for removing the viewer routes when the
// build flag `noViewer` is set
func setupViewer(group *httptreemux.Group) {}
