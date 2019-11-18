package hitmap

import (
	"log"
	"math/big"
	"sort"

	"github.com/go-spatial/geom/encoding/wkt"

	"github.com/go-spatial/geom"
	"github.com/go-spatial/geom/planar"
)

// PolygonHM implements a basic hit map that gives the label for a point based on the order of the rings.
type PolygonHM struct {
	// clipBox this is going to be either the clipping area or the bouding box of all the rings.
	// This allows us to quickly determine if a point is outside the set of rings.
	clipBox *geom.Extent
	// These are the rings
	rings []*Ring
}

// MustNewFromPolygons is like NewFromPolygons except on error it will panic.
func MustNewFromPolygons(clipbox *geom.Extent, plys ...[][][2]float64) *PolygonHM {
	p, err := NewFromPolygons(clipbox, plys...)
	if err != nil {
		panic(err)
	}
	return p
}

// NewFromPolygons assumes that the outer ring of each polygon is inside, and each inner ring is inside.
func NewFromPolygons(clipbox *geom.Extent, plys ...[][][2]float64) (*PolygonHM, error) {

	hm := &PolygonHM{
		clipBox: new(geom.Extent),
	}
	if debug {
		log.Printf("Setting up Hitmap")
		log.Printf("Polygons provided % 5v", len(plys))
		for i := range plys {
			log.Printf("[% 3v] Polygons Rings:[% 3v]", i, len(plys[i]))
			for j := range plys[i] {
				log.Printf("\t[%v]Ring: %v", j, wkt.MustEncode(geom.LineString(plys[i][j])))
			}
		}
	}

	for i := range plys {
		if len(plys[i]) == 0 {
			continue
		}
		{
			if len(plys[i][0]) == 0 {
				continue
			}
			ring := NewRing(plys[i][0], planar.Inside)
			if clipbox == nil {
				// add to the bb of ring to the hm clipbox
				hm.clipBox.Add(ring)
			}
			hm.rings = append(hm.rings, ring)
		}
		if len(plys[i]) <= 1 {
			continue
		}
		for j := range plys[i][1:] {
			if len(plys[i][j+1]) == 0 {
				// Empty cutout skip
				continue
			}
			// plys we assume the first ring is inside, and all other rings are outside.
			ring := NewRing(plys[i][j+1], planar.Outside)
			if clipbox == nil {
				// add to the bb of ring to the hm clipbox
				hm.clipBox.Add(ring)
			}
			hm.rings = append(hm.rings, ring)
		}
	}
	sort.Sort(bySmallestBBArea(hm.rings))
	if debug {
		log.Printf("The Rings are as follows: %v", len(hm.rings))
		for i, r := range hm.rings {
			log.Printf("\t% 5v : { Area: %v Label %v }", i, r.r.Extent().Area(), r.Label)
		}
	}
	return hm, nil
}

// LabelFor returns the label for the given point.
func (hm *PolygonHM) LabelFor(pt [2]float64) planar.Label {
	x, y := big.NewFloat(pt[0]).SetPrec(20), big.NewFloat(pt[1]).SetPrec(20)
	pt[0], _ = x.Float64()
	pt[1], _ = y.Float64()
	if debug {
		log.Printf("Getting label for %v ", pt)
	}
	// nil clipBox contains all points.
	if hm == nil || !hm.clipBox.ContainsPoint(pt) {
		if debug {
			log.Printf("Point out side of clipbox")
		}
		return planar.Outside
	}

	if debug {
		log.Printf("The Rings are as follows: %v", len(hm.rings))
		for i, r := range hm.rings {
			log.Printf("\t% 5v : { Area: %v Label %v }", i, r.r.Extent().Area(), r.Label)
		}
	}

	// We assume the []*Rings are sorted in from smallest area to largest area.
	for i := range hm.rings {
		if hm.rings[i].ContainsPoint(pt) {
			if debug {
				log.Printf("Got a hit on ring %v, with label %v", i, hm.rings[i].Label)
			}
			return hm.rings[i].Label
		}
	}
	if debug {
		log.Printf("No hit found returning outside.")
	}
	return planar.Outside
}

// Extent returns the extent of the hitmap.
func (hm *PolygonHM) Extent() [4]float64 { return hm.clipBox.Extent() }

// Area returns the area covered by the hitmap.
func (hm *PolygonHM) Area() float64 { return hm.clipBox.Area() }
