package geojson

import (
	"encoding/json"
	"fmt"

	"github.com/terranodo/tegola/geom"
)

func Encode(g geom.Geometry) (result []byte, err error) {
	switch g := g.(type) {
	case geom.Pointer, geom.MultiPointer, geom.LineStringer, geom.MultiLineStringer, geom.Polygoner,
		geom.MultiPolygoner:
		f, err := asFeature(g)
		if err != nil {
			return nil, err
		}
		result, err = json.Marshal(*f)
	case geom.Collectioner:
		result, err = EncodeCollection(g)
	default:
		err = fmt.Errorf("Unrecognized geometry type: %T", g)
	}

	return result, err
}

type coords struct {
	// Coordinates can be one of 4 different types.  This struct makes it simple for a 'geometry'
	//  to refer to coordinates and convert them to GeoJSON uniformly without having to know
	//  the details of which particular type they represent.

	// Flags indicating which coordinate type is being stored
	afloat    bool
	aafloat   bool
	aaafloat  bool
	aaaafloat bool
	// Expected for a Point
	tafloat [2]float64
	// Expected for a MultiPoint and LineString
	taafloat [][2]float64
	// Expected for a MultiLineString and Polygon
	taaafloat [][][2]float64
	// Expected for a MultiPolygon
	taaaafloat [][][][2]float64
}

func newCoords(value interface{}) (*coords, error) {
	c := &coords{}
	switch v := value.(type) {
	case [2]float64:
		c.afloat = true
		c.tafloat = v
	case [][2]float64:
		c.aafloat = true
		c.taafloat = v
	case [][][2]float64:
		c.aaafloat = true
		c.taaafloat = v
	case [][][][2]float64:
		c.aaaafloat = true
		c.taaaafloat = v
	default:
		return nil, fmt.Errorf("Unexpected inital value for coords: %v (%T)", value, value)
	}

	return c, nil
}

func (c coords) MarshalJSON() ([]byte, error) {
	if c.afloat {
		return json.Marshal(c.tafloat)
	} else if c.aafloat {
		return json.Marshal(c.taafloat)
	} else if c.aaafloat {
		return json.Marshal(c.taaafloat)
	} else if c.aaaafloat {
		return json.Marshal(c.taaaafloat)
	} else {
		return nil, fmt.Errorf("Uninitialized coords: %v", c)
	}
}

type geometry struct {
	Type   string `json:"type"`
	Coords coords `json:"coordinates"`
}

type feature struct {
	Type  string                 `json:"type"`
	Geom  geometry               `json:"geometry"`
	Props map[string]interface{} `json:"properties"`
}

type featureCollection struct {
	Type     string    `json:"type"`
	Features []feature `json:"features"`
}

func asFeature(g geom.Geometry) (*feature, error) {
	// Converts any geom.Geometry except geom.Collection to a 'feature' for json-serialization.
	var gtype string
	var coords *coords
	var err error
	switch gSpecific := g.(type) {
	case geom.Pointer:
		gtype = "Point"
		coords, err = newCoords(gSpecific.XY())
	case geom.MultiPointer:
		gtype = "MultiPoint"
		coords, err = newCoords(gSpecific.Points())
	case geom.LineStringer:
		gtype = "LineString"
		coords, err = newCoords(gSpecific.Verticies())
	case geom.MultiLineStringer:
		gtype = "MultiLineString"
		coords, err = newCoords(gSpecific.LineStrings())
	case geom.Polygoner:
		gtype = "Polygon"
		coords, err = newCoords(gSpecific.LinearRings())
	case geom.MultiPolygoner:
		gtype = "MultiPolygon"
		coords, err = newCoords(gSpecific.Polygons())
	default:
		err = fmt.Errorf("Can't convert geom.Geometry '%v' to feature (%T)\n", g, g)
	}

	if err != nil {
		return nil, err
	}

	f := &feature{
		Type:  "Feature",
		Geom:  geometry{Type: gtype, Coords: *coords},
		Props: map[string]interface{}{},
	}

	return f, nil
}

func EncodeCollection(c geom.Collectioner) ([]byte, error) {
	gs := c.Geometries()

	fc := featureCollection{
		Type:     "FeatureCollection",
		Features: nil,
	}
	fc.Features = make([]feature, len(gs))
	var fp *feature
	var err error
	for i, g := range gs {
		fp, err = asFeature(g)
		if err != nil {
			return nil, err
		}
		fc.Features[i] = *fp
	}

	return json.Marshal(fc)
}
