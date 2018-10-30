package cache

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/go-spatial/geom/slippy"
	"github.com/go-spatial/tegola/atlas"
	"github.com/go-spatial/tegola/internal/log"
	"github.com/go-spatial/tegola/provider"
	"github.com/spf13/cobra"

	gdcmd "github.com/go-spatial/tegola/internal/cmd"
)

var (
	// the minimum zoom to cache from
	maxZoom uint
	// the maximum zoom to cache to
	minZoom uint
	// the zoom ranges
	zooms []uint
	// input string format
	tileListFormat string
)

var tileListFile *os.File
var format Format
var explicit bool

var TileListCmd = &cobra.Command{
	Use:   "tile-list filename|-",
	Short: "path to file with tile entries.",
	RunE:  tileListCommand,
}

func init() {
	TileListCmd.Flags().UintVarP(&minZoom, "min-zoom", "", 0, "min zoom to seed cache from.")
	TileListCmd.Flags().UintVarP(&maxZoom, "max-zoom", "", atlas.MaxZoom, "max zoom to see cache to")
	TileListCmd.Flags().StringVarP(&tileListFormat, "tile-name-format", "", "/zxy", "4 character string where the first character is a non-numeric delimiter followed by \"z\", \"x\" and \"y\" defining the coordinate order")
}

func tileListValidate(cmd *cobra.Command, args []string) (err error) {

	explicit = !(cmd.Flag("min-zoom").Changed || cmd.Flag("max-zoom").Changed)
	if !explicit {
		// get the zoom ranges.
		if err = minMaxZoomValidate(cmd, args); err != nil {
			return err
		}
	}

	if len(args) == 0 {
		return fmt.Errorf("Filename must be provided.")
	}
	fname := strings.TrimSpace(args[0])
	// - is used to indicate the use of stdin.
	if fname != "-" {
		// we have been provided a file name
		// let's set that up
		if tileListFile, err = os.Open(args[0]); err != nil {
			return err
		}
	}
	format, err = NewFormat(tileListFormat)
	return err
}

func tileListCommand(cmd *cobra.Command, args []string) (err error) {

	ctx, cancel := context.WithCancel(context.Background())
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

	var in io.Reader = os.Stdin
	if tileListFile != nil {
		in = tileListFile
		defer tileListFile.Close()
	}

	log.Info("zoom list: ", zooms)

	tilechannel := generateTilesForTileList(ctx, in, explicit, zooms)

	// start up workers here
	return doWork(ctx, tilechannel, seedPurgeMaps, cacheConcurrency, seedPurgeWorker)
}

// generateTilesForTileList will return a channel where all the tiles in the list will be published
// if explicit is false and zooms is not empty, it will include the tiles above and below with in the provided zooms
func generateTilesForTileList(ctx context.Context, tilelist io.Reader, explicit bool, zooms []uint) *TileChannel {
	tce := &TileChannel{
		channel: make(chan *slippy.Tile),
	}
	go func() {
		defer close(tce.channel)

		var (
			err        error
			lineNumber int
			tile       *slippy.Tile
		)

		scanner := bufio.NewScanner(tilelist)

		for scanner.Scan() {
			lineNumber++
			tile, err = format.ParseTile(scanner.Text())
			if err != nil {
				tce.setError(fmt.Errorf("Failed to parse line[%v]: %v", lineNumber, err))
				return
			}

			if explicit || len(zooms) == 0 {
				select {
				case tce.channel <- tile:
				case <-ctx.Done():
					// we have been cancelled
					return
				}
				continue
			}

			for _, zoom := range zooms {
				// Range will include the original tile.
				err = tile.RangeFamilyAt(zoom, func(Tile *slippy.Tile) error {
					select {
					case tce.channel <- tile:
					case <-ctx.Done():
						// we have been cancelled
						return context.Canceled
					}
					return nil
				})
				// gracefully stop if cancelled
				if err != nil {
					return
				}
			}
		}
	}()
	return tce
}
