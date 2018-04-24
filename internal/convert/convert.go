package convert

import (
	"errors"

	"github.com/go-spatial/geom"
	"github.com/go-spatial/tegola"
	"github.com/go-spatial/tegola/basic"
)

var ErrUnknownGeometry = errors.New("Unknown Geometry")

func ToGeom(g tegola.Geometry) (geom.Geometry, error) {
	switch geo := g.(type) {
	default:
		return nil, ErrUnknownGeometry

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

func toBasicLine(g geom.LineString) basic.Line {
	l := make(basic.Line, len(g))
	for i := range g {
		l[i] = basic.Point(g[i])
	}
	return l
}

func toBasic(g geom.Geometry) basic.Geometry {
	switch geo := g.(type) {
	default:
		return nil

	case geom.Point:
		return basic.Point(geo)

	case geom.MultiPoint:
		mp := make(basic.MultiPoint, len(geo))
		for i := range geo {
			mp[i] = basic.Point(geo[i])
		}
		return mp

	case geom.LineString:
		return toBasicLine(geo)

	case geom.MultiLineString:
		ml := make(basic.MultiLine, len(geo))
		for i := range geo {
			ml[i] = toBasicLine(geo[i])
		}
		return ml
	case geom.Polygon:
		plg := make(basic.Polygon, len(geo))
		for i := range geo {
			plg[i] = toBasicLine(geo[i])
		}
		return plg
	case geom.MultiPolygon:
		mplg := make(basic.MultiPolygon, len(geo))
		for i := range geo {
			mplg[i] = make(basic.Polygon, len(geo[i]))
			for j := range geo[i] {
				mplg[i][j] = toBasicLine(geo[i][j])
			}
		}
		return mplg
	case geom.Collection:
		geometries := geo.Geometries()
		bc := make(basic.Collection, len(geometries))
		for i := range geometries {
			bc[i] = toBasic(geometries[i])
		}
		return bc
	}

}
func ToTegola(geom geom.Geometry) (tegola.Geometry, error) {
	g := toBasic(geom)
	if g == nil {
		return g, ErrUnknownGeometry
	}
	return g, nil
}
