package symbol

import (
	"strings"
	"unicode"

	"github.com/go-spatial/tegola/geom/internal/parsing"
)

const (
	Unknown byte = iota
	Chars
	Newline            // \r \r\n \n
	Letter             // a-f, A-F category L in unicode.
	Digit              // 0-9
	Space              // Category S in unicode
	DoubleQuote        // "
	NumberSign         // #
	Percent            // %
	Ampersand          // &
	Quote              // '
	LeftPren           // (
	RightPren          // )
	Asterisk           // *
	PlusSign           // +
	Period             // .
	Solidus            // /
	Colon              // :
	Semicolon          // ;
	LeftChevron        // <
	RightChevron       // >
	Equal              // =
	QuestionMark       // ?
	LeftBracket        // [
	RightBracket       // ]
	ReverseSolidus     // \
	Circumflex         // ^
	Underscore         // _
	LeftBrace          // {
	RightBrace         // }
	VerticalBar        // |
	DegreeSymbol       // º
	Comma              // ,
	Minus              // -
	Empty              // EMPTY
	ZM                 // ZM
	M                  // M
	GeometryCollection // GEOMETRYCOLLECTION
	Point              // POINT
	Multipoint         // MULTIPOINT
	Linestring         // LINESTIRNG
	Multilinestring    // MULTILINESTRING
	Polygon            // POLYGON
	Multipolygon       // MULTIPOLYGON

)

var keywordMap = map[string]byte{
	"zm":                 ZM,
	"m":                  M,
	"empty":              Empty,
	"point":              Point,
	"multipoint":         Multipoint,
	"linestring":         Linestring,
	"multilinestring":    Multilinestring,
	"polygon":            Polygon,
	"multipolygon":       Multipolygon,
	"geometrycollection": GeometryCollection,
}

var symbolMap = map[rune]byte{
	'\n': Newline,
	'\r': Newline,
	'+':  PlusSign,
	'(':  LeftPren,
	')':  RightPren,
	',':  Comma,
	'-':  Minus,
	'.':  Period,
	/*
		'"':  DoubleQuote,
		'#':  NumberSign,
		'%':  Percent,
		'&':  Ampersand,
		'\'': Quote,
		'*':  Asterisk,
		'/':  Solidus,
		':':  Colon,
		';':  Semicolon,
		'<':  LeftChevron,
		'>':  RightChevron,
		'=':  Equal,
		'?':  QuestionMark,
		'[':  LeftBracket,
		']':  RightBracket,
		'\\': ReverseSolidus,
		'^':  Circumflex,
		'_':  Underscore,
		'{':  LeftBrace,
		'}':  RightBrace,
		'|':  VerticalBar,
		'º':  DegreeSymbol,
		'_':  Underscore,
	*/
}

func isOther(r rune) bool {
	_, ok := symbolMap[r]
	return !(ok || parsing.IsSpace(r) || unicode.IsDigit(r) || unicode.IsLetter(r))
}

func SplitFn(data []byte, atEOF bool) (advance int, symbol []byte, err error) {
	var body []byte
	var count uint64
	var sym byte
	if len(data) == 0 {
		if atEOF {
			advance, sym, count, body, err = parsing.Eof()
			return advance, parsing.EncodeSymbol(sym, count, body), err
		}
		advance, sym, count, body, err = parsing.MoreData()
		return advance, parsing.EncodeSymbol(sym, count, body), err
	}
	fchar, n, err := parsing.GetRune(data, atEOF)
	if err != nil {
		advance, sym, count, body, err = parsing.Eof()
		return advance, parsing.EncodeSymbol(sym, count, body), err
	}
	switch fchar {
	case '-', '.', '+', ',', '(', ')':
		return n, parsing.EncodeSymbol(symbolMap[fchar], 1, data[:n]), nil
	default:
		switch {
		case unicode.IsDigit(fchar):
			return parsing.GetSeq(Digit, unicode.IsDigit, data, atEOF)
		case parsing.IsNewLine(fchar):
			return parsing.GetSeq(Newline, parsing.IsNewLine, data, atEOF)
		case parsing.IsSpace(fchar):
			return parsing.GetSeq(Space, parsing.IsSpace, data, atEOF)
		case unicode.IsLetter(fchar):
			advance, sym, count, body, err = parsing.GetSeq1(Letter, unicode.IsLetter, data, atEOF)
			if err == nil {
				// Let's check to see if it's one of the key words.
				if symb, ok := keywordMap[strings.ToLower(string(body))]; ok {
					sym = symb
				}
			}
			return advance, parsing.EncodeSymbol(sym, count, body), err
		}
		return parsing.GetSeq(Chars, isOther, data, atEOF)
	}
}
