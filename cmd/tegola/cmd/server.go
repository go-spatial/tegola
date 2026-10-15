package cmd

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/alexeydott/tegola/atlas"
	"github.com/alexeydott/tegola/config"
	"github.com/alexeydott/tegola/internal/build"
	gdcmd "github.com/alexeydott/tegola/internal/cmd"
	"github.com/alexeydott/tegola/internal/log"
	"github.com/alexeydott/tegola/observability"
	"github.com/alexeydott/tegola/provider"
	"github.com/alexeydott/tegola/server"
	"github.com/go-spatial/cobra"
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
	RunE:    func(cmd *cobra.Command, args []string) error { return runServer(cmd, args, nil) },
}

func runServer(cmd *cobra.Command, args []string, rt *FeatureRuntime) error {
	gdcmd.New()
	defer gdcmd.Complete()
	gdcmd.OnComplete(provider.Cleanup)
	gdcmd.OnComplete(observability.Cleanup)
	// completion runs in reverse registration order: in-flight metatile
	// regeneration is canceled and drained after http.Server.Shutdown
	// (registered later via shutdown(srv)) and before observability and
	// provider cleanup, so no background render writes to a cache whose
	// provider is being torn down behind it
	gdcmd.OnComplete(server.ShutdownMetatileRegeneration)

	// Resolve the listen port. An explicitly-passed --port flag always
	// overrides the config value; when the flag is not set we fall back to the
	// config value, then to the flag default. (Comparing against the default
	// string is not enough because an explicit ":8080" is indistinguishable
	// from unset, so use the flag's IsSet semantics instead.)
	serverPort = resolveServerPort(cmd.Flags().Changed("port"), serverPort, string(conf.Webserver.Port))

	if conf.Webserver.HostName.Host != "" {
		u := url.URL(conf.Webserver.HostName)
		server.HostName = &u
	}

	// set our server version
	server.Version = build.Version
	server.TileHTTPMaxAge = int(conf.Webserver.TileHTTPMaxAge)
	build.Commands = append(build.Commands, cmd.Name())
	atlas.StartSubProcesses()

	// set user defined response headers
	for name, value := range conf.Webserver.Headers {
		// cast to string
		val := fmt.Sprintf("%v", value)
		// check that we have a value set
		if val == "" {
			log.Errorf("webserver.header (%v) has no configured value", val)
			os.Exit(1)
		}

		server.Headers[name] = val
	}

	if conf.Webserver.URIPrefix != "" {
		server.URIPrefix = string(conf.Webserver.URIPrefix)
	}

	if conf.Webserver.ProxyProtocol != "" {
		server.ProxyProtocol = string(conf.Webserver.ProxyProtocol)
	}

	// wire the privileged tile operations gate ([webserver.tile_operations]).
	// Disabled unless enabled = true is configured; zero rate/concurrent
	// values are resolved to the server-side defaults (server/tile_operations.go).
	server.TileOperations = configureTileOperations(conf.Webserver.TileOperations)

	if conf.Webserver.SSLCert+conf.Webserver.SSLKey != "" {
		if conf.Webserver.SSLCert == "" {
			// error
			log.Error("config must have both or nether ssl_key and ssl_cert, missing ssl_cert")
			os.Exit(1)
		}

		if conf.Webserver.SSLKey == "" {
			// error
			log.Error("config must have both or nether ssl_key and ssl_cert, missing ssl_key")
			os.Exit(1)
		}

		server.SSLCert = string(conf.Webserver.SSLCert)
		server.SSLKey = string(conf.Webserver.SSLKey)
	}

	// start our webserver
	opts := server.RouterOptions{}
	if rt != nil {
		opts.Features = rt.API
		opts.WFS = rt.WFS
	}
	srv, err := server.StartWithOptions(nil, serverPort, opts)
	if err != nil {
		return err
	}
	shutdown(srv)
	<-gdcmd.Cancelled()
	return nil
}

// resolveServerPort returns the port the HTTP server should bind to. An
// explicitly-set CLI --port (flagSet true) always overrides the config value;
// otherwise the config value is used when present, falling back to flagPort
// (the flag's value or its default).
func resolveServerPort(flagSet bool, flagPort, configPort string) string {
	if flagSet {
		return flagPort
	}
	if configPort != "" {
		return configPort
	}
	return flagPort
}

// configureTileOperations maps the [webserver.tile_operations] config section
// onto the server's tile operation gate. The feature stays disabled unless
// enabled = true is set. Zero rate/concurrent values are passed through as-is;
// the server resolves them to its defaults (60 requests/minute, 4 concurrent
// operations) — see server/tile_operations.go.
func configureTileOperations(t config.TileOperationsConfig) server.TileOperationsConfig {
	return server.TileOperationsConfig{
		Enabled:       t.Enabled,
		Token:         string(t.Token),
		RatePerMinute: int(t.RatePerMinute),
		MaxConcurrent: int(t.MaxConcurrent),
	}
}

func shutdown(srv *http.Server) {
	gdcmd.OnComplete(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel() // releases resources if slowOperation completes before timeout elapses
		_ = srv.Shutdown(ctx)
	})
}
