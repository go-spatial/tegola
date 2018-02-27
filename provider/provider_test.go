package provider_test

import (
	"testing"

	"github.com/go-spatial/tegola/provider"
	"github.com/go-spatial/tegola/provider/test"
)

func TestProviderInterface(t *testing.T) {
	if _, err := provider.For(test.Name, nil); err != nil {
		t.Errorf("retieve provider err , expected nil got %v", err)
		return
	}
	if test.Count != 1 {
		t.Errorf(" expected count , expected 1 got %v", test.Count)
	}
	provider.Cleanup()
	if test.Count != 0 {
		t.Errorf(" expected count , expected 0 got %v", test.Count)
	}
}
