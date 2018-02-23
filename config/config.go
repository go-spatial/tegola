/*
	config loads and understands the tegola config format.
*/
package config

import (
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"regexp"
	"strings"
	"syscall"
	"time"

	"github.com/BurntSushi/toml"

	"github.com/terranodo/tegola/internal/log"
)

// Config represents a tegola config file.
type Config struct {
	//	the tile buffer to use
	TileBuffer int64 `toml:"tile_buffer"`
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
	HostName          string `toml:"hostname"`
	Port              string `toml:"port"`
	CORSAllowedOrigin string `toml:"cors_allowed_origin"`
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
	//	DontSimplify indicates wheather feature simplification should be applied.
	//	We use a negative in the name so the default is to simplify
	DontSimplify bool `toml:"dont_simplify"`
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

// replaceEnvVars replaces environment variable placeholders in reader stream with values
// i.e. "val = $VAR" -> "val = 3"
func replaceEnvVars(reader io.Reader) (io.Reader, error) {
	// Variable definition follows IEEE Std 1003.1-2001
	//   A dollar sign ($) followed by an upper-case letter, followed by
	//   zero or more upper-case letters, digits, or underscores (_).
	varNameRegexStr := `[A-Z]+[A-Z1-9_]*`
	// Var prepended by dollar sign ($)
	// Ex: $MY_VAR7
	regexStrDS := fmt.Sprintf(`\$%v`, varNameRegexStr)
	// Additionally, match a variable surrounded by curly braces with leading dollar sign.
	// Ex: ${MY_VAR7}
	regexStrBraces := fmt.Sprintf(`\$\{%v\}`, varNameRegexStr)

	// Regex to capture either syntax
	regexStr := fmt.Sprintf("(%v|%v)", regexStrDS, regexStrBraces)
	varFinder := regexp.MustCompile(regexStr)

	configBytes, err := ioutil.ReadAll(reader)
	if err != nil {
		return nil, err
	}

	configStr := string(configBytes)

	// Grab the regular & braced placeholders
	varPlaceHolders := varFinder.FindAllString(configStr, -1)

	varNameFinder := regexp.MustCompile(varNameRegexStr)

	for _, ph := range varPlaceHolders {
		// Get the environment variable value (drop the leading dollar sign ($) and surrounding braces ({}))
		varName := varNameFinder.FindString(ph)
		envVal, found := syscall.Getenv(varName)
		if !found {
			return nil, ErrMissingEnvVar{
				EnvVar: varName,
			}
		}

		// Explicit string replacement, no need for regex funny business any longer.
		configStr = strings.Replace(configStr, ph, envVal, -1)
	}

	return strings.NewReader(configStr), nil
}

// Load will load and parse the config file from the given location.
func Load(location string) (conf Config, err error) {
	var reader io.Reader

	//	check for http prefix
	if strings.HasPrefix(location, "http") {
		log.Infof("loading remote config (%v)", location)

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
		log.Infof("loading local config (%v)", location)

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
		return conf, err
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
