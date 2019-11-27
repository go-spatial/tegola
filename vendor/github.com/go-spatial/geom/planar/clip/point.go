package clip

import (
	"context"

	"github.com/go-spatial/geom"
)

// MultiPoint will filter out points that are not contained by the clipbox.
func MultiPointer(ctx context.Context, pts geom.MultiPointer, clipbox *geom.Extent) (geom.MultiPoint, error) {
	mpts := pts.Points()
	if clipbox.IsUniverse() {
		return geom.MultiPoint(mpts), nil
	}
	if len(mpts) == 0 {
		return nil, nil
	}
	var npts geom.MultiPoint
	for i := range mpts {
		if clipbox.ContainsPoint(mpts[i]) {
			npts = append(npts, mpts[i])
		}
		if err := ctx.Err(); err != nil {
			return nil, err
		}
	}
	return npts, nil
}
