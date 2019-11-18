package simplify

import (
	"context"
	"strings"

	"github.com/go-spatial/geom/planar"
)

type DouglasPeucker struct {

	// Tolerance is the tolerance used to eliminate points, a tolerance of zero is not eliminate any points.
	Tolerance float64

	// Dist is the distance function to use, defaults to planar.PerpendicularDistance
	Dist planar.PointLineDistanceFunc
}

func (dp DouglasPeucker) Simplify(ctx context.Context, linestring [][2]float64, isClosed bool) ([][2]float64, error) {
	ret := make([][2]float64, 0, len(linestring))
	return dp.simplify(ctx, 0, linestring, isClosed, ret)
}

func rdpPrintf(msg string, depth uint8, params ...interface{}) {
	ps := make([]interface{}, 1, len(params)+1)
	ps[0] = depth
	ps = append(ps, params...)
	logger.Printf(strings.Repeat(" ", int(depth*2))+"[%v]"+msg, ps...)
}

func (dp DouglasPeucker) simplify(ctx context.Context, depth uint8, linestring [][2]float64, isClosed bool, ret [][2]float64) ([][2]float64, error) {

	// helper function for debugging and tracing the code
	if debug {
		rdpPrintf("starting linestring: %v ; tolerance: %v", depth, linestring, dp.Tolerance)
	}

	if dp.Tolerance <= 0 || len(linestring) <= 2 {
		if debug {
			if dp.Tolerance <= 0 {
				rdpPrintf("skipping due to Tolerance (%v) ≤ zero:", depth, dp.Tolerance)

			}
			if len(linestring) <= 2 {
				rdpPrintf("skipping due to len(linestring) (%v) ≤ two: %v", depth, len(linestring), linestring)
			}
		}
		return append(ret, linestring...), nil
	}


	dmax, idx := 0.0, 0
	dist := planar.PerpendicularDistance
	if dp.Dist != nil {
		dist = dp.Dist
	}

	line := [2][2]float64{linestring[0], linestring[len(linestring)-1]}

	if debug {
		rdpPrintf("starting dmax: %v ; idx %v ;  line : %v", depth, dmax, idx, line)
	}

	// Find the point that is the furthest away.
	for i := 1; i <= len(linestring)-2; i++ {
		d := dist(line, linestring[i])
		if d > dmax {
			dmax, idx = d, i
		}

		if debug {
			rdpPrintf("looking at %v ; d : %v dmax %v ", depth, i, d, dmax)
		}
	}
	// If the furtherest point is greater then tolerance, we split at that point, and look again at each
	// subsections.
	if dmax > dp.Tolerance {
		if len(linestring) <= 3 {
			if debug {
				rdpPrintf("returning linestring %v", depth, linestring)
			}
			return append(ret, linestring...), nil
		}
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		// for debug
		startLen := len(ret)

		ret, _ = dp.simplify(ctx, depth+1, linestring[0:idx+1], isClosed, ret)
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		// cut off the last point before recursing because this function will
		// always write the endpoints to the slice
		firstLen := len(ret)
		ret, _ = dp.simplify(ctx, depth+1, linestring[idx:], isClosed, ret[:firstLen-1])
		if debug {
			rdpPrintf("returning combined lines: %v, %v", depth, ret[startLen:firstLen], ret[firstLen:])
		}
		return ret, ctx.Err()
	}

	// Drop all points between the end points.
	if debug {
		rdpPrintf("dropping all points between the end points: %v", depth, line)
	}
	return append(ret, line[:]...), nil
}
