package util

// BoundingBoxer represents an interface that returns a boundbox.
type BoundingBoxer interface {
	BBox() (bbox [2][2]float64)
}

// BoundingBox represents X1, Y1, X2, Y2 (LL, UR) of a geometry
type BoundingBox [2][2]float64

const (
	x = 0
	y = 1
)

// BBox returns X1, Y1, X2, Y2 (LL, UR) for the input points
func BBox(points ...[2]float64) (bbox BoundingBox) {
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
		case xy[x] < bbox[0][x]:
			bbox[0][x] = xy[x]
		case xy[x] > bbox[1][x]:
			bbox[1][0] = xy[x]
		}
		// Check the y coords
		switch {
		case xy[y] < bbox[0][y]:
			bbox[0][y] = xy[y]
		case xy[y] > bbox[1][y]:
			bbox[1][y] = xy[y]
		}
	}
	return bbox
}
