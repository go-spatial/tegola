package server_test

import (
	"github.com/terranodo/tegola/atlas"
	"github.com/terranodo/tegola/cache/memorycache"
	"github.com/terranodo/tegola/geom"
	"github.com/terranodo/tegola/provider/test"
	"github.com/terranodo/tegola/server"
)

//	test server config
const (
	httpPort       = ":8080"
	serverVersion  = "0.4.0"
	serverHostName = "tegola.io"
)

var (
	testMapName        = "test-map"
	testMapAttribution = "test attribution"
	testMapCenter      = [3]float64{1.0, 2.0, 3.0}
)

var testLayer1 = atlas.Layer{
	Name:              "test-layer",
	ProviderLayerName: "test-layer-1",
	MinZoom:           4,
	MaxZoom:           9,
	Provider:          &test.TileProvider{},
	GeomType:          geom.Point{},
	DefaultTags: map[string]interface{}{
		"foo": "bar",
	},
}

var testLayer2 = atlas.Layer{
	Name:              "test-layer-2-name",
	ProviderLayerName: "test-layer-2-provider-layer-name",
	MinZoom:           10,
	MaxZoom:           20,
	Provider:          &test.TileProvider{},
	GeomType:          geom.Line{},
	DefaultTags: map[string]interface{}{
		"foo": "bar",
	},
}

var testLayer3 = atlas.Layer{
	Name:              "test-layer",
	ProviderLayerName: "test-layer-3",
	MinZoom:           10,
	MaxZoom:           20,
	Provider:          &test.TileProvider{},
	GeomType:          geom.Point{},
	DefaultTags:       map[string]interface{}{},
}

//	pre test setup phase
func init() {
	server.Version = serverVersion
	server.HostName = serverHostName

	testMap := atlas.NewWebMercatorMap(testMapName)
	testMap.Attribution = testMapAttribution
	testMap.Center = testMapCenter
	testMap.Layers = append(testMap.Layers, []atlas.Layer{
		testLayer1,
		testLayer2,
		testLayer3,
	}...)

	atlas.SetCache(memorycache.New())

	//	register a map with atlas
	atlas.AddMap(testMap)

	server.Atlas = atlas.DefaultAtlas
}
