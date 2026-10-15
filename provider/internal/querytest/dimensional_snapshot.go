package querytest

import (
	"github.com/alexeydott/geom"
	"github.com/alexeydott/tegola/provider/geometrycodec"
)

func cloneXYZLines(lines [][][3]float64) [][][3]float64 {
	out := make([][][3]float64, len(lines))
	for i, line := range lines {
		out[i] = append([][3]float64{}, line...)
	}
	return out
}

func cloneDimensionalGeometry(g geom.Geometry) (geom.Geometry, bool) {
	switch value := g.(type) {
	case geom.MultiPointZ:
		return append(geom.MultiPointZ{}, value...), true
	case geom.LineStringZ:
		return append(geom.LineStringZ{}, value...), true
	case geom.MultiLineStringZ:
		return geom.MultiLineStringZ(cloneXYZLines(value)), true
	case geom.PolygonZ:
		return geom.PolygonZ(cloneXYZLines(value)), true
	case geometrycodec.MultiPolygonZ:
		out := make(geometrycodec.MultiPolygonZ, len(value))
		for i, polygon := range value {
			out[i] = cloneXYZLines(polygon)
		}
		return out, true
	}
	return nil, false
}

func mutateXYZLine(line [][3]float64) {
	if len(line) > 0 {
		line[0][2] = 999
	}
}

func mutateDimensionalGeometry(g geom.Geometry) {
	switch value := g.(type) {
	case geom.MultiPointZ:
		mutateXYZLine(value)
	case geom.LineStringZ:
		mutateXYZLine(value)
	case geom.MultiLineStringZ:
		for _, line := range value {
			mutateXYZLine(line)
		}
	case geom.PolygonZ:
		for _, ring := range value {
			mutateXYZLine(ring)
		}
	case geometrycodec.MultiPolygonZ:
		for _, polygon := range value {
			for _, ring := range polygon {
				mutateXYZLine(ring)
			}
		}
	}
}
