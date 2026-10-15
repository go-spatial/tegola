package wfs

// A26: OGC Filter Encoding (FES 2.0) parser.
//
// Parses the FILTER parameter (KVP) and <Filter> XML element into
// provider.FilterExpression. Supported:
//   - Comparison: PropertyIsEqualTo, PropertyIsNotEqualTo,
//     PropertyIsLessThan, PropertyIsGreaterThan,
//     PropertyIsLessThanOrEqualTo, PropertyIsGreaterThanOrEqualTo,
//     PropertyIsBetween
//   - Logical: And, Or, Not
// Identifier selection is supplied separately through featureId/resourceId KVP.
//
// Unsupported constructs return a descriptive error (not silently ignored).

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"strings"

	"github.com/alexeydott/tegola/provider"
)

// FESFilter retains bounded XML lexical values until a collection's proven
// queryable catalog supplies their types. It cannot be sent to a provider
// directly; Resolve constructs the typed neutral expression.
type FESFilter struct {
	lexical provider.FilterExpression
}

// Root returns a detached lexical snapshot; its string literals have not yet
// been assigned queryable types. Use Resolve before executing the filter.
func (f FESFilter) Root() provider.FilterNode { return f.lexical.Root() }

// Resolve binds each literal to the type of its admitted queryable property.
func (f FESFilter) Resolve(catalog provider.FeatureQueryables) (provider.FilterExpression, error) {
	if err := catalog.Validate(); err != nil {
		return provider.FilterExpression{}, err
	}
	if err := f.lexical.Validate(); err != nil {
		return provider.FilterExpression{}, err
	}
	root := f.lexical.Root()
	var bind func(*provider.FilterNode) error
	bind = func(node *provider.FilterNode) error {
		if node.Kind == provider.FilterCompare {
			field, ok := catalog.Lookup(node.Property)
			if !ok {
				return fmt.Errorf("fes: unknown queryable property %q", node.Property)
			}
			kind := map[provider.QueryableType]provider.FilterScalarType{
				provider.QueryableString:    provider.FilterString,
				provider.QueryableInteger:   provider.FilterNumber,
				provider.QueryableNumber:    provider.FilterNumber,
				provider.QueryableBoolean:   provider.FilterBoolean,
				provider.QueryableDate:      provider.FilterDate,
				provider.QueryableTimestamp: provider.FilterTimestamp,
			}[field.Type]
			text := node.Literal.Text()
			if kind != provider.FilterString {
				text = strings.TrimSpace(text)
			}
			if kind == provider.FilterBoolean {
				switch text {
				case "1":
					text = "true"
				case "0":
					text = "false"
				}
			}
			literal, err := provider.NewFilterLiteral(kind, text)
			if err != nil {
				return fmt.Errorf("fes: literal for property %q: %w", node.Property, err)
			}
			node.Literal = literal
		}
		for i := range node.Children {
			if err := bind(&node.Children[i]); err != nil {
				return err
			}
		}
		return nil
	}
	if err := bind(&root); err != nil {
		return provider.FilterExpression{}, err
	}
	expression, err := provider.NewFilterExpression(root)
	if err != nil {
		return provider.FilterExpression{}, err
	}
	resolved, err := provider.ResolveFeatureFilter(expression, catalog)
	if err != nil {
		return provider.FilterExpression{}, err
	}
	return resolved.Expression(), nil
}

// ParseFESFilter parses an OGC FES 2.0 <Filter> XML fragment. The result must
// be resolved against the selected collection's queryables before execution.
func ParseFESFilter(body []byte) (FESFilter, error) {
	return parseFESFilterContext(body, "", nil)
}

func parseFESFilterContext(body []byte, collection string, bindings map[string]string) (FESFilter, error) {
	if err := validateFESStructure(body); err != nil {
		return FESFilter{}, err
	}
	normalized, err := normalizeFESQNames(body, collection, bindings)
	if err != nil {
		return FESFilter{}, err
	}
	body = normalized
	var doc fesFilter
	if err := xml.Unmarshal(body, &doc); err != nil {
		return FESFilter{}, fmt.Errorf("fes: invalid Filter XML: %w", err)
	}
	node, err := fesNodeToFilter(doc.Inner)
	if err != nil {
		return FESFilter{}, err
	}
	lexical, err := provider.NewFilterExpression(node)
	if err != nil {
		return FESFilter{}, err
	}
	return FESFilter{lexical: lexical}, nil
}

type fesFilter struct {
	XMLName xml.Name
	Inner   fesInner `xml:",any"`
}

type fesInner struct {
	XMLName xml.Name
	Content []byte     `xml:",innerxml"`
	Attrs   []xml.Attr `xml:",attr"`
	// For nested parsing, we re-parse Content.
}

// fesLiteral validates and preserves lexical text without guessing a type.
// FilterString is the storage carrier only; FESFilter.Resolve replaces it
// using the collection's queryable type before provider execution.
func fesLiteral(text string) (provider.FilterLiteral, error) {
	return provider.NewFilterLiteral(provider.FilterString, text)
}

// fesNodeToFilter converts a parsed FES element to a FilterNode.
func fesNodeToFilter(inner fesInner) (provider.FilterNode, error) {
	local := inner.XMLName.Local
	switch local {
	case "And", "Or":
		return fesLogic(local, inner.Content)
	case "Not":
		return fesNot(inner.Content)
	case "PropertyIsEqualTo":
		return fesCompare(provider.FilterEqual, inner.Content)
	case "PropertyIsNotEqualTo":
		return fesCompare(provider.FilterNotEqual, inner.Content)
	case "PropertyIsLessThan":
		return fesCompare(provider.FilterLess, inner.Content)
	case "PropertyIsLessThanOrEqualTo":
		return fesCompare(provider.FilterLessEqual, inner.Content)
	case "PropertyIsGreaterThan":
		return fesCompare(provider.FilterGreater, inner.Content)
	case "PropertyIsGreaterThanOrEqualTo":
		return fesCompare(provider.FilterGreaterEqual, inner.Content)
	case "PropertyIsBetween":
		return fesBetween(inner.Content)
	default:
		return provider.FilterNode{}, fmt.Errorf("fes: unsupported filter operator %q", local)
	}
}

// fesParseChildren parses child filter elements from inner XML.
func fesParseChildren(content []byte) ([]provider.FilterNode, error) {
	// Wrap in a root to parse multiple children.
	wrapped := "<root>" + string(content) + "</root>"
	var root struct {
		Children []fesInner `xml:",any"`
	}
	if err := xml.Unmarshal([]byte(wrapped), &root); err != nil {
		return nil, fmt.Errorf("fes: parse children: %w", err)
	}
	var out []provider.FilterNode
	for _, c := range root.Children {
		// Skip text nodes (XMLName.Local empty for chardata).
		if c.XMLName.Local == "" {
			continue
		}
		n, err := fesNodeToFilter(c)
		if err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, nil
}

func fesLogic(op string, content []byte) (provider.FilterNode, error) {
	children, err := fesParseChildren(content)
	if err != nil {
		return provider.FilterNode{}, err
	}
	if len(children) < 2 {
		return provider.FilterNode{}, fmt.Errorf("fes: %s requires at least 2 operands, got %d", op, len(children))
	}
	kind := provider.FilterAnd
	if op == "Or" {
		kind = provider.FilterOr
	}
	return provider.FilterNode{Kind: kind, Children: children}, nil
}

func fesNot(content []byte) (provider.FilterNode, error) {
	children, err := fesParseChildren(content)
	if err != nil {
		return provider.FilterNode{}, err
	}
	if len(children) != 1 {
		return provider.FilterNode{}, fmt.Errorf("fes: Not requires exactly 1 operand, got %d", len(children))
	}
	return provider.FilterNode{Kind: provider.FilterNot, Children: children}, nil
}

// fesValueRef extracts <ValueReference> and <Literal> from comparison content.
func fesValueRef(content []byte) (prop, lit string, err error) {
	wrapped := "<root>" + string(content) + "</root>"
	var root struct {
		ValueRef     string `xml:"ValueReference"`
		PropertyName string `xml:"PropertyName"`
		Literal      string `xml:"Literal"`
	}
	if err := xml.Unmarshal([]byte(wrapped), &root); err != nil {
		return "", "", fmt.Errorf("fes: parse comparison: %w", err)
	}
	if root.ValueRef == "" {
		root.ValueRef = root.PropertyName
	}
	prop = strings.TrimSpace(root.ValueRef)
	lit = root.Literal
	if prop == "" {
		return "", "", fmt.Errorf("fes: comparison missing ValueReference")
	}
	return prop, lit, nil
}

func fesCompare(op provider.FilterCompareOperator, content []byte) (provider.FilterNode, error) {
	prop, lit, err := fesValueRef(content)
	if err != nil {
		return provider.FilterNode{}, err
	}
	literal, err := fesLiteral(lit)
	if err != nil {
		return provider.FilterNode{}, err
	}
	return provider.FilterNode{
		Kind:     provider.FilterCompare,
		Operator: op,
		Property: prop,
		Literal:  literal,
	}, nil
}

func fesBetween(content []byte) (provider.FilterNode, error) {
	wrapped := "<root>" + string(content) + "</root>"
	var root struct {
		ValueRef     string `xml:"ValueReference"`
		PropertyName string `xml:"PropertyName"`
		Lower        string `xml:"LowerBoundary>Literal"`
		Upper        string `xml:"UpperBoundary>Literal"`
	}
	if err := xml.Unmarshal([]byte(wrapped), &root); err != nil {
		return provider.FilterNode{}, fmt.Errorf("fes: parse Between: %w", err)
	}
	if root.ValueRef == "" {
		root.ValueRef = root.PropertyName
	}
	prop := strings.TrimSpace(root.ValueRef)
	if prop == "" {
		return provider.FilterNode{}, fmt.Errorf("fes: PropertyIsBetween missing ValueReference")
	}
	lower, err := fesLiteral(root.Lower)
	if err != nil {
		return provider.FilterNode{}, err
	}
	upper, err := fesLiteral(root.Upper)
	if err != nil {
		return provider.FilterNode{}, err
	}
	// Between(a, lo, hi) = (a >= lo) AND (a <= hi)
	lo := provider.FilterNode{
		Kind: provider.FilterCompare, Operator: provider.FilterGreaterEqual,
		Property: prop, Literal: lower,
	}
	hi := provider.FilterNode{
		Kind: provider.FilterCompare, Operator: provider.FilterLessEqual,
		Property: prop, Literal: upper,
	}
	return provider.FilterNode{Kind: provider.FilterAnd, Children: []provider.FilterNode{lo, hi}}, nil
}

// Validate the complete document before the narrow expression decoder runs.
// encoding/xml otherwise silently ignores unknown attributes and extra children.
type fesElement struct {
	XMLName  xml.Name
	Attrs    []xml.Attr   `xml:",any,attr"`
	Text     string       `xml:",chardata"`
	Children []fesElement `xml:",any"`
}

func validateFESStructure(body []byte) error {
	if len(body) > provider.MaxFilterBytes {
		return fmt.Errorf("fes: filter exceeds byte limit")
	}
	d := xml.NewDecoder(bytes.NewReader(body))
	var root fesElement
	if err := d.Decode(&root); err != nil {
		return fmt.Errorf("fes: invalid XML: %w", err)
	}
	for {
		tok, err := d.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if text, ok := tok.(xml.CharData); !ok || strings.TrimSpace(string(text)) != "" {
			return fmt.Errorf("fes: trailing XML content")
		}
	}
	if root.XMLName.Local != "Filter" || len(root.Children) != 1 {
		return fmt.Errorf("fes: Filter requires exactly one predicate")
	}
	nodes := 0
	var check func(fesElement, int) error
	check = func(n fesElement, depth int) error {
		nodes++
		if depth > provider.MaxFilterDepth || nodes > provider.MaxFilterNodes {
			return fmt.Errorf("fes: filter complexity limit exceeded")
		}
		ns := n.XMLName.Space
		if ns != "" && ns != "http://www.opengis.net/fes/2.0" && ns != "http://www.opengis.net/ogc" {
			return fmt.Errorf("fes: unsupported namespace %q", ns)
		}
		for _, a := range n.Attrs {
			if a.Name.Space != "xmlns" && a.Name.Local != "xmlns" {
				return fmt.Errorf("fes: unsupported attribute %q", a.Name.Local)
			}
		}
		name := n.XMLName.Local
		names := func(want ...string) bool {
			if len(n.Children) != len(want) {
				return false
			}
			for i, c := range n.Children {
				if c.XMLName.Local != want[i] {
					return false
				}
			}
			return true
		}
		valid := false
		switch name {
		case "Filter":
			valid = depth == 0 && len(n.Children) == 1
		case "And", "Or":
			valid = len(n.Children) >= 2
		case "Not":
			valid = len(n.Children) == 1
		case "PropertyIsEqualTo", "PropertyIsNotEqualTo", "PropertyIsLessThan", "PropertyIsLessThanOrEqualTo", "PropertyIsGreaterThan", "PropertyIsGreaterThanOrEqualTo":
			valid = names("ValueReference", "Literal") || names("PropertyName", "Literal")
		case "PropertyIsBetween":
			valid = names("ValueReference", "LowerBoundary", "UpperBoundary") || names("PropertyName", "LowerBoundary", "UpperBoundary")
		case "LowerBoundary", "UpperBoundary":
			valid = names("Literal")
		case "ValueReference", "PropertyName":
			valid = len(n.Children) == 0 && strings.TrimSpace(n.Text) != ""
		case "Literal":
			valid = len(n.Children) == 0
			if _, err := fesLiteral(n.Text); err != nil {
				return err
			}
		default:
			return fmt.Errorf("fes: unsupported filter operator %q", name)
		}
		if !valid {
			return fmt.Errorf("fes: invalid operands for %s", name)
		}
		if len(n.Children) > 0 && strings.TrimSpace(n.Text) != "" {
			return fmt.Errorf("fes: unexpected text in %s", name)
		}
		for _, c := range n.Children {
			// Operand elements cannot act as predicates in a logical expression.
			if name == "Filter" || name == "And" || name == "Or" || name == "Not" {
				switch c.XMLName.Local {
				case "Literal", "ValueReference", "PropertyName", "LowerBoundary", "UpperBoundary":
					return fmt.Errorf("fes: expected predicate")
				}
			}
			if err := check(c, depth+1); err != nil {
				return err
			}
		}
		return nil
	}
	return check(root, 0)
}
