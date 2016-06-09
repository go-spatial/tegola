package mvt

type Point struct {
	X int32
	Y int32
}

//	marshal a point relative to a provided point
func (p *Point) Marshal(p2 Point) []byte {
	//	relative point
	offset := p.Offset(p2)

	return []byte{
		byte(EncodeParamInt(offset.X)),
		byte(EncodeParamInt(offset.Y)),
	}
}

//	TODO: implement counter clockwise offset for multi polygons

//	returns a new point which is an offset of the provied point
func (p *Point) Offset(p2 Point) Point {
	//	check for (0,0)
	if p2.X == 0 && p2.Y == 0 {
		return *p
	}

	return Point{
		X: -(p2.X - p.X),
		Y: -(p2.Y - p.Y),
	}
}

//	return the type as defined in the specification
func (p *Point) Type() string {
	return GeoPoint
}
