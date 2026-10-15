//go:build !noViewer && go1.16

package server

import (
	"fmt"
	"io/fs"
	"strings"

	"github.com/alexeydott/tegola/ui"
)

func validateFeatureViewerPath(basePath string) error {
	entries, err := fs.ReadDir(ui.GetDistFS(), ".")
	if err != nil {
		return fmt.Errorf("features: inspect viewer prefixes: %w", err)
	}
	segment := strings.Split(strings.TrimPrefix(basePath, "/"), "/")[0]
	for _, entry := range entries {
		if entry.Name() == segment {
			return fmt.Errorf("features: basepath conflicts with viewer resource %q", segment)
		}
	}
	return nil
}
