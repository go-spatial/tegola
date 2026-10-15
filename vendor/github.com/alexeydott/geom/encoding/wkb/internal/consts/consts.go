package consts

//  geometry types
// http://edndoc.esri.com/arcsde/9.1/general_topics/wkb_representation.htm
const (
	Point           uint32 = 1
	LineString      uint32 = 2
	Polygon         uint32 = 3
	MultiPoint      uint32 = 4
	MultiLineString uint32 = 5
	MultiPolygon    uint32 = 6
	Collection      uint32 = 7
)
