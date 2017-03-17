package basic

import (
	"fmt"

	"github.com/terranodo/tegola"
	"github.com/terranodo/tegola/maths"
)

// Line is a basic line type which is made up of two or more points that don't
// interect.
// TODO: We don't really check to make sure the points don't intersect.
type Line []Point

// Just to make basic collection only usable with basic types.
func (Line) basicType()                      {}
func (Line) String() string                  { return "Line" }
func (l Line) Direction() maths.WindingOrder { return maths.WindingOrderOfLine(l) }
func (l Line) GoString() string {
	str := fmt.Sprintf("\n[%v--%v]{\n\t", len(l), l.Direction())
	count := 0
	for i, p := range l {
		if i != 0 {
			str += ","
		}
		str += fmt.Sprintf("(%v,%v)", p[0], p[1])
		if count == 10 {
			str += "\n\t"
			count = 0
		}
		count++
	}
	str += "\n}"
	return str
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

func CloneLine(line tegola.LineString) (l Line) {
	for _, pt := range line.Subpoints() {
		l = append(l, Point{pt.X(), pt.Y()})
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
