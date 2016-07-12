package postgis_test

import (
	"log"
	"testing"

	"github.com/terranodo/tegola"
	"github.com/terranodo/tegola/provider/postgis"
)

func TestNewProvider(t *testing.T) {
	config := postgis.Config{
		Host:     "localhost",
		Port:     5432,
		Database: "gdey",
		User:     "gdey",
		Layers: map[string]string{
			"buildings": "gis.zoning_base_3857",
		},
	}
	p, err := postgis.NewProvider(config)
	if err != nil {
		t.Errorf("Failed to create a new provider. %v", err)
	}
	tile := tegola.Tile{
		Z: 15,
		X: 12451,
		Y: 18527,
	}
	l, err := p.MVTLayer("buildings", tile)
	if err != nil {
		t.Errorf("Failed to create a new provider. %v", err)
	}
	log.Printf("Go to following layer %v\n", l)

}
