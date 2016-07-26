//tegola server
package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/BurntSushi/toml"

	"github.com/terranodo/tegola/mvt"
	"github.com/terranodo/tegola/mvt/provider"
	_ "github.com/terranodo/tegola/provider/postgis"
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
		Name   string `toml:"name"`
		Layers []struct {
			ProviderLayer string                 `toml:"provider_layer"`
			MinZoom       int                    `toml:"min_zoom"`
			MaxZoom       int                    `toml:"max_zoom"`
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

	if err = setupProviders(); err != nil {
		log.Fatal(err)
	}

	/*
		if err = setupLogger(); err != nil {
			log.Fatal(err)
		}
	*/

	//	start our webserver
	server.Start(conf.Webserver.Port)
}

/*
func setupLogger() {
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
}
*/
func setupProviders() error {
	var err error

	//	holder for registered providers
	registeredProviders := map[string]mvt.Provider{}

	//	iterate providers
	for _, p := range conf.Providers {
		log.Printf("provider %v", p)

		n, ok := p["name"]
		if !ok {
			return errors.New("missing 'name' parameter for provider")
		}

		name, found := n.(string)
		if !found {
			return fmt.Errorf("'name' or provider must be of type string")
		}

		//	register the provider
		prov, err := provider.For(name, p)
		if err != nil {
			return err
		}

		//	add the provider to our map of registered providers
		registeredProviders[name] = prov
	}

	log.Println(registeredProviders)

	//	iterate maps
	for _, m := range conf.Maps {
		var layers []server.Layer
		//	iterate layers
		for _, l := range m.Layers {
			//	split our provider into provider.query
			providerLayer := strings.Split(l.ProviderLayer, ".")

			//	we're expecting two params in the provider layer definition
			if len(providerLayer) != 2 {
				return fmt.Errorf("invalid provider layer (%v) for map (%v)", l.ProviderLayer, m)
			}

			//	lookup our proivder
			provider, ok := registeredProviders[providerLayer[0]]
			if !ok {
				return fmt.Errorf("provider not defined: %v", providerLayer[0])
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

	return err
}
