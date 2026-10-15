package cache

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/alexeydott/geom/slippy"
	"github.com/alexeydott/tegola"
	"github.com/alexeydott/tegola/atlas"
	"github.com/alexeydott/tegola/cache"
	"github.com/alexeydott/tegola/internal/log"
)

type seedPurgeWorkerTileError struct {
	Purge bool
	Tile  slippy.Tile
	Err   error
}

func (s seedPurgeWorkerTileError) Error() string {
	cmd := "seeding"
	if s.Purge {
		cmd = "purging"
	}
	return fmt.Sprintf("error %v tile (%+v): %v", cmd, s.Tile, s.Err)
}

func seedWorker(overwrite bool, logThresholdMs int64) func(ctx context.Context, mt MapTile) error {
	return func(ctx context.Context, mt MapTile) error {
		// track how long the tile generation is taking
		t := time.Now()

		//	lookup the Map
		m, err := atlas.GetMap(mt.MapName)
		if err != nil {
			return seedPurgeWorkerTileError{
				Tile: mt.Tile,
				Err:  err,
			}
		}

		z, x, y := mt.Tile.ZXY()

		//	filter down the layers we need for this zoom
		m = m.FilterLayersByZoom(z)

		//	check if overwriting the cache is not ok
		if !overwrite {
			//	lookup our cache
			c := atlas.GetCache()
			if c == nil {
				return fmt.Errorf("error fetching cache: %v", err)
			}

			//	cache key
			key := cache.Key{
				MapName: mt.MapName,
				Z:       uint(z),
				X:       x,
				Y:       y,
			}

			//	read the tile from the cache
			_, hit, err := c.Get(ctx, &key)
			if err != nil {
				return fmt.Errorf("error reading from cache: %v", err)
			}
			//	if we have a cache hit, then skip processing this tile
			if hit {
				log.Infof("cache seed set to not overwrite existing tiles. skipping map (%v) tile (%v/%v/%v)", mt.MapName, z, x, y)
				return nil
			}
		}

		//	seed the tile
		if err = atlas.SeedMapTile(ctx, m, uint(z), x, y); err != nil {
			if errors.Is(err, context.Canceled) {
				return err
			}
			return seedPurgeWorkerTileError{
				Tile: mt.Tile,
				Err:  err,
			}
		}

		// The previous per-tile runtime.GC() workaround for golang/go#14045 (large
		// arrays not collected) is gone: that issue was fixed in Go 1.11, so the
		// forced GC only added needless overhead. The large encode buffers live in
		// atlas (Map.Encode), outside this package, so buffer pooling is not
		// applicable here. Seeding behavior is unchanged.
		durationMs := time.Since(t).Nanoseconds() / 1000000
		if durationMs >= logThresholdMs {
			log.Infof("seeding map (%v) tile (%v/%v/%v) took: %dms", mt.MapName, z, x, y, durationMs)
		}

		return nil
	}

}

func purgeWorker(ctx context.Context, mt MapTile) error {

	z, x, y := mt.Tile.ZXY()

	log.Infof("purging map (%v) tile (%v/%v/%v)", mt.MapName, z, x, y)

	//	lookup the Map
	m, err := atlas.GetMap(mt.MapName)
	if err != nil {
		return seedPurgeWorkerTileError{
			Purge: true,
			Tile:  mt.Tile,
			Err:   err,
		}
	}

	//	purge the tile
	ttile := tegola.TileFromSlippyTile(mt.Tile)

	if err = atlas.PurgeMapTile(ctx, m, ttile); err != nil {
		return seedPurgeWorkerTileError{
			Purge: true,
			Tile:  mt.Tile,
			Err:   err,
		}
	}

	return nil
}
