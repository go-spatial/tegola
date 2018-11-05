package cmd

import (
	"fmt"

	"github.com/go-spatial/cobra"
	"github.com/go-spatial/tegola/atlas"
	"github.com/go-spatial/tegola/cmd/internal/register"
	cachecmd "github.com/go-spatial/tegola/cmd/tegola/cmd/cache"
	"github.com/go-spatial/tegola/config"
	"github.com/go-spatial/tegola/dict"
	"github.com/go-spatial/tegola/internal/log"
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
	cachecmd.Config = &conf
	RootCmd.AddCommand(cachecmd.Cmd)
	// version
	RootCmd.AddCommand(versionCmd)

}

var RootCmd = &cobra.Command{
	Use:   "tegola",
	Short: "tegola is a vector tile server",
	Long: fmt.Sprintf(`tegola is a vector tile server
Version: %v`, Version),
	PersistentPreRunE: rootCmdValidatePersistent,
}

func rootCmdValidatePersistent(cmd *cobra.Command, args []string) (err error) {
	return initConfig(configFile)
}

func initConfig(configFile string) (err error) {
	log.Infof("Loading config file: %v", configFile)
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

	providers, err := register.Providers(provArr)
	if err != nil {
		return fmt.Errorf("could not register providers: %v", err)
	}

	// init our maps
	if err = register.Maps(nil, conf.Maps, providers); err != nil {
		return fmt.Errorf("could not register maps: %v", err)
	}
	if len(conf.Cache) == 0 {
		return fmt.Errorf("No cache defined in config, please check your config (%v).", configFile)
	}
	// init cache backends
	cache, err := register.Cache(conf.Cache)
	if err != nil {
		return fmt.Errorf("could not register cache: %v", err)
	}
	if cache != nil {
		atlas.SetCache(cache)
	}
	return nil
}
