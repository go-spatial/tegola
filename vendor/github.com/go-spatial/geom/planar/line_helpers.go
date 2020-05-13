package planar

import (
	"sort"

	"github.com/go-spatial/geom"
	"github.com/go-spatial/geom/cmp"
)

func NormalizeLines(lines []geom.Line) {
	for i := range lines {
		if !cmp.PointLess(lines[i][0], lines[i][1]) {
			lines[i][0], lines[i][1] = lines[i][1], lines[i][0]
		}
	}
}

type LinesByXY []geom.Line

func (l LinesByXY) Len() int      { return len(l) }
func (l LinesByXY) Swap(i, j int) { l[i], l[j] = l[j], l[i] }
func (l LinesByXY) Less(i, j int) bool {
	if !cmp.PointEqual(l[i][0], l[j][0]) {
		return cmp.PointLess(l[i][0], l[j][0])
	}
	return cmp.PointLess(l[i][1], l[j][1])
}

func NormalizeUniqueLines(lines []geom.Line) []geom.Line {
	NormalizeLines(lines)
	sort.Sort(LinesByXY(lines))
	lns := lines[:0]
	for i := 0; i < len(lines); i++ {
		if i == 0 || !cmp.LineStringEqual(lines[i][:], lines[i-1][:]) {
			lns = append(lns, lines[i])
		}
	}
	return lns
}

type LinesByLength []geom.Line

func (l LinesByLength) Len() int      { return len(l) }
func (l LinesByLength) Swap(i, j int) { l[i], l[j] = l[j], l[i] }
func (l LinesByLength) Less(i, j int) bool {
	lilen := l[i].LengthSquared()
	ljlen := l[j].LengthSquared()
	if lilen == ljlen {
		if cmp.PointEqual(l[i][0], l[j][0]) {
			return cmp.PointLess(l[i][1], l[j][1])
		}
		return cmp.PointLess(l[i][0], l[j][0])
	}
	return lilen < ljlen
}
