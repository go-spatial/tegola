// Package sqltoken implements SQL-context-aware handling of tegola
// interpolation tokens — the !BBOX!-style placeholders that SQL providers
// substitute when building queries.
//
// Tokens are meaningful only in ordinary SQL code. A token that appears
// inside a string literal, a quoted identifier, or a comment is SQL text,
// not a substitution site: it must be left verbatim so queries can quote or
// document token-looking text (for example a label '!BBOX!' or a comment
// mentioning !ZOOM!). This package scans SQL into code and protected
// segments and rewrites, detects, or strips tokens in code segments only.
//
// Dialect methods apply the lexical rules of a specific SQL provider. The
// package-level functions preserve the historical union of SQL syntaxes for
// compatibility; providers must use explicit dialects for both detection and
// substitution. PostgreSQL uses standard_conforming_strings=on, with E strings
// providing explicit backslash escapes. MySQL uses its default string mode.
package sqltoken

import (
	"regexp"
	"strings"
)

// Dialect selects SQL lexical rules. Legacy preserves the historical scanner.
type Dialect uint8

const (
	Legacy Dialect = iota
	PostgreSQL
	MySQL
	SQLite
	HANA
)

// TokenRegexp matches a single tegola interpolation token. Token names
// consist of letters, digits, underscores, and dashes between two
// exclamation marks, e.g. !BBOX! or !ID_FIELD!.
var TokenRegexp = regexp.MustCompile(`![a-zA-Z0-9_-]+!`)

// dollarTagRegexp matches the opening delimiter of a PostgreSQL
// dollar-quoted string at the start of the input: $tag$ or $$.
var dollarTagRegexp = regexp.MustCompile(`^\$([A-Za-z_][A-Za-z0-9_]*)?\$`)

// Kind classifies a region of an SQL string.
type Kind int

const (
	// Code is ordinary SQL text; tokens here are substitution sites.
	Code Kind = iota
	// StringLiteral is a single-quoted string literal.
	StringLiteral
	// QuotedIdentifier is a double-quoted, backtick-quoted, or bracketed
	// identifier.
	QuotedIdentifier
	// Comment is a line comment (--, #) or a block comment.
	Comment
	// DollarQuoted is a PostgreSQL dollar-quoted string.
	DollarQuoted
)

// Protected reports whether tokens inside this kind of region must be left
// verbatim.
func (k Kind) Protected() bool { return k != Code }

// Segment is a contiguous region of an SQL string and its classification.
type Segment struct {
	Kind       Kind
	Start, End int // byte offsets; End is exclusive
}

// Scan classifies sql into consecutive segments covering the entire input.
// Adjacent segments never share a boundary gap: for i > 0, segs[i].Start ==
// segs[i-1].End, and the union of the segments is the whole string.
func (d Dialect) Scan(sql string) []Segment {
	var segs []Segment
	codeStart := 0
	i := 0
	n := len(sql)

	addCode := func(end int) {
		if end > codeStart {
			segs = append(segs, Segment{Kind: Code, Start: codeStart, End: end})
		}
	}
	add := func(k Kind, start, end int) {
		addCode(start)
		segs = append(segs, Segment{Kind: k, Start: start, End: end})
		codeStart = end
		i = end
	}

	for i < n {
		c := sql[i]
		switch {
		case c == '\'':
			add(StringLiteral, i, scanQuoted(sql, i, '\'', d == Legacy || d == MySQL || (d == PostgreSQL && postgresEscapePrefix(sql, i))))
		case c == '"':
			add(QuotedIdentifier, i, scanQuoted(sql, i, '"', d == Legacy || d == MySQL))
		case c == '`' && (d == Legacy || d == MySQL || d == SQLite):
			add(QuotedIdentifier, i, scanQuoted(sql, i, '`', d == Legacy))
		case c == '[' && (d == Legacy || d == SQLite):
			add(QuotedIdentifier, i, scanBracket(sql, i, d == Legacy))
		case c == '-' && i+1 < n && sql[i+1] == '-' && (d != MySQL || i+2 == n || sql[i+2] <= ' '):
			add(Comment, i, scanLineComment(sql, i))
		case c == '#' && (d == Legacy || d == MySQL):
			// MySQL line comment; PostgreSQL hash operators remain code.
			add(Comment, i, scanLineComment(sql, i))
		case c == '/' && i+1 < n && sql[i+1] == '*':
			add(Comment, i, scanBlockComment(sql, i, d == Legacy || d == PostgreSQL))
		case c == '$' && (d == Legacy || (d == PostgreSQL && (i == 0 || !identifierByte(sql[i-1])))):
			if end, ok := scanDollarQuote(sql, i); ok {
				add(DollarQuoted, i, end)
			} else {
				i++
			}
		default:
			i++
		}
	}
	addCode(n)
	return segs
}

// scanQuoted consumes a quoted region starting at sql[start] == quote and
// returns the index just past the closing quote. A doubled quote is an
// escaped quote; backslash escapes the next byte when backslash is set. An
// unterminated region extends to the end of sql.
func scanQuoted(sql string, start int, quote byte, backslash bool) int {
	i := start + 1
	for i < len(sql) {
		switch sql[i] {
		case '\\':
			if backslash && i+1 < len(sql) {
				i += 2
				continue
			}
			i++
		case quote:
			if i+1 < len(sql) && sql[i+1] == quote {
				i += 2
				continue
			}
			return i + 1
		default:
			i++
		}
	}
	return len(sql)
}

// scanLineComment consumes a -- or # comment starting at sql[start] and
// returns the index of the end-of-line (or end of sql).
func scanLineComment(sql string, start int) int {
	for i := start; i < len(sql); i++ {
		if sql[i] == '\n' || sql[i] == '\r' {
			return i
		}
	}
	return len(sql)
}

// scanBlockComment consumes a /* ... */ comment starting at sql[start] and
// returns the index just past the closing delimiter. Block comments nest,
// following PostgreSQL semantics. An unterminated comment extends to the
// end of sql.
func scanBlockComment(sql string, start int, nested bool) int {
	depth := 1
	i := start + 2
	for i < len(sql) {
		switch {
		case nested && sql[i] == '/' && i+1 < len(sql) && sql[i+1] == '*':
			depth++
			i += 2
		case sql[i] == '*' && i+1 < len(sql) && sql[i+1] == '/':
			depth--
			i += 2
			if depth == 0 {
				return i
			}
		default:
			i++
		}
	}
	return len(sql)
}

// scanDollarQuote consumes a PostgreSQL dollar-quoted string starting at
// sql[start] == '$' and returns its end and true. It reports ok == false
// when the text at start is not a dollar-quote delimiter (for example the
// $1 placeholder form).
func scanDollarQuote(sql string, start int) (end int, ok bool) {
	open := dollarTagRegexp.FindString(sql[start:])
	if open == "" {
		return 0, false
	}
	rest := sql[start+len(open):]
	if idx := strings.Index(rest, open); idx >= 0 {
		return start + len(open) + idx + len(open), true
	}
	// Unterminated: protect to the end of the input.
	return len(sql), true
}

// Token is a token-shaped match inside a code segment.
type Token struct {
	Start, End int // byte offsets; End is exclusive
	Text       string
}

// CodeTokens returns every token-shaped match in code segments of sql, in
// order of appearance. Tokens in protected contexts are not returned.
func (d Dialect) CodeTokens(sql string) []Token {
	var toks []Token
	for _, seg := range d.Scan(sql) {
		if seg.Kind != Code {
			continue
		}
		for _, loc := range TokenRegexp.FindAllStringIndex(sql[seg.Start:seg.End], -1) {
			s := seg.Start + loc[0]
			e := seg.Start + loc[1]
			toks = append(toks, Token{Start: s, End: e, Text: sql[s:e]})
		}
	}
	return toks
}

// MapTokens rebuilds sql with fn applied to each code-context token. The
// text of protected contexts is copied verbatim. fn receives the token text
// and returns its replacement; returning the input token keeps it verbatim.
func (d Dialect) MapTokens(sql string, fn func(token string) string) string {
	var b strings.Builder
	last := 0
	for _, tok := range d.CodeTokens(sql) {
		b.WriteString(sql[last:tok.Start])
		b.WriteString(fn(tok.Text))
		last = tok.End
	}
	b.WriteString(sql[last:])
	return b.String()
}

// ReplaceToken replaces exact matches of token in code context with value.
// Occurrences of token in protected contexts are left verbatim.
func (d Dialect) ReplaceToken(sql, token, value string) string {
	return d.MapTokens(sql, func(t string) string {
		if t == token {
			return value
		}
		return t
	})
}

// StripTokens removes every code-context token from sql. Token-looking text
// in protected contexts is preserved.
func (d Dialect) StripTokens(sql string) string {
	return d.MapTokens(sql, func(string) string { return "" })
}

// ContainsToken reports whether sql contains any of tokens in code context,
// matching token text exactly.
func (d Dialect) ContainsToken(sql string, tokens ...string) bool {
	for _, tok := range d.CodeTokens(sql) {
		for _, want := range tokens {
			if tok.Text == want {
				return true
			}
		}
	}
	return false
}

// ContainsTokenFold is ContainsToken with case-insensitive token matching.
func (d Dialect) ContainsTokenFold(sql string, tokens ...string) bool {
	for _, tok := range d.CodeTokens(sql) {
		for _, want := range tokens {
			if strings.EqualFold(tok.Text, want) {
				return true
			}
		}
	}
	return false
}

// SpanHasProtectedToken reports whether any token-shaped text within the
// byte range [start, end) of sql lies in a protected context. It guards
// regex-based rewrites that may consume quoted or commented regions: a
// rewrite span carrying a protected token must not be applied.
func (d Dialect) SpanHasProtectedToken(sql string, start, end int) bool {
	if start < 0 {
		start = 0
	}
	if end > len(sql) {
		end = len(sql)
	}
	for _, seg := range d.Scan(sql) {
		if !seg.Kind.Protected() || seg.End <= start || seg.Start >= end {
			continue
		}
		s := seg.Start
		if s < start {
			s = start
		}
		e := seg.End
		if e > end {
			e = end
		}
		if TokenRegexp.MatchString(sql[s:e]) {
			return true
		}
	}
	return false
}

// Scan uses the legacy compatibility scanner.
func Scan(sql string) []Segment { return Legacy.Scan(sql) }

// CodeTokens uses the legacy compatibility scanner.
func CodeTokens(sql string) []Token { return Legacy.CodeTokens(sql) }

// MapTokens uses the legacy compatibility scanner.
func MapTokens(sql string, fn func(token string) string) string { return Legacy.MapTokens(sql, fn) }

// ReplaceToken uses the legacy compatibility scanner.
func ReplaceToken(sql, token, value string) string { return Legacy.ReplaceToken(sql, token, value) }

// StripTokens uses the legacy compatibility scanner.
func StripTokens(sql string) string { return Legacy.StripTokens(sql) }

// ContainsToken uses the legacy compatibility scanner.
func ContainsToken(sql string, tokens ...string) bool { return Legacy.ContainsToken(sql, tokens...) }

// ContainsTokenFold uses the legacy compatibility scanner.
func ContainsTokenFold(sql string, tokens ...string) bool {
	return Legacy.ContainsTokenFold(sql, tokens...)
}

// SpanHasProtectedToken uses the legacy compatibility scanner.
func SpanHasProtectedToken(sql string, start, end int) bool {
	return Legacy.SpanHasProtectedToken(sql, start, end)
}

func identifierByte(c byte) bool {
	return c >= 128 || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '_' || c == '$'
}
func postgresEscapePrefix(sql string, quote int) bool {
	return quote > 0 && (sql[quote-1] == 'E' || sql[quote-1] == 'e') && (quote == 1 || !identifierByte(sql[quote-2]))
}
func scanBracket(sql string, start int, doubled bool) int {
	if doubled {
		return scanQuoted(sql, start, ']', false)
	}
	if end := strings.IndexByte(sql[start+1:], ']'); end >= 0 {
		return start + 2 + end
	}
	return len(sql)
}
