//  Package server implements the http frontend
package server

import (
	"fmt"
	"log"
	"net/http"

	"github.com/dimfeld/httptreemux"
)

const (
	//	MaxTileSize is 500k. Currently just throws a warning when tile
	//	is larger than MaxTileSize
	MaxTileSize = 500000
	//	MaxZoom will not render tile beyond this zoom level
	MaxZoom = 20
)

//	set at runtime from main
var Version string

//	incoming requests are associated with a map
var maps = map[string]Map{}

//	RegisterMap associates layers with map names
func RegisterMap(m Map) error {
	//	check if our map is already registered
	if _, ok := maps[m.Name]; ok {
		return fmt.Errorf("map (%v) is alraedy registered", m.Name)
	}

	//	associate our layers with a map
	maps[m.Name] = m

	return nil
}

//	Start starts the tile server binding to the provided port
func Start(port string) {
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
	group.UsingContext().Handler("GET", "/maps/:map_name/:z/:x/:y", HandleMapZXY{})
	group.UsingContext().Handler("OPTIONS", "/maps/:map_name/:z/:x/:y", HandleMapZXY{})

	//	map layer tiles
	group.UsingContext().Handler("GET", "/maps/:map_name/:layer_name/:z/:x/:y", HandleLayerZXY{})
	group.UsingContext().Handler("OPTIONS", "/maps/:map_name/:layer_name/:z/:x/:y", HandleLayerZXY{})

	//	static convenience routes
	group.UsingContext().Handler("GET", "/", http.FileServer(http.Dir("static")))
	group.UsingContext().Handler("GET", "/*path", http.FileServer(http.Dir("static")))

	//	start our server
	log.Fatal(http.ListenAndServe(port, r))
}
