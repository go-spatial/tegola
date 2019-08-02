package mvt

import (
	"github.com/go-spatial/geom"
	"github.com/go-spatial/geom/cmp"
	"github.com/go-spatial/tegola"
	"github.com/go-spatial/tegola/internal/log"
)

// ScaleGeo converts the geometry's coordinates to tile coordinates
func ScaleGeo(geo tegola.Geometry, tile *tegola.Tile) geom.Geometry {
	switch g := geo.(type) {
	case geom.Point:
		return scalept(g, tile)

	case geom.MultiPoint:
		pts := g.Points()
		if len(pts) == 0 {
			return nil
		}

		// TODO the ptmap is used for removing duplicate points.
		// This does not seem to be part of the spec, but it seems
		// helpful for reducing the point count, especially with
		// very zoomed out geometries. Further discussion should be had...
		// https://github.com/mapbox/vector-tile-spec/tree/master/2.1#4342-point-geometry-type
		var ptmap = make(map[geom.Point]struct{})
		var mp = make(geom.MultiPoint, 1, len(pts))
		mp[0] = scalept(pts[0], tile)

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

	case geom.LineString:
		return scalelinestr(g, tile)

	case geom.MultiLineString:
		var ml geom.MultiLineString
		for _, l := range g.LineStrings() {
			nl := scalelinestr(l, tile)
			if len(nl) > 0 {
				ml = append(ml, nl)
			}
		}
		return ml

	case geom.Polygon:
		return scalePolygon(g, tile)

	case geom.MultiPolygon:
		var mp geom.MultiPolygon
		for _, p := range g.Polygons() {
			np := scalePolygon(p, tile)
			if len(np) > 0 {
				mp = append(mp, np)
			}
		}
		return mp
	}

	return nil
}

func scalept(g geom.Point, tile *tegola.Tile) geom.Point {
	pt, err := tile.ToPixel(tegola.WebMercator, g)
	if err != nil {
		panic(err)
	}
	return geom.Point(pt)
}

func scalelinestr(g geom.LineString, tile *tegola.Tile) (ls geom.LineString) {
	pts := g
	// If the linestring
	if len(pts) < 2 {
		// Not enought points to make a line.
		return nil
	}
	ls = make(geom.LineString, 0, len(pts))
	ls = append(ls, scalept(pts[0], tile))
	lidx := len(ls) - 1
	for i := 1; i < len(pts); i++ {
		npt := scalept(pts[i], tile)
		if cmp.PointEqual(ls[lidx], npt) {
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

func scalePolygon(g geom.Polygon, tile *tegola.Tile) (p geom.Polygon) {
	lines := geom.MultiLineString(g.LinearRings())
	p = make(geom.Polygon, 0, len(lines))

	if len(lines) == 0 {
		return p
	}

	for _, line := range lines.LineStrings() {
		ln := scalelinestr(line, tile)
		if len(ln) < 2 {
			if debug {
				// skip lines that have been reduced to less then 2 points.
				log.Debug("skipping line 2", line, len(ln))
			}
			continue
		}
		p = append(p, ln)
	}
	return p
}
