package provider

import (
	"errors"
	"math/big"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

// Limits apply before copying caller-owned filter or catalog inputs.
const (
	MaxFilterBytes                   = 65536
	MaxFilterNodes                   = 4096
	MaxFilterDepth                   = 32
	MaxFilterPropertyBytes           = 1024
	MaxFilterLiteralBytes            = 16384
	MaxFilterNumericDigits           = 1024
	MaxFilterNumericExponent         = 4096
	MaxFilterTimestampFractionDigits = 1024
	MaxQueryableFields               = 4096
	MaxQueryableNameBytes            = 1024
	MaxQueryableTotalNameBytes       = 65536
)

type FilterScalarType uint8

const (
	FilterScalarInvalid FilterScalarType = iota
	FilterString
	FilterNumber
	FilterBoolean
	FilterDate
	FilterTimestamp
)

// FilterInstant preserves exact UTC timestamp precision and inserted seconds.
type FilterInstant struct {
	Time          time.Time
	SubNanosecond string
	LeapSecond    bool
}

// FilterLiteral contains a validated logical value, never a SQL fragment.
type FilterLiteral struct {
	kind    FilterScalarType
	text    string
	date    time.Time
	instant FilterInstant
}

var filterNumberPattern = regexp.MustCompile(`^[+-]?(?:[0-9]+(?:\.[0-9]*)?|\.[0-9]+)(?:[eE][+-]?[0-9]+)?$`)
var filterTimestampPattern = regexp.MustCompile(
	`^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}(?:\.([0-9]+))?Z$`,
)

func invalidFilter(reason string) error {
	return InvalidFeatureQueryError{Field: "filter", Reason: reason}
}

func unsupportedFilter() error {
	return errors.Join(invalidFilter("operator is outside the supported filter profile"), ErrUnsupported)
}

// NewFilterLiteral validates decoded logical text. String text has already had
// CQL quoting and escapes removed. No implicit conversion between types occurs.
func NewFilterLiteral(kind FilterScalarType, text string) (FilterLiteral, error) {
	if len(text) > MaxFilterLiteralBytes || !utf8.ValidString(text) {
		return FilterLiteral{}, invalidFilter("literal exceeds its byte limit or is not UTF-8")
	}
	literal := FilterLiteral{kind: kind, text: text}
	switch kind {
	case FilterString:
	case FilterNumber:
		if !filterNumberPattern.MatchString(text) {
			return FilterLiteral{}, invalidFilter("invalid numeric literal")
		}
		digits := 0
		for _, c := range text {
			if c >= '0' && c <= '9' {
				digits++
			}
		}
		if digits > MaxFilterNumericDigits {
			return FilterLiteral{}, invalidFilter("numeric literal exceeds its digit limit")
		}
		if index := strings.IndexAny(text, "eE"); index >= 0 {
			exponent, err := strconv.ParseInt(text[index+1:], 10, 64)
			if err != nil || exponent < -MaxFilterNumericExponent || exponent > MaxFilterNumericExponent {
				return FilterLiteral{}, invalidFilter("numeric exponent exceeds its limit")
			}
		}
		if _, ok := new(big.Rat).SetString(text); !ok {
			return FilterLiteral{}, invalidFilter("invalid numeric literal")
		}
	case FilterBoolean:
		if text != "true" && text != "false" {
			return FilterLiteral{}, invalidFilter("invalid boolean literal")
		}
	case FilterDate:
		if len(text) != 10 {
			return FilterLiteral{}, invalidFilter("date must be YYYY-MM-DD")
		}
		date, err := time.Parse("2006-01-02", text)
		if err != nil || date.Format("2006-01-02") != text {
			return FilterLiteral{}, invalidFilter("invalid calendar date")
		}
		literal.date = date
	case FilterTimestamp:
		instant, err := parseFilterInstant(text)
		if err != nil {
			return FilterLiteral{}, err
		}
		literal.instant = instant
	default:
		return FilterLiteral{}, invalidFilter("invalid scalar literal type")
	}
	return literal, nil
}

func parseFilterInstant(text string) (FilterInstant, error) {
	match := filterTimestampPattern.FindStringSubmatch(text)
	if match == nil {
		return FilterInstant{}, invalidFilter("timestamp must use UTC T/Z syntax")
	}
	fraction := match[1]
	if len(fraction) > MaxFilterTimestampFractionDigits {
		return FilterInstant{}, invalidFilter("timestamp fraction exceeds its limit")
	}
	ordinary := text[:19]
	leap := ordinary[17:19] == "60"
	if leap {
		ordinary = ordinary[:17] + "59"
	}
	tail := ""
	if len(fraction) > 9 {
		tail, fraction = fraction[9:], fraction[:9]
	}
	if fraction != "" {
		ordinary += "." + fraction
	}
	t, err := time.Parse(time.RFC3339Nano, ordinary+"Z")
	if err != nil {
		return FilterInstant{}, invalidFilter("invalid timestamp")
	}
	if err := validateTemporalEndpoint(&t, tail, leap); err != nil {
		return FilterInstant{}, invalidFilter("timestamp leap second is not an announced insertion")
	}
	return FilterInstant{Time: t, SubNanosecond: tail, LeapSecond: leap}, nil
}

func (l FilterLiteral) Type() FilterScalarType { return l.kind }
func (l FilterLiteral) Text() string           { return l.text }

// Number returns a new exact value. Modifying it cannot modify the literal.
func (l FilterLiteral) Number() (*big.Rat, bool) {
	if l.kind != FilterNumber {
		return nil, false
	}
	return new(big.Rat).SetString(l.text)
}
func (l FilterLiteral) Date() (time.Time, bool)        { return l.date, l.kind == FilterDate }
func (l FilterLiteral) Instant() (FilterInstant, bool) { return l.instant, l.kind == FilterTimestamp }

type FilterKind uint8

const (
	FilterInvalid FilterKind = iota
	FilterBooleanConstant
	FilterCompare
	FilterIsNull
	FilterIsNotNull
	FilterAnd
	FilterOr
	FilterNot
	// Recognized advanced kinds have no supported payload in this profile.
	FilterLike
	FilterIn
	FilterBetween
	FilterSpatial
	FilterTemporal
)

type FilterCompareOperator uint8

const (
	FilterComparisonInvalid FilterCompareOperator = iota
	FilterEqual
	FilterNotEqual
	FilterLess
	FilterLessEqual
	FilterGreater
	FilterGreaterEqual
)

// FilterNode is a construction descriptor. NewFilterExpression validates its
// entire resource budget before making an immutable recursive snapshot.
type FilterNode struct {
	Kind     FilterKind
	Operator FilterCompareOperator
	Property string
	Literal  FilterLiteral
	Boolean  bool
	Children []FilterNode
}

type FilterExpression struct {
	root  FilterNode
	valid bool
}

type filterBudget struct{ nodes, bytes int }

func validateFilterNode(node FilterNode, depth int, budget *filterBudget) error {
	if depth > MaxFilterDepth || budget.nodes >= MaxFilterNodes {
		return invalidFilter("filter exceeds its depth or node limit")
	}
	budget.nodes++
	if len(node.Property) > MaxFilterPropertyBytes || len(node.Property) > MaxFilterBytes-budget.bytes {
		return invalidFilter("filter property exceeds its byte limit")
	}
	budget.bytes += len(node.Property)
	if len(node.Literal.text) > MaxFilterBytes-budget.bytes {
		return invalidFilter("filter exceeds its byte limit")
	}
	budget.bytes += len(node.Literal.text)
	leafEmpty := node.Property == "" && node.Operator == FilterComparisonInvalid &&
		node.Literal == (FilterLiteral{}) && !node.Boolean
	switch node.Kind {
	case FilterBooleanConstant:
		if node.Property != "" || node.Operator != FilterComparisonInvalid ||
			node.Literal != (FilterLiteral{}) || len(node.Children) != 0 {
			return invalidFilter("invalid boolean node shape")
		}
	case FilterCompare:
		if !validFilterProperty(node.Property) || node.Operator < FilterEqual ||
			node.Operator > FilterGreaterEqual || node.Boolean || len(node.Children) != 0 {
			return invalidFilter("invalid comparison node shape")
		}
		if _, err := NewFilterLiteral(node.Literal.kind, node.Literal.text); err != nil {
			return err
		}
	case FilterIsNull, FilterIsNotNull:
		if !validFilterProperty(node.Property) || node.Operator != FilterComparisonInvalid ||
			node.Literal != (FilterLiteral{}) || node.Boolean || len(node.Children) != 0 {
			return invalidFilter("invalid null node shape")
		}
	case FilterAnd, FilterOr, FilterNot:
		if !leafEmpty || (node.Kind == FilterNot && len(node.Children) != 1) ||
			(node.Kind != FilterNot && len(node.Children) < 2) {
			return invalidFilter("invalid logical node shape")
		}
		for _, child := range node.Children {
			if err := validateFilterNode(child, depth+1, budget); err != nil {
				return err
			}
		}
	case FilterLike, FilterIn, FilterBetween, FilterSpatial, FilterTemporal:
		return unsupportedFilter()
	default:
		return invalidFilter("invalid filter node kind")
	}
	return nil
}

func validFilterProperty(name string) bool {
	return strings.TrimSpace(name) != "" && utf8.ValidString(name)
}

func cloneFilterNode(node FilterNode) FilterNode {
	if node.Children != nil {
		children := make([]FilterNode, len(node.Children))
		for i, child := range node.Children {
			children[i] = cloneFilterNode(child)
		}
		node.Children = children
	}
	return node
}

func NewFilterExpression(root FilterNode) (FilterExpression, error) {
	if err := validateFilterNode(root, 1, &filterBudget{}); err != nil {
		return FilterExpression{}, err
	}
	return FilterExpression{root: cloneFilterNode(root), valid: true}, nil
}
func (f FilterExpression) Validate() error {
	if !f.valid {
		return invalidFilter("uninitialized filter expression")
	}
	return validateFilterNode(f.root, 1, &filterBudget{})
}
func (f FilterExpression) Root() FilterNode { return cloneFilterNode(f.root) }
func (f FilterExpression) Clone() FilterExpression {
	return FilterExpression{root: cloneFilterNode(f.root), valid: f.valid}
}

type QueryableType uint8

const (
	QueryableInvalid QueryableType = iota
	QueryableString
	QueryableInteger
	QueryableNumber
	QueryableBoolean
	QueryableDate
	QueryableTimestamp
)

// FeatureQueryable describes an admitted public raw-feature alias. Providers
// must prove all Basic comparison and NULL operations for its physical lineage.
type FeatureQueryable struct {
	Name     string
	Type     QueryableType
	Nullable bool
}
type FeatureQueryables struct {
	fields []FeatureQueryable
	valid  bool
}

func validateQueryableBounds(fields []FeatureQueryable) error {
	if len(fields) > MaxQueryableFields {
		return invalidFilter("queryables exceed the field limit")
	}
	total := 0
	for _, field := range fields {
		if len(field.Name) > MaxQueryableNameBytes || len(field.Name) > MaxQueryableTotalNameBytes-total {
			return invalidFilter("queryables exceed the name byte limit")
		}
		total += len(field.Name)
	}
	return nil
}
func validateQueryableFields(fields []FeatureQueryable) error {
	if err := validateQueryableBounds(fields); err != nil {
		return err
	}
	seen := make(map[string]struct{}, len(fields))
	for _, field := range fields {
		if !validFilterProperty(field.Name) || field.Type < QueryableString || field.Type > QueryableTimestamp {
			return invalidFilter("invalid queryable metadata")
		}
		if _, exists := seen[field.Name]; exists {
			return invalidFilter("duplicate queryable name")
		}
		seen[field.Name] = struct{}{}
	}
	return nil
}
func NewFeatureQueryables(fields []FeatureQueryable) (FeatureQueryables, error) {
	if err := validateQueryableFields(fields); err != nil {
		return FeatureQueryables{}, err
	}
	owned := slices.Clone(fields)
	slices.SortFunc(owned, func(a, b FeatureQueryable) int { return strings.Compare(a.Name, b.Name) })
	return FeatureQueryables{fields: owned, valid: true}, nil
}
func (q FeatureQueryables) Validate() error {
	if !q.valid {
		return invalidFilter("uninitialized queryables catalog")
	}
	return validateQueryableFields(q.fields)
}
func (q FeatureQueryables) Fields() []FeatureQueryable { return slices.Clone(q.fields) }
func (q FeatureQueryables) Lookup(name string) (FeatureQueryable, bool) {
	for _, field := range q.fields {
		if field.Name == name {
			return field, true
		}
	}
	return FeatureQueryable{}, false
}

// FeatureQueryableLayerInfo is optional and returns detached, registration-proven
// metadata. Missing or unproved capability does not imply filtering support.
type FeatureQueryableLayerInfo interface {
	FeatureQueryables() (FeatureQueryables, error)
}
type ResolvedFeatureFilter struct {
	expression FilterExpression
	queryables FeatureQueryables
}

func ResolveFeatureFilter(expression FilterExpression, queryables FeatureQueryables) (ResolvedFeatureFilter, error) {
	if err := queryables.Validate(); err != nil {
		return ResolvedFeatureFilter{}, err
	}
	if err := expression.Validate(); err != nil {
		return ResolvedFeatureFilter{}, err
	}
	if err := resolveFilterNode(expression.root, queryables); err != nil {
		return ResolvedFeatureFilter{}, err
	}
	catalog, err := NewFeatureQueryables(queryables.fields)
	if err != nil {
		return ResolvedFeatureFilter{}, err
	}
	return ResolvedFeatureFilter{expression: expression.Clone(), queryables: catalog}, nil
}
func resolveFilterNode(node FilterNode, catalog FeatureQueryables) error {
	if node.Kind == FilterCompare || node.Kind == FilterIsNull || node.Kind == FilterIsNotNull {
		field, exists := catalog.Lookup(node.Property)
		if !exists {
			return invalidFilter("unknown queryable property")
		}
		if node.Kind == FilterCompare {
			compatible := false
			switch field.Type {
			case QueryableString:
				compatible = node.Literal.kind == FilterString
			case QueryableInteger, QueryableNumber:
				compatible = node.Literal.kind == FilterNumber
			case QueryableBoolean:
				compatible = node.Literal.kind == FilterBoolean
			case QueryableDate:
				compatible = node.Literal.kind == FilterDate
			case QueryableTimestamp:
				compatible = node.Literal.kind == FilterTimestamp
			}
			if !compatible {
				return invalidFilter("comparison literal type does not match queryable")
			}
		}
	}
	for _, child := range node.Children {
		if err := resolveFilterNode(child, catalog); err != nil {
			return err
		}
	}
	return nil
}
func (f ResolvedFeatureFilter) Expression() FilterExpression { return f.expression.Clone() }
func (f ResolvedFeatureFilter) Queryables() FeatureQueryables {
	return FeatureQueryables{fields: f.queryables.Fields(), valid: f.queryables.valid}
}
