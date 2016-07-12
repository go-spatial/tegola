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

<<<<<<< HEAD
//	mapping for layers
var maps map[string][]Layer

type layer struct {
	Name     string
	MinZoom  int
	MaxZoom  int
	Provider mvt.Provider
}

//	config

//Init maps sets up our data providers and builds out the
//	map and layer associations
func Init(conf Config) error {
	//	var providers map[string]tegola.Provider
	var layers map[string]*layer
	//	group our layers by provider
	var providerLayers map[string]map[string]string

	//	group our layers by providers
	for i, layer := range conf.Layers {
		//	lookup our provider
		_, ok := providerLayers[strings.ToLower(layer.Provider)]
		if !ok {
			providerLayers[layer.Provider] = map[string]string{}
		}

		//	add the layer to the provider and include it's config
		providerLayers[strings.ToLower(layer.Provider)][i] = layer.Config
	}

	//	init our providers
	for i, provider := range conf.Providers {
		//	switch on our various provider types
		switch strings.ToLower(provider.Type) {
		case tegola.ProviderPostGIS:

			//	lookup our layers for the provider
			postgisLayers, ok := providerLayers[strings.ToLower(i)]
			if !ok {
				return errors.New("missing provider: " + i)
			}

			c := postgis.Config{
				Host:     provider.Host,
				Port:     provider.Port,
				Database: provider.Database,
				User:     provider.User,
				Password: provider.Password,
				Layers:   postgisLayers,
			}

			log.Println("provider conf", c)

			//	init our provider
			p, err := postgis.NewProvider(c)
			if err != nil {
				return err
			}

			//	associate our layers with our instantiated provider
			for i := range postgisLayers {
				l := layer{
					Name:     i,
					Provider: p,
				}
				//	add the layer to our layers map
				layers[strings.ToLower(provider.Type)] = &l
			}
		}

	}

	/*
		//	setup our maps
		for i := range conf.Maps {
			//	look up map layer

		}
	*/
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
