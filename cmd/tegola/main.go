//tegola server
package main

import (
	"errors"
	"flag"
	"fmt"
	"html"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/BurntSushi/toml"

	"github.com/terranodo/tegola/mvt"
	"github.com/terranodo/tegola/mvt/provider"
	_ "github.com/terranodo/tegola/provider/postgis"
	"github.com/terranodo/tegola/server"
)

var (
	//	set at buildtime via the CI
	Version = "version not set"
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
	Name        string     `toml:"name"`
	Attribution string     `toml:"attribution"`
	Bounds      []float64  `toml:"bounds"`
	Center      [3]float64 `toml:"center"`
	Layers      []struct {
		ProviderLayer string      `toml:"provider_layer"`
		MinZoom       int         `toml:"min_zoom"`
		MaxZoom       int         `toml:"max_zoom"`
		DefaultTags   interface{} `toml:"default_tags"`
	} `toml:"layers"`
}

func main() {
	var err error

	//	parse our command line flags
	flag.Parse()

	conf, err := loadConfig(configFile)
	if err != nil {
		log.Fatal(err)
	}

	//	log.Println("config webserver port", conf.Webserver.Port)

	//	init our providers
	providers, err := initProviders(conf.Providers)
	if err != nil {
		log.Fatal(err)
	}

	//	init our maps
	if err = initMaps(conf.Maps, providers); err != nil {
		log.Fatal(err)
	}

	initLogger(logFile, logFormat, conf.Webserver.LogFile, conf.Webserver.LogFormat)

	//	check config for port setting
	if port == defaultHTTPPort && conf.Webserver.Port != "" {
		port = conf.Webserver.Port
	}

	//	set our server version
	server.Version = Version

	//	start our webserver
	server.Start(port)
}

//	parseConfig handles loading a config file locally or remote over http(s)
func loadConfig(confLocation string) (Config, error) {
	var err error
	var conf Config
	var reader io.Reader

	//	check for http prefix
	if strings.HasPrefix(confLocation, "http") {
		log.Printf("Loading remote config (%v)", confLocation)

		//	setup http client with a timeout
		var httpClient = &http.Client{
			Timeout: time.Second * 10,
		}

		//	make the http request
		res, err := httpClient.Get(confLocation)
		if err != nil {
			return conf, fmt.Errorf("error fetching remote config file (%v): %v ", confLocation, err)
		}

		//	set the reader to the response body
		reader = res.Body
	} else {
		log.Printf("Loading local config (%v)", confLocation)

		//	check the conf file exists
		if _, err := os.Stat(confLocation); os.IsNotExist(err) {
			return conf, fmt.Errorf("config file at location (%v) not found!", confLocation)
		}

		//	open the confi file
		reader, err = os.Open(confLocation)
		if err != nil {
			return conf, fmt.Errorf("error opening local config file (%v): %v ", confLocation, err)
		}
	}

	//	decode conf file
	if _, err := toml.DecodeReader(reader, &conf); err != nil {
		return conf, err
	}

	return conf, nil
}

//	initMaps registers maps with our server
func initMaps(maps []Map, providers map[string]mvt.Provider) error {

	//	iterate our maps
	for _, m := range maps {

		serverMap := server.NewMap(m.Name)
		//	sanitize the provided attirbution string
		serverMap.Attribution = html.EscapeString(m.Attribution)
		serverMap.Center = m.Center
		if len(m.Bounds) == 4 {
			serverMap.Bounds = [4]float64{m.Bounds[0], m.Bounds[1], m.Bounds[2], m.Bounds[3]}
		}

		//	var layers []server.Layer
		//	iterate our layers
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
				return fmt.Errorf("provider (%v) not defined", providerLayer[0])
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
			serverMap.Layers = append(serverMap.Layers, server.Layer{
				Name:        providerLayer[1],
				MinZoom:     l.MinZoom,
				MaxZoom:     l.MaxZoom,
				Provider:    provider,
				DefaultTags: defaultTags,
			})
		}

		//	register map
		server.RegisterMap(serverMap)
	}

	return nil
}

func initProviders(providers []map[string]interface{}) (map[string]mvt.Provider, error) {
	var err error

	//	holder for registered providers
	registeredProviders := map[string]mvt.Provider{}

	//	iterate providers
	for _, p := range providers {
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
			return registeredProviders, fmt.Errorf("missing 'type' parameter for provider (%v)", pname)
		}

		ptype, found := t.(string)
		if !found {
			return registeredProviders, fmt.Errorf("'type' for provider (%v) must be a string", pname)
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

func initLogger(cmdFile, cmdFormat, confFile, confFormat string) {
	var err error
	filename := cmdFile
	format := cmdFormat
	var file *os.File

	if filename == "" {
		filename = confFile
	}
	if filename == "" {
		return
	}
	if format == "" {
		format = confFormat
	}

	if file, err = os.OpenFile(filename, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0666); err != nil {
		log.Printf("Unable to open logfile (%v) for writing: %v", filename, err)
		os.Exit(3)
	}
	server.L = &server.Logger{
		File:   file,
		Format: format,
	}
}
