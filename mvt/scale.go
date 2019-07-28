package mvt

import (
	"github.com/go-spatial/tegola"
	"github.com/go-spatial/tegola/basic"
	"github.com/go-spatial/tegola/internal/log"
)

// ScaleGeo converts the geometry's coordinates to tile coordinates
func ScaleGeo(geo tegola.Geometry, tile *tegola.Tile) basic.Geometry {
	switch g := geo.(type) {
	case tegola.Point:
		return scalept(g, tile)

	case tegola.Point3:
		return scalept(g, tile)

	case tegola.MultiPoint:
		pts := g.Points()
		if len(pts) == 0 {
			return nil
		}
		var ptmap = make(map[basic.Point]struct{})
		var mp = make(basic.MultiPoint, 0, len(pts))
		mp = append(mp, scalept(pts[0], tile))

		ptmap[mp[0]] = struct{}{}
		for i := 1; i < len(pts); i++ {

			npt := scalept(pts[i], tile)
			if _, ok := ptmap[npt]; ok {
				// Skip duplicate points.
				continue
			}

			ptmap[npt] = struct{}{}
			mp = append(mp, npt)
		}
		return mp

	case tegola.LineString:
		return scalelinestr(g, tile)

	case tegola.MultiLine:
		var ml basic.MultiLine
		for _, l := range g.Lines() {
			nl := scalelinestr(l, tile)
			if len(nl) > 0 {
				ml = append(ml, nl)
			}
		}
		return ml

	case tegola.Polygon:
		return scalePolygon(g, tile)

	case tegola.MultiPolygon:
		var mp basic.MultiPolygon
		for _, p := range g.Polygons() {
			np := scalePolygon(p, tile)
			if len(np) > 0 {
				mp = append(mp, np)
			}
		}
		return mp
	}

	return basic.G{}
}

func scalept(g tegola.Point, tile *tegola.Tile) basic.Point {
	pt, err := tile.ToPixel(tegola.WebMercator, [2]float64{g.X(), g.Y()})
	if err != nil {
		panic(err)
	}
	return basic.Point{pt[0], pt[1]}
}

func scalelinestr(g tegola.LineString, tile *tegola.Tile) (ls basic.Line) {
	pts := g.Subpoints()
	// If the linestring
	if len(pts) < 2 {
		// Not enought points to make a line.
		return nil
	}
	ls = make(basic.Line, 0, len(pts))
	ls = append(ls, scalept(pts[0], tile))
	lidx := len(ls) - 1
	for i := 1; i < len(pts); i++ {
		npt := scalept(pts[i], tile)
		if tegola.IsPointEqual(ls[lidx], npt) {
			// drop any duplicate points.
			continue
		}
		ls = append(ls, npt)
		lidx = len(ls) - 1
	}

	if len(ls) < 2 {
		// Not enough points. the zoom must be too far out for this ring.
		return nil
	}
	return ls
}

func scalePolygon(g tegola.Polygon, tile *tegola.Tile) (p basic.Polygon) {
	lines := g.Sublines()
	p = make(basic.Polygon, 0, len(lines))

	if len(lines) == 0 {
		return p
	}
	for i := range lines {
		ln := scalelinestr(lines[i], tile)
		if len(ln) < 2 {
			if debug {
				// skip lines that have been reduced to less then 2 points.
				log.Debug("skipping line 2", lines[i], len(ln))
			}
			continue
		}
		p = append(p, ln)
	}
	return p
}
