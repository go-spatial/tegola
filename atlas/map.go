package atlas

import (
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"log"
	"strings"
	"sync"

	"github.com/golang/protobuf/proto"

	"github.com/go-spatial/geom"
	"github.com/go-spatial/geom/encoding/mvt"
	"github.com/go-spatial/geom/planar/makevalid"
	"github.com/go-spatial/geom/planar/makevalid/hitmap"
	"github.com/go-spatial/geom/slippy"
	"github.com/go-spatial/tegola"
	"github.com/go-spatial/tegola/basic"
	"github.com/go-spatial/tegola/dict"
	"github.com/go-spatial/tegola/internal/convert"
	"github.com/go-spatial/tegola/maths/simplify"
	"github.com/go-spatial/tegola/provider"
	"github.com/go-spatial/tegola/provider/debug"
)

// NewMap creates a new map with the necessary default values
func NewWebMercatorMap(name string) Map {
	return Map{
		Name: name,
		// default bounds
		Bounds:     tegola.WGS84Bounds,
		Layers:     []Layer{},
		SRID:       tegola.WebMercator,
		TileExtent: 4096,
		TileBuffer: uint64(tegola.DefaultTileBuffer),
	}
}

type Map struct {
	Name string
	// Contains an attribution to be displayed when the map is shown to a user.
	// 	This string is sanatized so it can't be abused as a vector for XSS or beacon tracking.
	Attribution string
	// The maximum extent of available map tiles in WGS:84
	// latitude and longitude values, in the order left, bottom, right, top.
	// Default: [-180, -85, 180, 85]
	Bounds *geom.Extent
	// The first value is the longitude, the second is latitude (both in
	// WGS:84 values), the third value is the zoom level.
	Center [3]float64
	Layers []Layer

	SRID uint64
	// MVT output values
	TileExtent uint64
	TileBuffer uint64
}

// AddDebugLayers returns a copy of a Map with the debug layers appended to the layer list
func (m Map) AddDebugLayers() Map {
	// make an explicit copy of the layers
	layers := make([]Layer, len(m.Layers))
	copy(layers, m.Layers)
	m.Layers = layers

	// setup a debug provider
	debugProvider, _ := debug.NewTileProvider(dict.Dict{})

	m.Layers = append(layers, []Layer{
		{
			Name:              debug.LayerDebugTileOutline,
			ProviderLayerName: debug.LayerDebugTileOutline,
			Provider:          debugProvider,
			GeomType:          geom.LineString{},
			MinZoom:           0,
			MaxZoom:           MaxZoom,
		},
		{
			Name:              debug.LayerDebugTileCenter,
			ProviderLayerName: debug.LayerDebugTileCenter,
			Provider:          debugProvider,
			GeomType:          geom.Point{},
			MinZoom:           0,
			MaxZoom:           MaxZoom,
		},
	}...)

	return m
}

// FilterLayersByZoom returns a copy of a Map with a subset of layers that match the given zoom
func (m Map) FilterLayersByZoom(zoom uint) Map {
	var layers []Layer

	for i := range m.Layers {
		if (m.Layers[i].MinZoom <= zoom || m.Layers[i].MinZoom == 0) && (m.Layers[i].MaxZoom >= zoom || m.Layers[i].MaxZoom == 0) {
			layers = append(layers, m.Layers[i])
			continue
		}
	}

	// overwrite the Map's layers with our subset
	m.Layers = layers

	return m
}

// FilterLayersByName returns a copy of a Map with a subset of layers that match the supplied list of layer names
func (m Map) FilterLayersByName(names ...string) Map {
	var layers []Layer

	nameStr := strings.Join(names, ",")
	for i := range m.Layers {
		// if we have a name set, use it for the lookup
		if m.Layers[i].Name != "" && strings.Contains(nameStr, m.Layers[i].Name) {
			layers = append(layers, m.Layers[i])
			continue
		} else if m.Layers[i].ProviderLayerName != "" && strings.Contains(nameStr, m.Layers[i].ProviderLayerName) { // default to using the ProviderLayerName for the lookup
			layers = append(layers, m.Layers[i])
			continue
		}
	}

	// overwrite the Map's layers with our subset
	m.Layers = layers

	return m
}

// TODO (arolek): support for max zoom
func (m Map) Encode(ctx context.Context, tile *slippy.Tile) ([]byte, error) {
	// tile container
	var mvtTile mvt.Tile
	// wait group for concurrent layer fetching
	var wg sync.WaitGroup

	// layer stack
	mvtLayers := make([]*mvt.Layer, len(m.Layers))

	// set our waitgroup count
	wg.Add(len(m.Layers))

	// iterate our layers
	for i, layer := range m.Layers {

		// go routine for fetching the layer concurrently
		go func(i int, l Layer) {
			mvtLayer := mvt.Layer{
				Name: l.MVTName(),
			}

			// on completion let the wait group know
			defer wg.Done()

			ptile := provider.NewTile(tile.Z, tile.X, tile.Y,
				uint(m.TileBuffer), uint(m.SRID))

			// fetch layer from data provider
			err := l.Provider.TileFeatures(ctx, l.ProviderLayerName, ptile, func(f *provider.Feature) error {
				// skip row if geometry collection empty.
				g, ok := f.Geometry.(geom.Collection)
				if ok && len(g.Geometries()) == 0 {
					return nil
				}

				geo := f.Geometry

				// check if the feature SRID and map SRID are different. If they are then reporject
				if f.SRID != m.SRID {

					// TODO(arolek): support for additional projections
					g, err := basic.ToWebMercator(f.SRID, geo)
					if err != nil {
						return fmt.Errorf("unable to transform geometry to webmercator from SRID (%v) for feature %v due to error: %v", f.SRID, f.ID, err)
					}
					geo = g

				}

				// add default tags, but don't overwrite a tag that already exists
				for k, v := range l.DefaultTags {
					if _, ok := f.Tags[k]; !ok {
						f.Tags[k] = v
					}
				}

				// multiple ways to turn off simplification. check the atlas init() function
				// for how the second two conditions are set
				if !l.DontSimplify && simplifyGeometries && tile.Z < simplificationMaxZoom {

					// TODO: remove this geom conversion step once the simplify function uses geom types
					tegolaGeo, err := convert.ToTegola(geo)
					if err != nil {
						return err
					}

					// TODO (arolek): change out the tile type for VTile. tegola.Tile will be deprecated
					tegolaTile := tegola.NewTile(tile.ZXY())

					sg := simplify.SimplifyGeometry(tegolaGeo, tegolaTile.ZEpislon())

					// TODO: remove this geom conversion step once the simplify function uses geom types
					geo, err = convert.ToGeom(sg)
					if err != nil {
						return err
					}
				}

				// check if we need to clip and if we do build the clip region (tile extent)
				var clipRegion *geom.Extent
				if !l.DontClip {
					clipRegion = tile.Extent3857().ExpandBy(m.TileBuffer)
				}

				// create a hitmap for the makevalid function
				hm, err := hitmap.New(clipRegion, geo)
				if err != nil {
					return err
				}

				// instantiate a new makevalid struct holding the hitmap
				mv := makevalid.Makevalid{
					Hitmap: hm,
				}

				// apply make valid routine
				geo, _, err = mv.Makevalid(ctx, geo, clipRegion)
				if err != nil {
					return err
				}

				// tranlate the geometry to tile coordinates
				geo = mvt.PrepareGeo(geo, tile.Extent3857(), float64(m.TileExtent))
				if geo == nil {
					return nil
				}

				mvtLayer.AddFeatures(mvt.Feature{
					ID:       &f.ID,
					Tags:     f.Tags,
					Geometry: geo,
				})

				return nil
			})
			if err != nil {
				switch err {
				case context.Canceled:
					// TODO (arolek): add debug logs
				default:
					z, x, y := tile.ZXY()
					// TODO (arolek): should we return an error to the response or just log the error?
					// we can't just write to the response as the waitgroup is going to write to the response as well
					log.Printf("err fetching tile (z: %v, x: %v, y: %v) features: %v", z, x, y, err)
				}
				return
			}

			// add the layer to the slice position
			mvtLayers[i] = &mvtLayer
		}(i, layer)
	}

	// wait for the waitgroup to finish
	wg.Wait()

	// stop processing if the context has an error. this check is necessary
	// otherwise the server continues processing even if the request was canceled
	// as the waitgroup was not notified of the cancel
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	// add layers to our tile
	mvtTile.AddLayers(mvtLayers...)

	// generate the MVT tile
	vtile, err := mvtTile.VTile(ctx)
	if err != nil {
		return nil, err
	}

	// encode our mvt tile
	tileBytes, err := proto.Marshal(vtile)
	if err != nil {
		return nil, err
	}

	// buffer to store our compressed bytes
	var gzipBuf bytes.Buffer

	// compress the encoded bytes
	w := gzip.NewWriter(&gzipBuf)
	_, err = w.Write(tileBytes)
	if err != nil {
		return nil, err
	}

	// flush and close the writer
	if err = w.Close(); err != nil {
		return nil, err
	}

	// return encoded, gzipped tile
	return gzipBuf.Bytes(), nil
}
