package mvt

import (
	"context"
	"fmt"

	"github.com/go-spatial/geom"
	"github.com/go-spatial/geom/encoding/wkt"
	"github.com/go-spatial/tegola/internal/log"
	vectorTile "github.com/go-spatial/tegola/mvt/vector_tile"
)

var (
	ErrNilFeature          = fmt.Errorf("feature is nil")
	ErrUnknownGeometryType = fmt.Errorf("unknown geometry type")
	ErrNilGeometryType     = fmt.Errorf("geometry is nil")
)

// TODO: Need to put in validation for the Geometry, as current the system
// does not check to make sure that the geometry is following the rules as
// laid out by the spec (i.e. polygons must not have the same start and end
// point).

// Feature describes a feature of a Layer. A layer will contain multiple features
// each of which has a geometry describing the interesting thing, and the metadata
// associated with it.
type Feature struct {
	ID       *uint64
	Tags     map[string]interface{}
	Geometry geom.Geometry
}

func wktEncode(g geom.Geometry) string {
	s, err := wkt.Encode(g)
	if err != nil {
		return fmt.Sprintf("encoding error for geom geom, %v", err)
	}

	return s
}

func (f Feature) String() string {
	g := wktEncode(f.Geometry)

	if f.ID != nil {
		return fmt.Sprintf("{Feature: %v, GEO: %v, Tags: %+v}", *f.ID, g, f.Tags)
	}

	return fmt.Sprintf("{Feature: GEO: %v, Tags: %+v}", g, f.Tags)
}

// NewFeatures returns one or more features for the given Geometry
func NewFeatures(geo geom.Geometry, tags map[string]interface{}) (f []Feature) {
	if geo == nil {
		return f // return empty feature set for a nil geometry
	}

	if g, ok := geo.(geom.Collection); ok {
		geos := g.Geometries()
		for i := range geos {
			f = append(f, NewFeatures(geos[i], tags)...)
		}
		return f
	}

	f = append(f, Feature{
		Tags:     tags,
		Geometry: geo,
	})

	return f
}

// VTileFeature will return a vectorTile.Feature that would represent the Feature
func (f *Feature) VTileFeature(ctx context.Context, keys []string, vals []interface{}) (tf *vectorTile.Tile_Feature, err error) {
	tf = new(vectorTile.Tile_Feature)
	tf.Id = f.ID

	if tf.Tags, err = keyvalTagsMap(keys, vals, f); err != nil {
		return tf, err
	}

	geo, gtype, err := encodeGeometry(ctx, f.Geometry)
	if err != nil {
		return tf, err
	}

	if len(geo) == 0 {
		return nil, nil
	}

	tf.Geometry = geo
	tf.Type = &gtype

	return tf, nil
}

// These values came from: https://github.com/mapbox/vector-tile-spec/tree/master/2.1
const (
	cmdMoveTo    uint32 = 1
	cmdLineTo    uint32 = 2
	cmdClosePath uint32 = 7

	maxCmdCount uint32 = 0x1FFFFFFF
)

type Command uint32

func NewCommand(cmd uint32, count int) Command {
	return Command((cmd & 0x7) | (uint32(count) << 3))
}

func (c Command) ID() uint32 {
	return uint32(c) & 0x7
}
func (c Command) Count() int {
	return int(uint32(c) >> 3)
}

func (c Command) String() string {
	switch c.ID() {
	case cmdMoveTo:
		return fmt.Sprintf("move Command with count %v", c.Count())
	case cmdLineTo:
		return fmt.Sprintf("line To command with count %v", c.Count())
	case cmdClosePath:
		return fmt.Sprintf("close path command with count %v", c.Count())
	default:
		return fmt.Sprintf("unknown command (%v) with count %v", c.ID(), c.Count())
	}
}

// encodeZigZag does the ZigZag encoding for small ints.
func encodeZigZag(i int64) uint32 {
	return uint32((i << 1) ^ (i >> 31))
}

// cursor reprsents the current position, this is needed to encode the geometry.
// the origin (0,0) is the top-left of the Tile.
type cursor struct {
	// The coordinates — these should be int64, when they were float64 they
	// introduced a slight drift in the coordinates.
	x int64
	y int64
}

func NewCursor() *cursor {
	return &cursor{}
}

// GetDeltaPointAndUpdate assumes the Point is in WebMercator.
func (c *cursor) GetDeltaPointAndUpdate(p geom.Point) (dx, dy int64) {
	var ix, iy int64
	var tx, ty = p.X(), p.Y()

	ix, iy = int64(tx), int64(ty)
	// compute our point delta
	dx = ix - int64(c.x)
	dy = iy - int64(c.y)

	// update our cursor
	c.x = ix
	c.y = iy
	return dx, dy
}

func (c *cursor) encodeCmd(cmd uint32, points [][2]float64) []uint32 {
	if len(points) == 0 {
		return []uint32{}
	}
	// new slice to hold our encode bytes. 2 bytes for each point pluse a command byte.
	g := make([]uint32, 0, (2*len(points))+1)
	// add the command integer
	g = append(g, cmd)

	// range through our points
	for _, p := range points {
		dx, dy := c.GetDeltaPointAndUpdate(geom.Point(p))
		// encode our delta point
		g = append(g, encodeZigZag(dx), encodeZigZag(dy))
	}

	return g
}

func (c *cursor) MoveTo(points ...[2]float64) []uint32 {
	return c.encodeCmd(uint32(NewCommand(cmdMoveTo, len(points))), points)
}

func (c *cursor) LineTo(points ...[2]float64) []uint32 {
	return c.encodeCmd(uint32(NewCommand(cmdLineTo, len(points))), points)
}

func (c *cursor) ClosePath() uint32 {
	return uint32(NewCommand(cmdClosePath, 1))
}

// encodeGeometry will take a geom.Geometry and encode it according to the
// mapbox vector_tile spec.
func encodeGeometry(ctx context.Context, geometry geom.Geometry) (g []uint32, vtyp vectorTile.Tile_GeomType, err error) {
	if geometry == nil {
		return nil, vectorTile.Tile_UNKNOWN, ErrNilGeometryType
	}

	c := NewCursor()

	switch t := geometry.(type) {
	case geom.Point:
		g = append(g, c.MoveTo(t)...)
		return g, vectorTile.Tile_POINT, nil

	case geom.MultiPoint:
		g = append(g, c.MoveTo(t.Points()...)...)
		return g, vectorTile.Tile_POINT, nil

	case geom.LineString:
		points := t.Verticies()
		g = append(g, c.MoveTo(points[0])...)
		g = append(g, c.LineTo(points[1:]...)...)
		return g, vectorTile.Tile_LINESTRING, nil

	case geom.MultiLineString:
		lines := t.LineStrings()
		for _, l := range lines {
			points := geom.LineString(l).Verticies()
			g = append(g, c.MoveTo(points[0])...)
			g = append(g, c.LineTo(points[1:]...)...)
		}
		return g, vectorTile.Tile_LINESTRING, nil

	case geom.Polygon:
		// TODO: Right now c.ScaleGeo() never returns a Polygon, so this is dead code.
		lines := t.LinearRings()
		for _, l := range lines {
			points := geom.LineString(l).Verticies()
			g = append(g, c.MoveTo(points[0])...)
			g = append(g, c.LineTo(points[1:]...)...)
			g = append(g, c.ClosePath())
		}
		return g, vectorTile.Tile_POLYGON, nil

	case geom.MultiPolygon:
		polygons := t.Polygons()
		for _, p := range polygons {
			lines := geom.Polygon(p).LinearRings()
			for _, l := range lines {
				points := geom.LineString(l).Verticies()
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
				if vt == nil {
					// ignore nil types
					continue
				}
				return keyMap, valMap, fmt.Errorf("unsupported type for value(%v) with key(%v) in tags for feature %v.", vt, k, f)

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

		kidx, vidx = -1, -1 // Set to known not found value.

		for i, k := range keyMap {
			if k != key {
				continue // move to the next key
			}
			kidx = int64(i)
			break // we found a match
		}

		if kidx == -1 {
			log.Errorf("did not find key (%v) in keymap.", key)
			return tags, fmt.Errorf("did not find key (%v) in keymap.", key)
		}

		// if val is nil we skip it for now
		// https://github.com/mapbox/vector-tile-spec/issues/62
		if val == nil {
			continue
		}

		for i, v := range valueMap {
			switch tv := val.(type) {
			default:
				return tags, fmt.Errorf("value (%[1]v) of type (%[1]T) for key (%[2]v) is not supported.", tv, key)
			case string:
				vmt, ok := v.(string) // Make sure the type of the Value map matches the type of the Tag's value
				if !ok || vmt != tv { // and that the values match
					continue // if they don't match move to the next value.
				}
			case fmt.Stringer:
				vmt, ok := v.(fmt.Stringer)
				if !ok || vmt.String() != tv.String() {
					continue
				}
			case int:
				vmt, ok := v.(int)
				if !ok || vmt != tv {
					continue
				}
			case int8:
				vmt, ok := v.(int8)
				if !ok || vmt != tv {
					continue
				}
			case int16:
				vmt, ok := v.(int16)
				if !ok || vmt != tv {
					continue
				}
			case int32:
				vmt, ok := v.(int32)
				if !ok || vmt != tv {
					continue
				}
			case int64:
				vmt, ok := v.(int64)
				if !ok || vmt != tv {
					continue
				}
			case uint:
				vmt, ok := v.(uint)
				if !ok || vmt != tv {
					continue
				}
			case uint8:
				vmt, ok := v.(uint8)
				if !ok || vmt != tv {
					continue
				}
			case uint16:
				vmt, ok := v.(uint16)
				if !ok || vmt != tv {
					continue
				}
			case uint32:
				vmt, ok := v.(uint32)
				if !ok || vmt != tv {
					continue
				}
			case uint64:
				vmt, ok := v.(uint64)
				if !ok || vmt != tv {
					continue
				}

			case float32:
				vmt, ok := v.(float32)
				if !ok || vmt != tv {
					continue
				}
			case float64:
				vmt, ok := v.(float64)
				if !ok || vmt != tv {
					continue
				}
			case bool:
				vmt, ok := v.(bool)
				if !ok || vmt != tv {
					continue
				}
			} // Values Switch Statement
			// if the values match let's record the index.
			vidx = int64(i)
			break // we found our value no need to continue on.
		} // range on value

		if vidx == -1 { // None of the values matched.
			return tags, fmt.Errorf("did not find a value: %v in valuemap.", val)
		}
		tags = append(tags, uint32(kidx), uint32(vidx))
	} // Move to the next tag key and value.

	return tags, nil
}
