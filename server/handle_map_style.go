package server

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/dimfeld/httptreemux"
	"gopkg.in/go-playground/colors.v1"

	"github.com/terranodo/tegola"
	"github.com/terranodo/tegola/atlas"
	"github.com/terranodo/tegola/mapbox/style"
)

type HandleMapStyle struct {
	//	required
	mapName string
	//	the requests extension defaults to "json"
	extension string
}

//	returns details about a map according to the
//	tileJSON spec (https://github.com/mapbox/tilejson-spec/tree/master/2.1.0)
//
//	URI scheme: /capabilities/:map_name.json
//		map_name - map name in the config file
func (req HandleMapStyle) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var err error

	params := httptreemux.ContextParams(r.Context())

	//	read the map_name value from the request
	mapName := params["map_name"]
	mapNameParts := strings.Split(mapName, ".")

	req.mapName = mapNameParts[0]
	//	check if we have a provided extension
	if len(mapNameParts) > 2 {
		req.extension = mapNameParts[len(mapNameParts)-1]
	} else {
		req.extension = "json"
	}

	//	lookup our Map
	m, err := atlas.GetMap(req.mapName)
	if err != nil {
		log.Printf("map (%v) not configured. check your config file", req.mapName)
		http.Error(w, "map ("+req.mapName+") not configured. check your config file", http.StatusNotFound)
		return
	}

	var debugQuery string
	if r.URL.Query().Get("debug") == "true" {
		debugQuery = "?debug=true"

		m = m.EnableDebugLayers()
	}

	sourceURL := fmt.Sprintf("%v://%v/capabilities/%v.json%v", scheme(r), hostName(r), req.mapName, debugQuery)

	mapboxStyle := style.Root{
		Name:    m.Name,
		Version: style.Version,
		Center:  [2]float64{m.Center[0], m.Center[1]},
		Zoom:    m.Center[2],
		Sources: map[string]style.Source{
			req.mapName: style.Source{
				Type: style.SourceTypeVector,
				URL:  sourceURL,
			},
		},
		Layers: []style.Layer{},
	}

	//	determing the min and max zoom for this map
	for _, l := range m.Layers {
		//	skip disabled layers
		if l.Disabled {
			continue
		}
		//	check if the layer already exists in our slice. this can happen if the config
		//	is using the "name" param for a layer to override the providerLayerName
		var skip bool
		for i := range mapboxStyle.Layers {
			if mapboxStyle.Layers[i].ID == l.MVTName() {
				skip = true
				break
			}
		}
		//	entry for layer already exists. move on
		if skip {
			continue
		}

		//	build our vector layer details
		layer := style.Layer{
			ID:          l.MVTName(),
			Source:      req.mapName,
			SourceLayer: l.MVTName(),
			Layout: &style.LayerLayout{
				Visibility: style.LayoutVisible,
			},
		}

		//	chose our paint type based on the geometry type
		switch l.GeomType.(type) {
		case tegola.Point, tegola.Point3, tegola.MultiPoint:
			layer.Type = style.LayerTypeCircle
			layer.Paint = &style.LayerPaint{
				CircleRadius: 3,
				CircleColor:  stringToColorHex(l.MVTName()),
			}
		case tegola.LineString, tegola.MultiLine:
			layer.Type = style.LayerTypeLine
			layer.Paint = &style.LayerPaint{
				LineColor: stringToColorHex(l.MVTName()),
			}
		case tegola.Polygon, tegola.MultiPolygon:
			layer.Type = style.LayerTypeFill
			hexColor := stringToColorHex(l.MVTName())

			hex, err := colors.ParseHEX(hexColor)
			if err != nil {
				log.Println("error parsing hex color (%v)", hexColor)
				hex, _ = colors.ParseHEX("#fff") //	default to white on error
			}

			rgba := hex.ToRGBA()
			//	set the opacity to 10%
			rgba.A = 0.10

			layer.Paint = &style.LayerPaint{
				FillColor:        rgba.String(),
				FillOutlineColor: hexColor,
			}
		default:
			log.Printf("layer (providerLayerName: %v) has unsupported geometry type (%v)", l.ProviderLayerName, l.GeomType)
		}

		//	add our layer to our tile layer response
		mapboxStyle.Layers = append(mapboxStyle.Layers, layer)
	}

	//	mimetype for protocol buffers
	w.Header().Add("Content-Type", "application/json")

	//	cache control headers (no-cache)
	w.Header().Add("Cache-Control", "no-cache, no-store, must-revalidate")
	w.Header().Add("Pragma", "no-cache")
	w.Header().Add("Expires", "0")

	if err = json.NewEncoder(w).Encode(mapboxStyle); err != nil {
		log.Printf("error encoding tileJSON for map (%v)", req.mapName)
	}
}

//	port of https://stackoverflow.com/questions/3426404/create-a-hexadecimal-colour-based-on-a-string-with-javascript
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
