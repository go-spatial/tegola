package gpkg

import (
	"context"
	"fmt"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/terranodo/tegola"
	"github.com/terranodo/tegola/maths/points"
)

var filePath string
var directory string
var GPKGFilePath string

func init() {
	_, filePath, _, _ = runtime.Caller(0)
	directory, _ = filepath.Split(filePath)
	GPKGFilePath = directory + "test_data/athens-osm-20170921.gpkg"
}

func TestNewGPKGProvider(t *testing.T) {
	layers := []map[string]interface{}{
		// With explicit id fieldname
		{"name": "a_points", "tablename": "amenities_points", "id_fieldname": "fid"},
		{"name": "r_lines", "tablename": "rail_lines", "id_fieldname": "fid"},
		// With default id fieldname
		{"name": "rd_lines", "tablename": "roads_lines"},
	}
	expectedLayerCount := len(layers)

	config := map[string]interface{}{
		"FilePath": GPKGFilePath,
		"layers":   layers,
	}
	p, err := NewProvider(config)
	assert.Nil(t, err, fmt.Sprintf("Error in call to NewProvider(): %v", err))
	lys, _ := p.Layers()
	assert.Equal(t, expectedLayerCount, len(lys), "")
}

type MockTile struct {
	tegola.TegolaTile
	bbox [4]float64
}

func (tile *MockTile) BoundingBox() tegola.BoundingBox {
	bb := tegola.BoundingBox{Minx: tile.bbox[0], Miny: tile.bbox[1], Maxx: tile.bbox[2], Maxy: tile.bbox[3]}
	return bb
}

func TestMVTLayerFiltering(t *testing.T) {
	layers := []map[string]interface{}{
		{"name": "rl_lines", "tablename": "rail_lines"},
		{"name": "rd_lines", "tablename": "roads_lines"},
	}

	config := map[string]interface{}{
		"FilePath": GPKGFilePath,
		"layers":   layers,
	}
	p, _ := NewProvider(config)

	type TestCase struct {
		ctx          context.Context
		layerName    string
		tile         tegola.TegolaTile
		tags         map[string]interface{}
		featureCount int
	}

	// The literal coordinates are in WSG:4326 which is what the test gpkg uses,
	//	convert to WebMercator, as that's what is expected in a tile bounding box
	// Y-values are swapped (origin at top left, so miny is larger than maxy,
	//	@see https://github.com/terranodo/tegola/issues/189
	// TODO: Swap them back when that's fixed.
	bboxLeftOfLayer := points.BoundingBox{20.0, 37.9431, 23.6, 37.85}
	tileLeftOfLayer := &MockTile{bbox: bboxLeftOfLayer.ConvertSrid(tegola.WGS84, tegola.WebMercator)}

	bboxContainsLayer := points.BoundingBox{23.6, 38.0, 23.8, 37.8}
	tileContainsLayer := &MockTile{bbox: bboxContainsLayer.ConvertSrid(tegola.WGS84, tegola.WebMercator)}

	testCases := []TestCase{
		// ---- Check that empty tile is returned if layer is outside tile bounding box
		// roads_lines bounding box is: [23.6655, 37.85, 23.7958, 37.9431] (see gpkg_contents table)
		TestCase{
			ctx:          context.TODO(),
			layerName:    "rd_lines",
			tile:         tileLeftOfLayer, // Left of layer
			tags:         make(map[string]interface{}),
			featureCount: 0,
		},
		// --- Check that a non-empty tile is returned if layer is inside bounding box
		// rail lines bounding box is: [23.6828, 37.8501, 23.7549, 37.9431]
		TestCase{
			ctx:          context.TODO(),
			layerName:    "rl_lines",
			tile:         tileContainsLayer, // Contains layer
			tags:         make(map[string]interface{}),
			featureCount: 187,
		},
		// --- *Note that an empty or non-empty tile may be returned in cases of partial overlap.
	}

	for i, tc := range testCases {
		resultTile, _ := p.MVTLayer(tc.ctx, tc.layerName, tc.tile, tc.tags)
		featureCount := len(resultTile.Features())
		assert.Equal(t, tc.featureCount, featureCount,
			fmt.Sprintf("Testcase[%v] - There should be %v features in this tile", i, tc.featureCount))
	}
}
