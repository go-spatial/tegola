//go:build !noViewer && go1.16

package server

import (
	"io/fs"
	"testing"

	"github.com/alexeydott/tegola/ui"
)

func TestFeatureDiscoveryViewerCollisions(t *testing.T) {
	preserveDiscoveryGlobals(t)
	entries, err := fs.ReadDir(ui.GetDistFS(), ".")
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) == 0 {
		t.Fatal("embedded viewer has no entries")
	}
	for _, entry := range entries {
		api := discoveryAPI(t)
		api.cfg.BasePath = "/" + entry.Name() + "/features"
		if _, err := NewRouterWithOptions(nil, RouterOptions{Features: api}); err == nil {
			t.Fatalf("viewer prefix %q shadowed", entry.Name())
		}
	}
}
