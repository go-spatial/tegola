//tegola server
package main

import (
	"errors"
	"flag"
	"fmt"
	"html"
	"log"
	"os"
	"strings"

	"github.com/terranodo/tegola"
	"github.com/terranodo/tegola/cache"
	"github.com/terranodo/tegola/config"
	"github.com/terranodo/tegola/mvt"
	"github.com/terranodo/tegola/mvt/provider"
	_ "github.com/terranodo/tegola/provider/gpkg"
	_ "github.com/terranodo/tegola/provider/postgis"
	"github.com/terranodo/tegola/server"
)

var (
	//	set at buildtime via the CI
	Version = "version not set"
)

var codeLogFile *os.File

func main() {
	var err error

	//	parse our command line flags
	flag.Parse()

	//	if the user is looking for tegola version info, print it and exit
	if *version {
		fmt.Println(Version)
		os.Exit(0)
	}

	defer setupProfiler().Stop()

	conf, err := config.Load(*configFile)
	if err != nil {
		log.Fatal(err)
	}

	//	validate our config
	if err = conf.Validate(); err != nil {
		log.Fatal(err)
	}

	//	init our providers
	providers, err := initProviders(conf.Providers)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("Initializing logFile: ", *logFile)
	initLogger(*logFile, *logFormat, conf.Webserver.LogFile, conf.Webserver.LogFormat)

	//	init our maps
	if err = initMaps(conf.Maps, providers); err != nil {
		log.Fatal(err)
	}
	fmt.Println("conf.Maps after initMaps(): ", conf.Maps)

	if len(conf.Cache) != 0 {
		//	init cache backends
		cache, err := initCache(conf.Cache)
		if err != nil {
			log.Fatal(err)
		}
		if cache != nil {
			server.Cache = cache
		}
	}

	initLogger(*logFile, *logFormat, conf.Webserver.LogFile, conf.Webserver.LogFormat)

	//	check config for port setting
	//	if you set the port via the comand line it will override a port setting in the config
	if *port == defaultHTTPPort && conf.Webserver.Port != "" {
		port = &conf.Webserver.Port
	}

	//	set our server version
	server.Version = Version
	server.HostName = conf.Webserver.HostName

	//	start our webserver
	server.Start(*port)
}

func initCache(config map[string]interface{}) (cache.Interface, error) {
	//	lookup our cache type
	t, ok := config["type"]
	if !ok {
		return nil, fmt.Errorf("missing 'type' parameter for cache")
	}

	cType, ok := t.(string)
	if !ok {
		return nil, fmt.Errorf("'type' parameter for cache must be of type string")
	}

	//	register the provider
	return cache.For(cType, config)
}

//	initMaps registers maps with our server
func initMaps(maps []config.Map, providers map[string]mvt.Provider) error {

	//	iterate our maps
	for _, m := range maps {

		serverMap := server.NewMap(m.Name)
		//	sanitize the provided attirbution string
		serverMap.Attribution = html.EscapeString(m.Attribution)
		serverMap.Center = m.Center
		if len(m.Bounds) == 4 {
			serverMap.Bounds = [4]float64{m.Bounds[0], m.Bounds[1], m.Bounds[2], m.Bounds[3]}
		}

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
			layerInfos, err := provider.Layers()
			if err != nil {
				return fmt.Errorf("error fetching layer info from provider (%v)", providerLayer[0])
			}

			//	confirm our providerLayer name is registered
			var found bool
			var layerGeomType tegola.Geometry
			for i := range layerInfos {
				if layerInfos[i].Name() == providerLayer[1] {
					found = true

					//	read the layerGeomType
					layerGeomType = layerInfos[i].GeomType()
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
				Name:              l.Name,
				ProviderLayerName: providerLayer[1],
				MinZoom:           l.MinZoom,
				MaxZoom:           l.MaxZoom,
				Provider:          provider,
				DefaultTags:       defaultTags,
				GeomType:          layerGeomType,
			})
		}

		//	register map
		server.RegisterMap(serverMap)
	}

	return nil
}

func initProviders(providers []map[string]interface{}) (map[string]mvt.Provider, error) {
	var err error

	for idx, prv := range providers {
		fmt.Println("---")
		fmt.Println("Found provider: ", idx, ": ", prv["name"])
	}

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

	fmt.Println("Opening log file at: ", filename)
	if file, err = os.OpenFile(filename, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0666); err != nil {
		log.Printf("Unable to open logfile (%v) for writing: %v", filename, err)
		os.Exit(3)
	}
	server.L = &server.Logger{
		File:   file,
		Format: format,
	}
}
