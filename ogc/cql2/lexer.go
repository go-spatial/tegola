// Package cql2 parses the bounded CQL2 text Basic/Permission 1 profile.
package cql2

import (
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/alexeydott/tegola/provider"
)

const maxInputBytes = 65536
const maxTokens = 8192
const maxPropertyBytes = 1024
const maxLiteralBytes = 16384

type tokenKind uint8

const (
	tokenEnd tokenKind = iota
	tokenName
	tokenQuotedName
	tokenString
	tokenNumber
	tokenOperator
	tokenOpen
	tokenClose
	tokenUnsupported
)

type token struct {
	kind   tokenKind
	text   string
	offset int
}

func invalid(offset int, reason string) error {
	return provider.InvalidFeatureQueryError{Field: "filter", Reason: fmt.Sprintf("cql2 %s at byte %d", reason, offset)}
}
func unsupported(offset int) error {
	return errors.Join(invalid(offset, "unsupported expression profile"), provider.ErrUnsupported)
}
func nameStart(r rune) bool {
	return r == ':' || r == '_' || r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z' ||
		r >= 0xc0 && r <= 0xd6 || r >= 0xd8 && r <= 0xf6 || r >= 0xf8 && r <= 0x2ff ||
		r >= 0x370 && r <= 0x37d || r >= 0x37f && r <= 0x1ffe || r >= 0x200c && r <= 0x200d ||
		r >= 0x2070 && r <= 0x218f || r >= 0x2c00 && r <= 0x2fef || r >= 0x3001 && r <= 0xd7ff ||
		r >= 0xf900 && r <= 0xfdcf || r >= 0xfdf0 && r <= 0xfffd || r >= 0x10000 && r <= 0xeffff
}
func namePart(r rune) bool {
	return nameStart(r) || r == '.' || r >= '0' && r <= '9' || r >= 0x300 && r <= 0x36f || r >= 0x203f && r <= 0x2040
}
func space(r rune) bool {
	switch r {
	case ' ', '\t', '\n', '\r', '\v', '\f', 0x85, 0xa0, 0x1680, 0x2028, 0x2029, 0x202f, 0x205f, 0x3000:
		return true
	}
	return r >= 0x2000 && r <= 0x200a
}
func lex(input string) ([]token, error) {
	if len(input) > maxInputBytes {
		return nil, invalid(0, "input exceeds limit")
	}
	if !utf8.ValidString(input) {
		return nil, invalid(0, "invalid utf8")
	}
	out := []token{}
	for i := 0; i < len(input); {
		r, width := utf8.DecodeRuneInString(input[i:])
		if space(r) {
			i += width
			continue
		}
		if len(out) >= maxTokens {
			return nil, invalid(i, "token count exceeds limit")
		}
		start := i
		var item token
		item.offset = start
		switch {
		case r == '\'':
			value, next, err := quotedString(input, i)
			if err != nil {
				return nil, err
			}
			item.kind, item.text = tokenString, value
			i = next
		case r == '"':
			i++
			nameStartAt := i
			first := true
			for i < len(input) && input[i] != '"' {
				n, w := utf8.DecodeRuneInString(input[i:])
				if first && !nameStart(n) || !first && !namePart(n) {
					return nil, invalid(i, "invalid quoted property")
				}
				first = false
				i += w
				if i-nameStartAt > maxPropertyBytes {
					return nil, invalid(start, "property exceeds limit")
				}
			}
			if i == len(input) || first {
				return nil, invalid(start, "unterminated or empty quoted property")
			}
			item.kind, item.text = tokenQuotedName, input[nameStartAt:i]
			i++
		case nameStart(r):
			i += width
			for i < len(input) {
				n, w := utf8.DecodeRuneInString(input[i:])
				if !namePart(n) {
					break
				}
				i += w
			}
			if i-start > maxPropertyBytes {
				return nil, invalid(start, "property exceeds limit")
			}
			item.kind, item.text = tokenName, input[start:i]
		case r >= '0' && r <= '9' || r == '.' || r == '+' || r == '-':
			if r == '-' && i+1 < len(input) && input[i+1] == '-' {
				return nil, invalid(i, "comments are not expressions")
			}
			i = numberEnd(input, start)
			item.kind, item.text = tokenNumber, input[start:i]
			if len(item.text) > maxLiteralBytes {
				return nil, invalid(start, "literal exceeds limit")
			}
		case r == '=' || r == '<' || r == '>':
			i++
			if i < len(input) && (input[i] == '=' || (r == '<' && input[i] == '>')) {
				i++
			}
			item.kind, item.text = tokenOperator, input[start:i]
		case r == '(':
			item.kind = tokenOpen
			i++
		case r == ')':
			item.kind = tokenClose
			i++
		case r == '*' || r == '/' || r == '%' || r == '^' || r == ',':
			if r == '/' && i+1 < len(input) && input[i+1] == '*' {
				return nil, invalid(i, "comments are not expressions")
			}
			item.kind = tokenUnsupported
			i += width
		default:
			return nil, invalid(i, "unexpected character")
		}
		out = append(out, item)
	}
	return append(out, token{kind: tokenEnd, offset: len(input)}), nil
}
func quotedString(input string, start int) (string, int, error) {
	var value strings.Builder
	for i := start + 1; i < len(input); {
		r, width := utf8.DecodeRuneInString(input[i:])
		i += width
		if r == '\'' {
			if i < len(input) && input[i] == '\'' {
				value.WriteByte('\'')
				i++
			} else {
				return value.String(), i, nil
			}
		} else if r == '\\' {
			if i >= len(input) {
				return "", 0, invalid(start, "unterminated string escape")
			}
			escape := input[i]
			i++
			var decoded byte
			switch escape {
			case '\'', '\\':
				decoded = escape
			case 'a':
				decoded = '\a'
			case 'b':
				decoded = '\b'
			case 't':
				decoded = '\t'
			case 'n':
				decoded = '\n'
			case 'v':
				decoded = '\v'
			case 'f':
				decoded = '\f'
			case 'r':
				decoded = '\r'
			default:
				return "", 0, invalid(i-1, "invalid string escape")
			}
			value.WriteByte(decoded)
		} else {
			if r == 0 || r < 7 || r > 13 && r < 32 || r == 0xfffe || r == 0xffff {
				return "", 0, invalid(i-width, "invalid string character")
			}
			value.WriteRune(r)
		}
		if value.Len() > maxLiteralBytes {
			return "", 0, invalid(start, "literal exceeds limit")
		}
	}
	return "", 0, invalid(start, "unterminated string")
}

func numberEnd(input string, start int) int {
	i := start
	if input[i] == '+' || input[i] == '-' {
		i++
	}
	for i < len(input) && input[i] >= '0' && input[i] <= '9' {
		i++
	}
	if i < len(input) && input[i] == '.' {
		i++
		for i < len(input) && input[i] >= '0' && input[i] <= '9' {
			i++
		}
	}
	if i < len(input) && (input[i] == 'e' || input[i] == 'E') {
		i++
		if i < len(input) && (input[i] == '+' || input[i] == '-') {
			i++
		}
		for i < len(input) && input[i] >= '0' && input[i] <= '9' {
			i++
		}
	}
	return i
}
