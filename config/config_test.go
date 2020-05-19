package config_test

import (
	"os"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/go-spatial/tegola/config"
	"github.com/go-spatial/tegola/internal/env"
)

const (
	ENV_TEST_PORT        = ":8888"
	ENV_TEST_CENTER_X    = -76.275329586789
	ENV_TEST_CENTER_Y    = 39.153492567373
	ENV_TEST_CENTER_Z    = 8.0
	ENV_TEST_HOST_1      = "cdn"
	ENV_TEST_HOST_2      = "tegola"
	ENV_TEST_HOST_3      = "io"
	ENV_TEST_HOST_CONCAT = ENV_TEST_HOST_1 + "." + ENV_TEST_HOST_2 + "." + ENV_TEST_HOST_3
)

func setEnv() {
	x := strconv.FormatFloat(ENV_TEST_CENTER_X, 'f', -1, 64)
	y := strconv.FormatFloat(ENV_TEST_CENTER_Y, 'f', -1, 64)
	z := strconv.FormatFloat(ENV_TEST_CENTER_Z, 'f', -1, 64)

	os.Setenv("ENV_TEST_PORT", ENV_TEST_PORT)
	os.Setenv("ENV_TEST_CENTER_X", x)
	os.Setenv("ENV_TEST_CENTER_Y", y)
	os.Setenv("ENV_TEST_CENTER_Z", z)
	os.Setenv("ENV_TEST_HOST_1", ENV_TEST_HOST_1)
	os.Setenv("ENV_TEST_HOST_2", ENV_TEST_HOST_2)
	os.Setenv("ENV_TEST_HOST_3", ENV_TEST_HOST_3)
}

func unsetEnv() {
	os.Unsetenv("ENV_TEST_PORT")
	os.Unsetenv("ENV_TEST_CENTER_X")
	os.Unsetenv("ENV_TEST_CENTER_Y")
	os.Unsetenv("ENV_TEST_CENTER_Z")
}

func TestParse(t *testing.T) {
	type tcase struct {
		config   string
		expected config.Config
	}

	setEnv()
	defer unsetEnv()

	fn := func(t *testing.T, tc tcase) {

		r := strings.NewReader(tc.config)

		conf, err := config.Parse(r, "")
		if err != nil {
			t.Error(err)
			return
		}

		// compare the various parts fo the config
		if !reflect.DeepEqual(conf.LocationName, tc.expected.LocationName) {
			t.Errorf("expected LocationName \n\n %+v \n\n got \n\n %+v ", tc.expected.LocationName, conf.LocationName)
			return
		}

		if !reflect.DeepEqual(conf.Webserver, tc.expected.Webserver) {
			t.Errorf("expected Webserver output \n\n %+v \n\n got \n\n %+v ", tc.expected.Webserver, conf.Webserver)
			return
		}

		if !reflect.DeepEqual(conf.Providers, tc.expected.Providers) {
			t.Errorf("expected Providers output \n\n (%+v) \n\n got \n\n (%+v) ", tc.expected.Providers, conf.Providers)
			return
		}

		if !reflect.DeepEqual(conf.Maps, tc.expected.Maps) {
			t.Errorf("expected Maps output \n\n (%+v) \n\n got \n\n (%+v) ", tc.expected.Maps, conf.Maps)
			return
		}

		if !reflect.DeepEqual(conf, tc.expected) {
			t.Errorf("expected \n\n (%+v) \n\n got \n\n (%+v) ", tc.expected, conf)
			return
		}
	}

	tests := map[string]tcase{
		"1": {
			config: `
				tile_buffer = 12

				[webserver]
				hostname = "cdn.tegola.io"
				port = ":8080"
				cors_allowed_origin = "tegola.io"

					[webserver.headers]
					Access-Control-Allow-Origin = "*"
					Access-Control-Allow-Methods = "GET, OPTIONS"

				[cache]
				type = "file"
				basepath = "/tmp/tegola-cache"

				[[providers]]
				name = "provider1"
				type = "postgis"
				host = "localhost"
				port = 5432
				database = "osm_water" 
				user = "admin"
				password = ""

					[[providers.layers]]
					name = "water"
					geometry_fieldname = "geom"
					id_fieldname = "gid"
					sql = "SELECT gid, ST_AsBinary(geom) AS geom FROM simplified_water_polygons WHERE geom && !BBOX!"

				[[maps]]
				name = "osm"
				attribution = "Test Attribution"
				bounds = [-180.0, -85.05112877980659, 180.0, 85.0511287798066]
				center = [-76.275329586789, 39.153492567373, 8.0]

					[[maps.layers]]
					provider_layer = "provider1.water"
					min_zoom = 10
					max_zoom = 20
					dont_simplify = true
					dont_clip = true`,
			expected: config.Config{
				TileBuffer:   env.IntPtr(env.Int(12)),
				LocationName: "",
				Webserver: config.Webserver{
					HostName: "cdn.tegola.io",
					Port:     ":8080",
					Headers: env.Dict{
						"Access-Control-Allow-Origin":  "*",
						"Access-Control-Allow-Methods": "GET, OPTIONS",
					},
				},
				Cache: env.Dict{
					"type":     "file",
					"basepath": "/tmp/tegola-cache",
				},
				Providers: []env.Dict{
					{
						"name":     "provider1",
						"type":     "postgis",
						"host":     "localhost",
						"port":     int64(5432),
						"database": "osm_water",
						"user":     "admin",
						"password": "",
						"layers": []map[string]interface{}{
							{
								"name":               "water",
								"geometry_fieldname": "geom",
								"id_fieldname":       "gid",
								"sql":                "SELECT gid, ST_AsBinary(geom) AS geom FROM simplified_water_polygons WHERE geom && !BBOX!",
							},
						},
					},
				},
				Maps: []config.Map{
					{
						Name:        "osm",
						Attribution: "Test Attribution",
						Bounds:      []env.Float{-180, -85.05112877980659, 180, 85.0511287798066},
						Center:      [3]env.Float{-76.275329586789, 39.153492567373, 8.0},
						TileBuffer:  env.IntPtr(env.Int(12)),
						Layers: []config.MapLayer{
							{
								ProviderLayer: "provider1.water",
								MinZoom:       env.UintPtr(10),
								MaxZoom:       env.UintPtr(20),
								DontSimplify:  true,
								DontClip:      true,
							},
						},
					},
				},
			},
		},
		"2 test env": {
			config: `
				[webserver]
				hostname = "${ENV_TEST_HOST_1}.${ENV_TEST_HOST_2}.${ENV_TEST_HOST_3}"
				port = "${ENV_TEST_PORT}"

				[[providers]]
				name = "provider1"
				type = "postgis"
				host = "localhost"
				port = 5432
				database = "osm_water" 
				user = "admin"
				password = ""

					[[providers.layers]]
					name = "water_0_5"
					geometry_fieldname = "geom"
					id_fieldname = "gid"
					sql = "SELECT gid, ST_AsBinary(geom) AS geom FROM simplified_water_polygons WHERE geom && !BBOX!"

					[[providers.layers]]
					name = "water_6_10"
					geometry_fieldname = "geom"
					id_fieldname = "gid"
					sql = "SELECT gid, ST_AsBinary(geom) AS geom FROM simplified_water_polygons WHERE geom && !BBOX!"

				[[maps]]
				name = "osm"
				attribution = "Test Attribution"
				bounds = [-180.0, -85.05112877980659, 180.0, 85.0511287798066]
				center = ["${ENV_TEST_CENTER_X}", "${ENV_TEST_CENTER_Y}", "${ENV_TEST_CENTER_Z}"]

					[[maps.layers]]
					name = "water"
					provider_layer = "provider1.water_0_5"

					[[maps.layers]]
					name = "water"
					provider_layer = "provider1.water_6_10"
					min_zoom = 6
					max_zoom = 10

				[[maps]]
				name = "osm_2"
				attribution = "Test Attribution"
				bounds = [-180.0, -85.05112877980659, 180.0, 85.0511287798066]
				center = [-76.275329586789, 39.153492567373, 8.0]

					[[maps.layers]]
					name = "water"
					provider_layer = "provider1.water_0_5"
					min_zoom = 0
					max_zoom = 5

					[[maps.layers]]
					name = "water"
					provider_layer = "provider1.water_6_10"
					min_zoom = 6
					max_zoom = 10`,
			expected: config.Config{
				LocationName: "",
				Webserver: config.Webserver{
					HostName: ENV_TEST_HOST_CONCAT,
					Port:     ENV_TEST_PORT,
				},
				Providers: []env.Dict{
					{
						"name":     "provider1",
						"type":     "postgis",
						"host":     "localhost",
						"port":     int64(5432),
						"database": "osm_water",
						"user":     "admin",
						"password": "",
						"layers": []map[string]interface{}{
							{
								"name":               "water_0_5",
								"geometry_fieldname": "geom",
								"id_fieldname":       "gid",
								"sql":                "SELECT gid, ST_AsBinary(geom) AS geom FROM simplified_water_polygons WHERE geom && !BBOX!",
							},
							{
								"name":               "water_6_10",
								"geometry_fieldname": "geom",
								"id_fieldname":       "gid",
								"sql":                "SELECT gid, ST_AsBinary(geom) AS geom FROM simplified_water_polygons WHERE geom && !BBOX!",
							},
						},
					},
				},
				Maps: []config.Map{
					{
						Name:        "osm",
						Attribution: "Test Attribution",
						Bounds:      []env.Float{-180, -85.05112877980659, 180, 85.0511287798066},
						Center:      [3]env.Float{ENV_TEST_CENTER_X, ENV_TEST_CENTER_Y, ENV_TEST_CENTER_Z},
						TileBuffer:  env.IntPtr(env.Int(64)),
						Layers: []config.MapLayer{
							{
								Name:          "water",
								ProviderLayer: "provider1.water_0_5",
								MinZoom:       nil,
								MaxZoom:       nil,
							},
							{
								Name:          "water",
								ProviderLayer: "provider1.water_6_10",
								MinZoom:       env.UintPtr(6),
								MaxZoom:       env.UintPtr(10),
							},
						},
					},
					{
						Name:        "osm_2",
						Attribution: "Test Attribution",
						Bounds:      []env.Float{-180, -85.05112877980659, 180, 85.0511287798066},
						Center:      [3]env.Float{-76.275329586789, 39.153492567373, 8.0},
						TileBuffer:  env.IntPtr(env.Int(64)),
						Layers: []config.MapLayer{
							{
								Name:          "water",
								ProviderLayer: "provider1.water_0_5",
								MinZoom:       env.UintPtr(0),
								MaxZoom:       env.UintPtr(5),
							},
							{
								Name:          "water",
								ProviderLayer: "provider1.water_6_10",
								MinZoom:       env.UintPtr(6),
								MaxZoom:       env.UintPtr(10),
							},
						},
					},
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

func TestValidate(t *testing.T) {
	type tcase struct {
		config      config.Config
		expectedErr error
	}

	fn := func(t *testing.T, tc tcase) {
		t.Parallel()

		err := tc.config.Validate()
		if err != tc.expectedErr {
			t.Errorf("expected err: %v got %v", tc.expectedErr, err)
			return
		}
	}

	tests := map[string]tcase{
		"1": {
			config: config.Config{
				LocationName: "",
				Webserver: config.Webserver{
					Port: ":8080",
				},
				Providers: []env.Dict{
					{
						"name":     "provider1",
						"type":     "postgis",
						"host":     "localhost",
						"port":     int64(5432),
						"database": "osm_water",
						"user":     "admin",
						"password": "",
						"layers": []map[string]interface{}{
							{
								"name":               "water",
								"geometry_fieldname": "geom",
								"id_fieldname":       "gid",
								"sql":                "SELECT gid, ST_AsBinary(geom) AS geom FROM simplified_water_polygons WHERE geom && !BBOX!",
							},
						},
					},
					{
						"name":     "provider2",
						"type":     "postgis",
						"host":     "localhost",
						"port":     int64(5432),
						"database": "osm_water",
						"user":     "admin",
						"password": "",
						"layers": []map[string]interface{}{
							{
								"name":               "water",
								"geometry_fieldname": "geom",
								"id_fieldname":       "gid",
								"sql":                "SELECT gid, ST_AsBinary(geom) AS geom FROM simplified_water_polygons WHERE geom && !BBOX!",
							},
						},
					},
				},
				Maps: []config.Map{
					{
						Name:        "osm",
						Attribution: "Test Attribution",
						Bounds:      []env.Float{-180, -85.05112877980659, 180, 85.0511287798066},
						Center:      [3]env.Float{-76.275329586789, 39.153492567373, 8.0},
						Layers: []config.MapLayer{
							{
								ProviderLayer: "provider1.water",
								MinZoom:       env.UintPtr(10),
								MaxZoom:       env.UintPtr(20),
							},
							{
								ProviderLayer: "provider2.water",
								MinZoom:       env.UintPtr(10),
								MaxZoom:       env.UintPtr(20),
							},
						},
					},
				},
			},
			expectedErr: config.ErrOverlappingLayerZooms{
				ProviderLayer1: "provider1.water",
				ProviderLayer2: "provider2.water",
			},
		},
		"2": {
			config: config.Config{
				Providers: []env.Dict{
					{
						"name":     "provider1",
						"type":     "postgis",
						"host":     "localhost",
						"port":     int64(5432),
						"database": "osm_water",
						"user":     "admin",
						"password": "",
						"layers": []map[string]interface{}{
							{
								"name":               "water_0_5",
								"geometry_fieldname": "geom",
								"id_fieldname":       "gid",
								"sql":                "SELECT gid, ST_AsBinary(geom) AS geom FROM simplified_water_polygons WHERE geom && !BBOX!",
							},
						},
					},
					{
						"name":     "provider2",
						"type":     "postgis",
						"host":     "localhost",
						"port":     int64(5432),
						"database": "osm_water",
						"user":     "admin",
						"password": "",
						"layers": []map[string]interface{}{
							{
								"name":               "water_5_10",
								"geometry_fieldname": "geom",
								"id_fieldname":       "gid",
								"sql":                "SELECT gid, ST_AsBinary(geom) AS geom FROM simplified_water_polygons WHERE geom && !BBOX!",
							},
						},
					},
				},
				Maps: []config.Map{
					{
						Name:        "osm",
						Attribution: "Test Attribution",
						Bounds:      []env.Float{-180, -85.05112877980659, 180, 85.0511287798066},
						Center:      [3]env.Float{-76.275329586789, 39.153492567373, 8.0},
						Layers: []config.MapLayer{
							{
								Name:          "water",
								ProviderLayer: "provider1.water_0_5",
								MinZoom:       env.UintPtr(0),
								MaxZoom:       env.UintPtr(5),
							},
							{
								Name:          "water",
								ProviderLayer: "provider2.water_5_10",
								MinZoom:       env.UintPtr(5),
								MaxZoom:       env.UintPtr(10),
							},
						},
					},
				},
			},
			expectedErr: config.ErrOverlappingLayerZooms{
				ProviderLayer1: "provider1.water_0_5",
				ProviderLayer2: "provider2.water_5_10",
			},
		},
		"3": {
			config: config.Config{
				LocationName: "",
				Webserver: config.Webserver{
					Port: ":8080",
				},
				Providers: []env.Dict{
					{
						"name":     "provider1",
						"type":     "postgis",
						"host":     "localhost",
						"port":     int64(5432),
						"database": "osm_water",
						"user":     "admin",
						"password": "",
						"layers": []map[string]interface{}{
							{
								"name":               "water",
								"geometry_fieldname": "geom",
								"id_fieldname":       "gid",
								"sql":                "SELECT gid, ST_AsBinary(geom) AS geom FROM simplified_water_polygons WHERE geom && !BBOX!",
							},
						},
					},
					{
						"name":     "provider2",
						"type":     "postgis",
						"host":     "localhost",
						"port":     int64(5432),
						"database": "osm_water",
						"user":     "admin",
						"password": "",
						"layers": []map[string]interface{}{
							{
								"name":               "water",
								"geometry_fieldname": "geom",
								"id_fieldname":       "gid",
								"sql":                "SELECT gid, ST_AsBinary(geom) AS geom FROM simplified_water_polygons WHERE geom && !BBOX!",
							},
						},
					},
				},
				Maps: []config.Map{
					{
						Name:        "osm",
						Attribution: "Test Attribution",
						Bounds:      []env.Float{-180, -85.05112877980659, 180, 85.0511287798066},
						Center:      [3]env.Float{-76.275329586789, 39.153492567373, 8.0},
						Layers: []config.MapLayer{
							{
								ProviderLayer: "provider1.water",
								MinZoom:       env.UintPtr(10),
								MaxZoom:       env.UintPtr(15),
							},
							{
								ProviderLayer: "provider2.water",
								MinZoom:       env.UintPtr(16),
								MaxZoom:       env.UintPtr(20),
							},
						},
					},
					{
						Name:        "osm_2",
						Attribution: "Test Attribution",
						Bounds:      []env.Float{-180, -85.05112877980659, 180, 85.0511287798066},
						Center:      [3]env.Float{-76.275329586789, 39.153492567373, 8.0},
						Layers: []config.MapLayer{
							{
								ProviderLayer: "provider1.water",
								MinZoom:       env.UintPtr(10),
								MaxZoom:       env.UintPtr(15),
							},
							{
								ProviderLayer: "provider2.water",
								MinZoom:       env.UintPtr(16),
								MaxZoom:       env.UintPtr(20),
							},
						},
					},
				},
			},
			expectedErr: nil,
		},
		"4 default zooms": {
			config: config.Config{
				LocationName: "",
				Webserver: config.Webserver{
					Port: ":8080",
				},
				Providers: []env.Dict{
					{
						"name":     "provider1",
						"type":     "postgis",
						"host":     "localhost",
						"port":     int64(5432),
						"database": "osm_water",
						"user":     "admin",
						"password": "",
						"layers": []map[string]interface{}{
							{
								"name":               "water",
								"geometry_fieldname": "geom",
								"id_fieldname":       "gid",
								"sql":                "SELECT gid, ST_AsBinary(geom) AS geom FROM simplified_water_polygons WHERE geom && !BBOX!",
							},
						},
					},
					{
						"name":     "provider2",
						"type":     "postgis",
						"host":     "localhost",
						"port":     int64(5432),
						"database": "osm_water",
						"user":     "admin",
						"password": "",
						"layers": []map[string]interface{}{
							{
								"name":               "water",
								"geometry_fieldname": "geom",
								"id_fieldname":       "gid",
								"sql":                "SELECT gid, ST_AsBinary(geom) AS geom FROM simplified_water_polygons WHERE geom && !BBOX!",
							},
						},
					},
				},
				Maps: []config.Map{
					{
						Name:        "osm",
						Attribution: "Test Attribution",
						Bounds:      []env.Float{-180, -85.05112877980659, 180, 85.0511287798066},
						Center:      [3]env.Float{-76.275329586789, 39.153492567373, 8.0},
						Layers: []config.MapLayer{
							{
								ProviderLayer: "provider1.water",
							},
						},
					},
					{
						Name:        "osm_2",
						Attribution: "Test Attribution",
						Bounds:      []env.Float{-180, -85.05112877980659, 180, 85.0511287798066},
						Center:      [3]env.Float{-76.275329586789, 39.153492567373, 8.0},
						Layers: []config.MapLayer{
							{
								ProviderLayer: "provider2.water",
							},
						},
					},
				},
			},
			expectedErr: nil,
		},
		"5 default zooms fail": {
			config: config.Config{
				LocationName: "",
				Webserver: config.Webserver{
					Port: ":8080",
				},
				Providers: []env.Dict{
					{
						"name":     "provider1",
						"type":     "postgis",
						"host":     "localhost",
						"port":     int64(5432),
						"database": "osm_water",
						"user":     "admin",
						"password": "",
						"layers": []map[string]interface{}{
							{
								"name":               "water",
								"geometry_fieldname": "geom",
								"id_fieldname":       "gid",
								"sql":                "SELECT gid, ST_AsBinary(geom) AS geom FROM simplified_water_polygons WHERE geom && !BBOX!",
							},
						},
					},
					{
						"name":     "provider2",
						"type":     "postgis",
						"host":     "localhost",
						"port":     int64(5432),
						"database": "osm_water",
						"user":     "admin",
						"password": "",
						"layers": []map[string]interface{}{
							{
								"name":               "water",
								"geometry_fieldname": "geom",
								"id_fieldname":       "gid",
								"sql":                "SELECT gid, ST_AsBinary(geom) AS geom FROM simplified_water_polygons WHERE geom && !BBOX!",
							},
						},
					},
				},
				Maps: []config.Map{
					{
						Name:        "osm",
						Attribution: "Test Attribution",
						Bounds:      []env.Float{-180, -85.05112877980659, 180, 85.0511287798066},
						Center:      [3]env.Float{-76.275329586789, 39.153492567373, 8.0},
						Layers: []config.MapLayer{
							{
								ProviderLayer: "provider1.water_default_z",
							},
							{
								ProviderLayer: "provider2.water_default_z",
							},
						},
					},
				},
			},
			expectedErr: config.ErrOverlappingLayerZooms{
				ProviderLayer1: "provider1.water_default_z",
				ProviderLayer2: "provider2.water_default_z",
			},
		},
		"6 blocked headers": {
			config: config.Config{
				LocationName: "",
				Webserver: config.Webserver{
					Port: ":8080",
					Headers: env.Dict{
						"Content-Encoding": "plain/text",
					},
				},
			},
			expectedErr: config.ErrInvalidHeader{
				Header: "Content-Encoding",
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

func TestConfigureTileBuffers(t *testing.T) {
	type tcase struct {
		config   config.Config
		expected config.Config
	}

	fn := func(t *testing.T, tc tcase) {
		t.Parallel()

		tc.config.ConfigureTileBuffers()
		if !reflect.DeepEqual(tc.expected, tc.config) {
			t.Errorf("expected \n\n %+v \n\n got \n\n %+v", tc.expected, tc.config)
			return
		}
	}

	tests := map[string]tcase{
		"1 tilebuffer is not set": {
			config: config.Config{
				Maps: []config.Map{
					{
						Name: "osm",
					},
				},
			},
			expected: config.Config{
				Maps: []config.Map{
					{
						Name:       "osm",
						TileBuffer: env.IntPtr(env.Int(64)),
					},
				},
			},
		},
		"2 tilebuffer is set in global section": {
			config: config.Config{
				TileBuffer: env.IntPtr(env.Int(32)),
				Maps: []config.Map{
					{
						Name: "osm",
					},
					{
						Name: "osm-2",
					},
				},
			},
			expected: config.Config{
				TileBuffer: env.IntPtr(env.Int(32)),
				Maps: []config.Map{
					{
						Name:       "osm",
						TileBuffer: env.IntPtr(env.Int(32)),
					},
					{
						Name:       "osm-2",
						TileBuffer: env.IntPtr(env.Int(32)),
					},
				},
			},
		},
		"3 tilebuffer is set in map section": {
			config: config.Config{
				Maps: []config.Map{
					{
						Name:       "osm",
						TileBuffer: env.IntPtr(env.Int(16)),
					},
					{
						Name:       "osm-2",
						TileBuffer: env.IntPtr(env.Int(32)),
					},
				},
			},
			expected: config.Config{
				Maps: []config.Map{
					{
						Name:       "osm",
						TileBuffer: env.IntPtr(env.Int(16)),
					},
					{
						Name:       "osm-2",
						TileBuffer: env.IntPtr(env.Int(32)),
					},
				},
			},
		},
		"4 tilebuffer is set in global and map sections": {
			config: config.Config{
				TileBuffer: env.IntPtr(env.Int(32)),
				Maps: []config.Map{
					{
						Name:       "osm",
						TileBuffer: env.IntPtr(env.Int(16)),
					},
				},
			},
			expected: config.Config{
				TileBuffer: env.IntPtr(env.Int(32)),
				Maps: []config.Map{
					{
						Name:       "osm",
						TileBuffer: env.IntPtr(env.Int(16)),
					},
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
