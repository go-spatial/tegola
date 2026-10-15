package featuresql

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	maxBytes       = 64 * 1024
	maxTokens      = 8192
	maxDepth       = 32
	maxProjections = 1024
	maxIn          = 256
)

type token struct {
	kind   byte
	text   string
	quoted bool
	offset int
}
type parser struct {
	tokens []token
	index  int
}

func invalid(category string, offset int) error {
	return fmt.Errorf("%w: %s at byte %d", ErrInvalid, category, offset)
}
func unsupported(offset int) error {
	return fmt.Errorf("%w: construct at byte %d", ErrUnsupported, offset)
}

func lex(input string, dialect Dialect) ([]token, error) {
	if dialect > HANA {
		return nil, invalid("dialect", 0)
	}
	if len(input) > maxBytes || !utf8.ValidString(input) {
		return nil, invalid("input bounds or encoding", 0)
	}
	if strings.ContainsAny(input, "\\;\x00") || strings.Contains(input, "--") ||
		strings.Contains(input, "/*") || strings.Contains(input, "*/") {
		return nil, invalid("forbidden lexical syntax", 0)
	}
	for _, macro := range []string{"!BBOX!", "!ZOOM!", "!SCALE_DENOMINATOR!", "!PIXEL_WIDTH!", "!PIXEL_HEIGHT!"} {
		if strings.Contains(strings.ToUpper(input), macro) {
			return nil, invalid("tile macro", 0)
		}
	}
	var tokens []token
	for pos := 0; pos < len(input); {
		r, width := utf8.DecodeRuneInString(input[pos:])
		if unicode.IsSpace(r) {
			pos += width
			continue
		}
		start := pos
		var t token
		switch {
		case r == '\'' || r == '"' || r == '`':
			quote := byte(r)
			if quote != '\'' && ((dialect == MySQL && quote != '`') ||
				((dialect == PostgreSQL || dialect == HANA) && quote != '"')) {
				return nil, invalid("identifier quoting", pos)
			}
			pos++
			var b strings.Builder
			closed := false
			for pos < len(input) {
				if input[pos] == quote {
					if pos+1 < len(input) && input[pos+1] == quote {
						b.WriteByte(quote)
						pos += 2
						continue
					}
					pos++
					closed = true
					break
				}
				b.WriteByte(input[pos])
				pos++
			}
			if !closed {
				return nil, invalid("unterminated quote", start)
			}
			kind := byte('i')
			if quote == '\'' {
				kind = 's'
			} else if b.Len() == 0 {
				return nil, invalid("empty identifier", start)
			}
			t = token{kind: kind, text: b.String(), quoted: kind == 'i', offset: start}
		case unicode.IsLetter(r) || r == '_':
			pos += width
			for pos < len(input) {
				next, w := utf8.DecodeRuneInString(input[pos:])
				if !unicode.IsLetter(next) && !unicode.IsDigit(next) && next != '_' && next != '$' {
					break
				}
				pos += w
			}
			t = token{kind: 'i', text: input[start:pos], offset: start}
		case (r >= '0' && r <= '9') || r == '+' || r == '-' ||
			(r == '.' && pos+1 < len(input) && input[pos+1] >= '0' && input[pos+1] <= '9'):
			if r == '+' || r == '-' {
				pos++
			}
			digits := 0
			for pos < len(input) && input[pos] >= '0' && input[pos] <= '9' {
				pos++
				digits++
			}
			if pos < len(input) && input[pos] == '.' {
				pos++
				for pos < len(input) && input[pos] >= '0' && input[pos] <= '9' {
					pos++
					digits++
				}
			}
			if digits == 0 {
				return nil, invalid("numeric literal", start)
			}
			if pos < len(input) && (input[pos] == 'e' || input[pos] == 'E') {
				pos++
				if pos < len(input) && (input[pos] == '+' || input[pos] == '-') {
					pos++
				}
				exp := pos
				for pos < len(input) && input[pos] >= '0' && input[pos] <= '9' {
					pos++
				}
				if pos == exp {
					return nil, invalid("numeric exponent", start)
				}
			}
			t = token{kind: 'n', text: input[start:pos], offset: start}
		case strings.ContainsRune(".,()=<>*", r):
			pos += width
			if (r == '<' || r == '>') && pos < len(input) && (input[pos] == '=' || (r == '<' && input[pos] == '>')) {
				pos++
			}
			t = token{kind: 'p', text: input[start:pos], offset: start}
		case r == '!':
			if pos+1 >= len(input) || input[pos+1] != '=' {
				return nil, invalid("forbidden lexical syntax", start)
			}
			pos += 2
			t = token{kind: 'p', text: "!=", offset: start}
		default:
			return nil, invalid("unknown lexical syntax", start)
		}
		tokens = append(tokens, t)
		if len(tokens) > maxTokens {
			return nil, invalid("token bound", start)
		}
	}
	tokens = append(tokens, token{kind: 'e', offset: len(input)})
	return tokens, nil
}
func (p *parser) peek() token { return p.tokens[p.index] }
func (p *parser) take() token {
	t := p.peek()
	if t.kind != 'e' {
		p.index++
	}
	return t
}
func keyword(t token, name string) bool {
	return t.kind == 'i' && !t.quoted && strings.EqualFold(t.text, name)
}
func (p *parser) accept(name string) bool {
	t := p.peek()
	if keyword(t, name) || (t.kind == 'p' && t.text == name) {
		p.take()
		return true
	}
	return false
}
func (p *parser) require(name string) error {
	if !p.accept(name) {
		return invalid("expected grammar token", p.peek().offset)
	}
	return nil
}
func outside(t token) bool {
	if t.kind == 'p' && t.text == "*" {
		return true
	}
	for _, name := range []string{
		"WITH", "DISTINCT", "JOIN", "LEFT", "RIGHT", "INNER", "OUTER", "CROSS",
		"UNION", "INTERSECT", "EXCEPT", "GROUP", "HAVING", "ORDER", "LIMIT",
		"OFFSET", "FOR", "WINDOW", "OVER", "RETURNING", "INTO", "SELECT",
	} {
		if keyword(t, name) {
			return true
		}
	}
	return false
}
func (p *parser) identifier() (Identifier, error) {
	t := p.peek()
	if outside(t) {
		return Identifier{}, unsupported(t.offset)
	}
	if t.kind != 'i' {
		return Identifier{}, invalid("identifier", t.offset)
	}
	for _, name := range []string{"FROM", "WHERE", "AS", "AND", "OR", "NOT", "IN", "IS", "NULL", "TRUE", "FALSE"} {
		if keyword(t, name) {
			return Identifier{}, invalid("identifier", t.offset)
		}
	}
	p.take()
	return Identifier{name: t.text, quoted: t.quoted}, nil
}
func (p *parser) column() (Identifier, Identifier, bool, error) {
	first, err := p.identifier()
	if err != nil {
		return Identifier{}, Identifier{}, false, err
	}
	if p.accept(".") {
		second, err := p.identifier()
		if err != nil {
			return Identifier{}, Identifier{}, false, err
		}
		if p.peek().text == "(" {
			return Identifier{}, Identifier{}, false, unsupported(p.peek().offset)
		}
		return first, second, true, nil
	}
	if p.peek().text == "(" {
		return Identifier{}, Identifier{}, false, unsupported(p.peek().offset)
	}
	return Identifier{}, first, false, nil
}
func Parse(input string, dialect Dialect) (*Plan, error) {
	tokens, err := lex(input, dialect)
	if err != nil {
		return nil, err
	}
	p := parser{tokens: tokens}
	if !p.accept("SELECT") {
		if outside(p.peek()) {
			return nil, unsupported(p.peek().offset)
		}
		return nil, invalid("SELECT required", p.peek().offset)
	}
	plan := &Plan{}
	for {
		qual, col, qualified, err := p.column()
		if err != nil {
			return nil, err
		}
		output := col
		if p.accept("AS") {
			output, err = p.identifier()
		} else if p.peek().kind == 'i' && !keyword(p.peek(), "FROM") && !outside(p.peek()) {
			output, err = p.identifier()
		}
		if err != nil {
			return nil, err
		}
		plan.projections = append(plan.projections, Projection{
			source: col, qualifier: qual, qualified: qualified, output: output,
		})
		if len(plan.projections) > maxProjections {
			return nil, invalid("projection bound", p.peek().offset)
		}
		if !p.accept(",") {
			break
		}
	}
	if err := p.require("FROM"); err != nil {
		return nil, err
	}
	if p.peek().text == "(" {
		return nil, unsupported(p.peek().offset)
	}
	relation, err := p.identifier()
	if err != nil {
		return nil, err
	}
	plan.relation.parts = append(plan.relation.parts, relation)
	if p.accept(".") {
		relation, err = p.identifier()
		if err != nil {
			return nil, err
		}
		plan.relation.parts = append(plan.relation.parts, relation)
	}
	if p.accept("AS") {
		plan.alias, err = p.identifier()
		plan.hasAlias = true
	} else if p.peek().kind == 'i' && !keyword(p.peek(), "WHERE") && !outside(p.peek()) {
		plan.alias, err = p.identifier()
		plan.hasAlias = true
	}
	if err != nil {
		return nil, err
	}
	if p.accept("WHERE") {
		plan.predicate, err = p.expression(1)
		if err != nil {
			return nil, err
		}
		plan.hasPredicate = true
	}
	if p.peek().kind != 'e' {
		if outside(p.peek()) || p.peek().text == "(" {
			return nil, unsupported(p.peek().offset)
		}
		return nil, invalid("trailing syntax", p.peek().offset)
	}
	if plan.hasPredicate && predicateDepth(plan.predicate) > maxDepth {
		return nil, invalid("predicate depth", 0)
	}
	return plan, nil
}
func predicateDepth(p Predicate) int {
	depth := 1
	for _, c := range p.children {
		candidate := 1 + predicateDepth(c)
		if candidate > depth {
			depth = candidate
		}
	}
	return depth
}
func (p *parser) expression(depth int) (Predicate, error) {
	left, err := p.conjunction(depth)
	if err != nil {
		return Predicate{}, err
	}
	for p.accept("OR") {
		right, err := p.conjunction(depth)
		if err != nil {
			return Predicate{}, err
		}
		left = Predicate{kind: Or, children: []Predicate{left, right}}
		if predicateDepth(left) > maxDepth {
			return Predicate{}, invalid("predicate depth", p.peek().offset)
		}
	}
	return left, nil
}
func (p *parser) conjunction(depth int) (Predicate, error) {
	left, err := p.atom(depth)
	if err != nil {
		return Predicate{}, err
	}
	for p.accept("AND") {
		right, err := p.atom(depth)
		if err != nil {
			return Predicate{}, err
		}
		left = Predicate{kind: And, children: []Predicate{left, right}}
		if predicateDepth(left) > maxDepth {
			return Predicate{}, invalid("predicate depth", p.peek().offset)
		}
	}
	return left, nil
}
func (p *parser) atom(depth int) (Predicate, error) {
	if depth > maxDepth {
		return Predicate{}, invalid("predicate depth", p.peek().offset)
	}
	if p.accept("NOT") {
		child, err := p.atom(depth + 1)
		return Predicate{kind: Not, children: []Predicate{child}}, err
	}
	if p.accept("(") {
		node, err := p.expression(depth + 1)
		if err != nil {
			return Predicate{}, err
		}
		if err := p.require(")"); err != nil {
			return Predicate{}, err
		}
		return node, nil
	}
	qual, col, qualified, err := p.column()
	if err != nil {
		return Predicate{}, err
	}
	node := Predicate{column: col, qualifier: qual, qualified: qualified}
	if p.accept("IS") {
		node.kind = IsNull
		node.negated = p.accept("NOT")
		if err := p.require("NULL"); err != nil {
			return Predicate{}, err
		}
		return node, nil
	}
	negated := p.accept("NOT")
	if p.accept("IN") {
		node.kind = In
		node.negated = negated
		node.operator = "IN"
		if err := p.require("("); err != nil {
			return Predicate{}, err
		}
		for {
			lit, err := p.literal()
			if err != nil {
				return Predicate{}, err
			}
			node.literals = append(node.literals, lit)
			if len(node.literals) > maxIn {
				return Predicate{}, invalid("IN bound", p.peek().offset)
			}
			if !p.accept(",") {
				break
			}
		}
		if err := p.require(")"); err != nil {
			return Predicate{}, err
		}
		return node, nil
	}
	if negated {
		return Predicate{}, invalid("NOT predicate", p.peek().offset)
	}
	op := p.take()
	if op.kind != 'p' || (op.text != "=" && op.text != "<>" && op.text != "!=" &&
		op.text != "<" && op.text != "<=" && op.text != ">" && op.text != ">=") {
		return Predicate{}, invalid("comparison operator", op.offset)
	}
	lit, err := p.literal()
	if err != nil {
		return Predicate{}, err
	}
	node.kind = Compare
	node.operator = op.text
	node.literals = []Literal{lit}
	return node, nil
}
func (p *parser) literal() (Literal, error) {
	t := p.take()
	switch t.kind {
	case 's':
		return Literal{StringLiteral, t.text}, nil
	case 'n':
		return Literal{NumberLiteral, t.text}, nil
	}
	if keyword(t, "TRUE") {
		return Literal{BooleanLiteral, "true"}, nil
	}
	if keyword(t, "FALSE") {
		return Literal{BooleanLiteral, "false"}, nil
	}
	if outside(t) {
		return Literal{}, unsupported(t.offset)
	}
	return Literal{}, invalid("scalar literal", t.offset)
}
