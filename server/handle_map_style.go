package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"

	"github.com/alexeydott/geom"
	"github.com/dimfeld/httptreemux"
	"gopkg.in/go-playground/colors.v1"

	"github.com/alexeydott/tegola/atlas"
	"github.com/alexeydott/tegola/internal/log"
	"github.com/alexeydott/tegola/mapbox/style"
)

type HandleMapStyle struct {
	// required
	mapName string
}

// returns details about a map according to the
// tileJSON spec (https://github.com/mapbox/tilejson-spec/tree/master/2.1.0)
//
// URI scheme: /capabilities/:map_name.json
//
//	map_name - map name in the config file
func (req HandleMapStyle) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var err error

	params := httptreemux.ContextParams(r.Context())

	// read the map_name value from the request
	mapName := params["map_name"]
	mapNameParts := strings.Split(mapName, ".")

	req.mapName = mapNameParts[0]

	// lookup our Map
	m, err := atlas.GetMap(req.mapName)
	if err != nil {
		log.Errorf("map (%v) not configured. check your config file", req.mapName)
		http.Error(w, "map ("+req.mapName+") not configured. check your config file", http.StatusNotFound)
		return
	}

	// if we have a debug param add it to our URLs
	debugQuery := url.Values{}
	if r.URL.Query().Get(QueryKeyDebug) == "true" {
		debugQuery.Set(QueryKeyDebug, "true")

		// update our map to include the debug layers
		m = m.AddDebugLayers()
	}

	mapboxStyle := style.Root{
		Name:    m.Name,
		Version: style.Version,
		Center:  [2]float64{m.Center[0], m.Center[1]},
		Zoom:    m.Center[2],
		Sources: map[string]style.Source{
			req.mapName: {
				Type: style.SourceTypeVector,
				URL: (&url.URL{
					Scheme:   scheme(r),
					Host:     hostName(r).Host,
					Path:     path.Join(URIPrefix, "capabilities", req.mapName+".json"),
					RawQuery: debugQuery.Encode(),
				}).String(),
			},
		},
		Layers: []style.Layer{},
	}
	usedLayerIDs := make(map[string]bool, len(m.Layers))
	for _, l := range m.Layers {
		usedLayerIDs[l.MVTName()] = true
	}
	emittedLayerNames := make(map[string]bool, len(m.Layers))

	// determining the min and max zoom for this map
	for _, l := range m.Layers {
		// check if the layer already exists in our slice. this can happen if the config
		// is using the "name" param for a layer to override the providerLayerName
		skip := emittedLayerNames[l.MVTName()]
		// entry for layer already exists. move on
		if skip {
			continue
		}

		// build our vector layer details
		layer := style.Layer{
			ID:          l.MVTName(),
			Source:      req.mapName,
			SourceLayer: l.MVTName(),
			Layout: &style.LayerLayout{
				Visibility: style.LayoutVisible,
			},
		}

		// chose our paint type based on the geometry type
		switch l.GeomType.(type) {
		case geom.Point, geom.MultiPoint:
			layer.Type = style.LayerTypeCircle
			layer.Paint = &style.LayerPaint{
				CircleRadius: 3,
				CircleColor:  stringToColorHex(l.MVTName()),
			}
		case geom.Line, geom.LineString, geom.MultiLineString:
			layer.Type = style.LayerTypeLine
			layer.Paint = &style.LayerPaint{
				LineColor: stringToColorHex(l.MVTName()),
			}
		case geom.Polygon, geom.MultiPolygon:
			layer.Type = style.LayerTypeFill
			layer.Paint = polygonPaint(l.MVTName())
		case geom.Collection:
			for _, collectionLayer := range collectionStyleLayers(req.mapName, l.MVTName()) {
				collectionLayer.ID = uniqueStyleLayerID(collectionLayer.ID, usedLayerIDs)
				usedLayerIDs[collectionLayer.ID] = true
				mapboxStyle.Layers = append(mapboxStyle.Layers, collectionLayer)
			}
			emittedLayerNames[l.MVTName()] = true
			continue
		default:
			log.Infof("unable to infer geometry type for providerLayerName: %v. style definition not generated", l.ProviderLayerName)
			continue
		}

		// A single provider layer can serve mixed geometry types (e.g. MOS
		// tables hold polygons and polylines in the same column). MapLibre
		// silently closes any LineString rendered by a fill layer, flooding
		// lines as polygons, so fill layers are restricted to polygons.
		if layer.Type == style.LayerTypeFill {
			layer.Filter = []interface{}{"==", "$type", "Polygon"}
		}

		if layer.Type == style.LayerTypeFill {
			// Add a sibling line layer for polylines served by the same
			// source layer. Keep the established ordering (line before fill)
			// so the generated style remains backwards-compatible.
			lineLayer := style.Layer{
				ID:          uniqueStyleLayerID(l.MVTName()+"-line", usedLayerIDs),
				Source:      req.mapName,
				SourceLayer: l.MVTName(),
				Type:        style.LayerTypeLine,
				Filter:      []interface{}{"==", "$type", "LineString"},
				Layout: &style.LayerLayout{
					Visibility: style.LayoutVisible,
				},
				Paint: &style.LayerPaint{
					LineColor: stringToColorHex(l.MVTName()),
				},
			}
			usedLayerIDs[lineLayer.ID] = true
			mapboxStyle.Layers = append(mapboxStyle.Layers, lineLayer)
		}

		// add our layer to our tile layer response
		mapboxStyle.Layers = append(mapboxStyle.Layers, layer)
		emittedLayerNames[l.MVTName()] = true
	}

	// mimetype for protocol buffers
	w.Header().Add("Content-Type", "application/json")

	// cache control headers (no-cache)
	w.Header().Add("Cache-Control", "no-cache, no-store, must-revalidate")
	w.Header().Add("Pragma", "no-cache")
	w.Header().Add("Expires", "0")

	if err = json.NewEncoder(w).Encode(mapboxStyle); err != nil {
		log.Errorf("error encoding tileJSON for map (%v)", req.mapName)
	}
}

func polygonPaint(name string) *style.LayerPaint {
	hexColor := stringToColorHex(name)
	hex, err := colors.ParseHEX(hexColor)
	if err != nil {
		log.Errorf("error parsing hex color (%v)", hexColor)
		hex, _ = colors.ParseHEX("#fff")
	}

	rgba := hex.ToRGBA()
	rgba.A = 0.10

	return &style.LayerPaint{
		FillColor:        rgba.String(),
		FillOutlineColor: hexColor,
	}
}

func visibleStyleLayout() *style.LayerLayout {
	return &style.LayerLayout{Visibility: style.LayoutVisible}
}

func collectionStyleLayers(mapName, sourceLayer string) []style.Layer {
	color := stringToColorHex(sourceLayer)
	return []style.Layer{
		{
			ID:          sourceLayer + "-point",
			Source:      mapName,
			SourceLayer: sourceLayer,
			Type:        style.LayerTypeCircle,
			Filter:      []interface{}{"==", "$type", "Point"},
			Layout:      visibleStyleLayout(),
			Paint: &style.LayerPaint{
				CircleRadius: 3,
				CircleColor:  color,
			},
		},
		{
			ID:          sourceLayer + "-line",
			Source:      mapName,
			SourceLayer: sourceLayer,
			Type:        style.LayerTypeLine,
			Filter:      []interface{}{"==", "$type", "LineString"},
			Layout:      visibleStyleLayout(),
			Paint: &style.LayerPaint{
				LineColor: color,
			},
		},
		{
			ID:          sourceLayer + "-fill",
			Source:      mapName,
			SourceLayer: sourceLayer,
			Type:        style.LayerTypeFill,
			Filter:      []interface{}{"==", "$type", "Polygon"},
			Layout:      visibleStyleLayout(),
			Paint:       polygonPaint(sourceLayer),
		},
	}
}

func uniqueStyleLayerID(base string, used map[string]bool) string {
	id := base
	for suffix := 2; used[id]; suffix++ {
		id = fmt.Sprintf("%s-%d", base, suffix)
	}
	return id
}

// port of https://stackoverflow.com/questions/3426404/create-a-hexadecimal-colour-based-on-a-string-with-javascript
func stringToColorHex(str string) string {
	var hash uint
	for i := range []rune(str) {
		hash = uint(str[i]) + ((hash << 5) - hash)
	}
	var color string
	for i := 0; i < 3; i++ {
		value := (hash >> (uint(i) * 8)) & 0xFF
		val := "00" + strconv.FormatUint(uint64(value), 16)
		color += val[len(val)-2:]
	}
	return "#" + color
}
