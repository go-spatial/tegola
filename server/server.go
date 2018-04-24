//  Package server implements the http frontend
package server

import (
	"net/http"
	"strings"

	"github.com/dimfeld/httptreemux"

	"github.com/go-spatial/tegola"
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

	// CORSAllowedOrigin is the "Access-Control-Allow-Origin" CORS header.
	// configurable via the tegola config.toml file (set in main.go)
	CORSAllowedOrigin string = "*"

	// TileBuffer is the tile buffer to use.
	// configurable via tegola config.tomal file (set in main.go)
	TileBuffer float64 = tegola.DefaultTileBuffer
)

// NewRouter set's up the our routes.
func NewRouter(a *atlas.Atlas) *httptreemux.TreeMux {
	r := httptreemux.New()
	group := r.NewGroup("/")

	// one handler to respond to all OPTIONS (CORS) requests for registered routes
	r.OptionsHandler = corsHandler

	// capabilities endpoints
	group.UsingContext().Handler("GET", "/capabilities", HandleCapabilities{})
	group.UsingContext().Handler("GET", "/capabilities/:map_name", HandleMapCapabilities{})

	// map tiles
	hMapLayerZXY := HandleMapLayerZXY{Atlas: a}
	group.UsingContext().Handler("GET", "/maps/:map_name/:z/:x/:y", TileCacheHandler(a, hMapLayerZXY))
	group.UsingContext().Handler("GET", "/maps/:map_name/:layer_name/:z/:x/:y", TileCacheHandler(a, hMapLayerZXY))

	// map style
	group.UsingContext().Handler("GET", "/maps/:map_name/style.json", HandleMapStyle{})

	//	setup viewer routes, which can excluded via build flags
	setupViewer(group)

	return r
}

// Start starts the tile server binding to the provided port
func Start(a *atlas.Atlas, port string) *http.Server {

	// notify the user the server is starting
	log.Infof("starting tegola server on port %v", port)

	// start our server
	srv := &http.Server{Addr: port, Handler: NewRouter(a)}
	go func() { log.Error(srv.ListenAndServe()) }()
	return srv
}

// determines the hostname:port to return based on the following hierarchy
// - HostName / Port vars as configured via the config file
// - The request host / port if config HostName or Port is missing
func hostName(r *http.Request) string {
	var requestHostname string
	var requestPort string

	substrs := strings.Split(r.Host, ":")

	switch len(substrs) {
	case 1:
		requestHostname = substrs[0]
	case 2:
		requestHostname = substrs[0]
		requestPort = substrs[1]
	default:
		log.Warnf("multiple colons (':') in host string: %v", r.Host)
	}

	retHost := HostName
	if HostName == "" {
		retHost = requestHostname
	}

	if Port != "" && Port != "none" {
		return retHost + Port
	}
	if requestPort != "" && Port != "none" {
		return retHost + ":" + requestPort
	}

	return retHost
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

// corsHanlder is used to respond to all OPTIONS requests for registered routes
func corsHandler(w http.ResponseWriter, r *http.Request, params map[string]string) {
	w.Header().Set("Access-Control-Allow-Origin", CORSAllowedOrigin)
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	return
}
