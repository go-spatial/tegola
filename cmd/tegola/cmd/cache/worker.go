package cache

import (
	"context"
	"fmt"
	"runtime"
	"time"

	"github.com/go-spatial/geom/slippy"
	"github.com/go-spatial/tegola"
	"github.com/go-spatial/tegola/atlas"
	"github.com/go-spatial/tegola/cache"
	"github.com/go-spatial/tegola/internal/log"
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

func seedWorker(overwrite bool) func(ctx context.Context, mt MapTile) error {
	return func(ctx context.Context, mt MapTile) error {
		// track how long the tile generation is taking
		t := time.Now()

		//	lookup the Map
		m, err := atlas.GetMap(mt.MapName)
		if err != nil {
			return seedPurgeWorkerTileError{
				Tile: *mt.Tile,
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
				Z:       z,
				X:       x,
				Y:       y,
			}

			//	read the tile from the cache
			_, hit, err := c.Get(&key)
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
		if err = atlas.SeedMapTile(ctx, m, z, x, y); err != nil {
			if err == context.Canceled {
				return err
			}
			return seedPurgeWorkerTileError{
				Tile: *mt.Tile,
				Err:  err,
			}
		}

		//	TODO: this is a hack to get around large arrays not being garbage collected
		//	https://github.com/golang/go/issues/14045 - should be addressed in Go 1.11
		runtime.GC()

		log.Infof("seeding map (%v) tile (%v/%v/%v) took: %dms", mt.MapName, z, x, y, time.Now().Sub(t).Nanoseconds()/1000000)

		return nil
	}

}

func purgeWorker(_ context.Context, mt MapTile) error {

	z, x, y := mt.Tile.ZXY()

	log.Infof("purging map (%v) tile (%v/%v/%v)", mt.MapName, z, x, y)

	//	lookup the Map
	m, err := atlas.GetMap(mt.MapName)
	if err != nil {
		return seedPurgeWorkerTileError{
			Purge: true,
			Tile:  *mt.Tile,
			Err:   err,
		}
	}

	//	purge the tile
	ttile := tegola.NewTile(mt.Tile.ZXY())

	if err = atlas.PurgeMapTile(m, ttile); err != nil {
		return seedPurgeWorkerTileError{
			Purge: true,
			Tile:  *mt.Tile,
			Err:   err,
		}
	}

	return nil
}
