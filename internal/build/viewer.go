//go:build !noViewer
// +build !noViewer

package build

import (
	"github.com/alexeydott/tegola/ui"
)

func ViewerVersion() string {
	version := ui.Version()
	if version == "" {
		return uiVersionDefaultText
	}

	return version
}
