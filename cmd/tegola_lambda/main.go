package main

import (
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"

	"github.com/arolek/algnhsa"
	"github.com/go-spatial/tegola/atlas"
	"github.com/go-spatial/tegola/cmd/internal/register"
	"github.com/go-spatial/tegola/config"
	"github.com/go-spatial/tegola/dict"
	"github.com/go-spatial/tegola/mvt"
	"github.com/go-spatial/tegola/server"
)

// set at build time via the CI
var Version = "version not set"

func init() {
	// override the URLRoot func with a lambda specific one
	server.URLRoot = URLRoot
}

func main() {
	var err error

	confLocation := "config.toml"

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
	// note that we are sending the whole config file to include both maps and providers
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

	// http route setup
	mux := server.NewRouter(nil)

	// the second argument here tells algnhasa to watch for the MVT MimeType Content-Type headers
	// if it detects this in the response the payload will be base64 encoded. Lambda needs to be configured
	// to handle binary responses so it can convert the base64 encoded payload back into binary prior
	// to sending to the client
	algnhsa.ListenAndServe(mux, []string{mvt.MimeType})
}

// URLRoot overrides the default server.URLRoot function in order to include the "stage" part of the root
// that is part of lambda's URL scheme
func URLRoot(r *http.Request) string {
	u := url.URL{
		Scheme: scheme(r),
		Host:   r.Header.Get("Host"),
	}

	// read the request context to pull out the lambda "stage" so it can be prepended to the URL Path
	if ctx, ok := algnhsa.ProxyRequestFromContext(r.Context()); ok {
		u.Path = ctx.RequestContext.Stage
	}

	return u.String()
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
