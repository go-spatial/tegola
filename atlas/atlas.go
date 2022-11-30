// Package atlas provides an abstraction for a collection of Maps.
package atlas

import (
	"context"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/go-spatial/geom/slippy"
	"github.com/go-spatial/tegola"
	"github.com/go-spatial/tegola/cache"
	"github.com/go-spatial/tegola/internal/log"
	"github.com/go-spatial/tegola/internal/observer"
	"github.com/go-spatial/tegola/observability"
)

var (
	simplifyGeometries    bool
	simplificationMaxZoom uint = 10
)

func init() {
	// TODO(arolek): the following env variable processing was pulled form the mvt package when
	// geometry processing was pulled out of the encoding package. This functionality could be
	// deprecated/removed as it's not well documented and is really a band aid to work around
	// some simplification issues. These concepts could just as easily live in the config file.
	options := strings.ToLower(os.Getenv("TEGOLA_OPTIONS"))
	if strings.Contains(options, "dontsimplifygeo") {
		simplifyGeometries = false
		log.Debugf("simplification is disable")
	}

	if strings.Contains(options, "simplifymaxzoom=") {
		idx := strings.Index(options, "simplifymaxzoom=")
		idx += 16

		eidx := strings.IndexAny(options[idx:], ",.\t \n")
		if eidx == -1 {
			eidx = len(options)
		} else {
			eidx += idx
		}

		i, err := strconv.Atoi(options[idx:eidx])
		if err != nil {
			log.Errorf("invalid value for SimplifyMaxZoom (%v). using default (%v).", options[idx:eidx], simplificationMaxZoom)
			return
		}

		simplificationMaxZoom = uint(i + 1)

		log.Debugf("SimplifyMaxZoom set to (%v)", simplificationMaxZoom)
	}
}

// defaultAtlas is instantiated for convenience
var defaultAtlas = &Atlas{}

const (
	// MaxZoom will not render tile beyond this zoom level
	MaxZoom = tegola.MaxZ
)

// Atlas holds a collection of maps.
// If the pointer to Atlas is nil, it will make use of the default atlas; as the container for maps.
// This is equivalent to using the functions in the package.
// An Atlas is safe to use concurrently.
type Atlas struct {
	// for managing current access to the map container
	sync.RWMutex
	// hold maps
	maps map[string]Map
	// holds a reference to the cache backend
	cacher cache.Interface

	// holds a reference to the observer backend
	observer observability.Interface

	// publishBuildInfo indicates if we should publish the build info on change of observer
	// this is set by calling PublishBuildInfo, which will publish
	// the build info on the observer and insure changes to observer
	// also publishes the build info.
	publishBuildInfo bool
}

// AllMaps returns a slice of all maps contained in the Atlas so far.
func (a *Atlas) AllMaps() []Map {

	if a == nil {
		// Use the default Atlas if a, is nil. This way the empty value is
		// still useful.
		return defaultAtlas.AllMaps()
	}

	a.RLock()
	defer a.RUnlock()

	var maps []Map
	for i := range a.maps {
		m := a.maps[i]
		// make an explicit copy of the layers
		layers := make([]Layer, len(m.Layers))
		copy(layers, m.Layers)
		m.Layers = layers

		maps = append(maps, m)
	}

	return maps
}

// SeedMapTile will generate a tile and persist it to the
// configured cache backend
func (a *Atlas) SeedMapTile(ctx context.Context, m Map, z, x, y uint) error {

	if a == nil {
		// Use the default Atlas if a, is nil. This way the empty value is
		// still useful.
		return defaultAtlas.SeedMapTile(ctx, m, z, x, y)
	}

	ctx = context.WithValue(ctx, observability.ObserveVarMapName, m.Name)
	// confirm we have a cache backend
	if a.cacher == nil {
		return ErrMissingCache
	}

	tile := slippy.NewTile(z, x, y)

	// encode the tile
	b, err := m.Encode(ctx, tile, nil)
	if err != nil {
		return err
	}

	// cache key
	key := cache.Key{
		MapName: m.Name,
		Z:       z,
		X:       x,
		Y:       y,
	}

	return a.cacher.Set(&key, b)
}

// PurgeMapTile will purge a map tile from the configured cache backend
func (a *Atlas) PurgeMapTile(m Map, tile *tegola.Tile) error {
	if a == nil {
		// Use the default Atlas if a, is nil. This way the empty value is
		// still useful.
		return defaultAtlas.PurgeMapTile(m, tile)
	}

	if a.cacher == nil {
		return ErrMissingCache
	}

	// cache key
	key := cache.Key{
		MapName: m.Name,
		Z:       tile.Z,
		X:       tile.X,
		Y:       tile.Y,
	}

	return a.cacher.Purge(&key)
}

// Map looks up a Map by name and returns a copy of the Map
func (a *Atlas) Map(mapName string) (Map, error) {
	if a == nil {
		// Use the default Atlas if a, is nil. This way the empty value is
		// still useful.
		return defaultAtlas.Map(mapName)
	}

	a.RLock()
	defer a.RUnlock()

	m, ok := a.maps[mapName]
	if !ok {
		return Map{}, ErrMapNotFound{
			Name: mapName,
		}
	}

	// make an explicit copy of the layers
	layers := make([]Layer, len(m.Layers))
	copy(layers, m.Layers)
	m.Layers = layers

	return m, nil
}

// AddMap registers a map by name. if the map already exists it will be overwritten
func (a *Atlas) AddMap(m Map) {
	if a == nil {
		// Use the default Atlas if a, is nil. This way the empty value is
		// still useful.
		defaultAtlas.AddMap(m)
		return
	}
	a.Lock()
	defer a.Unlock()

	if a.maps == nil {
		a.maps = map[string]Map{}
	}

	a.maps[m.Name] = m
}

// GetCache returns the registered cache if one is registered, otherwise nil
func (a *Atlas) GetCache() cache.Interface {
	if a == nil {
		// Use the default Atlas if a, is nil. This way the empty value is
		// still useful.
		return defaultAtlas.GetCache()
	}
	return a.cacher
}

// SetCache sets the cache backend
func (a *Atlas) SetCache(c cache.Interface) {
	if a == nil {
		// Use the default Atlas if a, is nil. This way the empty value is
		// still useful.
		defaultAtlas.SetCache(c)
		return
	}
	// let's see if we have an observer set. If so, we need to wrap
	// the given cache with the observer.
	if a.observer != nil {
		c = a.observer.InstrumentedCache(c)
	}
	a.cacher = c
}

// SetObservability will set the observability backend
func (a *Atlas) SetObservability(o observability.Interface) {
	if a == nil {
		defaultAtlas.SetObservability(o)
		return
	}
	if a.observer != nil {
		a.observer.Shutdown()
	}
	a.observer = o
	if a.publishBuildInfo {
		a.observer.Init()
	}
	if a.cacher != nil {
		if w, ok := a.cacher.(observability.Cache); ok && w.IsObserver() {
			a.cacher = o.InstrumentedCache(w.Original())
		} else {
			a.cacher = o.InstrumentedCache(a.cacher)
		}
	}
	for _, aMap := range a.maps {

		collectors, err := aMap.Collectors("tegola", o.CollectorConfig)
		if err != nil {
			log.Errorf("failed to register collector for map: %v ignoring", aMap.Name)
			continue
		}
		o.MustRegister(collectors...)
	}
}

func (a *Atlas) Observer() observability.Interface {
	if a == nil {
		return defaultAtlas.Observer()
	}
	if a.observer == nil {
		return nil
	}
	if _, ok := a.observer.(observer.Null); ok {
		return nil
	}
	return a.observer
}

func (a *Atlas) StartSubProcesses() {
	if a == nil {
		defaultAtlas.StartSubProcesses()
		return
	}
	o := a.Observer()
	if o == nil {
		return
	}
	a.publishBuildInfo = true
	o.Init()
}

// AllMaps returns all registered maps in defaultAtlas
func AllMaps() []Map {
	return defaultAtlas.AllMaps()
}

// GetMap returns a copy of the a map by name from defaultAtlas. if the map does not exist it will return an error
func GetMap(mapName string) (Map, error) {
	return defaultAtlas.Map(mapName)
}

// AddMap registers a map by name with defaultAtlas. if the map already exists it will be overwritten
func AddMap(m Map) {
	defaultAtlas.AddMap(m)
}

// GetCache returns the registered cache for defaultAtlas, if one is registered, otherwise nil
func GetCache() cache.Interface {
	return defaultAtlas.GetCache()
}

// SetCache sets the cache backend for defaultAtlas
func SetCache(c cache.Interface) {
	defaultAtlas.SetCache(c)
}

// SeedMapTile will generate a tile and persist it to the
// configured cache backend for the defaultAtlas
func SeedMapTile(ctx context.Context, m Map, z, x, y uint) error {
	return defaultAtlas.SeedMapTile(ctx, m, z, x, y)
}

// PurgeMapTile will purge a map tile from the configured cache backend
// for the defaultAtlas
func PurgeMapTile(m Map, tile *tegola.Tile) error {
	return defaultAtlas.PurgeMapTile(m, tile)
}

// SetObservability sets the observability backend for the defaultAtlas
func SetObservability(o observability.Interface) { defaultAtlas.SetObservability(o) }

func StartSubProcesses() { defaultAtlas.StartSubProcesses() }
