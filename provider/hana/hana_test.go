package hana_test

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/go-spatial/tegola"
	"github.com/go-spatial/tegola/dict"
	"github.com/go-spatial/tegola/internal/ttools"
	"github.com/go-spatial/tegola/provider"
	"github.com/go-spatial/tegola/provider/hana"
)

// TESTENV is the environment variable that must be set to "yes" to run HANA tests.
const TESTENV = "RUN_HANA_TESTS"

var test_schema_name string

func getConfigFromEnv() map[string]interface{} {
	return map[string]interface{}{
		hana.ConfigKeyName: "provider_name",
		hana.ConfigKeyURI:  GetConnectionURI(),
	}
}

type TCConfig struct {
	BaseConfig     map[string]interface{}
	ConfigOverride map[string]interface{}
	LayerConfig    []map[string]interface{}
}

func (cfg TCConfig) Config() dict.Dict {
	var config map[string]interface{}
	mConfig := getConfigFromEnv()
	if cfg.BaseConfig != nil {
		mConfig = cfg.BaseConfig
	}
	config = make(map[string]interface{}, len(mConfig))
	for k, v := range mConfig {
		config[k] = v
	}

	// set the config overrides
	for k, v := range cfg.ConfigOverride {
		config[k] = v
	}

	if len(cfg.LayerConfig) > 0 {
		layerConfig, _ := config[hana.ConfigKeyLayers].([]map[string]interface{})
		layerConfig = append(layerConfig, cfg.LayerConfig...)
		config[hana.ConfigKeyLayers] = layerConfig
	}

	return dict.Dict(config)
}

func GetConnectionURI() string {
	return os.Getenv("HANA_CONNECTION_STRING")
}

func CreateDBConnection() (*sql.DB, error) {
	db, err := hana.OpenDB(GetConnectionURI())
	if err != nil {
		return nil, err
	}

	if err := db.Ping(); err != nil {
		return nil, err
	}

	return db, nil
}

func getSQLWithSchema(sql string) string {
	return strings.Replace(sql, "[schema_name]", test_schema_name, -1)
}

func TestMain(m *testing.M) {
	if os.Getenv(TESTENV) == "yes" {
		err := SetUp()
		if err != nil {
			os.Exit(1)
			return
		}
		retCode := m.Run()
		err = TearDown()
		if err != nil {
			os.Exit(1)
			return
		}
		os.Exit(retCode)
	} else {
		retCode := m.Run()
		os.Exit(retCode)
	}
}

func SetUp() error {
	fmt.Printf("\033[1;36m%s\033[0m", "> Setup HANA tests completed\n")

	db, err := CreateDBConnection()
	if err != nil {
		return err
	}

	var uid string
	err = db.QueryRow("SELECT REPLACE(CURRENT_UTCDATE, '-', '') || '_' || BINTOHEX(SYSUUID) FROM DUMMY;").Scan(&uid)
	if err != nil {
		return err
	}
	test_schema_name = fmt.Sprintf("tegola_test_%v", uid)

	sql := getSQLWithSchema(`CREATE SCHEMA "[schema_name]"`)
	_, err = db.Exec(sql)
	if err != nil {
		return err
	}

	sql = `CREATE TABLE "[schema_name]"."table_1" (
		   "id" INTEGER NOT NULL PRIMARY KEY,
		   "clm_bool" BOOLEAN,
		   "clm_tinyint" TINYINT,
		   "clm_smallint" SMALLINT,
		   "clm_bigint" BIGINT,
		   "clm_decimal" DECIMAL,
		   "clm_smalldecimal" SMALLDECIMAL,
		   "clm_real" REAL,
		   "clm_double" DOUBLE,
		   "clm_char" CHAR,
		   "clm_varchar" VARCHAR(256),
		   "clm_nchar" NCHAR,
		   "clm_nvarchar" NVARCHAR(100),
		   "clm_date" DATE,
		   "clm_time" TIME,
		   "clm_timestamp" TIMESTAMP,
		   "clm_seconddate" SECONDDATE,
		   "clm_binary" BINARY(100),
		   "clm_varbinary" VARBINARY(50),
		   "clm_blob" BLOB,
		   "clm_nclob" NCLOB,
		   "clm_clob" CLOB,
		   "geom" ST_GEOMETRY(3857));`
	_, err = db.Exec(getSQLWithSchema(sql))
	if err != nil {
		return err
	}

	sql = `INSERT INTO "[schema_name]"."table_1" ("id", "clm_bool", "clm_tinyint", "clm_smallint", "clm_bigint", "clm_decimal", "clm_smalldecimal",
	                                              "clm_real", "clm_double", "clm_char", "clm_varchar", "clm_nchar", "clm_nvarchar",
												  "clm_date", "clm_time", "clm_timestamp", "clm_seconddate", "clm_binary", "clm_varbinary", "clm_blob",
												  "clm_nclob", "clm_clob", "geom")
	                                      VALUES (1, TRUE, 127, 7923, 8923732, 7234.89732, 3.14, 7.2342, 8912312.2131, 'a', 'varchar1', 'ß', 'nvarchar1',
										          '2018/04/25', '21:15:47', '2018/04/25 21:15:47.987', '0001-01-01 00:00:01', x'dcba00', x'00abcd', x'dcba00ff',
												  'Karlsruher Straße', 'New York', ST_GeomFromText('POINT(7 8)', 3857));`
	_, err = db.Exec(getSQLWithSchema(sql))
	if err != nil {
		return err
	}
	sql = `INSERT INTO "[schema_name]"."table_1" ("id", "clm_bool", "clm_tinyint", "clm_smallint", "clm_bigint", "clm_decimal", "clm_smalldecimal",
	                                              "clm_real", "clm_double", "clm_char", "clm_varchar", "clm_nchar", "clm_nvarchar",
												  "clm_date", "clm_time", "clm_timestamp", "clm_seconddate", "clm_binary", "clm_varbinary", "clm_blob",
												  "clm_nclob", "clm_clob", "geom")
	                                      VALUES (2, FALSE, 15, NULL, -21231, NULL, NULL, NULL, NULL, 'd', 'varchar2', NULL, 'nvarchar2',
										          NULL, NULL, NULL, NULL, NULL, NULL, NULL,
												  NULL, NULL, ST_GeomFromText('POINT(1 9)', 3857));`
	_, err = db.Exec(getSQLWithSchema(sql))
	if err != nil {
		return err
	}

	return err
}

func TearDown() error {
	db, err := CreateDBConnection()
	if err != nil {
		return err
	}

	sql := `DROP SCHEMA "[schema_name]" CASCADE`
	_, err = db.Exec(getSQLWithSchema(sql))

	fmt.Printf("\033[1;36m%s\033[0m", "> Teardown HANA tests completed")
	fmt.Printf("\n")

	return err
}

func TestNewTileProvider(t *testing.T) {
	ttools.ShouldSkip(t, TESTENV)

	fn := func(tc TCConfig) func(t *testing.T) {
		return func(t *testing.T) {
			config := tc.Config()
			_, err := hana.NewTileProvider(config, nil)
			if err != nil {
				t.Errorf("unable to create a new provider. err: %v", err)
				return
			}
		}
	}
	tests := map[string]TCConfig{
		"1": {
			LayerConfig: []map[string]interface{}{
				{
					hana.ConfigKeyLayerName: "layer_1",
					hana.ConfigKeyTablename: fmt.Sprintf(`"%v"."table_1"`, test_schema_name),
				},
			},
		},
		"2": {
			LayerConfig: []map[string]interface{}{
				{
					hana.ConfigKeyLayerName:      "layer_1",
					hana.ConfigKeyFeatureIDField: "id",
					hana.ConfigKeySQL:            fmt.Sprintf(`(SELECT "id", "geom" FROM "%v"."table_1") AS sub`, test_schema_name),
				},
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, fn(tc))
	}
}

func TestTileFeatures(t *testing.T) {
	ttools.ShouldSkip(t, TESTENV)

	type tcase struct {
		TCConfig
		tile                 provider.Tile
		expectedErr          error
		expectedFeatureCount int
		expectedTags         []string
	}

	fn := func(tc tcase) func(t *testing.T) {
		return func(t *testing.T) {
			config := tc.Config()
			p, err := hana.NewTileProvider(config, nil)
			if err != nil {
				if err == tc.expectedErr {
					return
				}
				t.Errorf("unexpected error; unable to create a new provider, expected: nil Got %v", err)
				return
			}

			layerName := tc.LayerConfig[0][hana.ConfigKeyLayerName].(string)

			var featureCount int
			err = p.TileFeatures(context.Background(), layerName, tc.tile, nil, func(f *provider.Feature) error {
				// only verify tags on first feature
				if featureCount == 0 {
					for _, tag := range tc.expectedTags {
						if _, ok := f.Tags[tag]; !ok {
							t.Errorf("expected tag %v in %v", tag, f.Tags)
							return nil
						}
					}
				}

				featureCount++

				return nil
			})
			if err != tc.expectedErr {
				t.Errorf("expected err (%v) got err (%v)", tc.expectedErr, err)
				return
			}

			if featureCount != tc.expectedFeatureCount {
				t.Errorf("feature count, expected %v got %v", tc.expectedFeatureCount, featureCount)
				return
			}
		}
	}

	tests := map[string]tcase{
		"tablename query": {
			TCConfig: TCConfig{
				LayerConfig: []map[string]interface{}{{
					hana.ConfigKeyLayerName: "land",
					hana.ConfigKeyTablename: `"TEGOLACI"."ne_10m_land_scale_rank"`,
				}},
			},
			tile:                 provider.NewTile(1, 1, 1, 64, tegola.WebMercator),
			expectedFeatureCount: 1212,
			expectedTags:         []string{"scalerank", "featurecla"},
		},
		"tablename query with fields": {
			TCConfig: TCConfig{
				LayerConfig: []map[string]interface{}{{
					hana.ConfigKeyLayerName: "land",
					hana.ConfigKeyTablename: `"TEGOLACI"."ne_10m_land_scale_rank"`,
					hana.ConfigKeySRID:      4326,
					hana.ConfigKeyFields:    []string{"scalerank"},
				}},
			},
			tile:                 provider.NewTile(1, 1, 1, 64, tegola.WebMercator),
			expectedFeatureCount: 1212,
			expectedTags:         []string{"scalerank"},
		},
		"tablename query with fields and id as field": {
			TCConfig: TCConfig{
				LayerConfig: []map[string]interface{}{{
					hana.ConfigKeyLayerName:      "land",
					hana.ConfigKeyTablename:      `"TEGOLACI"."ne_10m_land_scale_rank"`,
					hana.ConfigKeyFeatureIDField: "id",
					hana.ConfigKeyFields:         []string{"id", "scalerank"},
				}},
			},
			tile:                 provider.NewTile(1, 1, 1, 64, tegola.WebMercator),
			expectedFeatureCount: 1212,
			expectedTags:         []string{"id", "scalerank"},
		},
		"SQL sub-query": {
			TCConfig: TCConfig{
				LayerConfig: []map[string]interface{}{{
					hana.ConfigKeyLayerName: "land",
					hana.ConfigKeySQL:       `(SELECT "id", "geom", "featurecla" FROM "TEGOLACI"."ne_10m_land_scale_rank" WHERE !BBOX! ORDER BY "id" LIMIT 100) AS sub`,
				}},
			},
			tile:                 provider.NewTile(1, 1, 1, 64, tegola.WebMercator),
			expectedFeatureCount: 100,
			expectedTags:         []string{"featurecla"},
		},
		"SQL sub-query multi line": {
			TCConfig: TCConfig{
				LayerConfig: []map[string]interface{}{{
					hana.ConfigKeyLayerName: "land",
					hana.ConfigKeySQL: ` (
														SELECT "id", "geom", "featurecla" FROM "TEGOLACI"."ne_10m_land_scale_rank" WHERE !BBOX! LIMIT 100
													) AS sub`,
				}},
			},
			tile:                 provider.NewTile(1, 1, 1, 64, tegola.WebMercator),
			expectedFeatureCount: 100,
			expectedTags:         []string{"featurecla"},
		},
		"SQL sub-query and tablename": {
			TCConfig: TCConfig{
				LayerConfig: []map[string]interface{}{{
					hana.ConfigKeyLayerName: "land",
					hana.ConfigKeySQL:       `(SELECT "id", "geom", "featurecla" FROM "TEGOLACI"."ne_10m_land_scale_rank" WHERE !BBOX! LIMIT 100) AS sub`,
					hana.ConfigKeyTablename: "not_good_name",
				}},
			},
			tile:                 provider.NewTile(1, 1, 1, 64, tegola.WebMercator),
			expectedFeatureCount: 100,
			expectedTags:         []string{"featurecla"},
		},
		"SQL sub-query space after prens": {
			TCConfig: TCConfig{
				LayerConfig: []map[string]interface{}{{
					hana.ConfigKeyLayerName: "land",
					hana.ConfigKeySQL:       `(  SELECT "id", "geom", "featurecla" FROM "TEGOLACI"."ne_10m_land_scale_rank" WHERE !BBOX! LIMIT 100) AS sub`,
				}},
			},
			tile:                 provider.NewTile(1, 1, 1, 64, tegola.WebMercator),
			expectedFeatureCount: 100,
			expectedTags:         []string{"featurecla"},
		},
		"SQL sub-query space before prens": {
			TCConfig: TCConfig{
				LayerConfig: []map[string]interface{}{{
					hana.ConfigKeyLayerName: "land",
					hana.ConfigKeySQL:       `   (SELECT "id", "geom", "featurecla" FROM "TEGOLACI"."ne_10m_land_scale_rank" WHERE !BBOX! LIMIT 100) AS sub`,
				}},
			},
			tile:                 provider.NewTile(1, 1, 1, 64, tegola.WebMercator),
			expectedFeatureCount: 100,
			expectedTags:         []string{"featurecla"},
		},
		"SQL sub-query with *": {
			TCConfig: TCConfig{
				LayerConfig: []map[string]interface{}{{
					hana.ConfigKeyLayerName: "land",
					hana.ConfigKeySQL:       `(SELECT * FROM "TEGOLACI"."ne_10m_land_scale_rank" WHERE !BBOX! LIMIT 100) AS sub`,
				}},
			},
			tile:                 provider.NewTile(1, 1, 1, 64, tegola.WebMercator),
			expectedFeatureCount: 100,
			expectedTags:         []string{"scalerank", "featurecla"},
		},
		"SQL sub-query with * and fields": {
			TCConfig: TCConfig{
				LayerConfig: []map[string]interface{}{{
					hana.ConfigKeyLayerName: "land",
					hana.ConfigKeySQL:       `(SELECT * FROM "TEGOLACI"."ne_10m_land_scale_rank" WHERE !BBOX! LIMIT 100) AS sub`,
					hana.ConfigKeyFields:    []string{"scalerank"},
				}},
			},
			tile:                 provider.NewTile(1, 1, 1, 64, tegola.WebMercator),
			expectedFeatureCount: 100,
			expectedTags:         []string{"scalerank"},
		},
		"SQL with !ZOOM!": {
			TCConfig: TCConfig{
				LayerConfig: []map[string]interface{}{{
					hana.ConfigKeyLayerName: "land",
					hana.ConfigKeySQL:       `SELECT "id", "geom".ST_AsBinary() AS "geom" FROM "TEGOLACI"."ne_10m_land_scale_rank" WHERE "scalerank" IN (!ZOOM!) AND !BBOX!`,
					hana.ConfigKeySRID:      4326,
				}},
			},
			tile:                 provider.NewTile(1, 1, 1, 64, tegola.WebMercator),
			expectedFeatureCount: 23,
		},
		"SQL sub-query with token in SELECT": {
			TCConfig: TCConfig{
				LayerConfig: []map[string]interface{}{{
					hana.ConfigKeyLayerName: "land",
					hana.ConfigKeyGeomType:  "polygon", // required to disable SQL inspection
					hana.ConfigKeySQL:       `(SELECT "id", "geom", !ZOOM! * 2 AS "doublezoom" FROM "TEGOLACI"."ne_10m_land_scale_rank" WHERE "scalerank" IN (!ZOOM!) AND !BBOX!) AS sub`,
					hana.ConfigKeySRID:      4326,
				}},
			},
			tile:                 provider.NewTile(1, 1, 1, 64, tegola.WebMercator),
			expectedFeatureCount: 23,
			expectedTags:         []string{"doublezoom"},
		},
		"SQL sub-query with fields": {
			TCConfig: TCConfig{
				LayerConfig: []map[string]interface{}{{
					hana.ConfigKeyLayerName: "land",
					hana.ConfigKeySQL:       `(SELECT "id", "geom", 1 AS "a", '2' AS b, 3 AS "c" FROM "TEGOLACI"."ne_10m_land_scale_rank" WHERE "scalerank" IN (!ZOOM!) AND !BBOX!) AS sub`,
					hana.ConfigKeyFields:    []string{"id", "a", "B"},
					hana.ConfigKeySRID:      4326, // required to avoid a failure in auto detection of the SRID
				}},
			},
			tile:                 provider.NewTile(1, 1, 1, 64, tegola.WebMercator),
			expectedFeatureCount: 23,
			expectedTags:         []string{"id", "a", "B"},
		},
		"SQL with geom field name of a wrong type": {
			TCConfig: TCConfig{
				LayerConfig: []map[string]interface{}{{
					hana.ConfigKeyLayerName: "missing_geom_field_name",
					hana.ConfigKeyGeomField: "geom",
					hana.ConfigKeySQL:       `SELECT "id", "scalerank", 1 AS "geom" FROM "TEGOLACI"."ne_10m_land_scale_rank" WHERE !BBOX!`,
					hana.ConfigKeySRID:      4326, // required to avoid a failure in auto detection of the SRID
				}},
			},
			tile: provider.NewTile(16, 11241, 26168, 64, tegola.WebMercator),
			expectedErr: hana.ErrGeomFieldNotFound{
				GeomFieldName: "geom",
				LayerName:     "missing_geom_field_name",
			},
		},
		"SQL with missing geom field name": {
			TCConfig: TCConfig{
				LayerConfig: []map[string]interface{}{{
					hana.ConfigKeyLayerName: "missing_geom_field_name",
					hana.ConfigKeyGeomField: "geom",
					hana.ConfigKeySQL:       `SELECT "id", "scalerank" FROM "TEGOLACI"."ne_10m_land_scale_rank" WHERE !BBOX!`,
					hana.ConfigKeySRID:      4326, // required to avoid a failure in auto detection of the SRID
				}},
			},
			tile: provider.NewTile(16, 11241, 26168, 64, tegola.WebMercator),
			expectedErr: hana.ErrGeomFieldNotFound{
				GeomFieldName: "geom",
				LayerName:     "missing_geom_field_name",
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, fn(tc))
	}
}

func TestMVTForLayers(t *testing.T) {
	ttools.ShouldSkip(t, TESTENV)

	type tcase struct {
		TCConfig
		layerNames []string
		mvtTile    []byte
		err        string
		tile       provider.Tile
	}

	fn := func(tc tcase) func(t *testing.T) {
		return func(t *testing.T) {
			config := tc.Config()
			prvd, err := hana.NewMVTTileProvider(config, nil)
			// for now we will just check the length of the bytes.
			if tc.err != "" {
				if err == nil || !strings.Contains(err.Error(), tc.err) {
					t.Logf("error %#v", err)
					t.Errorf("expected error with %v in NewMVTTileProvider, got: %v", tc.err, err)
				}
				return
			}
			if err != nil {
				t.Errorf("NewMVTTileProvider unexpected error: %v", err)
				return
			}
			layers := make([]provider.Layer, len(tc.layerNames))

			for i := range tc.layerNames {
				layers[i] = provider.Layer{
					Name:    tc.layerNames[i],
					MVTName: tc.layerNames[i],
				}
			}
			mvtTile, err := prvd.MVTForLayers(context.Background(), tc.tile, nil, layers)
			if err != nil {
				t.Errorf("NewProvider unexpected error: %v", err)
				return
			}
			if len(tc.mvtTile) != len(mvtTile) {
				t.Errorf("tile byte length, expected %v got %v", len(tc.mvtTile), len(mvtTile))
			}
		}
	}
	tests := map[string]tcase{
		"SQL with fields and id": {
			TCConfig: TCConfig{
				LayerConfig: []map[string]interface{}{
					{
						hana.ConfigKeyFeatureIDField: "id",
						hana.ConfigKeyGeomType:       "multilinestring",
						hana.ConfigKeyGeomField:      "geom",
						hana.ConfigKeyLayerName:      "rivers",
						hana.ConfigKeySQL:            `SELECT * FROM (SELECT "id", "featurecla", "geom".ST_Transform(3857) AS "geom" FROM "TEGOLACI"."ne_50m_rivers_lake_centerlines") AS sub WHERE !BBOX!`,
						hana.ConfigKeySRID:           3857,
					},
				},
			},
			layerNames: []string{"rivers"},
			mvtTile:    make([]byte, 7619),
			tile:       provider.NewTile(2, 1, 1, 16, 4326),
		},
		"SQL with fields and without id": {
			TCConfig: TCConfig{
				LayerConfig: []map[string]interface{}{
					{
						hana.ConfigKeyGeomType:  "multilinestring",
						hana.ConfigKeyGeomField: "geom",
						hana.ConfigKeyLayerName: "rivers",
						hana.ConfigKeySQL:       `SELECT * FROM (SELECT "id", "featurecla", "geom".ST_Transform(3857) AS "geom" FROM "TEGOLACI"."ne_50m_rivers_lake_centerlines") AS sub WHERE !BBOX!`,
						hana.ConfigKeySRID:      3857,
					},
				},
			},
			layerNames: []string{"rivers"},
			mvtTile:    make([]byte, 7436),
			tile:       provider.NewTile(2, 1, 1, 16, 4326),
		},
		"SQL with id only": {
			TCConfig: TCConfig{
				LayerConfig: []map[string]interface{}{
					{
						hana.ConfigKeyFeatureIDField: "id",
						hana.ConfigKeyGeomType:       "multilinestring",
						hana.ConfigKeyGeomField:      "geom",
						hana.ConfigKeyLayerName:      "rivers",
						hana.ConfigKeySQL:            `SELECT * FROM (SELECT "id", "geom".ST_Transform(3857) AS "geom" FROM "TEGOLACI"."ne_50m_rivers_lake_centerlines") AS sub WHERE !BBOX!`,
						hana.ConfigKeySRID:           3857,
					},
				},
			},
			layerNames: []string{"rivers"},
			mvtTile:    make([]byte, 7443),
			tile:       provider.NewTile(2, 1, 1, 16, 4326),
		},
		"SQL without any fields": {
			TCConfig: TCConfig{
				LayerConfig: []map[string]interface{}{
					{
						hana.ConfigKeyGeomType:  "multilinestring",
						hana.ConfigKeyGeomField: "geom",
						hana.ConfigKeyLayerName: "rivers",
						hana.ConfigKeySQL:       `SELECT * FROM (SELECT "geom".ST_Transform(3857) AS "geom" FROM "TEGOLACI"."ne_50m_rivers_lake_centerlines") AS sub WHERE !BBOX!`,
						hana.ConfigKeySRID:      3857,
					},
				},
			},
			layerNames: []string{"rivers"},
			mvtTile:    make([]byte, 6676),
			tile:       provider.NewTile(2, 1, 1, 16, 4326),
		},
	}
	for name, tc := range tests {
		t.Run(name, fn(tc))
	}
}
