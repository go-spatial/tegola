//  Package server implements the http frontend
package server

import (
	"log"
	"net/http"
	"strings"

	"github.com/dimfeld/httptreemux"
	"github.com/terranodo/tegola/atlas"
	_ "github.com/terranodo/tegola/cache/filecache"
)

const (
	//	MaxTileSize is 500k. Currently just throws a warning when tile
	//	is larger than MaxTileSize
	MaxTileSize = 500000
)

var (
	//	set at runtime from main
	Version string
	//	configurable via the tegola config.toml file (set in main.go)
	HostName string
	//	configurable via the tegola config.toml file (set in main.go)
	Port string
	//	reference to the version of atlas to work with
	Atlas *atlas.Atlas
)

//	Start starts the tile server binding to the provided port
func Start(port string) {
	Atlas = atlas.DefaultAtlas

	//	notify the user the server is starting
	log.Printf("Starting tegola server on port %v", port)

	r := httptreemux.New()
	group := r.NewGroup("/")

	//	capabilities endpoints
	group.UsingContext().Handler("GET", "/capabilities", HandleCapabilities{})
	group.UsingContext().Handler("OPTIONS", "/capabilities", HandleCapabilities{})
	group.UsingContext().Handler("GET", "/capabilities/:map_name", HandleMapCapabilities{})
	group.UsingContext().Handler("OPTIONS", "/capabilities/:map_name", HandleMapCapabilities{})

	//	map tiles
	group.UsingContext().Handler("GET", "/maps/:map_name/:z/:x/:y", TileCacheHandler(HandleMapZXY{}))
	group.UsingContext().Handler("OPTIONS", "/maps/:map_name/:z/:x/:y", HandleMapZXY{})
	group.UsingContext().Handler("GET", "/maps/:map_name/style.json", HandleMapStyle{})

	//	map layer tiles
	group.UsingContext().Handler("GET", "/maps/:map_name/:layer_name/:z/:x/:y", TileCacheHandler(HandleMapLayerZXY{}))
	group.UsingContext().Handler("OPTIONS", "/maps/:map_name/:layer_name/:z/:x/:y", HandleMapLayerZXY{})

	//	static convenience routes
	group.UsingContext().Handler("GET", "/", http.FileServer(assetFS()))
	group.UsingContext().Handler("GET", "/*path", http.FileServer(assetFS()))

	//	start our server
	log.Fatal(http.ListenAndServe(port, r))
}

//	determines the hostname:port to return based on the following hierarchy
//	- HostName / Port vars as configured via the config file
//	- The request host / port if config HostName or Port is missing
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
		log.Printf("Multiple colons (':') in host string: %v", r.Host)
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

//	various checks to determin if the request is http or https. the scheme is needed for the TileURLs
//	r.URL.Scheme can be empty if a relative request is issued from the client. (i.e. GET /foo.html)
func scheme(r *http.Request) string {
	if r.Header.Get("X-Forwarded-Proto") != "" {
		return r.Header.Get("X-Forwarded-Proto")
	} else if r.TLS != nil {
		return "https"
	}

	return "http"
}
