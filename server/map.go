package server

import "github.com/terranodo/tegola/mvt"

//	NewMap creates a new map with the necessary default values
func NewMap(name string) Map {
	return Map{
		Name: name,
		//	default bounds
		Bounds: [4]float64{-180.0, -90.0, 180.0, 90.0},
	}
}

type Map struct {
	Name string
	//	Contains an attribution to be displayed when the map is shown to a user.
	// 	This string is sanatized so it can't be abused as a vector for XSS or beacon tracking.
	Attribution string
	//	The maximum extent of available map tiles in WGS:84
	//	latitude and longitude values, in the order left, bottom, right, top.
	//	Default: [-180, -90, 180, 90]
	Bounds [4]float64
	//	The first value is the longitude, the second is latitude (both in
	//	WGS:84 values), the third value is the zoom level.
	Center [3]float64
	Layers []Layer
}

//	FilterByZoom returns layers that that are to be rendered between a min and max zoom
func (m *Map) FilterLayersByZoom(zoom int) (filteredLayers []Layer) {
	for _, l := range m.Layers {
		if (l.MinZoom <= zoom || l.MinZoom == 0) && (l.MaxZoom >= zoom || l.MaxZoom == 0) {
			filteredLayers = append(filteredLayers, l)
		}
	}
	return
}

//	FilterByName returns a slice with the first layer that matches the provided name
//	the slice return is for convenience. MVT tiles require unique layer names
func (m *Map) FilterLayersByName(name string) (filteredLayers []Layer) {
	for _, l := range m.Layers {
		if l.Name == name {
			filteredLayers = append(filteredLayers, l)
			return
		}
	}
	return
}

type Layer struct {
	Name          string // if none is set, it will be inferred from ProviderLayer
	ProviderLayer string
	MinZoom       int
	MaxZoom       int
	//	instantiated provider
	Provider mvt.Provider
	//	default tags to include when encoding the layer. provider tags take precedence
	DefaultTags map[string]interface{}
}
