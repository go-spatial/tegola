package geom

import (
	"math"
)

// Extenter represents an interface that returns a boundbox.
type Extenter interface {
	Extent() (extent [4]float64)
}

type MinMaxer interface {
	MinX() float64
	MinY() float64
	MaxX() float64
	MaxY() float64
}

// Extent represents the minx, miny, maxx and maxy
// A nil extent represents the whole universe.
type Extent [4]float64

/* ========================= ATTRIBUTES ========================= */

// Vertices return the verticies of the Bounding Box. The verticies are ordered in the following maner.
// (minx,miny), (maxx,miny), (maxx,maxy), (minx,maxy)
func (e *Extent) Vertices() [][2]float64 {
	return [][2]float64{
		{e.MinX(), e.MinY()},
		{e.MaxX(), e.MinY()},
		{e.MaxX(), e.MaxY()},
		{e.MinX(), e.MaxY()},
	}
}

// ClockwiseFunc returns weather the set of points should be considered clockwise or counterclockwise. The last point is not the same as the first point, and the function should connect these points as needed.
type ClockwiseFunc func(...[2]float64) bool

// Edges returns the clockwise order of the edges that make up the extent.
func (e *Extent) Edges(cwfn ClockwiseFunc) [][2][2]float64 {
	v := e.Vertices()
	if cwfn != nil && !cwfn(v...) {
		v[0], v[1], v[2], v[3] = v[3], v[2], v[1], v[0]
	}
	return [][2][2]float64{
		{v[0], v[1]},
		{v[1], v[2]},
		{v[2], v[3]},
		{v[3], v[0]},
	}
}

// MaxX is the larger of the x values.
func (e *Extent) MaxX() float64 {
	if e == nil {
		return math.MaxFloat64
	}
	return e[2]
}

// MinX  is the smaller of the x values.
func (e *Extent) MinX() float64 {
	if e == nil {
		return -math.MaxFloat64
	}
	return e[0]
}

// MaxY is the larger of the y values.
func (e *Extent) MaxY() float64 {
	if e == nil {
		return math.MaxFloat64
	}
	return e[3]
}

// MinY is the smaller of the y values.
func (e *Extent) MinY() float64 {
	if e == nil {
		return -math.MaxFloat64
	}
	return e[1]
}

// Min returns the (MinX, MinY) values
func (e *Extent) Min() [2]float64 {
	return [2]float64{e[0], e[1]}
}

// Max returns the (MaxX, MaxY) values
func (e *Extent) Max() [2]float64 {
	return [2]float64{e[2], e[3]}
}

// TODO (gdey): look at how to have this function take into account the dpi.
func (e *Extent) XSpan() float64 {
	if e == nil {
		return math.Inf(1)
	}
	return e[2] - e[0]
}
func (e *Extent) YSpan() float64 {
	if e == nil {
		return math.Inf(1)
	}
	return e[3] - e[1]
}

func (e *Extent) Extent() [4]float64 {
	return [4]float64{e.MinX(), e.MinY(), e.MaxX(), e.MaxY()}
}

/* ========================= EXPANDING BOUNDING BOX ========================= */
// Add will expand the extent to contain the given extent.
func (e *Extent) Add(extent MinMaxer) {
	if e == nil {
		return
	}
	if e[0] > extent.MinX() {
		e[0] = extent.MinX()
	}
	if e[1] > extent.MinY() {
		e[1] = extent.MinY()
	}
	if e[2] < extent.MaxX() {
		e[2] = extent.MaxX()
	}
	if e[3] < extent.MaxY() {
		e[3] = extent.MaxY()
	}
}

// AddPoints will expand the extent to contain the given points.
func (e *Extent) AddPoints(points ...[2]float64) {
	// A nil extent is all encompassing.
	if e == nil {
		return
	}
	if len(points) == 0 {
		return
	}
	extent := NewExtent(points...)
	e.Add(extent)
}

func (e *Extent) AddPointers(pts ...Pointer) {
	for i := range pts {
		e.AddPoints(pts[i].XY())
	}
}

// AddPointer expands the specified envelop to contain p.
func (e *Extent) AddGeometry(g Geometry) error {
	points, err := GetCoordinates(g)
	if err != nil {
		return err
	}

	for i := range points {
		e.AddPointers(points[i])
	}
	return nil
}

// AsPolygon will return the extent as a Polygon
func (e *Extent) AsPolygon() Polygon { return Polygon{e.Vertices()} }

// Area returns the area of the extent, if the extent is nil, it will return 0
func (e *Extent) Area() float64 {
	return math.Abs((e.MaxY() - e.MinY()) * (e.MaxX() - e.MinX()))
}

// NewExtent returns an Extent for the provided points; in following format [4]float64{ MinX, MinY, MaxX, MaxY }
func NewExtent(points ...[2]float64) *Extent {
	var xy [2]float64
	if len(points) == 0 {
		return nil
	}

	extent := Extent{points[0][0], points[0][1], points[0][0], points[0][1]}
	if len(points) == 1 {
		return &extent
	}
	for i := 1; i < len(points); i++ {
		xy = points[i]
		// Check the x coords
		switch {
		case xy[0] < extent[0]:
			extent[0] = xy[0]
		case xy[0] > extent[2]:
			extent[2] = xy[0]
		}
		// Check the y coords
		switch {
		case xy[1] < extent[1]:
			extent[1] = xy[1]
		case xy[1] > extent[3]:
			extent[3] = xy[1]
		}
	}
	return &extent
}

// Contains will return whether the given  extent is inside of the  extent.
func (e *Extent) Contains(ne MinMaxer) bool {
	// Nil extent contains the world.
	if e == nil {
		return true
	}
	if ne == nil {
		return false
	}
	return e.MinX() <= ne.MinX() &&
		e.MaxX() >= ne.MaxX() &&
		e.MinY() <= ne.MinY() &&
		e.MaxY() >= ne.MaxY()
}

// ContainsPoint will return whether the given point is inside of the extent.
func (e *Extent) ContainsPoint(pt [2]float64) bool {
	if e == nil {
		return true
	}
	return e.MinX() <= pt[0] && pt[0] <= e.MaxX() &&
		e.MinY() <= pt[1] && pt[1] <= e.MaxY()
}

// ContainsLine will return weather the given line completely inside of the extent.
func (e *Extent) ContainsLine(l [2][2]float64) bool {
	if e == nil {
		return true
	}
	return e.ContainsPoint(l[0]) && e.ContainsPoint(l[1])
}

// ScaleBy will scale the points in the extent by the given scale factor.
func (e *Extent) ScaleBy(s float64) *Extent {
	if e == nil {
		return nil
	}
	return NewExtent(
		[2]float64{e[0] * s, e[1] * s},
		[2]float64{e[2] * s, e[3] * s},
	)
}

// ExpandBy will expand extent by the given factor.
func (e *Extent) ExpandBy(s float64) *Extent {
	if e == nil {
		return nil
	}
	return NewExtent(
		[2]float64{e[0] - s, e[1] - s},
		[2]float64{e[2] + s, e[3] + s},
	)
}

func (e *Extent) Clone() *Extent {
	if e == nil {
		return nil
	}
	return &Extent{e[0], e[1], e[2], e[3]}
}

// Intersect will return a new extent that is the intersect of the two extents.
//
// +-------------------------+
// |                         |
// |       A      +----------+------+
// |              |//////////|      |
// |              |/// C ////|      |
// |              |//////////|      |
// +--------------+----------+      |
//                |             B   |
//                +-----------------+
// For example the for the above Box A intersects Box B at the area surround by C.
//
// If the Boxes don't intersect does will be false, otherwise ibb will be the intersect.
func (e *Extent) Intersect(ne *Extent) (*Extent, bool) {
	// if e in nil, then the intersect is ne. As a nil extent is the whole universe.
	if e == nil {
		return ne.Clone(), true
	}
	// if ne is nil, then the intersect is e. As a nil extent is the whole universe.
	if ne == nil {
		return e.Clone(), true
	}

	minx := e.MinX()
	if minx < ne.MinX() {
		minx = ne.MinX()
	}
	maxx := e.MaxX()
	if maxx > ne.MaxX() {
		maxx = ne.MaxX()
	}
	// The boxes don't intersect.
	if minx >= maxx {
		return nil, false
	}
	miny := e.MinY()
	if miny < ne.MinY() {
		miny = ne.MinY()
	}
	maxy := e.MaxY()
	if maxy > ne.MaxY() {
		maxy = ne.MaxY()
	}

	// The boxes don't intersect.
	if miny >= maxy {
		return nil, false
	}
	return &Extent{minx, miny, maxx, maxy}, true
}
