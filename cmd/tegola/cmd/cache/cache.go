package cache

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"

	"github.com/go-spatial/geom/slippy"
	"github.com/go-spatial/tegola/atlas"
	"github.com/go-spatial/tegola/config"
	"github.com/go-spatial/tegola/internal/log"
	"github.com/spf13/cobra"
)

// The config from the main app
var Config *config.Config

var Cmd = &cobra.Command{
	Use:   "cache",
	Short: "command to manage the cache",
	Long:  "command to manage the cache",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		if cmd.HasParent() {
			// Let's run the parents Persistent Run commands.
			pcmd := cmd.Parent()
			if pcmd.PersistentPreRunE != nil {
				if err := pcmd.PersistentPreRunE(pcmd, args); err != nil {
					return err
				}
			}
		}
		return nil
	},
}

func init() {
	Cmd.AddCommand(SeedCmd)
	Cmd.AddCommand(PurgeCmd)
}

type TileChannel struct {
	channel  chan *slippy.Tile
	cl       sync.RWMutex
	isClosed bool
	l        sync.RWMutex
	err      error
}

func (tc *TileChannel) Channel() <-chan *slippy.Tile {
	if tc == nil {
		log.Infof("Returning nil for TileChannel")
		return nil
	}
	log.Infof("Returning tileChannel %v", tc.channel)
	return tc.channel
}

func (tc *TileChannel) Err() (e error) {
	if tc == nil {
		return nil
	}
	tc.l.RLock()
	e = tc.err
	tc.l.RUnlock()
	return e
}

func (tc *TileChannel) setError(err error) {
	if tc == nil {
		return
	}
	tc.l.Lock()
	tc.err = err
	tc.l.Unlock()
}
func (tc *TileChannel) Close() {
	var isClosed bool
	tc.cl.RLock()
	isClosed = tc.isClosed
	tc.cl.RUnlock()
	if isClosed {
		return
	}
	tc.cl.Lock()
	if !tc.isClosed {
		tc.isClosed = true
		close(tc.channel)
	}
	tc.cl.Unlock()
}

type MapTile struct {
	MapName string
	Tile    *slippy.Tile
}

func doWork(ctx context.Context, tileChannel *TileChannel, maps []atlas.Map, concurrency int, worker func(context.Context, MapTile) error) (err error) {
	var wg sync.WaitGroup
	// new channel for the workers
	tiler := make(chan MapTile)
	var cleanup bool
	var errLock sync.RWMutex
	var tileMapErr error

	if len(maps) == 0 {
		return fmt.Errorf("No maps defined.")
	}

	// Set up the workers
	wg.Add(concurrency)
	for i := 0; i < concurrency; i++ {
		go func(i int) {
			var cleanup bool
			log.Infof("%03v: Starting up worker for tiler...", i)
			// range our channel to listen for jobs
			for mt := range tiler {
				errLock.RLock()
				e := tileMapErr
				errLock.RUnlock()
				if e != nil {
					cleanup = true
					break
				}
				if err := worker(ctx, mt); err != nil {
					cleanup = true
					errLock.Lock()
					tileMapErr = err
					errLock.Unlock()
					break
				}
			}
			if cleanup {
				for _ = range tiler {
					log.Infof("%03v: Cleanup the tiler...", i)
					continue
				}
			}
			wg.Done()
		}(i)
	}

	log.Infof("Staring up TileChannelLoop")
	// Run through the incoming tiles, and generate the tileMaps as needed.
TileChannelLoop:
	for tile := range tileChannel.Channel() {
		for m := range maps {
			log.Info("Working on map for tile")

			if ctx.Err() != nil {
				cleanup = true
				break
			}

			{ // Worker error occured.
				errLock.RLock()
				e := tileMapErr
				errLock.RUnlock()
				if e != nil {
					cleanup = true
					break
				}
			}

			mapTile := MapTile{
				MapName: maps[m].Name,
				Tile:    tile,
			}

			log.Info("Going to wait on tiler")
			select {
			case tiler <- mapTile:
			case <-ctx.Done():
				log.Info("Got done from context for tiler.")
				cleanup = true
				break TileChannelLoop
			}
		}
	}
	log.Info("Closing out tiler....")

	close(tiler)
	if cleanup {
		// want to soak up any messages.
		for _ = range tileChannel.Channel() {
			log.Info("Soking up the tilChannel...")
			continue
		}
	}
	// Let our workers finish up.
	wg.Wait()
	if err = tileChannel.Err(); err != nil {
		return err
	}
	// if we did not have an error from the tile generator
	// return any error the workers may have had
	return tileMapErr
}

func IsValidLat(f64 float64) bool { return -90.0 <= f64 && f64 <= 90.0 }
func IsValidLatString(lat string) (float64, bool) {
	f64, err := strconv.ParseFloat(strings.TrimSpace(lat), 64)
	return f64, err == nil || IsValidLat(f64)
}

func IsValidLng(f64 float64) bool { return -180.0 <= f64 && f64 <= 180.0 }
func IsValidLngString(lng string) (float64, bool) {
	f64, err := strconv.ParseFloat(strings.TrimSpace(lng), 64)
	return f64, err == nil || IsValidLng(f64)
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
