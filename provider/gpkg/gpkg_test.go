package gpkg

import (
	"testing"
	"github.com/stretchr/testify/assert"
	"fmt"
	"runtime"
	"path/filepath"
)

func TestNewGPKGProvider(t *testing.T) {
	_, filePath, _, _ := runtime.Caller(0)
	dir, _ := filepath.Split(filePath)
	var GPKGFilePath string = dir + "test_data/athens-osm-20170921.gpkg"

	fmt.Println("Using path to gpkg: ", GPKGFilePath)
	layers := map[string]layer{}
	
	config := map[string]interface{} {
		"config": GPKGFilePath,
		"layers": layers,
		"srid": 0,
	}
	p, _ := NewProvider(config)

	lys, _ := p.Layers()
	fmt.Println("p.Layers(): ", lys)
	// MVTLayer(ctx context.Context, layerName string, tile tegola.Tile, tags map[string]interface{})
	//	(*Layer, error)
	assert.Equal(t, 19, len(lys), "")
}
