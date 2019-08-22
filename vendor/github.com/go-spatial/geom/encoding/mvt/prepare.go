package mvt

import (
	"log"

	"github.com/go-spatial/geom"
)

// PrepareGeo converts the geometry's coordinates to tile pixel coordinates. tile should be the
// extent of the tile, in the same projection as geo. pixelExtent is the dimension of the
// (square) tile in pixels usually 4096, see DefaultExtent.
// This function treats the tile extent elements as left, top, right, bottom. This is fine
// when working with a north-positive projection such as lat/long (epsg:4326)
// and web mercator (epsg:3857), but a south-positive projection (ie. epsg:2054) or west-postive
// projection would then flip the geomtery. To properly render these coordinate systems, simply
// swap the X's or Y's in the tile extent.
func PrepareGeo(geo geom.Geometry, tile *geom.Extent, pixelExtent float64) geom.Geometry {
	switch g := geo.(type) {
	case geom.Point:
		return preparept(g, tile, pixelExtent)

	case geom.MultiPoint:
		pts := g.Points()
		if len(pts) == 0 {
			return nil
		}

		mp := make(geom.MultiPoint, len(pts))
		for i, pt := range g {
			mp[i] = preparept(pt, tile, pixelExtent)
		}

		return mp

	case geom.LineString:
		return preparelinestr(g, tile, pixelExtent)

	case geom.MultiLineString:
		var ml geom.MultiLineString
		for _, l := range g.LineStrings() {
			nl := preparelinestr(l, tile, pixelExtent)
			if len(nl) > 0 {
				ml = append(ml, nl)
			}
		}
		return ml

	case geom.Polygon:
		return preparePolygon(g, tile, pixelExtent)

	case geom.MultiPolygon:
		var mp geom.MultiPolygon
		for _, p := range g.Polygons() {
			np := preparePolygon(p, tile, pixelExtent)
			if len(np) > 0 {
				mp = append(mp, np)
			}
		}
		return mp
	}

	return nil
}

func preparept(g geom.Point, tile *geom.Extent, pixelExtent float64) geom.Point {
	px := int64((g.X() - tile.MinX()) / tile.XSpan() * pixelExtent)
	py := int64((tile.MaxY() - g.Y()) / tile.YSpan() * pixelExtent)

	return geom.Point{float64(px), float64(py)}
}

func preparelinestr(g geom.LineString, tile *geom.Extent, pixelExtent float64) (ls geom.LineString) {
	pts := g
	// If the linestring
	if len(pts) < 2 {
		// Not enought points to make a line.
		return nil
	}

	ls = make(geom.LineString, len(pts))
	for i := 0; i < len(pts); i++ {
		ls[i] = preparept(pts[i], tile, pixelExtent)
	}

	return ls
}

func preparePolygon(g geom.Polygon, tile *geom.Extent, pixelExtent float64) (p geom.Polygon) {
	lines := geom.MultiLineString(g.LinearRings())
	p = make(geom.Polygon, 0, len(lines))

	if len(lines) == 0 {
		return p
	}

	for _, line := range lines.LineStrings() {
		ln := preparelinestr(line, tile, pixelExtent)
		if len(ln) < 2 {
			if debug {
				// skip lines that have been reduced to less then 2 points.
				log.Println("skipping line 2", line, len(ln))
			}
			continue
		}
		// TODO: check the last and first point to make sure
		// they are not the same, per the mvt spec
		p = append(p, ln)
	}
	return p
}
