//tegola server
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"text/template"

	"github.com/BurntSushi/toml"

	"github.com/terranodo/tegola/server"
)

type Config struct {
	Webserver struct {
		Port      string
		LogFile   string `toml:"log_file"`
		LogFormat string `toml:"log_format"`
	}
	Providers []map[string]interface{}
	Maps      []struct {
		Name   string
		Layers []struct {
			ProviderLayer string                 `toml:"provider_layer"`
			MinZoom       int                    `toml:"minzoom"`
			MaxZoom       int                    `toml:"maxzoom"`
			DefaultTags   map[string]interface{} `toml:"default_tags"`
		}
	}
}

//	hold parsed config from config file
var conf Config

func main() {
	var err error

	//	parse our command line flags
	flag.Parse()

	//	check the conf file exists
	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		log.Fatal(configFile + " not found!")
	}
	//	decode conf file
	if _, err := toml.DecodeFile(configFile, &conf); err != nil {
		log.Fatal(err)
	}

	setupServer()

	//	start our webserver
	server.Start(conf.Webserver.Port)
}

func setupServer() {
	var err error

	// Command line logfile overrides config file.
	if logFile != "" {
		conf.Webserver.LogFile = logFile
		// Need to make sure that the log file exists.
	}

	if server.DefaultLogFormat != logFormat || conf.Webserver.LogFormat == "" {
		conf.Webserver.LogFormat = logFormat
	}

	if conf.Webserver.LogFile != "" {
		if server.LogFile, err = os.OpenFile(logFile, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0666); err != nil {
			log.Printf("Unable to open logfile (%v) for writing: %v", logFile, err)
			os.Exit(2)
		}
	}

	if conf.Webserver.LogFormat == "" {
		conf.Webserver.LogFormat = server.DefaultLogFormat
	}

	if conf.Webserver.LogFile != "" {
		logFile = conf.Webserver.LogFile
	}

	// Command line logTemplate overrides config file.
	if logFormat == "" {
		server.LogTemplate = template.New("logfile")

		if _, err := server.LogTemplate.Parse(server.DefaultLogFormat); err != nil {
			log.Fatal(fmt.Sprintf("Could not parse default template: %v error: %v", server.DefaultLogFormat, err))
		}
	} else {
		server.LogTemplate = conf.Webserver.LogFormat
	}

	//	setup our server log template
	server.LogTemplate = template.New("logfile")

	if _, err := server.LogTemplate.Parse(conf.Webserver.LogFormat); err != nil {
		log.Printf("Could not parse log template: %v error: %v", conf.Webserver.LogFormat, err)
		os.Exit(3)
	}

	//	holder for registered providers
	var registeredProviders map[string]mvt.Proivder

	//	iterate providers
	for _, provider := range conf.Providers {
		log.Printf("provider %+v", provider)

		//	register the provider
	}

	//	iterate maps
	for _, m := range conf.Maps {
		var layers []server.Layer
		//	iterate layers
		for _, l := range m.Layers {
			//	split our provider into provider.query
			providerLayer := strings.Split(l.ProviderLayer, ".")

			//	we're expecting two params in the provider layer definition
			if len(providerLayer) != 2 {
				log.Fatal("invalid provider layer (%v) for map (%v)", l.ProviderLayer, m)
			}

			//	lookup our proivder
			provider, ok := registeredProviders[providerLayer[0]]
			if !ok {
				log.Fatal("provider not defined: %v", providerLayer[0])
			}

			//	add our layer to our layers slice
			layers = append(layers, server.Layer{
				Name:     providerLayer[1],
				MinZoom:  l.MinZoom,
				MaxZoom:  l.MaxZoom,
				Provider: provider,
			})
		}

		//	register map
		server.RegisterMap(m.Name, layers)
	}
}
