package geom

import (
	"log"
	"math/big"
)

const (
	PrecisionLevelBigFloat = 20
)

// Line has exactly two points
type Line [2][2]float64

func (l Line) IsVertical() bool   { return l[0][0] == l[1][0] }
func (l Line) IsHorizontal() bool { return l[0][1] == l[1][1] }
func (l Line) Point1() *Point     { return (*Point)(&l[0]) }
func (l Line) Point2() *Point     { return (*Point)(&l[1]) }

// ContainsPoint checks to see if the given pont lines on the linesegment. (Incliding the end points.)
func (l Line) ContainsPoint(pt [2]float64) bool {
	minx, maxx := l[0][0], l[1][0]
	if minx > maxx {
		minx, maxx = maxx, minx
	}
	miny, maxy := l[0][1], l[1][1]
	if miny > maxy {
		miny, maxy = maxy, miny
	}
	if debug {
		log.Printf("pt.x %v is between %v and %v: %v && %v", pt[0], minx, maxx, minx <= pt[0], pt[0] <= maxx)
		log.Printf("pt.y %v is between %v and %v: %v && %v", pt[1], miny, maxy, miny <= pt[1], pt[1] <= maxy)
	}

	return minx <= pt[0] && pt[0] <= maxx && miny <= pt[1] && pt[1] <= maxy
}

// ContainsPoint checks to see if the given pont lines on the linesegment. (Incliding the end points.)
func (l Line) ContainsPointBigFloat(pt [2]*big.Float) bool {
	pminx, pmaxx := l[0][0], l[1][0]
	if pminx > pmaxx {
		pminx, pmaxx = pmaxx, pminx
	}
	pminy, pmaxy := l[0][1], l[1][1]
	if pminy > pmaxy {
		pminy, pmaxy = pmaxy, pminy
	}

	minx, maxx := big.NewFloat(pminx).SetPrec(PrecisionLevelBigFloat), big.NewFloat(pmaxx).SetPrec(PrecisionLevelBigFloat)
	miny, maxy := big.NewFloat(pminy).SetPrec(PrecisionLevelBigFloat), big.NewFloat(pmaxy).SetPrec(PrecisionLevelBigFloat)
	px, py := pt[0].SetPrec(PrecisionLevelBigFloat), pt[1].SetPrec(PrecisionLevelBigFloat)

	cmpMinX, cmpMaxX := px.Cmp(minx), px.Cmp(maxx)
	cmpMinY, cmpMaxY := py.Cmp(miny), py.Cmp(maxy)

	goodX := (cmpMinX == 0 || cmpMinX == 1) && (cmpMaxX == 0 || cmpMaxX == -1)
	goodY := (cmpMinY == 0 || cmpMinY == 1) && (cmpMaxY == 0 || cmpMaxY == -1)

	if debug {
		log.Printf("pt.x %v is between %v and %v: %v ,%v : %v", px, minx, maxx, cmpMinX, cmpMaxX, goodX)
		log.Printf("pt.y %v is between %v and %v: %v ,%v : %v", py, miny, maxy, cmpMinY, cmpMaxY, goodY)
	}

	return goodX && goodY
}
