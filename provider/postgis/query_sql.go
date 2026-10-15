package postgis

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/alexeydott/tegola/provider/internal/featuresql"
)

type featureSQLCatalog struct {
	ctx     context.Context
	db      featureCatalogReader
	profile *featureProfile
	unique  map[string]bool
}

func featureIdentifier(id featuresql.Identifier) string {
	if id.Quoted() {
		return id.Name()
	}
	return featureFoldIdentifier(id.Name())
}
func featurePlanRelation(name featuresql.Name) (string, string, error) {
	parts := name.Parts()
	switch len(parts) {
	case 1:
		return "public", featureIdentifier(parts[0]), nil
	case 2:
		return featureIdentifier(parts[0]), featureIdentifier(parts[1]), nil
	default:
		return "", "", featureUnsupported("invalid physical relation")
	}
}
func (r *featureSQLCatalog) ResolveRelation(name featuresql.Name) (featuresql.RelationMetadata, error) {
	schema, table, err := featurePlanRelation(name)
	if err != nil {
		return featuresql.RelationMetadata{}, err
	}
	if (len(name.Parts()) == 2 && schema != r.profile.schema) || table != r.profile.table {
		return featuresql.RelationMetadata{}, featureInvalid("feature_sql", "relation identity does not match catalog admission")
	}
	return featuresql.RelationMetadata{Schema: r.profile.schema, Name: table, CatalogIdentity: strconv.FormatUint(uint64(r.profile.oid), 10), PhysicalTable: true, Deterministic: true}, nil
}
func (r *featureSQLCatalog) OutputKey(id featuresql.Identifier) (string, error) {
	name := featureIdentifier(id)
	limit := r.profile.identifierLimit
	if limit == 0 {
		limit = 63
	}
	if len(name) > limit {
		return "", featureInvalid("feature_sql", "identifier exceeds server identifier limit")
	}
	return name, nil
}
func (r *featureSQLCatalog) QualifierKey(id featuresql.Identifier) (string, error) {
	return r.OutputKey(id)
}
func (r *featureSQLCatalog) ResolveColumn(relation featuresql.RelationMetadata, id featuresql.Identifier) (featuresql.ColumnMetadata, error) {
	if relation.CatalogIdentity != strconv.FormatUint(uint64(r.profile.oid), 10) {
		return featuresql.ColumnMetadata{}, featureInvalid("feature_sql", "catalog identity does not match")
	}
	name, err := r.OutputKey(id)
	if err != nil {
		return featuresql.ColumnMetadata{}, err
	}
	var column featureColumn
	found := false
	for _, c := range r.profile.columns {
		if c.name == name {
			column, found = c, true
			break
		}
	}
	if !found {
		return featuresql.ColumnMetadata{}, featureInvalid("feature_sql", "source column does not exist")
	}
	meta := featuresql.ColumnMetadata{Name: name, Nullable: !column.notNull, Generated: column.generated != ""}
	switch column.oid {
	case 20:
		meta.Kind, meta.Bits = featuresql.IntegerColumn, 64
	case 21:
		meta.Kind, meta.Bits = featuresql.IntegerColumn, 16
	case 23:
		meta.Kind, meta.Bits = featuresql.IntegerColumn, 32
	case 700:
		meta.Kind, meta.Bits = featuresql.FloatColumn, 32
	case 701:
		meta.Kind, meta.Bits = featuresql.FloatColumn, 64
	case 25, 1042, 1043:
		meta.Kind = featuresql.StringColumn
	case 16:
		meta.Kind = featuresql.BooleanColumn
	case 17:
		meta.Kind = featuresql.BinaryColumn
	default:
		if column.typeName == "geometry" && column.extension == "postgis" {
			meta.Kind = featuresql.NativeGeometryColumn
		}
	}
	if meta.Kind == featuresql.IntegerColumn && column.typeSchema != "pg_catalog" {
		meta.Kind = featuresql.OtherColumn
	}
	if meta.Kind == featuresql.IntegerColumn {
		unique, known := r.unique[name]
		if !known {
			if err := r.db.QueryRow(r.ctx, `SELECT EXISTS(SELECT 1 FROM pg_catalog.pg_index i WHERE i.indrelid=$1 AND i.indisunique AND i.indisvalid AND i.indisready AND i.indislive AND i.indimmediate AND i.indnkeyatts=1 AND i.indkey[0]=$2 AND i.indpred IS NULL AND i.indexprs IS NULL)`, r.profile.oid, column.number).Scan(&unique); err != nil {
				return meta, err
			}
			r.unique[name] = unique
		}
		meta.SingleColumnUnique = unique
	}
	if column.collation != 0 {
		var schema, name string
		if err := r.db.QueryRow(r.ctx, `SELECT n.nspname,c.collname FROM pg_catalog.pg_collation c JOIN pg_catalog.pg_namespace n ON n.oid=c.collnamespace WHERE c.oid=$1`, column.collation).Scan(&schema, &name); err != nil {
			return meta, err
		}
		meta.Collation = schema + "." + name
	}
	return meta, nil
}

func (p *Provider) resolveFeatureSelection(ctx context.Context, f *featureProfile, plan *featuresql.Plan) error {
	return resolvePostGISFeatureSelection(ctx, p.pool, f, plan)
}

func resolvePostGISFeatureSelection(ctx context.Context, db featureCatalogReader, f *featureProfile, plan *featuresql.Plan) error {
	catalog := &featureSQLCatalog{ctx: ctx, db: db, profile: f, unique: map[string]bool{}}
	var temporal []string
	for _, field := range []string{f.temporal.InstantField, f.temporal.StartField, f.temporal.EndField} {
		if field != "" {
			temporal = append(temporal, field)
		}
	}
	resolved, err := featuresql.Resolve(plan, catalog, featuresql.ResolveOptions{IdentityOutput: f.id, GeometryOutput: f.geometry, TemporalOutputs: temporal})
	if err != nil {
		return err
	}
	for _, projection := range resolved.Projections() {
		var physical featureColumn
		for _, column := range f.columns {
			if column.name == projection.Column.Name {
				physical = column
				break
			}
		}
		f.projections = append(f.projections, featureProjection{output: projection.Output, column: physical})
	}
	predicate, args, _, err := featuresql.CompileWhere(resolved, featuresql.RenderOptions{QuoteIdentifier: pgQuoteIdent, Placeholder: func(index int) string { return fmt.Sprintf("$%d", index) }, BindLiteral: bindFeatureLiteral, FirstParameter: 1, Qualifier: "l"})
	if err != nil {
		return err
	}
	f.where, f.whereArgs = predicate, append([]any(nil), args...)
	return nil
}

func bindFeatureLiteral(column featuresql.ColumnMetadata, operator string, literal featuresql.Literal) (any, error) {
	switch column.Kind {
	case featuresql.IntegerColumn:
		if literal.Kind() != featuresql.NumberLiteral {
			return nil, featureUnsupported("integer predicate requires numeric literals")
		}
		value, err := featureLiteralInteger(literal.Text())
		if err != nil {
			return nil, err
		}
		if column.Bits < 64 {
			bound := int64(1) << (column.Bits - 1)
			if value < -bound || value >= bound {
				return nil, featureUnsupported("integer predicate literal exceeds source width")
			}
		}
		return value, nil
	case featuresql.FloatColumn:
		return nil, featureUnsupported("floating predicate comparisons lack an exact literal profile")
	case featuresql.BooleanColumn:
		if literal.Kind() != featuresql.BooleanLiteral {
			return nil, featureUnsupported("boolean predicates require boolean literals")
		}
		return literal.Text() == "true", nil
	case featuresql.StringColumn:
		if literal.Kind() != featuresql.StringLiteral || (column.Collation != "pg_catalog.C" && column.Collation != "pg_catalog.POSIX") {
			return nil, featureUnsupported("text predicate requires a proven bytewise collation and string literal")
		}
		return literal.Text(), nil
	default:
		return nil, featureUnsupported("predicate source type is not admitted")
	}
}

// Bound exponent handling before allocating powers of ten. SQL numeric tokens
// retain exact decimal meaning, including integral fractional/exponent forms.
func featureLiteralInteger(raw string) (int64, error) {
	sign := ""
	if strings.HasPrefix(raw, "-") {
		sign = "-"
		raw = raw[1:]
	} else if strings.HasPrefix(raw, "+") {
		raw = raw[1:]
	}
	exponent := int64(0)
	if index := strings.IndexAny(raw, "eE"); index >= 0 {
		value, err := strconv.ParseInt(raw[index+1:], 10, 64)
		coefficient := raw[:index]
		if err != nil {
			if strings.Trim(coefficient, "0.") == "" {
				return 0, nil
			}
			return 0, featureUnsupported("integer literal exponent exceeds storage profile")
		}
		exponent = value
		raw = coefficient
	}
	if index := strings.IndexByte(raw, '.'); index >= 0 {
		fraction := int64(len(raw) - index - 1)
		if exponent < math.MinInt64+fraction {
			return 0, featureUnsupported("integer literal exponent exceeds storage profile")
		}
		exponent -= fraction
		raw = raw[:index] + raw[index+1:]
	}
	raw = strings.TrimLeft(raw, "0")
	if raw == "" {
		return 0, nil
	}
	trimmed := strings.TrimRight(raw, "0")
	trailing := int64(len(raw) - len(trimmed))
	if exponent > math.MaxInt64-trailing {
		return 0, featureUnsupported("integer literal exponent exceeds storage profile")
	}
	exponent += trailing
	raw = trimmed
	if exponent < 0 || exponent > 19 || int64(len(raw))+exponent > 19 {
		return 0, featureUnsupported("integer predicate literal is not an exact stored integer")
	}
	value, err := strconv.ParseInt(sign+raw+strings.Repeat("0", int(exponent)), 10, 64)
	if err != nil {
		return 0, featureUnsupported("integer predicate literal exceeds storage width")
	}
	return value, nil
}

func featureFoldIdentifier(name string) string {
	return strings.Map(func(character rune) rune {
		if character >= 'A' && character <= 'Z' {
			return character + ('a' - 'A')
		}
		return character
	}, name)
}
