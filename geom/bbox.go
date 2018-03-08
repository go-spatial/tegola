package geom

import (
	"math"
)

// BoundingBoxer represents an interface that returns a boundbox.
type BoundingBoxer interface {
	BBox() (bbox [4]float64)
}

type MinMaxer interface {
	MinX() float64
	MinY() float64
	MaxX() float64
	MaxY() float64
}

// BoundingBox represents the minx, miny, maxx and maxy
type BoundingBox [4]float64

/* ========================= ATTRIBUTES ========================= */

// Vertices return the verticies of the Bounding Box. The verticies are ordered in the following maner.
// (minx,miny), (maxx,miny), (maxx,maxy), (minx,maxy)
func (bb *BoundingBox) Vertices() [][2]float64 {
	return [][2]float64{
		[2]float64{bb.MinX(), bb.MinY()},
		[2]float64{bb.MaxX(), bb.MinY()},
		[2]float64{bb.MaxX(), bb.MaxY()},
		[2]float64{bb.MinX(), bb.MaxY()},
	}
}

// ClockwiseFunc returns weather the set of points should be considered clockwise or counterclockwise. The last point is not the same as the first point, and the function should connect these poins as needed.
type ClockwiseFunc func(...[2]float64) bool

// Edges returns the clockwise order of the edges that make up the extent.
func (bb *BoundingBox) Edges(cwfn ClockwiseFunc) [][2][2]float64 {
	v := bb.Vertices()
	if cwfn != nil && !cwfn(v...) {
		v[0], v[1], v[2], v[3] = v[3], v[2], v[1], v[0]
	}
	return [][2][2]float64{
		[2][2]float64{v[0], v[1]},
		[2][2]float64{v[1], v[2]},
		[2][2]float64{v[2], v[3]},
		[2][2]float64{v[3], v[0]},
	}
}

// MaxX is the larger of the x values.
func (bb *BoundingBox) MaxX() float64 {
	if bb == nil {
		return math.MaxFloat64
	}
	return bb[2]
}

// MinX  is the smaller of the x values.
func (bb *BoundingBox) MinX() float64 {
	if bb == nil {
		return -math.MaxFloat64
	}
	return bb[0]
}

// MaxY is the larger of the y values.
func (bb *BoundingBox) MaxY() float64 {
	if bb == nil {
		return math.MaxFloat64
	}
	return bb[3]
}

// MinY is the smaller of the y values.
func (bb *BoundingBox) MinY() float64 {
	if bb == nil {
		return -math.MaxFloat64
	}
	return bb[1]
}

// TODO (gdey): look at how to have this function take into account the dpi.
func (bb *BoundingBox) XSpan() float64 {
	if bb == nil {
		return math.Inf(1)
	}
	return bb[2] - bb[0]
}
func (bb *BoundingBox) YSpan() float64 {
	if bb == nil {
		return math.Inf(1)
	}
	return bb[3] - bb[1]
}

func (bb *BoundingBox) BBox() [4]float64 {
	return [4]float64{bb.MinX(), bb.MinY(), bb.MaxX(), bb.MaxY()}
}

/* ========================= EXPANDING BOUNDING BOX ========================= */
// Add will expand the boundong box to contain the given bounding box.
func (bb *BoundingBox) Add(bbox MinMaxer) {
	if bb == nil {
		return
	}
	if bb[0] > bbox.MinX() {
		bb[0] = bbox.MinX()
	}
	if bb[1] > bbox.MinY() {
		bb[1] = bbox.MinY()
	}
	if bb[2] < bbox.MaxX() {
		bb[2] = bbox.MaxX()
	}
	if bb[3] < bbox.MaxY() {
		bb[3] = bbox.MaxY()
	}
}

// AddPoints will expand the bounding box to contain the given points.
func (bb *BoundingBox) AddPoints(points ...[2]float64) {
	// A nil bbox is all encompassing.
	if bb == nil {
		return
	}
	if len(points) == 0 {
		return
	}
	bbox := NewBBox(points...)
	bb.Add(bbox)
}

// AsPolygon will return the bounding box as a Polygon
func (bb *BoundingBox) AsPolygon() Polygon { return Polygon{bb.Vertices()} }

// Area returns the area of the bounding box, if the bounding box is nil, it will return 0
func (bb *BoundingBox) Area() float64 {
	return math.Abs((bb.MaxY() - bb.MinY()) * (bb.MaxX() - bb.MinX()))
}

// NewBBox returns MinX, MinY, MaxX, MaxY
func NewBBox(points ...[2]float64) *BoundingBox {
	var xy [2]float64
	if len(points) == 0 {
		return nil
	}

	bbox := BoundingBox{points[0][0], points[0][1], points[0][0], points[0][1]}
	if len(points) == 1 {
		return &bbox
	}
	for i := 1; i < len(points); i++ {
		xy = points[i]
		// Check the x coords
		switch {
		case xy[0] < bbox[0]:
			bbox[0] = xy[0]
		case xy[0] > bbox[2]:
			bbox[2] = xy[0]
		}
		// Check the y coords
		switch {
		case xy[1] < bbox[1]:
			bbox[1] = xy[1]
		case xy[1] > bbox[3]:
			bbox[3] = xy[1]
		}
	}
	return &bbox
}

// Contains will return weather the given bounding box is inside of the bounding box.
func (bb *BoundingBox) Contains(nbb MinMaxer) bool {
	// Nil bounding boxes contains the world.
	if bb == nil {
		return true
	}
	if nbb == nil {
		return false
	}
	return bb.MinX() <= nbb.MinX() &&
		bb.MaxX() >= nbb.MaxX() &&
		bb.MinY() <= nbb.MinY() &&
		bb.MaxY() >= nbb.MaxY()
}

// ContainsPoint will return weather the given point is inside of the bounding box.
func (bb *BoundingBox) ContainsPoint(pt [2]float64) bool {
	if bb == nil {
		return true
	}
	return bb.MinX() <= pt[0] && pt[0] <= bb.MaxX() &&
		bb.MinY() <= pt[1] && pt[1] <= bb.MaxY()
}

// ContainsLine will return weather the given line completely inside of the bounding box.
func (bb *BoundingBox) ContainsLine(l [2][2]float64) bool {
	if bb == nil {
		return true
	}
	return bb.ContainsPoint(l[0]) && bb.ContainsPoint(l[1])
}

// ScaleBy will scale the points in the bounding box by the given scale factor.
func (bb *BoundingBox) ScaleBy(s float64) *BoundingBox {
	if bb == nil {
		return nil
	}
	return NewBBox(
		[2]float64{bb[0] * s, bb[1] * s},
		[2]float64{bb[2] * s, bb[3] * s},
	)
}

// ExpandBy will expand bounding box by the given factor.
func (bb *BoundingBox) ExpandBy(s float64) *BoundingBox {
	if bb == nil {
		return nil
	}
	return NewBBox(
		[2]float64{bb[0] - s, bb[1] - s},
		[2]float64{bb[2] + s, bb[3] + s},
	)
}

func (bb *BoundingBox) Clone() *BoundingBox {
	if bb == nil {
		return nil
	}
	return &BoundingBox{bb[0], bb[1], bb[2], bb[3]}
}

// Intersect will returns a new bounding box that is the intersect of the two bounding boxes.
//
//	+-------------------------+
//	|                         |
//	|       A      +----------+------+
//	|              |//////////|      |
//	|              |/// C ////|      |
//	|              |//////////|      |
//	+--------------+----------+      |
//	               |             B   |
//	               +-----------------+
//	For example the for the above Box A intersects Box B at the area surround by C.
//
// If the Boxes don't intersect does will be false, otherwise ibb will be the intersect.
func (bb *BoundingBox) Intersect(nbb *BoundingBox) (ibb *BoundingBox, does bool) {
	// if bb in nil, then the intersect is nbb.
	if bb == nil {
		return nbb.Clone(), true
	}
	if nbb == nil {
		return bb.Clone(), true
	}

	minx := bb.MinX()
	if minx < nbb.MinX() {
		minx = nbb.MinX()
	}
	maxx := bb.MaxX()
	if maxx > nbb.MaxX() {
		maxx = nbb.MaxX()
	}
	// The boxes don't intersect.
	if minx >= maxx {
		return nil, false
	}
	miny := bb.MinY()
	if miny < nbb.MinY() {
		miny = nbb.MinY()
	}
	maxy := bb.MaxY()
	if maxy > nbb.MaxY() {
		maxy = nbb.MaxY()
	}

	// The boxes don't intersect.
	if miny >= maxy {
		return nil, false
	}
	return &BoundingBox{minx, miny, maxx, maxy}, true
}
