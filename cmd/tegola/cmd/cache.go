package cmd

import (
	"context"
	"fmt"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/spf13/cobra"

	gdcmd "github.com/gdey/cmd"

	"github.com/go-spatial/tegola"
	"github.com/go-spatial/tegola/atlas"
	"github.com/go-spatial/tegola/cache"
	"github.com/go-spatial/tegola/internal/log"
	"github.com/go-spatial/tegola/maths"
	"github.com/go-spatial/tegola/provider"
)

var (
	//	specify a tile to cache. ignored by default
	cacheZXY string
	//	filter which maps to process. default will operate on all mapps
	cacheMap string
	//	the min zoom to cache from
	cacheMinZoom uint
	//	the max zoom to cache to
	cacheMaxZoom uint
	//	bounds to cache within. default -180, -85.0511, 180, 85.0511
	cacheBounds string
	//	the amount of concurrency to use. defaults to the number of CPUs on the machine
	cacheConcurrency int
	//	cache overwrite
	cacheOverwrite bool
)

var cacheCmd = &cobra.Command{
	Use:       "cache [seed | purge]",
	Short:     "Manipulate the tile cache",
	Long:      `Use the cache command to seed or purge the tile cache`,
	ValidArgs: []string{"seed", "purge"},
	Args: func(cmd *cobra.Command, args []string) error {
		if len(args) != 1 {
			return fmt.Errorf("requires at least one argument: seed, purge")
		}

		for i, v := range cmd.ValidArgs {
			if args[0] == v {
				break
			}
			if i == len(cmd.ValidArgs)+1 {
				return fmt.Errorf("invliad arg (%v). supported: seed, purge", args[0])
			}
		}

		return nil
	},
	Run: func(cmd *cobra.Command, args []string) {
		defer gdcmd.New().Complete()
		gdcmd.OnComplete(provider.Cleanup)
		var err error
		var maps []atlas.Map

		initConfig()

		//	check if the user defined a single map to work on
		if cacheMap != "" {
			m, err := atlas.GetMap(cacheMap)
			if err != nil {
				log.Fatal(err)
			}

			maps = append(maps, m)
		} else {
			maps = atlas.AllMaps()
		}

		//	check for a cache backend
		if atlas.GetCache() == nil {
			log.Fatalf("mising cache backend. check your config (%v)", configFile)
		}

		var zooms []uint
		var minx, miny, maxx, maxy int
		var bounds [4]float64

		//	single tile caching
		if cacheZXY != "" {
			//	convert the input into a tile
			t, err := parseTileString(cacheZXY)
			if err != nil {
				log.Fatal(err)
			}

			zooms = append(zooms, t.Z)
			// read the tile bounds, which will be in lat, lon, it will be the north, east, south, west.
			bounds = t.Bounds()
		} else {
			//	bounding box caching
			boundsParts := strings.Split(cacheBounds, ",")
			if len(boundsParts) != 4 {
				log.Fatal("invalid value for bounds. expecting minx, miny, maxx, maxy")
			}

			for i := range boundsParts {
				bounds[i], err = strconv.ParseFloat(boundsParts[i], 64)
				if err != nil {
					log.Fatalf("invalid value for bounds (%v). must be a float64", boundsParts[i])
				}
			}
		}

		if len(zooms) == 0 {
			//	check user input for zoom range
			if cacheMaxZoom != 0 {
				if cacheMaxZoom >= cacheMinZoom {
					for i := cacheMinZoom; i <= cacheMaxZoom; i++ {
						zooms = append(zooms, i)
					}
				} else {
					log.Fatalf("invalid zoom range. min (%v) is greater than max (%v)", cacheMinZoom, cacheMaxZoom)
				}
			} else {
				//	every zoom
				for i := uint(0); i <= atlas.MaxZoom; i++ {
					zooms = append(zooms, i)
				}
			}

		}

		//	setup a waitgroup
		var wg sync.WaitGroup

		//	TODO: check for tile count. if tile count < concurrency, use tile count
		wg.Add(cacheConcurrency)

		//	new channel for the workers
		tiler := make(chan MapTile)

		//	setup our workers based on the amount of concurrency we have
		for i := 0; i < cacheConcurrency; i++ {
			//	spin off a worker listening on a channel
			go func(tiler chan MapTile) {
				ctx, cancel := context.WithCancel(context.Background())
				go func() {
					<-gdcmd.Cancelled()
					cancel()
				}()
				//	range our channel to listen for jobs
				for mt := range tiler {
					if gdcmd.IsCancelled() {
						continue
					}
					//	we will only have a single command arg so we can switch on index 0
					switch args[0] {
					case "seed":
						//	track how long the tile generation is taking
						t := time.Now()

						//	lookup the Map
						m, err := atlas.GetMap(mt.MapName)
						if err != nil {
							log.Fatalf("error seeding tile (%+v): %v", mt.Tile, err)
						}

						//	filter down the layers we need for this zoom
						m = m.FilterLayersByZoom(mt.Tile.Z)

						//	check if overwriting the cache is not ok
						if !cacheOverwrite {
							//	lookup our cache
							c := atlas.GetCache()
							if c == nil {
								log.Fatalf("error fetching cache: %v", err)
							}

							//	cache key
							key := cache.Key{
								MapName: mt.MapName,
								Z:       mt.Tile.Z,
								X:       mt.Tile.X,
								Y:       mt.Tile.Y,
							}

							//	read the tile from the cache
							_, hit, err := c.Get(&key)
							if err != nil {
								log.Fatal("error reading from cache: %v", err)
							}
							//	if we have a cache hit, then skip processing this tile
							if hit {
								log.Infof("cache seed set to not overwrite existing tiles. skipping map (%v) tile (%v/%v/%v)", mt.MapName, mt.Tile.Z, mt.Tile.X, mt.Tile.Y)
								continue
							}
						}

						//	set tile buffer if it was configured by the user
						if conf.TileBuffer > 0 {
							mt.Tile.Buffer = float64(conf.TileBuffer)
						}

						//	seed the tile
						if err = atlas.SeedMapTile(ctx, m, mt.Tile.Z, mt.Tile.X, mt.Tile.Y); err != nil {
							log.Errorf("error seeding tile (%+v): %v", mt.Tile, err)
							break
						}

						//	TODO: this is a hack to get around large arrays not being garbage collected
						//	https://github.com/golang/go/issues/14045 - should be addressed in Go 1.11
						runtime.GC()

						log.Infof("seeding map (%v) tile (%v/%v/%v) took: %v", mt.MapName, mt.Tile.Z, mt.Tile.X, mt.Tile.Y, time.Now().Sub(t))

					case "purge":
						log.Infof("purging map (%v) tile (%v/%v/%v)", mt.MapName, mt.Tile.Z, mt.Tile.X, mt.Tile.Y)

						//	lookup the Map
						m, err := atlas.GetMap(mt.MapName)
						if err != nil {
							log.Fatalf("error seeding tile (%+v): %v", mt.Tile, err)
						}

						//	purge the tile
						if err = atlas.PurgeMapTile(m, mt.Tile); err != nil {
							log.Errorf("error purging tile (%+v): %v", mt.Tile, err)
							break
						}
					}
				}

				//	Done() will be called after close(channel) is called and the final job this worker is processing completes
				wg.Done()
			}(tiler)
		}
		//	iterate our zoom range
	ZoomLoop:
		for i := range zooms {

			topLeft := *tegola.NewTileLatLong(zooms[i], bounds[1], bounds[0])
			minx, maxy = topLeft.Deg2Num()

			bottomRight := *tegola.NewTileLatLong(zooms[i], bounds[3], bounds[2])
			maxx, miny = bottomRight.Deg2Num()

			//	range rows
			for x := minx; x <= maxx; x++ {
				//	range columns
				for y := miny; y <= maxy; y++ {
					//	range maps
					for m := range maps {
						mapTile := MapTile{
							MapName: maps[m].Name,
							Tile:    tegola.NewTile(zooms[i], uint(x), uint(y)),
						}
						select {
						case tiler <- mapTile:
						case <-gdcmd.Cancelled():
							log.Info("cancel received; cleaning up…")
							break ZoomLoop
						}

					}
				}
			}
		}

		//	close the channel to notify the workers all jobs have been dispatched
		close(tiler)

		//	wait for the workers to complete any remaining jobs
		wg.Wait()
	},
}

type MapTile struct {
	MapName string
	Tile    *tegola.Tile
}

//parseTileString converts a Z/X/Y formatted string into a tegola tile
func parseTileString(str string) (*tegola.Tile, error) {
	var tile *tegola.Tile

	parts := strings.Split(str, "/")
	if len(parts) != 3 {
		return tile, fmt.Errorf("invalid zxy value (%v). expecting the format z/x/y", str)
	}

	z, err := strconv.ParseUint(parts[0], 10, 64)
	if err != nil || z > tegola.MaxZ {
		return tile, fmt.Errorf("invalid Z value (%v)", z)
	}

	maxXYatZ := maths.Exp2(z) - 1

	x, err := strconv.ParseUint(parts[1], 10, 64)
	if err != nil || x > maxXYatZ {
		return tile, fmt.Errorf("invalid X value (%v)", x)
	}

	y, err := strconv.ParseUint(parts[2], 10, 64)
	if err != nil || y > maxXYatZ {
		return tile, fmt.Errorf("invalid Y value (%v)", y)
	}
	tile = tegola.NewTile(uint(z), uint(x), uint(y))

	return tile, nil
}
