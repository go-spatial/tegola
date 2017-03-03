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

var ErrMapNotFound error

func init() {
	ErrMapNotFound = fmt.Errorf("Did not find map")
}

// A Config represents the a Tegola Config file.
type Config struct {
	// LocationName is the file name or http server that the config was read from. If this is "", it means that the location was unknown. This is the case if the Prase() function is used
	// directly.
	LocationName string
	Webserver    struct {
		Port      string
		LogFile   string `toml:"log_file"`
		LogFormat string `toml:"log_format"`
	}
	// Map of providers.
	Providers []map[string]interface{}
	Maps      []Map
}

// A Map represents a map in the Tegola Config file.
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
	return Map{}, ErrMapNotFound
}
