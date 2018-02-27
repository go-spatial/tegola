// +build !cgo

package gpkg

import (
	"testing"

	"github.com/go-spatial/tegola/provider"
)

// This is a test to just see that the init function is not doing
// anything and just returning notsupported.
func TestNewProviderStartup(t *testing.T) {
	_, err := NewTileProvider(nil)
	if err != provider.ErrUnsupported {
		t.Fatalf("unsupported, expected %v got %v", provider.ErrUnsupported, err)
	}
}
