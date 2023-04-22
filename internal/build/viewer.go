//go:build !noViewer
// +build !noViewer

package build

import (
	"github.com/go-spatial/tegola/ui"
	"io/fs"
	"strings"
)

func ViewerVersion() string {
	// get the js dir to get the app.${version}.js file
	files, err := fs.ReadDir(ui.GetDistFS(), "js")
	if err != nil {
		return uiVersionDefaultText
	}
	for _, entry := range files {
		name := entry.Name()
		if strings.HasPrefix(name, "app.") && strings.HasSuffix(name, ".js") {
			// expect it to be app.{version}.js
			return name[4 : len(name)-3]
		}
	}
	return uiVersionDefaultText
}
