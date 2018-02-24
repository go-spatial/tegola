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
		return json.Marshal(coordinates{
			Type: PolygonType,
			//	make sure our rings are closed
			Coords: closePolygon(g).LinearRings(),
		})

	case geom.MultiPolygoner:
		ps := g.Polygons()

		//	pre allocate our memory for the copy
		var mp = make(geom.MultiPolygon, 0, len(ps))
		//	iterate through the polygons making sure they're closed
		for _, p := range ps {
			mp = append(mp, closePolygon(geom.Polygon(p)).LinearRings())
		}

		return json.Marshal(coordinates{
			Type:   MultiPolygonType,
			Coords: mp.Polygons(),
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

// closePolygon will check if the polygon has the same value for the first and last point. if the value
// is not the same it will add an extra point to "close" the polygon per the GeoJSON spec.
func closePolygon(p geom.Polygoner) geom.Polygoner {
	rings := p.LinearRings()

	rs := make([][][2]float64, 0, len(rings))
	for i := range rings {
		if len(rings[i]) < 3 {
			log.Warn("encounted polygon with less than 3 points. dropping")
			continue
		}

		//	check if the first point and the last point are the same
		//	if they're not, make a copy of the first point and add it as the last position
		if rings[i][0] != rings[i][len(rings[i])-1] {
			rs = append(rs, append(rings[i], rings[i][0]))
		} else {
			rs = append(rs, rings[i])
		}
	}

	return geom.Polygon(rs)
}
