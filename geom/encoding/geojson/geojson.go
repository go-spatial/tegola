package geojson

import (
	"encoding/json"
	"fmt"

	"github.com/terranodo/tegola/geom"
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

type ErrUnknownGeometry struct {
	Geom geom.Geometry
}

func (e ErrUnknownGeometry) Error() string {
	return fmt.Sprintf("unknown geometry: %T", e.Geom)
}

type Geometry struct {
	geom.Geometry
}

func (geo Geometry) MarshalJSON() ([]byte, error) {
	type coordinates struct {
		Type  GeoJSONType `json:"type"`
		Coord interface{} `json:"coordinates,omitempty"`
	}
	type collection struct {
		Type       GeoJSONType `json:"type"`
		Geometries []Geometry  `json:"geometries,omitempty"`
	}

	switch g := geo.Geometry.(type) {
	case geom.Pointer:
		return json.Marshal(coordinates{
			Type:  PointType,
			Coord: g.XY(),
		})

	case geom.MultiPointer:
		return json.Marshal(coordinates{
			Type:  MultiPointType,
			Coord: g.Points(),
		})

	case geom.LineStringer:
		return json.Marshal(coordinates{
			Type:  LineStringType,
			Coord: g.Verticies(),
		})

	case geom.MultiLineStringer:
		return json.Marshal(coordinates{
			Type:  MultiLineStringType,
			Coord: g.LineStrings(),
		})

	case geom.Polygoner:
		return json.Marshal(coordinates{
			Type:  PolygonType,
			Coord: g.LinearRings(),
		})

	case geom.MultiPolygoner:
		return json.Marshal(coordinates{
			Type:  MultiPolygonType,
			Coord: g.Polygons(),
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
	}

	return nil, ErrUnknownGeometry{geom.Geometry(geo)}
}

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

type featureCollectionType struct{}

func (_ featureCollectionType) MarshalJSON() ([]byte, error) {
	return []byte(`"FeatureCollection"`), nil
}

type FeatureCollection struct {
	Type     featureCollectionType `json:"type"`
	Features []Feature             `json:"features"`
}
