// Package config loads and understands the tegola config format.
package config

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/alexeydott/tegola"
	"github.com/alexeydott/tegola/internal/env"
	"github.com/alexeydott/tegola/internal/log"
	"github.com/alexeydott/tegola/mapbox/tilejson"
	"github.com/alexeydott/tegola/provider"
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

// builtinReservedTokens returns a fresh copy of the built-in query injection
// tokens that are reserved for every config (part13 P6-28).
func builtinReservedTokens() map[string]struct{} {
	return map[string]struct{}{
		BboxToken:             {},
		ZoomToken:             {},
		XToken:                {},
		YToken:                {},
		ZToken:                {},
		ScaleDenominatorToken: {},
		PixelWidthToken:       {},
		PixelHeightToken:      {},
		IdFieldToken:          {},
		GeomFieldToken:        {},
		GeomTypeToken:         {},
	}
}

// ReservedTokens for query injection.
//
// Provider-compat mirror (part13 P6-28): provider implementations (e.g.
// provider/postgis) read this package-level map when they are created, so it
// must keep existing. Package variables, however, are shared process-wide
// mutable state: reloading a config used to accumulate tokens across loads.
// The authoritative per-load state now lives on Config.ReservedTokens and
// Validate republishes it here wholesale (replaced, never merged) after a
// successful validation. Do not write to this map outside of Validate.
var ReservedTokens = builtinReservedTokens()

// reservedTokensMu serializes publications to the ReservedTokens mirror:
// Validate may legitimately run concurrently (e.g. parallel test suites or
// config reloads) and would otherwise race on the shared map. Readers such
// as provider/postgis consult the mirror at provider-creation time, after
// startup validation has finished (part13 P6-28).
var reservedTokensMu sync.Mutex

// publishReservedTokens replaces the ReservedTokens mirror's contents with
// the given set wholesale (replaced, never merged).
func publishReservedTokens(tokens map[string]struct{}) {
	reservedTokensMu.Lock()
	defer reservedTokensMu.Unlock()
	clear(ReservedTokens)
	for token := range tokens {
		ReservedTokens[token] = struct{}{}
	}
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
	Webserver    Webserver      `toml:"webserver"`
	Features     FeaturesConfig `toml:"features"`
	WFS          WFSConfig      `toml:"wfs"`
	Cache        env.Dict       `toml:"cache"`
	Observer     env.Dict       `toml:"observer"`
	// Map of providers.
	//  all providers must have at least two entries.
	// 1. name -- this is the name that is referenced in
	// the maps section
	// 2. type -- this is the name the provider modules register
	// themselves under. (e.g. postgis, gpkg, mvt_postgis )
	// Note: Use the type to figure out if the provider is a mvt or std provider
	Providers []env.Dict     `toml:"providers"`
	Maps      []provider.Map `toml:"maps"`
	// ReservedTokens are the query tokens reserved for this config: the
	// built-in tokens plus the custom parameters declared by its maps. The
	// map is per-load state (part13 P6-28), populated lazily by Validate /
	// ValidateAndRegisterParams and never shared between loads. TOML
	// decoding must not touch it.
	ReservedTokens map[string]struct{} `toml:"-"`
}

// reservedTokens lazily initializes and returns the per-config reserved
// token set (part13 P6-28).
func (c *Config) reservedTokens() map[string]struct{} {
	if c.ReservedTokens == nil {
		c.ReservedTokens = builtinReservedTokens()
	}
	return c.ReservedTokens
}

// Webserver represents the config options for the webserver part of Tegola
type Webserver struct {
	TileHTTPMaxAge env.Int              `toml:"tile_http_max_age"`
	HostName       env.URL              `toml:"hostname"`
	Port           env.String           `toml:"port"`
	URIPrefix      env.String           `toml:"uri_prefix"`
	Headers        env.Dict             `toml:"headers"`
	SSLCert        env.String           `toml:"ssl_cert"`
	SSLKey         env.String           `toml:"ssl_key"`
	ProxyProtocol  env.String           `toml:"proxy_protocol"`
	TileOperations TileOperationsConfig `toml:"tile_operations"`
}

// TileOperationsConfig configures the privileged tile operation endpoints
// (?tile=update, ?tile=getupdated and the ?tile=dirty parameter). The feature
// is disabled by default and, when enabled, additionally requires the
// configured token (or the admin listener). See server/tile_operations.go for
// the enforcement semantics and server/README.md for the documentation.
type TileOperationsConfig struct {
	Enabled       bool       `toml:"enabled"`
	Token         env.String `toml:"token"`
	RatePerMinute env.Int    `toml:"rate_per_minute"`
	MaxConcurrent env.Int    `toml:"max_concurrent"`
}

// ValidateAndRegisterParams ensures configured params don't conflict with existing
// query tokens or have overlapping names. Reserved tokens are tracked on the
// Config (part13 P6-28), not in package-global state.
func (c *Config) ValidateAndRegisterParams(mapName string, params []provider.QueryParameter) error {
	if len(params) == 0 {
		return nil
	}

	reservedTokens := c.reservedTokens()
	usedNames := make(map[string]struct{})
	usedTokens := make(map[string]struct{})

	for _, param := range params {
		if _, ok := provider.ParamTypeDecoders[param.Type]; !ok {
			return ErrParamUnknownType{
				MapName:   string(mapName),
				Parameter: param,
			}
		}

		if len(param.DefaultSQL) > 0 && len(param.DefaultValue) > 0 {
			return ErrParamTwoDefaults{
				MapName:   string(mapName),
				Parameter: param,
			}
		}

		if len(param.DefaultValue) > 0 {
			decoderFn := provider.ParamTypeDecoders[param.Type]
			if _, err := decoderFn(param.DefaultValue); err != nil {
				return ErrParamInvalidDefault{
					MapName:   string(mapName),
					Parameter: param,
				}
			}
		}

		if _, ok := reservedTokens[param.Token]; ok {
			return ErrParamTokenReserved{
				MapName:   string(mapName),
				Parameter: param,
			}
		}

		if !provider.ParameterTokenRegexp.MatchString(param.Token) {
			return ErrParamBadTokenName{
				MapName:   string(mapName),
				Parameter: param,
			}
		}

		if _, ok := usedNames[param.Name]; ok {
			return ErrParamDuplicateName{
				MapName:   string(mapName),
				Parameter: param,
			}
		}

		if _, ok := usedTokens[param.Token]; ok {
			return ErrParamDuplicateToken{
				MapName:   string(mapName),
				Parameter: param,
			}
		}

		usedNames[param.Name] = struct{}{}
		usedTokens[param.Token] = struct{}{}
	}

	// Mark all used tokens as reserved on this config
	for token := range usedTokens {
		reservedTokens[token] = struct{}{}
	}

	return nil
}

// Validate checks the config for issues
func (c *Config) Validate() error {
	if err := c.Features.Validate(); err != nil {
		return err
	}
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
	mapLayers := map[string]map[string]provider.MapLayer{}
	// maps with configured parameters for logging
	mapsWithCustomParams := []string{}
	for mapKey, m := range c.Maps {
		// validate any declared query parameters
		if err := c.ValidateAndRegisterParams(string(m.Name), m.Parameters); err != nil {
			return err
		}

		if len(m.Parameters) > 0 {
			mapsWithCustomParams = append(mapsWithCustomParams, string(m.Name))
		}

		// map names must be unique (part13 P6-29)
		if _, ok := mapLayers[string(m.Name)]; ok {
			return ErrMapNameDuplicate{MapName: string(m.Name)}
		}
		mapLayers[string(m.Name)] = map[string]provider.MapLayer{}

		if m.TileJSONVersion != "" &&
			!slices.Contains(
				[]string{tilejson.Version3, tilejson.Version2},
				m.TileJSONVersion,
			) {
			return ErrUnknownTileJSONVersion
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

			// reject inverted zoom ranges (part13 P6-29)
			if *l.MinZoom > *l.MaxZoom {
				return ErrInvalidLayerZoomRange{
					MapName:       string(m.Name),
					ProviderLayer: string(l.ProviderLayer),
					MinZoom:       uint(*l.MinZoom),
					MaxZoom:       uint(*l.MaxZoom),
				}
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

	if len(mapsWithCustomParams) > 0 {
		log.Infof(
			"Caching is disabled for these maps, since they have configured custom parameters: %s",
			strings.Join(mapsWithCustomParams, ", "),
		)
	}

	if c.Webserver.TileHTTPMaxAge < 0 {
		return fmt.Errorf("webserver.tile_http_max_age must be non-negative")
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

	// Republish the per-config reserved tokens into the provider-compat
	// mirror for implementations that read config.ReservedTokens when they
	// are created (e.g. provider/postgis). Replace wholesale, never merge:
	// a reload must not keep tokens from a previous config (part13 P6-28).
	publishReservedTokens(c.reservedTokens())

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
		httpClient := &http.Client{
			Timeout: time.Second * 10,
		}

		// make the http request
		res, err := httpClient.Get(location)
		if err != nil {
			return conf, fmt.Errorf("error fetching remote config file (%v): %v ", location, err)
		}
		defer func() { _ = res.Body.Close() }()

		// require a success status so error pages don't end up in the TOML
		// parser (part13 P6-27)
		if res.StatusCode < 200 || res.StatusCode > 299 {
			return conf, fmt.Errorf("error fetching remote config file (%v). unexpected http status: %s", location, res.Status)
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
		f, err := os.Open(location)
		if err != nil {
			return conf, fmt.Errorf("error opening local config file (%v): %v ", location, err)
		}
		defer func() { _ = f.Close() }()
		reader = f
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
