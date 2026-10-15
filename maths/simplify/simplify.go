package simplify

import (
	"math"

	"github.com/alexeydott/tegola"
	"github.com/alexeydott/tegola/basic"
	"github.com/alexeydott/tegola/maths"
)

// SimplifyGeometry applies the DouglasPeucker simplification routine to the supplied geometry
func SimplifyGeometry(g tegola.Geometry, tolerance float64) tegola.Geometry {
	switch gg := g.(type) {
	case tegola.Polygon:
		return simplifyPolygon(gg, tolerance)

	case tegola.MultiPolygon:
		polygons := gg.Polygons()
		original := make(basic.MultiPolygon, len(polygons))
		candidate := make(basic.MultiPolygon, len(polygons))
		for i, p := range polygons {
			original[i] = basic.ClonePolygon(p)
			candidate[i] = polygonCandidate(p, tolerance)
		}
		if !preservesPolygonTopology(original, candidate) {
			return original
		}
		return candidate

	case tegola.LineString:
		return simplifyLineString(gg, tolerance)

	case tegola.MultiLine:
		var newML basic.MultiLine

		for _, l := range gg.Lines() {
			sl := simplifyLineString(l, tolerance)
			if sl == nil {
				continue
			}
			newML = append(newML, sl)
		}

		if len(newML) == 0 {
			return nil
		}

		return newML
	}

	return g
}

func simplifyLineString(g tegola.LineString, tolerance float64) basic.Line {
	line := basic.CloneLine(g)
	if len(line) <= 4 || maths.DistOfLine(g) < tolerance {
		return line
	}

	pts := line.AsPts()
	pts = DouglasPeucker(pts, tolerance)
	if len(pts) == 0 {
		return nil
	}

	// No coordinate truncation here: the MVT encoder quantizes to its output
	// grid anyway, and pre-truncating shifted simplified vertices off the
	// input subsequence (debt 3.6 corpus cases line/*-truncation).
	return basic.NewLineFromPt(pts...)
}

func simplifyPolygon(g tegola.Polygon, tolerance float64) basic.Polygon {
	original := basic.ClonePolygon(g)
	candidate := polygonCandidate(g, tolerance)
	if !preservesPolygonTopology(basic.MultiPolygon{original}, basic.MultiPolygon{candidate}) {
		return original
	}
	return candidate
}

// Build independent ring candidates; the caller validates their relationships
// together before accepting any changes to the polygon or multipolygon.
func polygonCandidate(g tegola.Polygon, tolerance float64) basic.Polygon {
	poly := basic.ClonePolygon(g)
	if tolerance <= 0 || math.IsNaN(tolerance) || math.IsInf(tolerance, 0) {
		return poly
	}
	for i, line := range poly {
		original := openRing(line.AsPts())
		if len(original) <= 4 || math.Abs(ringArea(original)) < tolerance*tolerance {
			continue
		}
		pts := normalizePoints(original)
		if len(pts) > 4 {
			pts = DouglasPeucker(pts, tolerance)
		}
		if !validRing(pts) || ringArea(original)*ringArea(pts) <= 0 {
			continue
		}
		if len(line) > 1 && line[0] == line[len(line)-1] {
			pts = append(pts, pts[0])
		}
		poly[i] = basic.NewLineFromPt(pts...)
	}
	return poly
}

// normalizePoints removes redundant vertices from the ring chain pts without
// changing the geometry: a vertex is dropped only when it lies on the segment
// between the neighbours kept around it. (The previous rule dropped points
// collinear with pts[0] and their successor regardless of position, which
// removed spike apexes that are not between their neighbours and folded the
// ring edge across the spike.)
func normalizePoints(pts []maths.Pt) (pnts []maths.Pt) {
	if pts[0] == pts[len(pts)-1] {
		pts = pts[1:]
	}

	if len(pts) <= 4 {
		return pts
	}

	lpt := 0
	pnts = append(pnts, pts[0])

	for i := 1; i < len(pts); i++ {
		ni := i + 1
		if ni >= len(pts) {
			ni = 0
		}
		if collinearBetween(pts[lpt], pts[i], pts[ni]) {
			continue // drop: the path pts[lpt]-pts[i]-pts[ni] keeps its geometry
		}
		pnts = append(pnts, pts[i])
		lpt = i
	}

	return pnts
}

// collinearBetween reports whether b lies on the segment from a to c
// (endpoints included), so dropping b leaves the path a-c unchanged.
func collinearBetween(a, b, c maths.Pt) bool {
	ab := sub2(b, a)
	bc := sub2(c, b)
	if !nearZero(cross2(ab, bc), math.Sqrt(dot2(ab, ab)*dot2(bc, bc))) {
		return false
	}
	return dot2(ab, bc) >= 0
}
