package geojson

import (
	"encoding/json"

	"github.com/terranodo/tegola/geom"
	"github.com/terranodo/tegola/geom/encoding"
	"github.com/terranodo/tegola/internal/log"
)

type GeoJSONType string

const (
	PointType              GeoJSONType = "Point"
	MultiPointType         GeoJSONType = "MultiPoint"
	LineStringType         GeoJSONType = "LineString"
	MultiLineStringType    GeoJSONType = "MultiLineString"
	PolygonType            GeoJSONType = "Polygon"
	MultiPolygonType       GeoJSONType = "MultiPolygon"
	GeometryCollectionType GeoJSONType = "GeometryCollection"
)

type Geometry struct {
	geom.Geometry
}

func (geo Geometry) MarshalJSON() ([]byte, error) {
	type coordinates struct {
		Type   GeoJSONType `json:"type"`
		Coords interface{} `json:"coordinates,omitempty"`
	}
	type collection struct {
		Type       GeoJSONType `json:"type"`
		Geometries []Geometry  `json:"geometries,omitempty"`
	}

	switch g := geo.Geometry.(type) {
	case geom.Pointer:
		return json.Marshal(coordinates{
			Type:   PointType,
			Coords: g.XY(),
		})

	case geom.MultiPointer:
		return json.Marshal(coordinates{
			Type:   MultiPointType,
			Coords: g.Points(),
		})

	case geom.LineStringer:
		return json.Marshal(coordinates{
			Type:   LineStringType,
			Coords: g.Verticies(),
		})

	case geom.MultiLineStringer:
		return json.Marshal(coordinates{
			Type:   MultiLineStringType,
			Coords: g.LineStrings(),
		})

	case geom.Polygoner:
		ps := g.LinearRings()
		closePolygon(ps)

		return json.Marshal(coordinates{
			Type: PolygonType,
			//	make sure our rings are closed
			Coords: ps,
		})

	case geom.MultiPolygoner:
		ps := g.Polygons()

		//	iterate through the polygons making sure they're closed
		for i := range ps {
			closePolygon(geom.Polygon(ps[i]))
		}

		return json.Marshal(coordinates{
			Type:   MultiPolygonType,
			Coords: ps,
		})

	case geom.Collectioner:
		gs := g.Geometries()

		var geos = make([]Geometry, 0, len(gs))
		for _, gg := range gs {
			geos = append(geos, Geometry{gg})
		}

		return json.Marshal(collection{
			Type:       GeometryCollectionType,
			Geometries: geos,
		})

	default:
		return nil, encoding.ErrUnknownGeometry{g}
	}
}

// featureType allows the GeoJSON type for Feature to be automatically set during json Marshalling
// which avoids the user from accidenlty setting the incorrect GeoJSON type.
type featureType struct{}

func (_ featureType) MarshalJSON() ([]byte, error) {
	return []byte(`"Feature"`), nil
}

type Feature struct {
	Type featureType `json:"type"`
	ID   *uint64     `json:"id,omitempty"`
	// can be null
	Geometry Geometry `json:"geometry"`
	// can be null
	Properties map[string]interface{} `json:"properties"`
}

// featureCollectionType allows the GeoJSON type for Feature to be automatically set during json Marshalling
// which avoids the user from accidenlty setting the incorrect GeoJSON type.
type featureCollectionType struct{}

func (_ featureCollectionType) MarshalJSON() ([]byte, error) {
	return []byte(`"FeatureCollection"`), nil
}

type FeatureCollection struct {
	Type     featureCollectionType `json:"type"`
	Features []Feature             `json:"features"`
}

func closePolygon(p geom.Polygon) {
	for i := range p {
		if len(p[i]) < 3 {
			log.Warn("encounted polygon with less than 3 points")
			continue
		}

		//	check if the first point and the last point are the same
		//	if they're not, make a copy of the first point and add it as the last position
		if p[i][0] != p[i][len(p[i])-1] {
			p[i] = append(p[i], p[i][0])
		}
	}
}
