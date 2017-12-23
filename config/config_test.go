package config_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/go-spatial/tegola/config"
)

func TestParse(t *testing.T) {
	type tcase struct {
		config   string
		expected config.Config
	}

	fn := func(t *testing.T, tc tcase) {
		t.Parallel()

		r := strings.NewReader(tc.config)

		conf, err := config.Parse(r, "")
		if err != nil {
			t.Error(err)
			return
		}

		//	compare the various parts fo the config
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
				port = 8080
				bind = ":8080"
				cors_allowed_origin = "tegola.io"
				log_file = "/var/log/tegola/tegola.log"
				log_format = "{{.Time}}:{{.RequestIP}} —— Tile:{{.Z}}/{{.X}}/{{.Y}}"

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
					dont_simplify = true`,
			expected: config.Config{
				TileBuffer:   12,
				LocationName: "",
				Webserver: config.Webserver{
					HostName:          "cdn.tegola.io",
					Port:              8080,
					Bind:              ":8080",
					CORSAllowedOrigin: "tegola.io",
					LogFile:           "/var/log/tegola/tegola.log",
					LogFormat:         "{{.Time}}:{{.RequestIP}} —— Tile:{{.Z}}/{{.X}}/{{.Y}}",
				},
				Cache: map[string]interface{}{
					"type":     "file",
					"basepath": "/tmp/tegola-cache",
				},
				Providers: []map[string]interface{}{
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
						Bounds:      []float64{-180, -85.05112877980659, 180, 85.0511287798066},
						Center:      [3]float64{-76.275329586789, 39.153492567373, 8.0},
						Layers: []config.MapLayer{
							{
								ProviderLayer: "provider1.water",
								MinZoom:       10,
								MaxZoom:       20,
								DontSimplify:  true,
							},
						},
					},
				},
			},
		},
		"2": {
			config: `
				[webserver]
				hostname = "cdn.tegola.io"
				port = 8080
				bind = ":8080"
				log_file = "/var/log/tegola/tegola.log"
				log_format = "{{.Time}}:{{.RequestIP}} —— Tile:{{.Z}}/{{.X}}/{{.Y}}"

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
					HostName:  "cdn.tegola.io",
					Port:      8080,
					Bind:      ":8080",
					LogFile:   "/var/log/tegola/tegola.log",
					LogFormat: "{{.Time}}:{{.RequestIP}} —— Tile:{{.Z}}/{{.X}}/{{.Y}}",
				},
				Providers: []map[string]interface{}{
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
						Bounds:      []float64{-180, -85.05112877980659, 180, 85.0511287798066},
						Center:      [3]float64{-76.275329586789, 39.153492567373, 8.0},
						Layers: []config.MapLayer{
							{
								Name:          "water",
								ProviderLayer: "provider1.water_0_5",
								MinZoom:       0,
								MaxZoom:       5,
							},
							{
								Name:          "water",
								ProviderLayer: "provider1.water_6_10",
								MinZoom:       6,
								MaxZoom:       10,
							},
						},
					},
					{
						Name:        "osm_2",
						Attribution: "Test Attribution",
						Bounds:      []float64{-180, -85.05112877980659, 180, 85.0511287798066},
						Center:      [3]float64{-76.275329586789, 39.153492567373, 8.0},
						Layers: []config.MapLayer{
							{
								Name:          "water",
								ProviderLayer: "provider1.water_0_5",
								MinZoom:       0,
								MaxZoom:       5,
							},
							{
								Name:          "water",
								ProviderLayer: "provider1.water_6_10",
								MinZoom:       6,
								MaxZoom:       10,
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
					Port:      8080,
					Bind:      ":8080",
					LogFile:   "/var/log/tegola/tegola.log",
					LogFormat: "{{.Time}}:{{.RequestIP}} —— Tile:{{.Z}}/{{.X}}/{{.Y}}",
				},
				Providers: []map[string]interface{}{
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
						Bounds:      []float64{-180, -85.05112877980659, 180, 85.0511287798066},
						Center:      [3]float64{-76.275329586789, 39.153492567373, 8.0},
						Layers: []config.MapLayer{
							{
								ProviderLayer: "provider1.water",
								MinZoom:       10,
								MaxZoom:       20,
							},
							{
								ProviderLayer: "provider2.water",
								MinZoom:       10,
								MaxZoom:       20,
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
				Providers: []map[string]interface{}{
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
						Bounds:      []float64{-180, -85.05112877980659, 180, 85.0511287798066},
						Center:      [3]float64{-76.275329586789, 39.153492567373, 8.0},
						Layers: []config.MapLayer{
							{
								Name:          "water",
								ProviderLayer: "provider1.water_0_5",
								MinZoom:       0,
								MaxZoom:       5,
							},
							{
								Name:          "water",
								ProviderLayer: "provider2.water_5_10",
								MinZoom:       5,
								MaxZoom:       10,
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
					Port:      8080,
					Bind:      ":8080",
					LogFile:   "/var/log/tegola/tegola.log",
					LogFormat: "{{.Time}}:{{.RequestIP}} —— Tile:{{.Z}}/{{.X}}/{{.Y}}",
				},
				Providers: []map[string]interface{}{
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
						Bounds:      []float64{-180, -85.05112877980659, 180, 85.0511287798066},
						Center:      [3]float64{-76.275329586789, 39.153492567373, 8.0},
						Layers: []config.MapLayer{
							{
								ProviderLayer: "provider1.water",
								MinZoom:       10,
								MaxZoom:       15,
							},
							{
								ProviderLayer: "provider2.water",
								MinZoom:       16,
								MaxZoom:       20,
							},
						},
					},
					{
						Name:        "osm_2",
						Attribution: "Test Attribution",
						Bounds:      []float64{-180, -85.05112877980659, 180, 85.0511287798066},
						Center:      [3]float64{-76.275329586789, 39.153492567373, 8.0},
						Layers: []config.MapLayer{
							{
								ProviderLayer: "provider1.water",
								MinZoom:       10,
								MaxZoom:       15,
							},
							{
								ProviderLayer: "provider2.water",
								MinZoom:       16,
								MaxZoom:       20,
							},
						},
					},
				},
			},
			expectedErr: nil,
		},
	}

	for name, tc := range tests {
		tc := tc
		t.Run(name, func(t *testing.T) {
			fn(t, tc)
		})
	}
}
