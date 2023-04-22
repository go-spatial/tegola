// +build !noViewer,go1.16

package server

import (
	"net/http"

	"github.com/dimfeld/httptreemux"
	"github.com/go-spatial/tegola/observability"
	"github.com/go-spatial/tegola/ui"
)

// setupViewer in this file is used for registering the viewer routes when the viewer
// is included in the build (default)
func setupViewer(o observability.Interface, group *httptreemux.Group) {

	group.UsingContext().Handler(observability.InstrumentViewerHandler(http.MethodGet, "/", o, http.FileServer(ui.GetDistFileSystem())))
	group.UsingContext().Handler(observability.InstrumentViewerHandler(http.MethodGet, "/*path", o, http.FileServer(ui.GetDistFileSystem())))
}
