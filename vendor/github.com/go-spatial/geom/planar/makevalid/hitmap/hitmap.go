package hitmap

import (
	"log"
	"math"
	"sort"

	"github.com/go-spatial/geom"
	"github.com/go-spatial/geom/planar"
)

func asGeomExtent(e [4]float64) *geom.Extent {
	ee := geom.Extent(e)
	return &ee

}

const (
	Inside  Always = Always(planar.Inside)
	Outside Always = Always(planar.Outside)
)

// always will return  the label for the point.
type Always planar.Label

func (a Always) LabelFor(_ [2]float64) planar.Label { return planar.Label(a) }
func (a Always) Extent() [4]float64 {
	return [4]float64{math.Inf(-1), math.Inf(-1), math.Inf(1), math.Inf(1)}
}
func (a Always) Area() float64 { return math.Inf(1) }

// PolygonHMSliceByAreaDec will allow you to sort a slice of PolygonHM in decending order
type ByAreaDec []planar.HitMapper

func (hm ByAreaDec) Len() int      { return len(hm) }
func (hm ByAreaDec) Swap(i, j int) { hm[i], hm[j] = hm[j], hm[i] }
func (hm ByAreaDec) Less(i, j int) bool {
	ia, ja := hm[i].Area(), hm[j].Area()
	return ia < ja
}

// OrderedHM will iterate through a set of HitMaps looking for the first one to return
// inside, if none of the hitmaps return inside it will return outside.
type OrderedHM []planar.HitMapper

func (hms OrderedHM) LabelFor(pt [2]float64) planar.Label {
	for i := range hms {
		if hms[i].LabelFor(pt) == planar.Inside {
			return planar.Inside
		}
	}
	return planar.Outside
}

// Extent is the accumlative extent of all the extens in the slice.
func (hms OrderedHM) Extent() [4]float64 {
	e := new(geom.Extent)
	for i := range hms {
		e.Add(asGeomExtent(hms[i].Extent()))
	}
	return e.Extent()
}

// Area returns the area of the total extent of the hitmaps that are contain in the slice.
func (hms OrderedHM) Area() float64 {
	return asGeomExtent(hms.Extent()).Area()
}

// NewOrderdHM will add the provided hitmaps in reverse order so that the last hit map is always tried first.
func NewOrderedHM(hms ...planar.HitMapper) OrderedHM {
	ohm := make(OrderedHM, len(hms))
	size := len(hms) - 1
	for i := size; i >= 0; i-- {
		ohm[size-i] = hms[i]
	}
	return ohm
}

func MustNew(clipbox *geom.Extent, geo geom.Geometry) planar.HitMapper {
	hm, err := New(clipbox, geo)
	if err != nil {
		panic(err)
	}
	return hm
}

// NewHitMap will return a Polygon Hit map, a Ordered Hit Map, or a nil Hit map based on the geomtry type.
func New(clipbox *geom.Extent, geo geom.Geometry) (planar.HitMapper, error) {

	switch g := geo.(type) {
	case geom.Polygoner:

		plyg := g.LinearRings()
		if debug {
			log.Printf("Settup up Polygon Hitmap")
			log.Printf("Polygon is: %v", plyg)
			log.Printf("Polygon rings: %v", len(plyg))
		}

		return NewFromPolygons(nil, plyg)

	case geom.MultiPolygoner:

		if debug {
			log.Printf("Settup up MultiPolygon Hitmap")
		}
		return NewFromPolygons(nil, g.Polygons()...)

	case geom.Collectioner:

		if debug {
			log.Printf("Settup up Collections Hitmap")
		}
		geometries := g.Geometries()
		ghms := make([]planar.HitMapper, 0, len(geometries))
		for i := range geometries {
			g, err := New(clipbox, geometries[i])
			if err != nil {
				return nil, err
			}
			// skip empty hitmaps.
			if g == nil {
				continue
			}
			ghms = append(ghms, g)
		}
		sort.Sort(ByAreaDec(ghms))
		return NewOrderedHM(ghms...), nil

	case geom.Pointer, geom.MultiPointer, geom.LineStringer, geom.MultiLineStringer:
		return nil, nil

	default:
		return nil, geom.ErrUnknownGeometry{geo}
	}
}
