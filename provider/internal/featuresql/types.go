// Package featuresql admits a bounded, catalog-proven raw feature selection.
// It never executes configured SQL or owns database connections.
package featuresql

import "errors"

var (
	ErrInvalid     = errors.New("invalid feature sql")
	ErrUnsupported = errors.New("unsupported feature sql profile")
)

type Dialect uint8

const (
	SQLite Dialect = iota
	MySQL
	PostgreSQL
	HANA
)

type Identifier struct {
	name   string
	quoted bool
}

func (i Identifier) Name() string { return i.name }
func (i Identifier) Quoted() bool { return i.quoted }

type Name struct{ parts []Identifier }

func (n Name) Parts() []Identifier { return append([]Identifier(nil), n.parts...) }

type Projection struct {
	source, qualifier, output Identifier
	qualified                 bool
}

func (p Projection) Source() Identifier            { return p.source }
func (p Projection) Qualifier() (Identifier, bool) { return p.qualifier, p.qualified }
func (p Projection) Output() Identifier            { return p.output }

type LiteralKind uint8

const (
	StringLiteral LiteralKind = iota
	NumberLiteral
	BooleanLiteral
)

type Literal struct {
	kind LiteralKind
	text string
}

func (l Literal) Kind() LiteralKind { return l.kind }
func (l Literal) Text() string      { return l.text }

type PredicateKind uint8

const (
	Compare PredicateKind = iota
	IsNull
	In
	And
	Or
	Not
)

type Predicate struct {
	kind              PredicateKind
	column, qualifier Identifier
	qualified         bool
	operator          string
	negated           bool
	literals          []Literal
	children          []Predicate
}

func (p Predicate) Kind() PredicateKind                    { return p.kind }
func (p Predicate) Column() (Identifier, Identifier, bool) { return p.qualifier, p.column, p.qualified }
func (p Predicate) Operator() string                       { return p.operator }
func (p Predicate) Negated() bool                          { return p.negated }
func (p Predicate) Literals() []Literal                    { return append([]Literal(nil), p.literals...) }
func (p Predicate) Children() []Predicate {
	out := make([]Predicate, len(p.children))
	for i, c := range p.children {
		out[i] = clonePredicate(c)
	}
	return out
}
func clonePredicate(p Predicate) Predicate {
	p.literals = p.Literals()
	p.children = p.Children()
	return p
}

type Plan struct {
	relation     Name
	alias        Identifier
	hasAlias     bool
	projections  []Projection
	predicate    Predicate
	hasPredicate bool
}

func (p *Plan) Relation() Name                 { return Name{p.relation.Parts()} }
func (p *Plan) TableAlias() (Identifier, bool) { return p.alias, p.hasAlias }
func (p *Plan) Projections() []Projection      { return append([]Projection(nil), p.projections...) }
func (p *Plan) Predicate() (Predicate, bool)   { return clonePredicate(p.predicate), p.hasPredicate }

type ColumnKind uint8

const (
	OtherColumn ColumnKind = iota
	IntegerColumn
	DecimalColumn
	FloatColumn
	StringColumn
	BooleanColumn
	BinaryColumn
	NativeGeometryColumn
)

type ColumnMetadata struct {
	Name                                    string
	Kind                                    ColumnKind
	Bits                                    int
	Unsigned                                bool
	Precision, Scale                        int
	Nullable, SingleColumnUnique, Generated bool
	Collation                               string
}
type RelationMetadata struct {
	Schema, Name, CatalogIdentity string
	PhysicalTable, Deterministic  bool
}
type CatalogResolver interface {
	ResolveRelation(Name) (RelationMetadata, error)
	ResolveColumn(RelationMetadata, Identifier) (ColumnMetadata, error)
	OutputKey(Identifier) (string, error)
	QualifierKey(Identifier) (string, error)
}
type ResolveOptions struct {
	IdentityOutput, GeometryOutput string
	TemporalOutputs                []string
}
type ResolvedProjection struct {
	Output string
	Column ColumnMetadata
}
type resolvedPredicate struct {
	syntax   Predicate
	column   ColumnMetadata
	children []resolvedPredicate
}
type ResolvedPlan struct {
	relation           RelationMetadata
	projections        []ResolvedProjection
	identity, geometry ResolvedProjection
	predicate          resolvedPredicate
	hasPredicate       bool
}

func (p *ResolvedPlan) Relation() RelationMetadata { return p.relation }
func (p *ResolvedPlan) Projections() []ResolvedProjection {
	return append([]ResolvedProjection(nil), p.projections...)
}
func (p *ResolvedPlan) Identity() ResolvedProjection { return p.identity }
func (p *ResolvedPlan) Geometry() ResolvedProjection { return p.geometry }

type RenderOptions struct {
	QuoteIdentifier func(string) string
	Placeholder     func(int) string
	BindLiteral     func(ColumnMetadata, string, Literal) (any, error)
	// ParameterExpression is a trusted, stateless provider cast around one
	// placeholder. BindLiteral must prove exact representability before casting.
	ParameterExpression func(ColumnMetadata, string, string) (string, error)
	FirstParameter      int
	Qualifier           string
}
