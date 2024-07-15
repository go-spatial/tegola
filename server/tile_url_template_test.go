package server_test

import (
	"encoding/json"
	"errors"
	"net/url"
	"reflect"
	"testing"

	"github.com/go-spatial/tegola/server"
)

func TestTileURLTemplateString(t *testing.T) {
	type tcase struct {
		input    server.TileURLTemplate
		expected string
	}

	fn := func(tc tcase) func(t *testing.T) {
		return func(t *testing.T) {
			output := tc.input.String()
			if output != tc.expected {
				t.Errorf("expected (%s) got (%s)", tc.expected, output)
			}
		}
	}

	tests := map[string]tcase{
		"map": {
			input: server.TileURLTemplate{
				Scheme:  "https",
				Host:    "go-spatial.org",
				MapName: "osm",
			},
			expected: "https://go-spatial.org/maps/osm/{z}/{x}/{y}.pbf",
		},
		"map with query params": {
			input: server.TileURLTemplate{
				Scheme:  "https",
				Host:    "go-spatial.org",
				MapName: "osm",
				Query: url.Values{
					server.QueryKeyDebug: []string{
						"true",
					},
				},
			},
			expected: "https://go-spatial.org/maps/osm/{z}/{x}/{y}.pbf?debug=true",
		},
		"map with path prefix": {
			input: server.TileURLTemplate{
				Scheme:     "https",
				Host:       "go-spatial.org",
				PathPrefix: "v1",
				MapName:    "osm",
			},
			expected: "https://go-spatial.org/v1/maps/osm/{z}/{x}/{y}.pbf",
		},
		"map layer": {
			input: server.TileURLTemplate{
				Scheme:    "https",
				Host:      "go-spatial.org",
				MapName:   "osm",
				LayerName: "water",
			},
			expected: "https://go-spatial.org/maps/osm/water/{z}/{x}/{y}.pbf",
		},
		"map layer with query params": {
			input: server.TileURLTemplate{
				Scheme:    "https",
				Host:      "go-spatial.org",
				MapName:   "osm",
				LayerName: "water",
				Query: url.Values{
					server.QueryKeyDebug: []string{
						"true",
					},
					"foo": []string{
						"bar",
					},
				},
			},
			expected: "https://go-spatial.org/maps/osm/water/{z}/{x}/{y}.pbf?debug=true&foo=bar",
		},
		"map layer with path prefix": {
			input: server.TileURLTemplate{
				Scheme:     "https",
				Host:       "go-spatial.org",
				PathPrefix: "v1",
				MapName:    "osm",
				LayerName:  "water",
			},
			expected: "https://go-spatial.org/v1/maps/osm/water/{z}/{x}/{y}.pbf",
		},
	}

	for name, tc := range tests {
		t.Run(name, fn(tc))
	}
}

func TestTileURLTemplateUnmarshalJSON(t *testing.T) {
	type tcase struct {
		input       []byte
		expected    server.TileURLTemplate
		expectedErr error
	}

	fn := func(tc tcase) func(t *testing.T) {
		return func(t *testing.T) {
			var output server.TileURLTemplate
			err := json.Unmarshal(tc.input, &output)
			if err != nil {
				if tc.expectedErr != nil {
					if !errors.Is(err, tc.expectedErr) {
						t.Fatalf("unepxected err: %s", err)
					}
					return
				}
				t.Fatalf("unepxected err: %s", err)
			}
			if tc.expectedErr != nil {
				t.Fatalf("expected err of type: %T, but got %T", tc.expectedErr, err)
			}

			if !reflect.DeepEqual(output, tc.expected) {
				t.Fatalf("expected (%+v) got (%+v)", tc.expected, output)
			}
		}
	}

	tests := map[string]tcase{
		"map": {
			input: []byte(`"https://go-spatial.org/maps/osm/{z}/{x}/{y}.pbf"`),
			expected: server.TileURLTemplate{
				Scheme:  "https",
				Host:    "go-spatial.org",
				MapName: "osm",
			},
		},
		"map with query params": {
			input: []byte(`"https://go-spatial.org/maps/osm/{z}/{x}/{y}.pbf?debug=true"`),
			expected: server.TileURLTemplate{
				Scheme:  "https",
				Host:    "go-spatial.org",
				MapName: "osm",
				Query: url.Values{
					server.QueryKeyDebug: []string{
						"true",
					},
				},
			},
		},
		"map with path prefix": {
			input: []byte(`"https://go-spatial.org/v1/maps/osm/{z}/{x}/{y}.pbf"`),
			expected: server.TileURLTemplate{
				Scheme:     "https",
				Host:       "go-spatial.org",
				PathPrefix: "v1",
				MapName:    "osm",
			},
		},
		"map layer": {
			input: []byte(`"https://go-spatial.org/maps/osm/water/{z}/{x}/{y}.pbf"`),
			expected: server.TileURLTemplate{
				Scheme:    "https",
				Host:      "go-spatial.org",
				MapName:   "osm",
				LayerName: "water",
			},
		},
		"map layer with query params": {
			input: []byte(`"https://go-spatial.org/maps/osm/water/{z}/{x}/{y}.pbf?debug=true&foo=bar"`),
			expected: server.TileURLTemplate{
				Scheme:    "https",
				Host:      "go-spatial.org",
				MapName:   "osm",
				LayerName: "water",
				Query: url.Values{
					server.QueryKeyDebug: []string{
						"true",
					},
					"foo": []string{
						"bar",
					},
				},
			},
		},
		"map layer with uri prefix": {
			input: []byte(`"https://go-spatial.org/v1/maps/osm/water/{z}/{x}/{y}.pbf"`),
			expected: server.TileURLTemplate{
				Scheme:     "https",
				Host:       "go-spatial.org",
				PathPrefix: "v1",
				MapName:    "osm",
				LayerName:  "water",
			},
		},
		"malformed url 1": {
			input: []byte(`"https://go-spatial.org/v1/maps/"`),
			expectedErr: server.ErrMalformedTileTemplateURL{
				Got: "https://go-spatial.org/v1/maps/",
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, fn(tc))
	}
}
