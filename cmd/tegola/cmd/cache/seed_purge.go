package cache

import (
	"context"
	"fmt"
	"runtime"
	"strings"

	"github.com/go-spatial/geom/slippy"
	"github.com/go-spatial/tegola"
	"github.com/go-spatial/tegola/atlas"
	"github.com/go-spatial/tegola/internal/log"
	"github.com/go-spatial/tegola/maths"
	"github.com/go-spatial/tegola/provider"
	"github.com/spf13/cobra"

	gdcmd "github.com/go-spatial/tegola/internal/cmd"
)

// Flag parameters
var (
	// the amount of concurrency to use. defaults to the number of CPUs on the machine
	cacheConcurrency int
	// cache overwrite
	cacheOverwrite bool
	// bounds to cache within. default -180, -85.0511, 180, 85.0511
	cacheBounds string
	// name of the map
	cacheMap string
)

// Variables that are not flags but set by the command.
var (
	seedPurgeWorker func(context.Context, MapTile) error
	seedPurgeBounds [4]float64
	seedPurgeMaps   []atlas.Map
)

var PurgeCmd = &cobra.Command{Use: "seed"}

var SeedCmd = &cobra.Command{Use: "purge"}

func init() {
	setupSeedPurgeCommands(SeedCmd, PurgeCmd)
}

func setupSeedPurgeCommands(commands ...*cobra.Command) {
	for _, command := range commands {
		command.PersistentFlags().StringVarP(&cacheMap, "map", "", "", "map name as defined in the config")
		command.PersistentFlags().IntVarP(&cacheConcurrency, "concurrency", "", runtime.NumCPU(), "the amount of concurrency to use. defaults to the number of CPUs on the machine")
		command.PersistentFlags().BoolVarP(&cacheOverwrite, "overwrite", "", false, "overwrite the cache if a tile already exists (default false)")

		command.Flags().StringVarP(&cacheBounds, "bounds", "", "-180,-85.0511,180,85.0511", "lng/lat bounds to seed the cache with in the format: minx, miny, maxx, maxy")
		command.Flags().UintVarP(&minZoom, "min-zoom", "", 0, "min zoom to seed cache from.")
		command.Flags().UintVarP(&maxZoom, "max-zoom", "", atlas.MaxZoom, "max zoom to see cache to")

		command.PersistentPreRunE = seedPurgeCmdValidatePersistent
		command.PreRunE = seedPurgeCmdValidate
		command.RunE = seedPurgeCommand

		command.Short = fmt.Sprintf("%v the cache", command.Use)
		command.Long = fmt.Sprintf("command to %v the tile cache", command.Use)
		command.Example = fmt.Sprintf("%v --bounds lng,lat,lng,lat", command.Use)

		command.AddCommand(TileListCmd)

	}
}

// seedPurgeCmdValidate will validate the presistent flags and set associated variables as needed
func seedPurgeCmdValidatePersistent(cmd *cobra.Command, args []string) error {

	if cmd.HasParent() {
		// Let's run the parents Persistent Run commands.
		pcmd := cmd.Parent()
		if pcmd.PersistentPreRunE != nil {
			if err := pcmd.PersistentPreRunE(pcmd, args); err != nil {
				return err
			}
		}
	}

	// check if the user defined a single map to work on
	if cacheMap != "" {
		m, err := atlas.GetMap(cacheMap)
		if err != nil {
			return err
		}

		seedPurgeMaps = append(seedPurgeMaps, m)
	} else {
		seedPurgeMaps = atlas.AllMaps()
		if len(seedPurgeMaps) == 0 {
			return fmt.Errorf("expected at least one map to be defined? Is you config correct?")
		}
	}

	switch cmdName := strings.ToLower(strings.TrimSpace(cmd.CalledAs())); cmdName {
	case "purge":
		seedPurgeWorker = purgeWorker
	case "seed":
		var pf64 *float64
		if Config.TileBuffer != nil {
			f64 := float64(*Config.TileBuffer)
			pf64 = &f64
		}
		seedPurgeWorker = seedWorker(pf64, cacheOverwrite)
	default:
		return fmt.Errorf("expected purge/seed got %v for command name.", cmdName)
	}

	return nil

}

func seedPurgeCmdValidate(cmd *cobra.Command, args []string) (err error) {

	// validate and set bounds flag
	boundsParts := strings.Split(strings.TrimSpace(cacheBounds), ",")
	if len(boundsParts) != 4 {
		return fmt.Errorf("invalid value for bounds (%v). expecting minx, miny, maxx, maxy", cacheBounds)
	}

	var ok bool

	if seedPurgeBounds[0], ok = IsValidLngString(boundsParts[0]); !ok {
		return fmt.Errorf("0: invalid lng value(%v) for bounds (%v).", boundsParts[0], cacheBounds)
	}
	if seedPurgeBounds[1], ok = IsValidLatString(boundsParts[1]); !ok {
		return fmt.Errorf("1: invalid lat value(%v) for bounds (%v).", boundsParts[1], cacheBounds)
	}
	if seedPurgeBounds[2], ok = IsValidLngString(boundsParts[2]); !ok {
		return fmt.Errorf("2: invalid lng value(%v) for bounds (%v).", boundsParts[2], cacheBounds)
	}
	if seedPurgeBounds[3], ok = IsValidLatString(boundsParts[3]); !ok {
		return fmt.Errorf("3: invalid lat value(%v) for bounds (%v).", boundsParts[3], cacheBounds)
	}

	// get the zoom ranges
	if err = minMaxZoomValidate(cmd, args); err != nil {
		return err
	}

	return nil
}

func minMaxZoomValidate(cmd *cobra.Command, args []string) (err error) {

	zooms, err = sliceFromRange(minZoom, maxZoom)
	if err != nil {
		return fmt.Errorf("invalid zoom range, %v", err)
	}
	return nil
}

func seedPurgeCommand(cmd *cobra.Command, args []string) (err error) {

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	defer gdcmd.New().Complete()
	gdcmd.OnComplete(provider.Cleanup)

	go func() {
		select {
		case <-ctx.Done():
			return
		case <-gdcmd.Cancelled():
			cancel()
		}
	}()

	log.Info("zoom list: ", zooms)
	tilechannel := generateTilesForBounds(ctx, seedPurgeBounds, zooms)
	log.Infof("Maps are: %v", seedPurgeMaps)
	return doWork(ctx, tilechannel, seedPurgeMaps, cacheConcurrency, seedPurgeWorker)

}

func generateTilesForBounds(ctx context.Context, bounds [4]float64, zooms []uint) *TileChannel {

	tce := &TileChannel{
		channel: make(chan *slippy.Tile),
	}

	go func() {
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

		MainLoop:
			for x := xi; x <= xf; x++ {
				// loop columns
				for y := yi; y <= yf; y++ {
					select {
					case tce.channel <- slippy.NewTile(z, x, y, 0, tegola.WebMercator):
					case <-ctx.Done():
						// we have been cancelled
						break MainLoop
					}
				}
			}
		}
		tce.Close()
	}()
	return tce
}
