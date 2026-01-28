// Package server implements the http frontend
package server

import (
	"net/http"
	"net/url"
	"os"

	"github.com/dimfeld/httptreemux"

	"github.com/go-spatial/tegola/atlas"
	"github.com/go-spatial/tegola/internal/build"
	"github.com/go-spatial/tegola/internal/log"
	"github.com/go-spatial/tegola/observability"
)

const (
	// MaxTileSize is 500k. Currently, just throws a warning when tile
	// is larger than MaxTileSize
	MaxTileSize = 500000

	// QueryKeyDebug is a common query string key used throughout the pacakge
	// the value should always be a boolean
	QueryKeyDebug = "debug"
)

var (
	// Version is the version of the software, this should be set by the main program, before starting up.
	// It is used by various Middleware to determine the version.
	Version string = "version not set"

	// HostName is the name of the host to use for construction of URLS.
	// configurable via the tegola config.toml file (set in main.go)
	HostName *url.URL

	// Port is the port the server is listening on, used for construction of URLS.
	// configurable via the tegola config.toml file (set in main.go)
	Port string

	// SSLCert is a filepath to an SSL cert, this will be used to enable https
	SSLCert string

	// SSLKey is a filepath to an SSL key, this will be used to enable https
	SSLKey string

	// Headers is the map of user defined response headers.
	// configurable via the tegola config.toml file (set in main.go)
	Headers = map[string]string{}

	// URIPrefix sets a prefix on all server endpoints. This is often used
	// when the server sits behind a reverse proxy with a prefix (i.e. /tegola)
	URIPrefix = "/"

	// ProxyProtocol is a custom protocol that will be used to generate the URLs
	// included in the capabilities endpoint responses. This is useful when he
	// server sits behind a reverse proxy
	// (See https://github.com/go-spatial/tegola/pull/967)
	ProxyProtocol string

	// DefaultCORSHeaders define the default CORS response headers added to all requests
	DefaultCORSHeaders = map[string]string{
		"Access-Control-Allow-Origin":  "*",
		"Access-Control-Allow-Methods": "GET, OPTIONS",
	}
)

// NewRouter set's up our routes.
func NewRouter(a *atlas.Atlas) *httptreemux.TreeMux {
	o := a.Observer()
	r := httptreemux.New()
	group := r.NewGroup(URIPrefix)

	// one handler to respond to all OPTIONS requests for registered routes with our CORS headers
	r.OptionsHandler = corsHandler

	if o != nil && o != observability.NullObserver {
		const (
			metricsRoute = "/metrics"
		)
		if h := o.Handler(metricsRoute); h != nil {
			// Only set up the /metrics endpoint if we have a configured observer
			log.Infof("setting up observer: %v", o.Name())
			group.UsingContext().Handler(http.MethodGet, metricsRoute, h)
		}
	}

	// capabilities endpoints
	group.UsingContext().
		Handler(observability.InstrumentAPIHandler(http.MethodGet, "/capabilities", o, HeadersHandler(HandleCapabilities{})))
	group.UsingContext().
		Handler(observability.InstrumentAPIHandler(http.MethodGet, "/capabilities/:map_name", o, HeadersHandler(HandleMapCapabilities{Atlas: a})))

	// map tiles
	hMapLayerZXY := HandleMapLayerZXY{Atlas: a}
	group.UsingContext().
		Handler(observability.InstrumentAPIHandler(http.MethodGet, "/maps/:map_name/:z/:x/:y", o, HeadersHandler(GZipHandler(TileCacheHandler(a, hMapLayerZXY)))))
	group.UsingContext().
		Handler(observability.InstrumentAPIHandler(http.MethodGet, "/maps/:map_name/:layer_name/:z/:x/:y", o, HeadersHandler(GZipHandler(TileCacheHandler(a, hMapLayerZXY)))))

	// map style
	group.UsingContext().
		Handler(observability.InstrumentAPIHandler(http.MethodGet, "/maps/:map_name/style.json", o, HeadersHandler(HandleMapStyle{})))

	// setup viewer routes, which can be excluded via build flags
	setupViewer(o, group)

	return r
}

// Start starts the tile server binding to the provided port
func Start(a *atlas.Atlas, port string) *http.Server {
	// notify the user the server is starting
	log.Infof("starting tegola server (%v) on port %v", build.Version, port)

	srv := &http.Server{Addr: port, Handler: NewRouter(a)}

	// start our server
	go func() {
		var err error

		if SSLCert+SSLKey != "" {
			err = srv.ListenAndServeTLS(SSLCert, SSLKey)
		} else {
			err = srv.ListenAndServe()
		}

		switch err {
		case nil:
			// noop
			return
		case http.ErrServerClosed:
			log.Info("http server closed")
			return
		default:
			log.Error(err)
			os.Exit(1)
			return
		}
	}()

	return srv
}

// hostName determines whether to use an user defined HostName
// or the host from the incoming request
func hostName(r *http.Request) *url.URL {
	// if the HostName has been configured, don't mutate it
	if HostName != nil {
		return HostName
	}

	// favor the r.URL.Host attribute in case tegola is behind a proxy
	// https://stackoverflow.com/questions/42921567/what-is-the-difference-between-host-and-url-host-for-golang-http-request
	if r.URL != nil && r.URL.Host != "" {
		return r.URL
	}

	return &url.URL{
		Host: r.Host,
	}
}

const (
	HeaderXForwardedProto = "X-Forwarded-Proto"
)

// various checks to determine if the request is http or https. the scheme is needed for the TileURLs
// r.URL.Scheme can be empty if a relative request is issued from the client. (i.e. GET /foo.html)
func scheme(r *http.Request) string {
	if ProxyProtocol != "" {
		return ProxyProtocol
	}

	if r.Header.Get(HeaderXForwardedProto) != "" {
		return r.Header.Get(HeaderXForwardedProto)
	}

	if r.TLS != nil {
		return "https"
	}

	return "http"
}

// URLRoot builds a string containing the scheme, host and port based on a combination of user defined values,
// headers and request parameters. The function is public so it can be overridden for other implementations.
var URLRoot = func(r *http.Request) *url.URL {
	return &url.URL{
		Scheme: scheme(r),
		Host:   hostName(r).Host,
	}
}

// corsHandler is used to respond to all OPTIONS requests for registered routes
func corsHandler(w http.ResponseWriter, _ *http.Request, _ map[string]string) {
	setHeaders(w)
}

// setHeaders sets default headers and user defined headers
func setHeaders(w http.ResponseWriter) {
	// add our default CORS headers
	for name, val := range DefaultCORSHeaders {
		if val == "" {
			log.Warnf("default CORS header (%s) has no value", name)
		}

		w.Header().Set(name, val)
	}

	// set user defined headers
	for name, val := range Headers {
		if val == "" {
			log.Warnf("header (%s) has no value", name)
		}

		w.Header().Set(name, val)
	}
}
