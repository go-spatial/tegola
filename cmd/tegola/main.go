//tegola server
package main

import (
	"flag"
	"log"
	"os"
	"strings"

	"github.com/BurntSushi/toml"

	"github.com/terranodo/tegola/server"
)

type Config struct {
	Webserver struct {
		Port string
	}
	Providers []map[string]interface{}
	Maps      []struct {
		Name   string
		Layers []struct {
			Name     string
			Provider string
			Minzoom  int
			Maxzoom  int
		}
	}
}

//	hold parsed config from config file
var conf Config

//	flags
var (
	confPath = flag.String("conf", "config.toml", "path to a toml config file")
)

func main() {
	var err error

	//	parse our command line flags
	flag.Parse()

	//	check the conf file exists
	if _, err := os.Stat(*confPath); os.IsNotExist(err) {
		log.Fatal("config.toml file not found!")
	}

	//	decode conf file
	if _, err := toml.DecodeFile(*confPath, &conf); err != nil {
		log.Fatal(err)
	}

	//	holder for registered providers
	var registeredProviders map[string]*mvt.Proivder

	//	iterate providers
	for _, provider := range conf.Providers {
		log.Printf("provider %+v", provider)

		//	register the provider
	}

	//	iterate maps
	for _, m := range conf.Maps {
		var layers []Layer
		//	iterate layers
		for _, l := range m.Layers {
			//	split our provider into provider.query
			providerQuery := strings.Split(l.Provider, ".")

			if len(providerQuery) != 2 {
				log.Fatal("invalid layer provider for map: %v, layer %v: %v.", m, l.Name, l.Provider)
			}

			//	lookup our proivder
			provider, ok := registeredProviders[providerQuery[0]]
			if !ok {
				log.Fatal("provider not defined: %v", providerQuery[0])
			}

			//	setup our layer properties
			layer = server.Layer{
				Name:     l.Name,
				Minzoom:  l.Minzoom,
				Maxzoom:  l.Maxzoom,
				Provider: provider,
			}

			//	add our layer to our layer size
			layers = append(layers, layer)
		}

		//	register map
		server.RegisterMap(m, layers)
	}

	//	bind our webserver
	server.Start(conf.Webserver.Port)
}
