// +build cgo

package gpkg_test

import (
	"context"
	"errors"
	"fmt"
	"math"
	"os"
	"reflect"
	"testing"

	"github.com/go-spatial/geom"
	"github.com/go-spatial/tegola/dict"
	"github.com/go-spatial/tegola/proj"
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

func confEqual(t *testing.T, conf, expectedConf map[string]interface{}) bool {
	equal := true

	confKeys := make([]string, 0, len(conf))
	for k := range conf {
		confKeys = append(confKeys, k)
	}

	exKeys := make([]string, 0, len(expectedConf))
	for k := range expectedConf {
		exKeys = append(exKeys, k)
	}

	if len(confKeys) != len(exKeys) {
		t.Errorf("Configs have different number of parameters: %v != %v", confKeys, exKeys)
		equal = false
	}

	for k, v := range conf {
		if k != "layers" {
			if v != expectedConf[k] {
				t.Errorf(`"%v": %v != %v`, k, v, expectedConf[k])
				equal = false
			}
		} else {
			lconf := v.([]map[string]interface{})
			econf := expectedConf["layers"].([]map[string]interface{})

			// safeLength is the smaller of these two lengths
			safeLength := len(lconf)
			if len(lconf) != len(econf) {
				t.Errorf("Layer configs have different lengths: %v != %v", len(lconf), len(econf))
				equal = false
				safeLength = int(math.Min(float64(len(lconf)), float64(len(econf))))
			}

			for i := 0; i < safeLength; i++ {
				if !reflect.DeepEqual(lconf[i], econf[i]) {
					t.Errorf("layer conf [%v]: %v != %v", i, lconf[i], econf[i])
					equal = false
				}
			}
		}
	}

	return equal
}

func TestAutoConfig(t *testing.T) {
	type tcase struct {
		gpkgPath     string
		expectedConf map[string]interface{}
	}

	fn := func(tc tcase) func(*testing.T) {
		return func(t *testing.T) {
			conf, err := gpkg.AutoConfig(tc.gpkgPath)
			if err != nil {
				t.Errorf("problem getting config for '%v': %v", tc.gpkgPath, err)
			}

			if !confEqual(t, conf, tc.expectedConf) {
				t.Errorf("config doesn't match expected")
			}
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
					{"name": "amenities_points", "tablename": "amenities_points", "id_fieldname": "fid", "fields": []string{"addr:housenumber", "addr:street", "amenity", "building", "historic", "information", "leisure", "name", "office", "osm_id", "religion", "shop", "tourism"}},
					{"name": "amenities_polygons", "tablename": "amenities_polygons", "id_fieldname": "fid", "fields": []string{"addr:housenumber", "addr:street", "amenity", "building", "historic", "information", "leisure", "name", "office", "osm_id", "osm_way_id", "religion", "shop", "tourism"}},
					{"name": "aviation_lines", "tablename": "aviation_lines", "id_fieldname": "fid", "fields": []string{"aeroway", "building", "iata", "icao", "name", "osm_id", "source", "surface", "type"}},
					{"name": "aviation_points", "id_fieldname": "fid", "fields": []string{"aeroway", "building", "iata", "icao", "name", "osm_id", "source", "surface", "type"}, "tablename": "aviation_points"},
					{"name": "aviation_polygons", "tablename": "aviation_polygons", "id_fieldname": "fid", "fields": []string{"aeroway", "building", "iata", "icao", "name", "osm_id", "osm_way_id", "source", "surface", "type"}},
					{"name": "boundary", "tablename": "boundary", "id_fieldname": "id", "fields": []string{}},
					{"name": "buildings_polygons", "tablename": "buildings_polygons", "id_fieldname": "fid", "fields": []string{"addr:housenumber", "addr:street", "building", "hazard_prone", "name", "osm_id", "osm_way_id"}},
					{"name": "harbours_points", "tablename": "harbours_points", "id_fieldname": "fid", "fields": []string{"harbour", "landuse", "leisure", "name", "osm_id"}},
					{"name": "land_polygons", "tablename": "land_polygons", "id_fieldname": "ogc_fid", "fields": []string{"fid"}},
					{"name": "landuse_polygons", "tablename": "landuse_polygons", "id_fieldname": "fid", "fields": []string{"landuse", "name", "osm_id", "osm_way_id"}},
					{"name": "leisure_polygons", "fields": []string{"leisure", "name", "osm_id", "osm_way_id"}, "tablename": "leisure_polygons", "id_fieldname": "fid"},
					{"name": "natural_lines", "tablename": "natural_lines", "id_fieldname": "fid", "fields": []string{"hazard_prone", "name", "natural", "osm_id"}},
					{"name": "natural_polygons", "id_fieldname": "fid", "fields": []string{"hazard_prone", "name", "natural", "osm_id", "osm_way_id"}, "tablename": "natural_polygons"},
					{"name": "places_points", "fields": []string{"is_in", "name", "osm_id", "place"}, "tablename": "places_points", "id_fieldname": "fid"},
					{"name": "places_polygons", "tablename": "places_polygons", "id_fieldname": "fid", "fields": []string{"is_in", "name", "osm_id", "osm_way_id", "place"}},
					{"name": "rail_lines", "tablename": "rail_lines", "id_fieldname": "fid", "fields": []string{"bridge", "cutting", "embankment", "frequency", "layer", "name", "operator", "osm_id", "railway", "service", "source", "tracks", "tunnel", "usage", "voltage", "z_index"}},
					{"name": "roads_lines", "tablename": "roads_lines", "id_fieldname": "fid", "fields": []string{"barrier", "bicycle_road", "ford", "hazard_prone", "highway", "layer", "name", "osm_id", "traffic_calming", "tunnel", "z_index"}},
					{"name": "towers_antennas_points", "id_fieldname": "fid", "fields": []string{"man_made", "name", "osm_id"}, "tablename": "towers_antennas_points"},
					{"name": "waterways_lines", "fields": []string{"hazard_prone", "name", "osm_id", "waterway"}, "tablename": "waterways_lines", "id_fieldname": "fid"},
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
					{"name": "ne_110m_land", "tablename": "ne_110m_land", "id_fieldname": "fid", "fields": []string{"featurecla", "min_zoom", "scalerank"}},
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
					{"name": "amenities_points", "tablename": "amenities_points", "id_fieldname": "fid", "fields": []string{"addr:housenumber", "addr:street", "amenity", "building", "historic", "information", "leisure", "name", "office", "osm_id", "shop", "tourism"}},
					{"name": "amenities_polygons", "tablename": "amenities_polygons", "id_fieldname": "fid", "fields": []string{"addr:housenumber", "addr:street", "amenity", "building", "historic", "information", "leisure", "name", "office", "osm_id", "osm_way_id", "shop", "tourism"}},
					{"name": "aviation_lines", "tablename": "aviation_lines", "id_fieldname": "fid", "fields": []string{"aeroway", "building", "iata", "icao", "name", "osm_id", "source", "surface", "type"}},
					{"name": "aviation_points", "tablename": "aviation_points", "id_fieldname": "fid", "fields": []string{"aeroway", "building", "iata", "icao", "name", "osm_id", "source", "surface", "type"}},
					{"name": "aviation_polygons", "tablename": "aviation_polygons", "id_fieldname": "fid", "fields": []string{"aeroway", "building", "iata", "icao", "name", "osm_id", "osm_way_id", "source", "surface", "type"}},
					{"name": "boundary", "fields": []string{}, "tablename": "boundary", "id_fieldname": "id"},
					{"name": "buildings_polygons", "tablename": "buildings_polygons", "id_fieldname": "fid", "fields": []string{"addr:housenumber", "addr:street", "building", "hazard_prone", "name", "osm_id", "osm_way_id"}},
					{"name": "harbours_points", "id_fieldname": "fid", "fields": []string{"harbour", "landuse", "leisure", "name", "osm_id"}, "tablename": "harbours_points"},
					{"name": "land_polygons", "fields": []string{"fid"}, "tablename": "land_polygons", "id_fieldname": "ogc_fid"},
					{"name": "landuse_polygons", "tablename": "landuse_polygons", "id_fieldname": "fid", "fields": []string{"landuse", "name", "osm_id", "osm_way_id"}},
					{"name": "leisure_polygons", "tablename": "leisure_polygons", "id_fieldname": "fid", "fields": []string{"leisure", "name", "osm_id", "osm_way_id"}},
					{"name": "natural_lines", "tablename": "natural_lines", "id_fieldname": "fid", "fields": []string{"hazard_prone", "name", "natural", "osm_id"}},
					{"name": "natural_polygons", "tablename": "natural_polygons", "id_fieldname": "fid", "fields": []string{"hazard_prone", "name", "natural", "osm_id", "osm_way_id"}},
					{"name": "places_points", "tablename": "places_points", "id_fieldname": "fid", "fields": []string{"is_in", "name", "osm_id", "place"}},
					{"name": "places_polygons", "id_fieldname": "fid", "fields": []string{"is_in", "name", "osm_id", "osm_way_id", "place"}, "tablename": "places_polygons"},
					{"name": "rail_lines", "fields": []string{"bridge", "cutting", "embankment", "frequency", "layer", "name", "operator", "osm_id", "railway", "service", "source", "tracks", "tunnel", "usage", "voltage", "z_index"}, "tablename": "rail_lines", "id_fieldname": "fid"},
					{"name": "roads_lines", "tablename": "roads_lines", "id_fieldname": "fid", "fields": []string{"barrier", "bicycle_road", "ford", "hazard_prone", "highway", "layer", "name", "osm_id", "traffic_calming", "tunnel", "z_index"}},
					{"name": "towers_antennas_points", "fields": []string{"man_made", "name", "osm_id"}, "tablename": "towers_antennas_points", "id_fieldname": "fid"},
					{"name": "waterways_lines", "tablename": "waterways_lines", "id_fieldname": "fid", "fields": []string{"hazard_prone", "name", "osm_id", "waterway"}},
				},
			},
		},
	}

	for tname, tc := range tests {
		t.Run(tname, fn(tc))
	}
}

func TestNewTileProvider(t *testing.T) {
	type tcase struct {
		config             dict.Dict
		expectedLayerCount int
		expectedErr        error
	}

	fn := func(tc tcase) func(*testing.T) {
		return func(t *testing.T) {
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
	}

	tests := map[string]tcase{
		"duplicate layer name": {
			config: map[string]interface{}{
				"filepath": GPKGAthensFilePath,
				"layers": []map[string]interface{}{
					{"name": "a_points", "tablename": "amenities_points"},
					{"name": "a_points", "tablename": "amenities_points"},
				},
			},
			expectedErr: errors.New("layer name (a_points) is duplicated in both layer 1 and layer 0"),
		},
		"3 layers": {
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
		t.Run(name, fn(tc))
	}
}

type MockTile struct {
	extent         *geom.Extent
	bufferedExtent *geom.Extent
	Z, X, Y        uint
	srid           uint64
}

// TODO(arolek): Extent needs to return a geom.Extent
func (t *MockTile) Extent() (*geom.Extent, uint64) { return t.extent, t.srid }

// TODO(arolek): BufferedExtent needs to return a geom.Extent
func (t *MockTile) BufferedExtent() (*geom.Extent, uint64) { return t.bufferedExtent, t.srid }

func (t *MockTile) ZXY() (uint, uint, uint) { return t.Z, t.X, t.Y }

func TestTileFeatures(t *testing.T) {
	type tcase struct {
		config               dict.Dict
		layerName            string
		tile                 MockTile
		expectedFeatureCount int
	}

	fn := func(tc tcase) func(*testing.T) {
		return func(t *testing.T) {
			t.Parallel()

			p, err := gpkg.NewTileProvider(tc.config)
			if err != nil {
				t.Fatalf("new tile, expected nil got %v", err)
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
	}

	tests := map[string]tcase{
		// roads_lines bounding box is: [23.6655, 37.85, 23.7958, 37.9431] (see gpkg_contents table)
		"tile outside layer extent": {
			config: map[string]interface{}{
				"filepath": GPKGAthensFilePath,
				"layers": []map[string]interface{}{
					{"name": "rd_lines", "tablename": "roads_lines"},
				},
			},
			layerName: "rd_lines",
			tile: MockTile{
				srid: proj.WGS84,
				bufferedExtent: geom.NewExtent(
					[2]float64{20.0, 37.85},
					[2]float64{23.6, 37.9431},
				),
			},
			expectedFeatureCount: 0,
		},
		// rail lines bounding box is: [23.6828, 37.8501, 23.7549, 37.9431]
		"tile inside layer extent": {
			config: map[string]interface{}{
				"filepath": GPKGAthensFilePath,
				"layers": []map[string]interface{}{
					{"name": "rl_lines", "tablename": "rail_lines"},
				},
			},
			layerName: "rl_lines",
			tile: MockTile{
				srid: proj.WGS84,
				bufferedExtent: geom.NewExtent(
					[2]float64{23.6, 37.8},
					[2]float64{23.8, 38.0},
				),
			},
			expectedFeatureCount: 187,
		},
		"zoom token": {
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
				srid: proj.WebMercator,
				bufferedExtent: geom.NewExtent(
					[2]float64{-20026376.39, -20048966.10},
					[2]float64{20026376.39, 20048966.10},
				),
			},
			expectedFeatureCount: 101,
		},
		"zoom token 2": {
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
				srid: proj.WebMercator,
				bufferedExtent: geom.NewExtent(
					[2]float64{-20026376.39, -20048966.10},
					[2]float64{20026376.39, 20048966.10},
				),
			},
			expectedFeatureCount: 44,
		},
		"join with ambiguous column name (id in data and index)": {
			config: map[string]interface{}{
				"filepath": GPKGAthensFilePath,
				"layers": []map[string]interface{}{
					{"name": "boundary", "tablename": "boundary", "id_fieldname": "id"},
				},
			},
			layerName: "boundary",
			tile: MockTile{
				srid: proj.WGS84,
				bufferedExtent: geom.NewExtent(
					[2]float64{20.0, 37.85},
					[2]float64{23.6, 37.9431},
				),
			},
			expectedFeatureCount: 0,
		},
	}

	for name, tc := range tests {
		t.Run(name, fn(tc))
	}
}

func TestConfigs(t *testing.T) {
	type tcase struct {
		config       dict.Dict
		tile         MockTile
		layerName    string
		expectedTags map[uint64]map[string]interface{}
	}

	fn := func(tc tcase) func(*testing.T) {
		return func(t *testing.T) {
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
	}

	tests := map[string]tcase{
		"expecting tags": {
			config: dict.Dict{
				"filepath": GPKGAthensFilePath,
				"layers": []map[string]interface{}{
					{"name": "a_points", "tablename": "amenities_points", "id_fieldname": "fid", "fields": []string{"amenity", "religion", "tourism", "shop"}},
					{"name": "r_lines", "tablename": "rail_lines", "id_fieldname": "fid", "fields": []string{"railway", "bridge", "tunnel"}},
					{"name": "rd_lines", "tablename": "roads_lines"},
				},
			},
			tile: MockTile{
				bufferedExtent: geom.NewExtent(
					[2]float64{-20026376.39, -20048966.10},
					[2]float64{20026376.39, 20048966.10},
				),
				srid: proj.WebMercator,
			},
			layerName: "a_points",
			expectedTags: map[uint64]map[string]interface{}{
				515: {
					"amenity": "boat_rental",
					"shop":    "yachts",
				},
				359: {
					"amenity": "bench",
					"tourism": "viewpoint",
				},
				273: {
					"amenity":  "place_of_worship",
					"religion": "christian",
				},
			},
		},
		"no tags provided": {
			config: dict.Dict{
				"filepath": GPKGAthensFilePath,
				"layers": []map[string]interface{}{
					{"name": "a_points", "tablename": "amenities_points", "id_fieldname": "fid", "fields": []string{"amenity", "religion", "tourism", "shop"}},
					{"name": "r_lines", "tablename": "rail_lines", "id_fieldname": "fid", "fields": []string{"railway", "bridge", "tunnel"}},
					{"name": "rd_lines", "tablename": "roads_lines"},
				},
			},
			tile: MockTile{
				bufferedExtent: geom.NewExtent(
					[2]float64{-20026376.39, -20048966.10},
					[2]float64{20026376.39, 20048966.10},
				),
				srid: proj.WebMercator,
			},
			layerName: "rd_lines",
			expectedTags: map[uint64]map[string]interface{}{
				1: {},
			},
		},
		"simple sql": {
			config: dict.Dict{
				"filepath": GPKGAthensFilePath,
				"layers": []map[string]interface{}{
					{
						"name": "a_points",
						"sql":  "SELECT fid, geom, amenity, religion, tourism, shop FROM amenities_points WHERE fid IN (515,359,273)",
					},
				},
			},
			tile: MockTile{
				bufferedExtent: geom.NewExtent(
					[2]float64{-20026376.39, -20048966.10},
					[2]float64{20026376.39, 20048966.10},
				),
				srid: proj.WebMercator,
			},
			layerName: "a_points",
			expectedTags: map[uint64]map[string]interface{}{
				515: {
					"amenity": "boat_rental",
					"shop":    "yachts",
				},
				359: {
					"amenity": "bench",
					"tourism": "viewpoint",
				},
				273: {
					"amenity":  "place_of_worship",
					"religion": "christian",
				},
			},
		},
		"complex sql": {
			config: dict.Dict{
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
				bufferedExtent: geom.NewExtent(
					[2]float64{-20026376.39, -20048966.10},
					[2]float64{20026376.39, 20048966.10},
				),
				srid: proj.WebMercator,
			},
			layerName: "a_p_points",
			expectedTags: map[uint64]map[string]interface{}{
				255: {
					"amenity":  "place_of_worship",
					"religion": "christian",
				},
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, fn(tc))
	}
}

// This is just to test that if we open a non-existant file.
func TestOpenNonExistantFile(t *testing.T) {

	type tcase struct {
		config dict.Dict
		err    error
	}

	const (
		NONEXISTANTFILE = "testdata/nonexistant.gpkg"
	)

	os.Remove(NONEXISTANTFILE)
	fn := func(tc tcase) func(*testing.T) {
		return func(t *testing.T) {
			_, err := gpkg.NewTileProvider(tc.config)
			if reflect.TypeOf(err) != reflect.TypeOf(tc.err) {
				t.Errorf("expected error, expected %v got %v", tc.err, err)
			}
		}
	}

	tests := map[string]tcase{
		"empty": tcase{
			config: dict.Dict{
				gpkg.ConfigKeyFilePath: "",
			},
			err: gpkg.ErrInvalidFilePath{FilePath: ""},
		},
		"nonexistance": tcase{
			// should not exists
			config: dict.Dict{
				gpkg.ConfigKeyFilePath: NONEXISTANTFILE,
			},
			err: gpkg.ErrInvalidFilePath{FilePath: NONEXISTANTFILE},
		},
	}

	for k, tc := range tests {
		t.Run(k, fn(tc))
	}

}
