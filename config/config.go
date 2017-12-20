/*
Config loads and understand the tegola config format.
*/
package config

import (
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"regexp"
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

type ErrOverlappingLayerZooms struct {
	ProviderLayer1 string
	ProviderLayer2 string
}

func (e ErrOverlappingLayerZooms) Error() string {
	return fmt.Sprintf("config: overlapping zooms for layer (%v) and layer (%v)", e.ProviderLayer1, e.ProviderLayer2)
}

// Config represents a tegola config file.
type Config struct {
	// LocationName is the file name or http server that the config was read from.
	// If this is an empty string, it means that the location was unknown. This is the case if
	// the Parse() function is used directly.
	LocationName string
	Webserver    Webserver              `toml:"webserver"`
	Cache        map[string]interface{} `toml:"cache"`
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
	mapLayers := map[string]map[string]MapLayer{}
	for _, m := range c.Maps {
		if _, ok := mapLayers[m.Name]; !ok {
			mapLayers[m.Name] = map[string]MapLayer{}
		}

		for _, l := range m.Layers {
			var name string

			if l.Name != "" {
				name = l.Name
			} else {
				//	split the provider layer (syntax is provider.layer)
				plParts := strings.Split(l.ProviderLayer, ".")
				if len(plParts) != 2 {
					return ErrInvalidProviderLayerName{
						ProviderLayerName: l.ProviderLayer,
					}
				}

				name = plParts[1]
			}

			//	check if we already have this layer
			if val, ok := mapLayers[m.Name][name]; ok {
				//	we have a hit. check for zoom range overlap
				if val.MinZoom <= l.MaxZoom && l.MinZoom <= val.MaxZoom {
					return ErrOverlappingLayerZooms{
						ProviderLayer1: val.ProviderLayer,
						ProviderLayer2: l.ProviderLayer,
					}
				}
				continue
			}

			//	add the MapLayer to our map
			mapLayers[m.Name][name] = l
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

func replaceEnvVars(reader io.Reader) (io.Reader, error) {
	// Replaces environment variable placeholders in reader stream with values
	// i.e. "val = $VAR" -> "val = 3"
	// Variable definition follows IEEE Std 1003.1-2001
	//   A dollar sign ($) followed by an upper-case letter, followed by
	//   zero or more upper-case letters, digits, or underscores (_).
	regexStr := `\$[A-Z]+[A-Z1-9_]*`
	varFinder := regexp.MustCompile(regexStr)
	configBytes, err := ioutil.ReadAll(reader)
	if err != nil {
		log.Printf("Problem reading from config reader: %v", err)
		return nil, err
	}
	configStr := string(configBytes)

	varPlaceHolders := varFinder.FindAllString(configStr, -1)
	for _, ph := range varPlaceHolders {
		// Get the environment variable value (drop the leading dollar sign ($))
		envVal := os.Getenv(ph[1:])
		// Escape the leading dollar sign for use in regex.
		replr := regexp.MustCompile(fmt.Sprintf("\\%v", ph))
		configStr = replr.ReplaceAllString(configStr, envVal)
	}

	return strings.NewReader(configStr), nil
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

	reader, err = replaceEnvVars(reader)
	if err != nil {
		log.Printf("Problem with call to replaceEnvVars: %v\n", err)
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
