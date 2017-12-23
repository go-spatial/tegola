package cmd

import (
	gdcmd "github.com/gdey/cmd"
	"github.com/go-spatial/tegola/provider"
	"github.com/go-spatial/tegola/server"
	"github.com/spf13/cobra"
)

var (
	defaultPort     = 8080
	defaultHostName = ""
	port            int
	hostName        string

	bindAddress        string
	defaultBindAddress = ":8080"
)

var serverCmd = &cobra.Command{
	Use:   "serve",
	Short: "Use tegola as a tile server",
	Long:  `Use tegola as a vector tile server. Maps tiles will be served at /maps/:map_name/:z/:x/:y`,
	Run: func(cmd *cobra.Command, args []string) {
		gdcmd.New()
		initConfig()
		gdcmd.OnComplete(provider.Cleanup)

		port = defaultPort
		if conf.Webserver.Port != 0 {
			port = conf.Webserver.Port
		}

		hostName = defaultHostName
		if conf.Webserver.HostName != "" {
			hostName = conf.Webserver.HostName
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
		srv := server.Start(hostName, port)
		shutdown(srv)
		<-gdcmd.Cancelled()
		gdcmd.Complete()
	},
}
