package server_test

import (
	"log"

	"github.com/terranodo/tegola"
	"github.com/terranodo/tegola/mvt"
	"github.com/terranodo/tegola/server"
)

const (
	httpPort      = ":8080"
	serverVersion = "0.3.0"
)

type testMVTProvider struct{}

func (tp *testMVTProvider) MVTLayer(providerLayerName string, layerName string, tile tegola.Tile, tags map[string]interface{}) (*mvt.Layer, error) {
	var layer mvt.Layer

	return &layer, nil
}

func (tp *testMVTProvider) LayerNames() []string {
	return []string{
		"test-layer",
	}
}

var testLayer1 = server.Layer{
	Name:     "test-layer",
	MinZoom:  10,
	MaxZoom:  20,
	Provider: &testMVTProvider{},
	DefaultTags: map[string]interface{}{
		"foo": "bar",
	},
}

var testMap = server.Map{
	Name:        "test-map",
	Attribution: "test attribution",
	Center:      [3]float64{1.0, 2.0, 3.0},
	Layers: []server.Layer{
		testLayer1,
	},
}

func init() {
	server.Version = serverVersion

	//	register a map with layers
	if err := server.RegisterMap(testMap); err != nil {
		log.Fatal("Failed to register test map")
	}
}
