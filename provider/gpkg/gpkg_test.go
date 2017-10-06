package gpkg_test

import (
	"testing"
	"github.com/terranodo/tegola/provider/gpkg"
)

func TestNewGPKGProvider(t *testing.T) {
	if os.Getenv("RUN_GPKG_TEST") != "yes" {
		return
	}

	filepath := gpkg.Name
	layers := map[string]layer{}
	
	config := gpkg.GPKGProvider{
		c: gpkg.Name,
		layers: layers,
		srid: 0,
	}
	p, err = gpkg.NewProvider(config)
}
