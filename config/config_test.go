package config_test

import (
	"errors"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/go-spatial/tegola/config"
	"github.com/go-spatial/tegola/internal/env"
	"github.com/go-spatial/tegola/provider"
	_ "github.com/go-spatial/tegola/provider/debug"
	_ "github.com/go-spatial/tegola/provider/postgis"
	_ "github.com/go-spatial/tegola/provider/test"
)

const (
	ENV_TEST_PORT                    = ":8888"
	ENV_TEST_CENTER_X                = -76.275329586789
	ENV_TEST_CENTER_Y                = 39.153492567373
	ENV_TEST_CENTER_Z                = 8.0
	ENV_TEST_HOST_1                  = "cdn"
	ENV_TEST_HOST_2                  = "tegola"
	ENV_TEST_HOST_3                  = "io"
	ENV_TEST_HOST_CONCAT             = ENV_TEST_HOST_1 + "." + ENV_TEST_HOST_2 + "." + ENV_TEST_HOST_3
	ENV_TEST_WEBSERVER_HEADER_STRING = "s-maxage=10"
	ENV_TEST_WEBSERVER_PORT          = "1234"
	ENV_TEST_PROVIDER_LAYER          = "provider1.water_0_5"
	ENV_TEST_MAP_LAYER_DEFAULT_TAG   = "postgis"
)

func setEnv(t *testing.T) {
	x := strconv.FormatFloat(ENV_TEST_CENTER_X, 'f', -1, 64)
	y := strconv.FormatFloat(ENV_TEST_CENTER_Y, 'f', -1, 64)
	z := strconv.FormatFloat(ENV_TEST_CENTER_Z, 'f', -1, 64)

	t.Setenv("ENV_TEST_PORT", ENV_TEST_PORT)
	t.Setenv("ENV_TEST_CENTER_X", x)
	t.Setenv("ENV_TEST_CENTER_Y", y)
	t.Setenv("ENV_TEST_CENTER_Z", z)
	t.Setenv("ENV_TEST_HOST_1", ENV_TEST_HOST_1)
	t.Setenv("ENV_TEST_HOST_2", ENV_TEST_HOST_2)
	t.Setenv("ENV_TEST_HOST_3", ENV_TEST_HOST_3)
	t.Setenv("ENV_TEST_WEBSERVER_HEADER_STRING", ENV_TEST_WEBSERVER_HEADER_STRING)
	t.Setenv("ENV_TEST_WEBSERVER_PORT", ENV_TEST_WEBSERVER_PORT)
	t.Setenv("ENV_TEST_PROVIDER_LAYER", ENV_TEST_PROVIDER_LAYER)
	t.Setenv("ENV_TEST_MAP_LAYER_DEFAULT_TAG", ENV_TEST_MAP_LAYER_DEFAULT_TAG)
}

func TestParse(t *testing.T) {
	type tcase struct {
		config      string
		expected    config.Config
		expectedErr error
	}

	setEnv(t)

	fn := func(tc tcase) func(*testing.T) {
		return func(t *testing.T) {

			r := strings.NewReader(tc.config)

			conf, err := config.Parse(r, "")

			if tc.expectedErr != nil {
				if err == nil {
					t.Errorf("expected err %v, got nil", tc.expectedErr.Error())
					return
				}

				// compare error messages
				if tc.expectedErr.Error() != err.Error() {
					t.Errorf("invalid error. expected %v, got %v", tc.expectedErr, err)
					return
				}

				return
			}

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
	}

	tests := map[string]tcase{
		"happy path": {
			config: `
				tile_buffer = 12

				[webserver]
				hostname = "cdn.tegola.io"
				port = ":8080"
				cors_allowed_origin = "tegola.io"
        		proxy_protocol = "https"

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
					dont_clip = true
					dont_clean = true

					[[maps.params]]
					name = "param1"
					token = "!param1!"
					type = "string"
					
					[[maps.params]]
					name = "param2"
					token = "!PARAM2!"
					type = "int"
					sql = "AND answer = ?"
					default_value = "42"
					
					[[maps.params]]
					name = "param3"
					token = "!PARAM3!"
					type = "float"
					default_sql = "AND pi = 3.1415926"
					`,
			expected: config.Config{
				TileBuffer:   env.IntPtr(env.Int(12)),
				LocationName: "",
				Webserver: config.Webserver{
					HostName:      "cdn.tegola.io",
					Port:          ":8080",
					ProxyProtocol: "https",
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
						"layers": []map[string]any{
							{
								"name":               "water",
								"geometry_fieldname": "geom",
								"id_fieldname":       "gid",
								"sql":                "SELECT gid, ST_AsBinary(geom) AS geom FROM simplified_water_polygons WHERE geom && !BBOX!",
							},
						},
					},
				},
				Maps: []provider.Map{
					{
						Name:        "osm",
						Attribution: "Test Attribution",
						Bounds:      []env.Float{-180, -85.05112877980659, 180, 85.0511287798066},
						Center:      [3]env.Float{-76.275329586789, 39.153492567373, 8.0},
						TileBuffer:  env.IntPtr(env.Int(12)),
						Layers: []provider.MapLayer{
							{
								ProviderLayer: "provider1.water",
								MinZoom:       env.UintPtr(10),
								MaxZoom:       env.UintPtr(20),
								DontSimplify:  true,
								DontClip:      true,
								DontClean:     true,
							},
						},
						Parameters: []provider.QueryParameter{
							{
								Name:  "param1",
								Token: "!PARAM1!",
								SQL:   "?",
								Type:  "string",
							},
							{
								Name:         "param2",
								Token:        "!PARAM2!",
								Type:         "int",
								SQL:          "AND answer = ?",
								DefaultValue: "42",
							},
							{
								Name:       "param3",
								Token:      "!PARAM3!",
								Type:       "float",
								SQL:        "?",
								DefaultSQL: "AND pi = 3.1415926",
							},
						},
					},
				},
			},
		},
		"test env": {
			config: `
				[webserver]
				hostname = "${ENV_TEST_HOST_1}.${ENV_TEST_HOST_2}.${ENV_TEST_HOST_3}"
				port = "${ENV_TEST_WEBSERVER_PORT}"
                
                [webserver.headers]
                   Cache-Control = "${ENV_TEST_WEBSERVER_HEADER_STRING}"
				   Test = "Test"
                   # impossible but to test ParseDict
                   Impossible-Header = {"test" = "${ENV_TEST_WEBSERVER_HEADER_STRING}"}

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
					provider_layer = "${ENV_TEST_PROVIDER_LAYER}"

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

                    [maps.layers.default_tags]
                    provider = "${ENV_TEST_MAP_LAYER_DEFAULT_TAG}"

					[[maps.layers]]
					name = "water"
					provider_layer = "provider1.water_6_10"
					min_zoom = 6
					max_zoom = 10`,
			expected: config.Config{
				LocationName: "",
				Webserver: config.Webserver{
					HostName: ENV_TEST_HOST_CONCAT,
					Port:     ENV_TEST_WEBSERVER_PORT,
					Headers: env.Dict{
						"Cache-Control": ENV_TEST_WEBSERVER_HEADER_STRING,
						"Test":          "Test",
						"Impossible-Header": env.Dict{
							"test": ENV_TEST_WEBSERVER_HEADER_STRING,
						},
					},
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
						"layers": []map[string]any{
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
				Maps: []provider.Map{
					{
						Name:        "osm",
						Attribution: "Test Attribution",
						Bounds:      []env.Float{-180, -85.05112877980659, 180, 85.0511287798066},
						Center:      [3]env.Float{ENV_TEST_CENTER_X, ENV_TEST_CENTER_Y, ENV_TEST_CENTER_Z},
						TileBuffer:  env.IntPtr(env.Int(64)),
						Layers: []provider.MapLayer{
							{
								Name:          "water",
								ProviderLayer: ENV_TEST_PROVIDER_LAYER,
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
						Layers: []provider.MapLayer{
							{
								Name:          "water",
								ProviderLayer: "provider1.water_0_5",
								MinZoom:       env.UintPtr(0),
								MaxZoom:       env.UintPtr(5),
								DefaultTags: env.Dict{
									"provider": ENV_TEST_MAP_LAYER_DEFAULT_TAG,
								},
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
		"missing env": {
			config: `
				[webserver]
				hostname = "${ENV_TEST_HOST_1}.${ENV_TEST_HOST_2}.${ENV_TEST_HOST_3}"
				port = "${ENV_TEST_WEBSERVER_PORT}"
                
                [webserver.headers]
                   Cache-Control = "${ENV_TEST_WEBSERVER_HEADER_STRING}"
				   Test = "${I_AM_MISSING}"

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
					provider_layer = "${ENV_TEST_PROVIDER_LAYER}"

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
			expected:    config.Config{},
			expectedErr: env.ErrEnvVar("I_AM_MISSING"),
		},
		"test empty proxy_protocol": {
			config: `
				[webserver]
				hostname = "${ENV_TEST_HOST_1}.${ENV_TEST_HOST_2}.${ENV_TEST_HOST_3}"
				port = "${ENV_TEST_WEBSERVER_PORT}"
        		proxy_protocol = ""
                
                [webserver.headers]
                   Cache-Control = "${ENV_TEST_WEBSERVER_HEADER_STRING}"
				   Test = "Test"
                   # impossible but to test ParseDict
                   Impossible-Header = {"test" = "${ENV_TEST_WEBSERVER_HEADER_STRING}"}

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
					provider_layer = "${ENV_TEST_PROVIDER_LAYER}"

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

                    [maps.layers.default_tags]
                    provider = "${ENV_TEST_MAP_LAYER_DEFAULT_TAG}"

					[[maps.layers]]
					name = "water"
					provider_layer = "provider1.water_6_10"
					min_zoom = 6
					max_zoom = 10`,
			expected: config.Config{
				LocationName: "",
				Webserver: config.Webserver{
					HostName: ENV_TEST_HOST_CONCAT,
					Port:     ENV_TEST_WEBSERVER_PORT,
					Headers: env.Dict{
						"Cache-Control": ENV_TEST_WEBSERVER_HEADER_STRING,
						"Test":          "Test",
						"Impossible-Header": env.Dict{
							"test": ENV_TEST_WEBSERVER_HEADER_STRING,
						},
					},
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
				Maps: []provider.Map{
					{
						Name:        "osm",
						Attribution: "Test Attribution",
						Bounds:      []env.Float{-180, -85.05112877980659, 180, 85.0511287798066},
						Center:      [3]env.Float{ENV_TEST_CENTER_X, ENV_TEST_CENTER_Y, ENV_TEST_CENTER_Z},
						TileBuffer:  env.IntPtr(env.Int(64)),
						Layers: []provider.MapLayer{
							{
								Name:          "water",
								ProviderLayer: ENV_TEST_PROVIDER_LAYER,
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
						Layers: []provider.MapLayer{
							{
								Name:          "water",
								ProviderLayer: "provider1.water_0_5",
								MinZoom:       env.UintPtr(0),
								MaxZoom:       env.UintPtr(5),
								DefaultTags: env.Dict{
									"provider": ENV_TEST_MAP_LAYER_DEFAULT_TAG,
								},
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
		t.Run(name, fn(tc))
	}
}

func TestValidateMutateZoom(t *testing.T) {

	type tcase struct {
		config          *config.Config
		expectedMinZoom int
		expectedMaxZoom int
	}

	fn := func(tc tcase) func(*testing.T) {
		return func(t *testing.T) {
			t.Parallel()

			err := tc.config.Validate()
			if err != nil {
				t.Errorf("an error occured: %v", err)
				return
			}

			minzoom := int(*tc.config.Maps[0].Layers[0].MinZoom)
			if minzoom != tc.expectedMinZoom {
				t.Errorf("expected min zoom: %v, got: %v", tc.expectedMinZoom, minzoom)
			}

			maxzoom := int(*tc.config.Maps[0].Layers[0].MaxZoom)
			if maxzoom != tc.expectedMaxZoom {
				t.Errorf("expected min zoom: %v, got: %v", tc.expectedMaxZoom, maxzoom)
			}
		}
	}

	tests := map[string]tcase{
		"default max zoom": {
			expectedMinZoom: 0,
			expectedMaxZoom: 22,
			config: &config.Config{
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
						"layers": []map[string]any{
							{
								"name":               "water",
								"geometry_fieldname": "geom",
								"id_fieldname":       "gid",
								"sql":                "SELECT gid, ST_AsBinary(geom) AS geom FROM simplified_water_polygons WHERE geom && !BBOX!",
							},
						},
					},
				},
				Maps: []provider.Map{
					{
						Name:        "osm",
						Attribution: "Test Attribution",
						Bounds:      []env.Float{-180, -85.05112877980659, 180, 85.0511287798066},
						Center:      [3]env.Float{-76.275329586789, 39.153492567373, 8.0},
						Layers: []provider.MapLayer{
							{
								ProviderLayer: "provider1.water",
								MinZoom:       nil,
								MaxZoom:       nil,
							},
						},
					},
				},
			},
		},
		"max zoom 0, default to 1": {
			expectedMinZoom: 0,
			expectedMaxZoom: 1,
			config: &config.Config{
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
						"layers": []map[string]any{
							{
								"name":               "water",
								"geometry_fieldname": "geom",
								"id_fieldname":       "gid",
								"sql":                "SELECT gid, ST_AsBinary(geom) AS geom FROM simplified_water_polygons WHERE geom && !BBOX!",
							},
						},
					},
				},
				Maps: []provider.Map{
					{
						Name:        "osm",
						Attribution: "Test Attribution",
						Bounds:      []env.Float{-180, -85.05112877980659, 180, 85.0511287798066},
						Center:      [3]env.Float{-76.275329586789, 39.153492567373, 8.0},
						Layers: []provider.MapLayer{
							{
								ProviderLayer: "provider1.water",
								MinZoom:       env.UintPtr(0),
								MaxZoom:       env.UintPtr(0),
							},
						},
					},
				},
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, fn(tc))
	}

}

func TestValidate(t *testing.T) {
	type tcase struct {
		config      config.Config
		expectedErr error
	}

	fn := func(tc tcase) func(*testing.T) {
		return func(t *testing.T) {
			t.Parallel()

			err := tc.config.Validate()
			if !errors.Is(err, tc.expectedErr) {
				t.Errorf("expected err: %s got %s", tc.expectedErr, err)
				return
			}
		}
	}

	tests := map[string]tcase{
		"happy path 1": {
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
						"layers": []map[string]any{
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
						"layers": []map[string]any{
							{
								"name":               "water",
								"geometry_fieldname": "geom",
								"id_fieldname":       "gid",
								"sql":                "SELECT gid, ST_AsBinary(geom) AS geom FROM simplified_water_polygons WHERE geom && !BBOX!",
							},
						},
					},
				},
				Maps: []provider.Map{
					{
						Name:        "osm",
						Attribution: "Test Attribution",
						Bounds:      []env.Float{-180, -85.05112877980659, 180, 85.0511287798066},
						Center:      [3]env.Float{-76.275329586789, 39.153492567373, 8.0},
						Layers: []provider.MapLayer{
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
		"happy path 2": {
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
						"layers": []map[string]any{
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
						"layers": []map[string]any{
							{
								"name":               "water_5_10",
								"geometry_fieldname": "geom",
								"id_fieldname":       "gid",
								"sql":                "SELECT gid, ST_AsBinary(geom) AS geom FROM simplified_water_polygons WHERE geom && !BBOX!",
							},
						},
					},
				},
				Maps: []provider.Map{
					{
						Name:        "osm",
						Attribution: "Test Attribution",
						Bounds:      []env.Float{-180, -85.05112877980659, 180, 85.0511287798066},
						Center:      [3]env.Float{-76.275329586789, 39.153492567373, 8.0},
						Layers: []provider.MapLayer{
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
		"happy path 3": {
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
						"layers": []map[string]any{
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
						"layers": []map[string]any{
							{
								"name":               "water",
								"geometry_fieldname": "geom",
								"id_fieldname":       "gid",
								"sql":                "SELECT gid, ST_AsBinary(geom) AS geom FROM simplified_water_polygons WHERE geom && !BBOX!",
							},
						},
					},
				},
				Maps: []provider.Map{
					{
						Name:        "osm",
						Attribution: "Test Attribution",
						Bounds:      []env.Float{-180, -85.05112877980659, 180, 85.0511287798066},
						Center:      [3]env.Float{-76.275329586789, 39.153492567373, 8.0},
						Layers: []provider.MapLayer{
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
						Layers: []provider.MapLayer{
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
		"default zooms": {
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
						"layers": []map[string]any{
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
						"layers": []map[string]any{
							{
								"name":               "water",
								"geometry_fieldname": "geom",
								"id_fieldname":       "gid",
								"sql":                "SELECT gid, ST_AsBinary(geom) AS geom FROM simplified_water_polygons WHERE geom && !BBOX!",
							},
						},
					},
				},
				Maps: []provider.Map{
					{
						Name:        "osm",
						Attribution: "Test Attribution",
						Bounds:      []env.Float{-180, -85.05112877980659, 180, 85.0511287798066},
						Center:      [3]env.Float{-76.275329586789, 39.153492567373, 8.0},
						Layers: []provider.MapLayer{
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
						Layers: []provider.MapLayer{
							{
								ProviderLayer: "provider2.water",
							},
						},
					},
				},
			},
			expectedErr: nil,
		},
		"default zooms fail": {
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
						"layers": []map[string]any{
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
						"layers": []map[string]any{
							{
								"name":               "water",
								"geometry_fieldname": "geom",
								"id_fieldname":       "gid",
								"sql":                "SELECT gid, ST_AsBinary(geom) AS geom FROM simplified_water_polygons WHERE geom && !BBOX!",
							},
						},
					},
				},
				Maps: []provider.Map{
					{
						Name:        "osm",
						Attribution: "Test Attribution",
						Bounds:      []env.Float{-180, -85.05112877980659, 180, 85.0511287798066},
						Center:      [3]env.Float{-76.275329586789, 39.153492567373, 8.0},
						Layers: []provider.MapLayer{
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
		"blocked headers": {
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
		"non-existant provider type": {
			expectedErr: config.ErrUnknownProviderType{Type: "nonexistant", Name: "provider1", KnownProviders: []string{"..."}},
			config: config.Config{
				Providers: []env.Dict{
					{
						"name": "provider1",
						"type": "nonexistant",
					},
				},
			},
		},
		"missing name field": {
			expectedErr: config.ErrProviderNameRequired{Pos: 0},
			config: config.Config{
				Providers: []env.Dict{
					{
						"type": "test",
					},
				},
			},
		},
		"duplicate name field": {
			expectedErr: config.ErrProviderNameDuplicate{Pos: 1},
			config: config.Config{
				Providers: []env.Dict{
					{
						"name": "provider1",
						"type": "test",
					},
					{
						"name": "provider1",
						"type": "test",
					},
				},
			},
		},
		"missing name field at pos 1": {
			expectedErr: config.ErrProviderNameRequired{Pos: 1},
			config: config.Config{
				Providers: []env.Dict{
					{
						"name": "provider1",
						"type": "test",
					},
					{
						"type": "test",
					},
				},
			},
		},
		"missing type field": {
			expectedErr: config.ErrProviderTypeRequired{Pos: 0},
			config: config.Config{
				Providers: []env.Dict{
					{
						"name": "provider1",
					},
				},
			},
		},
		"missing type field at pos 1": {
			expectedErr: config.ErrProviderTypeRequired{Pos: 1},
			config: config.Config{
				Providers: []env.Dict{
					{
						"name": "provider1",
						"type": "test",
					},
					{
						"name": "provider2",
					},
				},
			},
		},
		"happy 1 mvt provider only 1 layer": {
			config: config.Config{
				Providers: []env.Dict{
					{
						"name": "provider1",
						"type": "mvt_test",
					},
				},
				Maps: []provider.Map{
					{
						Name:        "happy",
						Attribution: "Test Attribution",
						Layers: []provider.MapLayer{
							{
								ProviderLayer: "provider1.water_default_z",
							},
						},
					},
				},
			},
		},
		"happy 1 mvt provider only 2 layer": {
			config: config.Config{
				Providers: []env.Dict{
					{
						"name": "provider1",
						"type": "mvt_test",
					},
				},
				Maps: []provider.Map{
					{
						Name:        "happy",
						Attribution: "Test Attribution",
						Layers: []provider.MapLayer{
							{
								ProviderLayer: "provider1.water_default_z",
							},
							{
								ProviderLayer: "provider1.land_default_z",
							},
						},
					},
				},
			},
		},
		"happy 1 mvt, 1 std provider only 1 layer": {
			config: config.Config{
				Providers: []env.Dict{
					{
						"name": "provider1",
						"type": "mvt_test",
					},
					{
						"name": "provider2",
						"type": "test",
					},
				},
				Maps: []provider.Map{
					{
						Name:        "happy",
						Attribution: "Test Attribution",
						Layers: []provider.MapLayer{
							{
								ProviderLayer: "provider1.water_default_z",
							},
							{
								ProviderLayer: "provider1.land_default_z",
							},
						},
					},
				},
			},
		},
		"invalid provider referenced in map": {
			expectedErr: config.ErrInvalidProviderForMap{
				MapName:      "happy",
				ProviderName: "bad",
			},
			config: config.Config{
				Providers: []env.Dict{
					{
						"name": "provider1",
						"type": "mvt_test",
					},
				},
				Maps: []provider.Map{
					{
						Name:        "happy",
						Attribution: "Test Attribution",
						Layers: []provider.MapLayer{
							{
								ProviderLayer: "bad.water_default_z",
							},
						},
					},
				},
			},
		},
		"mvt_provider comingle": {
			expectedErr: config.ErrMVTDifferentProviders{
				Original: "provider1",
				Current:  "stdprovider1",
			},
			config: config.Config{
				Providers: []env.Dict{
					{
						"name": "provider1",
						"type": "mvt_test",
					},
					{
						"name": "stdprovider1",
						"type": "test",
					},
				},
				Maps: []provider.Map{
					{
						Name:        "comingle",
						Attribution: "Test Attribution",
						Layers: []provider.MapLayer{
							{
								ProviderLayer: "provider1.water_default_z",
							},
							{
								ProviderLayer: "stdprovider1.water_default_z",
							},
						},
					},
				},
			},
		},
		"mvt_provider comingle; flip": {
			expectedErr: config.ErrMVTDifferentProviders{
				Original: "stdprovider1",
				Current:  "provider1",
			},
			config: config.Config{
				Providers: []env.Dict{
					{
						"name": "stdprovider1",
						"type": "test",
					},
					{
						"name": "provider1",
						"type": "mvt_test",
					},
				},
				Maps: []provider.Map{
					{
						Name:        "comingle",
						Attribution: "Test Attribution",
						Layers: []provider.MapLayer{
							{
								ProviderLayer: "stdprovider1.water_default_z",
							},
							{
								ProviderLayer: "provider1.water_default_z",
							},
						},
					},
				},
			},
		},
		"reserved token name": {
			config: config.Config{
				Maps: []provider.Map{
					{
						Name: "bad_param",
						Parameters: []provider.QueryParameter{
							{
								Name:  "param",
								Token: "!BBOX!",
								Type:  "int",
							},
						},
					},
				},
			},
			expectedErr: config.ErrParamTokenReserved{
				MapName: "bad_param",
				Parameter: provider.QueryParameter{
					Name:  "param",
					Token: "!BBOX!",
					Type:  "int",
				},
			},
		},
		"duplicate parameter name": {
			config: config.Config{
				Maps: []provider.Map{
					{
						Name: "dupe_param_name",
						Parameters: []provider.QueryParameter{
							{
								Name:  "param",
								Token: "!PARAM!",
								Type:  "int",
							},
							{
								Name:  "param",
								Token: "!PARAM2!",
								Type:  "int",
							},
						},
					},
				},
			},
			expectedErr: config.ErrParamDuplicateName{
				MapName: "dupe_param_name",
				Parameter: provider.QueryParameter{
					Name:  "param",
					Token: "!PARAM2!",
					Type:  "int",
				},
			},
		},
		"duplicate token name": {
			config: config.Config{
				Maps: []provider.Map{
					{
						Name: "dupe_param_token",
						Parameters: []provider.QueryParameter{
							{
								Name:  "param",
								Token: "!PARAM!",
								Type:  "int",
							},
							{
								Name:  "param2",
								Token: "!PARAM!",
								Type:  "int",
							},
						},
					},
				},
			},
			expectedErr: config.ErrParamDuplicateToken{
				MapName: "dupe_param_token",
				Parameter: provider.QueryParameter{
					Name:  "param2",
					Token: "!PARAM!",
					Type:  "int",
				},
			},
		},
		"parameter unknown type": {
			config: config.Config{
				Maps: []provider.Map{
					{
						Name: "unknown_param_type",
						Parameters: []provider.QueryParameter{
							{
								Name:  "param",
								Token: "!BBOX!",
								Type:  "foo",
							},
						},
					},
				},
			},
			expectedErr: config.ErrParamUnknownType{
				MapName: "unknown_param_type",
				Parameter: provider.QueryParameter{
					Name:  "param",
					Token: "!BBOX!",
					Type:  "foo",
				},
			},
		},
		"parameter two defaults": {
			config: config.Config{
				Maps: []provider.Map{
					{
						Name: "unknown_two_defaults",
						Parameters: []provider.QueryParameter{
							{
								Name:         "param",
								Token:        "!BBOX!",
								Type:         "string",
								DefaultSQL:   "foo",
								DefaultValue: "bar",
							},
						},
					},
				},
			},
			expectedErr: config.ErrParamTwoDefaults{
				MapName: "unknown_two_defaults",
				Parameter: provider.QueryParameter{
					Name:         "param",
					Token:        "!BBOX!",
					Type:         "string",
					DefaultSQL:   "foo",
					DefaultValue: "bar",
				},
			},
		},
		"parameter invalid default": {
			config: config.Config{
				Maps: []provider.Map{
					{
						Name: "parameter_invalid_default",

						Parameters: []provider.QueryParameter{
							{
								Name:         "param",
								Token:        "!BBOX!",
								Type:         "int",
								DefaultValue: "foo",
							},
						},
					},
				},
			},
			expectedErr: config.ErrParamInvalidDefault{
				MapName: "parameter_invalid_default",
				Parameter: provider.QueryParameter{
					Name:         "param",
					Token:        "!BBOX!",
					Type:         "int",
					DefaultValue: "foo",
				},
			},
		},
		"invalid token name": {
			config: config.Config{
				Maps: []provider.Map{
					{
						Name: "parameter_invalid_token",
						Parameters: []provider.QueryParameter{
							{
								Name:  "param",
								Token: "!Token with spaces!",
								Type:  "int",
							},
						},
					},
				},
			},
			expectedErr: config.ErrParamBadTokenName{
				MapName: "parameter_invalid_token",
				Parameter: provider.QueryParameter{
					Name:  "param",
					Token: "!Token with spaces!",
					Type:  "int",
				},
			},
		},
		"invalid webserver hostname": {
			config: config.Config{
				Webserver: config.Webserver{
					HostName: ":\\malformed.host",
				},
			},
			expectedErr: config.ErrInvalidHostName{},
		},
	}

	for name, tc := range tests {
		t.Run(name, fn(tc))
	}

}

func TestConfigureTileBuffers(t *testing.T) {
	type tcase struct {
		config   config.Config
		expected config.Config
	}

	fn := func(tc tcase) func(*testing.T) {
		return func(t *testing.T) {
			t.Parallel()

			tc.config.ConfigureTileBuffers()
			if !reflect.DeepEqual(tc.expected, tc.config) {
				t.Errorf("expected \n\n %+v \n\n got \n\n %+v", tc.expected, tc.config)
				return
			}
		}
	}

	tests := map[string]tcase{
		"1 tilebuffer is not set": {
			config: config.Config{
				Maps: []provider.Map{
					{
						Name: "osm",
					},
				},
			},
			expected: config.Config{
				Maps: []provider.Map{
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
				Maps: []provider.Map{
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
				Maps: []provider.Map{
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
				Maps: []provider.Map{
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
				Maps: []provider.Map{
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
				Maps: []provider.Map{
					{
						Name:       "osm",
						TileBuffer: env.IntPtr(env.Int(16)),
					},
				},
			},
			expected: config.Config{
				TileBuffer: env.IntPtr(env.Int(32)),
				Maps: []provider.Map{
					{
						Name:       "osm",
						TileBuffer: env.IntPtr(env.Int(16)),
					},
				},
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, fn(tc))
	}
}
