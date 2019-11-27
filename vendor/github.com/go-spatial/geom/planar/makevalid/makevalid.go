package makevalid

import (
	"context"
	"errors"
	"log"
	"sort"

	"github.com/go-spatial/geom/encoding/wkt"
	"github.com/go-spatial/geom/winding"

	"github.com/go-spatial/geom"
	pkgcmp "github.com/go-spatial/geom/cmp"
	"github.com/go-spatial/geom/planar"
	"github.com/go-spatial/geom/planar/intersect"
	"github.com/go-spatial/geom/planar/makevalid/hitmap"
	"github.com/go-spatial/geom/planar/makevalid/walker"
)

type Makevalid struct {
	Hitmap planar.HitMapper
	// Currently not used, but once we have the IsValid function, we can use this instead
	// Of running the MakeValid routine on a Geometry that is already valid.
	// Used to clip geometries that are not Polygon and MultiPolygons
	Clipper planar.Clipper
	CMP     pkgcmp.Compare
	Order   winding.Order
}

// asSegments calls the AsSegments functions and flattens the array of segments that are returned.
func asSegments(g geom.Geometry) (segs []geom.Line, err error) {
	switch g := g.(type) {
	case geom.LineString:
		return g.AsSegments()
	case geom.MultiLineString:
		s, err := g.AsSegments()
		if err != nil {
			return nil, err
		}
		for i := range s {
			segs = append(segs, s[i]...)
		}
		return segs, nil
	case geom.Polygon:
		s, err := g.AsSegments()
		if err != nil {
			return nil, err
		}
		for i := range s {
			segs = append(segs, s[i]...)
		}
		return segs, nil
	case geom.MultiPolygon:
		s, err := g.AsSegments()
		if err != nil {
			return nil, err
		}
		for i := range s {
			for j := range s[i] {
				segs = append(segs, s[i][j]...)
			}
		}
		return segs, nil
	}
	return nil, errors.New("Unsupported")
}

type ByXYPoint [][2]float64

func (a ByXYPoint) Len() int           { return len(a) }
func (a ByXYPoint) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a ByXYPoint) Less(i, j int) bool { return cmp.PointLess(a[i], a[j]) }

type ByXYLine []geom.Line

func (a ByXYLine) Len() int      { return len(a) }
func (a ByXYLine) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a ByXYLine) Less(i, j int) bool {
	return cmp.PointLess(a[i][0], a[j][0]) || (cmp.PointEqual(a[i][0], a[j][0]) && cmp.PointLess(a[i][1], a[j][1]))
}

// Destructure will take a multipolygon, break up the polygon into a set of segments that have the following characteristics:
// 1. no segment will intersect with another segment, other then at the end points; or colinear and partial-coliner lines.
// 2. normalize direction of line segments to left to right
// 3. line segments are generally unique.
// 4. line segments outside of the clipbox will be clipped
func Destructure(ctx context.Context, cmp pkgcmp.Compare, clipbox *geom.Extent, multipolygon *geom.MultiPolygon) ([]geom.Line, error) {

	segments, err := asSegments(*multipolygon)
	if err != nil {
		if debug {
			log.Printf("asSegments returned error: %v", err)
		}
		return nil, err
	}
	gext, err := geom.NewExtentFromGeometry(multipolygon)
	if err != nil {
		return nil, err
	}

	// Let's see if our clip box is bigger then our polygon.
	// if it is we don't need the clip box.
	hasClipbox := clipbox != nil && !clipbox.Contains(gext)
	// Let's get the edges of our clipbox; as segments and add it to the begining.
	if hasClipbox {
		edges := clipbox.Edges(nil)
		segments = append([]geom.Line{
			geom.Line(edges[0]), geom.Line(edges[1]),
			geom.Line(edges[2]), geom.Line(edges[3]),
		}, segments...)
	}
	ipts := make(map[int][][2]float64)

	// Lets find all the places we need to split the lines on.
	eq := intersect.NewEventQueue(segments)
	eq.FindIntersects(ctx, true, func(src, dest int, pt [2]float64) error {
		ipts[src] = append(ipts[src], pt)
		ipts[dest] = append(ipts[dest], pt)
		return nil
	})

	// Time to start splitting lines. if we have a clip box we can ignore the first 4 (0,1,2,3) lines.

	nsegs := make([]geom.Line, 0, len(segments))

	for i := 0; i < len(segments); i++ {
		pts := append([][2]float64{segments[i][0], segments[i][1]}, ipts[i]...)

		// Normalize the direction of the points.
		sort.Sort(ByXYPoint(pts))

		for j := 1; j < len(pts); j++ {
			if cmp.PointEqual(pts[j-1], pts[j]) {
				continue
			}
			nl := geom.Line{pts[j-1], pts[j]}
			if hasClipbox && !clipbox.ContainsLine(nl) {
				// Not in clipbox discard segment.
				continue
			}
			nsegs = append(nsegs, nl)
		}
		if ctx.Err() != nil {
			return nil, err
		}
	}

	unique(nsegs)
	return nsegs, nil
}

// unique sorts segments by XY and filters out duplicate segments.
func unique(segs []geom.Line) {
	sort.Sort(ByXYLine(segs))

	// we can use a slice trick to avoid copying the array again. Maybe better
	// than two index variables...
	uniqued := segs[:0]
	for i := 0; i < len(segs); i++ {
		if i == 0 || !(cmp.PointEqual(segs[i][0], segs[i-1][0]) && cmp.PointEqual(segs[i][1], segs[i-1][1])) {
			uniqued = append(uniqued, segs[i])
		}
	}
	// uniqued is backed by segs, no need to return it
}

func (mv *Makevalid) makevalidPolygon(ctx context.Context, clipbox *geom.Extent, multipolygon *geom.MultiPolygon) (*geom.MultiPolygon, error) {
	hm, err := hitmap.NewFromPolygons(nil, (*multipolygon)...)
	if err != nil {
		return nil, err
	}

	triangles, err := InsideTrianglesForMultiPolygon(ctx, clipbox, multipolygon, hm)
	if err != nil {
		return nil, err
	}
	if debug {
		log.Printf("Step   5 : generate multipolygon from triangles")
	}
	if len(triangles) == 0 {
		return nil, nil
	}

	triWalker := walker.New(triangles)
	mplygs := triWalker.MultiPolygon(ctx)

	return &mplygs, nil
}

func (mv *Makevalid) Makevalid(ctx context.Context, geo geom.Geometry, clipbox *geom.Extent) (geometry geom.Geometry, didClip bool, err error) {

	switch g := geo.(type) {

	case geom.LineStringer, geom.MultiLineStringer, geom.Pointer, geom.MultiPointer:
		if mv.Clipper != nil {
			gg, err := mv.Clipper.Clip(ctx, geo, clipbox)
			if err != nil {
				return nil, false, err
			}
			return gg, true, nil
		}
		return geo, false, nil
	case geom.Polygoner:
		if debug {
			log.Printf("Working on Polygoner: %v", geo)
		}
		mp := geom.MultiPolygon{g.LinearRings()}
		vmp, err := mv.makevalidPolygon(ctx, clipbox, &mp)
		if err != nil {
			return nil, false, err
		}
		if debug {
			log.Printf("Returning on Polygon: %T", vmp)
		}
		return vmp, true, nil
	case geom.MultiPolygoner:
		if debug {
			log.Printf("Working on MultiPolygoner: \n%v", wkt.MustEncode(geo))
		}
		mp := geom.MultiPolygon(g.Polygons())
		vmp, err := mv.makevalidPolygon(ctx, clipbox, &mp)
		if err != nil {
			return nil, false, err
		}
		if debug {
			log.Printf("Returning on MultiPolygon: %T", vmp)
		}
		return vmp, true, nil
	}
	if debug {
		log.Printf("Got an unknown geometry %T", geo)
	}
	return nil, false, geom.ErrUnknownGeometry{geo}

}
