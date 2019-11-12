package cache

import (
	"context"
	"fmt"
	"strings"

	"github.com/go-spatial/proj"

	"github.com/go-spatial/cobra"
	"github.com/go-spatial/geom/slippy"
	"github.com/go-spatial/tegola/atlas"
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
	//TODO (meilinger)
	tileNameTile, err = format.ParseTile(tileString, proj.WebMercator)
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
	tilechannel, err := generateTilesForTileName(ctx, tileNameTile, explicit, zooms, atlas.AllMaps())
	if err != nil {
		return err
	}

	// start up workers
	return doWork(ctx, tilechannel, cacheConcurrency, seedPurgeWorker)
}

func generateTilesForTileName(ctx context.Context, tile *slippy.Tile, explicit bool, zooms []uint, maps []atlas.Map) (*TileChannel, error) {
	if len(maps) == 0 {
		return nil, fmt.Errorf("no maps defined")
	}

	tce := &TileChannel{
		channel: make(chan *MapTile),
	}
	go func() {
		defer tce.Close()
		if tile == nil {
			return
		}

		for _, m := range maps {
			srid := uint(m.SRID)

			if explicit || len(zooms) == 0 {
				select {
				case tce.channel <- &MapTile{MapName: m.Name, Tile: tile}:
				case <-ctx.Done():
					// we have been cancelled
					return
				}
				return
			}

			for _, zoom := range zooms {
				// range will include the original tile.
				err := tile.RangeFamilyAt(zoom, srid, func(tile *slippy.Tile, srid uint) error {
					select {
					case tce.channel <- &MapTile{MapName: m.Name, Tile: tile}:
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
	return tce, nil
}
