package config_test

import (
	"errors"
	"testing"

	"github.com/alexeydott/tegola/config"
	"github.com/alexeydott/tegola/internal/env"
	"github.com/alexeydott/tegola/provider"
	_ "github.com/alexeydott/tegola/provider/debug"
)

// TestValidateMapNamesAndZooms covers the part13 P6-29 validation:
// duplicate map names must be rejected and layers must satisfy
// min_zoom <= max_zoom.
func TestValidateMapNamesAndZooms(t *testing.T) {
	type tcase struct {
		config      config.Config
		expectedErr error
	}

	fn := func(tc tcase) func(*testing.T) {
		return func(t *testing.T) {
			t.Parallel()

			err := tc.config.Validate()
			if !errors.Is(err, tc.expectedErr) {
				t.Errorf("expected err: %v got %v", tc.expectedErr, err)
			}
		}
	}

	tests := map[string]tcase{
		"duplicate map names rejected": {
			config: config.Config{
				Providers: []env.Dict{
					{"name": "provider1", "type": "debug"},
				},
				Maps: []provider.Map{
					{
						Name: "osm",
						Layers: []provider.MapLayer{
							{ProviderLayer: "provider1.water", MinZoom: env.UintPtr(0), MaxZoom: env.UintPtr(10)},
						},
					},
					{
						Name: "osm",
						Layers: []provider.MapLayer{
							{ProviderLayer: "provider1.water", MinZoom: env.UintPtr(0), MaxZoom: env.UintPtr(10)},
						},
					},
				},
			},
			expectedErr: config.ErrMapNameDuplicate{MapName: "osm"},
		},
		"unique map names accepted": {
			config: config.Config{
				Providers: []env.Dict{
					{"name": "provider1", "type": "debug"},
				},
				Maps: []provider.Map{
					{
						Name: "osm",
						Layers: []provider.MapLayer{
							{ProviderLayer: "provider1.water", MinZoom: env.UintPtr(0), MaxZoom: env.UintPtr(10)},
						},
					},
					{
						Name: "osm2",
						Layers: []provider.MapLayer{
							{ProviderLayer: "provider1.water", MinZoom: env.UintPtr(11), MaxZoom: env.UintPtr(20)},
						},
					},
				},
			},
			expectedErr: nil,
		},
		"layer min_zoom greater than max_zoom rejected": {
			config: config.Config{
				Providers: []env.Dict{
					{"name": "provider1", "type": "debug"},
				},
				Maps: []provider.Map{
					{
						Name: "osm",
						Layers: []provider.MapLayer{
							{ProviderLayer: "provider1.water", MinZoom: env.UintPtr(10), MaxZoom: env.UintPtr(5)},
						},
					},
				},
			},
			expectedErr: config.ErrInvalidLayerZoomRange{
				MapName:       "osm",
				ProviderLayer: "provider1.water",
				MinZoom:       10,
				MaxZoom:       5,
			},
		},
		"layer with equal zooms accepted": {
			config: config.Config{
				Providers: []env.Dict{
					{"name": "provider1", "type": "debug"},
				},
				Maps: []provider.Map{
					{
						Name: "osm",
						Layers: []provider.MapLayer{
							{ProviderLayer: "provider1.water", MinZoom: env.UintPtr(5), MaxZoom: env.UintPtr(5)},
						},
					},
				},
			},
			expectedErr: nil,
		},
	}

	for name, tc := range tests {
		t.Run(name, fn(tc))
	}
}
