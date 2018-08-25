package cmd_test

import (
	"context"
	"reflect"
	"testing"

	"github.com/go-spatial/geom/slippy"
	"github.com/go-spatial/tegola/atlas"
	"github.com/go-spatial/tegola/cache"
	"github.com/go-spatial/tegola/cmd/internal/register"
	"github.com/go-spatial/tegola/config"
	"github.com/go-spatial/tegola/dict"
	"github.com/go-spatial/tegola/internal/env"

	// what we're testing
	"github.com/go-spatial/tegola/cmd/tegola/cmd"

	// registering test cache and providers
	_ "github.com/go-spatial/tegola/cache/memory"
	_ "github.com/go-spatial/tegola/provider/test"
)

var (
	testCacheConf = env.Dict{
		"type": "memory",
	}
	testProvidersConf = []dict.Dicter{
		env.Dict{
			"type": "test",
			"name": "test-provider",
		},
	}
	testMapsConf = []config.Map{
		{
			Name: "test-map",
			Layers: []config.MapLayer{
				{
					Name:          "test-map-layer",
					ProviderLayer: "test-provider.test-layer",
				},
			},
		},
	}
)

func TestCacheSeed(t *testing.T) {
	type tcase struct {
		tiles []cmd.MapTile
	}

	fn := func(tc tcase, t *testing.T) {
		// init cache
		cacher, err := register.Cache(testCacheConf)
		if err != nil {
			t.Errorf("unexpected error, %v", err)
			return
		}
		atlas.SetCache(cacher)

		// init providers
		pvdrs, err := register.Providers(testProvidersConf)
		if err != nil {
			t.Errorf("unexpected error, %v", err)
			return
		}

		// init maps
		err = register.Maps(nil, testMapsConf, pvdrs)
		if err != nil {
			t.Errorf("unexpected error, %v", err)
			return
		}

		// seed tiles
		for _, v := range tc.tiles {
			err = cmd.SeedWorker(context.Background(), v)
			if err != nil {
				t.Errorf("unexpected error, %v", err)
				return
			}
		}

		cacher = atlas.GetCache()

		// test if tiles are the same between cache and provider
		for _, v := range tc.tiles {
			// get tile from atlas
			m, err := atlas.GetMap(v.MapName)
			if err != nil {
				t.Errorf("unexpected error, %v", err)
				return
			}

			z, x, y := v.Tile.ZXY()

			m = m.FilterLayersByZoom(z)

			atlasVal, err := m.Encode(context.Background(), v.Tile)
			if err != nil {
				t.Errorf("unexpected error, %v", err)
				return
			}

			// get tile from cache
			ckey := &cache.Key{
				MapName: v.MapName,
				Z:       z,
				X:       x,
				Y:       y,
			}

			cacheVal, hit, err := cacher.Get(ckey)
			if err != nil {
				t.Errorf("unexpected error, %v", err)
				return
			}
			if !hit {
				if err != nil {
					t.Errorf("unexpected cache miss, %v", ckey)
					return
				}
			}

			// make sure tile from cache is the same as the tile from the provider
			if !reflect.DeepEqual(atlasVal, cacheVal) {
				t.Errorf("cache fail tile %v does not match provider", ckey)
			}

		}

	}

	testcases := map[string]tcase{
		"single tile no buff": {
			tiles: []cmd.MapTile{
				{
					MapName: "test-map",
					Tile:    slippy.NewTile(0, 0, 0),
				},
			},
		},
		"single tile def buff": {
			tiles: []cmd.MapTile{
				{
					MapName: "test-map",
					Tile:    slippy.NewTile(0, 0, 0),
				},
			},
		},
	}

	for k, v := range testcases {
		t.Run(k, func(t *testing.T) {
			fn(v, t)
		})
	}
}
