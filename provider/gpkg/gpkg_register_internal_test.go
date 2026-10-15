//go:build cgo

package gpkg

import (
	"database/sql"
	"fmt"
	"reflect"
	"testing"

	"github.com/alexeydott/geom"
	"github.com/alexeydott/geom/cmp"
	"github.com/alexeydott/tegola/dict"

	_ "github.com/mattn/go-sqlite3"
)

var (
	GPKGAthensFilePath       = "testdata/athens-osm-20170921.gpkg"
	GPKGNaturalEarthFilePath = "testdata/natural_earth_minimal.gpkg"
	GPKGPuertoMontFilePath   = "testdata/puerto_mont-osm-20170922.gpkg"
	GPKGApeldoornFilePath    = "testdata/apeldoorn.gpkg"
)

func TestTableColumnsAndPK(t *testing.T) {
	type tcase struct {
		tableName    string
		createSQL    string
		expectedCols []string
		expectedPK   string
		expectErr    bool
	}

	fn := func(tc tcase) func(*testing.T) {
		return func(t *testing.T) {
			db, err := sql.Open("sqlite3", ":memory:")
			if err != nil {
				t.Fatalf("problem opening in-memory db: %v", err)
			}
			defer func() { _ = db.Close() }()

			if _, err := db.Exec(tc.createSQL); err != nil {
				t.Fatalf("problem creating table: %v", err)
			}

			cols, pk, err := tableColumnsAndPK(db, tc.tableName)
			if tc.expectErr {
				if err == nil {
					t.Fatalf("tableColumnsAndPK: expected error got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("tableColumnsAndPK: expected nil got %v", err)
			}
			if !reflect.DeepEqual(cols, tc.expectedCols) {
				t.Errorf("extract col, expected %v got %v", tc.expectedCols, cols)
			}
			if len(pk) != 1 || pk[0] != tc.expectedPK {
				t.Errorf("extract PK, expected %v got %v", tc.expectedPK, pk)
			}
		}
	}

	tests := map[string]tcase{
		"athens_boundary": {
			tableName: "boundary",
			createSQL: `CREATE TABLE 'boundary' ( "id" INTEGER PRIMARY KEY AUTOINCREMENT, "geom" MULTIPOLYGON)`,
			expectedCols: []string{
				"geom",
				"id",
			},
			expectedPK: "id",
		},
		"column without quotes": {
			tableName: "tablename",
			createSQL: `CREATE TABLE tablename ( id INTEGER primary Key AUTOINCREMENT, "geom" BLOB, value REAL)`,
			expectedCols: []string{
				"geom",
				"id",
				"value",
			},
			expectedPK: "id",
		},
		"column with defaults": {
			tableName: "tablename2",
			createSQL: `CREATE TABLE tablename2 ( "id" INTEGER PRIMARY KEY AUTOINCREMENT, "geom" BLOB, timestamp DATE DEFAULT (datetime('now','localtime')) , value REAL)`,
			expectedCols: []string{
				"geom",
				"id",
				"timestamp",
				"value",
			},
			expectedPK: "id",
		},
		"multi_line": {
			tableName: "gps_points",
			createSQL: "CREATE TABLE \"gps_points\" (\n  \"id\" INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,\n  \"geom\" POINT,\n  \"Lat\" REAL,\n  \"Lon\" REAL\n)",
			expectedCols: []string{
				"Lat",
				"Lon",
				"geom",
				"id",
			},
			expectedPK: "id",
		},
		"missing table": {
			tableName: "does_not_exist",
			createSQL: `CREATE TABLE tablename3 ( id INTEGER PRIMARY KEY, "geom" BLOB)`,
			expectErr: true,
		},
	}

	for testName, tc := range tests {
		t.Run(testName, fn(tc))
	}
}
func ftdEqual(tablename string, ftd, ftdExpected featureTableDetails) (bool, string) {
	if !reflect.DeepEqual(ftdExpected.colNames, ftd.colNames) {
		return false, fmt.Sprintf("%v colNames, expected %v got %v", tablename, ftdExpected.colNames, ftd.colNames)
	}
	if ftdExpected.idFieldname != ftd.idFieldname {
		return false, fmt.Sprintf("%v idFieldname, expected %v got %v", tablename, ftdExpected.idFieldname, ftd.idFieldname)
	}
	if ftdExpected.geomFieldname != ftd.geomFieldname {
		return false, fmt.Sprintf("%v geomFieldname, expected %v got %v", tablename, ftdExpected.geomFieldname, ftd.geomFieldname)
	}
	// Since it's used as a type indicator, we don't care about the content of this geometry.
	if fmt.Sprintf("%T", ftdExpected.geomType) != fmt.Sprintf("%T", ftd.geomType) {
		return false, fmt.Sprintf("%v geomType, expected %T got %T", tablename, ftdExpected.geomType, ftd.geomType)
	}
	if ftdExpected.srid != ftd.srid {
		return false, fmt.Sprintf("%v srid, expected %v got %v", tablename, ftdExpected.srid, ftd.srid)
	}
	if !cmp.GeomExtent(ftdExpected.bbox, ftd.bbox) {
		return false, fmt.Sprintf("%v bbox, expected %v got %v", tablename, ftdExpected.bbox, ftd.bbox)
	}
	return true, ""
}

func TestFeatureTableMetaData(t *testing.T) {
	type tcase struct {
		gpkgPath    string
		expectedFtd map[string]featureTableDetails
	}

	fn := func(tc tcase) func(*testing.T) {
		return func(t *testing.T) {
			db, err := sql.Open("sqlite3", tc.gpkgPath)
			if err != nil {
				panic(fmt.Sprintf("problem opening gpkg %v, err: %v", tc.gpkgPath, err))
			}
			defer func() { _ = db.Close() }()

			ftmd, err := featureTableMetaData(db)
			if err != nil {
				t.Errorf("unexpected error, expected nil got %v", err)
			}

			for tname, ftds := range ftmd {
				expectedFtd, ok := tc.expectedFtd[tname]
				if !ok {
					t.Errorf(" %v in featureTableDetails, expected true got false", tname)
					continue
				}
				if len(ftds) != 1 {
					t.Errorf("%v geometry columns = %v, expected 1 (fixtures carry a single geometry column per table)", tname, len(ftds))
				}
				for _, ftd := range ftds {
					if pass, errstr := ftdEqual(tname, ftd, expectedFtd); !pass {
						t.Error(errstr)
					}
				}
			}
		}
	}

	tests := map[string]tcase{
		"athens": {
			gpkgPath: GPKGAthensFilePath,
			expectedFtd: map[string]featureTableDetails{
				"amenities_points": {
					idFieldname:   "fid",
					geomFieldname: "geom",
					geomType:      geom.Point{},
					srid:          4326,
					bbox: geom.NewExtent(
						[2]float64{23.683, 37.8502},
						[2]float64{23.7783, 37.9431},
					),
					colNames: []string{
						"addr:housenumber",
						"addr:street",
						"amenity",
						"building",
						"fid",
						"geom",
						"historic",
						"information",
						"leisure",
						"name",
						"office",
						"osm_id",
						"religion",
						"shop",
						"tourism",
					},
				},
				"amenities_polygons": {
					idFieldname:   "fid",
					geomFieldname: "geom",
					geomType:      geom.MultiPolygon{},
					srid:          4326,
					bbox: geom.NewExtent(
						[2]float64{23.6698, 37.8502},
						[2]float64{23.7815, 37.9431},
					),
					colNames: []string{
						"addr:housenumber",
						"addr:street",
						"amenity",
						"building",
						"fid",
						"geom",
						"historic",
						"information",
						"leisure",
						"name",
						"office",
						"osm_id",
						"osm_way_id",
						"religion",
						"shop",
						"tourism",
					},
				},
				"aviation_lines": {
					idFieldname:   "fid",
					geomFieldname: "geom",
					geomType:      geom.MultiLineString{},
					srid:          4326,
					bbox: geom.NewExtent(
						[2]float64{23.7231, 37.8739},
						[2]float64{23.7422, 37.9036},
					),
					colNames: []string{
						"aeroway",
						"building",
						"fid",
						"geom",
						"iata",
						"icao",
						"name",
						"osm_id",
						"source",
						"surface",
						"type",
					},
				},
				"aviation_points": {
					idFieldname:   "fid",
					geomFieldname: "geom",
					geomType:      geom.Point{},
					srid:          4326,
					bbox: geom.NewExtent(
						[2]float64{23.6802, 37.9354},
						[2]float64{23.6802, 37.9354},
					),
					colNames: []string{
						"aeroway",
						"building",
						"fid",
						"geom",
						"iata",
						"icao",
						"name",
						"osm_id",
						"source",
						"surface",
						"type",
					},
				},
				"aviation_polygons": {
					idFieldname:   "fid",
					geomFieldname: "geom",
					geomType:      geom.MultiPolygon{},
					srid:          4326,
					bbox: geom.NewExtent(
						[2]float64{23.6698, 37.8842},
						[2]float64{23.7414, 37.9391},
					),
					colNames: []string{
						"aeroway",
						"building",
						"fid",
						"geom",
						"iata",
						"icao",
						"name",
						"osm_id",
						"osm_way_id",
						"source",
						"surface",
						"type",
					},
				},
				"boundary": {
					idFieldname:   "id",
					geomFieldname: "geom",
					geomType:      geom.MultiPolygon{},
					srid:          4326,
					bbox: geom.NewExtent(
						[2]float64{23.6654, 37.85},
						[2]float64{23.7958, 37.9431},
					),
					colNames: []string{
						"geom",
						"id",
					},
				},
				"buildings_polygons": {
					idFieldname:   "fid",
					geomFieldname: "geom",
					geomType:      geom.MultiPolygon{},
					srid:          4326,
					bbox: geom.NewExtent(
						[2]float64{23.6655, 37.8501},
						[2]float64{23.7957, 37.9431},
					),
					colNames: []string{
						"addr:housenumber",
						"addr:street",
						"building",
						"fid",
						"geom",
						"hazard_prone",
						"name",
						"osm_id",
						"osm_way_id",
					},
				},
				"harbours_points": {
					idFieldname:   "fid",
					geomFieldname: "geom",
					geomType:      geom.Point{},
					srid:          4326,
					bbox: geom.NewExtent(
						[2]float64{0, 0},
						[2]float64{0, 0},
					),
					colNames: []string{
						"fid",
						"geom",
						"harbour",
						"landuse",
						"leisure",
						"name",
						"osm_id",
					},
				},
				"land_polygons": {
					idFieldname:   "ogc_fid",
					geomFieldname: "geom",
					geomType:      geom.Polygon{},
					srid:          4326,
					bbox: geom.NewExtent(
						[2]float64{23.6654, 37.85},
						[2]float64{23.7958, 37.9431},
					),
					colNames: []string{
						"fid",
						"geom",
						"ogc_fid",
					},
				},
				"landuse_polygons": {
					idFieldname:   "fid",
					geomFieldname: "geom",
					geomType:      geom.MultiPolygon{},
					srid:          4326,
					bbox: geom.NewExtent(
						[2]float64{23.6655, 37.85},
						[2]float64{23.7958, 37.9431},
					),
					colNames: []string{
						"fid",
						"geom",
						"landuse",
						"name",
						"osm_id",
						"osm_way_id",
					},
				},
				"leisure_polygons": {
					idFieldname:   "fid",
					geomFieldname: "geom",
					geomType:      geom.MultiPolygon{},
					srid:          4326,
					bbox: geom.NewExtent(
						[2]float64{23.6655, 37.8501},
						[2]float64{23.7841, 37.9431},
					),
					colNames: []string{
						"fid",
						"geom",
						"leisure",
						"name",
						"osm_id",
						"osm_way_id",
					},
				},
				"natural_lines": {
					idFieldname:   "fid",
					geomFieldname: "geom",
					geomType:      geom.MultiLineString{},
					srid:          4326,
					bbox: geom.NewExtent(
						[2]float64{23.6654, 37.8501},
						[2]float64{23.7957, 37.9431},
					),
					colNames: []string{
						"fid",
						"geom",
						"hazard_prone",
						"name",
						"natural",
						"osm_id",
					},
				},
				"natural_polygons": {
					idFieldname:   "fid",
					geomFieldname: "geom",
					geomType:      geom.MultiPolygon{},
					srid:          4326,
					bbox: geom.NewExtent(
						[2]float64{23.6654, 37.8502},
						[2]float64{23.7957, 37.9431},
					),
					colNames: []string{
						"fid",
						"geom",
						"hazard_prone",
						"name",
						"natural",
						"osm_id",
						"osm_way_id",
					},
				},
				"places_points": {
					idFieldname:   "fid",
					geomFieldname: "geom",
					geomType:      geom.Point{},
					srid:          4326,
					bbox: geom.NewExtent(
						[2]float64{23.6854, 37.8503},
						[2]float64{23.7819, 37.9431},
					),
					colNames: []string{
						"fid",
						"geom",
						"is_in",
						"name",
						"osm_id",
						"place",
					},
				},
				"places_polygons": {
					idFieldname:   "fid",
					geomFieldname: "geom",
					geomType:      geom.MultiPolygon{},
					srid:          4326,
					bbox: geom.NewExtent(
						[2]float64{0, 0},
						[2]float64{0, 0},
					),
					colNames: []string{
						"fid",
						"geom",
						"is_in",
						"name",
						"osm_id",
						"osm_way_id",
						"place",
					},
				},
				"rail_lines": {
					idFieldname:   "fid",
					geomFieldname: "geom",
					geomType:      geom.MultiLineString{},
					srid:          4326,
					bbox: geom.NewExtent(
						[2]float64{23.6828, 37.8501},
						[2]float64{23.7549, 37.9431},
					),
					colNames: []string{
						"bridge",
						"cutting",
						"embankment",
						"fid",
						"frequency",
						"geom",
						"layer",
						"name",
						"operator",
						"osm_id",
						"railway",
						"service",
						"source",
						"tracks",
						"tunnel",
						"usage",
						"voltage",
						"z_index",
					},
				},
				"roads_lines": {
					idFieldname:   "fid",
					geomFieldname: "geom",
					geomType:      geom.MultiLineString{},
					srid:          4326,
					bbox: geom.NewExtent(
						[2]float64{23.6655, 37.85},
						[2]float64{23.7958, 37.9431},
					),
					colNames: []string{
						"barrier",
						"bicycle_road",
						"fid",
						"ford",
						"geom",
						"hazard_prone",
						"highway",
						"layer",
						"name",
						"osm_id",
						"traffic_calming",
						"tunnel",
						"z_index",
					},
				},
				"towers_antennas_points": {
					idFieldname:   "fid",
					geomFieldname: "geom",
					geomType:      geom.Point{},
					srid:          4326,
					bbox: geom.NewExtent(
						[2]float64{23.6903, 37.8656},
						[2]float64{23.783, 37.943},
					),
					colNames: []string{
						"fid",
						"geom",
						"man_made",
						"name",
						"osm_id",
					},
				},
				"waterways_lines": {
					idFieldname:   "fid",
					geomFieldname: "geom",
					geomType:      geom.MultiLineString{},
					srid:          4326,
					bbox: geom.NewExtent(
						[2]float64{23.6718, 37.8864},
						[2]float64{23.7707, 37.9429},
					),
					colNames: []string{
						"fid",
						"geom",
						"hazard_prone",
						"name",
						"osm_id",
						"waterway",
					},
				},
			},
		},
		"natural earth": {
			gpkgPath: GPKGNaturalEarthFilePath,
			expectedFtd: map[string]featureTableDetails{
				"ne_110m_land": {
					idFieldname:   "fid",
					geomFieldname: "geom",
					geomType:      geom.Polygon{},
					srid:          4326,
					bbox: geom.NewExtent(
						[2]float64{-180, -90},
						[2]float64{180, 83.6451},
					),
					colNames: []string{
						"featurecla",
						"fid",
						"geom",
						"min_zoom",
						"scalerank",
					},
				},
			},
		},
		"puerto monte": {
			gpkgPath: GPKGPuertoMontFilePath,
			expectedFtd: map[string]featureTableDetails{
				"amenities_points": {
					idFieldname:   "fid",
					geomFieldname: "geom",
					geomType:      geom.Point{},
					srid:          4326,
					bbox: geom.NewExtent(
						[2]float64{-72.9936, -41.5023},
						[2]float64{-72.8913, -41.4452},
					),
					colNames: []string{
						"addr:housenumber",
						"addr:street",
						"amenity",
						"building",
						"fid",
						"geom",
						"historic",
						"information",
						"leisure",
						"name",
						"office",
						"osm_id",
						"shop",
						"tourism",
					},
				},
				"amenities_polygons": {
					idFieldname:   "fid",
					geomFieldname: "geom",
					geomType:      geom.MultiPolygon{},
					srid:          4326,
					bbox: geom.NewExtent(
						[2]float64{-72.9945, -41.4949},
						[2]float64{-72.8897, -41.4411},
					),
					colNames: []string{
						"addr:housenumber",
						"addr:street",
						"amenity",
						"building",
						"fid",
						"geom",
						"historic",
						"information",
						"leisure",
						"name",
						"office",
						"osm_id",
						"osm_way_id",
						"shop",
						"tourism",
					},
				},
				"aviation_lines": {
					idFieldname:   "fid",
					geomFieldname: "geom",
					geomType:      geom.MultiLineString{},
					srid:          4326,
					bbox: geom.NewExtent(
						[2]float64{-72.9205, -41.4586},
						[2]float64{-72.9153, -41.4504},
					),
					colNames: []string{
						"aeroway",
						"building",
						"fid",
						"geom",
						"iata",
						"icao",
						"name",
						"osm_id",
						"source",
						"surface",
						"type",
					},
				},
				"aviation_points": {
					idFieldname:   "fid",
					geomFieldname: "geom",
					geomType:      geom.Point{},
					srid:          4326,
					bbox: geom.NewExtent(
						[2]float64{-72.9564, -41.4856},
						[2]float64{-72.9028, -41.4458},
					),
					colNames: []string{
						"aeroway",
						"building",
						"fid",
						"geom",
						"iata",
						"icao",
						"name",
						"osm_id",
						"source",
						"surface",
						"type",
					},
				},
				"aviation_polygons": {
					idFieldname:   "fid",
					geomFieldname: "geom",
					geomType:      geom.MultiPolygon{},
					srid:          4326,
					bbox: geom.NewExtent(
						[2]float64{-72.961, -41.4849},
						[2]float64{-72.9138, -41.449},
					),
					colNames: []string{
						"aeroway",
						"building",
						"fid",
						"geom",
						"iata",
						"icao",
						"name",
						"osm_id",
						"osm_way_id",
						"source",
						"surface",
						"type",
					},
				},
				"boundary": {
					idFieldname:   "id",
					geomFieldname: "geom",
					geomType:      geom.MultiPolygon{},
					srid:          4326,
					bbox: geom.NewExtent(
						[2]float64{-72.9965, -41.5069},
						[2]float64{-72.8718, -41.4336},
					),
					colNames: []string{
						"geom",
						"id",
					},
				},
				"buildings_polygons": {
					idFieldname:   "fid",
					geomFieldname: "geom",
					geomType:      geom.MultiPolygon{},
					srid:          4326,
					bbox: geom.NewExtent(
						[2]float64{-72.996, -41.4959},
						[2]float64{-72.8875, -41.4363},
					),
					colNames: []string{
						"addr:housenumber",
						"addr:street",
						"building",
						"fid",
						"geom",
						"hazard_prone",
						"name",
						"osm_id",
						"osm_way_id",
					},
				},
				"harbours_points": {
					idFieldname:   "fid",
					geomFieldname: "geom",
					geomType:      geom.Point{},
					srid:          4326,
					bbox: geom.NewExtent(
						[2]float64{0, 0},
						[2]float64{0, 0},
					),
					colNames: []string{
						"fid",
						"geom",
						"harbour",
						"landuse",
						"leisure",
						"name",
						"osm_id",
					},
				},
				"land_polygons": {
					idFieldname:   "ogc_fid",
					geomFieldname: "geom",
					geomType:      geom.Polygon{},
					srid:          4326,
					bbox: geom.NewExtent(
						[2]float64{-72.9965, -41.5069},
						[2]float64{-72.8718, -41.4336},
					),
					colNames: []string{
						"fid",
						"geom",
						"ogc_fid",
					},
				},
				"landuse_polygons": {
					idFieldname:   "fid",
					geomFieldname: "geom",
					geomType:      geom.MultiPolygon{},
					srid:          4326,
					bbox: geom.NewExtent(
						[2]float64{-72.9964, -41.5067},
						[2]float64{-72.8873, -41.4355},
					),
					colNames: []string{
						"fid",
						"geom",
						"landuse",
						"name",
						"osm_id",
						"osm_way_id",
					},
				},
				"leisure_polygons": {
					idFieldname:   "fid",
					geomFieldname: "geom",
					geomType:      geom.MultiPolygon{},
					srid:          4326,
					bbox: geom.NewExtent(
						[2]float64{-72.9912, -41.4999},
						[2]float64{-72.8826, -41.447},
					),
					colNames: []string{
						"fid",
						"geom",
						"leisure",
						"name",
						"osm_id",
						"osm_way_id",
					},
				},
				"natural_lines": {
					idFieldname:   "fid",
					geomFieldname: "geom",
					geomType:      geom.MultiLineString{},
					srid:          4326,
					bbox: geom.NewExtent(
						[2]float64{-72.9958, -41.5069},
						[2]float64{-72.8718, -41.4534},
					),
					colNames: []string{
						"fid",
						"geom",
						"hazard_prone",
						"name",
						"natural",
						"osm_id",
					},
				},
				"natural_polygons": {
					idFieldname:   "fid",
					geomFieldname: "geom",
					geomType:      geom.MultiPolygon{},
					srid:          4326,
					bbox: geom.NewExtent(
						[2]float64{-72.9964, -41.5068},
						[2]float64{-72.8718, -41.4484},
					),
					colNames: []string{
						"fid",
						"geom",
						"hazard_prone",
						"name",
						"natural",
						"osm_id",
						"osm_way_id",
					},
				},
				"places_points": {
					idFieldname:   "fid",
					geomFieldname: "geom",
					geomType:      geom.Point{},
					srid:          4326,
					bbox: geom.NewExtent(
						[2]float64{-72.9962, -41.4927},
						[2]float64{-72.8761, -41.4369},
					),
					colNames: []string{
						"fid",
						"geom",
						"is_in",
						"name",
						"osm_id",
						"place",
					},
				},
				"places_polygons": {
					idFieldname:   "fid",
					geomFieldname: "geom",
					geomType:      geom.MultiPolygon{},
					srid:          4326,
					bbox: geom.NewExtent(
						[2]float64{-72.9908, -41.5067},
						[2]float64{-72.9482, -41.4824},
					),
					colNames: []string{
						"fid",
						"geom",
						"is_in",
						"name",
						"osm_id",
						"osm_way_id",
						"place",
					},
				},
				"rail_lines": {
					idFieldname:   "fid",
					geomFieldname: "geom",
					geomType:      geom.MultiLineString{},
					srid:          4326,
					bbox: geom.NewExtent(
						[2]float64{-72.958, -41.4973},
						[2]float64{-72.8763, -41.435},
					),
					colNames: []string{
						"bridge",
						"cutting",
						"embankment",
						"fid",
						"frequency",
						"geom",
						"layer",
						"name",
						"operator",
						"osm_id",
						"railway",
						"service",
						"source",
						"tracks",
						"tunnel",
						"usage",
						"voltage",
						"z_index",
					},
				},
				"roads_lines": {
					idFieldname:   "fid",
					geomFieldname: "geom",
					geomType:      geom.MultiLineString{},
					srid:          4326,
					bbox: geom.NewExtent(
						[2]float64{-72.9965, -41.5068},
						[2]float64{-72.8718, -41.4336},
					),
					colNames: []string{
						"barrier",
						"bicycle_road",
						"fid",
						"ford",
						"geom",
						"hazard_prone",
						"highway",
						"layer",
						"name",
						"osm_id",
						"traffic_calming",
						"tunnel",
						"z_index",
					},
				},
				"towers_antennas_points": {
					idFieldname:   "fid",
					geomFieldname: "geom",
					geomType:      geom.Point{},
					srid:          4326,
					bbox: geom.NewExtent(
						[2]float64{-72.9679, -41.4901},
						[2]float64{-72.9575, -41.4677},
					),
					colNames: []string{
						"fid",
						"geom",
						"man_made",
						"name",
						"osm_id",
					},
				},
				"waterways_lines": {
					idFieldname:   "fid",
					geomFieldname: "geom",
					geomType:      geom.MultiLineString{},
					srid:          4326,
					bbox: geom.NewExtent(
						[2]float64{-72.9901, -41.4877},
						[2]float64{-72.896, -41.435},
					),
					colNames: []string{
						"fid",
						"geom",
						"hazard_prone",
						"name",
						"osm_id",
						"waterway",
					},
				},
			},
		},
		"apeldoorn": {
			gpkgPath: GPKGApeldoornFilePath,
			expectedFtd: map[string]featureTableDetails{
				"plaats": {
					idFieldname:   "fid",
					geomFieldname: "geom",
					geomType:      nil,
					srid:          28992,
					bbox: geom.NewExtent(
						[2]float64{190942, 465141},
						[2]float64{199718, 473822},
					),
					colNames: []string{
						"fid",
						"geom",
						"naam",
					},
				},
			},
		},
	}

	for tname, tc := range tests {
		t.Run(tname, fn(tc))
	}
}

func TestCleanup(t *testing.T) {
	type tcase struct {
		config dict.Dict
	}

	fn := func(tc tcase) func(*testing.T) {
		return func(t *testing.T) {
			_, err := NewTileProvider(tc.config, nil)
			if err != nil {
				t.Fatalf("err creating NewTileProvider: %v", err)
				return
			}

			if len(providers) != 1 {
				t.Errorf("expecting 1 providers, got %v", len(providers))
				return
			}

			Cleanup()

			if len(providers) != 0 {
				t.Errorf("expecting 0 providers, got %v", len(providers))
				return
			}
		}
	}

	tests := map[string]tcase{
		"cleanup": {
			config: map[string]interface{}{
				"filepath": GPKGAthensFilePath,
				"layers": []map[string]interface{}{
					{"name": "a_points", "tablename": "amenities_points", "id_fieldname": "fid", "fields": []string{"amenity", "religion", "tourism", "shop"}},
					{"name": "r_lines", "tablename": "rail_lines", "id_fieldname": "fid", "fields": []string{"railway", "bridge", "tunnel"}},
					{"name": "rd_lines", "tablename": "roads_lines"},
				},
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, fn(tc))
	}
}
