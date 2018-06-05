package server_test

import (
	"io"
	"net/http"
	"net/http/httptest"

	"github.com/dimfeld/httptreemux"
	"github.com/go-spatial/geom"
	"github.com/go-spatial/tegola/atlas"
	"github.com/go-spatial/tegola/provider/test"
	"github.com/go-spatial/tegola/server"
)

// test server config
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
	MaxZoom:           15,
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

func newTestMapWithLayers(layers ...atlas.Layer) *atlas.Atlas {

	testMap := atlas.NewWebMercatorMap(testMapName)
	testMap.Attribution = testMapAttribution
	testMap.Center = testMapCenter
	testMap.Layers = append(testMap.Layers, layers...)

	a := &atlas.Atlas{}
	a.AddMap(testMap)

	return a
}

func doRequest(a *atlas.Atlas, method string, uri string, body io.Reader) (w *httptest.ResponseRecorder, router *httptreemux.TreeMux, err error) {

	router = server.NewRouter(a)

	// Default Method to GET
	if method == "" {
		method = "GET"
	}

	r, err := http.NewRequest(method, uri, body)
	if err != nil {
		return nil, nil, err
	}
	w = httptest.NewRecorder()
	router.ServeHTTP(w, r)
	return w, router, nil
}

// pre test setup phase
func init() {
	server.Version = serverVersion
	server.HostName = serverHostName

	testMap := atlas.NewWebMercatorMap(testMapName)
	testMap.Attribution = testMapAttribution
	testMap.Center = testMapCenter
	testMap.Layers = append(testMap.Layers,
		testLayer1,
		testLayer2,
		testLayer3,
	)

	// register a map with atlas
	atlas.AddMap(testMap)
}
