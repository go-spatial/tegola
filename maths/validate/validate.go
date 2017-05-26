package validate

import (
	"github.com/terranodo/tegola"
	"github.com/terranodo/tegola/basic"
	"github.com/terranodo/tegola/maths"
)

// CleanLinstring will remove duplicate points, and points between the duplicate points. The exception to this, is the first and last points,
// are the same.
func CleanLine(g tegola.LineString) (l basic.Line, err error) {

	var ptsMap = make(map[maths.Pt][]int)
	var pts []maths.Pt
	for i, pt := range g.Subpoints() {

		p := maths.Pt{pt.X(), pt.Y()}
		ptsMap[p] = append(ptsMap[p], i)
		pts = append(pts, p)
	}

	for i := 0; i < len(pts); i++ {
		pt := pts[i]
		fpts := ptsMap[pt]
		l = append(l, basic.Point{pt.X, pt.Y})
		if len(fpts) > 1 {
			// we will need to skip a bunch of points.
			i = fpts[len(fpts)-1]
		}
	}
	return l, nil
}

func CleanLinestring(g []float64) (l []float64, err error) {

	var ptsMap = make(map[maths.Pt][]int)
	var pts []maths.Pt
	i := 0
	for x, y := 0, 1; y < len(g); x, y = x+2, y+2 {

		p := maths.Pt{g[x], g[y]}
		ptsMap[p] = append(ptsMap[p], i)
		pts = append(pts, p)
		i++
	}

	for i := 0; i < len(pts); i++ {
		pt := pts[i]
		fpts := ptsMap[pt]
		l = append(l, pt.X, pt.Y)
		if len(fpts) > 1 {
			// we will need to skip a bunch of points.
			i = fpts[len(fpts)-1]
		}
	}
	return l, nil
}

func CleanPolygon(g tegola.Polygon) (p basic.Polygon, err error) {

	for _, ln := range g.Sublines() {
		cln, err := CleanLine(ln)
		if err != nil {
			return p, err
		}
		p = append(p, cln)
	}
	return p, nil
}

func CleanGeometry(g tegola.Geometry) (geo tegola.Geometry, err error) {
	if g == nil {
		return nil, nil
	}
	switch gg := g.(type) {

	case tegola.Polygon:
		return CleanPolygon(gg)
	case tegola.MultiPolygon:
		var mp basic.MultiPolygon
		for _, p := range gg.Polygons() {
			cp, err := CleanPolygon(p)
			if err != nil {
				return mp, err
			}
			mp = append(mp, cp)
		}
		return mp, nil
	}
	return g, nil
}
