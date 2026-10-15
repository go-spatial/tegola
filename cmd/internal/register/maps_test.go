package register_test

import (
	"errors"
	"testing"

	"github.com/alexeydott/tegola/atlas"
	"github.com/alexeydott/tegola/cmd/internal/register"
	"github.com/alexeydott/tegola/config"
	"github.com/alexeydott/tegola/dict"
	"github.com/alexeydott/tegola/internal/env"
	"github.com/alexeydott/tegola/mapbox/tilejson"
	"github.com/alexeydott/tegola/provider"
)

func TestMaps(t *testing.T) {
	type tcase struct {
		atlas       atlas.Atlas
		maps        []provider.Map
		providers   []dict.Dict
		expectedErr error
	}

	fn := func(tc *tcase) func(*testing.T) {
		return func(t *testing.T) {
			var err error

			// convert []dict.Dict -> []dict.Dicter
			provArr := make([]dict.Dicter, len(tc.providers))
			for i := range provArr {
				provArr[i] = tc.providers[i]
			}

			providers, err := register.Providers(provArr, tc.maps)
			if err != nil {
				t.Errorf("unexpected err: %v", err)
				return
			}

			err = register.Maps(&tc.atlas, tc.maps, providers)
			if !errors.Is(err, tc.expectedErr) {
				t.Errorf("invalid error, expected %v got %v", tc.expectedErr, err)
			}
		}
	}

	tests := map[string]*tcase{
		"provider layer invalid": {
			maps: []provider.Map{
				{
					Name: "foo",
					Layers: []provider.MapLayer{
						{
							ProviderLayer: "bar",
						},
					},
				},
			},
			providers: []dict.Dict{
				{
					"name": "test",
					"type": "debug",
				},
			},
			expectedErr: register.ErrProviderLayerInvalid{
				ProviderLayer: "bar",
				Map:           "foo",
			},
		},
		"provider not found": {
			maps: []provider.Map{
				{
					Name: "foo",
					Layers: []provider.MapLayer{
						{
							ProviderLayer: "bar.baz",
						},
					},
				},
			},
			expectedErr: register.ErrProviderNotFound{
				Provider: "bar",
			},
		},
		"provider layer not registered with provider": {
			maps: []provider.Map{
				{
					Name: "foo",
					Layers: []provider.MapLayer{
						{
							ProviderLayer: "test.bar",
						},
					},
				},
			},
			providers: []dict.Dict{
				{
					"name": "test",
					"type": "debug",
				},
			},
			expectedErr: register.ErrProviderLayerNotRegistered{
				MapName:       "foo",
				ProviderLayer: "test.bar",
				Provider:      "test",
			},
		},
		"default tags": {
			maps: []provider.Map{
				{
					Name: "foo",
					Layers: []provider.MapLayer{
						{
							ProviderLayer: "test.debug-tile-outline",
							DefaultTags: env.Dict{
								"test": "test",
							},
						},
					},
				},
			},
			providers: []dict.Dict{
				{
					"name": "test",
					"type": "debug",
				},
			},
		},
		"fails when map requires TileJSON v3 but provider does not implement": {
			maps: []provider.Map{
				{
					Name:            "test",
					TileJSONVersion: tilejson.Version3,
					Layers: []provider.MapLayer{
						{
							ProviderLayer: "foo.bar",
						},
					},
				},
			},
			providers: []dict.Dict{
				{
					"name": "foo",
					"type": "debug",
				},
			},
			expectedErr: provider.ErrNotTileJSONV3Compatible,
		},
		"duplicate map names rejected": {
			maps: []provider.Map{
				{
					Name: "foo",
					Layers: []provider.MapLayer{
						{
							ProviderLayer: "test.debug-tile-outline",
						},
					},
				},
				{
					Name: "foo",
					Layers: []provider.MapLayer{
						{
							ProviderLayer: "test.debug-tile-outline",
						},
					},
				},
			},
			providers: []dict.Dict{
				{
					"name": "test",
					"type": "debug",
				},
			},
			expectedErr: config.ErrMapNameDuplicate{MapName: "foo"},
		},
		"layer with inverted zooms rejected": {
			maps: []provider.Map{
				{
					Name: "foo",
					Layers: []provider.MapLayer{
						{
							ProviderLayer: "test.debug-tile-outline",
							MinZoom:       env.UintPtr(10),
							MaxZoom:       env.UintPtr(5),
						},
					},
				},
			},
			providers: []dict.Dict{
				{
					"name": "test",
					"type": "debug",
				},
			},
			expectedErr: config.ErrInvalidLayerZoomRange{
				MapName:       "foo",
				ProviderLayer: "test.debug-tile-outline",
				MinZoom:       10,
				MaxZoom:       5,
			},
		},
		"success": {
			maps: []provider.Map{},
			providers: []dict.Dict{
				{
					"name": "test",
					"type": "debug",
				},
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, fn(tc))
	}
}

func TestSanitizeAttribution(t *testing.T) {
	type tcase struct {
		input    string
		expected string
	}

	fn := func(tc tcase) func(*testing.T) {
		return func(t *testing.T) {
			result := register.SanitizeAttribution(tc.input)
			if result != tc.expected {
				t.Errorf("expected %v got %v", tc.expected, result)
			}
		}
	}

	tests := map[string]tcase{
		"plain text": {
			input:    `foo`,
			expected: `foo`,
		},
		"HTML must escaped": {
			input:    `<script>true</script>`,
			expected: `&lt;script&gt;true&lt;/script&gt;`,
		},
		"link must not escaped": {
			input:    `<a href="http://example.com">foo</a>`,
			expected: `<a href="http://example.com">foo</a>`,
		},
		"2 links": {
			input:    `foo <a href="http://example.com">bar</a> - <a href="http://example.com" target="_blank">zoo</a>`,
			expected: `foo <a href="http://example.com">bar</a> - <a href="http://example.com" target="_blank">zoo</a>`,
		},
	}

	for name, tc := range tests {
		t.Run(name, fn(tc))
	}
}
