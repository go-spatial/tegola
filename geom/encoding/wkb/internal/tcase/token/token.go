package token

import (
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/go-spatial/tegola/geom"
	"github.com/go-spatial/tegola/geom/encoding/wkb/internal/tcase/symbol"
	"github.com/go-spatial/tegola/geom/internal/parsing"
)

type T struct {
	Sym *parsing.Scanner
}

var InvalidStartMarkerErr = fmt.Errorf("invalid start marker.")

func (t *T) NextRaw() ([]byte, bool) { return t.Sym.RawPeek() }
func (t *T) Peek() byte              { return t.Sym.NextSymbol() }
func (t *T) Bytes() []byte           { return t.Sym.Bytes() }
func (t *T) Scan() bool              { return t.Sym.Scan() }
func (t *T) Symbol() byte            { return t.Sym.Symbol() }
func (t *T) AtEnd() bool             { return t.Sym.AtEnd() }
func (t *T) NextText() string        { return t.Sym.NextText() }

func (t *T) ScanTill(sym byte) (contents []byte) {
	for t.Peek() != sym {
		contents = append(contents, t.Bytes()...)
		if !t.Scan() {
			return contents
		}
	}
	contents = append(contents, t.Bytes()...)
	return contents
}

func (t *T) ParseTillEndIgnoreComments() (contents []byte) {
	for {
		switch t.Peek() {
		case symbol.Comment:
			t.ParseComment()
		case symbol.Lncomment:
			// This is basically to the end of the line.
			t.ParseLineComment()
			return contents
		case symbol.Newline, parsing.EOF:
			return contents
		default:
			t.Scan()
			contents = append(contents, t.Bytes()...)
		}
		if t.AtEnd() {
			break
		}
	}
	return contents
}

// Assumes that the first token is going to be the symbol for a line comment.
func (t *T) ParseLineComment() (content []byte, err error) {
	if t.Peek() != symbol.Lncomment {
		return nil, InvalidStartMarkerErr
	}
	t.Scan() // move to the next entry.
	t.Scan()
	return t.ScanTill(symbol.Newline), nil
}

func (t *T) ParseComment() (content []byte, err error) {
	var depth int = 1
	if t.Peek() != symbol.Comment {
		return nil, InvalidStartMarkerErr
	}
	t.Scan()
	for t.Scan() {
		if t.Symbol() == symbol.Comment {
			depth++
		}
		if t.Symbol() == symbol.Ccomment {
			depth--
			if depth == 0 {
				return content, nil
			}
		}
		content = append(content, t.Bytes()...)
	}
	return content, fmt.Errorf("End of File with not matching closing comment.")
}
func (t *T) EatCommentsAndSpaces() {
Loop:
	for !t.AtEnd() {
		switch t.Peek() {
		case symbol.Space, symbol.Newline:
			t.Scan()
		case symbol.Comment:
			t.ParseComment()
		case symbol.Lncomment:
			t.ParseLineComment()
		default:
			break Loop
		}
	}
}
func (t *T) EatComment() {
	for t.Peek() == symbol.Comment {
		t.ParseComment()
	}
}
func (t *T) EatSpace() {
	for t.Peek() == symbol.Space {
		t.Scan()
	}
}

// We will ignore all starting spaces, and comments.
func (t *T) ParseLabel() (content []byte, err error) {
loop1:
	for {
		switch t.Peek() {
		case symbol.Comment:
			t.ParseComment()
		case symbol.Lncomment:
			t.ParseLineComment()
		case symbol.Space, symbol.Newline:
			t.Scan()
		case symbol.Letter:
			break loop1
		default:
			return content, fmt.Errorf("Expected to find start of label")

		}
		if t.AtEnd() {
			return content, fmt.Errorf("Expected to find start of label")
		}
	}
	t.Scan()
	content = append(content, t.Bytes()...)
	for !t.AtEnd() {
		switch t.Peek() {
		case symbol.Colon:
			// That's a label.
			t.Scan()
			return content, nil
		case symbol.Dot, symbol.Letter, symbol.Digit, symbol.Dash, symbol.Underscore:
			t.Scan()
			content = append(content, t.Bytes()...)
		default:
			return content, fmt.Errorf("Expected to find colon")
		}
	}
	return content, fmt.Errorf("Expected to find colon")
}

func parseHexString(byts []byte) (vals []byte, err error) {
	if len(byts)%2 != 0 {
		return vals, fmt.Errorf("Not even number.")
	}
	for i := 0; i < len(byts); i += 2 {
		i64, err := strconv.ParseUint(string(byts[i:i+2]), 16, 8)
		if err != nil {
			return vals, err
		}
		vals = append(vals, byte(i64))
	}
	return vals, nil
}
func (t *T) ParseBinaryField() (content []byte, err error) {
loop1:
	for {
		switch t.Peek() {
		case symbol.Comment:
			t.ParseComment()
		case symbol.Lncomment:
			t.ParseLineComment()
		case symbol.Space, symbol.Newline:
			t.Scan()
		case symbol.Dbrace:
			t.Scan()
			break loop1
		default:
			return content, fmt.Errorf("Expected to find start of binary block")

		}
		if t.AtEnd() {
			return content, fmt.Errorf("Expected to find start of binary block")
		}
	}
	var cbytes []byte
	for {
		switch t.Peek() {
		case symbol.Comment:
			t.ParseComment()
		case symbol.Lncomment:
			t.ParseLineComment()
		case symbol.Letter, symbol.Digit:
			t.Scan()
			cbytes = append(cbytes, t.Bytes()...)
		case symbol.Space:
			t.Scan()
		case symbol.Newline:
			t.Scan()
			if len(cbytes) > 0 {
				b, err := parseHexString(cbytes)
				if err != nil {
					return content, err
				}
				content = append(content, b...)
			}
			cbytes = cbytes[0:0]
		case symbol.Cdbrace:
			t.Scan()
			if len(cbytes) > 0 {
				b, err := parseHexString(cbytes)
				if err != nil {
					return content, err
				}
				content = append(content, b...)
			}
			cbytes = cbytes[0:0]
			return content, nil

		case parsing.EOF:
			return content, fmt.Errorf("Unexpected end of file.")
		default:
			return content, fmt.Errorf("Unexpected chars: %v", t.NextText())

		}
		if t.AtEnd() {
			break
		}
	}
	return content, fmt.Errorf("Unexpected end of file.")
}

func (t *T) parseFloat64EVal() []byte {
	var seenPN bool
	var d []byte
	for {
		switch t.Peek() {
		case symbol.Plus, symbol.Dash:
			if seenPN {
				return d
			}
			seenPN = true
			t.Scan()
			d = append(d, t.Bytes()...)
		case symbol.Digit:
			t.Scan()
			d = append(d, t.Bytes()...)
		default:
			return d
		}
	}
	return d
}

func (t *T) ParseFloat64() (float64, error) {
	// (+/-)XXX.xxx
	var seenPN, seenDot bool
	var d []byte
Loop:
	for {
		switch t.Peek() {
		// Skip underscores.
		case symbol.Underscore:
			t.Scan()
		case symbol.Dot:
			if seenDot {
				break Loop
			}
			seenDot = true
			t.Scan()
			d = append(d, t.Bytes()...)
		case symbol.Plus, symbol.Dash:
			if seenPN {
				break Loop
			}
			seenPN = true
			t.Scan()
			d = append(d, t.Bytes()...)
		case symbol.Digit:
			t.Scan()
			d = append(d, t.Bytes()...)
		case symbol.Letter:
			nt := strings.ToLower(t.NextText())
			if nt != "e" {
				break Loop
			}
			t.Scan()
			d = append(d, 'e')
			d = append(d, t.parseFloat64EVal()...)

		default:
			break Loop
		}
	}
	return strconv.ParseFloat(string(d), 64)
}

func (t *T) ParsePoint() (pt geom.Point, err error) {
	// XXX.xxx,YYY.yyy
	var coordidx int
	var lookingForComma bool
	for {
		switch t.Peek() {
		case symbol.Space:
			// Skipp any spaces.
			t.Scan()
		case symbol.Comment:
			t.ParseComment()
		case symbol.Digit, symbol.Dash, symbol.Dot, symbol.Plus:
			if lookingForComma {
				return pt, fmt.Errorf("expected a ',' not '%v'", t.NextText())
			}
			pt[coordidx], err = t.ParseFloat64()
			if err != nil {
				return pt, err
			}
			lookingForComma = true
			if coordidx == 1 {
				return pt, nil
			}
		case symbol.Comma:
			if !lookingForComma {
				return pt, fmt.Errorf("expected to find a float found a comma instead.")
			}
			coordidx = 1
			lookingForComma = false
			t.Scan()
		case parsing.EOF:
			return pt, nil
		default:
			return pt, fmt.Errorf("Expected a number or comma, not '%v' -- %v", t.NextText(), coordidx)
		}
	}
	return pt, fmt.Errorf("Expected a number or comma, not '%v'", t.NextText())
}

func (t *T) ParseMultiPoint() (pts geom.MultiPoint, err error) {
	//  [ XXX.xxx,YYY.yyy XXX.xxx,YYY.yyy ]
	var stringStarted bool
	for !t.AtEnd() {
		switch t.Peek() {
		case symbol.Space, symbol.Newline:
			// Skip any spaces.
			t.Scan()
		case symbol.Comment:
			// Skip any comments
			t.ParseComment()
		case symbol.Lncomment:
			t.ParseLineComment()
		case symbol.Pren:
			stringStarted = true
			t.Scan()
		case symbol.Cpren:
			t.Scan()
			return pts, nil
		case symbol.Digit, symbol.Dash, symbol.Dot, symbol.Plus:
			if !stringStarted {
				return nil, fmt.Errorf("Expected '(', found '%v'", t.NextText())
			}
			// Looks like a number, assume we have a point.
			pt, err := t.ParsePoint()
			if err != nil {
				return nil, err
			}
			pts = append(pts, pt)

		default:
			if !stringStarted {
				return nil, fmt.Errorf("Expected '(', found '%v'", t.NextText())
			}
			return nil, fmt.Errorf("Expected point or ')' not '%v'", t.NextText())
		}
	}
	return nil, fmt.Errorf("Expected point or ')' not end of file.")
}

func (t *T) ParseLineString() (pts geom.LineString, err error) {
	//  [ XXX.xxx,YYY.yyy XXX.xxx,YYY.yyy ]
	var stringStarted bool
	for !t.AtEnd() {
		switch t.Peek() {
		case symbol.Space, symbol.Newline:
			// Skip any spaces.
			t.Scan()
		case symbol.Comment:
			// Skip any comments
			t.ParseComment()
		case symbol.Lncomment:
			t.ParseLineComment()
		case symbol.Bracket:
			stringStarted = true
			t.Scan()
		case symbol.Cbracket:
			t.Scan()
			return pts, nil
		case symbol.Digit, symbol.Dash, symbol.Dot, symbol.Plus:
			if !stringStarted {
				return nil, fmt.Errorf("Expected '[', found '%v'", t.NextText())
			}
			// Looks like a number, assume we have a point.
			pt, err := t.ParsePoint()
			if err != nil {
				return nil, err
			}
			pts = append(pts, pt)

		default:
			return nil, fmt.Errorf("Expected point or ']' not '%v'", t.NextText())
		}
	}
	return nil, fmt.Errorf("Expected point or ']' not end of file.")
}

func (t *T) ParseMultiLineString() (lns geom.MultiLineString, err error) {
	//  [[ [ XXX.xxx,YYY.yyy XXX.xxx,YYY.yyy ] ]]
	var started bool
	for !t.AtEnd() {
		switch t.Peek() {
		case symbol.Space, symbol.Newline:
			// Skip any spaces.
			t.Scan()
		case symbol.Comment:
			// Skip any comments
			t.ParseComment()
		case symbol.Lncomment:
			t.ParseLineComment()
		case symbol.Dbracket:
			started = true
			t.Scan()
		case symbol.Cdbracket:
			t.Scan()
			return lns, nil
		case symbol.Bracket:
			if !started {
				return nil, fmt.Errorf("Expected '[[', found '['")
			}
			// Looks like a number, assume we have a point.
			ln, err := t.ParseLineString()
			if err != nil {
				return nil, err
			}
			lns = append(lns, ln)

		default:
			if !started {
				return nil, fmt.Errorf("Expected linestring or '[[' not '%v'", t.NextText())
			}
			return nil, fmt.Errorf("Expected linestring or ']]' not '%v'", t.NextText())
		}
	}
	if !started {
		return nil, fmt.Errorf("Expected linestring or '[[' not end of file.")
	}
	return nil, fmt.Errorf("Expected linestring or ']]' not end of file.")
}
func (t *T) ParsePolygon() (lns geom.Polygon, err error) {
	//  { [ XXX.xxx,YYY.yyy XXX.xxx,YYY.yyy ] }
	var started bool
Loop:
	for !t.AtEnd() {
		switch t.Peek() {
		case symbol.Space, symbol.Newline:
			// Skip any spaces.
			t.Scan()
		case symbol.Comment:
			// Skip any comments
			t.ParseComment()
		case symbol.Lncomment:
			t.ParseLineComment()
		case symbol.Brace:
			started = true
			t.Scan()
		case symbol.Cbrace:
			t.Scan()
			return lns, nil
		case symbol.Bracket:
			if !started {
				return nil, fmt.Errorf("Expected '{' found '['")
			}
			// Looks like a number, assume we have a point.
			ln, err := t.ParseLineString()
			if err != nil {
				return nil, err
			}
			lns = append(lns, ln)
		case parsing.EOF:
			break Loop

		default:
			if !started {
				return nil, fmt.Errorf("Expected linestring or '{' not '%v'", t.NextText())
			}
			return nil, fmt.Errorf("Expected linestring or '}' not '%v'\n%v:%v", t.NextText(), t.Peek(), string(t.Bytes()))
		}
	}
	if !started {
		return nil, fmt.Errorf("Expected linestring or '{' not end of file.")
	}
	return nil, fmt.Errorf("Expected linestring or '}' not end of file.")
}
func (t *T) ParseMultiPolygon() (pys geom.MultiPolygon, err error) {
	//  {{ { [ XXX.xxx,YYY.yyy XXX.xxx,YYY.yyy ] } }}
	var started bool
Loop:
	for !t.AtEnd() {
		switch t.Peek() {
		case symbol.Space, symbol.Newline:
			// Skip any spaces.
			t.Scan()
		case symbol.Comment:
			// Skip any comments
			t.ParseComment()
		case symbol.Lncomment:
			t.ParseLineComment()
		case symbol.Dbrace:
			started = true
			t.Scan()
		case symbol.Brace:
			if !started {
				return nil, fmt.Errorf("Expected '{{' found '{'")
			}
			// Looks like a polygon, assume it's a polygon.
			py, err := t.ParsePolygon()
			if err != nil {
				return nil, err
			}
			pys = append(pys, py)
		case symbol.Cdbrace:
			t.Scan()
			return pys, nil
		case parsing.EOF:
			break Loop

		default:
			if !started {
				return nil, fmt.Errorf("Expected polygon or '{{' not '%v'", t.NextText())
			}
			return nil, fmt.Errorf("Expected polygon or '}}' not '%v'", t.NextText())
		}
	}
	if !started {
		return nil, fmt.Errorf("Expected polygon or '{{' not end of file.")
	}
	return nil, fmt.Errorf("Expected polygon or '}}' not end of file.")
}
func (t *T) ParseCollection() (geo geom.Collection, err error) {
	//  {{ { [ XXX.xxx,YYY.yyy XXX.xxx,YYY.yyy ] } }}
	var started bool
Loop:
	for !t.AtEnd() {
		switch t.Peek() {
		case symbol.Space, symbol.Newline:
			// Skip any spaces.
			t.Scan()
		case symbol.Comment:
			// Skip any comments
			t.ParseComment()
		case symbol.Lncomment:
			t.ParseLineComment()

		case symbol.Dpren:
			if !started {
				started = true
				t.Scan()
				break
			}
			col, err := t.ParseCollection()
			if err != nil {
				return nil, err
			}
			geo = append(geo, col)

		case symbol.Digit, symbol.Dot, symbol.Dash, symbol.Plus:
			if !started {
				return nil, fmt.Errorf("Expected '((' found a possible point.")
			}
			pt, err := t.ParsePoint()
			if err != nil {
				return nil, err
			}
			geo = append(geo, pt)
		case symbol.Pren:
			if !started {
				return nil, fmt.Errorf("Expected '((' found a possible muilt-point.")
			}
			pt, err := t.ParseMultiPoint()
			if err != nil {
				return nil, err
			}
			geo = append(geo, pt)

		case symbol.Bracket:
			if !started {
				return nil, fmt.Errorf("Expected '((' found a possible linestring.")
			}
			ln, err := t.ParseLineString()
			if err != nil {
				return nil, err
			}
			geo = append(geo, ln)
		case symbol.Dbracket:
			if !started {
				return nil, fmt.Errorf("Expected '((' found a possible linestring.")
			}
			ln, err := t.ParseLineString()
			if err != nil {
				return nil, err
			}
			geo = append(geo, ln)

		case symbol.Brace:
			if !started {
				return nil, fmt.Errorf("Expected '((' found '{'")
			}
			// Looks like a polygon, assume it's a polygon.
			py, err := t.ParsePolygon()
			if err != nil {
				return nil, err
			}
			geo = append(geo, py)
		case symbol.Dbrace:
			if !started {
				return nil, fmt.Errorf("Expected '((' found '{{'")
			}
			// Looks like a polygon, assume it's a polygon.
			py, err := t.ParseMultiPolygon()
			if err != nil {
				return nil, err
			}
			geo = append(geo, py)

		case symbol.Cdpren:
			t.Scan()
			return geo, nil
		case parsing.EOF:
			break Loop

		default:
			if !started {
				return nil, fmt.Errorf("Expected polygon or '((' not '%v'", t.NextText())
			}
			return nil, fmt.Errorf("Expected polygon or '))' not '%v'", t.NextText())
		}
	}
	if !started {
		return nil, fmt.Errorf("Expected polygon or '((' not end of file.")
	}
	return nil, fmt.Errorf("Expected polygon or '))' not end of file.")
}

func (t *T) ParseExpectedField() (geo interface{}, err error) {
	//  {{ { [ XXX.xxx,YYY.yyy XXX.xxx,YYY.yyy ] } }}
Loop:
	for !t.AtEnd() {
		switch t.Peek() {
		case symbol.Space, symbol.Newline:
			// Skip any spaces.
			t.Scan()
		case symbol.Comment:
			// Skip any comments
			t.ParseComment()
		case symbol.Lncomment:
			t.ParseLineComment()

		case symbol.Dpren:
			col, err := t.ParseCollection()
			if err != nil {
				return nil, err
			}
			var coll geom.Collection
			for _, interf := range col {
				coll = append(coll, geom.Geometry(interf))
			}
			return coll, nil

		case symbol.Digit, symbol.Dot, symbol.Dash, symbol.Plus:
			pt, err := t.ParsePoint()
			if err != nil {
				return nil, err
			}
			return pt, nil
		case symbol.Pren:
			pt, err := t.ParseMultiPoint()
			if err != nil {
				return nil, err
			}
			return pt, nil

		case symbol.Bracket:
			ln, err := t.ParseLineString()
			if err != nil {
				return nil, err
			}
			return ln, nil
		case symbol.Dbracket:
			ln, err := t.ParseMultiLineString()
			if err != nil {
				return nil, err
			}
			return ln, nil

		case symbol.Brace:
			// Looks like a polygon, assume it's a polygon.
			py, err := t.ParsePolygon()
			if err != nil {
				return nil, err
			}
			return py, nil
		case symbol.Dbrace:
			// Looks like a multipolygon, assume it's a multipolygon.
			py, err := t.ParseMultiPolygon()
			if err != nil {
				return nil, err
			}
			return py, nil

		default:
			break Loop
		}
	}
	return nil, fmt.Errorf("Expected point, polygon, linestring or the multivations.")
}

func NewT(r io.Reader) *T {
	return &T{Sym: parsing.NewScanner(r, symbol.SplitFn)}
}
