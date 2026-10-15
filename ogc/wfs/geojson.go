package wfs

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/alexeydott/geom"
)

// parseServiceGeometry decodes a GeoJSON geometry (as served by the
// feature service) into a geom.Geometry for GML encoding.
func parseServiceGeometry(raw json.RawMessage) (geom.Geometry, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var gm map[string]interface{}
	if err := dec.Decode(&gm); err != nil {
		return nil, fmt.Errorf("invalid GeoJSON geometry: %w", err)
	}
	t, _ := gm["type"].(string)
	coords := gm["coordinates"]
	switch t {
	case "Point":
		c, err := wfsCoord(coords)
		return geom.Point(c), err
	case "LineString":
		return wfsLineString(coords)
	case "Polygon":
		return wfsPolygon(coords)
	case "MultiPoint":
		l, err := wfsLineString(coords)
		return geom.MultiPoint(l), err
	case "MultiLineString":
		return wfsMultiLineString(coords)
	case "MultiPolygon":
		return wfsMultiPolygon(coords)
	default:
		return nil, fmt.Errorf("unsupported geometry type %q", t)
	}
}

func wfsFloat(v interface{}) (float64, error) {
	switch n := v.(type) {
	case json.Number:
		return n.Float64()
	case float64:
		return n, nil
	default:
		return 0, fmt.Errorf("not a number")
	}
}

func wfsCoord(v interface{}) ([2]float64, error) {
	arr, ok := v.([]interface{})
	if !ok || len(arr) < 2 {
		return [2]float64{}, fmt.Errorf("invalid coordinate")
	}
	x, err := wfsFloat(arr[0])
	if err != nil {
		return [2]float64{}, err
	}
	y, err := wfsFloat(arr[1])
	if err != nil {
		return [2]float64{}, err
	}
	return [2]float64{x, y}, nil
}

func wfsLineString(v interface{}) (geom.LineString, error) {
	arr, ok := v.([]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid linestring")
	}
	out := make(geom.LineString, 0, len(arr))
	for _, c := range arr {
		cc, err := wfsCoord(c)
		if err != nil {
			return nil, err
		}
		out = append(out, cc)
	}
	return out, nil
}

func wfsPolygon(v interface{}) (geom.Polygon, error) {
	arr, ok := v.([]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid polygon")
	}
	out := make(geom.Polygon, 0, len(arr))
	for _, r := range arr {
		ring, err := wfsLineString(r)
		if err != nil {
			return nil, err
		}
		out = append(out, ring)
	}
	return out, nil
}

func wfsMultiLineString(v interface{}) (geom.MultiLineString, error) {
	arr, ok := v.([]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid multilinestring")
	}
	out := make(geom.MultiLineString, 0, len(arr))
	for _, l := range arr {
		line, err := wfsLineString(l)
		if err != nil {
			return nil, err
		}
		out = append(out, line)
	}
	return out, nil
}

func wfsMultiPolygon(v interface{}) (geom.MultiPolygon, error) {
	arr, ok := v.([]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid multipolygon")
	}
	out := make(geom.MultiPolygon, 0, len(arr))
	for _, p := range arr {
		poly, err := wfsPolygon(p)
		if err != nil {
			return nil, err
		}
		out = append(out, poly)
	}
	return out, nil
}
