package validate

import (
	"github.com/terranodo/tegola"
	"github.com/terranodo/tegola/basic"
	"github.com/terranodo/tegola/maths"
	"github.com/terranodo/tegola/maths/hitmap"
	"github.com/terranodo/tegola/maths/makevalid"
)

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

func LineStringToSegments(l tegola.LineString) ([]maths.Line, error) {
	ppln := tegola.LineAsPointPairs(l)
	return maths.NewSegments(ppln)
}
func makePolygonValid(hm hitmap.M, extent float64, gs ...tegola.Polygon) (mp basic.MultiPolygon, err error) {
	var plygLines [][]maths.Line
	for _, g := range gs {
		for _, l := range g.Sublines() {
			segs, err := LineStringToSegments(l)
			if err != nil {
				return mp, err
			}
			plygLines = append(plygLines, segs)
		}
	}
	plyPoints, err := makevalid.MakeValid(&hm, extent, plygLines...)
	if err != nil {
		return mp, err
	}
	for i := range plyPoints {
		// Each i is a polygon. Made up of line string points.
		var p basic.Polygon
		for j := range plyPoints[i] {
			// We need to transform plyPoints[i][j] into a basic.LineString.
			nl := basic.NewLineFromPt(plyPoints[i][j]...)
			p = append(p, nl)
		}
		mp = append(mp, p)
	}
	return mp, err
}

func CleanGeometry(ogeo, g tegola.Geometry, extent float64) (geo tegola.Geometry, err error) {
	if g == nil {
		return nil, nil
	}
	hm := hitmap.NewFromGeometry(g)
	switch gg := g.(type) {
	case tegola.Polygon:
		return makePolygonValid(hm, extent, gg)

	case tegola.MultiPolygon:
		return makePolygonValid(hm, extent, gg.Polygons()...)
	}
	return g, nil
}
