package mvt

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"

	"github.com/arolek/p"
	"github.com/go-spatial/geom"
	vectorTile "github.com/go-spatial/geom/encoding/mvt/vector_tile"
	"github.com/go-spatial/geom/winding"
	"github.com/golang/protobuf/proto"
)

// TileGeomCollection returns all geometries in a tile
// as a collection
func TileGeomCollection(tile *Tile) geom.Collection {
	ret := geom.Collection{}
	for _, v := range tile.layers {
		for _, vv := range v.features {
			ret = append(ret, vv.Geometry)
		}
	}

	return ret
}

// Decode reads all the data from r and decodes the MVT tile into a Tile
// TODO(ear7h): handle tile tags
func Decode(r io.Reader) (*Tile, error) {
	byt, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}

	return DecodeByte(byt)
}

// DecodeByte decodes the MVT encoded bytes into a Tile.
// TODO(ear7h): handle tile tags
func DecodeByte(b []byte) (*Tile, error) {
	vtile := new(vectorTile.Tile)

	err := proto.Unmarshal(b, vtile)
	if err != nil {
		return nil, err
	}

	ret := new(Tile)
	ret.layers = make([]Layer, len(vtile.Layers))

	for i, v := range vtile.Layers {
		err = decodeLayer(v, &ret.layers[i])
		if err != nil {
			return nil, err
		}
	}

	return ret, nil
}

func decodeLayer(pb *vectorTile.Tile_Layer, dst *Layer) error {
	dst.Name = *pb.Name
	dst.extent = p.Int(int(*pb.Extent))

	dst.features = make([]Feature, len(pb.Features))

	for i, v := range pb.Features {
		err := decodeFeature(v, &dst.features[i])
		if err != nil {
			return err
		}
	}

	return nil
}

func decodeFeature(pb *vectorTile.Tile_Feature, dst *Feature) error {
	dst.ID = pb.Id
	// TODO tag support
	var err error
	dst.Geometry, err = DecodeGeometry(*pb.Type, pb.Geometry)
	return err
}

func DecodeGeometry(gtype vectorTile.Tile_GeomType, b []uint32) (geom.Geometry, error) {

	switch gtype {
	case vectorTile.Tile_LINESTRING:
		return decodeLineString(b)
	case vectorTile.Tile_POINT:
		return decodePoint(b)
	case vectorTile.Tile_POLYGON:
		return decodePoly(b)
	default:
		panic("unreachable")
	}
}

func decodePoint(buf []uint32) (geom.Geometry, error) {
	ret := [][2]float64{}
	curs := decodeCursor{}

	if len(buf) > 0 {
		cmd := Command(buf[0])
		buf = buf[1:]

		if len(buf) < cmd.Count()*2 {
			return nil, fmt.Errorf("not enough integers (%v) for %s", len(buf), cmd)
		}

		switch cmd.ID() {
		case cmdMoveTo:
			ret = curs.decodeNPoints(cmd.Count(), buf, false)
			buf = buf[cmd.Count()*2:]

		default:
			return nil, fmt.Errorf("invalid command for POINT, %s", cmd)
		}
	}

	if len(buf) != 0 {
		fmt.Println(buf)
		return ret, ErrExtraData
	}

	switch len(ret) {
	case 0:
		return nil, nil
	case 1:
		return geom.Point(ret[0]), nil
	default:
		return geom.MultiPoint(ret), nil
	}
}

var ErrExtraData = errors.New("mvt: invalid extra data")

func decodeLineString(buf []uint32) (geom.Geometry, error) {
	ret := [][][2]float64{}
	curs := decodeCursor{}
	var lastCmd Command
	var cmd Command

	for ; len(buf) > 0; lastCmd = cmd {
		cmd = Command(buf[0])
		buf = buf[1:]

		if len(buf) < cmd.Count()*2 {
			return nil, fmt.Errorf("not enough integers (%v) for %s", len(buf), cmd)
		}

		switch cmd.ID() {
		case cmdMoveTo:
			if lastCmd != 0 && lastCmd.ID() != cmdLineTo {
				return nil, fmt.Errorf("%v cannot follow %v for LINESTRING", cmd, lastCmd)
			}

			if cmd.Count() != 1 {
				// return error
			}

			curs.decodePoint(buf[0], buf[1])
			buf = buf[2:]

		case cmdLineTo:
			if lastCmd.ID() != cmdMoveTo {
				return nil, fmt.Errorf("%v cannot follow %v for LINESTRING", cmd, lastCmd)
			}

			if cmd.Count() <= 0 {
				return nil, fmt.Errorf("%v must have count > 0 for LINESTRING", cmd)
			}

			ln := curs.decodeNPoints(cmd.Count(), buf, true)
			buf = buf[cmd.Count()*2:]
			ret = append(ret, ln)

		default:
			return nil, fmt.Errorf("invalid command for LINESTRING, %s", cmd)
		}
	}

	if len(buf) != 0 {
		return ret, ErrExtraData
	}

	switch len(ret) {
	case 0:
		panic("unreachable")
	case 1:
		return geom.LineString(ret[0]), nil
	default:
		return geom.MultiLineString(ret), nil
	}
}

func decodePoly(buf []uint32) (geom.Geometry, error) {
	ret := [][][][2]float64{}
	curs := decodeCursor{}
	var lastCmd Command
	var cmd Command

	for ; len(buf) > 0; lastCmd = cmd {
		cmd = Command(buf[0])
		buf = buf[1:]

		if cmd.ID() != cmdClosePath && len(buf) < cmd.Count()*2 {
			return nil, fmt.Errorf("not enough integers (%v) for %s", len(buf), cmd)
		}

		switch cmd.ID() {
		case cmdMoveTo:
			if lastCmd != 0 && lastCmd.ID() != cmdClosePath {
				return nil, fmt.Errorf("%v cannot follow %v for POLYGON", cmd, lastCmd)
			}

			if cmd.Count() != 1 {
				// cannot be 1
			}

			curs.decodePoint(buf[0], buf[1])
			buf = buf[2:]

		case cmdLineTo:
			if lastCmd.ID() != cmdMoveTo {
				return nil, fmt.Errorf("%v cannot follow %v for POLYGON", cmd, lastCmd)
			}

			if cmd.Count() <= 1 {
				return nil, fmt.Errorf("%v must have count > 1 for POLYGON", cmd)
			}

			ln := curs.decodeNPoints(cmd.Count(), buf, true)
			buf = buf[cmd.Count()*2:]

			if (winding.Order{YPositiveDown: true}).OfPoints(ln...).IsClockwise() {
				ret = append(ret, nil)
			} else if len(ret) == 0 {
				return nil, fmt.Errorf("first ring of POLYGON must be an exterior ring")
			}

			polyIdx := len(ret) - 1
			ret[polyIdx] = append(ret[polyIdx], ln)

		case cmdClosePath:
			if lastCmd.ID() != cmdLineTo {
				return nil, fmt.Errorf("%v cannot follow %v for POLYGON", cmd, lastCmd)
			}
		}
	}

	if len(buf) != 0 {
		return ret, ErrExtraData
	}

	switch len(ret) {
	case 0:
		panic("unreachable")
	case 1:
		return geom.Polygon(ret[0]), nil
	default:
		return geom.MultiPolygon(ret), nil
	}
}

type decodeCursor struct {
	x, y float64
}

// n and len(pts) should be error checked before this function
// this is for more informative context on errors
func (c *decodeCursor) decodeNPoints(n int, pts []uint32, encHere bool) [][2]float64 {
	nd := 0

	if encHere {
		nd = 1
	}

	ret := make([][2]float64, n+nd)

	if encHere {
		ret[0] = [2]float64{c.x, c.y}
	}

	for i := 0; i < n; i++ {
		ret[i+nd] = c.decodePoint(pts[i*2], pts[i*2+1])
	}

	return ret
}

// decodes the zig zag encoded uint32's, moves the cursor
func (c *decodeCursor) decodePoint(x, y uint32) [2]float64 {
	c.x += float64(decodeZigZag(x))
	c.y += float64(decodeZigZag(y))
	return [2]float64{c.x, c.y}
}

// decodes the zig zag encoded int32
func decodeZigZag(i uint32) int32 {
	return int32((i >> 1) ^ (-(i & 1)))
}
