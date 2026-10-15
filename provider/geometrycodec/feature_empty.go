package geometrycodec

import "github.com/alexeydott/geom"

// FeatureSpatialGeometryEmpty reports whether a supported feature geometry is
// nil or wholly empty, including nested XY and XYZ collections. It does not
// validate geometry; callers must validate before normalizing empty geometry.
func FeatureSpatialGeometryEmpty(g geom.Geometry) bool {
	return spatialEmpty(g)
}
