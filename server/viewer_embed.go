//go:build !noViewer && go1.16
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
	// We need to Strip the URIPrefix from the request path before serving the file
	// This is used when the server sits behind a reverse proxy with a prefix (i.e. /tegola)
	group.UsingContext().Handler(observability.InstrumentViewerHandler(http.MethodGet, "/", o, http.StripPrefix(URIPrefix, http.FileServer(ui.GetDistFileSystem()))))
	group.UsingContext().Handler(observability.InstrumentViewerHandler(http.MethodGet, "/*path", o, http.StripPrefix(URIPrefix, http.FileServer(ui.GetDistFileSystem()))))
}
