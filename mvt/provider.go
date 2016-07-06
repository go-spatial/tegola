package mvt

import "github.com/gdey/tegola/mvt"

type Provider interface {
	MVTLayer(layerName string, tile Tile) (*mvt.Tile, error)
}
