package atlas

import (
	"context"
	"log"
	"math"
	"strings"
	"sync"

	"github.com/golang/protobuf/proto"

	"github.com/terranodo/tegola"
	"github.com/terranodo/tegola/basic"
	"github.com/terranodo/tegola/cache"
	"github.com/terranodo/tegola/mvt"
	"github.com/terranodo/tegola/provider/debug"
)

//	NewMap creates a new map with the necessary default values
func NewWGS84Map(name string) Map {
	//	setup a debug provider
	debugProvider, _ := debug.NewProvider(map[string]interface{}{})

	return Map{
		Name: name,
		//	default bounds
		Bounds: [4]float64{-180.0, -85.0511, 180.0, 85.0511},
		Layers: []Layer{
			{
				Name:              debug.LayerDebugTileOutline,
				ProviderLayerName: debug.LayerDebugTileOutline,
				Provider:          debugProvider,
				GeomType:          basic.Line{},
				Disabled:          true,
				MinZoom:           0,
				MaxZoom:           MaxZoom,
			},
			{
				Name:              debug.LayerDebugTileCenter,
				ProviderLayerName: debug.LayerDebugTileCenter,
				Provider:          debugProvider,
				GeomType:          basic.Point{},
				Disabled:          true,
				MinZoom:           0,
				MaxZoom:           MaxZoom,
			},
		},
		TileWidth:  256,
		TileHeight: 256,
		SRID:       tegola.WGS84,
		Resolutions: []float64{
			0.703125000000000,
			0.351562500000000,
			0.175781250000000,
			8.78906250000000e-2,
			4.39453125000000e-2,
			2.19726562500000e-2,
			1.09863281250000e-2,
			5.49316406250000e-3,
			2.74658203125000e-3,
			1.37329101562500e-3,
			6.86645507812500e-4,
			3.43322753906250e-4,
			1.71661376953125e-4,
			8.58306884765625e-5,
			4.29153442382812e-5,
			2.14576721191406e-5,
			1.07288360595703e-5,
			5.36441802978516e-6,
		},
	}
}

func NewWebMercatorMap(name string) Map {
	//	setup a debug provider
	debugProvider, _ := debug.NewProvider(map[string]interface{}{})

	return Map{
		Name:   name,
		Bounds: [4]float64{-20026376.39, -20048966.10, 20026376.39, 20048966.10},
		Layers: []Layer{
			{
				Name:              debug.LayerDebugTileOutline,
				ProviderLayerName: debug.LayerDebugTileOutline,
				Provider:          debugProvider,
				GeomType:          basic.Line{},
				Disabled:          true,
				MinZoom:           0,
				MaxZoom:           MaxZoom,
			},
			{
				Name:              debug.LayerDebugTileCenter,
				ProviderLayerName: debug.LayerDebugTileCenter,
				Provider:          debugProvider,
				GeomType:          basic.Point{},
				Disabled:          true,
				MinZoom:           0,
				MaxZoom:           MaxZoom,
			},
		},
		TileWidth:   256,
		TileHeight:  256,
		SRID:        tegola.WebMercator,
		Resolutions: []float64{},
	}

}

type Map struct {
	Name string
	//	Contains an attribution to be displayed when the map is shown to a user.
	// 	This string is sanatized so it can't be abused as a vector for XSS or beacon tracking.
	Attribution string
	//	The maximum extent of available map tiles in WGS:84
	//	latitude and longitude values, in the order left, bottom, right, top.
	//	Default: [-180, -85, 180, 85]
	Bounds [4]float64
	//	The first value is the longitude, the second is latitude (both in
	//	WGS:84 values), the third value is the zoom level.
	Center [3]float64
	Layers []Layer

	Resolutions []float64

	SRID int

	TileWidth  float64
	TileHeight float64
}

func (m Map) DisableAllLayers() Map {
	//	make an explict copy of the layers
	layers := make([]Layer, len(m.Layers))
	copy(layers, m.Layers)
	m.Layers = layers

	for i := range m.Layers {
		m.Layers[i].Disabled = true
	}

	return m
}

func (m Map) EnableAllLayers() Map {
	//	make an explict copy of the layers
	layers := make([]Layer, len(m.Layers))
	copy(layers, m.Layers)
	m.Layers = layers

	for i := range m.Layers {
		m.Layers[i].Disabled = false
	}

	return m
}

func (m Map) EnableDebugLayers() Map {
	//	make an explict copy of the layers
	layers := make([]Layer, len(m.Layers))
	copy(layers, m.Layers)
	m.Layers = layers

	for i := range m.Layers {
		if m.Layers[i].Name == debug.LayerDebugTileCenter || m.Layers[i].Name == debug.LayerDebugTileOutline {
			m.Layers[i].Disabled = false
		}
	}

	return m
}

func (m Map) DisableDebugLayers() Map {
	//	make an explict copy of the layers
	layers := make([]Layer, len(m.Layers))
	copy(layers, m.Layers)
	m.Layers = layers

	for i := range m.Layers {
		if m.Layers[i].Name == debug.LayerDebugTileCenter || m.Layers[i].Name == debug.LayerDebugTileOutline {
			m.Layers[i].Disabled = true
		}
	}

	return m
}

//	FilterByZoom returns layers that that are to be rendered between a min and max zoom
func (m Map) EnableLayersByZoom(zoom int) Map {
	//	make an explict copy of the layers
	layers := make([]Layer, len(m.Layers))
	copy(layers, m.Layers)
	m.Layers = layers

	for i := range m.Layers {
		if (m.Layers[i].MinZoom <= zoom || m.Layers[i].MinZoom == 0) && (m.Layers[i].MaxZoom >= zoom || m.Layers[i].MaxZoom == 0) {
			m.Layers[i].Disabled = false
			continue
		}

		m.Layers[i].Disabled = true
	}

	return m
}

//	EnableLayersByName will enable layers that match the provided layer names
//	this method will not disable layers
func (m Map) EnableLayersByName(names ...string) Map {

	//	make an explict copy of the layers
	layers := make([]Layer, len(m.Layers))
	copy(layers, m.Layers)
	m.Layers = layers

	nameStr := strings.Join(names, ",")
	for i := range m.Layers {
		//	if we have a name set, use it for the lookup
		if m.Layers[i].Name != "" && strings.Contains(nameStr, m.Layers[i].Name) {
			m.Layers[i].Disabled = false
			continue
		} else if m.Layers[i].ProviderLayerName != "" && strings.Contains(nameStr, m.Layers[i].ProviderLayerName) { //	default to using the ProviderLayerName for the lookup
			m.Layers[i].Disabled = false
			continue
		}
	}

	return m
}

func (m Map) SeedTile(tile tegola.Tile) error {
	b, err := m.Encode(context.Background(), tile)
	if err != nil {
		return err
	}

	//	TODO: should we support cache on individual maps?
	//	TODO: the DefaultAtlas is not necessarly the correct instance to fetch the cache from
	c := DefaultAtlas.GetCache()
	if c == nil {
		return ErrMissingCache
	}

	//	cache key
	key := cache.Key{
		MapName: m.Name,
		Z:       tile.Z,
		X:       tile.X,
		Y:       tile.Y,
	}

	return c.Set(&key, b)
}

func (m Map) PurgeTile(tile tegola.Tile) error {
	//	TODO: the DefaultAtlas is not necessarly the correct instance to fetch the cache from
	c := DefaultAtlas.GetCache()
	if c == nil {
		return ErrMissingCache
	}

	//	cache key
	key := cache.Key{
		MapName: m.Name,
		Z:       tile.Z,
		X:       tile.X,
		Y:       tile.Y,
	}

	return c.Purge(&key)
}

//	TODO: support for max zoom
func (m Map) Encode(ctx context.Context, tile tegola.Tile) ([]byte, error) {
	//	generate a tile
	var mvtTile mvt.Tile
	//	wait group for concurrent layer fetching
	var wg sync.WaitGroup

	//	layer stack
	mvtLayers := make([]*mvt.Layer, len(m.Layers))

	//	set our waitgroup count
	wg.Add(len(m.Layers))

	//	iterate our layers
	for i, layer := range m.Layers {
		// check if the label is disabled
		if layer.Disabled {
			wg.Done()
			continue
		}

		//	go routine for fetching the layer concurrently
		go func(i int, l Layer) {
			//	on completion let the wait group know
			defer wg.Done()

			//	fetch layer from data provider
			mvtLayer, err := l.Provider.MVTLayer(ctx, l.ProviderLayerName, tile, l.DefaultTags)
			if err == mvt.ErrCanceled {
				return
			}
			if err != nil {
				//	TODO: should we return an error to the response or just log the error?
				//	we can't just write to the response as the waitgroup is going to write to the response as well
				log.Printf("Error Getting MVTLayer for tile Z: %v, X: %v, Y: %v: %v", tile.Z, tile.X, tile.Y, err)
				return
			}

			//	check if we have a layer name
			if l.Name != "" {
				mvtLayer.Name = l.Name
			}

			//	add the layer to the slice position
			mvtLayers[i] = mvtLayer
		}(i, layer)
	}

	//	wait for the waitgroup to finish
	wg.Wait()

	//	stop processing if the context has an error. this check is necessary
	//	otherwise the server continues processing even if the request was canceled
	//	as the waitgroup was not notified of the cancel
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	//	add layers to our tile
	mvtTile.AddLayers(mvtLayers...)

	//	generate our tile
	vtile, err := mvtTile.VTile(ctx, tile.BoundingBox())
	if err != nil {
		return nil, err
	}

	//	encode the tile
	return proto.Marshal(vtile)
}

//	credit: TileCache
//	https://github.com/OSGeo/tilecache/
func (m *Map) ClosestCell(zoom int, minx, miny float64) (z, x, y int) {
	res := m.Resolutions[zoom]

	maxx := minx + m.TileWidth*res
	maxy := miny + m.TileHeight*res

	return m.Cell(minx, miny, maxx, maxy)
}

func (m *Map) Cell(minx, miny, maxx, maxy float64) (z, x, y int) {

	res := m.Resolution(minx, miny, maxx, maxy)
	z = m.ClosestLevel(res)
	res = m.Resolutions[z]

	x0 := (minx - m.Bounds[0]) / (res * m.TileWidth)
	y0 := (miny - m.Bounds[1]) / (res * m.TileHeight)

	x = round(x0)
	y = round(y0)

	return
}

func (m *Map) Resolution(minx, miny, maxx, maxy float64) float64 {
	v1 := (maxx - minx) / m.TileWidth
	v2 := (maxy - miny) / m.TileHeight

	return math.Max(v1, v2)
}

//	ClosestLevel find the closest zoom from a provided resolution
func (m *Map) ClosestLevel(res float64) int {
	var z int
	diff := math.MaxFloat64

	for i := 0; i <= MaxZoom; i++ {
		r := math.Abs(m.Resolutions[i] - res)

		if diff > r {
			diff = r
			z = i
			continue
		}
		break
	}

	return z
}

func (m *Map) Level(res float64) int {
	var z int
	maxDiff := res / math.Max(m.TileWidth, m.TileHeight)

	for i := 0; i <= MaxZoom; i++ {
		r := math.Abs(m.Resolutions[i] - res)

		if r < maxDiff {
			res = m.Resolutions[i]
			z = i
			continue
		}
		break
	}

	return z
}

func round(f float64) int {
	if f < -0.5 {
		return int(f - 0.5)
	}
	if f > 0.5 {
		return int(f + 0.5)
	}
	return 0
}
