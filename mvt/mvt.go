//	mapbox vetor tile (MVT) encoding package for specification version 2.1
//	specifcation https://github.com/mapbox/vector-tile-spec
package mvt

//	version
const Version = 2.1

//	CommandIntegers
const (
	CommandMoveTo    uint32 = 1
	CommandLineTo           = 2
	CommandClosePath        = 7
)

//	geometry types
const (
	GeoPoint      string = "POINT"
	GeoLinestring        = "LINESTRING"
	GeoPolygon           = "POLYGON"
	GeoUnknown           = "UNKNOWN"
)

type Geometry interface {
	Marshal() []byte
	Type() string
}

//	encode a CommandInteger
func EncodeCommandInt(id uint32, count uint32) uint32 {
	return (id & 0x7) | (count << 3)
}

//	encode a ParameterInteger
func EncodeParamInt(val int32) int32 {
	return (val << 1) ^ (val >> 31)
}

//	decode a ParameterInteger
func DecodeParamInt(val uint32) uint32 {
	return ((val >> 1) ^ (-(val & 1)))
}
