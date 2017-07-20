package server

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"strconv"
	"strings"

	"github.com/dimfeld/httptreemux"

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
	var rScheme string
	//	check if the request is http or https. the scheme is needed for the TileURLs and
	//	r.URL.Scheme can be empty if a relative request is issued from the client. (i.e. GET /foo.html)
	if r.TLS != nil {
		rScheme = "https://"
	} else {
		rScheme = "http://"
	}

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
	m, ok := maps[req.mapName]
	if !ok {
		log.Printf("map (%v) not configured. check your config file", req.mapName)
		http.Error(w, "map ("+req.mapName+") not configured. check your config file", http.StatusNotFound)
		return
	}

	mapboxStyle := style.Root{
		Name:    m.Name,
		Version: style.Version,
		Center:  [2]float64{m.Center[0], m.Center[1]},
		Zoom:    m.Center[2],
		Sources: map[string]style.Source{
			req.mapName: style.Source{
				Type: style.SourceTypeVector,
				URL:  fmt.Sprintf("%v%v/capabilities/%v.json", rScheme, hostName(r), req.mapName),
			},
		},
		Layers: []style.Layer{},
	}

	//	determing the min and max zoom for this map
	for _, l := range m.Layers {
		//	build our vector layer details
		layer := style.Layer{
			ID:          l.Name,
			Source:      req.mapName,
			SourceLayer: l.Name,
			Type:        style.LayerTypeLine,
			Layout: &style.LayerLayout{
				Visibility: style.LayoutVisible,
			},
			Paint: &style.LayerPaint{
				LineColor: stringToColor(l.Name),
			},
		}

		//	add our layer to our tile layer response
		mapboxStyle.Layers = append(mapboxStyle.Layers, layer)
	}

	//	TODO: how configurable do we want the CORS policy to be?
	//	set CORS header
	w.Header().Add("Access-Control-Allow-Origin", "*")

	//	mimetype for protocol buffers
	w.Header().Add("Content-Type", "application/json")

	if err = json.NewEncoder(w).Encode(mapboxStyle); err != nil {
		log.Printf("error encoding tileJSON for map (%v)", req.mapName)
	}
}

func randomColor() string {
	val := int64(rand.Float64() * 16777215)

	return "#" + strconv.FormatInt(val, 16)
}

//	port of https://stackoverflow.com/questions/3426404/create-a-hexadecimal-colour-based-on-a-string-with-javascript
func stringToColor(str string) string {
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
