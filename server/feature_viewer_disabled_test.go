//go:build noViewer

package server

import "testing"

func TestFeatureDiscoveryNoViewerPrefixes(t *testing.T) {
	preserveDiscoveryGlobals(t)
	api := discoveryAPI(t)
	api.cfg.BasePath = "/assets/features"
	if _, err := NewRouterWithOptions(nil, RouterOptions{Features: api}); err != nil {
		t.Fatalf("absent viewer prefix rejected: %v", err)
	}
}
