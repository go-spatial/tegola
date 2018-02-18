// +build cgo

package gpkg_test

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/go-spatial/tegola"
	"github.com/go-spatial/tegola/provider"
	"github.com/go-spatial/tegola/provider/gpkg"
)

const (
	GPKGAthensFilePath       = "testdata/athens-osm-20170921.gpkg"
	GPKGNaturalEarthFilePath = "testdata/natural_earth_minimal.gpkg"
	GPKGPuertoMontFilePath   = "testdata/puerto_mont-osm-20170922.gpkg"
)

func init() {
	//log.SetLogLevel(log.DEBUG)
}

func TestAutoConfig(t *testing.T) {
	type tcase struct {
		gpkgPath     string
		expectedConf map[string]interface{}
	}

	fn := func(t *testing.T, tc tcase) {
		conf, err := gpkg.AutoConfig(tc.gpkgPath)
		if err != nil {
			t.Errorf("problem getting config for '%v': %v", tc.gpkgPath, err)
		}
		if !reflect.DeepEqual(conf, tc.expectedConf) {
			t.Errorf("expected %v - got %v", tc.expectedConf, conf)
		}
	}

	tests := map[string]tcase{
		"athens": {
			gpkgPath: GPKGAthensFilePath,
			expectedConf: map[string]interface{}{
				"name":     "autoconfd_gpkg",
				"type":     "gpkg",
				"filepath": GPKGAthensFilePath,
				"layers": []map[string]interface{}{
					{"name": "amenities_points", "tablename": "amenities_points", "id_fieldname": "fid"},
					{"tablename": "amenities_polygons", "id_fieldname": "fid", "name": "amenities_polygons"},
					{"id_fieldname": "fid", "name": "aviation_lines", "tablename": "aviation_lines"},
					{"name": "aviation_points", "tablename": "aviation_points", "id_fieldname": "fid"},
					{"name": "aviation_polygons", "tablename": "aviation_polygons", "id_fieldname": "fid"},
					{"name": "boundary", "tablename": "boundary", "id_fieldname": "id"},
					{"name": "buildings_polygons", "tablename": "buildings_polygons", "id_fieldname": "fid"},
					{"name": "harbours_points", "tablename": "harbours_points", "id_fieldname": "fid"},
					{"tablename": "land_polygons", "id_fieldname": "ogc_fid", "name": "land_polygons"},
					{"name": "landuse_polygons", "tablename": "landuse_polygons", "id_fieldname": "fid"},
					{"id_fieldname": "fid", "name": "leisure_polygons", "tablename": "leisure_polygons"},
					{"name": "natural_lines", "tablename": "natural_lines", "id_fieldname": "fid"},
					{"tablename": "natural_polygons", "id_fieldname": "fid", "name": "natural_polygons"},
					{"id_fieldname": "fid", "name": "places_points", "tablename": "places_points"},
					{"name": "places_polygons", "tablename": "places_polygons", "id_fieldname": "fid"},
					{"tablename": "rail_lines", "id_fieldname": "fid", "name": "rail_lines"},
					{"name": "roads_lines", "tablename": "roads_lines", "id_fieldname": "fid"},
					{"id_fieldname": "fid", "name": "towers_antennas_points", "tablename": "towers_antennas_points"},
					{"name": "waterways_lines", "tablename": "waterways_lines", "id_fieldname": "fid"},
				},
			},
		},
		"natural earth": {
			gpkgPath: GPKGNaturalEarthFilePath,
			expectedConf: map[string]interface{}{
				"name":     "autoconfd_gpkg",
				"type":     "gpkg",
				"filepath": GPKGNaturalEarthFilePath,
				"layers": []map[string]interface{}{
					{"name": "ne_110m_land", "tablename": "ne_110m_land", "id_fieldname": "fid"},
				},
			},
		},
		"puerto monte": {
			gpkgPath: GPKGPuertoMontFilePath,
			expectedConf: map[string]interface{}{
				"name":     "autoconfd_gpkg",
				"type":     "gpkg",
				"filepath": GPKGPuertoMontFilePath,
				"layers": []map[string]interface{}{
					{"name": "amenities_points", "tablename": "amenities_points", "id_fieldname": "fid"},
					{"name": "amenities_polygons", "tablename": "amenities_polygons", "id_fieldname": "fid"},
					{"name": "aviation_lines", "tablename": "aviation_lines", "id_fieldname": "fid"},
					{"name": "aviation_points", "tablename": "aviation_points", "id_fieldname": "fid"},
					{"name": "aviation_polygons", "tablename": "aviation_polygons", "id_fieldname": "fid"},
					{"name": "boundary", "tablename": "boundary", "id_fieldname": "id"},
					{"name": "buildings_polygons", "tablename": "buildings_polygons", "id_fieldname": "fid"},
					{"name": "harbours_points", "tablename": "harbours_points", "id_fieldname": "fid"},
					{"name": "land_polygons", "tablename": "land_polygons", "id_fieldname": "ogc_fid"},
					{"name": "landuse_polygons", "tablename": "landuse_polygons", "id_fieldname": "fid"},
					{"name": "leisure_polygons", "tablename": "leisure_polygons", "id_fieldname": "fid"},
					{"name": "natural_lines", "tablename": "natural_lines", "id_fieldname": "fid"},
					{"name": "natural_polygons", "tablename": "natural_polygons", "id_fieldname": "fid"},
					{"name": "places_points", "tablename": "places_points", "id_fieldname": "fid"},
					{"name": "places_polygons", "tablename": "places_polygons", "id_fieldname": "fid"},
					{"name": "rail_lines", "tablename": "rail_lines", "id_fieldname": "fid"},
					{"name": "roads_lines", "tablename": "roads_lines", "id_fieldname": "fid"},
					{"name": "towers_antennas_points", "tablename": "towers_antennas_points", "id_fieldname": "fid"},
					{"name": "waterways_lines", "tablename": "waterways_lines", "id_fieldname": "fid"},
				},
			},
		},
	}

	for tname, tc := range tests {
		tc := tc
		t.Run(tname, func(t *testing.T) {
			fn(t, tc)
		})
	}
}

func TestNewTileProvider(t *testing.T) {
	type tcase struct {
		config             map[string]interface{}
		expectedLayerCount int
		expectedErr        error
	}

	fn := func(t *testing.T, tc tcase) {
		t.Parallel()

		p, err := gpkg.NewTileProvider(tc.config)
		if err != nil {
			if err.Error() != tc.expectedErr.Error() {
				t.Errorf("expectedErr %v got %v", tc.expectedErr, err)
			}
			return
		}

		lys, err := p.Layers()
		if err != nil {
			t.Errorf("unable to fetch provider layers: %v", err)
			return
		}

		if tc.expectedLayerCount != len(lys) {
			t.Errorf("expected %v got %v", tc.expectedLayerCount, len(lys))
			return
		}
	}

	tests := map[string]tcase{
		"duplicate layer name": tcase{
			config: map[string]interface{}{
				"filepath": GPKGAthensFilePath,
				"layers": []map[string]interface{}{
					{"name": "a_points", "tablename": "amenities_points"},
					{"name": "a_points", "tablename": "amenities_points"},
				},
			},
			expectedErr: errors.New("layer name (a_points) is duplicated in both layer 1 and layer 0"),
		},
		"3 layers": tcase{
			config: map[string]interface{}{
				"filepath": GPKGAthensFilePath,
				"layers": []map[string]interface{}{
					// explicit id fieldname
					{"name": "a_points", "tablename": "amenities_points", "id_fieldname": "fid"},
					{"name": "r_lines", "tablename": "rail_lines", "id_fieldname": "fid"},
					// default id fieldname
					{"name": "rd_lines", "tablename": "roads_lines"},
				},
			},
			expectedLayerCount: 3,
		},
	}

	for name, tc := range tests {
		tc := tc
		t.Run(name, func(t *testing.T) {
			fn(t, tc)
		})
	}
}

type MockTile struct {
	extent         [2][2]float64
	bufferedExtent [2][2]float64
	Z, X, Y        uint64
	srid           uint64
}

// TODO(arolek): Extent needs to return a geom.Extent
func (t *MockTile) Extent() ([2][2]float64, uint64) {
	return t.extent, t.srid
}

// TODO(arolek): BufferedExtent needs to return a geom.Extent
func (t *MockTile) BufferedExtent() ([2][2]float64, uint64) {
	return t.bufferedExtent, t.srid
}

func (t *MockTile) ZXY() (uint64, uint64, uint64) {
	return t.Z, t.X, t.Y
}

func TestTileFeatures(t *testing.T) {
	type tcase struct {
		config               map[string]interface{}
		layerName            string
		tile                 MockTile
		expectedFeatureCount int
	}

	fn := func(t *testing.T, tc tcase) {
		t.Parallel()

		p, err := gpkg.NewTileProvider(tc.config)
		if err != nil {
			t.Fatalf("err creating NewTileProvider: %v", err)
			return
		}

		var featureCount int
		err = p.TileFeatures(context.TODO(), tc.layerName, &tc.tile, func(f *provider.Feature) error {
			featureCount++
			return nil
		})
		if err != nil {
			t.Errorf("err fetching features: %v", err)
			return
		}

		if tc.expectedFeatureCount != featureCount {
			t.Errorf("expected %v got %v", tc.expectedFeatureCount, featureCount)
			return
		}
	}

	tests := map[string]tcase{
		// roads_lines bounding box is: [23.6655, 37.85, 23.7958, 37.9431] (see gpkg_contents table)
		"tile outside layer extent": tcase{
			config: map[string]interface{}{
				"filepath": GPKGAthensFilePath,
				"layers": []map[string]interface{}{
					{"name": "rd_lines", "tablename": "roads_lines"},
				},
			},
			layerName: "rd_lines",
			tile: MockTile{
				srid: tegola.WGS84,
				bufferedExtent: [2][2]float64{
					{20.0, 37.85},
					{23.6, 37.9431},
				},
			},
			expectedFeatureCount: 0,
		},
		// rail lines bounding box is: [23.6828, 37.8501, 23.7549, 37.9431]
		"tile inside layer extent": tcase{
			config: map[string]interface{}{
				"filepath": GPKGAthensFilePath,
				"layers": []map[string]interface{}{
					{"name": "rl_lines", "tablename": "rail_lines"},
				},
			},
			layerName: "rl_lines",
			tile: MockTile{
				srid: tegola.WGS84,
				bufferedExtent: [2][2]float64{
					{23.6, 38.0},
					{23.8, 37.8},
				},
			},
			expectedFeatureCount: 187,
		},
		"zoom token": tcase{
			config: map[string]interface{}{
				"filepath": GPKGNaturalEarthFilePath,
				"layers": []map[string]interface{}{
					{
						"name": "land1",
						"sql": `
							SELECT
								fid, geom, featurecla, min_zoom, 22 as max_zoom, minx, miny, maxx, maxy
							FROM
								ne_110m_land t JOIN rtree_ne_110m_land_geom si ON t.fid = si.id
							WHERE
								!BBOX! AND min_zoom <= !ZOOM!`,
					},
				},
			},
			layerName: "land1",
			tile: MockTile{
				Z:    1,
				srid: tegola.WebMercator,
				bufferedExtent: [2][2]float64{
					{-20026376.39, -20048966.10},
					{20026376.39, 20048966.10},
				},
			},
			expectedFeatureCount: 101,
		},
		"zoom token 2": tcase{
			config: map[string]interface{}{
				"filepath": GPKGNaturalEarthFilePath,
				"layers": []map[string]interface{}{
					{
						"name": "land2",
						"sql": `
							SELECT
								fid, geom, featurecla, min_zoom, 22 as max_zoom, minx, miny, maxx, maxy
							FROM
								ne_110m_land t JOIN rtree_ne_110m_land_geom si ON t.fid = si.id
							WHERE
								!BBOX! AND min_zoom <= !ZOOM! AND max_zoom >= !ZOOM!`,
					},
				},
			},
			layerName: "land2",
			tile: MockTile{
				Z:    0,
				srid: tegola.WebMercator,
				bufferedExtent: [2][2]float64{
					{-20026376.39, -20048966.10},
					{20026376.39, 20048966.10},
				},
			},
			expectedFeatureCount: 44,
		},
	}

	for name, tc := range tests {
		tc := tc
		t.Run(name, func(t *testing.T) {
			fn(t, tc)
		})
	}
}

func TestConfigs(t *testing.T) {
	type tcase struct {
		config       map[string]interface{}
		tile         MockTile
		layerName    string
		expectedTags map[uint64]map[string]interface{}
	}

	fn := func(t *testing.T, tc tcase) {
		p, err := gpkg.NewTileProvider(tc.config)
		if err != nil {
			t.Fatalf("err creating NewTileProvider: %v", err)
			return
		}

		err = p.TileFeatures(context.TODO(), tc.layerName, &tc.tile, func(f *provider.Feature) error {
			// check if the feature is part of the test
			if _, ok := tc.expectedTags[f.ID]; !ok {
				return nil
			}

			expectedTagCount := len(tc.expectedTags[f.ID])
			actualTagCount := len(f.Tags)

			if actualTagCount != expectedTagCount {
				return fmt.Errorf("expected %v tags, got %v", expectedTagCount, actualTagCount)
			}

			// Check that expected tags are present and their values match expected values.
			for tName, tValue := range f.Tags {
				exTagValue := tc.expectedTags[f.ID][tName]
				if exTagValue != nil && exTagValue != tValue {
					return fmt.Errorf("feature ID: %v - %v: %v != %v", f.ID, tName, tValue, exTagValue)
				}
			}

			return nil
		})
		if err != nil {
			t.Errorf("err fetching features: %v", err)
			return
		}
	}

	tests := map[string]tcase{
		"expecting tags": tcase{
			config: map[string]interface{}{
				"filepath": GPKGAthensFilePath,
				"layers": []map[string]interface{}{
					{"name": "a_points", "tablename": "amenities_points", "id_fieldname": "fid", "fields": []string{"amenity", "religion", "tourism", "shop"}},
					{"name": "r_lines", "tablename": "rail_lines", "id_fieldname": "fid", "fields": []string{"railway", "bridge", "tunnel"}},
					{"name": "rd_lines", "tablename": "roads_lines"},
				},
			},
			tile: MockTile{
				bufferedExtent: [2][2]float64{
					{-20026376.39, -20048966.10},
					{20026376.39, 20048966.10},
				},
				srid: tegola.WebMercator,
			},
			layerName: "a_points",
			expectedTags: map[uint64]map[string]interface{}{
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
		"no tags provided": tcase{
			config: map[string]interface{}{
				"filepath": GPKGAthensFilePath,
				"layers": []map[string]interface{}{
					{"name": "a_points", "tablename": "amenities_points", "id_fieldname": "fid", "fields": []string{"amenity", "religion", "tourism", "shop"}},
					{"name": "r_lines", "tablename": "rail_lines", "id_fieldname": "fid", "fields": []string{"railway", "bridge", "tunnel"}},
					{"name": "rd_lines", "tablename": "roads_lines"},
				},
			},
			tile: MockTile{
				bufferedExtent: [2][2]float64{
					{-20026376.39, -20048966.10},
					{20026376.39, 20048966.10},
				},
				srid: tegola.WebMercator,
			},
			layerName: "rd_lines",
			expectedTags: map[uint64]map[string]interface{}{
				1: map[string]interface{}{},
			},
		},
		"simple sql": tcase{
			config: map[string]interface{}{
				"filepath": GPKGAthensFilePath,
				"layers": []map[string]interface{}{
					{
						"name": "a_points",
						"sql":  "SELECT fid, geom, amenity, religion, tourism, shop FROM amenities_points WHERE fid IN (515,359,273)",
					},
				},
			},
			tile: MockTile{
				bufferedExtent: [2][2]float64{
					{-20026376.39, -20048966.10},
					{20026376.39, 20048966.10},
				},
				srid: tegola.WebMercator,
			},
			layerName: "a_points",
			expectedTags: map[uint64]map[string]interface{}{
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
		"complex sql": tcase{
			config: map[string]interface{}{
				"filepath": GPKGAthensFilePath,
				"layers": []map[string]interface{}{
					{
						"name": "a_p_points",
						"sql": `
							SELECT * FROM (
								SELECT
									fid, geom, NULL AS place, NULL AS is_in, amenity, religion, tourism, shop, si.minx, si.miny, si.maxx, si.maxy
								FROM
									amenities_points ap JOIN rtree_amenities_points_geom si ON ap.fid = si.id
								UNION
									SELECT
										fid, geom, place, is_in, NULL, NULL, NULL, NULL, si.minx, si.miny, si.maxx, si.maxy
									FROM
										places_points pp JOIN rtree_places_points_geom si ON pp.fid = si.id
							)
							WHERE !BBOX!`,
					},
				},
			},
			tile: MockTile{
				bufferedExtent: [2][2]float64{
					{-20026376.39, -20048966.10},
					{20026376.39, 20048966.10},
				},
				srid: tegola.WebMercator,
			},
			layerName: "a_p_points",
			expectedTags: map[uint64]map[string]interface{}{
				255: map[string]interface{}{
					"amenity":  "place_of_worship",
					"religion": "christian",
				},
			},
		},
	}

	for name, tc := range tests {
		tc := tc
		t.Run(name, func(t *testing.T) {
			fn(t, tc)
		})
	}
}
