// Package fuzz provides primitives to generate random geom geometry types.
package fuzz

import (
	"math/rand"
	"time"

	"github.com/go-spatial/tegola/geom"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

func genNil(withNil bool) bool { return withNil && rand.Intn(100) < 2 }

// GenRandPoint will generate a random point. It is possible that the point may be nil.
func GenRandPoint() *geom.Point {
	if genNil(true) {
		return (*geom.Point)(nil)
	}
	return &geom.Point{rand.NormFloat64(), rand.NormFloat64()}
}

func genRandSlicePoint(size int) (pts [][2]float64) {
	for i := 0; i < size; i++ {
		pts = append(pts, [2]float64{rand.NormFloat64(), rand.NormFloat64()})
	}
	return pts
}

// GenRandMultiPoint will generate a MultiPoint that may be nil, and will have a random number of points. There is no guarantee that all points are unique.
func GenRandMultiPoint() *geom.MultiPoint {
	if genNil(true) {
		return (*geom.MultiPoint)(nil)
	}
	mp := geom.MultiPoint(genRandSlicePoint(rand.Intn(1000)))
	return &mp
}

// GenRandLineString will generate a random LineString (that may be nil depending on withNil), and a randome number of points. There is no guarantee that the line string is simple.
func GenRandLineString(withNil bool) *geom.LineString {
	if genNil(withNil) {
		return (*geom.LineString)(nil)
	}
	ls := geom.LineString(genRandSlicePoint(rand.Intn(1000)))
	return &ls
}

// GenRandMultiLineString will generate a random MultiLineString (that may be nil depending on withNil), and a random number of linestrings. There is no gaurantee that the line strings are simple.
func GenRandMultiLineString(withNil bool) *geom.MultiLineString {
	if genNil(withNil) {
		return (*geom.MultiLineString)(nil)
	}
	num := rand.Intn(1000)
	var ml geom.MultiLineString
	for i := 0; i < num; i++ {
		ls := GenRandLineString(false)
		ml = append(ml, [][2]float64(*ls))
	}
	return &ml
}

// GenRandPolygon will generate a random Polygon (that may be nil depending on withNil). The Polygon may not be valid or simple.
func GenRandPolygon(withNil bool) *geom.Polygon {
	if genNil(withNil) {
		return (*geom.Polygon)(nil)
	}
	num := rand.Intn(100)
	var p geom.Polygon
	for i := 0; i < num; i++ {
		ls := GenRandLineString(false)
		p = append(p, [][2]float64(*ls))
	}
	return &p
}

// GenRandMultiPolygon will generate a random MultiPolygon (that may be nil depending on withNil). The Polygons may not be valid or simple.
func GenRandMultiPolygon(withNil bool) *geom.MultiPolygon {
	if genNil(withNil) {
		return (*geom.MultiPolygon)(nil)
	}
	num := rand.Intn(10)
	var mp geom.MultiPolygon
	for i := 0; i < num; i++ {
		p := GenRandPolygon(false)
		mp = append(mp, [][][2]float64(*p))
	}
	return &mp
}

// GenRandCollection will generate a random Collection (that may be nil depending on withNil).
func GenRandCollection(withNil bool) *geom.Collection {
	if genNil(withNil) {
		return (*geom.Collection)(nil)
	}
	num := rand.Intn(10)
	var col geom.Collection
	for i := 0; i < num; i++ {
		col = append(col, GenGeometry())
	}
	return &col
}

// GenGenometry will generate a random Geometry. The geometry may be nil.
func GenGeometry() geom.Geometry {
	switch rand.Intn(22) {
	default:
		return nil
	case 0, 13, 20:
		return GenRandPoint()
	case 2, 11, 19:
		return GenRandMultiPoint()
	case 4, 9, 18:
		return GenRandLineString(true)
	case 6, 7, 17:
		return GenRandMultiLineString(true)
	case 8, 5, 16:
		return GenRandPolygon(true)
	case 10, 3, 15:
		return GenRandMultiPolygon(true)
	case 12, 1, 14:
		return GenRandCollection(true)
	}

}
