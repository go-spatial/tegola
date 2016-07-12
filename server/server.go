package server

import (
	"errors"
	"log"
	"net/http"
	"strings"

	"github.com/terranodo/tegola"
	"github.com/terranodo/tegola/mvt"
	"github.com/terranodo/tegola/provider/postgis"
)

//	requests come in looking for a map name.

var maps = map[string][]*mapLayer{}

type mapLayer struct {
	Name     string
	Provider mvt.Provider
}

/*
func init() {
	config := postgis.Config{
		Host:     "localhost",
		Port:     5432,
		Database: "gdey",
		User:     "gdey",
		Layers: map[string]string{
			"landuse": "gis.zoning_base_3857",
		},
	}
	var err error
	postgisProvider, err = postgis.NewProvider(config)
	if err != nil {
		panic(fmt.Sprintf("Failed to create a new provider. %v", err))
	}
}
*/

//	config

//Init maps sets up our data providers and builds out the
//	map and layer associations
func Init(conf Config) error {
	//	var providers map[string]tegola.Provider
	layers := map[string]*mapLayer{}
	//	group our layers by provider
	providerLayers := map[string]map[string]string{}

	//	group our layers by providers
	for _, layer := range conf.Layers {
		//	get layer provider name
		providerName := strings.ToLower(layer.Provider)

		//	lookup our provider
		_, ok := providerLayers[providerName]
		if !ok {
			//	provider not found, create an entry
			providerLayers[providerName] = map[string]string{}
		}

		//	add the layer to the provider and include it's SQL
		providerLayers[providerName][layer.Name] = layer.SQL

		log.Printf("providerLayers %+v", providerLayers)
	}

	//	init our providers
	for _, provider := range conf.Providers {
		//	switch on our various provider types
		switch strings.ToLower(provider.Type) {
		case tegola.ProviderPostGIS:

			//	lookup our layers for the provider
			postgisLayers, ok := providerLayers[strings.ToLower(provider.Name)]
			if !ok {
				return errors.New("missing provider: " + provider.Name)
			}

			c := postgis.Config{
				Host:     provider.Host,
				Port:     provider.Port,
				Database: provider.Database,
				User:     provider.User,
				Password: provider.Password,
				Layers:   postgisLayers,
			}

			log.Printf("provider conf %v", c)

			//	init our provider
			p, err := postgis.NewProvider(c)
			if err != nil {
				return err
			}

			//	associate our layers with our instantiated provider
			for i, _ := range postgisLayers {
				//	add the layer to our layers map
				layers[i] = &mapLayer{
					Name:     i,
					Provider: p,
				}
			}
		}

	}

	//	setup our maps
	for _, m := range conf.Maps {
		//	look up map layer
		layer, ok := layers[m.Layer]
		if !ok {
			return errors.New("missing layer: " + m.Layer)
		}

		//	check if our map key exists
		_, ok = maps[m.Name]
		if !ok {
			//	provider not found, create an entry
			maps[m.Name] = []*mapLayer{}
		}

		//	add our layer to the maps layer slice
		maps[m.Name] = append(maps[m.Name], layer)
	}

	log.Printf("conf %+v\n", conf)

	return nil
}

// Start starts the tile server binding to the provided port
func Start(port string) {
	//	notify the user the server is starting
	log.Printf("Starting tegola server on port %v\n", port)

	// Main page.
	http.Handle("/", http.FileServer(http.Dir("static")))
	// setup routes
	http.HandleFunc("/maps/", handleZXY)

	// TODO: make http port configurable
	log.Fatal(http.ListenAndServe(port, nil))
}
