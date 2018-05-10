package cmd

import (
	"fmt"
	"html"
	"log"
	"runtime"
	"strings"

	"github.com/spf13/cobra"

	"github.com/go-spatial/geom"
	"github.com/go-spatial/tegola"
	"github.com/go-spatial/tegola/atlas"
	"github.com/go-spatial/tegola/cache"
	"github.com/go-spatial/tegola/config"
	"github.com/go-spatial/tegola/internal/dict/dict"
	"github.com/go-spatial/tegola/provider"
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
	providers, err := initProviders(provArr)
	if err != nil {
		log.Fatal(err)
	}

	// init our maps
	if err = initMaps(conf.Maps, providers); err != nil {
		log.Fatal(err)
	}

	if len(conf.Cache) != 0 {
		// init cache backends
		cache, err := initCache(conf.Cache)
		if err != nil {
			log.Fatal(err)
		}
		if cache != nil {
			atlas.SetCache(cache)
		}
	}
}

func initCache(config dict.Dicter) (cache.Interface, error) {
	// lookup our cache type
	cType, err := config.String("type", nil)
	if err != nil {
		return nil, fmt.Errorf("missing 'type' parameter for cache")
	}

	// register the provider
	return cache.For(cType, config)
}

// initMaps registers maps with our server
func initMaps(maps []config.Map, providers map[string]provider.Tiler) error {

	// iterate our maps
	for _, m := range maps {
		newMap := atlas.NewWebMercatorMap(string(m.Name))
		newMap.Attribution = html.EscapeString(string(m.Attribution))

		// convert from env package
		centerArr := [3]float64{}
		for i, v := range m.Center {
			centerArr[i] = float64(v)
		}

		newMap.Center = centerArr

		if len(m.Bounds) == 4 {
			newMap.Bounds = geom.NewExtent(
				[2]float64{float64(m.Bounds[0]), float64(m.Bounds[1])},
				[2]float64{float64(m.Bounds[2]), float64(m.Bounds[3])},
			)
		}

		// iterate our layers
		for _, l := range m.Layers {
			// split our provider name (provider.layer) into [provider,layer]
			providerLayer := strings.Split(string(l.ProviderLayer), ".")

			// we're expecting two params in the provider layer definition
			if len(providerLayer) != 2 {
				return fmt.Errorf("invalid provider layer (%v) for map (%v)", l.ProviderLayer, m)
			}

			// lookup our proivder
			provider, ok := providers[providerLayer[0]]
			if !ok {
				return fmt.Errorf("provider (%v) not defined", providerLayer[0])
			}

			// read the provider's layer names
			layerInfos, err := provider.Layers()
			if err != nil {
				return fmt.Errorf("error fetching layer info from provider (%v)", providerLayer[0])
			}

			// confirm our providerLayer name is registered
			var found bool
			var layerGeomType tegola.Geometry
			for i := range layerInfos {
				if layerInfos[i].Name() == providerLayer[1] {
					found = true

					// read the layerGeomType
					layerGeomType = layerInfos[i].GeomType()
				}
			}
			if !found {
				return fmt.Errorf("map (%v) 'provider_layer' (%v) is not registered with provider (%v)", m.Name, l.ProviderLayer, providerLayer[1])
			}

			var defaultTags map[string]interface{}
			if l.DefaultTags != nil {
				var ok bool
				defaultTags, ok = l.DefaultTags.(map[string]interface{})
				if !ok {
					return fmt.Errorf("'default_tags' for 'provider_layer' (%v) should be a TOML table", l.ProviderLayer)
				}
			}

			// add our layer to our layers slice
			newMap.Layers = append(newMap.Layers, atlas.Layer{
				Name:              string(l.Name),
				ProviderLayerName: providerLayer[1],
				MinZoom:           uint(*l.MinZoom),
				MaxZoom:           uint(*l.MaxZoom),
				Provider:          provider,
				DefaultTags:       defaultTags,
				GeomType:          layerGeomType,
				DontSimplify:      bool(l.DontSimplify),
			})
		}

		// register map
		atlas.AddMap(newMap)
	}

	return nil
}

func initProviders(providers []dict.Dicter) (map[string]provider.Tiler, error) {
	var err error

	// holder for registered providers
	registeredProviders := map[string]provider.Tiler{}

	// iterate providers
	for _, p := range providers {
		// lookup our proivder name
		pname, err := p.String("name", nil)
		if err != nil {
			return registeredProviders, err
		}

		// check if a proivder with this name is alrady registered
		_, ok := registeredProviders[pname]
		if ok {
			return registeredProviders, fmt.Errorf("provider (%v) already registered!", pname)
		}

		// lookup our provider type
		ptype, err := p.String("type", nil)
		if err != nil {
			return registeredProviders, err
		}

		// register the provider
		prov, err := provider.For(ptype, p)
		if err != nil {
			return registeredProviders, err
		}

		// add the provider to our map of registered providers
		registeredProviders[pname] = prov
	}

	return registeredProviders, err
}
