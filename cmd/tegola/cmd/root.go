package cmd

import (
	"fmt"
	"log"
	"runtime"

	"github.com/spf13/cobra"

	"github.com/go-spatial/tegola/atlas"
	"github.com/go-spatial/tegola/cmd/internal/register"
	"github.com/go-spatial/tegola/config"
	"github.com/go-spatial/tegola/internal/dict"
)

var (
	configFile string
	// set at build time via the CI
	Version = "version not set"
	// parsed config
	conf config.Config
)

func init() {
	// root
	RootCmd.PersistentFlags().StringVar(&configFile, "config", "config.toml", "path to config file")

	// server
	serverCmd.Flags().StringVarP(&serverPort, "port", "p", ":8080", "port to bind tile server to")
	RootCmd.AddCommand(serverCmd)

	// cache seed / purge
	cacheCmd.Flags().StringVarP(&cacheMap, "map", "", "", "map name as defined in the config")
	cacheCmd.Flags().StringVarP(&cacheZXY, "zxy", "", "", "tile in z/x/y format")
	cacheCmd.Flags().UintVarP(&cacheMinZoom, "minzoom", "", 0, "min zoom to seed cache from")
	cacheCmd.Flags().UintVarP(&cacheMaxZoom, "maxzoom", "", 0, "max zoom to seed cache to")
	cacheCmd.Flags().StringVarP(&cacheBounds, "bounds", "", "-180,-85.0511,180,85.0511", "lat / long bounds to seed the cache with in the format: minx, miny, maxx, maxy")
	cacheCmd.Flags().IntVarP(&cacheConcurrency, "concurrency", "", runtime.NumCPU(), "the amount of concurrency to use. defaults to the number of CPUs on the machine")
	cacheCmd.Flags().BoolVarP(&cacheOverwrite, "overwrite", "", false, "overwrite the cache if a tile already exists")

	RootCmd.AddCommand(cacheCmd)

	// version
	RootCmd.AddCommand(versionCmd)
}

var RootCmd = &cobra.Command{
	Use:   "tegola",
	Short: "tegola is a vector tile server",
	Long: fmt.Sprintf(`tegola is a vector tile server
Version: %v`, Version),
}

func initConfig() {
	var err error

	conf, err = config.Load(configFile)
	if err != nil {
		log.Fatal(err)
	}

	// validate our config
	if err = conf.Validate(); err != nil {
		log.Fatal(err)
	}

	// init our providers
	// but first convert []env.Map -> []dict.Dicter
	provArr := make([]dict.Dicter, len(conf.Providers))
	for i := range provArr {
		provArr[i] = conf.Providers[i]
	}

	providers, err := register.Providers(provArr)
	if err != nil {
		log.Fatal(err)
	}

	// init our maps
	if err = register.Maps(nil, conf.Maps, providers); err != nil {
		log.Fatal(err)
	}

	if len(conf.Cache) != 0 {
		// init cache backends
		cache, err := register.Cache(conf.Cache)
		if err != nil {
			log.Fatal(err)
		}
		if cache != nil {
			atlas.SetCache(cache)
		}
	}
}
