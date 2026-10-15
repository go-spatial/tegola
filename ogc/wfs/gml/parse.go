package gml

import (
	"encoding/xml"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"

	"github.com/alexeydott/geom"
)

// A06: structural GML parser.
//
// The previous flat parser collected every CharData node into one number
// list, silently merging polygon holes into the exterior ring, collapsing
// multi-part boundaries, and dropping a trailing odd ordinate. This parser
// tracks the geometry tree instead:
//
//   - Polygon: exterior ring first, then interior rings, via the
//     exterior/interior wrapper elements.
//   - MultiPolygon: polygonMember boundaries preserved.
//   - MultiLineString: lineStringMember boundaries preserved.
//   - MultiPoint: pointMember boundaries preserved.
//
// Coordinate sources: posList, pos, and coordinates (GML 3.1.1).
// Only 2D (XY) is accepted: srsDimension != 2, an odd ordinate count, or
// non-finite values are hard errors, never silent truncation.
//
// Unsupported geometry kinds (Curve, Surface, Arc, Circle, etc.) are
// rejected explicitly instead of being flattened into something else.

// ParseGeometry parses a GML geometry element from raw XML. Coordinates are
// expected in the document's axis order; the caller swaps axes when the
// version/SRS requires it.
// axisSwapForSRS determines whether coordinates need XY swap based on
// the srsName (A07). Axis order comes from the CRS definition, not the
// WFS version:
//   - urn:ogc:def:crs:EPSG::4326 -> lat,lon order -> swap to x,y
//   - http://www.opengis.net/gml/srs/epsg.xml#4326 -> lon,lat -> no swap
//   - EPSG:4326 -> commonly lon,lat -> no swap
func axisSwapForSRS(srsName string) bool {
	_, swap, _ := parseSRS(srsName)
	return swap
}

// parseSRS admits only the input CRS profiles supported by the mutation providers.
func parseSRS(name string) (uint64, bool, error) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "", "epsg:4326", "http://www.opengis.net/gml/srs/epsg.xml#4326", "crs:84", "urn:ogc:def:crs:ogc:1.3:crs84", "http://www.opengis.net/def/crs/ogc/1.3/crs84":
		return 4326, false, nil
	case "urn:ogc:def:crs:epsg::4326", "urn:ogc:def:crs:epsg:6.6:4326", "http://www.opengis.net/def/crs/epsg/0/4326":
		return 4326, true, nil
	case "epsg:3857", "urn:ogc:def:crs:epsg::3857", "http://www.opengis.net/def/crs/epsg/0/3857", "http://www.opengis.net/gml/srs/epsg.xml#3857":
		return 3857, false, nil
	default:
		return 0, false, fmt.Errorf("gml: unsupported srsName %q", name)
	}
}

// ParseGeometry parses GML geometry. The srsNameHint is used when the
// geometry has no srsName attribute; axis order is otherwise determined
// from the srsName in the XML (A07).
func ParseGeometry(raw string, srsNameHint string) (geom.Geometry, error) {
	g, _, err := ParseGeometryWithSRID(raw, srsNameHint)
	return g, err
}

// ParseGeometryWithSRID also returns the CRS of the decoded XY coordinates.
func ParseGeometryWithSRID(raw string, srsNameHint string) (geom.Geometry, uint64, error) {
	p := &parser{srsNameHint: srsNameHint}
	if err := p.parse(raw); err != nil {
		return nil, 0, err
	}
	srid, _, err := parseSRS(p.srsName)
	if err != nil {
		return nil, 0, err
	}
	g, err := p.build()
	return g, srid, err
}

// coordList is one parsed coordinate sequence (a ring, a line part, a point).
type coordList struct {
	pts  [][2]float64
	kind string // originating element, for error messages
	// inner is meaningful for polygon rings: true inside <interior>.
	inner bool
	node  int // identity of the containing Point, LineString or LinearRing
}

type parser struct {
	srsNameHint string
	srsName     string // detected from geometry root element

	geomType string
	// stack of open element local names; stack[0] is the geometry root.
	stack    []string
	nodes    []int
	nextNode int
	// dimStack parallels stack: srsDimension at each level (R05 inheritance).
	dimStack []string
	// coordinate text accumulation
	coordText strings.Builder
	inCoords  bool
	coordDim  string

	// structural accumulation
	rings []coordList // polygon rings (Polygon) in document order
	parts []coordList // members (Point/LineString/Multi*)
	// polys accumulates finished polygons of a MultiPolygon; curPoly/curInner
	// accumulate the polygon currently being read.
	polys   [][]coordList
	curPoly []coordList

	seenCoords bool
	rootSeen   bool
	// R05 G15: track open member elements to detect empty members.
	memberStack     []string
	memberHasCoords []bool
}

func (p *parser) parse(raw string) error {
	dec := xml.NewDecoder(strings.NewReader(raw))
	for {
		tok, err := dec.Token()
		if err != nil {
			// R05 G14: only io.EOF after a closed root is clean.
			// Syntax errors after the root must fail, not be ignored.
			if err == io.EOF && p.rootSeen && len(p.stack) == 0 {
				break
			}
			if p.rootSeen {
				return fmt.Errorf("gml: invalid XML after <%s>: %w", p.geomType, err)
			}
			return fmt.Errorf("gml: invalid XML: %w", err)
		}
		switch t := tok.(type) {
		case xml.StartElement:
			name := t.Name.Local
			if len(p.stack) == 0 {
				if p.rootSeen {
					return fmt.Errorf("gml: multiple geometry roots")
				}
				p.rootSeen = true
				p.geomType = name
				// A07: capture srsName for axis order determination.
				p.srsName = attrValue(t.Attr, "srsName")
				if p.srsName == "" {
					p.srsName = p.srsNameHint
				}
				if _, _, err := parseSRS(p.srsName); err != nil {
					return err
				}
				if err := checkSupportedType(name); err != nil {
					return err
				}
			}
			if nameSRS := attrValue(t.Attr, "srsName"); len(p.stack) > 0 && nameSRS != "" {
				srid, swap, err := parseSRS(nameSRS)
				rootSRID, rootSwap, _ := parseSRS(p.srsName)
				if err != nil {
					return err
				}
				if srid != rootSRID || swap != rootSwap {
					return fmt.Errorf("gml: mixed coordinate reference systems are not supported")
				}
			}
			if len(p.stack) > 0 && !validChild(p.stack[len(p.stack)-1], name) {
				return fmt.Errorf("gml: unexpected <%s> inside <%s>", name, p.stack[len(p.stack)-1])
			}
			p.nextNode++
			p.nodes = append(p.nodes, p.nextNode)
			p.stack = append(p.stack, name)
			p.dimStack = append(p.dimStack, attrValue(t.Attr, "srsDimension"))
			// R05 G15: track member boundaries.
			if isMemberElement(name) {
				p.memberStack = append(p.memberStack, name)
				p.memberHasCoords = append(p.memberHasCoords, false)
			}
			if isCoordElement(name) {
				p.inCoords = true
				p.coordText.Reset()
				// R05: inherit srsDimension from ancestors (G09).
				p.coordDim = p.inheritedDim()
			}
		case xml.EndElement:
			name := t.Name.Local
			if p.inCoords && isCoordElement(name) && len(p.stack) > 0 && p.stack[len(p.stack)-1] == name {
				if err := p.finishCoords(); err != nil {
					return err
				}
				p.inCoords = false
			}
			if (name == "polygonMember" || name == "surfaceMember") && (p.geomType == "MultiPolygon" || p.geomType == "MultiSurface") {
				p.polys = append(p.polys, p.curPoly)
				p.curPoly = nil
			}
			// R05 G15: verify member had coordinates.
			if isMemberElement(name) && len(p.memberStack) > 0 {
				idx := len(p.memberStack) - 1
				if p.memberStack[idx] == name {
					if !p.memberHasCoords[idx] {
						return fmt.Errorf("gml: <%s> member carries no coordinates", name)
					}
					p.memberStack = p.memberStack[:idx]
					p.memberHasCoords = p.memberHasCoords[:idx]
				}
			}
			if len(p.stack) > 0 {
				p.stack = p.stack[:len(p.stack)-1]
				p.nodes = p.nodes[:len(p.nodes)-1]
				p.dimStack = p.dimStack[:len(p.dimStack)-1]
			}
		case xml.CharData:
			if p.inCoords {
				p.coordText.Write(t)
			} else if strings.TrimSpace(string(t)) != "" {
				return fmt.Errorf("gml: unexpected text outside coordinates")
			}
		}
	}
	if !p.rootSeen {
		return fmt.Errorf("gml: empty geometry document")
	}
	return nil
}

// enclosingWrapper reports the nearest structural wrapper above the current
// coordinate element.
func (p *parser) enclosingWrapper() string {
	for i := len(p.stack) - 1; i >= 1; i-- {
		switch p.stack[i] {
		case "exterior":
			return "exterior"
		case "interior":
			return "interior"
		}
	}
	return ""
}

// finishCoords parses accumulated coordinate text into the right bucket.
// inheritedDim returns the nearest srsDimension from the stack (R05).
// isMemberElement reports whether name is a multi-geometry member wrapper.
func isMemberElement(name string) bool {
	switch name {
	case "pointMember", "lineStringMember", "polygonMember", "surfaceMember":
		return true
	}
	return false
}

func (p *parser) inheritedDim() string {
	for i := len(p.dimStack) - 1; i >= 0; i-- {
		if p.dimStack[i] != "" {
			return p.dimStack[i]
		}
	}
	return ""
}

func (p *parser) finishCoords() error {
	var elem string
	if len(p.stack) > 0 {
		elem = p.stack[len(p.stack)-1]
	}
	// A07: axis order from srsName in XML, fallback to hint.
	srs := p.srsName
	if srs == "" {
		srs = p.srsNameHint
	}
	pts, err := parseCoordText(elem, p.coordText.String(), p.coordDim, axisSwapForSRS(srs))
	if err != nil {
		return err
	}
	p.seenCoords = true
	// R05 G15: mark current member as having coordinates.
	if len(p.memberHasCoords) > 0 {
		p.memberHasCoords[len(p.memberHasCoords)-1] = true
	}
	inner := p.enclosingWrapper() == "interior"

	node := p.nodes[len(p.nodes)-2]
	cl := coordList{pts: pts, kind: elem, inner: inner, node: node}
	appendTo := func(dst *[]coordList) error {
		if len(*dst) > 0 && (*dst)[len(*dst)-1].node == node {
			last := &(*dst)[len(*dst)-1]
			if elem != "pos" || last.kind != "pos" {
				return fmt.Errorf("gml: repeated or mixed coordinate sequences")
			}
			last.pts = append(last.pts, pts...)
		} else {
			*dst = append(*dst, cl)
		}
		return nil
	}
	switch p.geomType {
	case "Point", "LineString", "MultiPoint", "MultiLineString":
		return appendTo(&p.parts)
	case "Polygon":
		return appendTo(&p.rings)
	case "MultiPolygon", "MultiSurface":
		return appendTo(&p.curPoly)
	default:
		return fmt.Errorf("gml: unsupported geometry %q", p.geomType)
	}
}

// build constructs the final geometry with validation.
func (p *parser) build() (geom.Geometry, error) {
	if !p.seenCoords {
		return nil, fmt.Errorf("gml: %s carries no coordinates", p.geomType)
	}
	switch p.geomType {
	case "Point":
		if len(p.parts) != 1 || len(p.parts[0].pts) != 1 {
			return nil, fmt.Errorf("gml: Point needs exactly 1 coordinate, got %d lists", len(p.parts))
		}
		return geom.Point(p.parts[0].pts[0]), nil
	case "LineString":
		if len(p.parts) != 1 {
			return nil, fmt.Errorf("gml: LineString needs exactly 1 coordinate list, got %d", len(p.parts))
		}
		pts := p.parts[0].pts
		if len(pts) < 2 {
			return nil, fmt.Errorf("gml: LineString needs at least 2 points, got %d", len(pts))
		}
		return geom.LineString(pts), nil
	case "Polygon":
		rings, err := orderedRings(p.rings)
		if err != nil {
			return nil, err
		}
		return geom.Polygon(rings), nil
	case "MultiPoint":
		var out geom.MultiPoint
		for _, part := range p.parts {
			if len(part.pts) != 1 {
				return nil, fmt.Errorf("gml: MultiPoint member needs 1 coordinate, got %d", len(part.pts))
			}
			out = append(out, part.pts[0])
		}
		if len(out) == 0 {
			return nil, fmt.Errorf("gml: MultiPoint carries no members")
		}
		return out, nil
	case "MultiLineString":
		var out geom.MultiLineString
		for _, part := range p.parts {
			if len(part.pts) < 2 {
				return nil, fmt.Errorf("gml: MultiLineString member needs at least 2 points, got %d", len(part.pts))
			}
			out = append(out, part.pts)
		}
		if len(out) == 0 {
			return nil, fmt.Errorf("gml: MultiLineString carries no members")
		}
		return out, nil
	case "MultiPolygon", "MultiSurface":
		if len(p.polys) == 0 {
			return nil, fmt.Errorf("gml: %s carries no polygon members", p.geomType)
		}
		var out geom.MultiPolygon
		for i, poly := range p.polys {
			if len(poly) == 0 {
				return nil, fmt.Errorf("gml: polygon member %d carries no rings", i)
			}
			rings, err := orderedRings(poly)
			if err != nil {
				return nil, fmt.Errorf("gml: polygon member %d: %w", i, err)
			}
			out = append(out, rings)
		}
		return out, nil
	default:
		return nil, fmt.Errorf("gml: unsupported geometry %q", p.geomType)
	}
}

// orderedRings validates rings and returns exterior first, then interiors.
// Each coordList already carries its inner flag from the parser.
func orderedRings(rings []coordList) ([][][2]float64, error) {
	if len(rings) == 0 {
		return nil, fmt.Errorf("gml: polygon carries no rings")
	}
	var exterior *coordList
	var interiors []coordList
	for i := range rings {
		if rings[i].inner {
			interiors = append(interiors, rings[i])
		} else if exterior == nil {
			exterior = &rings[i]
		} else {
			return nil, fmt.Errorf("gml: polygon has more than one exterior ring")
		}
	}
	if exterior == nil {
		return nil, fmt.Errorf("gml: polygon has no exterior ring")
	}
	out := make([][][2]float64, 0, len(rings))
	ring, err := validatedRing(*exterior, "exterior")
	if err != nil {
		return nil, err
	}
	out = append(out, ring)
	for i, r := range interiors {
		ring, err := validatedRing(r, fmt.Sprintf("interior %d", i))
		if err != nil {
			return nil, err
		}
		out = append(out, ring)
	}
	return out, nil
}

// validatedRing checks ring validity: >= 4 points, closed.
func validatedRing(cl coordList, what string) ([][2]float64, error) {
	pts := cl.pts
	if len(pts) < 4 {
		return nil, fmt.Errorf("gml: %s ring needs at least 4 points, got %d", what, len(pts))
	}
	if first, last := pts[0], pts[len(pts)-1]; first != last {
		return nil, fmt.Errorf("gml: %s ring not closed: first %v != last %v", what, first, last)
	}
	return pts, nil
}

func checkSupportedType(name string) error {
	switch name {
	case "Point", "LineString", "Polygon", "MultiPoint", "MultiLineString", "MultiPolygon", "MultiSurface":
		return nil
	default:
		return fmt.Errorf("gml: unsupported geometry type %q (Curve/Surface/Arc/Circle are not supported)", name)
	}
}

func isCoordElement(name string) bool {
	switch name {
	case "pos", "posList", "coordinates":
		return true
	}
	return false
}

func attrValue(attrs []xml.Attr, name string) string {
	for _, a := range attrs {
		if a.Name.Local == name {
			return a.Value
		}
	}
	return ""
}

// parseCoordText parses one coordinate-bearing element.
func parseCoordText(elem, text, dim string, swapXY bool) ([][2]float64, error) {
	if dim != "" && dim != "2" {
		return nil, fmt.Errorf("gml: <%s> srsDimension=%q not supported (only 2D XY)", elem, dim)
	}
	var nums []float64
	var err error
	switch elem {
	case "pos", "posList":
		nums, err = parseNumbers(text)
	case "coordinates":
		nums, err = parseCoordinates311(text)
	default:
		return nil, fmt.Errorf("gml: unexpected coordinate element <%s>", elem)
	}
	if err != nil {
		return nil, err
	}
	// A06: odd ordinate count is a hard error, never a silent drop.
	if elem == "pos" && len(nums) != 2 {
		return nil, fmt.Errorf("gml: pos requires exactly two ordinates")
	}
	if len(nums)%2 != 0 {
		return nil, fmt.Errorf("gml: <%s> has odd ordinate count %d (only 2D XY supported; Z/M rejected)", elem, len(nums))
	}
	if len(nums) == 0 {
		return nil, fmt.Errorf("gml: <%s> carries no coordinates", elem)
	}
	pts := make([][2]float64, 0, len(nums)/2)
	for i := 0; i < len(nums); i += 2 {
		x, y := nums[i], nums[i+1]
		if math.IsNaN(x) || math.IsNaN(y) || math.IsInf(x, 0) || math.IsInf(y, 0) {
			return nil, fmt.Errorf("gml: <%s> has non-finite ordinate at position %d", elem, i)
		}
		if swapXY {
			x, y = y, x
		}
		pts = append(pts, [2]float64{x, y})
	}
	return pts, nil
}

func parseNumbers(s string) ([]float64, error) {
	var out []float64
	for _, f := range strings.Fields(strings.ReplaceAll(strings.ReplaceAll(s, ",", " "), "\n", " ")) {
		v, err := strconv.ParseFloat(f, 64)
		if err != nil {
			return nil, fmt.Errorf("gml: invalid ordinate %q", f)
		}
		out = append(out, v)
	}
	return out, nil
}

// parseCoordinates311 parses GML 3.1.1 <coordinates>: "x,y x,y ...".
func parseCoordinates311(s string) ([]float64, error) {
	var out []float64
	for _, tuple := range strings.Fields(s) {
		parts := strings.Split(tuple, ",")
		if len(parts) != 2 {
			return nil, fmt.Errorf("gml: <coordinates> tuple %q must have exactly 2 ordinates", tuple)
		}
		for _, f := range parts {
			v, err := strconv.ParseFloat(strings.TrimSpace(f), 64)
			if err != nil {
				return nil, fmt.Errorf("gml: invalid ordinate %q", f)
			}
			out = append(out, v)
		}
	}
	return out, nil
}

// validChild keeps unsupported or misplaced geometry nodes from being flattened.
func validChild(parent, child string) bool {
	switch parent {
	case "Point":
		return child == "pos" || child == "coordinates"
	case "LineString", "LinearRing":
		return isCoordElement(child)
	case "Polygon":
		return child == "exterior" || child == "interior"
	case "exterior", "interior":
		return child == "LinearRing"
	case "MultiPoint":
		return child == "pointMember"
	case "MultiLineString":
		return child == "lineStringMember"
	case "MultiPolygon":
		return child == "polygonMember"
	case "MultiSurface":
		return child == "surfaceMember"
	case "pointMember":
		return child == "Point"
	case "lineStringMember":
		return child == "LineString"
	case "polygonMember", "surfaceMember":
		return child == "Polygon"
	}
	return false
}
