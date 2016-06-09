//	mapbox vetor tile (MVT) encoding package for specification version 2.1
//	specifcation https://github.com/mapbox/vector-tile-spec
package mvt

//	version
const Version = 2.1

//	command integers
const (
	CommandMoveTo uint32 = 1
	CommandLineTo        = 2
	CommandClose         = 7
)

//	geometry types
const (
	GeoPoint      string = "POINT"
	GeoLineString        = "LINESTRING"
	GeoPolygon           = "POLYGON"
	GeoUnknown           = "UNKNOWN"
)

type Geometry interface {
	Marshal() []byte
	Type() string
}

//	encode an integer to a parameter integer
func EncodeParamInt(val int32) int32 {
	return (val << 1) ^ (val >> 31)
}

//	decode a parameter integer to integer
func DecodeParamInt(val uint32) uint32 {
	return ((val >> 1) ^ (-(val & 1)))
}
