package atlas

import (
	"context"
	"sync"

	"github.com/terranodo/tegola"
	"github.com/terranodo/tegola/cache"
	_ "github.com/terranodo/tegola/cache/filecache"
	_ "github.com/terranodo/tegola/cache/s3cache"
)

//	DefaultAtlas is instanitated for convenience
var DefaultAtlas = &Atlas{}

const (
	//	MaxZoom will not render tile beyond this zoom level
	MaxZoom = 22
)

type Atlas struct {
	// for managing current access to the map container
	sync.RWMutex
	// hold maps
	maps map[string]Map
	//	holds a reference to the cache backend
	cacher cache.Interface
}

func (a *Atlas) AllMaps() []Map {
	a.RLock()
	defer a.RUnlock()

	var maps []Map
	for i := range a.maps {
		m := a.maps[i]
		//	make an explict copy of the layers
		layers := make([]Layer, len(m.Layers))
		copy(layers, m.Layers)
		m.Layers = layers

		maps = append(maps, m)
	}

	return maps
}

//	SeedMapTile will generate a tile and persist it to the
//	configured cache backend
func (a *Atlas) SeedMapTile(m Map, tile tegola.Tile) error {
	//	confirm we have a cache backend
	if a.cacher == nil {
		return ErrMissingCache
	}

	//	encode the tile
	b, err := m.Encode(context.Background(), tile)
	if err != nil {
		return err
	}

	//	cache key
	key := cache.Key{
		MapName: m.Name,
		Z:       tile.Z,
		X:       tile.X,
		Y:       tile.Y,
	}

	return a.cacher.Set(&key, b)
}

//	PurgeMapTile will purge a map tile from the configured cache backend
func (a *Atlas) PurgeMapTile(m Map, tile tegola.Tile) error {
	if a.cacher == nil {
		return ErrMissingCache
	}

	//	cache key
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
	a.RLock()
	defer a.RUnlock()

	m, ok := a.maps[mapName]
	if !ok {
		return Map{}, ErrMapNotFound{
			Name: mapName,
		}
	}

	//	make an explict copy of the layers
	layers := make([]Layer, len(m.Layers))
	copy(layers, m.Layers)
	m.Layers = layers

	return m, nil
}

//	AddMap registers a map by name. if the map already exists it will be overwritten
func (a *Atlas) AddMap(m Map) {
	a.Lock()
	defer a.Unlock()

	if a.maps == nil {
		a.maps = map[string]Map{}
	}

	a.maps[m.Name] = m
}

//	GetCache returns the registered cache if one is registered, otherwise nil
func (a *Atlas) GetCache() cache.Interface {
	return a.cacher
}

//	SetCache sets the cache backend
func (a *Atlas) SetCache(c cache.Interface) {
	a.cacher = c
}

//	AllMaps returns all registered maps in DefaultAtlas
func AllMaps() []Map {
	return DefaultAtlas.AllMaps()
}

//	GetMap returns a copy of the a map by name from DefaultAtlas. if the map does not exist it will return an error
func GetMap(mapName string) (Map, error) {
	return DefaultAtlas.Map(mapName)
}

//	AddMap registers a map by name with DefaultAtlas. if the map already exists it will be overwritten
func AddMap(m Map) {
	DefaultAtlas.AddMap(m)
}

//	GetCache returns the registered cache for DefaultAtlas, if one is registered, otherwise nil
func GetCache() cache.Interface {
	return DefaultAtlas.GetCache()
}

//	SetCache sets the cache backend for DefaultAtlas
func SetCache(c cache.Interface) {
	DefaultAtlas.SetCache(c)
}

//	SeedMapTile will generate a tile and persist it to the
//	configured cache backend for the DefaultAtlas
func SeedMapTile(m Map, tile tegola.Tile) error {
	return DefaultAtlas.SeedMapTile(m, tile)
}

//	PurgeMapTile will purge a map tile from the configured cache backend
//	for the DefaultAtlas
func PurgeMapTile(m Map, tile tegola.Tile) error {
	return DefaultAtlas.PurgeMapTile(m, tile)
}
