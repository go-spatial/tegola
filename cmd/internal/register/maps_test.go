package register_test

import (
	"testing"

	"github.com/go-spatial/tegola/atlas"
	"github.com/go-spatial/tegola/cmd/internal/register"
	"github.com/go-spatial/tegola/config"
	"github.com/go-spatial/tegola/dict"
)

func TestMaps(t *testing.T) {
	type tcase struct {
		atlas       atlas.Atlas
		maps        []config.Map
		providers   []dict.Dict
		expectedErr error
	}

	fn := func(t *testing.T, tc tcase) {
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
		if tc.expectedErr != nil {
			if err.Error() != tc.expectedErr.Error() {
				t.Errorf("invalid error. expected: %v, got: %v", tc.expectedErr, err.Error())
			}
			return
		}
		if err != nil {
			t.Errorf("unexpected err: %v", err)
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
		"default tags invalid": {
			maps: []config.Map{
				{
					Name: "foo",
					Layers: []config.MapLayer{
						{
							ProviderLayer: "test.debug-tile-outline",
							DefaultTags:   false, // should be a map[string]interface{}
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
			expectedErr: register.ErrDefaultTagsInvalid{
				ProviderLayer: "test.debug-tile-outline",
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
		tc := tc
		t.Run(name, func(t *testing.T) { fn(t, tc) })
	}
}
