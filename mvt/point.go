package mvt

type Point struct {
	X int32
	Y int32
}

//	marshal a point relative to a point
func (p *Point) Marshal() []byte {
	return []byte{
		byte(EncodeParamInt(p.X)),
		byte(EncodeParamInt(p.Y)),
	}
}

//	returns a new point that is the offset of this point
//	relative to another point
func (p *Point) Offset(p2 Point) Point {
	//	check for (0,0)
	if p2.X == 0 && p2.Y == 0 {
		return *p
	}

	return Point{
		X: p2.X - p.X,
		Y: p2.Y - p.Y,
	}
}

//	return the type as defined in the specification
func (p *Point) Type() string {
	return "POINT"
}
