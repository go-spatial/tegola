package mvt

import (
	"context"

	"github.com/terranodo/tegola"
)

var ErrCanceled = context.Canceled

//Provider is the mechanism by which the system talks to different data providers.
type Provider interface {
	// MVTLayer returns a layer object based
	MVTLayer(layerName string, tile tegola.Tile, tags map[string]interface{}) (*Layer, error)
	// MVTLayerWithContext is just like MVTLayer but the context is used to cancel, the layer generation. If the Layer is canceled
	// the error returned should be ErrCanceled.
	MVTLayerWithContext(ctx context.Context, layerName string, tile tegola.Tile, tags map[string]interface{}) (*Layer, error)
	// LayerNames returns a list of layer name the Provider knows about.
	LayerNames() []string
}
