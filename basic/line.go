package basic

import (
	"fmt"

	"github.com/go-spatial/tegola"
	"github.com/go-spatial/tegola/maths"
)

// Line is a basic line type which is made up of two or more points that don't
// intersect.
// TODO: We don't really check to make sure the points don't intersect.
type Line []Point

// Just to make basic collection only usable with basic types.
func (Line) basicType()                      {}
func (Line) String() string                  { return "Line" }
func (l Line) Direction() maths.WindingOrder { return maths.WindingOrderOfLine(l) }

func (l Line) AsPts() []maths.Pt {
	var line []maths.Pt
	for _, p := range l {
		line = append(line, p.AsPt())
	}
	return line
}

// TODO: gdey remove this function when we have moved over to geomLinestring.
func (l Line) AsGeomLineString() (ln [][2]float64) {
	for i := range l {
		ln = append(ln, [2]float64{l[i].X(), l[i].Y()})
	}
	return ln
}

// Contains tells you weather the given point is contained by the Linestring.
// This assumes the linestring is a connected linestring.
func (l Line) Contains(pt Point) bool {
	pt0 := l[len(l)-1]
	ptln := maths.Line{pt.AsPt(), maths.Pt{pt.X() + 1, pt.Y()}}
	count := 0
	for _, pt1 := range l {
		ln := maths.Line{pt0.AsPt(), pt1.AsPt()}
		if ipt, ok := maths.Intersect(ln, ptln); ok {
			if ipt.IsEqual(pt.AsPt()) {
				return false
			}
			if ln.InBetween(ipt) && ipt.X < pt.X() {
				count++
			}
		}
		pt0 = pt1
	}
	return count%2 != 0
}

func (l Line) ContainsLine(ln Line) bool {
	for _, pt := range ln {
		if !l.Contains(pt) {
			return false
		}
	}
	return true
}

// NewLine creates a line given pairs for floats.
func NewLine(pointPairs ...float64) Line {
	var line Line
	if (len(pointPairs) % 2) != 0 {
		panic(fmt.Sprintf("NewLine requires pair of points. %v", len(pointPairs)%2))
	}
	for i := 0; i < len(pointPairs); i += 2 {
		line = append(line, Point{pointPairs[i], pointPairs[i+1]})
	}
	return line
}

func NewLineFromPt(points ...maths.Pt) Line {
	var line Line
	for _, p := range points {
		line = append(line, Point{p.X, p.Y})
	}
	return line
}
func NewLineTruncatedFromPt(points ...maths.Pt) Line {
	var line Line
	for _, p := range points {
		line = append(line, Point{float64(int64(p.X)), float64(int64(p.Y))})
	}
	return line
}

func NewLineFromSubPoints(points ...tegola.Point) (l Line) {
	l = make(Line, 0, len(points))
	for i := range points {
		l = append(l, Point{points[i].X(), points[i].Y()})
	}
	return l
}

func NewLineFrom2Float64(points ...[2]float64) (l Line) {
	l = make(Line, 0, len(points))
	for i := range points {
		l = append(l, Point{points[i][0], points[i][1]})
	}
	return l
}

// Subpoints return the points in a line.
func (l Line) Subpoints() (points []tegola.Point) {
	points = make([]tegola.Point, 0, len(l))
	for i := range l {
		points = append(points, tegola.Point(l[i]))
	}
	return points
}

// MultiLine is a set of lines.
type MultiLine []Line

func NewMultiLine(pointPairLines ...[]float64) (ml MultiLine) {
	for _, pp := range pointPairLines {
		ml = append(ml, NewLine(pp...))
	}
	return ml
}

func (MultiLine) String() string { return "MultiLine" }

// Just to make basic collection only usable with basic types.
func (MultiLine) basicType() {}

// Lines are the lines in a Multiline
func (ml MultiLine) Lines() (lines []tegola.LineString) {
	lines = make([]tegola.LineString, 0, len(ml))
	for i := range ml {
		lines = append(lines, tegola.LineString(ml[i]))
	}
	return lines
}
