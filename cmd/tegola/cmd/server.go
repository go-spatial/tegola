package cmd

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/go-spatial/cobra"
	"github.com/go-spatial/tegola/atlas"
	"github.com/go-spatial/tegola/internal/build"
	gdcmd "github.com/go-spatial/tegola/internal/cmd"
	"github.com/go-spatial/tegola/internal/log"
	"github.com/go-spatial/tegola/observability"
	"github.com/go-spatial/tegola/provider"
	"github.com/go-spatial/tegola/server"
)

var (
	serverPort      string
	serverNoCache   bool
	defaultHTTPPort = ":8080"
)

var serverCmd = &cobra.Command{
	Use:     "serve",
	Short:   "Use tegola as a tile server",
	Aliases: []string{"server"},
	Long:    `Use tegola as a vector tile server. Maps tiles will be served at /maps/:map_name/:z/:x/:y`,
	Run: func(cmd *cobra.Command, args []string) {
		gdcmd.New()
		gdcmd.OnComplete(provider.Cleanup)
		gdcmd.OnComplete(observability.Cleanup)

		// check config for server port setting
		// if you set the port via the command line it will override the port setting in the config
		if serverPort == defaultHTTPPort && conf.Webserver.Port != "" {
			serverPort = string(conf.Webserver.Port)
		}

		// set our server version
		server.Version = build.Version
		server.HostName = string(conf.Webserver.HostName)
		build.Commands = append(build.Commands, cmd.Name())
		atlas.StartSubProcesses()

		// set user defined response headers
		for name, value := range conf.Webserver.Headers {
			// cast to string
			val := fmt.Sprintf("%v", value)
			// check that we have a value set
			if val == "" {
				log.Fatalf("webserver.header (%v) has no configured value", val)
			}

			server.Headers[name] = val
		}

		if conf.Webserver.URIPrefix != "" {
			server.URIPrefix = string(conf.Webserver.URIPrefix)
		}

		if conf.Webserver.ProxyProtocol != "" {
			server.ProxyProtocol = string(conf.Webserver.ProxyProtocol)
		}

		if conf.Webserver.SSLCert+conf.Webserver.SSLKey != "" {
			if conf.Webserver.SSLCert == "" {
				// error
				log.Fatal("config must have both or nether ssl_key and ssl_cert, missing ssl_cert")
			}

			if conf.Webserver.SSLKey == "" {
				// error
				log.Fatal("config must have both or nether ssl_key and ssl_cert, missing ssl_key")
			}

			server.SSLCert = string(conf.Webserver.SSLCert)
			server.SSLKey = string(conf.Webserver.SSLKey)
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
