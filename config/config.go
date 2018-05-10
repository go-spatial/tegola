/*
	config loads and understands the tegola config format.
*/
package config

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/BurntSushi/toml"

	"github.com/go-spatial/tegola"
	"github.com/go-spatial/tegola/internal/dict/env"
	"github.com/go-spatial/tegola/internal/log"
)

// Config represents a tegola config file.
type Config struct {
	// the tile buffer to use
	TileBuffer int64 `toml:"tile_buffer"`
	// LocationName is the file name or http server that the config was read from.
	// If this is an empty string, it means that the location was unknown. This is the case if
	// the Parse() function is used directly.
	LocationName string
	Webserver    Webserver              `toml:"webserver"`
	Cache        env.Map `toml:"cache"`
	// Map of providers.
	Providers []env.Map
	Maps      []Map
}

type Webserver struct {
	HostName          env.String `toml:"hostname"`
	Port              env.String `toml:"port"`
	CORSAllowedOrigin env.String `toml:"cors_allowed_origin"`
}

// A Map represents a map in the Tegola Config file.
type Map struct {
	Name        env.String   `toml:"name"`
	Attribution env.String   `toml:"attribution"`
	Bounds      []env.Float  `toml:"bounds"`
	Center      [3]env.Float `toml:"center"`
	Layers      []MapLayer   `toml:"layers"`
}

type MapLayer struct {
	// Name is optional. If it's not defined the name of the ProviderLayer will be used.
	// Name can also be used to group multiple ProviderLayers under the same namespace.
	Name          env.String  `toml:"name"`
	ProviderLayer env.String  `toml:"provider_layer"`
	MinZoom       *env.Uint   `toml:"min_zoom"`
	MaxZoom       *env.Uint   `toml:"max_zoom"`
	DefaultTags   interface{} `toml:"default_tags"`
	// DontSimplify indicates wheather feature simplification should be applied.
	// We use a negative in the name so the default is to simplify
	DontSimplify env.Bool `toml:"dont_simplify"`
}

// GetName helper to get the name we care about.
func (ml MapLayer) GetName() (string, error) {
	if ml.Name != "" {
		return string(ml.Name), nil
	}
	// split the provider layer (syntax is provider.layer)
	plParts := strings.Split(string(ml.ProviderLayer), ".")
	if len(plParts) != 2 {
		return "", ErrInvalidProviderLayerName{ProviderLayerName: string(ml.ProviderLayer)}
	}

	return plParts[1], nil
}

// checks the config for issues
func (c *Config) Validate() error {

	// check for map layer name / zoom collisions
	// map of layers to providers
	mapLayers := map[string]map[string]MapLayer{}
	for mapKey, m := range c.Maps {
		if _, ok := mapLayers[string(m.Name)]; !ok {
			mapLayers[string(m.Name)] = map[string]MapLayer{}
		}

		for layerKey, l := range m.Layers {
			name, err := l.GetName()
			if err != nil {
				return err
			}

			// MaxZoom default
			if l.MaxZoom == nil {
				ph := env.Uint(tegola.MaxZ)
				// set in iterated value
				l.MaxZoom = &ph
				// set in underlying config struct
				c.Maps[mapKey].Layers[layerKey].MaxZoom = &ph
			}
			// MinZoom default
			if l.MinZoom == nil {
				ph := env.Uint(0)
				// set in iterated value
				l.MinZoom = &ph
				// set in underlying config struct
				c.Maps[mapKey].Layers[layerKey].MinZoom = &ph
			}

			// check if we already have this layer
			if val, ok := mapLayers[string(m.Name)][name]; ok {
				// we have a hit. check for zoom range overlap
				if uint(*val.MinZoom) <= uint(*l.MaxZoom) && uint(*l.MinZoom) <= uint(*val.MaxZoom) {
					return ErrOverlappingLayerZooms{
						ProviderLayer1: string(val.ProviderLayer),
						ProviderLayer2: string(l.ProviderLayer),
					}
				}
				continue
			}

			// add the MapLayer to our map
			mapLayers[string(m.Name)][name] = l
		}
	}

	return nil
}

// Parse will parse the Tegola config file provided by the io.Reader.
func Parse(reader io.Reader, location string) (conf Config, err error) {
	// decode conf file, don't care about the meta data.
	_, err = toml.DecodeReader(reader, &conf)
	conf.LocationName = location

	return conf, err
}

// Load will load and parse the config file from the given location.
func Load(location string) (conf Config, err error) {
	var reader io.Reader

	// check for http prefix
	if strings.HasPrefix(location, "http") {
		log.Infof("loading remote config (%v)", location)

		// setup http client with a timeout
		var httpClient = &http.Client{
			Timeout: time.Second * 10,
		}

		// make the http request
		res, err := httpClient.Get(location)
		if err != nil {
			return conf, fmt.Errorf("error fetching remote config file (%v): %v ", location, err)
		}

		// set the reader to the response body
		reader = res.Body
	} else {
		log.Infof("loading local config (%v)", location)

		// check the conf file exists
		if _, err := os.Stat(location); os.IsNotExist(err) {
			return conf, fmt.Errorf("config file at location (%v) not found!", location)
		}
		// open the confi file
		reader, err = os.Open(location)
		if err != nil {
			return conf, fmt.Errorf("error opening local config file (%v): %v ", location, err)
		}
	}

	return Parse(reader, location)
}

func LoadAndValidate(filename string) (cfg Config, err error) {
	cfg, err = Load(filename)
	if err != nil {
		return cfg, err
	}
	// validate our config
	return cfg, cfg.Validate()
}
