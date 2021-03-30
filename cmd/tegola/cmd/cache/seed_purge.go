package cache

import (
	"context"
	"fmt"
	"runtime"
	"strings"

	"github.com/go-spatial/tegola/internal/build"

	"github.com/go-spatial/cobra"
	"github.com/go-spatial/geom/slippy"
	"github.com/go-spatial/tegola/atlas"
	"github.com/go-spatial/tegola/internal/log"
	"github.com/go-spatial/tegola/maths"
	"github.com/go-spatial/tegola/provider"

	gdcmd "github.com/go-spatial/tegola/internal/cmd"
)

const defaultUsage = `Usage:{{if .Runnable}}
  {{.UseLine}}{{end}}{{if .HasAvailableSubCommands}}
  {{.CommandPath}} [command]{{end}}{{if gt (len .Aliases) 0}}

Aliases:
  {{.NameAndAliases}}{{end}}{{if .HasExample}}

Examples:
  {{.Example}}{{end}}{{if .HasAvailableSubCommands}}

Available Commands:{{range .Commands}}{{if (or .IsAvailableCommand (eq .Name "help"))}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{end}}{{if .HasAvailableLocalFlags}}

Flags:
{{.LocalFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasAvailableInheritedFlags}}
Global Flags:
{{.InheritedFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasHelpSubCommands}}
Additional help topics:{{range .Commands}}{{if .IsAdditionalHelpTopicCommand}}
  {{rpad .CommandPath .CommandPathPadding}} {{.Short}}{{end}}{{end}}{{end}}{{if .HasAvailableSubCommands}}
Use "{{.CommandPath}} [command] --help" for more information about a command.{{end}}
`

// flag parameters
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

// variables that are not flags but set by the command.
var (
	seedPurgeWorker func(context.Context, MapTile) error
	seedPurgeBounds [4]float64
	seedPurgeMaps   []atlas.Map
)

var SeedPurgeCmd = &cobra.Command{
	Use:     "seed",
	Aliases: []string{"purge"},
	Short:   "seed or purge tiles from the cache",
	Long:    "command to seed or purge tiles from the cache",
	Example: "tegola cache seed --bounds lng,lat,lng,lat",
}

func init() {
	setupMinMaxZoomFlags(SeedPurgeCmd, 0, atlas.MaxZoom)
	SeedPurgeCmd.PersistentFlags().StringVarP(&cacheMap, "map", "", "", "map name as defined in the config")
	SeedPurgeCmd.PersistentFlags().IntVarP(&cacheConcurrency, "concurrency", "", runtime.NumCPU(), "the amount of concurrency to use. defaults to the number of CPUs on the machine")
	SeedPurgeCmd.PersistentFlags().BoolVarP(&cacheOverwrite, "overwrite", "", false, "overwrite the cache if a tile already exists (default false)")

	SeedPurgeCmd.Flags().StringVarP(&cacheBounds, "bounds", "", "-180,-85.0511,180,85.0511", "lng/lat bounds to seed the cache with in the format: minx, miny, maxx, maxy")

	SeedPurgeCmd.PersistentPreRunE = seedPurgeCmdValidatePersistent
	SeedPurgeCmd.PreRunE = seedPurgeCmdValidate
	SeedPurgeCmd.RunE = seedPurgeCommand

	SeedPurgeCmd.SetUsageTemplate(defaultUsage)

	SeedPurgeCmd.AddCommand(TileListCmd)
	SeedPurgeCmd.AddCommand(TileNameCmd)
}

// seedPurgeCmdValidate will validate the persistent flags and set associated variables as needed
func seedPurgeCmdValidatePersistent(cmd *cobra.Command, args []string) error {

	if cmd.HasParent() {
		// run the parents Persistent Run commands.
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

		seedPurgeMaps = []atlas.Map{m}
	} else {
		seedPurgeMaps = atlas.AllMaps()
		if len(seedPurgeMaps) == 0 {
			return fmt.Errorf("expected at least one map to be defined. check your config")
		}
	}

	// Find the seed command and find out what it was called as.
	seedcmd := cmd
	cmdName := ""
	for seedcmd != nil {
		if seedcmd.Name() == "seed" {
			cmdName = seedcmd.CalledAs()
			break
		}
		seedcmd = seedcmd.Parent()
	}

	//cmdName := strings.ToLower(strings.TrimSpace(cmd.CalledAs()))
	switch cmdName {
	case "purge":
		seedPurgeWorker = purgeWorker
	case "seed":
		seedPurgeWorker = seedWorker(cacheOverwrite)
	default:

		return fmt.Errorf("expected purge/seed got (%v) for command name", cmdName)
	}
	build.Commands = append(build.Commands, "cache", cmdName)

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
		return fmt.Errorf("invalid lng value(%v) for bounds (%v)", boundsParts[0], cacheBounds)
	}
	if seedPurgeBounds[1], ok = IsValidLatString(boundsParts[1]); !ok {
		return fmt.Errorf("invalid lat value(%v) for bounds (%v)", boundsParts[1], cacheBounds)
	}
	if seedPurgeBounds[2], ok = IsValidLngString(boundsParts[2]); !ok {
		return fmt.Errorf("invalid lng value(%v) for bounds (%v)", boundsParts[2], cacheBounds)
	}
	if seedPurgeBounds[3], ok = IsValidLatString(boundsParts[3]); !ok {
		return fmt.Errorf("invalid lat value(%v) for bounds (%v)", boundsParts[3], cacheBounds)
	}

	// get the zoom ranges
	if err = minMaxZoomValidate(cmd, args); err != nil {
		return err
	}

	return nil
}

func seedPurgeCommand(_ *cobra.Command, _ []string) (err error) {

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	defer gdcmd.New().Complete()
	gdcmd.OnComplete(provider.Cleanup)
	atlas.PublishBuildInfo()

	go func() {
		select {
		case <-ctx.Done():
			return
		case <-gdcmd.Cancelled():
			cancel()
		}
	}()

	log.Info("zoom list: ", zooms)
	tileChannel := generateTilesForBounds(ctx, seedPurgeBounds, zooms)

	return doWork(ctx, tileChannel, seedPurgeMaps, cacheConcurrency, seedPurgeWorker)
}

func generateTilesForBounds(ctx context.Context, bounds [4]float64, zooms []uint) *TileChannel {

	tce := &TileChannel{
		channel: make(chan *slippy.Tile),
	}

	go func() {
		defer tce.Close()
		for _, z := range zooms {
			// get the tiles at the corners given the bounds and zoom
			corner1 := slippy.NewTileLatLon(z, bounds[1], bounds[0])
			corner2 := slippy.NewTileLatLon(z, bounds[3], bounds[2])

			// x,y initials and finals
			_, xi, yi := corner1.ZXY()
			_, xf, yf := corner2.ZXY()

			maxXYatZ := uint(maths.Exp2(uint64(z))) - 1

			// ensure the initials are smaller than finals
			// this breaks at the anti meridian: https://github.com/go-spatial/tegola/issues/500
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
					case tce.channel <- slippy.NewTile(z, x, y):
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
