package token

import (
	"fmt"
	"io"
	"log"
	"strconv"
	"strings"

	"github.com/terranodo/tegola/geom"
	"github.com/terranodo/tegola/geom/encoding/wkt/internal/symbol"
	"github.com/terranodo/tegola/geom/internal/parsing"
)

type T struct {
	Sym *parsing.Scanner
}

var UnexpectedEOFErr = fmt.Errorf("unexpected end of file")

var InvalidStartMarkerErr = fmt.Errorf("invalid start marker")

func (t *T) NextRaw() ([]byte, bool) { return t.Sym.RawPeek() }
func (t *T) Peek() byte              { return t.Sym.NextSymbol() }
func (t *T) Bytes() []byte           { return t.Sym.Bytes() }
func (t *T) Scan() bool              { return t.Sym.Scan() }
func (t *T) Symbol() byte            { return t.Sym.Symbol() }
func (t *T) AtEnd() bool             { return t.Sym.AtEnd() }
func (t *T) NextText() string        { return t.Sym.NextText() }
func (t *T) PeekMatch(syms ...byte) bool {
	nsym := t.Peek()
	for _, sym := range syms {
		if nsym == sym {
			return true
		}
	}
	return false
}

func NewT(r io.Reader) *T {
	return &T{Sym: parsing.NewScanner(r, symbol.SplitFn)}
}

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

func (t *T) EatSpace() {
	for t.Peek() == symbol.Space {
		t.Scan()
	}
}

func (t *T) parseFloat64EVal() []byte {
	var seenPN bool
	var d []byte
	for {
		switch t.Peek() {
		case symbol.PlusSign, symbol.Minus:
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
		case symbol.Period:
			if seenDot {
				break Loop
			}
			seenDot = true
			t.Scan()
			d = append(d, t.Bytes()...)
		case symbol.PlusSign, symbol.Minus:
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

func (t *T) parsePointValue() (pt []float64, err error) {
	// XXX YYY [ ZZZ MMM … ]
	var val float64
	for {
		t.EatSpace()
		if t.AtEnd() || t.PeekMatch(symbol.Comma, symbol.RightPren) {
			// This is the end, return what we have.
			break
		}
		val, err = t.ParseFloat64()
		if err != nil {
			break
		}
		pt = append(pt, val)
	}
	return pt, err
}

func (t *T) ParsePoint() (*geom.Point, error) {
	// POINT ( xxx yyy )
	t.EatSpace()
	// First expect to see POINT
	if t.Peek() != symbol.Point {
		return nil, fmt.Errorf("expected to find “POINT”.")
	}
	t.Scan()

	var zm byte
LOOP:
	t.EatSpace()
	switch t.Peek() {
	case symbol.LeftPren:
		t.Scan()
	case symbol.Empty:
		t.Scan()
		// It's a empty point.
		return nil, nil
	case symbol.ZM, symbol.M:
		if zm != 0 {
			return nil, fmt.Errorf("”ZM” or ”M” can only appear once")
		}
		zm = t.Peek()
		t.Scan()
		goto LOOP

	default:
		return nil, fmt.Errorf("expected to find “(” or “EMPTY”")
	}
	pt, err := t.parsePointValue()
	// First We need to see if there is a '('
	if err != nil {
		return nil, err
	}
	t.EatSpace()
	if t.Peek() != symbol.RightPren {
		return nil, fmt.Errorf("expected to find “)”")
	}
	t.Scan()
	if len(pt) < 2 {
		return nil, fmt.Errorf("expected to have at least 2 coordinates in a POINT")
	}
	if len(pt) > 4 {
		return nil, fmt.Errorf("expected to have no more then 4 coordinates in a POINT")
	}
	switch zm {
	case symbol.M:
		if len(pt) != 3 {
			return nil, fmt.Errorf("M POINT should have 3 coordinates")
		}
	case symbol.ZM:
		if len(pt) != 4 {
			return nil, fmt.Errorf("ZM POINT should have 4 coordinates")
		}
	default:
		if len(pt) != 2 {
			return nil, fmt.Errorf("POINT should only have 2 coordinates")
		}
	}

	//TODO: Later will need to support the POINTM and POINTZ and POINTMZ variants.

	return &geom.Point{pt[0], pt[1]}, nil
}

func (t *T) ParseMultiPoint() (pts geom.MultiPoint, err error) {
	// MULTIPOINT (XXX YYY, XXX YYY )
	// MULTIPOINT ((XXX YYY), (XXX YYY))
	t.EatSpace()
	// First expect to see POINT
	if t.Peek() != symbol.Multipoint {
		return nil, fmt.Errorf("expected to find “MULTIPOINT”.")
	}
	t.Scan()
	t.EatSpace()
	switch t.Peek() {
	case symbol.LeftPren:
		t.Scan()
		if debug {
			log.Println("found Left Pren")
		}
	case symbol.Empty:
		t.Scan()
		if debug {
			log.Println("found Empty")
		}
		// It's a empty point.
		return nil, nil

	default:
		return nil, fmt.Errorf("expected to find “(” or “EMPTY”")
	}
	for {
		t.EatSpace()
		// Grab the sub points. Need to check to see if there is a (
		var needRightPren bool

		// First We need to see if there is a '('
		if t.Peek() == symbol.LeftPren {
			t.Scan()
			if debug {
				log.Println("found Left Pren; setting need for right pren")
			}
			needRightPren = true
		}

		pt, err := t.parsePointValue()
		if err != nil {
			return nil, err
		}
		//TODO: Only supporting standard points and M and ZM.
		pts = append(pts, [2]float64{pt[0], pt[1]})
		t.EatSpace()
		if needRightPren {
			if t.Peek() != symbol.RightPren {
				return nil, fmt.Errorf("expected to find “)”")
			}
			t.Scan()
			if debug {
				log.Println("Found matching right pren.")
			}
			needRightPren = false
			t.EatSpace()
		}
		switch t.Peek() {
		case symbol.RightPren:
			t.Scan()
			if debug {
				log.Println("found right pren. ending.")
			}
			// return the single point.
			return pts, nil
		default:
			return nil, fmt.Errorf("expected to find “,” or “)”")
		case symbol.Comma:
			t.Scan()
			if debug {
				log.Println("found a comma, looking for more values.")
			}
			// Let's loop and get more points.
		}
	}
	if debug {
		log.Println("Returning empty point.")
	}

	return nil, nil
}

/*
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
*/
