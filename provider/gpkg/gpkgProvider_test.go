package gpkg

import (
	"context"
	"fmt"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/terranodo/tegola"
	"github.com/terranodo/tegola/internal/assert"
	"github.com/terranodo/tegola/maths/points"
)

var filePath string
var directory string
var GPKGFilePath string
var GPKGNaturalEarthFilePath string

func init() {
	_, filePath, _, _ = runtime.Caller(0)
	directory, _ = filepath.Split(filePath)
	GPKGFilePath = directory + "test_data/athens-osm-20170921.gpkg"
	// This gpkg has zoom level information
	GPKGNaturalEarthFilePath = directory + "test_data/natural_earth_minimal.gpkg"
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
	bbox   [4]float64
	zlevel int
}

func (tile *MockTile) BoundingBox() tegola.BoundingBox {
	bb := tegola.BoundingBox{Minx: tile.bbox[0], Miny: tile.bbox[1], Maxx: tile.bbox[2], Maxy: tile.bbox[3]}
	return bb
}

func (tile *MockTile) ZLevel() int {
	return tile.zlevel
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

func TestConfigFields(t *testing.T) {
	// Checks the proper functioning of a "fields" config variable which specifies which
	//	columns of a table should be converted to tags beyond the defaults.

	// --- Get provider with tag fields specified in config.
	layers := []map[string]interface{}{
		{"name": "a_points", "tablename": "amenities_points", "id_fieldname": "fid",
			"fields": []string{"amenity", "religion", "tourism", "shop"}},
		{"name": "r_lines", "tablename": "rail_lines", "id_fieldname": "fid",
			"fields": []string{"railway", "bridge", "tunnel"}},
		{"name": "rd_lines", "tablename": "roads_lines"},
	}

	//	expectedLayerTags := map[string][]string{
	//		"a_points": []string{"religion", "tourism", "shop"},
	//		"r_lines":  []string{"railway", "bridge", "tunnel"},
	//		"rd_lines": []string{},
	//	}
	config := map[string]interface{}{
		"FilePath": GPKGFilePath,
		"layers":   layers,
	}
	p, err := NewProvider(config)
	if err != nil {
		fmt.Printf("Error creating provider: %v\n", err)
		t.FailNow()
	}

	// --- Check that features are populated
	ctx := context.TODO()
	// TODO: There's some confusion between pixel coordinates & WebMercator positions in the tile
	//	bounding box, making the smallest y-value in pos #4 instead of pos #2
	//	At some point, clean up this problem: https://github.com/terranodo/tegola/issues/189
	pixelExtentEntireWorld := [4]float64{-20026376.39, 20048966.10, 20026376.39, -20048966.10}
	mt := &MockTile{bbox: pixelExtentEntireWorld}
	tags := make(map[string]interface{})

	type TagLookupByFeatureId map[uint64]map[string]interface{}
	type TestCase struct {
		lName        string
		expectedTags TagLookupByFeatureId
	}

	testCases := []TestCase{
		{
			lName: "a_points",
			expectedTags: TagLookupByFeatureId{
				515: map[string]interface{}{
					"amenity": "boat_rental",
					"shop":    "yachts",
				},
				359: map[string]interface{}{
					"amenity": "bench",
					"tourism": "viewpoint",
				},
				273: map[string]interface{}{
					"amenity":  "place_of_worship",
					"religion": "christian",
				},
			},
		},
		// Check that without fields specified in config, no tags are provided.
		{
			lName: "rd_lines",
			expectedTags: TagLookupByFeatureId{
				1: map[string]interface{}{},
			},
		},
	}

	for i, tc := range testCases {
		l, err := p.MVTLayer(ctx, tc.lName, mt, tags)
		if err != nil {
			t.Errorf("TestCase[%v]: Error in call to p.MVTLayer(%v): %v\n", i, tc.lName, err)
		}

		var testCount int
		for _, f := range l.Features() {
			if tc.expectedTags[*f.ID] == nil {
				continue
			}

			expectedTagCount := len(tc.expectedTags[*f.ID])
			actualTagCount := len(f.Tags)
			if actualTagCount != expectedTagCount {
				t.Errorf("Testcase[%v]: ID: %v - Expecting %v tags, got %v\n",
					i, expectedTagCount, actualTagCount)
			}

			// Check that expected tags are present and their values match expected values.
			for tName, tValue := range f.Tags {
				exTagValue := tc.expectedTags[*f.ID][tName]
				if exTagValue != nil && exTagValue != tValue {
					t.Errorf("TestCase[%v]: ID: %v - %v: %v != %v\n", i, *f.ID, tName, tValue, exTagValue)
				}
			}
			testCount++
		}

		if testCount != len(tc.expectedTags) {
			t.Errorf("TestCase[%v]: Tested tags for %v features, was expecting to test %v\n",
				i, testCount, len(tc.expectedTags))
		}
	}
}

func TestConfigSQL(t *testing.T) {
	// Checks the proper functioning of a "fields" config variable which specifies which
	//	columns of a table should be converted to tags beyond the defaults.

	// --- Get provider with sql specified for layers in config.
	layers := []map[string]interface{}{
		{"name": "a_points",
			"sql": "SELECT fid, geom, amenity, religion, tourism, shop FROM amenities_points"},
		// Currently only one BBOX token is supported per query, so we need to use a subquery here
		{"name": "a_p_points",
			"sql": "SELECT * FROM (" +
				"SELECT fid, geom, NULL AS place, NULL AS is_in, amenity, religion, " +
				"  tourism, shop, si.minx, si.miny, si.maxx, si.maxy" +
				"  FROM amenities_points ap JOIN rtree_amenities_points_geom si ON ap.fid = si.id " +
				"UNION " +
				"SELECT fid, geom, place, is_in, NULL, NULL, NULL, NULL, " +
				"	  si.minx, si.miny, si.maxx, si.maxy " +
				"FROM places_points pp JOIN rtree_places_points_geom si ON pp.fid = si.id) " +
				"WHERE !BBOX!"},
	}

	config := map[string]interface{}{
		"FilePath": GPKGFilePath,
		"layers":   layers,
	}
	p, err := NewProvider(config)
	if err != nil {
		fmt.Printf("Error creating provider: %v\n", err)
		t.FailNow()
	}

	// --- Check that features are populated
	ctx := context.TODO()
	// TODO: There's some confusion between pixel coordinates & WebMercator positions in the tile
	//	bounding box, making the smallest y-value in pos #4 instead of pos #2
	//	At some point, clean up this problem: https://github.com/terranodo/tegola/issues/189
	pixelExtentEntireWorld := [4]float64{-20026376.39, 20048966.10, 20026376.39, -20048966.10}
	mt := &MockTile{bbox: pixelExtentEntireWorld}
	tags := make(map[string]interface{})

	type TagLookupByFeatureId map[uint64]map[string]interface{}
	type TestCase struct {
		lName        string
		expectedTags TagLookupByFeatureId
	}

	testCases := []TestCase{
		{
			lName: "a_points",
			expectedTags: TagLookupByFeatureId{
				515: map[string]interface{}{
					"amenity": "boat_rental",
					"shop":    "yachts",
				},
				359: map[string]interface{}{
					"amenity": "bench",
					"tourism": "viewpoint",
				},
				273: map[string]interface{}{
					"amenity":  "place_of_worship",
					"religion": "christian",
				},
			},
		},
		{
			lName: "a_p_points",
			expectedTags: TagLookupByFeatureId{
				255: map[string]interface{}{
					"amenity":  "place_of_worship",
					"religion": "christian",
				},
			},
		},
	}

	for i, tc := range testCases {
		l, err := p.MVTLayer(ctx, tc.lName, mt, tags)
		if err != nil {
			t.Errorf("TestCase[%v]: Error in call to p.MVTLayer(%v): %v\n", i, tc.lName, err)
		}

		var testCount int
		for _, f := range l.Features() {
			if tc.expectedTags[*f.ID] == nil {
				continue
			}

			expectedTagCount := len(tc.expectedTags[*f.ID])
			actualTagCount := len(f.Tags)
			if actualTagCount != expectedTagCount {
				t.Errorf("Testcase[%v]: ID: %v - Expecting %v tags, got %v\n",
					i, *f.ID, expectedTagCount, actualTagCount)
			}

			// Check that expected tags are present and their values match expected values.
			for tName, tValue := range f.Tags {
				exTagValue := tc.expectedTags[*f.ID][tName]
				if exTagValue != nil && exTagValue != tValue {
					t.Errorf("TestCase[%v]: ID: %v - %v: %v != %v\n", i, *f.ID, tName, tValue, exTagValue)
				}
			}
			testCount++
		}

		if testCount != len(tc.expectedTags) {
			t.Errorf("TestCase[%v]: Tested tags for %v features, was expecting to test %v\n",
				i, testCount, len(tc.expectedTags))
		}
	}
}

func TestConfigZOOM(t *testing.T) {
	// Checks the proper functioning of a "fields" config variable which specifies which
	//	columns of a table should be converted to tags beyond the defaults.

	// --- Get provider with sql specified for layers in config.
	layers := []map[string]interface{}{
		{"name": "land1",
			"sql": "SELECT fid, geom, featurecla, min_zoom, 22 as max_zoom, minx, miny, maxx, maxy " +
				"FROM ne_110m_land t JOIN rtree_ne_110m_land_geom si ON t.fid = si.id " +
				"WHERE !BBOX! AND !ZOOM!",
		},
		{"name": "land2",
			"sql": "SELECT fid, geom, featurecla, min_zoom, 22 as max_zoom, minx, miny, maxx, maxy " +
				"FROM ne_110m_land t JOIN rtree_ne_110m_land_geom si ON t.fid = si.id " +
				"WHERE !BBOX! AND !ZOOM!",
		},
	}

	config := map[string]interface{}{
		"FilePath": GPKGNaturalEarthFilePath,
		"layers":   layers,
	}
	p, err := NewProvider(config)
	if err != nil {
		fmt.Printf("Error creating provider: %v\n", err)
		t.FailNow()
	}

	// --- Check that features are populated
	ctx := context.TODO()
	// TODO: There's some confusion between pixel coordinates & WebMercator positions in the tile
	//	bounding box, making the smallest y-value in pos #4 instead of pos #2
	//	At some point, clean up this problem: https://github.com/terranodo/tegola/issues/189
	pixelExtentEntireWorld := [4]float64{-20026376.39, 20048966.10, 20026376.39, -20048966.10}
	mt := &MockTile{bbox: pixelExtentEntireWorld}
	tags := make(map[string]interface{})

	type TagLookupByFeatureId map[uint64]map[string]interface{}
	type TestCase struct {
		lName                string
		zlevel               int
		expectedFeatureCount int
	}

	testCases := []TestCase{
		{
			lName:                "land1",
			zlevel:               1,
			expectedFeatureCount: 101,
		},
		{
			lName:                "land2",
			zlevel:               0,
			expectedFeatureCount: 44,
		},
	}

	for i, tc := range testCases {
		mt.zlevel = tc.zlevel
		l, err := p.MVTLayer(ctx, tc.lName, mt, tags)
		if err != nil {
			t.Errorf("TestCase[%v]: Error in call to p.MVTLayer(%v): %v\n", i, tc.lName, err)
		}

		assert.Equal(t, tc.expectedFeatureCount, len(l.Features()))
	}

}
