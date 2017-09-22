/*
Config loads and understand the tegola config format.
*/
package config

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
)

type ErrMapNotFound struct {
	MapName string
}

func (e ErrMapNotFound) Error() string {
	return fmt.Sprintf("config: map (%v) not found", e.MapName)
}

type ErrInvalidProviderLayerName struct {
	ProviderLayerName string
}

func (e ErrInvalidProviderLayerName) Error() string {
	return fmt.Sprintf("config: invalid provider layer name (%v)", e.ProviderLayerName)
}

type ErrLayerCollision struct {
	ProviderLayer1 string
	ProviderLayer2 string
}

func (e ErrLayerCollision) Error() string {
	return fmt.Sprintf("config: layer collision (%v) and (%v)", e.ProviderLayer1, e.ProviderLayer2)
}

// A Config represents the a Tegola Config file.
type Config struct {
	// LocationName is the file name or http server that the config was read from. If this is "", it means that the location was unknown. This is the case if the Prase() function is used
	// directly.
	LocationName string
	Webserver    Webserver `toml:"webserver"`
	// Map of providers.
	Providers []map[string]interface{}
	Maps      []Map
}

type Webserver struct {
	HostName  string `toml:"hostname"`
	Port      string `toml:"port"`
	LogFile   string `toml:"log_file"`
	LogFormat string `toml:"log_format"`
}

// A Map represents a map in the Tegola Config file.
type Map struct {
	Name        string     `toml:"name"`
	Attribution string     `toml:"attribution"`
	Bounds      []float64  `toml:"bounds"`
	Center      [3]float64 `toml:"center"`
	Layers      []MapLayer `toml:"layers"`
}

type MapLayer struct {
	//	Name is optional. If it's not defined the name of the ProviderLayer will be used.
	//	Name can also be used to group multiple ProviderLayers under the same namespace.
	Name          string      `toml:"name"`
	ProviderLayer string      `toml:"provider_layer"`
	MinZoom       int         `toml:"min_zoom"`
	MaxZoom       int         `toml:"max_zoom"`
	DefaultTags   interface{} `toml:"default_tags"`
}

//	checks the config for issues
func (c *Config) Validate() error {

	//	check for map layer name / zoom collisions
	//	map of layers to providers
	layerNames := map[string]MapLayer{}
	for _, m := range c.Maps {
		for _, l := range m.Layers {
			//	split the provider layer (syntax is provider.layer)
			plParts := strings.Split(l.ProviderLayer, ".")
			if len(plParts) != 2 {
				return ErrInvalidProviderLayerName{
					ProviderLayerName: l.ProviderLayer,
				}
			}

			//	check if already have this layer
			if val, ok := layerNames[plParts[1]]; ok {
				//	we have a hit. check for zoom range overlap
				if val.MinZoom <= l.MaxZoom && l.MinZoom <= val.MaxZoom {
					return ErrLayerCollision{
						ProviderLayer1: val.ProviderLayer,
						ProviderLayer2: l.ProviderLayer,
					}
				}
				continue
			}

			//	add the MapLayer to our map
			layerNames[plParts[1]] = l
		}
	}

	return nil
}

// Parse will parse the Tegola config file provided by the io.Reader.
func Parse(reader io.Reader, location string) (conf Config, err error) {
	//	decode conf file, don't care about the meta data.
	_, err = toml.DecodeReader(reader, &conf)
	conf.LocationName = location

	return conf, err
}

// Load will load and parse the config file from the given location.
func Load(location string) (conf Config, err error) {
	var reader io.Reader

	//	check for http prefix
	if strings.HasPrefix(location, "http") {
		log.Printf("Loading remote config (%v)", location)

		//	setup http client with a timeout
		var httpClient = &http.Client{
			Timeout: time.Second * 10,
		}

		//	make the http request
		res, err := httpClient.Get(location)
		if err != nil {
			return conf, fmt.Errorf("error fetching remote config file (%v): %v ", location, err)
		}

		//	set the reader to the response body
		reader = res.Body
	} else {
		log.Printf("Loading local config (%v)", location)

		//	check the conf file exists
		if _, err := os.Stat(location); os.IsNotExist(err) {
			return conf, fmt.Errorf("config file at location (%v) not found!", location)
		}
		//	open the confi file
		reader, err = os.Open(location)
		if err != nil {
			return conf, fmt.Errorf("error opening local config file (%v): %v ", location, err)
		}
	}

	return Parse(reader, location)
}

// FindMap will find the map with the provided name. If "" is used for the name, it will return the first
// Map in the config, if one is defined.
// If a map with the name is not found it will return ErrMapNotFound error.
func (cfg *Config) FindMap(name string) (Map, error) {
	if name == "" && len(cfg.Maps) > 0 {
		return cfg.Maps[0], nil
	}

	for _, m := range cfg.Maps {
		if m.Name == name {
			return m, nil
		}
	}

	return Map{}, ErrMapNotFound{
		MapName: name,
	}
}
