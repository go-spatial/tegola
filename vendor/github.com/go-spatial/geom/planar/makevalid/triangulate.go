package makevalid

import (
	"context"
	"log"

	"github.com/go-spatial/geom/encoding/wkt"

	"github.com/go-spatial/geom/planar/triangulate/delaunay"

	"github.com/go-spatial/geom"
	"github.com/go-spatial/geom/planar"
)

// InsideTrianglesForSegments returns triangles that are painted as as inside triangles
func InsideTrianglesForSegments(ctx context.Context, segs []geom.Line, hm planar.HitMapper) ([]geom.Triangle, error) {
	if debug {
		log.Printf("Step   3 : generate triangles")
	}
	triangulator := delaunay.GeomConstrained{
		Constraints: segs,
	}
	allTriangles, err := triangulator.Triangles(ctx, false)
	if err != nil {
		if debug {
			log.Println("Step     3a: got error", err)
		}
		return nil, err
	}
	if debug {
		log.Printf("Step   4 : label triangles and discard outside triangles")
		log.Printf("Step   4a: All Triangles:\n%v", wkt.MustEncode(allTriangles))
	}
	if len(allTriangles) == 0 {
		return []geom.Triangle{}, nil
	}
	triangles := make([]geom.Triangle, 0, len(allTriangles))

	for _, triangle := range allTriangles {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if hm.LabelFor(triangle.Center()) == planar.Outside {
			continue
		}
		triangles = append(triangles, triangle)
	}
	if debug {
		log.Printf("Step   4b: Inside Triangles:\n%v", wkt.MustEncode(triangles))
	}
	return triangles, nil

}

// InsideTrianglesForMultiPolygon returns triangles that are painted as inside triangles for the multipolygon
func InsideTrianglesForMultiPolygon(ctx context.Context, clipbox *geom.Extent, multipolygon *geom.MultiPolygon, hm planar.HitMapper) ([]geom.Triangle, error) {
	segs, err := Destructure(ctx, cmp, clipbox, multipolygon)
	if err != nil {
		if debug {
			log.Printf("Destructure returned err %v", err)
		}
		return nil, err
	}
	if len(segs) == 0 {
		if debug {
			log.Printf("Step   1a: Segments are zero.")
			log.Printf("\t multiPolygon: %+v", multipolygon)
			log.Printf("\n clipbox:      %+v", clipbox)
		}
		return nil, nil
	}
	if debug {
		log.Printf("Step   2 : Convert segments(%v) to linestrings to use in triangulation.", len(segs))
		log.Printf("Step   2a: %v", wkt.MustEncode(segs))
	}
	triangles, err := InsideTrianglesForSegments(ctx, segs, hm)
	if err != nil {
		return nil, err
	}
	if len(triangles) == 0 {
		return nil, nil
	}
	return triangles, nil
}
