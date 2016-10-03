package server_test

import (
	"github.com/terranodo/tegola"
	"github.com/terranodo/tegola/mvt"
	"github.com/terranodo/tegola/server"
)

const (
	httpPort      = ":8080"
	serverVersion = "0.3.0"
)

type testMVTProvider struct{}

func (tp *testMVTProvider) MVTLayer(layerName string, tile tegola.Tile, tags map[string]interface{}) (*mvt.Layer, error) {
	var layer mvt.Layer

	return &layer, nil
}

func (tp *testMVTProvider) LayerNames() []string {
	return []string{
		"test-layer",
	}
}

func init() {
	server.Version = serverVersion
}
