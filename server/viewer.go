// +build !noViewer,!go1.16

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

	group.UsingContext().Handler(observability.InstrumentViewerHandler(http.MethodGet, "/", o, http.FileServer(&prefixStripper)))
	group.UsingContext().Handler(observability.InstrumentViewerHandler(http.MethodGet, "/*path", o, http.FileServer(&prefixStripper)))
}

type FilePathPrefixStripper struct {
	fs *bindata.AssetFS
}

func (fsps *FilePathPrefixStripper) Open(name string) (http.File, error) {
	// Don't want to modify the Global.
	// TODO(GDEY): Why is URIPrefix a global? Can it change after startup?
	prefix := URIPrefix
	// Strip any leading "/" from the URIPrefix.
	if len(prefix) > 0 && prefix[len(prefix)-1] == '/' {
		prefix = prefix[:len(prefix)-1]
	}
	if prefix != "" {
		name = strings.TrimPrefix(name, prefix)
	}

	return fsps.fs.Open(name)
}
