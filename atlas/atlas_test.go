package atlas_test

import (
	"context"

	"github.com/terranodo/tegola"
	"github.com/terranodo/tegola/atlas"
	"github.com/terranodo/tegola/basic"
	"github.com/terranodo/tegola/geom"
	"github.com/terranodo/tegola/mvt"
	"github.com/terranodo/tegola/provider"
)

type testMVTProvider struct{}

func (tp *testMVTProvider) MVTLayer(ctx context.Context, layerName string, tile *tegola.Tile, tags map[string]interface{}) (*mvt.Layer, error) {
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

type testTileProvider struct{}

func (tp *testTileProvider) TileFeatures(ctx context.Context, layer string, t provider.Tile, fn func(f *provider.Feature) error) error {
	//	get tile bounding box
	ext, _ := t.Extent()

	debugTileOutline := provider.Feature{
		ID: 0,
		Geometry: geom.Polygon{
			[][2]float64{
				[2]float64{ext[0][0], ext[0][1]}, // Minx, Miny
				[2]float64{ext[1][0], ext[0][1]}, // Maxx, Miny
				[2]float64{ext[1][0], ext[1][1]}, // Maxx, Maxy
				[2]float64{ext[0][0], ext[1][1]}, // Minx, Maxy
			},
		},
		SRID: tegola.WebMercator,
		Tags: map[string]interface{}{
			"type": "debug_buffer_outline",
		},
	}

	if err := fn(&debugTileOutline); err != nil {
		return err
	}

	return nil
}

var testLayer1 = atlas.Layer{
	Name:              "test-layer",
	ProviderLayerName: "test-layer-1",
	MinZoom:           4,
	MaxZoom:           9,
	Provider:          &testTileProvider{},
	GeomType:          basic.Point{},
	DefaultTags: map[string]interface{}{
		"foo": "bar",
	},
}

var testLayer2 = atlas.Layer{
	Name:              "test-layer-2-name",
	ProviderLayerName: "test-layer-2-provider-layer-name",
	MinZoom:           10,
	MaxZoom:           20,
	Provider:          &testTileProvider{},
	GeomType:          basic.Line{},
	DefaultTags: map[string]interface{}{
		"foo": "bar",
	},
}

var testLayer3 = atlas.Layer{
	Name:              "test-layer",
	ProviderLayerName: "test-layer-3",
	MinZoom:           10,
	MaxZoom:           20,
	Provider:          &testTileProvider{},
	GeomType:          basic.Point{},
	DefaultTags:       map[string]interface{}{},
}

var testMap = atlas.Map{
	Name:        "test-map",
	Attribution: "test attribution",
	Center:      [3]float64{1.0, 2.0, 3.0},
	Layers: []atlas.Layer{
		testLayer1,
		testLayer2,
		testLayer3,
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
