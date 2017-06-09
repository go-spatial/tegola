package tegola

import (
	"fmt"
	"strings"
)

const goStringPointFormat = "{%.0f,%.0f}, "

func defaultPointDecorator(pt Point) string { return fmt.Sprintf(goStringPointFormat, pt.X(), pt.Y()) }

func pointDecorator(p Point, withType bool, indent int, comment string, ptDecorator func(pt Point) string) string {
	const (
		ptFormat = "%v%v%v // %v\n"
	)
	if ptDecorator == nil {
		ptDecorator = defaultPointDecorator
	}
	indentString := strings.Repeat("\t", indent)
	pstr := strings.Trim(ptDecorator(p), ", ")

	tn := ""
	if withType {
		tn = "basic.Point"
	}
	return fmt.Sprintf(ptFormat, indentString, tn, pstr, comment)
}

func lineDecorator(l LineString, withType bool, indent int, ptsPerLine int, comment string, pointDecorator func(pt Point) string) string {
	if ptsPerLine == 0 {
		ptsPerLine = 10
	}
	const (
		lineFormat      = "%v%v{ // basic.Line len(%06d) %v.\n%v%v}"
		pointLineFormat = "%v\t%v // %06d — %06d\n"
	)
	typeName := ""
	if withType {
		typeName = "basic.Line"
	}
	if pointDecorator == nil {
		pointDecorator = defaultPointDecorator
	}
	indentString := strings.Repeat("\t", indent)
	var byteString, bytestr []rune
	lastI := -1
	pts := l.Subpoints()
	for i, p := range pts {
		byteString = append(byteString, []rune(pointDecorator(p))...)

		if (i+1)%ptsPerLine == 0 {
			bytestr = append(bytestr, []rune(fmt.Sprintf(pointLineFormat, indentString, string(byteString), lastI+1, i))...)
			byteString = byteString[:0] // truncate string.
			lastI = i
		}
	}
	if len(byteString) > 0 {
		bytestr = append(bytestr, []rune(fmt.Sprintf(pointLineFormat, indentString, string(byteString), lastI+1, len(pts)-1))...)
		byteString = byteString[:0] // truncate string.
	}

	return fmt.Sprintf(lineFormat, indentString, typeName, len(pts), comment, string(bytestr), indentString)
}

func polygonDecorator(p Polygon, withType bool, indent int, ptsPerLine int, comment string, pointDecorator func(pt Point) string) string {
	const (
		polygonFormat = "%v%v{ // basic.Polygon len(%06d)%v.\n%v\n%v}"
	)
	typeName := ""
	if withType {
		typeName = "basic.Polygon"
	}
	indentString := strings.Repeat("\t", indent)
	lines := ""
	lns := p.Sublines()
	for i, line := range lns {
		lines += lineDecorator(line, false, indent+1, ptsPerLine, fmt.Sprintf(" line(%02d)", i), pointDecorator) + ",\n"
	}
	return fmt.Sprintf(polygonFormat, indentString, typeName, len(lns), comment, lines, indentString)
}

func multiPolygonDecorator(mp MultiPolygon, withType bool, indent int, ptsPerLine int, comment string, pointDecorator func(pt Point) string) string {
	const (
		polygonFormat = "%v%v{ // basic.MultiPolygon len(%02d)%v.\n%v\n%v}"
	)
	typeName := ""
	if withType {
		typeName = "basic.MultiPolygon"
	}
	indentString := strings.Repeat("\t", indent)
	polygons := ""
	plygs := mp.Polygons()
	for i, p := range plygs {
		polygons += polygonDecorator(p, false, indent+1, ptsPerLine, fmt.Sprintf(" polygon(%02d)", i), pointDecorator) + ",\n"
	}
	return fmt.Sprintf(polygonFormat, indentString, typeName, len(plygs), comment, polygons, indentString)
}

func GeometeryDecorator(g Geometry, ptsPerLine int, comment string, ptDecorator func(pt Point) string) string {
	switch gg := g.(type) {
	case Point:
		return pointDecorator(gg, true, 0, comment, ptDecorator)
	case LineString:
		return lineDecorator(gg, true, 0, ptsPerLine, comment, ptDecorator)
	case Polygon:
		return polygonDecorator(gg, true, 0, ptsPerLine, comment, ptDecorator)
	case MultiPolygon:
		return multiPolygonDecorator(gg, true, 0, ptsPerLine, comment, ptDecorator)
	//case MultiLine:
	default:
		return ""
	}
}
