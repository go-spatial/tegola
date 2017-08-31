package server_test

import (
	"log"

	"context"

	"github.com/terranodo/tegola"
	"github.com/terranodo/tegola/basic"
	"github.com/terranodo/tegola/mvt"
	"github.com/terranodo/tegola/server"
)

const (
	httpPort      = ":8080"
	serverVersion = "0.3.0"
)

type testMVTProvider struct{}

func (tp *testMVTProvider) MVTLayer(ctx context.Context, layerName string, tile tegola.Tile, tags map[string]interface{}) (*mvt.Layer, error) {
	var layer mvt.Layer

	return &layer, nil
}

func (tp *testMVTProvider) Layers() ([]mvt.LayerInfo, error) {
	return []mvt.LayerInfo{
		layer{
			name:     "test-layer",
			geomType: basic.Polygon{},
			srid:     tegola.WebMercator,
		},
	}, nil
}

var testLayer1 = server.Layer{
	Name:     "test-layer-1",
	MinZoom:  4,
	MaxZoom:  9,
	Provider: &testMVTProvider{},
	DefaultTags: map[string]interface{}{
		"foo": "bar",
	},
}

var testLayer2 = server.Layer{
	Name:     "test-layer-2",
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
		testLayer2,
	},
}

type layer struct {
	name     string
	geomType tegola.Geometry
	srid     int
}

func (l layer) Name() string {
	return l.name
}

func (l layer) GeomType() tegola.Geometry {
	return l.geomType
}

func (l layer) SRID() int {
	return l.srid
}

func init() {
	server.Version = serverVersion

	//	register a map with layers
	if err := server.RegisterMap(testMap); err != nil {
		log.Fatal("Failed to register test map")
	}
}
