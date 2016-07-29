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
	Maps      []Map
}

type Map struct {
	Name   string `toml:"name"`
	Layers []struct {
		ProviderLayer string      `toml:"provider_layer"`
		MinZoom       int         `toml:"min_zoom"`
		MaxZoom       int         `toml:"max_zoom"`
		DefaultTags   interface{} `toml:"default_tags"`
	} `toml:"layers"`
}

func main() {
	var err error
	//	hold parsed config from config file
	var conf Config

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

	//	init our providers
	providers, err := initProviders(conf.Providers)
	if err != nil {
		log.Fatal(err)
	}

	//	init our maps
	if err = initMaps(conf.Maps, providers); err != nil {
		log.Fatal(err)
	}

	/*
		if err = setupLogger(); err != nil {
			log.Fatal(err)
		}
	*/

	//	if port was not set via the command line
	if port == "" {
		//	do we have a port in our config file
		if conf.Webserver.Port != "" {
			port = conf.Webserver.Port
		}
		//	default is :8080
	}

	//	start our webserver
	server.Start(port)
}

//	initMaps registers maps with our server
func initMaps(maps []Map, providers map[string]mvt.Provider) error {

	//	range over our maps
	for _, m := range maps {
		var layers []server.Layer
		//	iterate layers
		for _, l := range m.Layers {
			//	split our provider name (provider.layer) into [provider,layer]
			providerLayer := strings.Split(l.ProviderLayer, ".")

			//	we're expecting two params in the provider layer definition
			if len(providerLayer) != 2 {
				return fmt.Errorf("invalid provider layer (%v) for map (%v)", l.ProviderLayer, m)
			}

			//	lookup our proivder
			provider, ok := providers[providerLayer[0]]
			if !ok {
				return fmt.Errorf("provider not defined: %v", providerLayer[0])
			}

			//	read the provider's layer names
			names := provider.LayerNames()

			//	confirm our providerLayer name is registered
			var found bool
			for i := range names {
				if names[i] == providerLayer[1] {
					found = true
				}
			}
			if !found {
				return fmt.Errorf("map (%v) 'provider_layer' (%v) is not registered with provider (%v)", m.Name, l.ProviderLayer, providerLayer[1])
			}

			var defaultTags map[string]interface{}
			if l.DefaultTags != nil {
				var ok bool
				defaultTags, ok = l.DefaultTags.(map[string]interface{})
				if !ok {
					return fmt.Errorf("'default_tags' for 'provider_layer' (%v) should be a TOML table", l.ProviderLayer)
				}
			}

			//	add our layer to our layers slice
			layers = append(layers, server.Layer{
				Name:        providerLayer[1],
				MinZoom:     l.MinZoom,
				MaxZoom:     l.MaxZoom,
				Provider:    provider,
				DefaultTags: defaultTags,
			})
		}

		//	register map
		server.RegisterMap(m.Name, layers)
	}

	return nil
}

func initProviders(providers []map[string]interface{}) (map[string]mvt.Provider, error) {
	var err error

	//	holder for registered providers
	registeredProviders := map[string]mvt.Provider{}

	//	iterate providers
	for _, p := range providers {
		log.Printf("provider %v", p)

		//	lookup our proivder name
		n, ok := p["name"]
		if !ok {
			return registeredProviders, errors.New("missing 'name' parameter for provider")
		}

		pname, found := n.(string)
		if !found {
			return registeredProviders, fmt.Errorf("'name' or provider must be of type string")
		}

		//	check if a proivder with this name is alrady registered
		_, ok = registeredProviders[pname]
		if ok {
			return registeredProviders, fmt.Errorf("provider (%v) already registered!", pname)
		}

		//	lookup our provider type
		t, ok := p["type"]
		if !ok {
			return registeredProviders, errors.New("missing 'type' parameter for provider")
		}

		ptype, found := t.(string)
		if !found {
			return registeredProviders, fmt.Errorf("'type' or provider must be of type string")
		}

		//	register the provider
		prov, err := provider.For(ptype, p)
		if err != nil {
			return registeredProviders, err
		}

		//	add the provider to our map of registered providers
		registeredProviders[pname] = prov
	}

	return registeredProviders, err
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
