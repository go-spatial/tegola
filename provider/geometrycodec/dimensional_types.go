package geometrycodec

// MultiPolygonZ preserves the XYZ MultiPolygon family missing from geom's
// concrete types. Rings contain XYZ positions; measures are not supported.
type MultiPolygonZ [][][][3]float64
