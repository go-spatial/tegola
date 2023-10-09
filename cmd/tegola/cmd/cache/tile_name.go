package cache

import (
	"context"
	"fmt"
	"strings"

	"github.com/go-spatial/cobra"
	"github.com/go-spatial/geom/slippy"
	gdcmd "github.com/go-spatial/tegola/internal/cmd"
	"github.com/go-spatial/tegola/internal/log"
	"github.com/go-spatial/tegola/provider"
)

var tileNameTile *slippy.Tile

var TileNameCmd = &cobra.Command{
	Use:     "tile-name z/x/y",
	Short:   "operate on a single tile formatted according to --format",
	Example: "tile-name 0/0/0",
	PreRunE: tileNameValidate,
	RunE:    tileNameCommand,
}

func init() {
	setupMinMaxZoomFlags(TileNameCmd, 0, 0)
	TileNameCmd.Flags().StringVarP(&tileListFormat, "format", "", "/zxy", "4 character string where the first character is a non-numeric delimiter followed by 'z', 'x' and 'y' defining the coordinate order")
}

func tileNameValidate(cmd *cobra.Command, args []string) (err error) {

	explicit = IsMinMaxZoomExplicit(cmd)
	if !explicit {
		// get the zoom ranges.
		if err = minMaxZoomValidate(cmd, args); err != nil {
			return err
		}
	}

	if len(args) == 0 {
		return fmt.Errorf("tile must be provided")
	}
	if err = tileNameFormatValidate(cmd, args); err != nil {
		return err
	}
	tileString := strings.TrimSpace(args[0])
	if tileString == "" {
		return fmt.Errorf("tile must be provided")
	}
	tileNameTile, err = format.ParseTile(tileString)
	if err != nil {
		return fmt.Errorf("unable to prase tile string (%v): %v", tileString, err)
	}
	return nil
}

func tileNameCommand(cmd *cobra.Command, args []string) (err error) {

	log.Infof("Running TileNameCommand: %v", args)

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

	log.Info("zoom list: ", zooms)
	tilechannel := generateTilesForTileName(ctx, tileNameTile, explicit, zooms)

	// start up workers
	return doWork(ctx, tilechannel, seedPurgeMaps, cacheConcurrency, seedPurgeWorker)

}

func generateTilesForTileName(ctx context.Context, tile *slippy.Tile, explicit bool, zooms []uint) *TileChannel {
	tce := &TileChannel{
		channel: make(chan *slippy.Tile),
	}

	go func() {
		defer tce.Close()
		if tile == nil {
			return
		}
		if explicit || len(zooms) == 0 {
			select {
			case tce.channel <- tile:
			case <-ctx.Done():
				// we have been cancelled
				return
			}
			return
		}
		for _, zoom := range zooms {
			// range will include the original tile.
			err := rangeFamilyAt(tile, zoom, func(tile *slippy.Tile) error {
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
	}()
	return tce
}
