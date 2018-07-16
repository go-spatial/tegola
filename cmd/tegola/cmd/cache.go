package cmd

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/spf13/cobra"

	gdcmd "github.com/go-spatial/tegola/internal/cmd"

	"github.com/go-spatial/geom/slippy"
	"github.com/go-spatial/tegola"
	"github.com/go-spatial/tegola/atlas"
	"github.com/go-spatial/tegola/cache"
	"github.com/go-spatial/tegola/internal/log"
	"github.com/go-spatial/tegola/maths"
	"github.com/go-spatial/tegola/provider"
)

var (
	// specify a tile to cache. ignored by default
	cacheZXY string
	// read zxy values from a file
	cacheFile string
	//	filter which maps to process. default will operate on all mapps
	cacheMap string
	// the min zoom to cache from
	cacheMinZoom uint
	// the max zoom to cache to
	cacheMaxZoom uint
	// bounds to cache within. default -180, -85.0511, 180, 85.0511
	cacheBounds string
	// the amount of concurrency to use. defaults to the number of CPUs on the machine
	cacheConcurrency int
	// cache overwrite
	cacheOverwrite bool
	// input string format
	cacheFormat string
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

		var maps []atlas.Map

		initConfig()

		// check if the user defined a single map to work on
		if cacheMap != "" {
			m, err := atlas.GetMap(cacheMap)
			if err != nil {
				log.Fatal(err)
			}

			maps = append(maps, m)
		} else {
			maps = atlas.AllMaps()
		}

		// check for a cache backend
		if atlas.GetCache() == nil {
			log.Fatalf("mising cache backend. check your config (%v)", configFile)
		}

		zooms, err := sliceFromRange(cacheMinZoom, cacheMaxZoom)
		if err != nil {
			log.Fatalf("invalid zoom range, %v", err)
		}

		tileChan := make(chan *slippy.Tile)
		go func() {
			if err := sendTiles(zooms, tileChan); err != nil {
				log.Fatal(err)
			}
		}()

		log.Info("zoom list: ", zooms)
		//	setup a waitgroup
		var wg sync.WaitGroup

		// TODO: check for tile count. if tile count < concurrency, use tile count
		wg.Add(cacheConcurrency)

		// new channel for the workers
		tiler := make(chan MapTile)

		// setup our workers based on the amount of concurrency we have
		for i := 0; i < cacheConcurrency; i++ {
			go func() {
				ctx, cancel := context.WithCancel(context.Background())
				go func() {
					<-gdcmd.Cancelled()
					cancel()
				}()

				// range our channel to listen for jobs
				for mt := range tiler {
					if gdcmd.IsCancelled() {
						break
					}
					//	we will only have a single command arg so we can switch on index 0
					var err error
					switch args[0] {
					case "seed":
						err = SeedWorker(ctx, mt)
					case "purge":
						err = PurgeWorker(mt)
					default:
						log.Fatalf("sub-command %q not recognized", args[0])
					}

					if err != nil {
						log.Fatal(err)
					}
				}

				wg.Done()
			}()

			//	Done() will be called after close(channel) is called and the final job this SeedWorker is processing completes
		}

		for tile := range tileChan {
			for m := range maps {
				mapTile := MapTile{
					MapName: maps[m].Name,
					Tile:    tile,
				}
				select {
				case tiler <- mapTile:
				case <-gdcmd.Cancelled():
					log.Info("cancel recieved. cleaning up…")
					break
				}
			}
		}

		// close the channel to notify the workers all jobs have been dispatched
		close(tiler)

		// wait for the workers to complete any remaining jobs
		wg.Wait()
	},
}

func sliceFromRange(min, max uint) ([]uint, error) {
	var ret []uint
	if max < min {
		return nil, fmt.Errorf("min (%v) is greater than max (%v)", min, max)
	}

	ret = make([]uint, max-min+1)
	for i := min; i <= max; i++ {
		ret[i-min] = i
	}

	return ret, nil
}

type MapTile struct {
	MapName string
	Tile    *slippy.Tile
}

func SeedWorker(ctx context.Context, mt MapTile) error {
	//	track how long the tile generation is taking
	t := time.Now()

	//	lookup the Map
	m, err := atlas.GetMap(mt.MapName)
	if err != nil {
		return fmt.Errorf("error seeding tile (%+v): %v", mt.Tile, err)
	}

	z, x, y := mt.Tile.ZXY()

	//	filter down the layers we need for this zoom
	m = m.FilterLayersByZoom(z)

	//	check if overwriting the cache is not ok
	if !cacheOverwrite {
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

	//	set tile buffer if it was configured by the user
	if conf.TileBuffer != nil {
		mt.Tile.Buffer = float64(*conf.TileBuffer)
	}

	//	seed the tile
	if err = atlas.SeedMapTile(ctx, m, z, x, y); err != nil {
		return fmt.Errorf("error seeding tile (%+v): %v", mt.Tile, err)
	}

	//	TODO: this is a hack to get around large arrays not being garbage collected
	//	https://github.com/golang/go/issues/14045 - should be addressed in Go 1.11
	runtime.GC()

	log.Infof("seeding map (%v) tile (%v/%v/%v) took: %v", mt.MapName, z, x, y, time.Now().Sub(t))

	return nil
}

func PurgeWorker(mt MapTile) error {

	z, x, y := mt.Tile.ZXY()

	log.Infof("purging map (%v) tile (%v/%v/%v)", mt.MapName, z, x, y)

	//	lookup the Map
	m, err := atlas.GetMap(mt.MapName)
	if err != nil {
		return fmt.Errorf("error seeding tile (%+v): %v", mt.Tile, err)
	}

	//	purge the tile
	ttile := tegola.NewTile(mt.Tile.ZXY())

	if err = atlas.PurgeMapTile(m, ttile); err != nil {
		return fmt.Errorf("error purging tile (%+v): %v", mt.Tile, err)
	}

	return nil
}

func sendTiles(zooms []uint, c chan *slippy.Tile) error {
	defer close(c)

	format, err := NewFormat(cacheFormat)
	if err != nil {
		return err
	}

	switch {
	case cacheZXY != "": // single tile
		//	convert the input into a tile
		z, x, y, err := format.Parse(cacheZXY)
		if err != nil {
			return err
		}

		tile := slippy.NewTile(z, x, y, tegola.DefaultTileBuffer, tegola.WebMercator)

		for _, zoom := range zooms {
			err := tile.RangeFamilyAt(zoom, func(t *slippy.Tile) error {
				if gdcmd.IsCancelled() {
					return fmt.Errorf("cache manipulation interrupted")
				}

				c <- t
				return nil
			})

			// graceful stop if cancelled
			if err != nil {
				return err
			}
		}

		return nil
	case cacheFile != "": // read xyz from a file (tile list)
		f, err := os.Open(cacheFile)
		if err != nil {
			return fmt.Errorf("unable to open file (%v): %v", cacheFile, err)
		}

		scanner := bufio.NewScanner(f)

		for scanner.Scan() {
			z, x, y, err := format.Parse(scanner.Text())
			if err != nil {
				return err
			}
			tile := slippy.NewTile(z, x, y, 0, tegola.WebMercator)

			c <- tile

			// range
			for _, zoom := range zooms {
				err := tile.RangeFamilyAt(zoom, func(t *slippy.Tile) error {
					if gdcmd.IsCancelled() {
						return fmt.Errorf("cache manipulation interrupted")
					}

					if z1, x1, y1 := t.ZXY(); z != z1 || x != x1 || y != y1 {
						c <- t
					}

					return nil
				})

				// graceful stop if cancelled
				if err != nil {
					return err
				}
			}
		}

		return nil
	default: // bounding box caching
		var err error

		boundsParts := strings.Split(cacheBounds, ",")
		if len(boundsParts) != 4 {
			return fmt.Errorf("invalid value for bounds (%v). expecting minx, miny, maxx, maxy", cacheBounds)
		}

		bounds := make([]float64, 4)

		for i := range boundsParts {
			bounds[i], err = strconv.ParseFloat(boundsParts[i], 64)
			if err != nil {
				return fmt.Errorf("invalid value for bounds (%v). must be a float64", boundsParts[i])
			}
		}

		for _, z := range zooms {
			// get the tiles at the corners given the bounds and zoom
			corner1 := slippy.NewTileLatLon(z, bounds[1], bounds[0], 0, tegola.WebMercator)
			corner2 := slippy.NewTileLatLon(z, bounds[3], bounds[2], 0, tegola.WebMercator)

			// x,y initials and finals
			_, xi, yi := corner1.ZXY()
			_, xf, yf := corner2.ZXY()

			maxXYatZ := uint(maths.Exp2(uint64(z))) - 1

			// ensure the initials are smaller than finals
			if xi > xf {
				xi, xf = xf, xi
			}
			if yi > yf {
				yi, yf = yf, yi
			}

			// prevent seeding out of bounds
			xf = maths.Min(xf, maxXYatZ)
			yf = maths.Min(yf, maxXYatZ)

			// loop rows
			for x := xi; x <= xf; x++ {
				// loop columns
				for y := yi; y <= yf; y++ {

					if gdcmd.IsCancelled() {
						return fmt.Errorf("cache manipulation interrupted")
					}

					// send tile over the channel
					c <- slippy.NewTile(z, x, y, 0, tegola.WebMercator)
				}
			}
		}

		return nil
	}
}
