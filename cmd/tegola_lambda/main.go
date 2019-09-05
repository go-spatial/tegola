package main

import (
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"

	"github.com/akrylysov/algnhsa"
	"github.com/dimfeld/httptreemux"

	"github.com/go-spatial/geom/encoding/mvt"
	"github.com/go-spatial/tegola/atlas"
	"github.com/go-spatial/tegola/cmd/internal/register"
	"github.com/go-spatial/tegola/config"
	"github.com/go-spatial/tegola/dict"
	"github.com/go-spatial/tegola/server"
)

var (
	// Version is at build time via the CI
	Version = "version not set"
	// mux is a reference to the http muxer. it's stored as a package
	// var so we can take advantage of Lambda's "Global State".
	mux *httptreemux.TreeMux
)

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
	server.URLRoot = URLRoot

	confLocation := DefaultConfLocation

	// check if the env TEGOLA_CONFIG is set
	if os.Getenv("TEGOLA_CONFIG") != "" {
		confLocation = os.Getenv("TEGOLA_CONFIG")
	}

	// read our config
	conf, err := config.Load(confLocation)
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

	// register the providers
	providers, err := register.Providers(provArr)
	if err != nil {
		log.Fatal(err)
	}

	// register the maps
	if err = register.Maps(nil, conf.Maps, providers); err != nil {
		log.Fatal(err)
	}

	// check if a cache backend is provided
	if len(conf.Cache) != 0 {
		// register the cache backend
		cache, err := register.Cache(conf.Cache)
		if err != nil {
			log.Fatal(err)
		}
		if cache != nil {
			atlas.SetCache(cache)
		}
	}

	// set our server version
	server.Version = Version
	if conf.Webserver.HostName != "" {
		server.HostName = string(conf.Webserver.HostName)
	}

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

	// http route setup
	mux = server.NewRouter(nil)
}

func main() {
	// the second argument here tells algnhasa to watch for the MVT MimeType Content-Type headers
	// if it detects this in the response the payload will be base64 encoded. Lambda needs to be configured
	// to handle binary responses so it can convert the base64 encoded payload back into binary prior
	// to sending to the client
	algnhsa.ListenAndServe(mux, &algnhsa.Options{
		BinaryContentTypes: []string{mvt.MimeType},
		UseProxyPath:       true,
	})
}

// URLRoot overrides the default server.URLRoot function in order to include the "stage" part of the root
// that is part of lambda's URL scheme
func URLRoot(r *http.Request) *url.URL {
	u := url.URL{
		Scheme: scheme(r),
		Host:   r.Header.Get("Host"),
	}

	// read the request context to pull out the lambda "stage" so it can be prepended to the URL Path
	if ctx, ok := algnhsa.ProxyRequestFromContext(r.Context()); ok {
		u.Path = ctx.RequestContext.Stage
	}

	return &u
}

// various checks to determine if the request is http or https. the scheme is needed for the TileJSON URLs
// r.URL.Scheme can be empty if a relative request is issued from the client. (i.e. GET /foo.html)
func scheme(r *http.Request) string {
	if r.Header.Get("X-Forwarded-Proto") != "" {
		return r.Header.Get("X-Forwarded-Proto")
	} else if r.TLS != nil {
		return "https"
	}

	return "http"
}
