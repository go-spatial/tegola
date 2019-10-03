package cache

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/go-spatial/cobra"
	"github.com/go-spatial/geom/slippy"
	gdcmd "github.com/go-spatial/tegola/internal/cmd"
	"github.com/go-spatial/tegola/internal/log"
	"github.com/go-spatial/tegola/provider"
)

var (
	// the minimum zoom to cache from
	maxZoom uint
	// the maximum zoom to cache to
	minZoom uint
	// the zoom range
	zooms []uint
	// input string format
	tileListFormat string
)

var tileListFile *os.File
var format Format = defaultTileNameFormat
var explicit bool

var TileListCmd = &cobra.Command{
	Use:     "tile-list filename|-",
	Short:   "operate on a list of tile names separated by new lines",
	Example: "tile-list my-tile-list.txt",
	PreRunE: tileListValidate,
	RunE:    tileListCommand,
}

func init() {
	setupMinMaxZoomFlags(TileListCmd, 0, 0)
	setupTileNameFormat(TileListCmd)
}

func tileListValidate(cmd *cobra.Command, args []string) (err error) {

	explicit = IsMinMaxZoomExplicit(cmd)
	if !explicit {
		// get the zoom ranges.
		if err = minMaxZoomValidate(cmd, args); err != nil {
			return err
		}
	}

	if len(args) == 0 {
		return fmt.Errorf("filename must be provided.")
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
	return tileNameFormatValidate(cmd, args)
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

	tilechannel := generateTilesForTileList(ctx, in, explicit, zooms, format)

	// start up workers here
	return doWork(ctx, tilechannel, seedPurgeMaps, cacheConcurrency, seedPurgeWorker)
}

// generateTilesForTileList will return a channel where all the tiles in the list will be published
// if explicit is false and zooms is not empty, it will include the tiles above and below with in the provided zooms
func generateTilesForTileList(ctx context.Context, tilelist io.Reader, explicit bool, zooms []uint, format Format) *TileChannel {
	tce := &TileChannel{
		channel: make(chan *slippy.Tile),
	}
	go func() {
		defer tce.Close()

		var (
			err        error
			lineNumber int
			tile       *slippy.Tile
		)

		scanner := bufio.NewScanner(tilelist)

		for scanner.Scan() {
			lineNumber++
			txt := scanner.Text()
			tile, err = format.ParseTile(txt)
			if err != nil {
				tce.setError(fmt.Errorf("failed to parse line [%v]: %v", lineNumber, err))
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
				// range will include the original tile.
				err = tile.RangeFamilyAt(zoom, 3857, func(tile *slippy.Tile, srid uint) error {
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
