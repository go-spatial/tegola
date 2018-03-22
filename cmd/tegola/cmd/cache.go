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
	"github.com/go-spatial/tegola/geom/slippy"
	"os"
	"bufio"
)

var (
	//	specify a tile to cache. ignored by default
	cacheZXY string
	// read zxy values from a file
	cacheFile string
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
		if cacheMaxZoom+cacheMinZoom != 0 {
			if cacheMaxZoom <= cacheMinZoom {
				log.Fatalf("invalid zoom range. min (%v) is greater than max (%v)", cacheMinZoom, cacheMaxZoom)
			}

			for i := cacheMinZoom; i <= cacheMaxZoom; i++ {
				zooms = append(zooms, i)
			}
		}

		tiles := make(chan *slippy.Tile)
		go sendTiles(zooms, tiles)

		log.Info("zoom list: ", zooms)
		//	setup a waitgroup
		var wg sync.WaitGroup

		//	TODO: check for tile count. if tile count < concurrency, use tile count
		wg.Add(cacheConcurrency)

		//	new channel for the workers
		tiler := make(chan MapTile)

		//	setup our workers based on the amount of concurrency we have
		for i := 0; i < cacheConcurrency; i++ {
			go func() {
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
						seedWorker(ctx, mt)
					case "purge":
						purgeWorker(mt)
					}
				}

				wg.Done()
			}()

			//	Done() will be called after close(channel) is called and the final job this seedWorker is processing completes
		}

		for tile := range tiles {
			for m := range maps {
				mapTile := MapTile{
					MapName: maps[m].Name,
					Tile:    tile,
				}
				select {
				case tiler <- mapTile:
				case <-gdcmd.Cancelled():
					log.Info("cancel recieved; cleaning up…")
					break
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
	Tile    *slippy.Tile
}

// parseTileString converts a Z/X/Y formatted string into a tegola tile
// format string "{delim}{order}" (ex. "/zxy", " zxy", ",zxy"
func parseTileString(format, str string) (*slippy.Tile, error) {
	var tile *slippy.Tile

	ix := 1
	iy := strings.Index(format, "y") - 1
	iz := strings.Index(format, "z") - 1

	parts := strings.Split(str, format[:1])
	//parts := strings.Split(str, "/")
	if len(parts) != 3 {
		return tile, fmt.Errorf("invalid zxy value (%v). expecting the format z/x/y", str)
	}

	z, err := strconv.ParseUint(parts[iz], 10, 64)
	if err != nil || z > tegola.MaxZ {
		return tile, fmt.Errorf("invalid Z value (%v)", z)
	}

	maxXYatZ := maths.Exp2(z) - 1

	x, err := strconv.ParseUint(parts[ix], 10, 64)
	if err != nil || x > maxXYatZ {
		return tile, fmt.Errorf("invalid X value (%v)", x)
	}

	y, err := strconv.ParseUint(parts[iy], 10, 64)
	if err != nil || y > maxXYatZ {
		return tile, fmt.Errorf("invalid Y value (%v)", y)
	}

	tile = slippy.NewTile(uint(z), uint(x), uint(y), 0, tegola.WebMercator)

	return tile, nil
}

func seedWorker(ctx context.Context, mt MapTile) {
	//	track how long the tile generation is taking
	t := time.Now()

	//	lookup the Map
	m, err := atlas.GetMap(mt.MapName)
	if err != nil {
		log.Fatalf("error seeding tile (%+v): %v", mt.Tile, err)
	}

	z, x, y := mt.Tile.ZXY()

	//	filter down the layers we need for this zoom
	m = m.FilterLayersByZoom(x)

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
			Z:       z,
			X:       x,
			Y:       y,
		}

		//	read the tile from the cache
		_, hit, err := c.Get(&key)
		if err != nil {
			log.Fatal("error reading from cache: %v", err)
		}
		//	if we have a cache hit, then skip processing this tile
		if hit {
			log.Infof("cache seed set to not overwrite existing tiles. skipping map (%v) tile (%v/%v/%v)", mt.MapName, z, x, y)
			return
		}
	}

	//	set tile buffer if it was configured by the user
	if conf.TileBuffer > 0 {
		mt.Tile.Buffer = float64(conf.TileBuffer)
	}

	//	seed the tile
	if err = atlas.SeedMapTile(ctx, m, z, x, y); err != nil {
		log.Errorf("error seeding tile (%+v): %v", mt.Tile, err)
		return
	}

	//	TODO: this is a hack to get around large arrays not being garbage collected
	//	https://github.com/golang/go/issues/14045 - should be addressed in Go 1.11
	runtime.GC()

	log.Infof("seeding map (%v) tile (%v/%v/%v) took: %v", mt.MapName, z, x, y, time.Now().Sub(t))
}

func purgeWorker(mt MapTile) {

	z, x, y := mt.Tile.ZXY()

	log.Infof("purging map (%v) tile (%v/%v/%v)", mt.MapName, z, x, y)

	//	lookup the Map
	m, err := atlas.GetMap(mt.MapName)
	if err != nil {
		log.Fatalf("error seeding tile (%+v): %v", mt.Tile, err)
	}

	//	purge the tile
	ttile := tegola.NewTile(mt.Tile.ZXY())
	if err = atlas.PurgeMapTile(m, ttile); err != nil {
		log.Errorf("error purging tile (%+v): %v", mt.Tile, err)
	}
}

func sendTiles(zooms []uint, c chan *slippy.Tile) {
	if cacheZXY != "" {
		// single xyz
		//	convert the input into a tile
		tile, err := parseTileString(cacheFormat, cacheZXY)
		if err != nil {
			log.Fatal(err)
		}

		for _, zoom := range zooms {
			tile.RangeChildren(zoom, func(t *slippy.Tile) error {
				c <- t
				return nil
			})
		}

		close(c)
	} else if cacheFile != "" {
		// read xyz from a file
		f, err := os.Open(cacheFile)
		if err != nil {
			log.Fatal("could not open file")
		}

		scanner := bufio.NewScanner(f)

		for scanner.Scan() {
			tile, err := parseTileString(cacheFormat, scanner.Text())
			if err != nil {
				log.Fatal(err)
			}

			// range
			for _, zoom := range zooms {
				tile.RangeChildren(zoom, func(t *slippy.Tile) error {
					c <- t
					return nil
				})
			}
		}
	} else {
		// bounding box caching
		boundsParts := strings.Split(cacheBounds, ",")
		if len(boundsParts) != 4 {
			log.Fatal("invalid value for bounds. expecting minx, miny, maxx, maxy")
		}

		var err error
		bounds := make([]float64, 4)

		for i := range boundsParts {
			bounds[i], err = strconv.ParseFloat(boundsParts[i], 64)
			if err != nil {
				log.Fatalf("invalid value for bounds (%v). must be a float64", boundsParts[i])
			}
		}

		maxZoom := zooms[len(zooms)-1]

		upperLeft := slippy.NewTileLatLon(maxZoom, bounds[1], bounds[0], 0, tegola.WebMercator)
		bottomRight := slippy.NewTileLatLon(maxZoom, bounds[3], bounds[2], 0, tegola.WebMercator)

		_, xi, yi := upperLeft.ZXY()
		_, xf, yf := bottomRight.ZXY()

		// TODO (@ear7h): find a way to keep from doing the same tile twice
		for x := xi; x <= xf; x ++ {
			for y := yi; y <= yf; y++ {
				root := slippy.NewTile(maxZoom, x, y, 0, tegola.WebMercator)

				for _, z := range zooms {
					root.RangeChildren(z, func(t *slippy.Tile) error {
						c <- t
						return nil
					})
				}
			}
		}

		close(c)
	}
}
