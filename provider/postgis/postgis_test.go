package postgis_test

import (
	"os"
	"testing"

	"context"

	"github.com/go-spatial/geom/slippy"
	"github.com/go-spatial/tegola"
	"github.com/go-spatial/tegola/dict"
	"github.com/go-spatial/tegola/provider"
	"github.com/go-spatial/tegola/provider/postgis"
	"github.com/jackc/pgx"
)

func TestTLSConfig(t *testing.T) {

	testConnConfig := pgx.ConnConfig{
		Host:     "testhost",
		Port:     8080,
		Database: "testdb",
		User:     "testuser",
		Password: "testpassword",
	}

	type tcase struct {
		sslMode     string
		sslKey      string
		sslCert     string
		sslRootCert string
		testFunc    func(config pgx.ConnConfig)
		shouldError bool
	}

	fn := func(t *testing.T, tc tcase) {
		err := postgis.ConfigTLS(tc.sslMode, tc.sslKey, tc.sslCert, tc.sslRootCert, &testConnConfig)
		if !tc.shouldError && err != nil {
			t.Errorf("unable to create a new provider: %v", err)
			return
		} else if tc.shouldError && err == nil {
			t.Errorf("Error expected but got no error")
			return
		}

		tc.testFunc(testConnConfig)
	}

	tests := map[string]tcase{
		"1": {
			sslMode:     "",
			sslKey:      "",
			sslCert:     "",
			sslRootCert: "",
			shouldError: true,
			testFunc: func(config pgx.ConnConfig) {
			},
		},
		"2": {
			sslMode:     "disable",
			sslKey:      "",
			sslCert:     "",
			sslRootCert: "",
			shouldError: false,
			testFunc: func(config pgx.ConnConfig) {
				if config.UseFallbackTLS != false {
					t.Error("When using disable ssl mode; UseFallbackTLS, expected false got true")
				}

				if config.TLSConfig != nil {
					t.Errorf("When using disable ssl mode; UseFallbackTLS, expected nil got %v", testConnConfig.TLSConfig)
				}

				if config.FallbackTLSConfig != nil {
					t.Errorf("When using disable ssl mode; UseFallbackTLS, expected nil got %v", testConnConfig.FallbackTLSConfig)
				}
			},
		},
		"3": {
			sslMode:     "allow",
			sslKey:      "",
			sslCert:     "",
			sslRootCert: "",
			shouldError: false,
			testFunc: func(config pgx.ConnConfig) {
				if config.UseFallbackTLS != true {
					t.Error("When using allow ssl mode; UseFallbackTLS, expected true got false")
				}

				if config.FallbackTLSConfig == nil {
					t.Error("When using allow ssl mode; UseFallbackTLS, expected not nil got nil")
				}

				if config.FallbackTLSConfig != nil && config.FallbackTLSConfig.InsecureSkipVerify == false {
					t.Error("When using allow ssl mode; UseFallbackTLS.InsecureSkipVerify, expected true got false")
				}
			},
		},
		"4": {
			sslMode:     "prefer",
			sslKey:      "",
			sslCert:     "",
			sslRootCert: "",
			shouldError: false,
			testFunc: func(config pgx.ConnConfig) {
				if config.UseFallbackTLS != true {
					t.Error("When using prefer ssl mode; UseFallbackTLS, expected true got false")
				}

				if config.FallbackTLSConfig != nil {
					t.Errorf("When using prefer ssl mode; UseFallbackTLS, expected nil got %v", config.FallbackTLSConfig)
				}

				if config.TLSConfig == nil {
					t.Error("When using prefer ssl mode; TLSConfig, expected not nil got nil")
				}

				if config.TLSConfig != nil && config.TLSConfig.InsecureSkipVerify == false {
					t.Error("When using prefer ssl mode; TLSConfig.InsecureSkipVerify, expected true got false")
				}
			},
		},
		"5": {
			sslMode:     "require",
			sslKey:      "",
			sslCert:     "",
			sslRootCert: "",
			shouldError: false,
			testFunc: func(config pgx.ConnConfig) {
				if config.TLSConfig == nil {
					t.Error("When using prefer ssl mode; TLSConfig, expected not nil got nil")
				}

				if config.TLSConfig != nil && config.TLSConfig.InsecureSkipVerify == false {
					t.Error("When using prefer ssl mode; TLSConfig.InsecureSkipVerify, expected true got false")
				}
			},
		},
		"6": {
			sslMode:     "verify-ca",
			sslKey:      "",
			sslCert:     "",
			sslRootCert: "",
			shouldError: false,
			testFunc: func(config pgx.ConnConfig) {
				if config.TLSConfig == nil {
					t.Error("When using prefer ssl mode; TLSConfig, expected not nil got nil")
				}

				if config.TLSConfig != nil && config.TLSConfig.ServerName != testConnConfig.Host {
					t.Errorf("When using prefer ssl mode; TLSConfig.ServerName, expected %s got %s", testConnConfig.Host, config.TLSConfig.ServerName)
				}
			},
		},
		"7": {
			sslMode:     "verify-full",
			sslKey:      "",
			sslCert:     "",
			sslRootCert: "",
			shouldError: false,
			testFunc: func(config pgx.ConnConfig) {
				if config.TLSConfig == nil {
					t.Error("When using prefer ssl mode; TLSConfig, expected not nil got nil")
				}

				if config.TLSConfig != nil && config.TLSConfig.ServerName != testConnConfig.Host {
					t.Errorf("When using prefer ssl mode; TLSConfig.ServerName, expected %s got %s", testConnConfig.Host, config.TLSConfig.ServerName)
				}
			},
		},
	}

	for name, tc := range tests {
		tc := tc
		t.Run(name, func(t *testing.T) { fn(t, tc) })
	}
}

func TestNewTileProvider(t *testing.T) {
	port := postgis.GetTestPort(t)

	type tcase struct {
		config dict.Dict
	}

	fn := func(t *testing.T, tc tcase) {
		_, err := postgis.NewTileProvider(tc.config)
		if err != nil {
			t.Errorf("unable to create a new provider. err: %v", err)
			return
		}
	}

	tests := map[string]tcase{
		"1": {
			config: dict.Dict{
				postgis.ConfigKeyHost:        os.Getenv("PGHOST"),
				postgis.ConfigKeyPort:        port,
				postgis.ConfigKeyDB:          os.Getenv("PGDATABASE"),
				postgis.ConfigKeyUser:        os.Getenv("PGUSER"),
				postgis.ConfigKeyPassword:    os.Getenv("PGPASSWORD"),
				postgis.ConfigKeySSLMode:     os.Getenv("PGSSLMODE"),
				postgis.ConfigKeySSLKey:      os.Getenv("PGSSLKEY"),
				postgis.ConfigKeySSLCert:     os.Getenv("PGSSLCERT"),
				postgis.ConfigKeySSLRootCert: os.Getenv("PGSSLROOTCERT"),
				postgis.ConfigKeyLayers: []map[string]interface{}{
					{
						postgis.ConfigKeyLayerName: "land",
						postgis.ConfigKeyTablename: "ne_10m_land_scale_rank",
					},
				},
			},
		},
	}

	for name, tc := range tests {
		tc := tc
		t.Run(name, func(t *testing.T) { fn(t, tc) })
	}
}

func TestTileFeatures(t *testing.T) {
	port := postgis.GetTestPort(t)

	type tcase struct {
		config               dict.Dict
		tile                 *slippy.Tile
		expectedFeatureCount int
	}

	fn := func(t *testing.T, tc tcase) {
		p, err := postgis.NewTileProvider(tc.config)
		if err != nil {
			t.Errorf("unexpected error; unable to create a new provider, expected: nil Got %v", err)
			return
		}

		// iterate our configured layers
		for _, tcLayer := range tc.config[postgis.ConfigKeyLayers].([]map[string]interface{}) {
			layerName := tcLayer[postgis.ConfigKeyLayerName].(string)

			var featureCount int
			err := p.TileFeatures(context.Background(), layerName, tc.tile, func(f *provider.Feature) error {
				featureCount++

				return nil
			})
			if err != nil {
				t.Errorf("unexpected error; failed to create mvt layer, expected nil got %v", err)
				return
			}

			if featureCount != tc.expectedFeatureCount {
				t.Errorf("feature count, expected %v got %v", tc.expectedFeatureCount, featureCount)
				return
			}
		}
	}

	tests := map[string]tcase{
		"land query": {
			config: dict.Dict{
				postgis.ConfigKeyHost:        os.Getenv("PGHOST"),
				postgis.ConfigKeyPort:        port,
				postgis.ConfigKeyDB:          os.Getenv("PGDATABASE"),
				postgis.ConfigKeyUser:        os.Getenv("PGUSER"),
				postgis.ConfigKeyPassword:    os.Getenv("PGPASSWORD"),
				postgis.ConfigKeySSLMode:     os.Getenv("PGSSLMODE"),
				postgis.ConfigKeySSLKey:      os.Getenv("PGSSLKEY"),
				postgis.ConfigKeySSLCert:     os.Getenv("PGSSLCERT"),
				postgis.ConfigKeySSLRootCert: os.Getenv("PGSSLROOTCERT"),
				postgis.ConfigKeyLayers: []map[string]interface{}{
					{
						postgis.ConfigKeyLayerName: "land",
						postgis.ConfigKeyTablename: "ne_10m_land_scale_rank",
					},
				},
			},
			tile:                 slippy.NewTile(1, 1, 1, 64, tegola.WebMercator),
			expectedFeatureCount: 4032,
		},
		"scalerank test": {
			config: dict.Dict{
				postgis.ConfigKeyHost:        os.Getenv("PGHOST"),
				postgis.ConfigKeyPort:        port,
				postgis.ConfigKeyDB:          os.Getenv("PGDATABASE"),
				postgis.ConfigKeyUser:        os.Getenv("PGUSER"),
				postgis.ConfigKeyPassword:    os.Getenv("PGPASSWORD"),
				postgis.ConfigKeySSLMode:     os.Getenv("PGSSLMODE"),
				postgis.ConfigKeySSLKey:      os.Getenv("PGSSLKEY"),
				postgis.ConfigKeySSLCert:     os.Getenv("PGSSLCERT"),
				postgis.ConfigKeySSLRootCert: os.Getenv("PGSSLROOTCERT"),
				postgis.ConfigKeyLayers: []map[string]interface{}{
					{
						postgis.ConfigKeyLayerName: "land",
						postgis.ConfigKeySQL:       "SELECT gid, ST_AsBinary(geom) AS geom FROM ne_10m_land_scale_rank WHERE scalerank=!ZOOM! AND geom && !BBOX!",
					},
				},
			},
			tile:                 slippy.NewTile(1, 1, 1, 64, tegola.WebMercator),
			expectedFeatureCount: 98,
		},
		"decode numeric(x,x) types": {
			config: dict.Dict{
				postgis.ConfigKeyHost:        os.Getenv("PGHOST"),
				postgis.ConfigKeyPort:        port,
				postgis.ConfigKeyDB:          os.Getenv("PGDATABASE"),
				postgis.ConfigKeyUser:        os.Getenv("PGUSER"),
				postgis.ConfigKeyPassword:    os.Getenv("PGPASSWORD"),
				postgis.ConfigKeySSLMode:     os.Getenv("PGSSLMODE"),
				postgis.ConfigKeySSLKey:      os.Getenv("PGSSLKEY"),
				postgis.ConfigKeySSLCert:     os.Getenv("PGSSLCERT"),
				postgis.ConfigKeySSLRootCert: os.Getenv("PGSSLROOTCERT"),
				postgis.ConfigKeyLayers: []map[string]interface{}{
					{
						postgis.ConfigKeyLayerName:   "buildings",
						postgis.ConfigKeyGeomIDField: "osm_id",
						postgis.ConfigKeyGeomField:   "geometry",
						postgis.ConfigKeySQL:         "SELECT ST_AsBinary(geometry) AS geometry, osm_id, name, nullif(as_numeric(height),-1) AS height, type FROM osm_buildings_test WHERE geometry && !BBOX!",
					},
				},
			},
			tile:                 slippy.NewTile(16, 11241, 26168, 64, tegola.WebMercator),
			expectedFeatureCount: 101,
		},
	}

	for name, tc := range tests {
		tc := tc
		t.Run(name, func(t *testing.T) { fn(t, tc) })
	}
}
