// Package server implements the http frontend
package server

import (
	"fmt"
	"net/http"
	"net/url"
	"path"

	"github.com/dimfeld/httptreemux"

	"github.com/go-spatial/tegola/atlas"
	"github.com/go-spatial/tegola/internal/log"
)

const (
	// MaxTileSize is 500k. Currently just throws a warning when tile
	// is larger than MaxTileSize
	MaxTileSize = 500000
)

var (
	// Version is the version of the software, this should be set by the main program, before starting up.
	// It is used by various Middleware to determine the version.
	Version string = "Version Not Set"

	// HostName is the name of the host to use for construction of URLS.
	// configurable via the tegola config.toml file (set in main.go)
	HostName string

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

	// DefaultCORSHeaders define the default CORS response headers added to all requests
	DefaultCORSHeaders = map[string]string{
		"Access-Control-Allow-Origin":  "*",
		"Access-Control-Allow-Methods": "GET, OPTIONS",
	}
)

// NewRouter set's up the our routes.
func NewRouter(a *atlas.Atlas) *httptreemux.TreeMux {
	r := httptreemux.New()
	group := r.NewGroup(URIPrefix)

	// one handler to respond to all OPTIONS requests for registered routes with our CORS headers
	r.OptionsHandler = corsHandler

	// capabilities endpoints
	group.UsingContext().Handler("GET", "/capabilities", HeadersHandler(HandleCapabilities{}))
	group.UsingContext().Handler("GET", "/capabilities/:map_name", HeadersHandler(HandleMapCapabilities{}))

	// map tiles
	hMapLayerZXY := HandleMapLayerZXY{Atlas: a}
	group.UsingContext().Handler("GET", "/maps/:map_name/:z/:x/:y", HeadersHandler(GZipHandler(TileCacheHandler(a, hMapLayerZXY))))
	group.UsingContext().Handler("GET", "/maps/:map_name/:layer_name/:z/:x/:y", HeadersHandler(GZipHandler(TileCacheHandler(a, hMapLayerZXY))))

	// map style
	group.UsingContext().Handler("GET", "/maps/:map_name/style.json", HeadersHandler(HandleMapStyle{}))

	// setup viewer routes, which can be excluded via build flags
	setupViewer(group)

	return r
}

// Start starts the tile server binding to the provided port
func Start(a *atlas.Atlas, port string) *http.Server {

	// notify the user the server is starting
	log.Infof("starting tegola server on port %v", port)

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
			log.Fatal(err)
			return
		}
	}()

	return srv
}

// hostName determines weather to use an user defined HostName
// or the host from the incoming request
func hostName(r *http.Request) string {
	// if the HostName has been configured, don't mutate it
	if HostName != "" {
		return HostName
	}

	return r.Host
}

// various checks to determin if the request is http or https. the scheme is needed for the TileURLs
// r.URL.Scheme can be empty if a relative request is issued from the client. (i.e. GET /foo.html)
func scheme(r *http.Request) string {
	if r.Header.Get("X-Forwarded-Proto") != "" {
		return r.Header.Get("X-Forwarded-Proto")
	} else if r.TLS != nil {
		return "https"
	}

	return "http"
}

// URLRoot builds a string containing the scheme, host and port based on a combination of user defined values,
// headers and request parameters. The function is public so it can be overridden for other implementations.
var URLRoot = func(r *http.Request) *url.URL {
	root := url.URL{
		Scheme: scheme(r),
		Host:   hostName(r),
	}

	return &root
}

// buildCapabilitiesURL is responsible for building the various URLs which are returned by
// the capabilities endpoints using the request, uri parts, and query params the function
// will determine the protocol host:port and URI prefix that need to be included based on
// user defined configurations and request context
func buildCapabilitiesURL(r *http.Request, uriParts []string, query url.Values) string {
	uri := path.Join(uriParts...)
	q := query.Encode()
	if q != "" {
		// prepend our query identifier
		q = "?" + q
	}

	// usually the url.URL package would be used for building the URL, but the
	// uri template for the tiles contains characters that the package does not like:
	// {z}/{x}/{y}. These values are escaped during the String() call which does not
	// work for the capabilities URLs.
	return fmt.Sprintf("%v%v%v", URLRoot(r), path.Join(URIPrefix, uri), q)
}

// corsHanlder is used to respond to all OPTIONS requests for registered routes
func corsHandler(w http.ResponseWriter, r *http.Request, params map[string]string) {
	setHeaders(w)
	return
}

// setHeaders sets deafult headers and user defined headers
func setHeaders(w http.ResponseWriter) {
	// add our default CORS headers
	for name, val := range DefaultCORSHeaders {
		if val == "" {
			log.Warnf("default CORS header (%v) has no value", name)
		}

		w.Header().Set(name, val)
	}

	// set user defined headers
	for name, val := range Headers {
		if val == "" {
			log.Warnf("header (%v) has no value", name)
		}

		w.Header().Set(name, val)
	}
}
