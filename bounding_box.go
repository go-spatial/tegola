package tegola

//BoundingBox defines the extent of a tile
type BoundingBox struct {
	Minx, Miny, Maxx, Maxy float64
	// Epsilon is the tolerance for the simplification function.
	Epsilon float64
	// X,Y,Z are just for debug and display purposes.
	X, Y, Z int
	HasXYZ  bool
}
