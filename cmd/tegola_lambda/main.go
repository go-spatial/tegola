package main

import (
	"fmt"
	"net/http"
	"net/url"
	"os"
	"time"

	awslambda "github.com/aws/aws-lambda-go/lambda"

	"github.com/alexeydott/tegola/atlas"
	"github.com/alexeydott/tegola/cmd/internal/register"
	"github.com/alexeydott/tegola/config"
	"github.com/alexeydott/tegola/dict"
	"github.com/alexeydott/tegola/internal/build"
	"github.com/alexeydott/tegola/internal/log"
	"github.com/alexeydott/tegola/server"
	lambdaserver "github.com/alexeydott/tegola/server/lambda"
)

// mux is a reference to the http muxer. it's stored as a package
// var so we can take advantage of Lambda's "Global State".
var mux http.Handler

const DefaultConfLocation = "config.toml"

// instantiate the server during the init() function and then store
// the muxer in a package variable. This allows us to take advantage
// of "Global State" to avoid needing to re-parse the config, connect
// to databases, tile caches, etc. on each function invocation.
//
// For more info, see Using Global State:
// https://docs.aws.amazon.com/lambda/latest/dg/go-programming-model-handler-types.html
func init() {
	var err error

	// override the URLRoot func with a lambda specific one
	server.URLRoot = lambdaserver.URLRoot

	confLocation := DefaultConfLocation

	// check if the env TEGOLA_CONFIG is set
	if os.Getenv("TEGOLA_CONFIG") != "" {
		confLocation = os.Getenv("TEGOLA_CONFIG")
	}

	// read our config
	conf, err := config.Load(confLocation)
	if err != nil {
		log.Error(err)
		os.Exit(1)
	}

	// validate our config
	if err = conf.Validate(); err != nil {
		log.Error(err)
		os.Exit(1)
	}

	// init our providers
	// but first convert []env.Map -> []dict.Dicter
	provArr := make([]dict.Dicter, len(conf.Providers))
	for i := range provArr {
		provArr[i] = conf.Providers[i]
	}

	// register the providers
	providers, err := register.Providers(provArr, nil)
	if err != nil {
		log.Error(err)
		os.Exit(1)
	}

	// register the maps
	if err = register.Maps(nil, conf.Maps, providers); err != nil {
		log.Error(err)
		os.Exit(1)
	}

	// check if a cache backend is provided
	if len(conf.Cache) != 0 {
		// register the cache backend
		cache, err := register.Cache(conf.Cache)
		if err != nil {
			log.Error(err)
			os.Exit(1)
		}
		if cache != nil {
			atlas.SetCache(cache)
		}
	}

	// set our server version
	server.Version = build.Version
	if conf.Webserver.HostName.Host != "" {
		u := url.URL(conf.Webserver.HostName)
		server.HostName = &u
	}

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

	// Bind the same explicit publications as the CLI before assembling routes.
	service, err := register.Features(conf.Features, providers)
	if err != nil {
		log.Error(err)
		os.Exit(1)
	}
	options := server.RouterOptions{}
	if service != nil {
		settings := conf.Features.Resolved()
		options.Features, err = server.NewFeatureAPI(service, server.FeatureAPIConfig{
			BasePath:         string(settings.BasePath),
			DefaultLimit:     uint(*settings.DefaultLimit),
			MaxLimit:         uint(*settings.MaxLimit),
			MaxResponseBytes: int64(*settings.MaxResponseBytes),
			QueryTimeout:     time.Duration(*settings.QueryTimeoutMS) * time.Millisecond,
			Title:            string(settings.Title),
			Description:      string(settings.Description),
		})
		if err != nil {
			log.Error(err)
			os.Exit(1)
		}
	}
	// http route setup
	mux, err = server.NewRouterWithOptions(nil, options)
	if err != nil {
		log.Error(err)
		os.Exit(1)
	}
}

func main() {
	build.Commands = []string{"lambda"}
	handler, err := lambdaserver.New(mux, lambdaserver.Options{
		PublicURL:    server.HostName,
		UseProxyPath: true,
	})
	if err != nil {
		log.Error(err)
		os.Exit(1)
	}
	awslambda.StartHandler(handler)
}
