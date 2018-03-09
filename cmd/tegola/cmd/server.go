package cmd

import (
	gdcmd "github.com/gdey/cmd"
	"github.com/go-spatial/tegola/provider"
	"github.com/go-spatial/tegola/server"
	"github.com/spf13/cobra"
)

var (
	serverPort      string
	defaultHTTPPort = ":8080"
)

var serverCmd = &cobra.Command{
	Use:   "serve",
	Short: "Use tegola as a tile server",
	Long:  `Use tegola as a vector tile server. Maps tiles will be served at /maps/:map_name/:z/:x/:y`,
	Run: func(cmd *cobra.Command, args []string) {
		gdcmd.New()
		initConfig()
		gdcmd.OnComplete(provider.Cleanup)

		//	check config for server port setting
		//	if you set the port via the comand line it will override the port setting in the config
		if serverPort == defaultHTTPPort && conf.Webserver.Port != "" {
			serverPort = conf.Webserver.Port
		}

		//	set our server version
		server.Version = Version
		server.HostName = conf.Webserver.HostName

		//	set the CORSAllowedOrigin if a value is provided
		if conf.Webserver.CORSAllowedOrigin != "" {
			server.CORSAllowedOrigin = conf.Webserver.CORSAllowedOrigin
		}

		//	set tile buffer
		if conf.TileBuffer > 0 {
			server.TileBuffer = float64(conf.TileBuffer)
		}

		//	start our webserver
		srv := server.Start(serverPort)
		shutdown(srv)
		<-gdcmd.Cancelled()
		gdcmd.Complete()

	},
}
