package makevalid

import (
	"context"
	"fmt"
	"log"

	"github.com/go-spatial/geom/planar/triangulate/gdey/quadedge"

	"github.com/go-spatial/geom"
	"github.com/go-spatial/geom/cmp"
	"github.com/go-spatial/geom/planar"
	"github.com/go-spatial/geom/planar/triangulate/constraineddelaunay"
)

func TriangulateGeometry(ctx context.Context, g geom.Geometry) ([]geom.Triangle, error) {
	uut := new(constraineddelaunay.Triangulator)
	if debug {
		log.Printf("Triangulator for given segments %v ", g)
	}
	if err := uut.InsertGeometries([]geom.Geometry{g}, nil); err != nil {
		if debug {
			log.Printf("Triangulator error for given segments %v : %v", g, err)
		}
		return []geom.Triangle{}, fmt.Errorf("error triangulating geometry: %v", err)
	}

	if debug {
		err := uut.Validate()
		if err != nil {
			log.Printf("Triangulator is not validate for the given segments %v : %v", g, err)
			return []geom.Triangle{}, err
		}
	}
	// TODO(gdey): We need to insure that GetTriangles does not dup the first point to the
	//              last point. It may be better if it returned triangles and we moved triangles to Geom.
	gtris, err := uut.GetTriangles()
	if err != nil {
		if debug {
			log.Printf("got the following error %v", err)
		}
		return []geom.Triangle{}, err
	}
	if debug {
		log.Printf("Triangulator genereated %v triangles", len(gtris))
	}
	var triangles = make([]geom.Triangle, len(gtris))
	for i, ply := range gtris {
		triangles[i] = geom.NewTriangleFromPolygon(ply)
		cmp.RotateToLeftMostPoint(triangles[i][:])
	}
	return triangles, nil
}

func InsideTrianglesForSegments(ctx context.Context, segs []geom.Line, hm planar.HitMapper) ([]geom.Triangle, error) {
	if debug {
		log.Printf("Step   3 : generate triangles")
	}
	triangulator := qetriangulate.GeomConstrained{
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
	return triangles, nil

}
