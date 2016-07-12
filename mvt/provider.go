package mvt

import "github.com/terranodo/tegola"

type Provider interface {
	MVTLayer(layerName string, tile tegola.Tile) (*Layer, error)
}
