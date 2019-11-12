package cache

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-spatial/cobra" // The config from the main app
	"github.com/go-spatial/geom/slippy"
	"github.com/go-spatial/tegola/config"
	"github.com/go-spatial/tegola/internal/log"
)

// the config from the main app
var Config *config.Config
var RequireCache bool

var Cmd = &cobra.Command{
	Use:   "cache",
	Short: "command to manage the cache",
	Long:  "command to manage the cache",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		RequireCache = true
		if cmd.HasParent() {
			// run the parents Persistent Run commands.
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
	Cmd.AddCommand(SeedPurgeCmd)
	Cmd.SetUsageTemplate(`Usage: {{.CommandPath}} [command]{{if .HasExample}}

Examples:
{{.Example}}{{end}}

Available Commands:
  {{rpad "seed" .NamePadding}} seed tiles to the cache
  {{rpad "purge" .NamePadding}} purge tiles from the cache{{if .HasAvailableLocalFlags}}

Flags:
{{.LocalFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasAvailableInheritedFlags}}
Global Flags:
{{.InheritedFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasHelpSubCommands}}
Additional help topics:{{range .Commands}}{{if .IsAdditionalHelpTopicCommand}}
  {{rpad .CommandPath .CommandPathPadding}} {{.Short}}{{end}}{{end}}{{end}}{{if .HasAvailableSubCommands}}
Use "{{.CommandPath}} [command] --help" for more information about a command.{{end}}
`)
}

type TileChannel struct {
	channel  chan *MapTile
	cl       sync.RWMutex
	isClosed bool
	l        sync.RWMutex
	err      error
}

func (tc *TileChannel) Channel() <-chan *MapTile {
	if tc == nil {
		return nil
	}
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

func doWork(ctx context.Context, tileChannel *TileChannel, concurrency int, worker func(context.Context, MapTile) error) (err error) {
	var wg sync.WaitGroup
	// new channel for the workers
	tiler := make(chan MapTile)
	var cleanup bool
	var errLock sync.RWMutex
	var mapTileErr error

	// set up the workers
	wg.Add(concurrency)
	for i := 0; i < concurrency; i++ {
		go func(i int) {
			var cleanup bool
			// range our channel to listen for jobs
			for mt := range tiler {
				errLock.RLock()
				e := mapTileErr
				errLock.RUnlock()
				if e != nil {
					cleanup = true
					break
				}
				if err := worker(ctx, mt); err != nil {
					cleanup = true
					errLock.Lock()
					mapTileErr = err
					errLock.Unlock()
					break
				}
			}
			if cleanup {
				log.Debugf("worker %v waiting on clean up of tiler", i)
				for _ = range tiler {
					continue
				}
			}
			log.Debugf("worker %v done", i)
			wg.Done()
		}(i)
	}

	// run through the incoming tiles, and generate the mapTiles as needed.
TileChannelLoop:
	for tile := range tileChannel.Channel() {
		if ctx.Err() != nil {
			cleanup = true
			break
		}

		{ // worker error occured.
			errLock.RLock()
			e := mapTileErr
			errLock.RUnlock()
			if e != nil {
				cleanup = true
				break
			}
		}

		mapTile := MapTile{
			MapName: tile.MapName,
			Tile:    tile.Tile,
		}

		log.Debugf("seeding: %v", mapTile)

		select {
		case tiler <- mapTile:
		case <-ctx.Done():
			cleanup = true
			break TileChannelLoop
		}
	}

	close(tiler)
	if cleanup {
		// want to soak up any messages
		for _ = range tileChannel.Channel() {
			continue
		}
	}
	// let our workers finish up
	log.Info("waiting for workers to finish up")
	shouldExit := true
	go func() {
		<-time.After(60 * time.Second)
		if !shouldExit {
			log.Info("60 seconds passed killing")
			os.Exit(1)
		}
	}()
	wg.Wait()
	log.Info("all workers are done")
	shouldExit = false
	err = tileChannel.Err()
	if err == nil {
		err = mapTileErr
	}
	if err == context.Canceled {
		return nil
	}
	return err
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
