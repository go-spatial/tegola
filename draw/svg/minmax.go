package svg

import (
	"fmt"

	"github.com/go-spatial/tegola"
)

type MinMax struct {
	MinX, MinY, MaxX, MaxY int64
}

func (mm MinMax) Min() (int64, int64) {
	return mm.MinX, mm.MinY
}
func (mm MinMax) Max() (int64, int64) {
	return mm.MaxX, mm.MaxY
}

func (mm MinMax) Width() int64 {
	return mm.MaxX - mm.MinX
}
func (mm MinMax) Height() int64 {
	return mm.MaxY - mm.MinY
}
func (mm MinMax) SentinalPts() [][]int64 {
	return [][]int64{
		{mm.MinX, mm.MinY},
		{mm.MaxX, mm.MinY},
		{mm.MaxX, mm.MaxY},
		{mm.MinX, mm.MaxY},
	}
}

func (mm *MinMax) MinMax(m1 *MinMax) *MinMax {

	if mm == nil {
		mm = &MinMax{}
	}

	if m1 == nil {
		return mm
	}

	if m1.MinX < mm.MinX {
		mm.MinX = m1.MinX
	}
	if m1.MinY < mm.MinY {
		mm.MinY = m1.MinY
	}
	if m1.MaxX > mm.MaxX {
		mm.MaxX = m1.MaxX
	}
	if m1.MaxY > mm.MaxY {
		mm.MaxY = m1.MaxY
	}
	return mm
}

func (mm *MinMax) Fn() *MinMax                        { return mm }
func (mm *MinMax) MinMaxFn(fn func() *MinMax) *MinMax { return mm.MinMax(fn()) }
func (mm *MinMax) MinMaxPt(x, y int64) *MinMax        { return mm.MinMax(&MinMax{x, y, x, y}) }
func (mm *MinMax) OfGeometry(gs ...tegola.Geometry) *MinMax {
	for _, g := range gs {
		switch geo := g.(type) {
		case tegola.Point:
			mm.MinMaxPt(int64(geo.X()), int64(geo.Y()))
		case tegola.MultiPoint:
			for _, pt := range geo.Points() {
				mm.MinMaxPt(int64(pt.X()), int64(pt.Y()))
			}
		case tegola.LineString:
			for _, pt := range geo.Subpoints() {
				mm.MinMaxPt(int64(pt.X()), int64(pt.Y()))
			}
		case tegola.MultiLine:
			for _, ln := range geo.Lines() {
				for _, pt := range ln.Subpoints() {
					mm.MinMaxPt(int64(pt.X()), int64(pt.Y()))
				}
			}
		case tegola.Polygon:
			for _, ln := range geo.Sublines() {
				for _, pt := range ln.Subpoints() {
					mm.MinMaxPt(int64(pt.X()), int64(pt.Y()))
				}
			}
		case tegola.MultiPolygon:
			for _, p := range geo.Polygons() {
				for _, ln := range p.Sublines() {
					for _, pt := range ln.Subpoints() {
						mm.MinMaxPt(int64(pt.X()), int64(pt.Y()))
					}
				}
			}
		}
	}
	return mm
}
func (mm *MinMax) String() string {
	if mm == nil {
		return "(nil)[0 0 , 0 0]"
	}
	return fmt.Sprintf("[%v %v , %v %v]", mm.MinX, mm.MinY, mm.MaxX, mm.MaxY)
}

func (mm *MinMax) IsZero() bool {
	return mm == nil ||
		(mm.MinX == 0 && mm.MinY == 0 && mm.MaxX == 0 && mm.MaxY == 0)
}

func (mm *MinMax) ExpandBy(n int64) *MinMax {
	mm.MinX -= n
	mm.MinY -= n
	mm.MaxX += n
	mm.MaxY += n
	return mm
}
