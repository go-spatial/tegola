package convert

import (
	"github.com/terranodo/tegola"
	"github.com/terranodo/tegola/geom"
)

func ToGeom(g tegola.Geometry) (geom.Geometry, error) {
	switch geo := g.(type) {
	default:
		return nil, geom.UnknownGeometryError

	case tegola.Point:
		return geom.Point{geo.X(), geo.Y()}, nil
	case tegola.MultiPoint:
		pts := geo.Points()
		var mpts = make(geom.MultiPoint, len(pts))
		for i := range pts {
			mpts[i][0] = pts[i].X()
			mpts[i][1] = pts[i].Y()
		}
		return mpts, nil
	case tegola.LineString:
		pts := geo.Subpoints()
		var ln = make(geom.LineString, len(pts))
		for i := range pts {
			ln[i][0] = pts[i].X()
			ln[i][1] = pts[i].Y()
		}
		return ln, nil
	case tegola.MultiLine:
		lns := geo.Lines()
		var mln = make(geom.MultiLineString, len(lns))
		for i := range lns {
			pts := lns[i].Subpoints()
			mln[i] = make(geom.LineString, len(pts))
			for j := range pts {
				mln[i][j][0] = pts[j].X()
				mln[i][j][1] = pts[j].Y()
			}
		}
		return mln, nil
	case tegola.Polygon:
		lns := geo.Sublines()
		var ply = make(geom.Polygon, len(lns))
		for i := range lns {
			pts := lns[i].Subpoints()
			ply[i] = make(geom.LineString, len(pts))
			for j := range pts {
				ply[i][j][0] = pts[j].X()
				ply[i][j][1] = pts[j].Y()
			}
		}
		return ply, nil
	case tegola.MultiPolygon:
		mply := geo.Polygons()
		var mp = make(geom.MultiPolygon, len(mply))
		for i := range mply {
			lns := mply[i].Sublines()
			mp[i] = make(geom.Polygon, len(lns))
			for j := range lns {
				pts := lns[j].Subpoints()
				mp[i][j] = make(geom.LineString, len(pts))
				for k := range pts {
					mp[i][j][k][0] = pts[k].X()
					mp[i][j][k][1] = pts[k].Y()
				}
			}
		}
		return mp, nil
	case tegola.Collection:
		geometries := geo.Geometries()
		var cl = make(geom.Collection, len(geometries))
		for i := range geometries {
			tgeo, err := ToGeom(geometries[i])
			if err != nil {
				return nil, err
			}
			cl[i] = tgeo
		}
		return cl, nil
	}
}
