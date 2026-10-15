package cql2

import (
	"github.com/alexeydott/tegola/provider"
)

const maxASTDepth = 32
const maxASTNodes = 4096

type parsedNode struct {
	node  provider.FilterNode
	depth int
}
type parser struct {
	tokens []token
	index  int
	nodes  int
}

// Parse parses CQL2 text using the Basic profile's Permission 1 restrictions.
// Property lookup and backend value admission occur after this syntax boundary.
func Parse(text string) (provider.FilterExpression, error) {
	tokens, err := lex(text)
	if err != nil {
		return provider.FilterExpression{}, err
	}
	p := parser{tokens: tokens}
	root, err := p.expression(0)
	if err != nil {
		return provider.FilterExpression{}, err
	}
	if p.current().kind != tokenEnd {
		if advancedToken(p.current()) {
			return provider.FilterExpression{}, unsupported(p.current().offset)
		}
		return provider.FilterExpression{}, invalid(p.current().offset, "unexpected trailing input")
	}
	expression, err := provider.NewFilterExpression(root.node)
	if err != nil {
		return provider.FilterExpression{}, err
	}
	return expression, nil
}
func (p *parser) current() token { return p.tokens[p.index] }
func (p *parser) take() token {
	t := p.current()
	if t.kind != tokenEnd {
		p.index++
	}
	return t
}
func (p *parser) keyword(word string) bool {
	t := p.current()
	return t.kind == tokenName && asciiEqual(t.text, word)
}
func (p *parser) accept(word string) bool {
	if !p.keyword(word) {
		return false
	}
	p.take()
	return true
}
func (p *parser) makeNode(node provider.FilterNode, depth int) (parsedNode, error) {
	p.nodes++
	if p.nodes > maxASTNodes || depth > maxASTDepth {
		return parsedNode{}, invalid(p.current().offset, "expression exceeds structural limit")
	}
	return parsedNode{node: node, depth: depth}, nil
}
func (p *parser) expression(groupDepth int) (parsedNode, error) {
	left, err := p.term(groupDepth)
	if err != nil {
		return parsedNode{}, err
	}
	for p.accept("OR") {
		right, err := p.term(groupDepth)
		if err != nil {
			return parsedNode{}, err
		}
		left, err = p.makeNode(provider.FilterNode{Kind: provider.FilterOr, Children: []provider.FilterNode{left.node, right.node}}, max(left.depth, right.depth)+1)
		if err != nil {
			return parsedNode{}, err
		}
	}
	return left, nil
}
func (p *parser) term(groupDepth int) (parsedNode, error) {
	left, err := p.factor(groupDepth)
	if err != nil {
		return parsedNode{}, err
	}
	for p.accept("AND") {
		right, err := p.factor(groupDepth)
		if err != nil {
			return parsedNode{}, err
		}
		left, err = p.makeNode(provider.FilterNode{Kind: provider.FilterAnd, Children: []provider.FilterNode{left.node, right.node}}, max(left.depth, right.depth)+1)
		if err != nil {
			return parsedNode{}, err
		}
	}
	return left, nil
}
func (p *parser) factor(groupDepth int) (parsedNode, error) {
	if groupDepth >= maxASTDepth {
		return parsedNode{}, invalid(p.current().offset, "expression exceeds nesting limit")
	}
	if p.accept("NOT") {
		child, err := p.factor(groupDepth + 1)
		if err != nil {
			return parsedNode{}, err
		}
		return p.makeNode(provider.FilterNode{Kind: provider.FilterNot, Children: []provider.FilterNode{child.node}}, child.depth+1)
	}
	t := p.current()
	if t.kind == tokenOpen {
		p.take()
		node, err := p.expression(groupDepth + 1)
		if err != nil {
			return parsedNode{}, err
		}
		if p.current().kind != tokenClose {
			return parsedNode{}, invalid(p.current().offset, "missing closing parenthesis")
		}
		p.take()
		return node, nil
	}
	if p.keyword("TRUE") || p.keyword("FALSE") {
		p.take()
		if p.current().kind == tokenOperator {
			return parsedNode{}, unsupported(t.offset)
		}
		return p.makeNode(provider.FilterNode{Kind: provider.FilterBooleanConstant, Boolean: asciiEqual(t.text, "TRUE")}, 1)
	}
	if t.kind != tokenName && t.kind != tokenQuotedName {
		if t.kind == tokenNumber || t.kind == tokenString || advancedToken(t) {
			return parsedNode{}, unsupported(t.offset)
		}
		return parsedNode{}, invalid(t.offset, "expected predicate")
	}
	if t.kind == tokenName && reserved(t.text) {
		return parsedNode{}, unsupported(t.offset)
	}
	p.take()
	if p.current().kind == tokenOpen {
		return parsedNode{}, unsupported(t.offset)
	}
	property := t.text
	if p.accept("IS") {
		kind := provider.FilterIsNull
		if p.accept("NOT") {
			kind = provider.FilterIsNotNull
		}
		if !p.accept("NULL") {
			return parsedNode{}, invalid(p.current().offset, "expected null test")
		}
		return p.makeNode(provider.FilterNode{Kind: kind, Property: property}, 1)
	}
	opToken := p.current()
	if opToken.kind != tokenOperator {
		if advancedToken(opToken) || p.keyword("NOT") || opToken.kind == tokenNumber {
			return parsedNode{}, unsupported(opToken.offset)
		}
		return parsedNode{}, invalid(opToken.offset, "expected comparison")
	}
	p.take()
	var operator provider.FilterCompareOperator
	switch opToken.text {
	case "=":
		operator = provider.FilterEqual
	case "<>":
		operator = provider.FilterNotEqual
	case "<":
		operator = provider.FilterLess
	case "<=":
		operator = provider.FilterLessEqual
	case ">":
		operator = provider.FilterGreater
	case ">=":
		operator = provider.FilterGreaterEqual
	default:
		return parsedNode{}, invalid(opToken.offset, "invalid comparison operator")
	}
	literal, err := p.literal()
	if err != nil {
		return parsedNode{}, err
	}
	return p.makeNode(provider.FilterNode{Kind: provider.FilterCompare, Operator: operator, Property: property, Literal: literal}, 1)
}
func (p *parser) literal() (provider.FilterLiteral, error) {
	t := p.current()
	switch t.kind {
	case tokenString:
		p.take()
		return provider.NewFilterLiteral(provider.FilterString, t.text)
	case tokenNumber:
		p.take()
		return provider.NewFilterLiteral(provider.FilterNumber, t.text)
	case tokenName:
		if p.keyword("NULL") {
			return provider.FilterLiteral{}, invalid(t.offset, "null requires is null")
		}
		if p.keyword("TRUE") || p.keyword("FALSE") {
			p.take()
			return provider.NewFilterLiteral(provider.FilterBoolean, asciiBoolean(t.text))
		}
		if p.keyword("DATE") || p.keyword("TIMESTAMP") {
			p.take()
			kind := provider.FilterDate
			if asciiEqual(t.text, "TIMESTAMP") {
				kind = provider.FilterTimestamp
			}
			if p.current().kind != tokenOpen {
				return provider.FilterLiteral{}, invalid(p.current().offset, "expected temporal literal parentheses")
			}
			p.take()
			value := p.current()
			if value.kind != tokenString {
				return provider.FilterLiteral{}, invalid(value.offset, "expected temporal literal string")
			}
			p.take()
			if p.current().kind != tokenClose {
				return provider.FilterLiteral{}, invalid(p.current().offset, "expected temporal literal closing parenthesis")
			}
			p.take()
			return provider.NewFilterLiteral(kind, value.text)
		}
		return provider.FilterLiteral{}, unsupported(t.offset)
	case tokenQuotedName, tokenUnsupported, tokenOpen:
		return provider.FilterLiteral{}, unsupported(t.offset)
	default:
		return provider.FilterLiteral{}, invalid(t.offset, "expected scalar literal")
	}
}
func reserved(value string) bool {
	switch asciiUpper(value) {
	case "AND", "OR", "NOT", "IS", "NULL", "TRUE", "FALSE", "DATE", "TIMESTAMP", "LIKE", "IN", "BETWEEN", "CASEI", "ACCENTI", "DIV", "BBOX", "INTERVAL", "POINT", "LINESTRING", "POLYGON", "MULTIPOINT", "MULTILINESTRING", "MULTIPOLYGON", "GEOMETRYCOLLECTION",
		"S_INTERSECTS", "S_EQUALS", "S_DISJOINT", "S_TOUCHES", "S_WITHIN", "S_OVERLAPS", "S_CROSSES", "S_CONTAINS",
		"T_AFTER", "T_BEFORE", "T_CONTAINS", "T_DISJOINT", "T_DURING", "T_EQUALS", "T_FINISHEDBY", "T_FINISHES", "T_INTERSECTS", "T_MEETS", "T_METBY", "T_OVERLAPPEDBY", "T_OVERLAPS", "T_STARTEDBY", "T_STARTS",
		"A_EQUALS", "A_CONTAINS", "A_CONTAINEDBY", "A_OVERLAPS":
		return true
	}
	return false
}
func advancedToken(t token) bool {
	if t.kind == tokenNumber && len(t.text) > 0 && (t.text[0] == '+' || t.text[0] == '-') {
		return true
	}
	if t.kind == tokenUnsupported {
		return true
	}
	if t.kind == tokenName {
		switch asciiUpper(t.text) {
		case "LIKE", "IN", "BETWEEN", "CASEI", "ACCENTI", "DIV", "INTERVAL":
			return true
		}
	}
	return false
}

func asciiUpper(value string) string {
	out := []byte(value)
	for i, b := range out {
		if b >= 'a' && b <= 'z' {
			out[i] = b - ('a' - 'A')
		}
	}
	return string(out)
}
func asciiEqual(left, right string) bool { return asciiUpper(left) == asciiUpper(right) }
func asciiBoolean(value string) string {
	if asciiEqual(value, "TRUE") {
		return "true"
	}
	return "false"
}
