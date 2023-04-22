package cmd

import (
	"fmt"

	"github.com/go-spatial/cobra"
	"github.com/go-spatial/tegola/atlas"
	"github.com/go-spatial/tegola/cmd/internal/register"
	cachecmd "github.com/go-spatial/tegola/cmd/tegola/cmd/cache"
	"github.com/go-spatial/tegola/config"
	"github.com/go-spatial/tegola/dict"
	"github.com/go-spatial/tegola/internal/build"
	"github.com/go-spatial/tegola/internal/log"
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

	// init our providers
	// but first convert []env.Map -> []dict.Dicter
	provArr := make([]dict.Dicter, len(conf.Providers))
	for i := range provArr {
		provArr[i] = conf.Providers[i]
	}

	providers, err := register.Providers(provArr, conf.Maps)
	if err != nil {
		return fmt.Errorf("could not register providers: %v", err)
	}

	// init our maps
	if err = register.Maps(nil, conf.Maps, providers); err != nil {
		return fmt.Errorf("could not register maps: %v", err)
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
