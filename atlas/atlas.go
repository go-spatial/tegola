package atlas

import (
	"sync"

	"github.com/terranodo/tegola/cache"
)

//	DefaultAtlas is instanitated for convenience
var DefaultAtlas = &Atlas{}

const (
	//	MaxZoom will not render tile beyond this zoom level
	MaxZoom = 22
)

//	holds a reference to the cache backend
//
//	TODO: this is a weak implementation right now. it's confusing that
//	the cache backend is a singleton but instances of the Atlas can be
//	instantiated. if cache backends were associated with maps this would
//	be addressed. should Maps have their own cache backends? -arolek
var cacher cache.Interface

type Atlas struct {
	// for managing current access to the map container
	sync.RWMutex
	// hold maps
	maps map[string]Map
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
	return cacher
}

//	SetCache sets the cache backend
func (a *Atlas) SetCache(c cache.Interface) {
	cacher = c
}

//	AllMaps returns all registered maps
func AllMaps() []Map {
	return DefaultAtlas.AllMaps()
}

//	GetMap returns a copy of the a map by name. if the map does not exist it will return an error
func GetMap(mapName string) (Map, error) {
	return DefaultAtlas.Map(mapName)
}

//	AddMap registers a map by name. if the map already exists it will be overwritten
func AddMap(m Map) {
	DefaultAtlas.AddMap(m)
}

//	GetCache returns the registered cache if one is registered, otherwise nil
func GetCache() cache.Interface {
	return cacher
}

//	SetCache sets the cache backend
func SetCache(c cache.Interface) {
	cacher = c
}
