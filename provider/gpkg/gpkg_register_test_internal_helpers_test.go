//go:build cgo

package gpkg

import (
	"errors"
	"testing"

	"github.com/alexeydott/tegola/dict"
)

// NewProviderInternal creates a provider via NewTileProvider so tests can inspect
// unexported layer state (id/geometry fieldnames).
func NewProviderInternal(config dict.Dict) (*Provider, error) {
	tp, err := NewTileProvider(config, nil)
	if err != nil {
		return nil, err
	}
	np, ok := tp.(*Provider)
	if !ok {
		return nil, errNotAGPKGProvider
	}
	return np, nil
}

// errNotAGPKGProvider is returned when the tile provider is not a *Provider.
var errNotAGPKGProvider = errors.New("tile provider is not a gpkg Provider")

// LayerInternal returns the unexported Layer for the given name.
func (p *Provider) LayerInternal(name string) (*Layer, bool) {
	l, ok := p.layers[name]
	return l, ok
}

var _ = testing.Short // keep testing import for future helpers
