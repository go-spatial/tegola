package geom

// BoundingBoxer represents an interface that returns a boundbox.
type BoundingBoxer interface {
	BBox() (bbox [2][2]float64)
}

// BoundingBox represents X1, Y1, X2, Y2 (LL, UR) of a geometry
type BoundingBox [2][2]float64

/* ========================= ATTRIBUTES ========================= */

// TopLeft point of the bounding box.
func (bb BoundingBox) TopLeft() [2]float64 { return bb[0] }

// TopRight point of the bounding box.
func (bb BoundingBox) TopRight() [2]float64 { return [2]float64{bb[1][0], bb[0][1]} }

// BottomLeft point of the bounding box.
func (bb BoundingBox) BottomLeft() [2]float64 { return [2]float64{bb[0][0], bb[1][1]} }

// BottomRight point of the bounding box.
func (bb BoundingBox) BottomRight() [2]float64 { return bb[1] }

// Verticies return the verticies of the Bounding Box.
func (bb BoundingBox) Verticies() [][2]float64 {
	return [][2]float64{
		bb.TopLeft(),
		bb.TopRight(),
		bb.BottomRight(),
		bb.BottomLeft(),
	}
}

// Edges returns the clockwise order of the edges that make up the extent.
func (bb BoundingBox) Edges() [][2][2]float64 {
	return [][2][2]float64{
		{bb.TopLeft(), bb.TopRight()},
		{bb.TopRight(), bb.BottomRight()},
		{bb.BottomRight(), bb.BottomLeft()},
		{bb.BottomLeft(), bb.TopLeft()},
	}
}

// LREdges are the edges starting witht he left most edge to the lower right edge.
func (bb BoundingBox) LREdges() [][2][2]float64 {
	return [][2][2]float64{
		{bb.TopLeft(), bb.TopRight()},
		{bb.TopLeft(), bb.BottomLeft()},
		{bb.BottomLeft(), bb.BottomRight()},
		{bb.TopRight(), bb.BottomRight()},
	}
}

// MaxX is the larger of the x values.
func (bb BoundingBox) MaxX() float64 {
	if bb[0][0] >= bb[1][0] {
		return bb[0][0]
	}
	return bb[1][0]
}

// MinX  is the smaller of the x values.
func (bb BoundingBox) MinX() float64 {
	if bb[0][0] <= bb[1][0] {
		return bb[0][0]
	}
	return bb[1][0]
}

// MaxY is the larger of the y values.
func (bb BoundingBox) MaxY() float64 {
	if bb[0][1] >= bb[1][1] {
		return bb[0][1]
	}
	return bb[1][1]
}

// MinY is the smaller of the y values.
func (bb BoundingBox) MinY() float64 {
	if bb[0][1] <= bb[1][1] {
		return bb[0][1]
	}
	return bb[1][1]
}

/* ========================= EXPANDING BOUNDING BOX ========================= */
// Add will expand the boundong box to contain the given bounding box.
func (bb *BoundingBox) Add(bbox BoundingBox) {
	if bb == nil {
		return
	}
	// min :  c = x=0;y=1
	for c := 0; c <= 1; c++ {
		if bb[0][c] > bbox[0][c] {
			bb[0][c] = bbox[0][c]
		}
	}
	//
	// max : c = x=0;y=1
	for c := 0; c <= 1; c++ {
		if bb[1][c] < bbox[1][c] {
			bb[1][c] = bbox[1][c]
		}
	}
}

// AddPoints will expand the bounding box to contain the given points.
func (bb *BoundingBox) AddPoints(points ...[2]float64) {
	if bb == nil {
		return
	}
	bbox := NewBBox(points...)
	bb.Add(bbox)
}

// Contains will return weather the given point is inside of the bounding box.
func (bb *BoundingBox) Contains(pt [2]float64) (v bool) {
	if bb == nil {
		return false
	}
	// Check the X coords.
	if pt[0] < bb[0][0] || pt[0] > bb[1][0] {
		return false
	}
	// Check the Y coords.
	if pt[1] < bb[0][1] || pt[1] > bb[1][1] {
		return false
	}
	return true
}

// NewBBox returns X1, Y1, X2, Y2 (LL, UR) for the input points
func NewBBox(points ...[2]float64) (bbox BoundingBox) {
	var xy [2]float64
	if len(points) == 0 {
		return bbox
	}
	bbox[0] = points[0]
	bbox[1] = points[0]
	if len(points) == 1 {
		return bbox
	}
	for i := 1; i < len(points); i++ {
		xy = points[i]
		// Check the x coords
		switch {
		case xy[0] < bbox[0][0]:
			bbox[0][0] = xy[0]
		case xy[0] > bbox[1][0]:
			bbox[1][0] = xy[0]
		}
		// Check the y coords
		switch {
		case xy[1] < bbox[0][1]:
			bbox[0][1] = xy[1]
		case xy[1] > bbox[1][1]:
			bbox[1][1] = xy[1]
		}
	}
	return bbox
}
