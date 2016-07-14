package server

import (
	"errors"
	"strings"

	"github.com/terranodo/tegola"
	"github.com/terranodo/tegola/provider/postgis"
)

type Config struct {
	Providers []Provider
	Maps      []Map
	Layers    []Layer
}

type Provider struct {
	Name     string
	Type     string
	Host     string
	Port     uint16
	Database string
	User     string
	Password string
}

type Map struct {
	Name   string
	Layers []Layer
}

type Layer struct {
	Name      string
	Provider  string
	Minzoom   int
	Maxzoom   int
	TableName string
	SQL       string
}

//Init maps sets up our data providers and builds out the
//	map and layer associations
func Init(conf Config) error {
	//	instantiated layer holder
	layers := map[string]*mapLayer{}

	//	group our layers by provider
	providerLayers := map[string]map[string]string{}

	//	iterate our maps config
	for _, m := range conf.Maps {
		//	iterate our layers config
		for _, l := range m.Layers {
			//	get layer provider name
			providerName := strings.ToLower(l.Provider)
			layerName := strings.ToLower(l.Name)

			//	lookup provider
			_, ok := providerLayers[providerName]
			if !ok {
				//	provider not found, create an entry
				providerLayers[providerName] = map[string]string{}
			}

			//	add the layer to the provider and include it's SQL
			providerLayers[providerName][layerName] = l.SQL
		}
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

		//	iterate our layers
		for _, l := range m.Layers {
			//	look up map layer
			layer, ok := layers[strings.ToLower(l.Name)]
			if !ok {
				return errors.New("missing layer: " + l.Name)
			}

			//	check if our map key exists
			_, ok = maps[m.Name]
			if !ok {
				//	provider not found, create an entry
				maps[m.Name] = []*mapLayer{}
			}

			//	add additional web server params
			layer.Minzoom = l.Minzoom
			layer.Maxzoom = l.Maxzoom

			//	add our layer to the maps layer slice
			maps[m.Name] = append(maps[m.Name], layer)
		}
	}

	return nil
}
