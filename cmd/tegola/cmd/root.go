package cmd

import (
	"context"
	"fmt"

	"github.com/go-spatial/cobra"
	"github.com/go-spatial/tegola/atlas"
	"github.com/go-spatial/tegola/cmd/internal/register"
	cachecmd "github.com/go-spatial/tegola/cmd/tegola/cmd/cache"
	"github.com/go-spatial/tegola/config"
	"github.com/go-spatial/tegola/config/source"
	"github.com/go-spatial/tegola/dict"
	"github.com/go-spatial/tegola/internal/build"
	"github.com/go-spatial/tegola/internal/env"
	"github.com/go-spatial/tegola/internal/log"
	"github.com/go-spatial/tegola/provider"
)

var (
	logLevel   string
	configFile string
	logger     string
	// parsed config
	conf config.Config

	// RequireCache in this instance
	RequireCache bool
)

func validateSupportedLoggers(logger string) error {
	switch logger {
	case log.STANDARD:
		return nil
	case log.ZAP:
		return nil
	default:
		return fmt.Errorf("invalid logger %s", logger)
	}
}

func getLogLevelFromString(level string) (log.Level, error) {
	switch level {
	case "TRACE":
		return log.TRACE, nil
	case "DEBUG":
		return log.DEBUG, nil
	case "INFO":
		return log.INFO, nil
	case "WARN":
		return log.WARN, nil
	case "ERROR":
		return log.ERROR, nil
	default:
		return 0, fmt.Errorf("invalid log level use")
	}
}

func init() {
	// root
	RootCmd.PersistentFlags().StringVar(&configFile, "config", "config.toml",
		"path or http url to a config file, or \"-\" for stdin")
	RootCmd.PersistentFlags().StringVar(&logLevel, "log-level", "INFO",
		"set log level to: TRACE, DEBUG, INFO, WARN or ERROR")
	RootCmd.PersistentFlags().StringVar(&logger, "logger", log.STANDARD,
		"set logger to: standard, zap - default: standard")

	// server
	serverCmd.Flags().StringVarP(&serverPort, "port", "p", ":8080", "port to bind tile server to")
	serverCmd.Flags().BoolVarP(&serverNoCache, "no-cache", "n", false, "turn off the cache")

	RootCmd.AddCommand(serverCmd)
	// cache seed / purge
	cachecmd.Config = &conf
	RootCmd.AddCommand(cachecmd.Cmd)
	// version
	RootCmd.AddCommand(versionCmd)
}

var RootCmd = &cobra.Command{
	Use:   "tegola",
	Short: "tegola is a vector tile server",
	Long: fmt.Sprintf(`tegola is a vector tile server
Version: %v`, build.Version),
	PersistentPreRunE: rootCmdValidatePersistent,
}

func rootCmdValidatePersistent(cmd *cobra.Command, _ []string) (err error) {
	requireCache := RequireCache || cachecmd.RequireCache
	cmdName := cmd.CalledAs()
	switch cmdName {
	case "help", "version":
		build.Commands = append(build.Commands, cmdName)
		return nil
	default:
		return initConfig(configFile, requireCache, logLevel, logger)
	}
}

func initConfig(configFile string, cacheRequired bool, logLevel string, logger string) (err error) {
	err = validateSupportedLoggers(logger)
	if err != nil {
		return err
	}
	log.SetLogger(logger)

	// set log level before the first log is called
	level, err := getLogLevelFromString(logLevel)
	if err != nil {
		return err
	}
	log.SetLogLevel(level)

	if conf, err = config.Load(configFile); err != nil {
		return err
	}
	if err = conf.Validate(); err != nil {
		return err
	}

	// Init providers from the primary config file.
	providers, err := initProviders(conf.Providers, conf.Maps, "default")
	if err != nil {
		return err
	}

	// Init maps from the primary config file.
	if err = initMaps(conf.Maps, providers); err != nil {
		return err
	}

	// Setup the app config source.
	ctx := context.Background()
	if err = initAppConfigSource(ctx, conf); err != nil {
		return err
	}

	if len(conf.Cache) == 0 && cacheRequired {
		return fmt.Errorf("no cache defined in config, please check your config (%v)", configFile)
	}
	if serverNoCache {
		log.Info("Cache explicitly turned off by user via command line")
	} else if len(conf.Cache) > 0 {
		// init cache backends
		cache, err := register.Cache(conf.Cache)
		if err != nil {
			return fmt.Errorf("could not register cache: %v", err)
		}
		if cache != nil {
			atlas.SetCache(cache)
		}
	}
	observer, err := register.Observer(conf.Observer)
	if err != nil {
		return err
	}
	atlas.SetObservability(observer)
	return nil
}

// initProviders translate provider config from a TOML file into usable Provider objects.
func initProviders(providersConfig []env.Dict, maps []provider.Map, namespace string) (map[string]provider.TilerUnion, error) {
	// first convert []env.Map -> []dict.Dicter
	provArr := make([]dict.Dicter, len(providersConfig))
	for i := range provArr {
		provArr[i] = providersConfig[i]
	}

	providers, err := register.Providers(provArr, conf.Maps, namespace)
	if err != nil {
		return nil, fmt.Errorf("could not register providers: %v", err)
	}

	return providers, nil
}

// initMaps registers maps with Atlas to be ready for service.
func initMaps(maps []provider.Map, providers map[string]provider.TilerUnion) error {
	if err := register.Maps(nil, maps, providers); err != nil {
		return fmt.Errorf("could not register maps: %v", err)
	}

	return nil
}

// initAppConfigSource sets up an additional configuration source for "apps" (groups of providers and maps) to be loaded and unloaded on-the-fly.
func initAppConfigSource(ctx context.Context, conf config.Config) error {
	// Get the config source type. If none, return.
	val, err := conf.AppConfigSource.String("type", nil)
	if err != nil || val == "" {
		return nil
	}

	// Initialize the source.
	src, err := source.InitSource(val, conf.AppConfigSource, conf.BaseDir)
	if err != nil {
		return err
	}

	// Load and start watching for new apps.
	watcher, err := src.LoadAndWatch(ctx)
	if err != nil {
		return err
	}

	go watchAppUpdates(ctx, watcher)

	return nil
}

// watchAppUpdates will pull from the channels supplied by the given watcher to process new app config.
func watchAppUpdates(ctx context.Context, watcher source.ConfigWatcher) {
	// Keep a record of what we've loaded so that we can unload when needed.
	apps := make(map[string]source.App)

	for {
		select {
		case app, ok := <-watcher.Updates:
			if !ok {
				return
			}

			// Check for validity first.
			if err := config.ValidateApp(&app); err != nil {
				log.Errorf("Failed validating app %s. %s", app.Key, err)
				continue
			}

			// If the new app is named the same as an existing app, first unload the existing one.
			if old, exists := apps[app.Key]; exists {
				log.Infof("Unloading app %s...", old.Key)
				register.UnloadMaps(nil, getMapNames(old))
				register.UnloadProviders(getProviderNames(old), old.Key)
				delete(apps, old.Key)
			}

			log.Infof("Loading app %s...", app.Key)

			// Init new providers
			providers, err := initProviders(app.Providers, app.Maps, app.Key)
			if err != nil {
				log.Errorf("Failed initializing providers from %s: %s", app.Key, err)
				continue
			}

			// Init new maps
			if err = initMaps(app.Maps, providers); err != nil {
				log.Errorf("Failed initializing maps from %s: %s", app.Key, err)
				continue
			}

			// Record that we've loaded this app.
			apps[app.Key] = app

		case deleted, ok := <-watcher.Deletions:
			if !ok {
				return
			}

			// Unload an app's maps if it was previously loaded.
			if app, exists := apps[deleted]; exists {
				log.Infof("Unloading app %s...", app.Key)
				register.UnloadMaps(nil, getMapNames(app))
				register.UnloadProviders(getProviderNames(app), app.Key)
				delete(apps, app.Key)
			} else {
				log.Infof("Received an unload event for app %s, but couldn't find it.", deleted)
			}

		case <-ctx.Done():
			return
		}
	}
}

func getMapNames(app source.App) []string {
	names := make([]string, 0, len(app.Maps))
	for _, m := range app.Maps {
		names = append(names, string(m.Name))
	}

	return names
}

func getProviderNames(app source.App) []string {
	names := make([]string, 0, len(app.Providers))
	for _, p := range app.Providers {
		name, err := p.String("name", nil)
		if err != nil {
			log.Warnf("Encountered a provider in app %s with an empty name.", app.Key)
			continue
		}
		names = append(names, name)
	}

	return names
}
