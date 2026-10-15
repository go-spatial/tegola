//go:build cgo

package gpkg_test

import (
	"context"
	"errors"
	"fmt"
	"math"
	"os"
	"reflect"
	"testing"

	"github.com/alexeydott/geom"
	"github.com/alexeydott/tegola"
	"github.com/alexeydott/tegola/dict"
	"github.com/alexeydott/tegola/provider"
	"github.com/alexeydott/tegola/provider/gpkg"
	"github.com/alexeydott/tegola/provider/test/fixture"
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

			p, err := gpkg.NewTileProvider(tc.config, nil)
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
		"custom sql with tile and layer metadata tokens": {
			config: map[string]interface{}{
				"filepath": GPKGAthensFilePath,
				"layers": []map[string]interface{}{
					{
						"name": "tokenized_points",
						"sql": `SELECT fid, geom,
							!X! AS tile_x, !Y! AS tile_y, !Z! AS tile_z, !ZOOM! AS tile_zoom,
							!PIXEL_WIDTH! AS pixel_width, !PIXEL_HEIGHT! AS pixel_height,
							!SCALE_DENOMINATOR! AS scale_denominator,
							'!ID_FIELD!' AS id_field, '!GEOM_FIELD!' AS geom_field,
							'!GEOM_TYPE!' AS geom_type
							FROM amenities_points WHERE fid = 515`,
					},
				},
			},
			expectedLayerCount: 1,
		},
		"custom sql layer with 0 rows is registered": {
			// A custom-SQL layer whose query currently matches no rows (e.g. an
			// empty table, or filter criteria that exclude everything) must not
			// prevent the provider - and therefore the whole server - from
			// starting. Keep the layer registered without an inferred geometry
			// type so it can begin serving if the query later returns data.
			config: map[string]interface{}{
				"filepath": GPKGAthensFilePath,
				"layers": []map[string]interface{}{
					{"name": "a_points", "tablename": "amenities_points"},
					{"name": "empty_layer", "sql": "SELECT fid, geom FROM amenities_points WHERE fid = -1"},
				},
			},
			expectedLayerCount: 2,
		},
		"all custom sql layers with 0 rows are registered": {
			// No rows means geometry type inference is unavailable, but it is
			// not itself a registration error for custom SQL.
			config: map[string]interface{}{
				"filepath": GPKGAthensFilePath,
				"layers": []map[string]interface{}{
					{"name": "empty_layer", "sql": "SELECT fid, geom FROM amenities_points WHERE fid = -1"},
				},
			},
			expectedLayerCount: 1,
		},
		"custom sql keeps configured id fieldname": {
			// Custom SQL layers must honor an explicit id_fieldname even after
			// a successful geometry inspection; previously the inspected branch
			// reset it back to the default 'fid'.
			config: map[string]interface{}{
				"filepath": GPKGAthensFilePath,
				"layers": []map[string]interface{}{
					{
						"name":         "a_points",
						"sql":          "SELECT osm_id, geom, amenity FROM amenities_points WHERE fid IN (515,359,273)",
						"id_fieldname": "osm_id",
					},
				},
			},
			expectedLayerCount: 1,
		},
		"custom sql keeps default id fieldname when not configured": {
			config: map[string]interface{}{
				"filepath": GPKGAthensFilePath,
				"layers": []map[string]interface{}{
					{
						"name": "a_points",
						"sql":  "SELECT fid, geom, amenity FROM amenities_points WHERE fid IN (515,359,273)",
					},
				},
			},
			expectedLayerCount: 1,
		},
		"custom sql with non-standard geometry column": {
			// geometry_fieldname lets custom SQL use any geometry column name,
			// matching the contract of the other standard providers. The layer
			// here has no matching rows in the sample (the_geom does not exist
			// in this test fixture), so it registers without inspection.
			config: map[string]interface{}{
				"filepath": GPKGAthensFilePath,
				"layers": []map[string]interface{}{
					{
						"name":               "a_points",
						"sql":                "SELECT fid, geom, amenity FROM amenities_points WHERE fid IN (515,359,273)",
						"geometry_fieldname": "geom",
					},
				},
			},
			expectedLayerCount: 1,
		},
	}

	for name, tc := range tests {
		t.Run(name, fn(tc))
	}
}

func TestCustomSQLLayerFieldnames(t *testing.T) {
	// Verifies the layer's id/geometry fieldnames after registration for the
	// three custom-SQL states: inspected, deferred, and empty result set.
	// Uses the internal Layer accessors, which expose the configured fields.
	fn := func(tc struct {
		config              dict.Dict
		expectedIDFieldname string
		expectedGeomField   string
	}) func(*testing.T) {
		return func(t *testing.T) {
			p, err := gpkg.NewProviderInternal(tc.config)
			if err != nil {
				t.Fatalf("err creating provider: %v", err)
			}
			layer, ok := p.LayerInternal("a_points")
			if !ok {
				t.Fatalf("layer a_points not registered")
			}
			if layer.IDFieldName() != tc.expectedIDFieldname {
				t.Errorf("id fieldname: expected %v got %v", tc.expectedIDFieldname, layer.IDFieldName())
			}
			if layer.GeomFieldName() != tc.expectedGeomField {
				t.Errorf("geom fieldname: expected %v got %v", tc.expectedGeomField, layer.GeomFieldName())
			}
		}
	}

	tests := map[string]struct {
		config              dict.Dict
		expectedIDFieldname string
		expectedGeomField   string
	}{
		"inspected sql keeps configured id fieldname": {
			config: dict.Dict{
				"filepath": GPKGAthensFilePath,
				"layers": []map[string]interface{}{
					{
						"name":         "a_points",
						"sql":          "SELECT osm_id, geom, amenity FROM amenities_points WHERE fid IN (515,359,273)",
						"id_fieldname": "osm_id",
					},
				},
			},
			expectedIDFieldname: "osm_id",
			expectedGeomField:   "geom",
		},
		"inspected sql default id fieldname": {
			config: dict.Dict{
				"filepath": GPKGAthensFilePath,
				"layers": []map[string]interface{}{
					{
						"name": "a_points",
						"sql":  "SELECT fid, geom, amenity FROM amenities_points WHERE fid IN (515,359,273)",
					},
				},
			},
			expectedIDFieldname: "fid",
			expectedGeomField:   "geom",
		},
		"inspected sql honors geometry_fieldname": {
			config: dict.Dict{
				"filepath": GPKGAthensFilePath,
				"layers": []map[string]interface{}{
					{
						"name":               "a_points",
						"sql":                "SELECT fid, geom, amenity FROM amenities_points WHERE fid IN (515,359,273)",
						"geometry_fieldname": "geom",
						"id_fieldname":       "fid",
					},
				},
			},
			expectedIDFieldname: "fid",
			expectedGeomField:   "geom",
		},
		"deferred sql keeps configured id fieldname": {
			config: dict.Dict{
				"filepath": GPKGAthensFilePath,
				"layers": []map[string]interface{}{
					{
						"name":         "a_points",
						"sql":          "SELECT osm_id, geom, amenity FROM amenities_points WHERE !ZOOM! >= 0",
						"id_fieldname": "osm_id",
					},
				},
			},
			expectedIDFieldname: "osm_id",
			expectedGeomField:   "geom",
		},
		"empty result sql keeps configured id fieldname": {
			config: dict.Dict{
				"filepath": GPKGAthensFilePath,
				"layers": []map[string]interface{}{
					{
						"name":         "a_points",
						"sql":          "SELECT osm_id, geom, amenity FROM amenities_points WHERE fid = -1",
						"id_fieldname": "osm_id",
					},
				},
			},
			expectedIDFieldname: "osm_id",
			expectedGeomField:   "geom",
		},
	}

	for name, tc := range tests {
		t.Run(name, fn(tc))
	}
}

func TestTileFeatures(t *testing.T) {
	type tcase struct {
		config               dict.Dict
		layerName            string
		tile                 fixture.Tile
		expectedFeatureCount int
	}

	fn := func(tc tcase) func(*testing.T) {
		return func(t *testing.T) {
			t.Parallel()

			p, err := gpkg.NewTileProvider(tc.config, nil)
			if err != nil {
				t.Fatalf("new tile, expected nil got %v", err)
				return
			}

			var featureCount int
			err = p.TileFeatures(context.TODO(), tc.layerName, &tc.tile, nil, func(f *provider.Feature) error {
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
			tile: fixture.Tile{
				SRID: tegola.WGS84,
				BufferedBounds: geom.NewExtent(
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
			tile: fixture.Tile{
				SRID: tegola.WGS84,
				BufferedBounds: geom.NewExtent(
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
			tile: fixture.Tile{
				Z:    1,
				SRID: tegola.WebMercator,
				BufferedBounds: geom.NewExtent(
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
			tile: fixture.Tile{
				Z:    0,
				SRID: tegola.WebMercator,
				BufferedBounds: geom.NewExtent(
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
			tile: fixture.Tile{
				SRID: tegola.WGS84,
				BufferedBounds: geom.NewExtent(
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
		tile         fixture.Tile
		layerName    string
		expectedTags map[uint64]map[string]interface{}
	}

	fn := func(tc tcase) func(*testing.T) {
		return func(t *testing.T) {
			p, err := gpkg.NewTileProvider(tc.config, nil)
			if err != nil {
				t.Fatalf("err creating NewTileProvider: %v", err)
				return
			}

			err = p.TileFeatures(context.TODO(), tc.layerName, &tc.tile, nil, func(f *provider.Feature) error {
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
			tile: fixture.Tile{
				BufferedBounds: geom.NewExtent(
					[2]float64{-20026376.39, -20048966.10},
					[2]float64{20026376.39, 20048966.10},
				),
				SRID: tegola.WebMercator,
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
			tile: fixture.Tile{
				BufferedBounds: geom.NewExtent(
					[2]float64{-20026376.39, -20048966.10},
					[2]float64{20026376.39, 20048966.10},
				),
				SRID: tegola.WebMercator,
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
			tile: fixture.Tile{
				BufferedBounds: geom.NewExtent(
					[2]float64{-20026376.39, -20048966.10},
					[2]float64{20026376.39, 20048966.10},
				),
				SRID: tegola.WebMercator,
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
			tile: fixture.Tile{
				BufferedBounds: geom.NewExtent(
					[2]float64{-20026376.39, -20048966.10},
					[2]float64{20026376.39, 20048966.10},
				),
				SRID: tegola.WebMercator,
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

	_ = os.Remove(NONEXISTANTFILE)
	fn := func(tc tcase) func(*testing.T) {
		return func(t *testing.T) {
			_, err := gpkg.NewTileProvider(tc.config, nil)
			if reflect.TypeOf(err) != reflect.TypeOf(tc.err) {
				t.Errorf("expected error, expected %v got %v", tc.err, err)
			}
		}
	}

	tests := map[string]tcase{
		"empty": {
			config: dict.Dict{
				gpkg.ConfigKeyFilePath: "",
			},
			err: gpkg.ErrInvalidFilePath{FilePath: ""},
		},
		"nonexistance": {
			// should not exist
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

func TestSRIDConfigAndFeatureSRID(t *testing.T) {
	config := dict.Dict{
		"filepath": GPKGAthensFilePath,
		"srid":     4326,
		"layers": []map[string]interface{}{
			{
				"name":      "rd_lines",
				"tablename": "roads_lines",
			},
			{
				"name":      "rl_lines",
				"tablename": "rail_lines",
				"srid":      3857,
			},
			{
				"name": "a_points_sql",
				"sql":  "SELECT fid, geom, amenity FROM amenities_points WHERE fid = 515",
				"srid": 4326,
			},
		},
	}

	p, err := gpkg.NewTileProvider(config, nil)
	if err != nil {
		t.Fatalf("err creating NewTileProvider: %v", err)
	}

	layers, err := p.Layers()
	if err != nil {
		t.Fatalf("err getting layers: %v", err)
	}

	for _, l := range layers {
		if l.Name() == "rd_lines" && l.SRID() != 4326 {
			t.Errorf("expected layer rd_lines SRID to be 4326, got %v", l.SRID())
		}
		if l.Name() == "rl_lines" && l.SRID() != 3857 {
			t.Errorf("expected layer rl_lines SRID override to be 3857, got %v", l.SRID())
		}
		if l.Name() == "a_points_sql" && l.SRID() != 4326 {
			t.Errorf("expected custom SQL layer a_points_sql SRID to be 4326, got %v", l.SRID())
		}
	}

	tile := fixture.Tile{
		SRID: tegola.WebMercator,
		BufferedBounds: geom.NewExtent(
			[2]float64{-20026376.39, -20048966.10},
			[2]float64{20026376.39, 20048966.10},
		),
	}

	var featureSRID uint64
	err = p.TileFeatures(context.TODO(), "rd_lines", &tile, nil, func(f *provider.Feature) error {
		featureSRID = f.SRID
		return nil
	})
	if err != nil {
		t.Fatalf("err fetching features: %v", err)
	}

	if featureSRID != 4326 {
		t.Errorf("expected feature SRID to be 4326, got %v", featureSRID)
	}
}

// TestExplicitGeometryType verifies the common geometry_type layer key
// contract for GPKG (docs/provider-contract.md): the explicit value wins
// over gpkg_contents metadata inference, it is validated at construction
// time, and for custom SQL it removes the deferred startup inspection.
func TestExplicitGeometryType(t *testing.T) {
	baseConfig := func() dict.Dict {
		return dict.Dict{
			"name":     "explicit_geom_type",
			"type":     "gpkg",
			"filepath": GPKGAthensFilePath,
			"layers": []map[string]interface{}{
				{
					"name":         "amenities_points",
					"tablename":    "amenities_points",
					"id_fieldname": "fid",
				},
			},
		}
	}

	t.Run("explicit type wins over gpkg metadata", func(t *testing.T) {
		config := baseConfig()
		config["layers"].([]map[string]interface{})[0]["geometry_type"] = "LineString"

		prov, err := gpkg.NewTileProvider(config, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		lyrs, err := prov.(provider.Layerer).Layers()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(lyrs) != 1 {
			t.Fatalf("expected 1 layer, got %d", len(lyrs))
		}
		if _, ok := lyrs[0].GeomType().(geom.LineString); !ok {
			t.Fatalf("expected explicit geom.LineString, got %T", lyrs[0].GeomType())
		}
	})

	t.Run("metadata type retained by default", func(t *testing.T) {
		prov, err := gpkg.NewTileProvider(baseConfig(), nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		lyrs, err := prov.(provider.Layerer).Layers()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if _, ok := lyrs[0].GeomType().(geom.Point); !ok {
			t.Fatalf("expected metadata geom.Point, got %T", lyrs[0].GeomType())
		}
	})

	t.Run("invalid type fails construction", func(t *testing.T) {
		config := baseConfig()
		config["layers"].([]map[string]interface{})[0]["geometry_type"] = "triangle"

		if _, err := gpkg.NewTileProvider(config, nil); err == nil {
			t.Fatal("expected error for unsupported geometry_type, got nil")
		}
	})

	t.Run("custom sql with explicit type skips deferred inspection", func(t *testing.T) {
		config := baseConfig()
		config["layers"].([]map[string]interface{})[0] = map[string]interface{}{
			"name":          "amenities_points",
			"sql":           "SELECT fid, geom FROM amenities_points WHERE !ZOOM! > 0",
			"id_fieldname":  "fid",
			"geometry_type": "Point",
		}

		prov, err := gpkg.NewTileProvider(config, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		lyrs, err := prov.(provider.Layerer).Layers()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(lyrs) != 1 {
			t.Fatalf("expected 1 layer, got %d", len(lyrs))
		}
		if _, ok := lyrs[0].GeomType().(geom.Point); !ok {
			t.Fatalf("expected explicit geom.Point, got %T", lyrs[0].GeomType())
		}
	})
}
