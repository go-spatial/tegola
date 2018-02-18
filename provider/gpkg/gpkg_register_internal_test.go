// +build cgo

package gpkg

import (
	"database/sql"
	"fmt"
	"testing"

	_ "github.com/mattn/go-sqlite3"
	"github.com/terranodo/tegola/geom"
)

var (
	GPKGAthensFilePath       = "testdata/athens-osm-20170921.gpkg"
	GPKGNaturalEarthFilePath = "testdata/natural_earth_minimal.gpkg"
	GPKGPuertoMontFilePath   = "testdata/puerto_mont-osm-20170922.gpkg"
)

func ftdEqual(t *testing.T, tableName string, ftd, ftdExpected featureTableDetails) bool {
	equal := true
	if ftd.idFieldname != ftdExpected.idFieldname {
		equal = false
		t.Logf("(%v) idFieldname %v != %v", tableName, ftd.idFieldname, ftdExpected.idFieldname)
	}
	if ftd.geomFieldname != ftdExpected.geomFieldname {
		equal = false
		t.Logf("(%v) geomFieldname %v != %v", tableName, ftd.geomFieldname, ftdExpected.geomFieldname)
	}
	// Since it's used as a type indicator, we don't care about the content of this geometry.
	if fmt.Sprintf("%T", ftd.geomType) != fmt.Sprintf("%T", ftdExpected.geomType) {
		equal = false
		t.Logf("(%v) geomType %T != %T", tableName, ftd.geomType, ftdExpected.geomType)
	}
	if ftd.srid != ftdExpected.srid {
		equal = false
		t.Logf("(%v) srid %v != %v", tableName, ftd.srid, ftdExpected.srid)
	}
	if ftd.bbox != ftdExpected.bbox {
		equal = false
		t.Logf("(%v) bbox %v != %v", tableName, ftd.bbox, ftdExpected.bbox)
	}

	return equal
}

func TestFeatureTableMetaData(t *testing.T) {
	type tcase struct {
		gpkgPath    string
		expectedFtd map[string]featureTableDetails
	}

	fn := func(t *testing.T, tc tcase) {
		db, err := sql.Open("sqlite3", tc.gpkgPath)
		if err != nil {
			t.Errorf("problem opening gpkg: %v", err)
		}
		defer db.Close()

		ftmd, err := featureTableMetaData(db)
		if err != nil {
			t.Errorf("problem extracting metadata: %v", err)
		}

		for tname, ftd := range ftmd {
			expectedFtd, ok := tc.expectedFtd[tname]
			if !ok {
				t.Errorf("unexpected table in featureTableDetails: '%v'", tname)
				t.Log(tc.expectedFtd)
			}
			if !ftdEqual(t, tname, ftd, expectedFtd) {
				t.Fail()
			}
		}
	}

	tests := map[string]tcase{
		"athens": {
			gpkgPath: GPKGAthensFilePath,
			expectedFtd: map[string]featureTableDetails{
				"amenities_points":       {idFieldname: "fid", geomFieldname: "geom", geomType: geom.Point{}, srid: 4326, bbox: geom.BoundingBox{{23.683, 37.8502}, {23.7783, 37.9431}}},
				"amenities_polygons":     {idFieldname: "fid", geomFieldname: "geom", geomType: geom.MultiPolygon{}, srid: 4326, bbox: geom.BoundingBox{{23.6698, 37.8502}, {23.7815, 37.9431}}},
				"aviation_lines":         {idFieldname: "fid", geomFieldname: "geom", geomType: geom.MultiLineString{}, srid: 4326, bbox: geom.BoundingBox{{23.7231, 37.8739}, {23.7422, 37.9036}}},
				"aviation_points":        {idFieldname: "fid", geomFieldname: "geom", geomType: geom.Point{}, srid: 4326, bbox: geom.BoundingBox{{23.6802, 37.9354}, {23.6802, 37.9354}}},
				"aviation_polygons":      {idFieldname: "fid", geomFieldname: "geom", geomType: geom.MultiPolygon{}, srid: 4326, bbox: geom.BoundingBox{{23.6698, 37.8842}, {23.7414, 37.9391}}},
				"boundary":               {idFieldname: "id", geomFieldname: "geom", geomType: geom.MultiPolygon{}, srid: 4326, bbox: geom.BoundingBox{{23.6654, 37.85}, {23.7958, 37.9431}}},
				"buildings_polygons":     {idFieldname: "fid", geomFieldname: "geom", geomType: geom.MultiPolygon{}, srid: 4326, bbox: geom.BoundingBox{{23.6655, 37.8501}, {23.7957, 37.9431}}},
				"harbours_points":        {idFieldname: "fid", geomFieldname: "geom", geomType: geom.Point{}, srid: 4326, bbox: geom.BoundingBox{{0, 0}, {0, 0}}},
				"land_polygons":          {idFieldname: "ogc_fid", geomFieldname: "geom", geomType: geom.Polygon{}, srid: 4326, bbox: geom.BoundingBox{{23.6654, 37.85}, {23.7958, 37.9431}}},
				"landuse_polygons":       {idFieldname: "fid", geomFieldname: "geom", geomType: geom.MultiPolygon{}, srid: 4326, bbox: geom.BoundingBox{{23.6655, 37.85}, {23.7958, 37.9431}}},
				"leisure_polygons":       {idFieldname: "fid", geomFieldname: "geom", geomType: geom.MultiPolygon{}, srid: 4326, bbox: geom.BoundingBox{{23.6655, 37.8501}, {23.7841, 37.9431}}},
				"natural_lines":          {idFieldname: "fid", geomFieldname: "geom", geomType: geom.MultiLineString{}, srid: 4326, bbox: geom.BoundingBox{{23.6654, 37.8501}, {23.7957, 37.9431}}},
				"natural_polygons":       {idFieldname: "fid", geomFieldname: "geom", geomType: geom.MultiPolygon{}, srid: 4326, bbox: geom.BoundingBox{{23.6654, 37.8502}, {23.7957, 37.9431}}},
				"places_points":          {idFieldname: "fid", geomFieldname: "geom", geomType: geom.Point{}, srid: 4326, bbox: geom.BoundingBox{{23.6854, 37.8503}, {23.7819, 37.9431}}},
				"places_polygons":        {idFieldname: "fid", geomFieldname: "geom", geomType: geom.MultiPolygon{}, srid: 4326, bbox: geom.BoundingBox{{0, 0}, {0, 0}}},
				"rail_lines":             {idFieldname: "fid", geomFieldname: "geom", geomType: geom.MultiLineString{}, srid: 4326, bbox: geom.BoundingBox{{23.6828, 37.8501}, {23.7549, 37.9431}}},
				"roads_lines":            {idFieldname: "fid", geomFieldname: "geom", geomType: geom.MultiLineString{}, srid: 4326, bbox: geom.BoundingBox{{23.6655, 37.85}, {23.7958, 37.9431}}},
				"towers_antennas_points": {idFieldname: "fid", geomFieldname: "geom", geomType: geom.Point{}, srid: 4326, bbox: geom.BoundingBox{{23.6903, 37.8656}, {23.783, 37.943}}},
				"waterways_lines":        {idFieldname: "fid", geomFieldname: "geom", geomType: geom.MultiLineString{}, srid: 4326, bbox: geom.BoundingBox{{23.6718, 37.8864}, {23.7707, 37.9429}}},
			},
		},
		"natural earth": {
			gpkgPath: GPKGNaturalEarthFilePath,
			expectedFtd: map[string]featureTableDetails{
				"ne_110m_land": {idFieldname: "fid", geomFieldname: "geom", geomType: geom.Polygon{}, srid: 4326, bbox: geom.BoundingBox{{-180, -90}, {180, 83.6451}}},
			},
		},
		"puerto monte": {
			gpkgPath: GPKGPuertoMontFilePath,
			expectedFtd: map[string]featureTableDetails{
				"amenities_points":       {idFieldname: "fid", geomFieldname: "geom", geomType: geom.Point{}, srid: 4326, bbox: geom.BoundingBox{{-72.9936, -41.5023}, {-72.8913, -41.4452}}},
				"amenities_polygons":     {idFieldname: "fid", geomFieldname: "geom", geomType: geom.MultiPolygon{}, srid: 4326, bbox: geom.BoundingBox{{-72.9945, -41.4949}, {-72.8897, -41.4411}}},
				"aviation_lines":         {idFieldname: "fid", geomFieldname: "geom", geomType: geom.MultiLineString{}, srid: 4326, bbox: geom.BoundingBox{{-72.9205, -41.4586}, {-72.9153, -41.4504}}},
				"aviation_points":        {idFieldname: "fid", geomFieldname: "geom", geomType: geom.Point{}, srid: 4326, bbox: geom.BoundingBox{{-72.9564, -41.4856}, {-72.9028, -41.4458}}},
				"aviation_polygons":      {idFieldname: "fid", geomFieldname: "geom", geomType: geom.MultiPolygon{}, srid: 4326, bbox: geom.BoundingBox{{-72.961, -41.4849}, {-72.9138, -41.449}}},
				"boundary":               {idFieldname: "id", geomFieldname: "geom", geomType: geom.MultiPolygon{}, srid: 4326, bbox: geom.BoundingBox{{-72.9965, -41.5069}, {-72.8718, -41.4336}}},
				"buildings_polygons":     {idFieldname: "fid", geomFieldname: "geom", geomType: geom.MultiPolygon{}, srid: 4326, bbox: geom.BoundingBox{{-72.996, -41.4959}, {-72.8875, -41.4363}}},
				"harbours_points":        {idFieldname: "fid", geomFieldname: "geom", geomType: geom.Point{}, srid: 4326, bbox: geom.BoundingBox{{00}, {00}}},
				"land_polygons":          {idFieldname: "ogc_fid", geomFieldname: "geom", geomType: geom.Polygon{}, srid: 4326, bbox: geom.BoundingBox{{-72.9965, -41.5069}, {-72.8718, -41.4336}}},
				"landuse_polygons":       {idFieldname: "fid", geomFieldname: "geom", geomType: geom.MultiPolygon{}, srid: 4326, bbox: geom.BoundingBox{{-72.9964, -41.5067}, {-72.8873, -41.4355}}},
				"leisure_polygons":       {idFieldname: "fid", geomFieldname: "geom", geomType: geom.MultiPolygon{}, srid: 4326, bbox: geom.BoundingBox{{-72.9912, -41.4999}, {-72.8826, -41.447}}},
				"natural_lines":          {idFieldname: "fid", geomFieldname: "geom", geomType: geom.MultiLineString{}, srid: 4326, bbox: geom.BoundingBox{{-72.9958, -41.5069}, {-72.8718, -41.4534}}},
				"natural_polygons":       {idFieldname: "fid", geomFieldname: "geom", geomType: geom.MultiPolygon{}, srid: 4326, bbox: geom.BoundingBox{{-72.9964, -41.5068}, {-72.8718, -41.4484}}},
				"places_points":          {idFieldname: "fid", geomFieldname: "geom", geomType: geom.Point{}, srid: 4326, bbox: geom.BoundingBox{{-72.9962, -41.4927}, {-72.8761, -41.4369}}},
				"places_polygons":        {idFieldname: "fid", geomFieldname: "geom", geomType: geom.MultiPolygon{}, srid: 4326, bbox: geom.BoundingBox{{-72.9908, -41.5067}, {-72.9482, -41.4824}}},
				"rail_lines":             {idFieldname: "fid", geomFieldname: "geom", geomType: geom.MultiLineString{}, srid: 4326, bbox: geom.BoundingBox{{-72.958, -41.4973}, {-72.8763, -41.435}}},
				"roads_lines":            {idFieldname: "fid", geomFieldname: "geom", geomType: geom.MultiLineString{}, srid: 4326, bbox: geom.BoundingBox{{-72.9965, -41.5068}, {-72.8718, -41.4336}}},
				"towers_antennas_points": {idFieldname: "fid", geomFieldname: "geom", geomType: geom.Point{}, srid: 4326, bbox: geom.BoundingBox{{-72.9679, -41.4901}, {-72.9575, -41.4677}}},
				"waterways_lines":        {idFieldname: "fid", geomFieldname: "geom", geomType: geom.MultiLineString{}, srid: 4326, bbox: geom.BoundingBox{{-72.9901, -41.4877}, {-72.896, -41.435}}},
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

func TestCleanup(t *testing.T) {
	type tcase struct {
		config map[string]interface{}
	}

	fn := func(t *testing.T, tc tcase) {
		_, err := NewTileProvider(tc.config)
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

	tests := map[string]tcase{
		"cleanup": tcase{
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
		tc := tc
		t.Run(name, func(t *testing.T) {
			fn(t, tc)
		})
	}
}
