package atlas

import (
	"context"
	"log"
	"strings"
	"sync"

	"github.com/golang/protobuf/proto"

	"github.com/terranodo/tegola"
	"github.com/terranodo/tegola/basic"
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
		Bounds: tegola.WGS84Bounds,
		//	default debug layers
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
		SRID: tegola.WGS84,
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

	SRID int
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

//	EnableLayersByZoom returns layers that that are to be rendered between a min and max zoom
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

//	TODO: support for max zoom
func (m Map) Encode(ctx context.Context, tile *tegola.Tile) ([]byte, error) {
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
			if err != nil {
				switch err {
				case mvt.ErrCanceled:
					//	TODO: add debug logs
				case context.Canceled:
					//	TODO: add debug logs
				default:
					//	TODO: should we return an error to the response or just log the error?
					//	we can't just write to the response as the waitgroup is going to write to the response as well
					log.Printf("Error Getting MVTLayer for tile Z: %v, X: %v, Y: %v: %v", tile.Z, tile.X, tile.Y, err)
				}
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
	vtile, err := mvtTile.VTile(ctx, tile)
	if err != nil {
		return nil, err
	}

	//	encode the tile
	return proto.Marshal(vtile)
}
