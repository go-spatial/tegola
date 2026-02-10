package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"sync"

	"github.com/dimfeld/httptreemux"

	"github.com/go-spatial/geom"
	"github.com/go-spatial/tegola"
	"github.com/go-spatial/tegola/atlas"
	"github.com/go-spatial/tegola/internal/log"
	"github.com/go-spatial/tegola/mapbox/tilejson"
	"github.com/go-spatial/tegola/provider"
)

var capabilitiesCache sync.Map

type cacheEntry struct {
	once     sync.Once
	tileJSON tilejson.TileJSON
	err      error
}

type HandleMapCapabilities struct {
	// function to retrieve a map, defaults to atlas.GetMap if nil
	// required because of our defaultAtlas
	GetMap func(string) (atlas.Map, error)
	// required
	mapName string
	// the requests extension defaults to "json"
	extension string
}

// ServeHTTP returns details about a map according to the
// tileJSON spec (https://github.com/mapbox/tilejson-spec/tree/master/3.0.0)
//
// URI scheme: /capabilities/:map_name.json
// map_name - map name in the config file
func (req HandleMapCapabilities) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	params := httptreemux.ContextParams(r.Context())
	mapName := params["map_name"]
	mapNameParts := strings.Split(mapName, ".")
	req.mapName = mapNameParts[0]

	// check if we have a provided extension
	if len(mapNameParts) > 2 {
		req.extension = mapNameParts[len(mapNameParts)-1]
	} else {
		req.extension = "json"
	}

	cacheKey := req.mapName + ":" + URLRoot(r).String() + ":" + r.URL.Query().Encode()
	value, _ := capabilitiesCache.LoadOrStore(cacheKey, &cacheEntry{})
	entry, ok := value.(*cacheEntry)
	if !ok || entry == nil {
		http.Error(w, "internal cache error", http.StatusInternalServerError)
		log.Errorf("cache entry for map (%v) is invalid", req.mapName)
	}
	entry.once.Do(func() {
		entry.tileJSON, entry.err = req.buildTileJSON(r)
	})
	if entry.err != nil {
		capabilitiesCache.Delete(cacheKey) // remove failed entry to enable a retry
		log.Errorf("error building tilejson: %v", entry.err)
		http.Error(w, "error building map capabilities", http.StatusInternalServerError)
		return
	}

	w.Header().Add("Content-Type", "application/json")

	// cache control headers (no-cache)
	w.Header().Add("Cache-Control", "no-cache, no-store, must-revalidate")
	w.Header().Add("Pragma", "no-cache")
	w.Header().Add("Expires", "0")

	if err := json.NewEncoder(w).Encode(entry.tileJSON); err != nil {
		log.Errorf("error encoding tileJSON for map (%v)", req.mapName)
	}
}

// buildTileJSON is a helper to declutter the ServeHTTP method.
func (req HandleMapCapabilities) buildTileJSON(r *http.Request) (tilejson.TileJSON, error) {
	getMap := req.GetMap
	if getMap == nil {
		getMap = atlas.GetMap
	}

	m, err := getMap(req.mapName)
	if err != nil {
		return tilejson.TileJSON{}, err
	}

	// determine TileJSON version based on whether ALL providers implement LayerFielder.
	// we only use TileJSON 3.0.0 if every provider can supply field metadata to ensure
	// consistent capabilities across all layers. If a provider lacks LayerFielder support,
	// we fall back to TileJSON 2.0.0 to maintain predictable behavior for clients.
	// this prevents mixed scenarios where some layers have field metadata and others don't,
	// which would make it difficult for clients to reliably work with field information.
	// NOTE: to be discussed
	hasLayerFielder := true
	for i := range m.Layers {
		if _, ok := m.Layers[i].Provider.(provider.LayerFielder); !ok {
			hasLayerFielder = false
			break
		}
	}

	tileJSONVersion := tilejson.Version2
	if hasLayerFielder {
		tileJSONVersion = tilejson.Version3
	}

	tileJSON := tilejson.TileJSON{
		Attribution:  &m.Attribution,
		Bounds:       m.Bounds.Extent(),
		Center:       m.Center,
		Format:       TileURLFileFormat,
		Name:         &m.Name,
		Scheme:       tilejson.SchemeXYZ,
		TileJSON:     tileJSONVersion,
		Version:      "1.0.0",
		Grids:        make([]string, 0),
		Data:         make([]string, 0),
		VectorLayers: []tilejson.VectorLayer{},
	}

	query := r.URL.Query()
	debugQuery := url.Values{}
	if query.Get(QueryKeyDebug) == "true" {
		debugQuery.Set(QueryKeyDebug, "true")
		m = m.AddDebugLayers()
	}

	for i := range m.Layers {
		mvtName := m.Layers[i].MVTName()

		// now check if we already have a VectorLayer with this ID as
		// multple config layers can map to the same mvt layer name
		if idx, found := findVectorLayerByID(tileJSON.VectorLayers, mvtName); found {
			if tileJSON.VectorLayers[idx].MinZoom > m.Layers[i].MinZoom {
				tileJSON.VectorLayers[idx].MinZoom = m.Layers[i].MinZoom
			}
			if tileJSON.VectorLayers[idx].MaxZoom < m.Layers[i].MaxZoom {
				tileJSON.VectorLayers[idx].MaxZoom = m.Layers[i].MaxZoom
			}

			// update map level zoom range
			if tileJSON.MinZoom > m.Layers[i].MinZoom {
				tileJSON.MinZoom = m.Layers[i].MinZoom
			}
			if tileJSON.MaxZoom < m.Layers[i].MaxZoom {
				tileJSON.MaxZoom = m.Layers[i].MaxZoom
			}

			continue
		}

		// first layer sets the initial map zoom level
		if len(tileJSON.VectorLayers) == 0 {
			tileJSON.MinZoom = m.Layers[i].MinZoom
			tileJSON.MaxZoom = m.Layers[i].MaxZoom
		} else {
			// update map zoom level for subsequent layer
			if tileJSON.MinZoom > m.Layers[i].MinZoom {
				tileJSON.MinZoom = m.Layers[i].MinZoom
			}
			if tileJSON.MaxZoom < m.Layers[i].MaxZoom {
				tileJSON.MaxZoom = m.Layers[i].MaxZoom
			}
		}

		layer := tilejson.VectorLayer{
			Version: 2,
			Extent:  tegola.DefaultExtent,
			ID:      mvtName,
			Name:    mvtName,
			MinZoom: m.Layers[i].MinZoom,
			MaxZoom: m.Layers[i].MaxZoom,
			Tiles: []string{
				TileURLTemplate{
					Scheme:     scheme(r),
					Host:       hostName(r).Host,
					PathPrefix: URIPrefix,
					MapName:    req.mapName,
					LayerName:  mvtName,
					Query:      debugQuery,
				}.String(),
			},
		}

		// always initialize Fields for all layers regardless of TileJSON version.
		// in TileJSON 3.0.0, fields is REQUIRED (must be present, even if empty).
		// in TileJSON 2.0.0, fields was already optional and harmless to include.
		// this ensures spec compliance in both cases.
		layer.Fields = make(map[string]interface{})

		// try to populate field information from the provider if it supports LayerFielder
		// ony providers that implement LayerFielder will have their fields populated
		if hasLayerFielder {
			if fielder, ok := m.Layers[i].Provider.(provider.LayerFielder); ok {
				// NOTE: we explicitly use a new context here to avoid a cancelled request context
				// to result in an initiallized, but emtpy TileJSON cache entry - think sync.Once.
				// even if request is cancelled we finish the work for the next requesting client.
				if fields, err := fielder.LayerFields(
					context.Background(), m.Layers[i].ProviderLayerName,
				); err == nil {
					layer.Fields = fields
				} else {
					log.Debugf("error getting fields for layer (%v): %v", mvtName, err)
				}
			}
		}

		// set geometry type
		switch m.Layers[i].GeomType.(type) {
		case geom.Point, geom.MultiPoint:
			layer.GeometryType = tilejson.GeomTypePoint
		case geom.Line, geom.LineString, geom.MultiLineString:
			layer.GeometryType = tilejson.GeomTypeLine
		case geom.Polygon, geom.MultiPolygon:
			layer.GeometryType = tilejson.GeomTypePolygon
		default:
			layer.GeometryType = tilejson.GeomTypeUnknown
		}

		tileJSON.VectorLayers = append(tileJSON.VectorLayers, layer)
	}

	tileURL := TileURLTemplate{
		Scheme:     scheme(r),
		Host:       hostName(r).Host,
		PathPrefix: URIPrefix,
		MapName:    req.mapName,
		Query:      debugQuery,
	}.String()

	// build our URL scheme for the tile grid
	tileJSON.Tiles = append(tileJSON.Tiles, tileURL)

	return tileJSON, nil
}

// findVectorLayerByID searches for a VectorLayer with the given ID
// Returns the index and true if found, -1 and false otherwise
func findVectorLayerByID(layers []tilejson.VectorLayer, id string) (int, bool) {
	for i := range layers {
		if layers[i].ID == id {
			return i, true
		}
	}
	return -1, false
}
