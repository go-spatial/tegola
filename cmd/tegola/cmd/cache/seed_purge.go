package cache

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"strconv"
	"strings"

	"github.com/alexeydott/geom"
	"github.com/alexeydott/geom/slippy"
	"github.com/alexeydott/proj"
	"github.com/alexeydott/tegola/atlas"
	"github.com/alexeydott/tegola/internal/build"
	gdcmd "github.com/alexeydott/tegola/internal/cmd"
	"github.com/alexeydott/tegola/internal/log"
	"github.com/alexeydott/tegola/observability"
	"github.com/alexeydott/tegola/provider"
	"github.com/go-spatial/cobra"
)

// metricBoundLimit is the canonical Web Mercator world extent in meters:
// half the earth circumference at the equator (20037508.342789244). It is
// used to validate bounds for the supported metric source SRIDs (EPSG:3857,
// EPSG:3395 and EPSG:4087).
const metricBoundLimit = 20037508.342789244

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
	// cacheConcurrency is the amount of concurrency to use. defaults to the number of CPUs on the machine
	cacheConcurrency int
	// cacheOverwrite determines if we should overwrite already existing files or skip them
	cacheOverwrite bool
	// cacheBounds is the bounds that the cache is within. defaults to -180, -85.0511, 180, 85.0511
	cacheBounds string
	// cacheBoundsSRID is the srid of grid system that the bounds is at. Should Default to 4326
	cacheBoundsSRID int
	// cacheMap is the name of the map
	cacheMap string
	// cacheLogThreshold is cache threshold while seeding, to log output for tiles that take longer than this (in milliseconds) to render
	cacheLogThreshold int64
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
	SeedPurgeCmd.PersistentFlags().Int64VarP(&cacheLogThreshold, "log-threshold", "", 0, "during seeding, only log tiles that take this number of milliseconds or longer to render (default all tiles)")

	SeedPurgeCmd.Flags().StringVarP(&cacheBounds, "bounds", "", "-180,-85.0511,180,85.0511", "lng/lat bounds to seed the cache with in the format: minx, miny, maxx, maxy")
	SeedPurgeCmd.Flags().IntVarP(&cacheBoundsSRID, "bounds-srid", "", int(proj.EPSG4326), "the srid of the grid system for bounds.")

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
		seedPurgeWorker = seedWorker(cacheOverwrite, cacheLogThreshold)
	default:

		return fmt.Errorf("expected purge/seed got (%v) for command name", cmdName)
	}
	build.Commands = append(build.Commands, "cache", cmdName)

	return nil

}

func IsKnownSrcConversionSRID(code proj.EPSGCode) bool {
	return code == proj.EPSG3395 ||
		code == proj.WebMercator ||
		code == proj.WGS84 ||
		code == proj.WorldEquidistantCylindrical
}

func AvailableSrcConversions() []proj.EPSGCode {
	return []proj.EPSGCode{
		proj.EPSG3395,
		proj.WebMercator,
		proj.WGS84,
		proj.WorldEquidistantCylindrical,
	}
}

// validateConcurrency ensures the requested worker count is usable: 0 would
// leave the seeder with no workers (it hangs) and negative values panic
// (part13 P6-24).
func validateConcurrency(n int) error {
	if n < 1 {
		return fmt.Errorf("invalid concurrency value (%d). concurrency must be at least 1", n)
	}
	return nil
}

// parseValidateBounds parses a "minx,miny,maxx,maxy" bounds string and
// validates it against the declared source SRID (part13 P6-26). Bounds are
// no longer unconditionally interpreted as lon/lat:
//
//   - EPSG:4326 bounds are degrees: longitude -180..180 and latitude
//     -90..90 (the widest valid values; note the default --bounds uses the
//     Web Mercator cut-off latitude +/-85.0511).
//   - The supported metric SRIDs (EPSG:3857, EPSG:3395, EPSG:4087) are
//     validated against the canonical Web Mercator world extent
//     +/-20037508.342789244 meters on both axes.
//
// For every SRID min <= max must hold on both axes.
func parseValidateBounds(srid int, bounds string) (b [4]float64, err error) {
	boundsParts := strings.Split(strings.TrimSpace(bounds), ",")
	if len(boundsParts) != 4 {
		return b, fmt.Errorf("invalid value for bounds (%v). expecting minx, miny, maxx, maxy", bounds)
	}

	xName, yName := "x", "y"
	xMin, xMax := -metricBoundLimit, metricBoundLimit
	yMin, yMax := -metricBoundLimit, metricBoundLimit
	if proj.EPSGCode(srid) == proj.WGS84 {
		xName, yName = "lng", "lat"
		xMin, xMax = -180, 180
		yMin, yMax = -90, 90
	}

	// parseBoundsValue parses and range-checks one axis value.
	parseBoundsValue := func(idx int, name string, lo, hi float64) error {
		v, err := strconv.ParseFloat(strings.TrimSpace(boundsParts[idx]), 64)
		if err != nil {
			return fmt.Errorf("invalid %s value(%v) for bounds (%v)", name, boundsParts[idx], bounds)
		}
		if v < lo || v > hi {
			return fmt.Errorf("invalid %s value(%v) for bounds (%v). for srid %d %s must be within %v..%v", name, boundsParts[idx], bounds, srid, name, lo, hi)
		}
		b[idx] = v
		return nil
	}

	if err = parseBoundsValue(0, xName, xMin, xMax); err != nil {
		return b, err
	}
	if err = parseBoundsValue(1, yName, yMin, yMax); err != nil {
		return b, err
	}
	if err = parseBoundsValue(2, xName, xMin, xMax); err != nil {
		return b, err
	}
	if err = parseBoundsValue(3, yName, yMin, yMax); err != nil {
		return b, err
	}

	// reject inverted bounds (min > max) on either axis
	if b[0] > b[2] {
		return b, fmt.Errorf("invalid bounds (%v). %s min (%v) is greater than %s max (%v)", bounds, xName, b[0], xName, b[2])
	}
	if b[1] > b[3] {
		return b, fmt.Errorf("invalid bounds (%v). %s min (%v) is greater than %s max (%v)", bounds, yName, b[1], yName, b[3])
	}

	return b, nil
}

func seedPurgeCmdValidate(cmd *cobra.Command, args []string) (err error) {
	// validate the concurrency flag
	if err = validateConcurrency(cacheConcurrency); err != nil {
		return err
	}

	// validate the cache-bounds-srid
	if !IsKnownSrcConversionSRID(proj.EPSGCode(cacheBoundsSRID)) {
		var str strings.Builder
		str.WriteString(fmt.Sprintf("SRID=%d is not a know conversion  ePSG code\n known codes are:", cacheBoundsSRID))
		for _, code := range AvailableSrcConversions() {
			str.WriteString(fmt.Sprintf(" %d\n", int(code)))
		}
		return errors.New(str.String())
	}

	// validate and set bounds flag according to the declared SRID (part13 P6-26)
	b, err := parseValidateBounds(cacheBoundsSRID, cacheBounds)
	if err != nil {
		return err
	}
	seedPurgeBounds = b

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
	gdcmd.OnComplete(observability.Cleanup)
	atlas.StartSubProcesses()

	go func() {
		select {
		case <-ctx.Done():
			return
		case <-gdcmd.Cancelled():
			cancel()
		}
	}()

	grid := slippy.NewGrid(proj.EPSGCode(cacheBoundsSRID), 0)

	log.Infof("zoom list: %v", zooms)
	tileChannel := generateTilesForBounds(ctx, seedPurgeBounds, zooms, grid)

	return doWork(ctx, tileChannel, seedPurgeMaps, cacheConcurrency, seedPurgeWorker)
}

func generateTilesForBounds(ctx context.Context, bounds [4]float64, zooms []uint, grid slippy.TileGridder) *TileChannel {

	tce := &TileChannel{
		channel: make(chan slippy.Tile),
	}

	if grid == nil {
		grid = slippy.NewGrid(proj.EPSGCode(cacheBoundsSRID), 0)
	}

	go func() {
		defer tce.Close()

		var extent geom.Extent = bounds
		for _, z := range zooms {

			tiles, err := slippy.FromBounds(grid, &extent, slippy.Zoom(z))
			if err != nil {
				tce.setError(fmt.Errorf("got error trying to get tiles: %w", err))
				tce.Close()
				return
			}
			for _, tile := range tiles {
				t := tile
				select {
				case tce.channel <- t:
				case <-ctx.Done():
					// we have been cancelled
					return
				}
			}
		}
	}()
	return tce
}
