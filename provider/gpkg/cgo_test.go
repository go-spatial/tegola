// +build cgo

package gpkg

import (
	"testing"

	"github.com/terranodo/tegola/provider"
)

// This is a test to just see that the init function is not doing
// anything and just returning notsupported.
func TestNewProviderStartup(t *testing.T) {
	_, err := NewTileProvider(nil)
	if err == provider.ErrUnsupported {
		t.Fatal("supported, expected any but unsupported got %v", err)
	}
}
