package hitmap

import (
	"log"

	"github.com/go-spatial/geom"
	"github.com/go-spatial/geom/planar"
	"github.com/go-spatial/geom/planar/intersect"
)

func createSegments(ls [][2]float64, isClosed bool) (segs [][2][2]float64, err error) {
	if len(ls) <= 1 {
		if debug {
			log.Printf("got an invalid linestring for hitmap: %v -- %v", ls, isClosed)
		}
		return nil, geom.ErrInvalidLineString
	}
	i := 0
	for j := 1; j < len(ls); j++ {
		segs = append(segs, [2][2]float64{ls[i], ls[j]})
		i = j
	}
	if isClosed {
		segs = append(segs, [2][2]float64{ls[len(ls)-1], ls[0]})
	}
	return segs, nil
}

type Ring struct {
	r     *intersect.Ring
	Label planar.Label
}

func (r *Ring) MinX() float64 {
	if r == nil || r.r == nil {
		var e *geom.Extent
		return e.MinX()
	}
	return r.r.Extent().MinX()
}
func (r *Ring) MinY() float64 {
	if r == nil || r.r == nil {
		var e *geom.Extent
		return e.MinY()
	}
	return r.r.Extent().MinY()
}
func (r *Ring) MaxX() float64 {
	if r == nil || r.r == nil {
		var e *geom.Extent
		return e.MaxX()
	}
	return r.r.Extent().MaxX()
}
func (r *Ring) MaxY() float64 {
	if r == nil || r.r == nil {
		var e *geom.Extent
		return e.MaxY()
	}
	return r.r.Extent().MaxY()
}

// Contains returns weather the point is contained by the ring, if the point is on the border it is considered not contained.
func (r Ring) ContainsPoint(pt [2]float64) bool { return r.r.ContainsPoint(pt) }

func NewRing(ring [][2]float64, label planar.Label) *Ring {
	r := intersect.NewRingFromPoints(ring...)
	r.IncludeBorder = label == planar.Inside
	return &Ring{r: r, Label: label}
}

type bySmallestBBArea []*Ring

// Sort Interface

func (rs bySmallestBBArea) Len() int { return len(rs) }
func (rs bySmallestBBArea) Less(i, j int) bool {
	ia, ja := rs[i].r.Extent().Area(), rs[j].r.Extent().Area()
	if rs[i].Label != rs[j].Label {
		return rs[i].Label == planar.Outside
	}
	return ia < ja
}
func (rs bySmallestBBArea) Swap(i, j int) { rs[i], rs[j] = rs[j], rs[i] }
