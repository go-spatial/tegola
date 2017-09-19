package mvt

import (
	"context"

	"github.com/terranodo/tegola"
)

var ErrCanceled = context.Canceled

//Provider is the mechanism by which the system talks to different data providers.
type Provider interface {
	// MVTLayer returns a layer object based
	MVTLayer(ctx context.Context, layerName string, tile tegola.Tile, tags map[string]interface{}) (*Layer, error)
	// Layers returns information about the various layers the provider supports
	Layers() ([]LayerInfo, error)
}

type LayerInfo interface {
	Name() string
	GeomType() tegola.Geometry
	SRID() int
}
