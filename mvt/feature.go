package mvt

import (
	"fmt"

	"github.com/terranodo/tegola"
	"github.com/terranodo/tegola/mvt/vector_tile"
)

// errors
var (
	ErrNilFeature = fmt.Errorf("Feature is nil")
	// ErrUnknownGeometryType is the error retuned when the geometry is unknown.
	ErrUnknownGeometryType = fmt.Errorf("Unknown geometry type")
)

// TODO: Need to put in validation for the Geometry, at current the system
// does not check to make sure that the geometry is following the rules as
// laided out by the spec. It just assumes the user is good.

// Feature describes a feature of a Layer. A layer will contain multiple features
// each of which has a geometry describing the interesting thing, and the metadata
// associated with it.
type Feature struct {
	ID   *uint64
	Tags map[string]interface{}
	// Does not support the collection geometry, for this you have to create a feature for each
	// geometry in the collection.
	Geometry tegola.Geometry
}

// VTileFeature will return a vectorTile.Feature that would represent the Feature
func (f *Feature) VTileFeature(keyMap []string, valMap []interface{}) (tf *vectorTile.Tile_Feature, err error) {
	tf = new(vectorTile.Tile_Feature)
	tf.Id = f.ID
	if tf.Tags, err = keyvalTagsMap(keyMap, valMap, f); err != nil {
		return tf, err
	}

	geo, gtype, err := encodeGeometry(f.Geometry)
	if err != nil {
		return tf, err
	}
	tf.Geometry = geo
	tf.Type = &gtype
	return tf, nil
}

const (
	cmdMoveTo    uint32 = 1
	cmdLineTo    uint32 = 2
	cmdClosePath uint32 = 7

	maxCmdCount uint32 = 0x1FFFFFFF
)

// cursor reprsents the current position, this is needed to encode the geometry.
// 0,0 is the origin, it which is the top-left most part of the tile.
type cursor struct {
	X int64
	Y int64
}

func encodeZigZag(i int64) uint32 {
	return uint32((i << 1) ^ (i >> 31))
}

func (c *cursor) MoveTo(points ...tegola.Point) []uint32 {
	if len(points) == 0 {
		return []uint32{}
	}

	//	new slice to hold our encode bytes
	g := make([]uint32, 0, (2*len(points))+1)
	//	compute command integere
	g = append(g, (cmdMoveTo&0x7)|(uint32(len(points))<<3))

	//	range through our points
	for _, p := range points {
		//	computer our point delta
		dx := int64(p.X()) - c.X
		dy := int64(p.Y()) - c.Y

		//	update our cursor
		c.X = int64(p.X())
		c.Y = int64(p.Y())

		//	encode our delta point
		g = append(g, encodeZigZag(dx), encodeZigZag(dy))
	}
	return g
}
func (c *cursor) LineTo(points ...tegola.Point) []uint32 {
	if len(points) == 0 {
		return []uint32{}
	}
	g := make([]uint32, 0, (2*len(points))+1)
	g = append(g, (cmdLineTo&0x7)|(uint32(len(points))<<3))
	for _, p := range points {
		dx := int64(p.X()) - c.X
		dy := int64(p.Y()) - c.Y
		c.X = int64(p.X())
		c.Y = int64(p.Y())
		g = append(g, encodeZigZag(dx), encodeZigZag(dy))
	}
	return g
}

func (c *cursor) ClosePath() uint32 {
	return (cmdClosePath&0x7 | (1 << 3))
}

// encodeGeometry will take a tegola.Geometry type and encode it according to the
// mapbox vector_tile spec.
func encodeGeometry(geo tegola.Geometry) (g []uint32, vtyp vectorTile.Tile_GeomType, err error) {
	var c cursor
	switch t := geo.(type) {
	case tegola.Point:
		g = append(g, c.MoveTo(t)...)
		return g, vectorTile.Tile_POINT, nil

	case tegola.Point3:
		g = append(g, c.MoveTo(t)...)
		return g, vectorTile.Tile_POINT, nil

	case tegola.MultiPoint:
		g = append(g, c.MoveTo(t.Points()...)...)
		return g, vectorTile.Tile_POINT, nil

	case tegola.LineString:
		points := t.Subpoints()
		g = append(g, c.MoveTo(points[0])...)
		g = append(g, c.LineTo(points[1:]...)...)
		return g, vectorTile.Tile_LINESTRING, nil

	case tegola.MultiLine:
		lines := t.Lines()
		for _, l := range lines {
			points := l.Subpoints()
			g = append(g, c.MoveTo(points[0])...)
			g = append(g, c.LineTo(points[1:]...)...)
		}
		return g, vectorTile.Tile_LINESTRING, nil

	case tegola.Polygon:
		lines := t.Sublines()
		for _, l := range lines {
			points := l.Subpoints()
			g = append(g, c.MoveTo(points[0])...)
			g = append(g, c.LineTo(points[1:]...)...)
			g = append(g, c.ClosePath())
		}
		return g, vectorTile.Tile_POLYGON, nil

	case tegola.MultiPolygon:
		polygons := t.Polygons()
		for _, p := range polygons {
			lines := p.Sublines()
			for _, l := range lines {
				points := l.Subpoints()
				g = append(g, c.MoveTo(points[0])...)
				g = append(g, c.LineTo(points[1:]...)...)
				g = append(g, c.ClosePath())
			}
		}
		return g, vectorTile.Tile_POLYGON, nil

	default:
		return nil, vectorTile.Tile_UNKNOWN, ErrUnknownGeometryType
	}
}

// keyvalMapsFromFeatures returns a key map and value map, to help with the translation
// to mapbox tile format. In the Tile format, the Tile contains a mapping of all the unique
// keys and values, and then each feature contains a vector map to these two. This is an
// intermediate data structure to help with the construction of the three mappings.
func keyvalMapsFromFeatures(features []Feature) (keyMap []string, valMap []interface{}, err error) {
	var didFind bool
	for _, f := range features {
		for k, v := range f.Tags {
			didFind = false
			for _, mk := range keyMap {
				if k == mk {
					didFind = true
					break
				}
			}
			if !didFind {
				keyMap = append(keyMap, k)
			}
			didFind = false

			switch vt := v.(type) {
			default:
				return keyMap, valMap, fmt.Errorf("Unsupported type for value(%v) with key(%v) in tags for feature %v.", vt, k, f)

			case string:
				for _, mv := range valMap {
					tmv, ok := mv.(string)
					if !ok {
						continue
					}
					if tmv == vt {
						didFind = true
						break
					}
				}

			case fmt.Stringer:
				for _, mv := range valMap {
					tmv, ok := mv.(fmt.Stringer)
					if !ok {
						continue
					}
					if tmv.String() == vt.String() {
						didFind = true
						break
					}
				}

			case int:
				for _, mv := range valMap {
					tmv, ok := mv.(int)
					if !ok {
						continue
					}
					if tmv == vt {
						didFind = true
						break
					}
				}

			case int8:
				for _, mv := range valMap {
					tmv, ok := mv.(int8)
					if !ok {
						continue
					}
					if tmv == vt {
						didFind = true
						break
					}
				}

			case int16:
				for _, mv := range valMap {
					tmv, ok := mv.(int16)
					if !ok {
						continue
					}
					if tmv == vt {
						didFind = true
						break
					}
				}

			case int32:
				for _, mv := range valMap {
					tmv, ok := mv.(int32)
					if !ok {
						continue
					}
					if tmv == vt {
						didFind = true
						break
					}
				}

			case int64:
				for _, mv := range valMap {
					tmv, ok := mv.(int64)
					if !ok {
						continue
					}
					if tmv == vt {
						didFind = true
						break
					}
				}

			case uint:
				for _, mv := range valMap {
					tmv, ok := mv.(uint)
					if !ok {
						continue
					}
					if tmv == vt {
						didFind = true
						break
					}
				}

			case uint8:
				for _, mv := range valMap {
					tmv, ok := mv.(uint8)
					if !ok {
						continue
					}
					if tmv == vt {
						didFind = true
						break
					}
				}

			case uint16:
				for _, mv := range valMap {
					tmv, ok := mv.(uint16)
					if !ok {
						continue
					}
					if tmv == vt {
						didFind = true
						break
					}
				}

			case uint32:
				for _, mv := range valMap {
					tmv, ok := mv.(uint32)
					if !ok {
						continue
					}
					if tmv == vt {
						didFind = true
						break
					}
				}

			case uint64:
				for _, mv := range valMap {
					tmv, ok := mv.(uint64)
					if !ok {
						continue
					}
					if tmv == vt {
						didFind = true
						break
					}
				}

			case float32:
				for _, mv := range valMap {
					tmv, ok := mv.(float32)
					if !ok {
						continue
					}
					if tmv == vt {
						didFind = true
						break
					}
				}

			case float64:
				for _, mv := range valMap {
					tmv, ok := mv.(float64)
					if !ok {
						continue
					}
					if tmv == vt {
						didFind = true
						break
					}
				}

			case bool:
				for _, mv := range valMap {
					tmv, ok := mv.(bool)
					if !ok {
						continue
					}
					if tmv == vt {
						didFind = true
						break
					}
				}

			} // value type switch

			if !didFind {
				valMap = append(valMap, v)
			}

		} // For f.Tags
	} // for features
	return keyMap, valMap, nil
}

// keyvalTagsMap will return the tags map as expected by the mapbox tile spec. It takes
// a keyMap and a valueMap that list the the order of the expected keys and values. It will
// return a vector map that refers to these two maps.
func keyvalTagsMap(keyMap []string, valueMap []interface{}, f *Feature) (tags []uint32, err error) {
	if f == nil {
		return nil, ErrNilFeature
	}

	var kidx, vidx int64
	for key, val := range f.Tags {
		kidx = -1
		vidx = -1
		for i, k := range keyMap {
			if k == key {
				kidx = int64(i)
				break
			}
		}
		if kidx == -1 {
			return tags, fmt.Errorf("Did not find key: %v in keymap.", key)
		}
		switch tv := val.(type) {
		default:
			return tags, fmt.Errorf("Value(%[1]v) type of %[1]t is not supported.", tv)

		case string:
			for i, v := range valueMap {
				if vmt, ok := v.(string); ok {
					if vmt == tv {
						vidx = int64(i)
						break
					}
				}
			}

		case fmt.Stringer:
			for i, v := range valueMap {
				if vmt, ok := v.(fmt.Stringer); ok {
					if vmt.String() == tv.String() {
						vidx = int64(i)
						break
					}
				}
			}

		case int:
			for i, v := range valueMap {
				if vmt, ok := v.(int); ok {
					if vmt == tv {
						vidx = int64(i)
						break
					}
				}
			}

		case int8:
			for i, v := range valueMap {
				if vmt, ok := v.(int8); ok {
					if vmt == tv {
						vidx = int64(i)
						break
					}
				}
			}

		case int16:
			for i, v := range valueMap {
				if vmt, ok := v.(int16); ok {
					if vmt == tv {
						vidx = int64(i)
						break
					}
				}
			}

		case int32:
			for i, v := range valueMap {
				if vmt, ok := v.(int32); ok {
					if vmt == tv {
						vidx = int64(i)
						break
					}
				}
			}

		case int64:
			for i, v := range valueMap {
				if vmt, ok := v.(int64); ok {
					if vmt == tv {
						vidx = int64(i)
						break
					}
				}
			}

		case uint:
			for i, v := range valueMap {
				if vmt, ok := v.(uint); ok {
					if vmt == tv {
						vidx = int64(i)
						break
					}
				}
			}

		case uint8:
			for i, v := range valueMap {
				if vmt, ok := v.(uint8); ok {
					if vmt == tv {
						vidx = int64(i)
						break
					}
				}
			}

		case uint16:
			for i, v := range valueMap {
				if vmt, ok := v.(uint16); ok {
					if vmt == tv {
						vidx = int64(i)
						break
					}
				}
			}

		case uint32:
			for i, v := range valueMap {
				if vmt, ok := v.(uint32); ok {
					if vmt == tv {
						vidx = int64(i)
						break
					}
				}
			}

		case uint64:
			for i, v := range valueMap {
				if vmt, ok := v.(uint64); ok {
					if vmt == tv {
						vidx = int64(i)
						break
					}
				}
			}

		case float32:
			for i, v := range valueMap {
				if vmt, ok := v.(float32); ok {
					if vmt == tv {
						vidx = int64(i)
						break
					}
				}
			}

		case float64:
			for i, v := range valueMap {
				if vmt, ok := v.(float64); ok {
					if vmt == tv {
						vidx = int64(i)
						break
					}
				}
			}

		case bool:
			for i, v := range valueMap {
				if vmt, ok := v.(bool); ok {
					if vmt == tv {
						vidx = int64(i)
						break
					}
				}
			}

		} // Values Switch Statement
		if vidx == -1 {
			return tags, fmt.Errorf("Did not find a value: %v in valuemap.", val)
		}

	} // KeyVal For
	tags = append(tags, uint32(kidx), uint32(vidx))

	return tags, nil
}
