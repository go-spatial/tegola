package querytest

import (
	"testing"

	"github.com/alexeydott/geom"
	"github.com/alexeydott/tegola/provider/geometrycodec"
)

func withNativeRingOrientation(factory Factory, enabled bool) Factory {
	return func(t *testing.T, fixture Fixture) Instance {
		instance := factory(t, fixture)
		instance.nativeRingOrientationEquivalent = enabled
		return instance
	}
}

func nativeCopy[T any](v []T) []T {
	if v == nil {
		return nil
	}
	out := make([]T, len(v))
	copy(out, v)
	return out
}

// nativeRing keeps the first vertex and terminal closure untouched. No coordinate
// arithmetic, rotation, closure repair or structural normalization is permitted.
func nativeRing[T comparable](v []T, less func(T, T) bool) []T {
	out := nativeCopy(v)
	end := len(v) - 1
	if end > 0 && v[end] == v[0] {
		end--
	}
	for i := 1; i <= end; i++ {
		a, b := v[i], v[end-i+1]
		if a == b {
			continue
		}
		if less(b, a) {
			for l, r := 1, end; l < r; l, r = l+1, r-1 {
				out[l], out[r] = out[r], out[l]
			}
		}
		break
	}
	return out
}
func nativeXYLess(a, b [2]float64) bool {
	for i := range a {
		if a[i] != b[i] {
			return a[i] < b[i]
		}
	}
	return false
}
func nativeXYZLess(a, b [3]float64) bool {
	for i := range a {
		if a[i] != b[i] {
			return a[i] < b[i]
		}
	}
	return false
}
func nativeLines[T comparable](v [][]T, ring bool, less func(T, T) bool) [][]T {
	if v == nil {
		return nil
	}
	out := make([][]T, len(v))
	for i := range v {
		if ring {
			out[i] = nativeRing(v[i], less)
		} else {
			out[i] = nativeCopy(v[i])
		}
	}
	return out
}
func nativePolygons[T comparable](v [][][]T, less func(T, T) bool) [][][]T {
	if v == nil {
		return nil
	}
	out := make([][][]T, len(v))
	for i := range v {
		out[i] = nativeLines(v[i], true, less)
	}
	return out
}
func normalizeNativeRingOrientation(g geom.Geometry) geom.Geometry {
	switch v := g.(type) {
	case geom.Collection:
		if v == nil {
			return geom.Collection(nil)
		}
		out := make(geom.Collection, len(v))
		for i := range v {
			out[i] = normalizeNativeRingOrientation(v[i])
		}
		return out
	case geom.MultiPoint:
		return geom.MultiPoint(nativeCopy(v))
	case geom.MultiPointZ:
		return geom.MultiPointZ(nativeCopy(v))
	case geom.LineString:
		return geom.LineString(nativeCopy(v))
	case geom.LineStringZ:
		return geom.LineStringZ(nativeCopy(v))
	case geom.MultiLineString:
		return geom.MultiLineString(nativeLines(v, false, nativeXYLess))
	case geom.MultiLineStringZ:
		return geom.MultiLineStringZ(nativeLines(v, false, nativeXYZLess))
	case geom.Polygon:
		return geom.Polygon(nativeLines(v, true, nativeXYLess))
	case geom.PolygonZ:
		return geom.PolygonZ(nativeLines(v, true, nativeXYZLess))
	case geom.MultiPolygon:
		return geom.MultiPolygon(nativePolygons(v, nativeXYLess))
	case geometrycodec.MultiPolygonZ:
		return geometrycodec.MultiPolygonZ(nativePolygons(v, nativeXYZLess))
	default:
		return g
	}
}
