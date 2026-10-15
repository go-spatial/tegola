package geometrycodec

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/alexeydott/geom"
)

// DecodeRawWKT strictly decodes XY and WKT Z. Untagged geometries default to XY;
// a GEOMETRYCOLLECTION Z passes Z to untagged descendants. An unqualified
// collection may mix explicitly Z children and XY children. Third ordinates
// are never inferred, M/ZM are unsupported, and trailing tokens are errors.
// POINT EMPTY becomes nil; empty Point members of MultiPoint are unsupported.
func DecodeRawWKT(v any) (geom.Geometry, error) {
	if v == nil {
		return nil, nil
	}
	var text string
	switch value := v.(type) {
	case string:
		text = value
	case []byte:
		if len(value) > rawMaxInputBytes {
			return nil, fmt.Errorf("raw WKT input exceeds byte limit")
		}
		text = string(value)
	default:
		return nil, fmt.Errorf("raw WKT requires text, got %T", v)
	}
	if len(text) > rawMaxInputBytes {
		return nil, fmt.Errorf("raw WKT input exceeds byte limit")
	}
	parser := rawWKTParser{text: text}
	value, err := parser.geometry(0, false)
	if err != nil {
		return nil, fmt.Errorf("raw WKT: %w", err)
	}
	if parser.peek() != "" {
		return nil, fmt.Errorf("raw WKT trailing data")
	}
	return value.geometry, nil
}

type rawWKTParser struct {
	text      string
	offset    int
	elements  uint64
	cached    string
	cachedEnd int
	hasCached bool
}

func rawWKTSeparator(c byte) bool {
	return c == '(' || c == ')' || c == ',' || strings.IndexByte(" \t\r\n\v\f", c) >= 0
}
func (p *rawWKTParser) peek() string {
	if p.hasCached {
		return p.cached
	}
	start := p.offset
	for start < len(p.text) && strings.IndexByte(" \t\r\n\v\f", p.text[start]) >= 0 {
		start++
	}
	end := start
	if end < len(p.text) {
		if p.text[end] == '(' || p.text[end] == ')' || p.text[end] == ',' {
			end++
		} else {
			for end < len(p.text) && !rawWKTSeparator(p.text[end]) {
				end++
			}
		}
	}
	p.cached = p.text[start:end]
	p.cachedEnd = end
	p.hasCached = true
	return p.cached
}
func (p *rawWKTParser) next() string {
	token := p.peek()
	p.offset = p.cachedEnd
	p.hasCached = false
	return token
}
func (p *rawWKTParser) expect(token string) error {
	if p.next() != token {
		return fmt.Errorf("expected %q", token)
	}
	return nil
}
func (p *rawWKTParser) reserve() error {
	if p.elements >= rawMaxElements {
		return fmt.Errorf("element budget exceeded")
	}
	p.elements++
	return nil
}
func (p *rawWKTParser) position(z bool) ([3]float64, error) {
	if err := p.reserve(); err != nil {
		return [3]float64{}, err
	}
	dimensions := 2
	if z {
		dimensions = 3
	}
	var position [3]float64
	for axis := range dimensions {
		token := p.next()
		if !rawWKTNumber(token) {
			return position, fmt.Errorf("invalid numeric grammar")
		}
		value, err := strconv.ParseFloat(token, 64)
		if err != nil || math.IsNaN(value) || math.IsInf(value, 0) {
			return position, fmt.Errorf("invalid or nonfinite ordinate")
		}
		position[axis] = value
	}
	return position, nil
}
func (p *rawWKTParser) positions(z bool) ([][3]float64, error) {
	if err := p.expect("("); err != nil {
		return nil, err
	}
	points := [][3]float64{}
	for {
		position, err := p.position(z)
		if err != nil {
			return nil, err
		}
		points = append(points, position)
		if p.peek() != "," {
			break
		}
		p.next()
	}
	if err := p.expect(")"); err != nil {
		return nil, err
	}
	return points, nil
}
func (p *rawWKTParser) polygon(z bool) (geom.Geometry, error) {
	if err := p.expect("("); err != nil {
		return nil, err
	}
	rings := [][][3]float64{}
	for {
		if err := p.reserve(); err != nil {
			return nil, err
		}
		ring, err := p.positions(z)
		if err != nil {
			return nil, err
		}
		if len(ring) < 4 || ring[0] != ring[len(ring)-1] {
			return nil, fmt.Errorf("polygon wire ring requires at least four positions and exact closure")
		}
		rings = append(rings, ring)
		if p.peek() != "," {
			break
		}
		p.next()
	}
	if err := p.expect(")"); err != nil {
		return nil, err
	}
	return rawPolygon(rings, z), nil
}
func rawEmpty(kind uint32, z bool) (geom.Geometry, error) {
	switch kind {
	case 1:
		return nil, nil
	case 2:
		return rawLine([][3]float64{}, z), nil
	case 3:
		return rawPolygon([][][3]float64{}, z), nil
	default:
		return rawMulti(kind, z, []rawDecoded{})
	}
}
func (p *rawWKTParser) geometry(depth int, inheritedZ bool) (rawDecoded, error) {
	if depth > rawMaxDepth {
		return rawDecoded{}, fmt.Errorf("maximum nesting depth exceeded")
	}
	if err := p.reserve(); err != nil {
		return rawDecoded{}, err
	}
	name := strings.ToUpper(p.next())
	kinds := map[string]uint32{
		"POINT": 1, "LINESTRING": 2, "POLYGON": 3,
		"MULTIPOINT": 4, "MULTILINESTRING": 5, "MULTIPOLYGON": 6,
		"GEOMETRYCOLLECTION": 7,
	}
	kind, ok := kinds[name]
	if !ok {
		return rawDecoded{}, fmt.Errorf("%w: unsupported geometry name", ErrUnsupportedRawGeometry)
	}
	z := inheritedZ
	switch strings.ToUpper(p.peek()) {
	case "Z":
		p.next()
		z = true
	case "M", "ZM":
		return rawDecoded{}, fmt.Errorf("%w: M/ZM geometry", ErrUnsupportedRawGeometry)
	}
	decoded := rawDecoded{kind: kind, z: z}
	if strings.EqualFold(p.peek(), "EMPTY") {
		p.next()
		geometry, err := rawEmpty(kind, z)
		decoded.geometry = geometry
		return decoded, err
	}
	switch kind {
	case 1:
		points, err := p.positions(z)
		if err != nil {
			return decoded, err
		}
		if len(points) != 1 {
			return decoded, fmt.Errorf("Point requires one position")
		}
		if z {
			decoded.geometry = geom.PointZ(points[0])
		} else {
			decoded.geometry = geom.Point{points[0][0], points[0][1]}
		}
	case 2:
		points, err := p.positions(z)
		if err != nil {
			return decoded, err
		}
		if len(points) < 2 {
			return decoded, fmt.Errorf("line requires at least two positions")
		}
		decoded.geometry = rawLine(points, z)
	case 3:
		geometry, err := p.polygon(z)
		if err != nil {
			return decoded, err
		}
		decoded.geometry = geometry
	default:
		if err := p.expect("("); err != nil {
			return decoded, err
		}
		children := []rawDecoded{}
		for {
			var child rawDecoded
			var err error
			switch kind {
			case 4:
				if strings.EqualFold(p.peek(), "EMPTY") {
					return decoded, fmt.Errorf("%w: empty point member of MultiPoint", ErrUnsupportedRawGeometry)
				}
				wrapped := p.peek() == "("
				if wrapped {
					p.next()
				}
				position, pointErr := p.position(z)
				if pointErr != nil {
					return decoded, pointErr
				}
				if wrapped {
					if err := p.expect(")"); err != nil {
						return decoded, err
					}
				}
				child = rawDecoded{kind: 1, z: z}
				if z {
					child.geometry = geom.PointZ(position)
				} else {
					child.geometry = geom.Point{position[0], position[1]}
				}
			case 5, 6:
				if err := p.reserve(); err != nil {
					return decoded, err
				}
				child = rawDecoded{kind: kind - 3, z: z}
				if strings.EqualFold(p.peek(), "EMPTY") {
					p.next()
					child.geometry, err = rawEmpty(kind-3, z)
				} else if kind == 5 {
					var points [][3]float64
					points, err = p.positions(z)
					if err == nil && len(points) < 2 {
						err = fmt.Errorf("line requires at least two positions")
					}
					if err == nil {
						child.geometry = rawLine(points, z)
					}
				} else {
					child.geometry, err = p.polygon(z)
				}
			case 7:
				child, err = p.geometry(depth+1, z)
			}
			if err != nil {
				return decoded, err
			}
			children = append(children, child)
			if p.peek() != "," {
				break
			}
			p.next()
		}
		if err := p.expect(")"); err != nil {
			return decoded, err
		}
		geometry, err := rawMulti(kind, z, children)
		if err != nil {
			return decoded, err
		}
		decoded.geometry = geometry
	}
	return decoded, nil
}

func rawWKTNumber(token string) bool {
	i := 0
	if i < len(token) && (token[i] == '+' || token[i] == '-') {
		i++
	}
	digits := 0
	for i < len(token) && token[i] >= '0' && token[i] <= '9' {
		digits++
		i++
	}
	if i < len(token) && token[i] == '.' {
		i++
		for i < len(token) && token[i] >= '0' && token[i] <= '9' {
			digits++
			i++
		}
	}
	if digits == 0 {
		return false
	}
	if i < len(token) && (token[i] == 'e' || token[i] == 'E') {
		i++
		if i < len(token) && (token[i] == '+' || token[i] == '-') {
			i++
		}
		exponentDigits := 0
		for i < len(token) && token[i] >= '0' && token[i] <= '9' {
			exponentDigits++
			i++
		}
		if exponentDigits == 0 {
			return false
		}
	}
	return i == len(token)
}
