// Package config loads and understands the tegola config format.
package config

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/go-spatial/tegola"
	"github.com/go-spatial/tegola/internal/env"
	"github.com/go-spatial/tegola/internal/log"
	"github.com/go-spatial/tegola/provider"
)

const (
	BboxToken             = "!BBOX!"
	ZoomToken             = "!ZOOM!"
	XToken                = "!X!"
	YToken                = "!Y!"
	ZToken                = "!Z!"
	ScaleDenominatorToken = "!SCALE_DENOMINATOR!"
	PixelWidthToken       = "!PIXEL_WIDTH!"
	PixelHeightToken      = "!PIXEL_HEIGHT!"
	IdFieldToken          = "!ID_FIELD!"
	GeomFieldToken        = "!GEOM_FIELD!"
	GeomTypeToken         = "!GEOM_TYPE!"
)

// ReservedTokens for query injection
var ReservedTokens = []string{
	BboxToken,
	ZoomToken,
	XToken,
	YToken,
	ZToken,
	ScaleDenominatorToken,
	PixelWidthToken,
	PixelHeightToken,
	IdFieldToken,
	GeomFieldToken,
	GeomTypeToken,
}

// IsReservedToken returns true if the specified token is reserved
func IsReservedToken(token string) bool {
	for _, t := range ReservedTokens {
		if token == t {
			return true
		}
	}
	return false
}

// ParameterTokenRegexp to validate QueryParameters
var ParameterTokenRegexp = regexp.MustCompile("![a-zA-Z0-9_-]+!")

// ParamTypeDecoders is a collection of parsers for different types of user-defined parameters
var ParamTypeDecoders = map[string]func(string) (interface{}, error){
	"int": func(s string) (interface{}, error) {
		return strconv.Atoi(s)
	},
	"float": func(s string) (interface{}, error) {
		return strconv.ParseFloat(s, 32)
	},
	"string": func(s string) (interface{}, error) {
		return s, nil
	},
	"bool": func(s string) (interface{}, error) {
		return strconv.ParseBool(s)
	},
}

var blacklistHeaders = []string{"content-encoding", "content-length", "content-type"}

// Config represents a tegola config file.
type Config struct {
	// the tile buffer to use
	TileBuffer *env.Int `toml:"tile_buffer"`
	// LocationName is the file name or http server that the config was read from.
	// If this is an empty string, it means that the location was unknown. This is the case if
	// the Parse() function is used directly.
	LocationName string
	Webserver    Webserver `toml:"webserver"`
	Cache        env.Dict  `toml:"cache"`
	Observer     env.Dict  `toml:"observer"`
	// Map of providers.
	//  all providers must have at least two entries.
	// 1. name -- this is the name that is referenced in
	// the maps section
	// 2. type -- this is the name the provider modules register
	// themselves under. (e.g. postgis, gpkg, mvt_postgis )
	// Note: Use the type to figure out if the provider is a mvt or std provider
	Providers []env.Dict `toml:"providers"`
	Maps      []Map      `toml:"maps"`
}

// Webserver represents the config options for the webserver part of Tegola
type Webserver struct {
	HostName  env.String `toml:"hostname"`
	Port      env.String `toml:"port"`
	URIPrefix env.String `toml:"uri_prefix"`
	Headers   env.Dict   `toml:"headers"`
	SSLCert   env.String `toml:"ssl_cert"`
	SSLKey    env.String `toml:"ssl_key"`
}

// A Map represents a map in the Tegola Config file.
type Map struct {
	Name        env.String       `toml:"name"`
	Attribution env.String       `toml:"attribution"`
	Bounds      []env.Float      `toml:"bounds"`
	Center      [3]env.Float     `toml:"center"`
	Layers      []MapLayer       `toml:"layers"`
	Parameters  []QueryParameter `toml:"params"`
	TileBuffer  *env.Int         `toml:"tile_buffer"`
}

// ValidateParams ensures configured params don't conflict with existing
// query tokens or have overlapping names
func (m Map) ValidateParams() error {
	if len(m.Parameters) == 0 {
		return nil
	}

	var usedNames, usedTokens []string

	for _, param := range m.Parameters {
		if _, ok := ParamTypeDecoders[param.Type]; !ok {
			return ErrParamUnknownType{
				MapName:   string(m.Name),
				Parameter: param,
			}
		}

		if len(param.DefaultSQL) > 0 && len(param.DefaultValue) > 0 {
			return ErrParamTwoDefaults{
				MapName:   string(m.Name),
				Parameter: param,
			}
		}

		if len(param.DefaultValue) > 0 {
			decoderFn := ParamTypeDecoders[param.Type]
			if _, err := decoderFn(param.DefaultValue); err != nil {
				return ErrParamInvalidDefault{
					MapName:   string(m.Name),
					Parameter: param,
				}
			}
		}

		if IsReservedToken(param.Token) {
			return ErrParamTokenReserved{
				MapName:   string(m.Name),
				Parameter: param,
			}
		}

		if !ParameterTokenRegexp.MatchString(param.Token) {
			return ErrParamBadTokenName{
				MapName:   string(m.Name),
				Parameter: param,
			}
		}

		for _, name := range usedNames {
			if name == param.Name {
				return ErrParamNameDuplicate{
					MapName:   string(m.Name),
					Parameter: param,
				}
			}
		}

		for _, token := range usedTokens {
			if token == param.Token {
				return ErrParamTokenDuplicate{
					MapName:   string(m.Name),
					Parameter: param,
				}
			}
		}

		usedNames = append(usedNames, param.Name)
		usedTokens = append(usedTokens, param.Token)
	}

	return nil
}

// MapLayer represents a the config for a layer in a map
type MapLayer struct {
	// Name is optional. If it's not defined the name of the ProviderLayer will be used.
	// Name can also be used to group multiple ProviderLayers under the same namespace.
	Name          env.String `toml:"name"`
	ProviderLayer env.String `toml:"provider_layer"`
	MinZoom       *env.Uint  `toml:"min_zoom"`
	MaxZoom       *env.Uint  `toml:"max_zoom"`
	DefaultTags   env.Dict   `toml:"default_tags"`
	// DontSimplify indicates whether feature simplification should be applied.
	// We use a negative in the name so the default is to simplify
	DontSimplify env.Bool `toml:"dont_simplify"`
	// DontClip indicates whether feature clipping should be applied.
	// We use a negative in the name so the default is to clipping
	DontClip env.Bool `toml:"dont_clip"`
	// DontClip indicates whether feature cleaning (e.g. make valid) should be applied.
	// We use a negative in the name so the default is to clean
	DontClean env.Bool `toml:"dont_clean"`
}

// ProviderLayerName returns the names of the layer and provider or an error
func (ml MapLayer) ProviderLayerName() (provider, layer string, err error) {
	// split the provider layer (syntax is provider.layer)
	plParts := strings.Split(string(ml.ProviderLayer), ".")
	if len(plParts) != 2 {
		return "", "", ErrInvalidProviderLayerName{ProviderLayerName: string(ml.ProviderLayer)}
	}
	return plParts[0], plParts[1], nil
}

// GetName will return the user-defined Layer name from the config,
// or if the name is empty, return the name of the layer associated with
// the provider
func (ml MapLayer) GetName() (string, error) {
	if ml.Name != "" {
		return string(ml.Name), nil
	}
	_, name, err := ml.ProviderLayerName()
	return name, err
}

// QueryParameter represents an HTTP query parameter specified for use with
// a given map instance.
type QueryParameter struct {
	Name  string `toml:"name"`
	Token string `toml:"token"`
	Type  string `toml:"type"`
	SQL   string `toml:"sql"`
	// DefaultSQL replaces SQL if param wasn't passed. Either default_sql or
	// default_value can be specified
	DefaultSQL   string `toml:"default_sql"`
	DefaultValue string `toml:"default_value"`
	IsRequired   bool
}

// Normalize will normalize param and set the default values
func (param *QueryParameter) Normalize() {
	param.Token = strings.ToUpper(param.Token)

	sql := "?"
	if len(param.SQL) > 0 {
		sql = param.SQL
	}
	param.SQL = sql

	isRequired := true
	if len(param.DefaultSQL) > 0 || len(param.DefaultValue) > 0 {
		isRequired = false
	}
	param.IsRequired = isRequired
}

// Validate checks the config for issues
func (c *Config) Validate() error {

	var knownTypes []string
	drivers := make(map[string]int)
	for _, name := range provider.Drivers(provider.TypeStd) {
		drivers[name] = int(provider.TypeStd)
		knownTypes = append(knownTypes, name)
	}
	for _, name := range provider.Drivers(provider.TypeMvt) {
		drivers[name] = int(provider.TypeMvt)
		knownTypes = append(knownTypes, name)
	}
	// mvtproviders maps a known provider name to whether that provider is
	// an mvt provider or not.
	mvtproviders := make(map[string]bool, len(c.Providers))
	for i, prvd := range c.Providers {
		name, _ := prvd.String("name", nil)
		if name == "" {
			return ErrProviderNameRequired{Pos: i}
		}
		typ, _ := prvd.String("type", nil)
		if typ == "" {
			return ErrProviderTypeRequired{Pos: i}
		}
		// Check to see if the name has already been seen before.
		if _, ok := mvtproviders[name]; ok {
			return ErrProviderNameDuplicate{Pos: i}
		}
		drv, ok := drivers[typ]
		if !ok {
			return ErrUnknownProviderType{
				Name:           name,
				Type:           typ,
				KnownProviders: knownTypes,
			}
		}
		mvtproviders[name] = drv == int(provider.TypeMvt)
	}
	// check for map layer name / zoom collisions
	// map of layers to providers
	mapLayers := map[string]map[string]MapLayer{}
	for mapKey, m := range c.Maps {

		// validate any declared query parameters
		if err := m.ValidateParams(); err != nil {
			return err
		}

		if _, ok := mapLayers[string(m.Name)]; !ok {
			mapLayers[string(m.Name)] = map[string]MapLayer{}
		}

		// Set current provider to empty, for MVT providers
		// we can only have the same provider for all layers.
		// This allow us to track what the first found provider
		// is.
		currentProvider := ""
		isMVTProvider := false
		for layerKey, l := range m.Layers {
			pname, _, err := l.ProviderLayerName()
			if err != nil {
				return err
			}

			if currentProvider == "" {
				// This is the first provider we found.
				// For MVTProviders all others need to be the same, so store it
				// so we can check later
				currentProvider = pname
			}

			isMvt, doesExists := mvtproviders[pname]
			if !doesExists {
				return ErrInvalidProviderForMap{
					MapName:      string(m.Name),
					ProviderName: pname,
				}
			}

			// check to see if any of the prior provider or this one is
			// an mvt provider. If it is, then the mvtProvider check needs
			// to be done
			isMVTProvider = isMVTProvider || isMvt

			// only need to do this check if we are dealing with MVTProviders
			if isMVTProvider && pname != currentProvider {
				// for mvt_providers we can only have the same provider
				// for all layers
				// check to see
				if mvtproviders[pname] || isMVTProvider {
					return ErrMVTDifferentProviders{
						Original: currentProvider,
						Current:  pname,
					}
				}
			}

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

			if int(*l.MaxZoom) == 0 {
				log.Warn("max_zoom of 0 is not supported. adjusting to '1'")
				ph := env.Uint(1)
				// set in iterated value
				l.MaxZoom = &ph
				// set in underlying config struct
				c.Maps[mapKey].Layers[layerKey].MaxZoom = &ph
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

	// check for blacklisted headers
	for k := range c.Webserver.Headers {
		for _, v := range blacklistHeaders {
			if v == strings.ToLower(k) {
				return ErrInvalidHeader{Header: k}
			}
		}
	}

	// check if webserver.uri_prefix is set and if so
	// confirm it starts with a forward slash "/"
	if string(c.Webserver.URIPrefix) != "" {
		uriPrefix := string(c.Webserver.URIPrefix)
		if string(uriPrefix[0]) != "/" {
			return ErrInvalidURIPrefix(uriPrefix)
		}
	}

	return nil
}

// ConfigureTileBuffers handles setting the tile buffer for a Map
func (c *Config) ConfigureTileBuffers() {
	// range our configured maps
	for mapKey, m := range c.Maps {
		// if there is a tile buffer config for this map, use it
		if m.TileBuffer != nil {
			c.Maps[mapKey].TileBuffer = m.TileBuffer
			continue
		}

		// if there is a global tile buffer config, use it
		if c.TileBuffer != nil {
			c.Maps[mapKey].TileBuffer = c.TileBuffer
			continue
		}

		// tile buffer is not configured, use default
		c.Maps[mapKey].TileBuffer = env.IntPtr(env.Int(tegola.DefaultTileBuffer))
	}
}

// Parse will parse the Tegola config file provided by the io.Reader.
func Parse(reader io.Reader, location string) (conf Config, err error) {
	// decode conf file, don't care about the meta data.
	_, err = toml.NewDecoder(reader).Decode(&conf)
	if err != nil {
		return conf, err
	}

	for _, m := range conf.Maps {
		for k, p := range m.Parameters {
			p.Normalize()
			m.Parameters[k] = p
		}
	}

	conf.LocationName = location

	conf.ConfigureTileBuffers()

	return conf, nil
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
	} else if location == "-" {
		log.Infof("loading local config from stdin")
		reader = os.Stdin
	} else {
		log.Infof("loading local config (%v)", location)

		// check the conf file exists
		if _, err := os.Stat(location); os.IsNotExist(err) {
			return conf, fmt.Errorf("config file at location (%v) not found", location)
		}
		// open the config file
		reader, err = os.Open(location)
		if err != nil {
			return conf, fmt.Errorf("error opening local config file (%v): %v ", location, err)
		}
	}

	return Parse(reader, location)
}

// LoadAndValidate will load the config from the given filename and validate it if it was
// able to load the file
func LoadAndValidate(filename string) (cfg Config, err error) {
	cfg, err = Load(filename)
	if err != nil {
		return cfg, err
	}
	// validate our config
	return cfg, cfg.Validate()
}
