package server

import (
	"errors"
	"log"
	"net/http"

	"github.com/terranodo/tegola/mvt"
)

//	incoming requests are associated with a map
var maps = map[string][]Layer{}

type Layer struct {
	Name    string
	MinZoom int
	MaxZoom int
	//	instantiated provider
	Provider mvt.Provider
	//	default tags to include when encoding the layer. provider tags take precedence
	DefaultTags map[string]interface{}
	//	the layer stacking position as setup in the config file
	OrderIndex int
}

//	RegisterMap associates layers with map names
func RegisterMap(name string, layers []Layer) error {
	//	check if our map is already registered
	if _, ok := maps[name]; ok {
		return errors.New("map is alraedy registered: " + name)
	}

	//	associate our layers with a map
	maps[name] = layers

	return nil
}

//	Start starts the tile server binding to the provided port
func Start(port string) {
	//	notify the user the server is starting
	log.Printf("Starting tegola server on port %v", port)

	//	setup routes
	http.Handle("/", http.FileServer(http.Dir("static")))
	http.HandleFunc("/maps/", handleZXY)

	//	start our server
	log.Fatal(http.ListenAndServe(port, nil))
}
