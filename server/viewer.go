// +build !noViewer

package server

import (
	"net/http"
	"strings"

	"github.com/dimfeld/httptreemux"

	"github.com/go-spatial/tegola/server/bindata"
)

// setupViewer in this file is used for registering the viewer routes when the viewer
// is included in the build (default)
func setupViewer(group *httptreemux.Group) {
	prefixStripper := FilePathPrefixStripper{
		fs: bindata.AssetFileSystem(),
	}

	group.UsingContext().Handler("GET", "/", http.FileServer(&prefixStripper))
	group.UsingContext().Handler("GET", "/*path", http.FileServer(&prefixStripper))
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
