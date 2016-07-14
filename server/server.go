package server

import (
	"log"
	"net/http"

	"github.com/terranodo/tegola/mvt"
)

//	incoming requests are associated with a map
var maps = map[string][]*mapLayer{}

//	map layers point to a provider
type mapLayer struct {
	Name     string
	Minzoom  int
	Maxzoom  int
	Provider mvt.Provider
}

// Start starts the tile server binding to the provided port
func Start(port string) {
	//	notify the user the server is starting
	log.Printf("Starting tegola server on port %v\n", port)

	//	setup routes
	http.Handle("/", http.FileServer(http.Dir("static")))
	http.HandleFunc("/maps/", handleZXY)

	// TODO: make http port configurable
	log.Fatal(http.ListenAndServe(port, nil))
}
