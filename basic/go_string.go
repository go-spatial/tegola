package basic

import (
	"fmt"
	"strings"
)

/*

   basic.Line{ // len(1000); direction: Counter-Clockwise.
      {100,100}, {100,010}, … // 0 - 40
      … // 40 …
   }


*/

const DefaultPointFormat = "{%06f,%06f}, "

func DefaultPointDecorator(pt Point) string { return fmt.Sprintf(DefaultPointFormat, pt[0], pt[1]) }
func (l Line) GoStringTypeDecorated(withType bool, indent int, lineComment string, pointDecorator func(pt Point) string) string {
	const (
		numberOfPointsPerLine = 10
	)

	const (
		lineFormat      = "%v%v{ // basic.Line len(%06d) direction(%v)%v.\n%v%v}"
		pointLineFormat = "%v\t%v // %06d — %06d\n"
	)
	typeName := ""
	if withType {
		typeName = "basic.Line"
	}
	if pointDecorator == nil {
		pointDecorator = DefaultPointDecorator
	}
	indentString := strings.Repeat("\t", indent)

	var byteString, bytestr []rune
	lastI := -1
	for i, p := range l {
		byteString = append(byteString, []rune(pointDecorator(p))...)

		if (i+1)%numberOfPointsPerLine == 0 {
			bytestr = append(bytestr, []rune(fmt.Sprintf(pointLineFormat, indentString, string(byteString), lastI+1, i))...)
			byteString = byteString[:0] // truncate string.
			lastI = i
		}
	}
	if len(byteString) > 0 {
		bytestr = append(bytestr, []rune(fmt.Sprintf(pointLineFormat, indentString, string(byteString), lastI+1, len(l)-1))...)
		byteString = byteString[:0] // truncate string.
	}

	return fmt.Sprintf(lineFormat, indentString, typeName, len(l), l.Direction(), lineComment, string(bytestr), indentString)
}
func (l Line) GoStringTyped(withType bool, indent int, lineComment string) string {
	return l.GoStringTypeDecorated(withType, indent, lineComment, nil)
}

func (l Line) GoString() string { return l.GoStringTyped(true, 0, "") }

/*
basic.Polygon { // basic.Polygon len(1);
   { lines… },
}
*/
func (p Polygon) GoStringTypeDecorated(withType bool, indent int, lineComment string, pointDecorator func(pt Point) string) string {
	const (
		polygonFormat = "%v%v{ // basic.Polygon len(%06d)%v.\n%v\n%v}"
	)
	typeName := ""
	if withType {
		typeName = "basic.Polygon"
	}
	indentString := strings.Repeat("\t", indent)
	lines := ""
	for i, line := range p {
		lines += line.GoStringTypeDecorated(false, indent+1, fmt.Sprintf(" line(%02d)", i), pointDecorator) + ",\n"
	}
	return fmt.Sprintf(polygonFormat, indentString, typeName, len(p), lineComment, lines, indentString)
}
func (p Polygon) GoStringTyped(withType bool, indent int, lineComment string) string {
	return p.GoStringTypeDecorated(withType, indent, lineComment, nil)
}
func (p Polygon) GoString() string { return p.GoStringTyped(true, 0, "") }

func (p MultiPolygon) GoStringTyped(withType bool, indent int, lineComment string) string {
	const (
		polygonFormat = "%v%v{ // basic.MultiPolygon len(%02d)%v.\n%v\n%v}"
	)
	typeName := ""
	if withType {
		typeName = "basic.MultiPolygon"
	}
	indentString := strings.Repeat("\t", indent)
	polygons := ""
	for i, polygon := range p {
		polygons += polygon.GoStringTyped(false, indent+1, fmt.Sprintf(" polygon(%02d)", i)) + ",\n"
	}
	return fmt.Sprintf(polygonFormat, indentString, typeName, len(p), lineComment, polygons, indentString)
}
func (p MultiPolygon) GoString() string { return p.GoStringTyped(true, 0, "") }
