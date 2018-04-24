// +build !noViewer

package server

import (
	"net/http"

	"github.com/dimfeld/httptreemux"

	"github.com/go-spatial/tegola/server/bindata"
)

// setupViewer in this file is used for reigstering the viewer routes when the viewer
// is included in the build (default)
func setupViewer(group *httptreemux.Group) {
	group.UsingContext().Handler("GET", "/", http.FileServer(bindata.AssetFileSystem()))
	group.UsingContext().Handler("GET", "/*path", http.FileServer(bindata.AssetFileSystem()))
}
