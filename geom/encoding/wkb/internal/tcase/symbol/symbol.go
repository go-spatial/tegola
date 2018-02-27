package symbol

import (
	"io"
	"unicode"

	"github.com/go-spatial/tegola/geom/internal/parsing"
)

const (
	Unknown  byte = iota
	Asterisk      // *
	Chars
	Digit        // 1 2 3 4 5 6 7 8 9 0
	Dash         // -
	Plus         // +
	Dot          // .
	Letter       // a-f,A-F category L in unicode
	Space        // Category S in unicode
	Newline      // \r \r\n \n
	ForwardSlash // /
	Underscore   // _
	Comma        // ,
	Colon        // :
	Comment      // \*
	Ccomment     // */
	Lncomment    // //
	Pren         // (
	Dpren        // ((
	Cpren        // )
	Cdpren       // ))
	Brace        // {
	Dbrace       // {{
	Cbrace       // }
	Cdbrace      // }}
	Bracket      // [
	Dbracket     // [[
	Cbracket     // ]
	Cdbracket    // ]]
)

var symbMap = map[rune]byte{
	'-':  Dash,
	'+':  Plus,
	'.':  Dot,
	'_':  Underscore,
	',':  Comma,
	':':  Colon,
	'(':  Pren,
	')':  Cpren,
	'{':  Brace,
	'}':  Cbrace,
	'[':  Bracket,
	']':  Cbracket,
	'/':  ForwardSlash,
	'*':  Asterisk,
	'\n': Newline,
	'\r': Newline,
}

func isOther(r rune) bool {
	_, ok := symbMap[r]
	return !(ok || parsing.IsSpace(r) || unicode.IsDigit(r) || unicode.IsLetter(r))
}
func SplitFn(data []byte, atEOF bool) (advance int, symbol []byte, err error) {
	if len(data) == 0 {
		if atEOF {
			return 0, []byte{parsing.EOF}, io.EOF
		}
		return 0, nil, nil
	}
	fchar, n, err := parsing.GetRune(data, atEOF)
	if err != nil {
		return 0, nil, err
	}
	switch fchar {
	case '-', '.', '_', ':', '+', ',':
		return n, parsing.EncodeSymbol(symbMap[fchar], 1, data[:n]), nil
	case '(', ')', '[', ']', '{', '}':
		return parsing.GetPossibleDouble(fchar, n, symbMap[fchar], data[n:], atEOF)
	case '/':
		schar, sn, err := parsing.GetRune(data[n:], atEOF)
		if err != nil {
			return 0, nil, err
		}
		// Need more data.
		if sn == 0 {
			return 0, nil, nil
		}
		switch schar {
		case '/':
			return n + sn, parsing.EncodeSymbol(Lncomment, 1, data[:n+sn]), nil
		case '*':
			return n + sn, parsing.EncodeSymbol(Comment, 1, data[:n+sn]), nil
		}
		return n, parsing.EncodeSymbol(ForwardSlash, 1, data[:n]), nil
	case '*':
		schar, sn, err := parsing.GetRune(data[n:], atEOF)
		if err != nil {
			return 0, nil, err
		}
		// Need more data.
		if sn == 0 {
			return 0, nil, nil
		}
		switch schar {
		case '/':
			return n + sn, parsing.EncodeSymbol(Ccomment, 1, data[:n+sn]), nil
		}
		return n, parsing.EncodeSymbol(Asterisk, 1, data[:n]), nil

	default:
		switch {
		case unicode.IsDigit(fchar):
			return parsing.GetSeq(Digit, unicode.IsDigit, data, atEOF)
		case parsing.IsNewLine(fchar):
			return parsing.GetSeq(Newline, parsing.IsNewLine, data, atEOF)
		case parsing.IsSpace(fchar):
			return parsing.GetSeq(Space, parsing.IsSpace, data, atEOF)
		case unicode.IsLetter(fchar):
			return parsing.GetSeq(Letter, unicode.IsLetter, data, atEOF)
		}
		return parsing.GetSeq(Chars, isOther, data, atEOF)
	}
}
