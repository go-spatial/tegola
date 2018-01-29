package server_test

import (
	"context"
	"sync"

	"github.com/terranodo/tegola"
	"github.com/terranodo/tegola/atlas"
	"github.com/terranodo/tegola/cache"
	"github.com/terranodo/tegola/geom"
	"github.com/terranodo/tegola/provider"
	"github.com/terranodo/tegola/server"
)

//	test server config
const (
	httpPort       = ":8080"
	serverVersion  = "0.4.0"
	serverHostName = "tegola.io"
)

var (
	testMapName        = "test-map"
	testMapAttribution = "test attribution"
	testMapCenter      = [3]float64{1.0, 2.0, 3.0}
)

type testTileProvider struct{}

func (tp *testTileProvider) TileFeatures(ctx context.Context, layer string, t provider.Tile, fn func(f *provider.Feature) error) error {
	// TODO: return test features

	return nil
}

func (tp *testTileProvider) Layers() ([]provider.LayerInfo, error) {
	return []provider.LayerInfo{
		layer{
			name:     "test-layer",
			geomType: geom.Polygon{},
			srid:     tegola.WebMercator,
		},
	}, nil
}

var testLayer1 = atlas.Layer{
	Name:              "test-layer",
	ProviderLayerName: "test-layer-1",
	MinZoom:           4,
	MaxZoom:           9,
	Provider:          &testTileProvider{},
	GeomType:          geom.Point{},
	DefaultTags: map[string]interface{}{
		"foo": "bar",
	},
}

var testLayer2 = atlas.Layer{
	Name:              "test-layer-2-name",
	ProviderLayerName: "test-layer-2-provider-layer-name",
	MinZoom:           10,
	MaxZoom:           20,
	Provider:          &testTileProvider{},
	GeomType:          geom.Line{},
	DefaultTags: map[string]interface{}{
		"foo": "bar",
	},
}

var testLayer3 = atlas.Layer{
	Name:              "test-layer",
	ProviderLayerName: "test-layer-3",
	MinZoom:           10,
	MaxZoom:           20,
	Provider:          &testTileProvider{},
	GeomType:          geom.Point{},
	DefaultTags:       map[string]interface{}{},
}

type layer struct {
	name     string
	geomType geom.Geometry
	srid     int
}

func (l layer) Name() string {
	return l.name
}

func (l layer) GeomType() geom.Geometry {
	return l.geomType
}

func (l layer) SRID() int {
	return l.srid
}

func NewMemoryCache() *MemoryCache {
	return &MemoryCache{
		keyVals: map[string][]byte{},
	}
}

//	test cacher, implements the cache.Interface
type MemoryCache struct {
	keyVals map[string][]byte
	sync.RWMutex
}

func (mc *MemoryCache) Get(key *cache.Key) ([]byte, bool, error) {
	mc.RLock()
	defer mc.RUnlock()

	val, ok := mc.keyVals[key.String()]
	if !ok {
		return nil, false, nil
	}

	return val, true, nil
}

func (mc *MemoryCache) Set(key *cache.Key, val []byte) error {
	mc.Lock()
	defer mc.Unlock()

	mc.keyVals[key.String()] = val

	return nil
}

func (mc *MemoryCache) Purge(key *cache.Key) error {
	mc.Lock()
	defer mc.Unlock()

	delete(mc.keyVals, key.String())

	return nil
}

//	pre test setup phase
func init() {
	server.Version = serverVersion
	server.HostName = serverHostName

	testMap := atlas.NewWGS84Map(testMapName)
	testMap.Attribution = testMapAttribution
	testMap.Center = testMapCenter
	testMap.Layers = append(testMap.Layers, []atlas.Layer{
		testLayer1,
		testLayer2,
		testLayer3,
	}...)

	atlas.SetCache(NewMemoryCache())

	//	register a map with atlas
	atlas.AddMap(testMap)

	server.Atlas = atlas.DefaultAtlas
}
