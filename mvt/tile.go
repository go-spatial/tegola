package mvt

import (
	"fmt"
	"log"

	"github.com/terranodo/tegola"
	"github.com/terranodo/tegola/mvt/vector_tile"
)

//Tile describes a tile.
type Tile struct {
	layers []Layer
}

//AddLayers adds a Layer to the tile
func (t *Tile) AddLayers(layers ...*Layer) error {
	// Need to make sure that all layer names are unique.
	for i := range layers {
		nl := layers[i]
		if nl == nil {
			log.Printf("Got a nil layer for %v", i)
			continue
		}
		for i, l := range t.layers {
			if l.Name == nl.Name {
				return fmt.Errorf("Layer %v, already is named %v, new layer not added.", i, l.Name)
			}
		}
		t.layers = append(t.layers, *nl)
	}
	return nil
}

// Layers returns a copy of the layers in this tile.
func (t *Tile) Layers() (l []Layer) {
	l = append(l, t.layers...)
	return l
}

//VTile returns a tile object according to the Google Protobuff def. This function
// does the hard work of converting everthing to the standard.
func (t *Tile) VTile(extent tegola.BoundingBox) (vt *vectorTile.Tile, err error) {
	vt = new(vectorTile.Tile)
	for _, l := range t.layers {
		vtl, err := l.VTileLayer(extent)
		if err != nil {
			return nil, fmt.Errorf("Error Getting VTileLayer: %v", err)
		}
		vt.Layers = append(vt.Layers, vtl)
	}
	return vt, nil
}

//TODO: What is this functions suppose to do?
//TileFromVTile will return a Tile object from the given vectorTile Tile object
func TileFromVTile(t *vectorTile.Tile) (*Tile, error) {
	return nil, nil
}
