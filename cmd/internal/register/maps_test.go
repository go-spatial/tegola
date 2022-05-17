package register_test

import (
	"errors"
	"testing"

	"github.com/go-spatial/tegola/atlas"
	"github.com/go-spatial/tegola/cmd/internal/register"
	"github.com/go-spatial/tegola/config"
	"github.com/go-spatial/tegola/dict"
	"github.com/go-spatial/tegola/internal/env"
)

func TestMaps(t *testing.T) {
	type tcase struct {
		atlas       atlas.Atlas
		maps        []config.Map
		providers   []dict.Dict
		expectedErr error
	}

	fn := func(tc tcase) func(*testing.T) {
		return func(t *testing.T) {
			var err error

			// convert []dict.Dict -> []dict.Dicter
			provArr := make([]dict.Dicter, len(tc.providers))
			for i := range provArr {
				provArr[i] = tc.providers[i]
			}

			providers, err := register.Providers(provArr)
			if err != nil {
				t.Errorf("unexpected err: %v", err)
				return
			}

			err = register.Maps(&tc.atlas, tc.maps, providers)
			if !errors.Is(err, tc.expectedErr) {
				t.Errorf("invalid error, expected %v got %v", tc.expectedErr, err)
			}
			return
		}
	}

	tests := map[string]tcase{
		"provider layer invalid": {
			maps: []config.Map{
				{
					Name: "foo",
					Layers: []config.MapLayer{
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
			maps: []config.Map{
				{
					Name: "foo",
					Layers: []config.MapLayer{
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
			maps: []config.Map{
				{
					Name: "foo",
					Layers: []config.MapLayer{
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
			maps: []config.Map{
				{
					Name: "foo",
					Layers: []config.MapLayer{
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
		"success": {
			maps: []config.Map{},
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
