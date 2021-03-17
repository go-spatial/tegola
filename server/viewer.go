// +build !noViewer

package server

import (
	"net/http"
	"strings"

	"github.com/go-spatial/tegola/observability"

	"github.com/dimfeld/httptreemux"

	"github.com/go-spatial/tegola/server/bindata"
)

// setupViewer in this file is used for registering the viewer routes when the viewer
// is included in the build (default)
func setupViewer(o observability.Interface, group *httptreemux.Group) {
	prefixStripper := FilePathPrefixStripper{
		fs: bindata.AssetFileSystem(),
	}

	group.UsingContext().Handler(observability.InstrumentHandler(http.MethodGet, "/", o, http.FileServer(&prefixStripper)))
	group.UsingContext().Handler(observability.InstrumentHandler(http.MethodGet, "/*path", o, http.FileServer(&prefixStripper)))
}

type FilePathPrefixStripper struct {
	fs *bindata.AssetFS
}

func (fsps *FilePathPrefixStripper) Open(name string) (http.File, error) {
	if URIPrefix != "/" {
		name = strings.TrimPrefix(name, URIPrefix)
	}

	return fsps.fs.Open(name)
}
