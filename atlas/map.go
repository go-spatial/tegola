package atlas

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"

	"github.com/golang/protobuf/proto"

	"github.com/go-spatial/geom"
	"github.com/go-spatial/geom/encoding/mvt"
	"github.com/go-spatial/geom/encoding/wkb"
	"github.com/go-spatial/geom/encoding/wkt"
	"github.com/go-spatial/geom/planar"
	"github.com/go-spatial/geom/planar/clip"
	"github.com/go-spatial/geom/planar/makevalid"
	"github.com/go-spatial/geom/planar/makevalid/hitmap"
	"github.com/go-spatial/geom/planar/simplify"
	"github.com/go-spatial/geom/slippy"
	"github.com/go-spatial/tegola"
	"github.com/go-spatial/tegola/basic"
	"github.com/go-spatial/tegola/dict"
	"github.com/go-spatial/tegola/mvtprovider"
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
		TileExtent: uint64(mvt.DefaultExtent),
		TileBuffer: uint64(tegola.DefaultTileBuffer),
	}
}

type Map struct {
	Name string
	// Contains an attribution to be displayed when the map is shown to a user.
	// 	This string is sanitized so it can't be abused as a vector for XSS or beacon tracking.
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

	mvtProviderName string
	mvtProvider     mvtprovider.Tiler
}

// HasMVTProvider indicates if map is a mvt provider based map
func (m Map) HasMVTProvider() bool { return m.mvtProvider != nil }

// MVTProvider returns the mvt provider if this map is a mvt provider based map, otherwise nil
func (m Map) MVTProvider() mvtprovider.Tiler { return m.mvtProvider }

// MVTProviderName returns the mvt provider name if this map is a mvt provider based map, otherwise ""
func (m Map) MVTProviderName() string { return m.mvtProviderName }

// SetMVTProvder sets the map to be based on the passed in mvt provider, and returning the provider
func (m *Map) SetMVTProvider(name string, p mvtprovider.Tiler) mvtprovider.Tiler {
	m.mvtProviderName = name
	m.mvtProvider = p
	return p
}

// AddDebugLayers returns a copy of a Map with the debug layers appended to the layer list
func (m Map) AddDebugLayers() Map {
	// can not modify the layers of an mvt provider based map
	if m.mvtProvider != nil {
		return m
	}

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

func (m Map) encodeMVTProviderTile(ctx context.Context, tile *slippy.Tile) ([]byte, error) {
	// get the list of our layers
	ptile := provider.NewTile(tile.Z, tile.X, tile.Y, uint(m.TileBuffer), uint(m.SRID))

	layers := make([]mvtprovider.Layer, len(m.Layers))
	for i := range m.Layers {
		layers[i] = mvtprovider.Layer{
			Name:    m.Layers[i].ProviderLayerName,
			MVTName: m.Layers[i].MVTName(),
		}
	}
	return m.mvtProvider.MVTForLayers(ctx, ptile, layers)

}

// encodeMVTTile will encode the given tile into mvt format
// TODO (arolek): support for max zoom
func (m Map) encodeMVTTile(ctx context.Context, tile *slippy.Tile) ([]byte, error) {

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
						return fmt.Errorf("unable to transform geometry to webmercator from SRID (%v) for feature %v due to error: %w", f.SRID, f.ID, err)
					}
					geo = g

				}

				// add default tags, but don't overwrite a tag that already exists
				for k, v := range l.DefaultTags {
					if _, ok := f.Tags[k]; !ok {
						f.Tags[k] = v
					}
				}

				defer func() {
					if r := recover(); r != nil {
						log.Println("geometry:")
						fname := fmt.Sprintf("panic_geo_%s_%d_%d_%d", l.MVTName(), tile.Z, tile.X, tile.Y)
						file, err := os.OpenFile(fname+".wkt", os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
						if err == nil {
							err = wkt.Encode(file, geo)
							if err != nil {
								log.Println("ERROR WRITING panic_dump", err)
							}
							file.Close()
						}

						file, err = os.OpenFile(fname+".wkb", os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
						if err == nil {
							err = wkb.Encode(hex.NewEncoder(file), geo)
							if err != nil {
								log.Println("ERROR WRITING panic_dump", err)
							}
							file.Close()
						}
						// panic(r)
					}
				}()

				// multiple ways to turn off simplification. check the atlas init() function
				// for how the second two conditions are set
				if !l.DontSimplify && simplifyGeometries && tile.Z < simplificationMaxZoom {
					simp := simplify.DouglasPeucker{
						Tolerance: slippy.Pixels2Webs(tile.Z, tegola.DefaultEpsilon),
					}

					var err error
					geo, err = planar.Simplify(ctx, simp, geo)
					if err != nil {
						return err
					}
				}

				// check if we need to clip and if we do build the clip region (tile extent)
				var clipRegion *geom.Extent
				if !l.DontClip {
					webs := slippy.Pixels2Webs(tile.Z, uint(m.TileBuffer))
					clipRegion = tile.Extent3857().ExpandBy(webs)
				}

				// create a hitmap for the makevalid function
				hm, err := hitmap.New(clipRegion, geo)
				if err != nil {
					return err
				}

				// instantiate a new makevalid struct holding the hitmap
				mv := makevalid.Makevalid{
					Hitmap:  hm,
					Clipper: clip.Default,
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
				switch {
				case errors.Is(err, context.Canceled):
					// Do nothing if we were cancelled.

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
	return proto.Marshal(vtile)
}

// Encode will encode the given tile into mvt format
func (m Map) Encode(ctx context.Context, tile *slippy.Tile) ([]byte, error) {
	var (
		tileBytes []byte
		err       error
	)
	if m.HasMVTProvider() {
		tileBytes, err = m.encodeMVTProviderTile(ctx, tile)
	} else {
		tileBytes, err = m.encodeMVTTile(ctx, tile)
	}
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
