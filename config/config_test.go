package config_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/terranodo/tegola/config"
)

func TestParse(t *testing.T) {
	testcases := []struct {
		config   string
		expected config.Config
	}{
		{
			config: `
				[webserver]
				hostname = "cdn.tegola.io"
				port = ":8080"
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
				    max_zoom = 20`,
			expected: config.Config{
				LocationName: "",
				Webserver: config.Webserver{
					ServerName: "cdn.tegola.io",
					Port:       ":8080",
					LogFile:    "/var/log/tegola/tegola.log",
					LogFormat:  "{{.Time}}:{{.RequestIP}} —— Tile:{{.Z}}/{{.X}}/{{.Y}}",
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
							},
						},
					},
				},
			},
		},
	}

	for i, tc := range testcases {
		r := strings.NewReader(tc.config)

		conf, err := config.Parse(r, "")
		if err != nil {
			t.Errorf("test case (%v) failed err: %v", i, err)
			return
		}

		//	compare the various parts fo the config
		if !reflect.DeepEqual(conf.LocationName, tc.expected.LocationName) {
			t.Errorf("test case (%v) failed. LocationName output \n\n (%+v) \n\n does not match expected \n\n (%+v) ", i, conf.LocationName, tc.expected.LocationName)
			return
		}

		if !reflect.DeepEqual(conf.Webserver, tc.expected.Webserver) {
			t.Errorf("test case (%v) failed. Webserver output \n\n (%+v) \n\n does not match expected \n\n (%+v) ", i, conf.Webserver, tc.expected.Webserver)
			return
		}

		if !reflect.DeepEqual(conf.Providers, tc.expected.Providers) {
			t.Errorf("test case (%v) failed. Providers output \n\n (%+v) \n\n does not match expected \n\n (%+v) ", i, conf.Providers, tc.expected.Providers)
			return
		}

		if !reflect.DeepEqual(conf.Maps, tc.expected.Maps) {
			t.Errorf("test case (%v) failed. Maps output \n\n (%+v) \n\n does not match expected \n\n (%+v) ", i, conf.Maps, tc.expected.Maps)
			return
		}
	}
}

func TestValidate(t *testing.T) {
	testcases := []struct {
		config   config.Config
		expected error
	}{
		{
			config: config.Config{
				LocationName: "",
				Webserver: config.Webserver{
					Port:      ":8080",
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
			expected: config.ErrLayerCollision{
				ProviderLayer1: "provider1.water",
				ProviderLayer2: "provider2.water",
			},
		},
		{
			config: config.Config{
				LocationName: "",
				Webserver: config.Webserver{
					Port:      ":8080",
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
				},
			},
			expected: nil,
		},
	}

	for i, tc := range testcases {
		err := tc.config.Validate()
		if err != tc.expected {
			t.Errorf("test case (%v) failed. \n\n expected \n\n (%v) \n\n got \n\n (%v)", i, tc.expected, err)
			return
		}
	}
}
