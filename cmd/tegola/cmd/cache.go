package cmd

import (
	"fmt"
	"log"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/spf13/cobra"

	"github.com/terranodo/tegola"
	"github.com/terranodo/tegola/atlas"
	"github.com/terranodo/tegola/cache"
	"github.com/terranodo/tegola/maths/webmercator"
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

		var zooms []int
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
			//	read the tile bounds, which will be in web mercator
			tBounds := t.BoundingBox()
			//	convert the bounds points to lat lon
			ul, err := webmercator.PToLonLat(tBounds.Minx, tBounds.Miny)
			if err != nil {
				log.Fatal(err)
			}
			lr, err := webmercator.PToLonLat(tBounds.Maxx, tBounds.Maxy)
			if err != nil {
				log.Fatal(err)
			}
			//	use the tile bounds as the bounds for the job.
			//	the grid flips between web mercator and WGS84 which is why we use must use lat from one point and lon from the other
			//	TODO: this smells funny. Investigate why the grid is flipping - arolek
			bounds[0] = ul[0]
			bounds[1] = lr[1]

			bounds[2] = lr[0]
			bounds[3] = ul[1]
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
						zooms = append(zooms, int(i))
					}
				} else {
					log.Fatalf("invalid zoom range. min (%v) is greater than max (%v)", cacheMinZoom, cacheMaxZoom)
				}
			} else {
				//	every zoom
				for i := 0; i <= atlas.MaxZoom; i++ {
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
				//	range our channel to listen for jobs
				for mt := range tiler {
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

						//	log.Println("Tile Z", mt.Tile.Z)

						//	filter down the layers we need for this zoom
						m = m.DisableAllLayers().EnableLayersByZoom(mt.Tile.Z)

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
								log.Printf("cache seed set to not overwrite existing tiles. skipping map (%v) tile (%v/%v/%v)", mt.MapName, mt.Tile.Z, mt.Tile.X, mt.Tile.Y)
								continue
							}
						}

						//	seed the tile
						if err = atlas.SeedMapTile(m, mt.Tile); err != nil {
							log.Fatalf("error seeding tile (%+v): %v", mt.Tile, err)
						}

						//	TODO: this is a hack to get around large arrays not being garbage collected
						//	https://github.com/golang/go/issues/14045 - should be addressed in Go 1.11
						runtime.GC()

						log.Printf("seeding map (%v) tile (%v/%v/%v) took: %v", mt.MapName, mt.Tile.Z, mt.Tile.X, mt.Tile.Y, time.Now().Sub(t))

					case "purge":
						log.Printf("purging map (%v) tile (%v/%v/%v)", mt.MapName, mt.Tile.Z, mt.Tile.X, mt.Tile.Y)

						//	lookup the Map
						m, err := atlas.GetMap(mt.MapName)
						if err != nil {
							log.Fatalf("error seeding tile (%+v): %v", mt.Tile, err)
						}

						//	purge the tile
						if err = atlas.PurgeMapTile(m, mt.Tile); err != nil {
							log.Fatalf("error purging tile (%+v): %v", mt.Tile, err)
						}
					}
				}

				//	Done() will be called after close(channel) is called and the final job this worker is processing completes
				wg.Done()
			}(tiler)
		}

		//	iterate our zoom range
		for i := range zooms {
			var err error

			topLeft := tegola.Tile{Z: zooms[i], Long: bounds[0], Lat: bounds[1]}
			minx, maxy, err = topLeft.Deg2Num()
			if err != nil {
				log.Printf("error calculating Z/X/Y tile from zoom (%v), and lat / long (%v, %v) skipping: %v", zooms[i], topLeft.Lat, topLeft.Long, err)
				continue
			}

			bottomRight := tegola.Tile{Z: zooms[i], Long: bounds[2], Lat: bounds[3]}

			maxx, miny, err = bottomRight.Deg2Num()
			if err != nil {
				log.Printf("error calculating Z/X/Y tile from zoom (%v), and lat / long (%v, %v) skipping: %v", zooms[i], bottomRight.Lat, bottomRight.Long, err)
				continue
			}

			//	range rows
			for x := minx; x <= maxx; x++ {
				//	range columns
				for y := miny; y <= maxy; y++ {
					//	range maps
					for m := range maps {
						mapTile := MapTile{
							MapName: maps[m].Name,
							Tile:    tegola.Tile{Z: zooms[i], X: x, Y: y},
						}

						tiler <- mapTile
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
	Tile    tegola.Tile
}

//	parseTileString converts a Z/X/Y formatted string into a tegola tile
func parseTileString(str string) (tegola.Tile, error) {
	var tile tegola.Tile

	parts := strings.Split(cacheZXY, "/")
	if len(parts) != 3 {
		return tile, fmt.Errorf("invalid zxy value (%v). expecting the format z/x/y", cacheZXY)
	}

	z, err := strconv.Atoi(parts[0])
	if err != nil {
		return tile, fmt.Errorf("invalid Z value (%v)", z)
	}
	if z < 0 {
		return tile, fmt.Errorf("negative zoom levels are not allowed")
	}

	x, err := strconv.Atoi(parts[1])
	if err != nil {
		return tile, fmt.Errorf("invalid X value (%v)", x)
	}

	y, err := strconv.Atoi(parts[2])
	if err != nil {
		return tile, fmt.Errorf("invalid Y value (%v)", y)
	}

	tile = tegola.Tile{
		Z: z,
		X: x,
		Y: y,
	}

	return tile, nil
}
