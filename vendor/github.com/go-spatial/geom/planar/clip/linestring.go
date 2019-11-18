package clip

import (
	"context"
	"log"

	"github.com/go-spatial/geom"
	"github.com/go-spatial/geom/cmp"
	"github.com/go-spatial/geom/planar"
)

func uniqueSegmentIntersectPoints(clipbox *geom.Extent, ln geom.Line) [][2]float64 {
	// if the clipbox contains the unverse, then there are not intersects.
	if clipbox.IsUniverse() {
		return nil
	}
	pts := make([][2]float64, 0, 2)
	edges := clipbox.Edges(nil)
	for i := range edges {
		pt, ok := planar.SegmentIntersect(geom.Line(edges[i]), ln)
		if !ok {
			continue
		}
		for _, ept := range pts {
			if cmp.PointEqual(ept, pt) {
				// A line will only cross two lines for a rectangular clip box.
				// We have found a point, before and this is the second point.
				// We are not going to add it as it is equal.
				return pts
			}
		}
		pts = append(pts, pt)
		if len(pts) >= 2 {
			return pts
		}
	}
	return pts
}

// LineString will clip the give linestring to the the given clipbox, breaking it up into multiple linestring as needed.
func LineStringer(ctx context.Context, linestringer geom.LineStringer, clipbox *geom.Extent) (mls geom.MultiLineString, err error) {
	return lineString(ctx, linestringer.Vertices(), clipbox)
}

func lineString(ctx context.Context, ls [][2]float64, clipbox *geom.Extent) (mls geom.MultiLineString, err error) {

	if debug {
		log.Printf("Clipping linestring: %v", ls)
	}

	// The clipbox contains everything, no need to clip anything, just return the LineString in a MultiLineString.
	if len(ls) == 0 {
		return geom.MultiLineString{}, nil
	}

	if len(ls) == 1 {
		if debug {
			log.Println("got an invalid linestring")
		}
		return geom.MultiLineString{}, geom.ErrInvalidLineString
	}

	// if the linestring is in the clipbox nothing to do.
	if contains, _ := clipbox.ContainsGeom(ls); contains {
		return geom.MultiLineString{ls}, nil
	}

	cls := make([][2]float64, 0)

	// Was the Last point in the clipping region.
	lptIsIn := clipbox.ContainsPoint(ls[0])
	if lptIsIn {
		cls = append(cls, ls[0])
	}

	if debug {
		log.Printf("Looking at point[0]: %v", ls[0])
	}

	for i := 1; i < len(ls); i++ {
		if debug {
			log.Printf("Looking at point[%v]: %v", i, ls[i])
		}

		if err := ctx.Err(); err != nil {
			return nil, err
		}

		ln := geom.Line{ls[i-1], ls[i]}

		// is the current point inside the clip box.
		cptIsIn := clipbox.ContainsPoint(ls[i])

		switch {

		// Both points are outside, however it is possible that the line they form goes
		// through the clipbox.
		case !lptIsIn && !cptIsIn:
			if debug {
				log.Printf("both points (%v,%v) are not in clipbox.", i-1, i)
			}
			if ipts := uniqueSegmentIntersectPoints(clipbox, ln); len(ipts) > 1 {
				isLess := cmp.PointLess(ls[i-1], ls[i])
				isCLess := cmp.PointLess(ipts[0], ipts[1])
				f, s := 0, 1
				if isLess != isCLess {
					f, s = 1, 0
				}
				mls = append(mls, [][2]float64{ipts[f], ipts[s]})
				if debug {
					log.Printf("Adding line[%v,%v] to mls: %v", ipts[f], ipts[s], mls)
				}
			}

		// Both points are inside, add it to the current line.
		case lptIsIn && cptIsIn:
			cls = append(cls, ls[i])
			if debug {
				log.Printf("Both points(%v,%v) are inside cls: %v", i-1, i, cls)
			}

		// Entering into the location, we should always have at least one intersection point.
		case !lptIsIn && cptIsIn:
			if ipts := uniqueSegmentIntersectPoints(clipbox, ln); len(ipts) > 0 {
				if len(ipts) == 1 {
					cls = append(cls, ipts[0])
				} else {
					isLess := cmp.PointLess(ls[i-1], ls[i])
					isCLess := cmp.PointLess(ipts[0], ipts[1])
					idx := 1
					if isLess == isCLess {
						idx = 0
					}
					cls = append(cls, ipts[idx])
				}
			}
			cls = append(cls, ls[i])
			if debug {
				log.Printf("point(%v) outside point(%v) inside, cls: %v", i-1, i, cls)
			}

		// Exiting the region, we should always ahve at least one intersection point.
		case lptIsIn && !cptIsIn:
			if ipts := uniqueSegmentIntersectPoints(clipbox, ln); len(ipts) > 0 {
				// It's is possible that our intersect point is the same as our ls[i-1].
				// if this is the case we need to ignore it.
				lptidx := len(cls) - 1
				for i := range ipts {
					if !cmp.PointEqual(ipts[i], cls[lptidx]) {
						cls = append(cls, ipts[i])
					}
				}
			}
			mls = append(mls, cls)
			cls = make([][2]float64, 0)
			if debug {
				log.Printf("point(%v) inside point(%v) outside, mls: %v", i-1, i, mls)
			}

		}
		lptIsIn = cptIsIn

	}

	// There needs to be at least two points in the line for it to be a good line.
	if len(cls) > 1 {
		mls = append(mls, cls)
	}
	return mls, nil
}

func MultiLineStringer(ctx context.Context, multils geom.MultiLineStringer, clipbox *geom.Extent) (nmls geom.MultiLineString, err error) {
	mls := multils.LineStrings()
	if clipbox.IsUniverse() {
		return geom.MultiLineString(mls), nil
	}

	if len(mls) == 0 {
		return nmls, nil
	}

	for i := range mls {
		mls1, err := lineString(ctx, mls[i], clipbox)
		if err != nil {
			return nil, err
		}
		nmls = append(nmls, mls1...)
	}
	return nmls, nil
}
