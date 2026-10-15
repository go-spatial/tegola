package basic

import (
	"fmt"

	"github.com/alexeydott/geom"
)

const extentEdgeSamples = 16

// FromWebMercatorExtent converts every corner of a Web Mercator extent to
// SRID and returns the enclosing source-CRS extent. The perimeter is sampled
// between corners because transforming only corners is not conservative for
// projections that rotate or otherwise curve the tile boundary.
func FromWebMercatorExtent(SRID uint64, extent *geom.Extent) (*geom.Extent, error) {
	if extent == nil {
		return nil, fmt.Errorf("cannot convert a nil Web Mercator extent")
	}

	corners := extent.Vertices()
	var convertedExtent geom.Extent
	var initialized bool

	for edge := range corners {
		start := corners[edge]
		end := corners[(edge+1)%len(corners)]
		for sample := 0; sample <= extentEdgeSamples; sample++ {
			fraction := float64(sample) / float64(extentEdgeSamples)
			corner := [2]float64{
				start[0] + (end[0]-start[0])*fraction,
				start[1] + (end[1]-start[1])*fraction,
			}
			converted, err := FromWebMercator(SRID, geom.Point{corner[0], corner[1]})
			if err != nil {
				return nil, err
			}

			point, ok := converted.(geom.Point)
			if !ok {
				return nil, fmt.Errorf("expected converted extent corner to be geom.Point, got %T", converted)
			}

			if !initialized {
				convertedExtent = geom.Extent{point[0], point[1], point[0], point[1]}
				initialized = true
				continue
			}
			if point[0] < convertedExtent.MinX() {
				convertedExtent[0] = point[0]
			}
			if point[1] < convertedExtent.MinY() {
				convertedExtent[1] = point[1]
			}
			if point[0] > convertedExtent.MaxX() {
				convertedExtent[2] = point[0]
			}
			if point[1] > convertedExtent.MaxY() {
				convertedExtent[3] = point[1]
			}
		}
	}

	return &convertedExtent, nil
}
