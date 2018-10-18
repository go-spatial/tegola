package cmd

import (
	"context"
	"log"
	"net/http"
	"time"

	gdcmd "github.com/go-spatial/tegola/internal/cmd"
	"github.com/go-spatial/tegola/provider"
	"github.com/go-spatial/tegola/server"
	"github.com/spf13/cobra"
)

var (
	serverPort                string
	defaultHTTPPort           = ":8080"
	defaultCORSAllowedOrigin  = "*"
	defaultCORSAllowedMethods = "GET, OPTIONS"
)

var serverCmd = &cobra.Command{
	Use:     "serve",
	Short:   "Use tegola as a tile server",
	Aliases: []string{"server"},
	Long:    `Use tegola as a vector tile server. Maps tiles will be served at /maps/:map_name/:z/:x/:y`,
	Run: func(cmd *cobra.Command, args []string) {
		gdcmd.New()
		initConfig()
		gdcmd.OnComplete(provider.Cleanup)

		// check config for server port setting
		// if you set the port via the comand line it will override the port setting in the config
		if serverPort == defaultHTTPPort && conf.Webserver.Port != "" {
			serverPort = string(conf.Webserver.Port)
		}

		// set our server version
		server.Version = Version
		server.HostName = string(conf.Webserver.HostName)

		// set the CORSAllowedOrigin to configured or default value
		header, err := conf.Webserver.Headers.String("Access-Control-Allow-Origin", &defaultCORSAllowedOrigin)
		if err != nil {
			log.Fatal(err)
		}
		server.CORSAllowedOrigin = header

		// set the CORSAllowedMethods to configured or default value
		header, err = conf.Webserver.Headers.String("Access-Control-Allow-Methods", &defaultCORSAllowedMethods)
		if err != nil {
			log.Fatal(err)
		}
		server.CORSAllowedMethods = header

		// set tile buffer
		if conf.TileBuffer != nil {
			server.TileBuffer = float64(*conf.TileBuffer)
		}

		// start our webserver
		srv := server.Start(nil, serverPort)
		shutdown(srv)
		<-gdcmd.Cancelled()
		gdcmd.Complete()

	},
}

func shutdown(srv *http.Server) {
	gdcmd.OnComplete(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel() // releases resources if slowOperation completes before timeout elapses
		srv.Shutdown(ctx)
	})
}
