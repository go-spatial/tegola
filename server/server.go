//  Package server implements the http frontend
package server

import (
	"errors"
	"log"
	"net/http"

	"github.com/terranodo/tegola/mvt"
)

//	incoming requests are associated with a map
var maps = map[string]layers{}

type layers []Layer

//	FilterByZoom returns layers that that are to be rendered between a min and max zoom
func (ls layers) FilterByZoom(zoom int) (filteredLayers []Layer) {
	for _, l := range ls {
		if (l.MinZoom <= zoom || l.MinZoom == 0) && (l.MaxZoom >= zoom || l.MaxZoom == 0) {
			filteredLayers = append(filteredLayers, l)
		}
	}
	return
}

type Layer struct {
	Name    string
	MinZoom int
	MaxZoom int
	//	instantiated provider
	Provider mvt.Provider
	//	default tags to include when encoding the layer. provider tags take precedence
	DefaultTags map[string]interface{}
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
	http.HandleFunc("/capabilities", handleCapabilities)

	//	start our server
	log.Fatal(http.ListenAndServe(port, nil))
}
