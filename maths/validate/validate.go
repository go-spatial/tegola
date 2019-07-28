package validate

import (
	"context"

	"github.com/go-spatial/geom"
	"github.com/go-spatial/tegola"
	"github.com/go-spatial/tegola/basic"
	"github.com/go-spatial/tegola/maths"
	"github.com/go-spatial/tegola/maths/clip"
	"github.com/go-spatial/tegola/maths/hitmap"
	"github.com/go-spatial/tegola/maths/makevalid"
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

func makePolygonValid(ctx context.Context, hm *hitmap.M, extent *geom.Extent, gs ...tegola.Polygon) (mp basic.MultiPolygon, err error) {
	var plygLines [][]maths.Line
	for _, g := range gs {
		for _, l := range g.Sublines() {
			segs, err := LineStringToSegments(l)
			if err != nil {
				return mp, err
			}
			plygLines = append(plygLines, segs)
			if err := ctx.Err(); err != nil {
				return mp, err
			}
		}
	}
	plyPoints, err := makevalid.MakeValid(ctx, hm, extent, plygLines...)
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
			if err := ctx.Err(); err != nil {
				return mp, err
			}
		}
		mp = append(mp, p)
	}
	return mp, err
}

func scalePolygon(p tegola.Polygon, factor float64) (bp basic.Polygon) {
	lines := p.Sublines()
	bp = make(basic.Polygon, len(lines))
	for i := range lines {
		pts := lines[i].Subpoints()
		bp[i] = make(basic.Line, len(pts))
		for j := range pts {
			bp[i][j] = basic.Point{pts[j].X() * factor, pts[j].Y() * factor}
		}
	}
	return bp
}

func scaleMultiPolygon(p tegola.MultiPolygon, factor float64) (bmp basic.MultiPolygon) {
	polygons := p.Polygons()
	bmp = make(basic.MultiPolygon, len(polygons))
	for i := range polygons {
		bmp[i] = scalePolygon(polygons[i], factor)
	}
	return bmp
}

// CleanGeometry will apply various geoprocessing algorithems to the provided geometry.
// the extent will be used as a clipping region. if no clipping is desired, pass in a nil extent.
func CleanGeometry(ctx context.Context, g tegola.Geometry, extent *geom.Extent) (geo tegola.Geometry, err error) {
	if g == nil {
		return nil, nil
	}
	switch gg := g.(type) {
	case tegola.Polygon:
		expp := scalePolygon(gg, 10.0)
		ext := extent.ScaleBy(10.0)
		hm := hitmap.NewFromGeometry(expp)
		mp, err := makePolygonValid(ctx, &hm, ext, expp)
		if err != nil {
			return nil, err
		}
		return scaleMultiPolygon(mp, 0.10), nil

	case tegola.MultiPolygon:
		expp := scaleMultiPolygon(gg, 10.0)
		ext := extent.ScaleBy(10.0)
		hm := hitmap.NewFromGeometry(expp)
		mp, err := makePolygonValid(ctx, &hm, ext, expp.Polygons()...)
		if err != nil {
			return nil, err
		}
		return scaleMultiPolygon(mp, 0.10), nil

	case tegola.MultiLine:
		var ml basic.MultiLine
		lns := gg.Lines()
		for i := range lns {
			// log.Println("Clip MultiLine Buff", buff)
			nls, err := clip.LineString(lns[i], extent)
			if err != nil {
				return ml, err
			}
			ml = append(ml, nls...)
		}
		return ml, nil
	case tegola.LineString:
		// 	log.Println("Clip LineString Buff", buff)
		nls, err := clip.LineString(gg, extent)
		return basic.MultiLine(nls), err
	}
	return g, nil
}
