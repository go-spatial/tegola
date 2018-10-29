package cache

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"

	"github.com/go-spatial/geom/slippy"
	"github.com/go-spatial/tegola/atlas"
	"github.com/spf13/cobra"
)

var Cmd = &cobra.Command{
	Use:   "cache",
	Short: "command to manage the cache",
	Long:  "command to manage the cache",
}

func init() {
	Cmd.AddCommand(SeedCmd)
	Cmd.AddCommand(PurgeCmd)
}

type TileChannelError struct {
	channel chan *slippy.Tile
	l       sync.RWMutex
	err     error
}

func (tc *TileChannelError) Channel() <-chan *slippy.Tile {
	if tc == nil {
		return nil
	}
	return tc.channel
}

func (tc *TileChannelError) Err() (e error) {
	if tc == nil {
		return nil
	}
	tc.l.RLock()
	e = tc.err
	tc.l.RUnlock()
	return e
}

func (tc *TileChannelError) setError(err error) {
	if tc == nil {
		return
	}
	tc.l.Lock()
	tc.err = err
	tc.l.Unlock()
}

type MapTile struct {
	MapName string
	Tile    *slippy.Tile
}

func doWork(ctx context.Context, tileChannelError *TileChannelError, maps []atlas.Map, concurrency int, worker func(context.Context, MapTile) error) (err error) {
	var wg sync.WaitGroup
	// new channel for the workers
	tiler := make(chan MapTile)
	var cleanup bool
	var errLock sync.RWMutex
	var tileMapErr error

	// Set up the workers

	wg.Add(concurrency)
	for i := 0; i < concurrency; i++ {
		go func() {
			var cleanup bool
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
					continue
				}
			}
			wg.Done()
		}()
	}

	// Run through the incoming tiles, and generate the tileMaps as needed.
	for tile := range tileChannelError.Channel() {
		for m := range maps {

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

			select {
			case tiler <- mapTile:
			case <-ctx.Done():
				cleanup = true
				break
			}
		}
	}

	close(tiler)
	if cleanup {
		// want to soak up any messages.
		for _ = range tileChannelError.Channel() {
			continue
		}
	}
	// Let our workers finish up.
	wg.Wait()
	if err = tileChannelError.Err(); err != nil {
		return err
	}
	// if we did not have an error from the tile generator
	// return any error the workers may have had
	return tileMapErr
}

func IsValidLat(f64 float64) bool { return -90.0 <= f64 && f64 <= 90.0 }
func IsValidLng(f64 float64) bool { return -180.0 <= f64 && f64 <= 180.0 }
func IsValidLatString(lat string) (float64, bool) {
	f64, err := strconv.ParseFloat(strings.TrimSpace(lat), 64)
	return f64, err != nil || !IsValidLat(f64)
}
func IsValidLngString(lng string) (float64, bool) {
	f64, err := strconv.ParseFloat(strings.TrimSpace(lng), 64)
	return f64, err != nil || !IsValidLng(f64)
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
