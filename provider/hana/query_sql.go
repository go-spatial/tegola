package hana

import (
	"errors"
	"fmt"
	"math/big"
	"strconv"
	"strings"

	"github.com/alexeydott/tegola/provider/internal/featuresql"
)

type featureSQLCatalog struct{ catalog featureCatalog }

func hanaSQLKey(id featuresql.Identifier) string {
	if id.Quoted() {
		return id.Name()
	}
	return strings.ToUpper(id.Name())
}

func (c featureSQLCatalog) ResolveRelation(name featuresql.Name) (featuresql.RelationMetadata, error) {
	parts := name.Parts()
	if len(parts) == 0 || len(parts) > 2 || hanaSQLKey(parts[len(parts)-1]) != c.catalog.Table || (len(parts) == 2 && hanaSQLKey(parts[0]) != c.catalog.Schema) {
		return featuresql.RelationMetadata{}, featureUnsupported("relation mismatch")
	}
	return featuresql.RelationMetadata{Schema: c.catalog.Schema, Name: c.catalog.Table, CatalogIdentity: strconv.FormatInt(c.catalog.OID, 10), PhysicalTable: true, Deterministic: true}, nil
}

func (c featureSQLCatalog) ResolveColumn(relation featuresql.RelationMetadata, id featuresql.Identifier) (featuresql.ColumnMetadata, error) {
	if relation.Schema != c.catalog.Schema || relation.Name != c.catalog.Table || relation.CatalogIdentity != strconv.FormatInt(c.catalog.OID, 10) {
		return featuresql.ColumnMetadata{}, featureUnsupported("relation identity mismatch")
	}
	column, ok := c.catalog.column(hanaSQLKey(id))
	if !ok || column.Hidden || column.Masked {
		return featuresql.ColumnMetadata{}, featureInvalid("feature_sql", "unknown or hidden column")
	}
	m := featuresql.ColumnMetadata{Name: column.Name, Nullable: column.Nullable, Generated: featureComputed(column), Collation: column.Collation, Precision: int(column.Length), Scale: int(column.Scale), SingleColumnUnique: c.catalog.uniqueInteger(column.Name)}
	switch column.Type {
	case "TINYINT":
		m.Kind, m.Bits, m.Unsigned = featuresql.IntegerColumn, 8, true
	case "SMALLINT":
		m.Kind, m.Bits = featuresql.IntegerColumn, 16
	case "INTEGER":
		m.Kind, m.Bits = featuresql.IntegerColumn, 32
	case "BIGINT":
		m.Kind, m.Bits = featuresql.IntegerColumn, 64
	case "DECIMAL":
		m.Kind = featuresql.DecimalColumn
	case "REAL":
		m.Kind, m.Bits = featuresql.FloatColumn, 32
	case "DOUBLE":
		m.Kind, m.Bits = featuresql.FloatColumn, 64
	case "VARCHAR", "NVARCHAR", "CHAR", "NCHAR", "CLOB", "NCLOB":
		m.Kind = featuresql.StringColumn
	case "BOOLEAN":
		m.Kind = featuresql.BooleanColumn
	case "BINARY", "VARBINARY", "BLOB":
		m.Kind = featuresql.BinaryColumn
	}
	return m, nil
}
func (featureSQLCatalog) OutputKey(id featuresql.Identifier) (string, error) {
	return hanaSQLKey(id), nil
}
func (featureSQLCatalog) QualifierKey(id featuresql.Identifier) (string, error) {
	return hanaSQLKey(id), nil
}

func bindFeatureSQLLiteral(column featuresql.ColumnMetadata, operator string, literal featuresql.Literal) (any, error) {
	unsupported := func() (any, error) { return nil, featureUnsupported("literal/storage comparison is not exact") }
	switch column.Kind {
	case featuresql.IntegerColumn:
		validWidth := (column.Unsigned && column.Bits == 8) || (!column.Unsigned && (column.Bits == 16 || column.Bits == 32 || column.Bits == 64))
		if literal.Kind() != featuresql.NumberLiteral || !validWidth {
			return unsupported()
		}
		r, ok := featureBoundedNumber(literal.Text())
		if !ok || !r.IsInt() {
			return unsupported()
		}
		n := r.Num()
		if column.Unsigned {
			if n.Sign() < 0 || n.BitLen() > column.Bits {
				return unsupported()
			}
			return n.Int64(), nil
		}
		min := new(big.Int).Neg(new(big.Int).Lsh(big.NewInt(1), uint(column.Bits-1)))
		max := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), uint(column.Bits-1)), big.NewInt(1))
		if n.Cmp(min) < 0 || n.Cmp(max) > 0 {
			return unsupported()
		}
		return n.Int64(), nil
	case featuresql.DecimalColumn:
		if literal.Kind() != featuresql.NumberLiteral || column.Scale < 0 || column.Precision <= 0 || column.Precision > 38 || column.Scale > column.Precision {
			return unsupported()
		}
		r, ok := featureBoundedNumber(literal.Text())
		if !ok {
			return unsupported()
		}
		scaled := new(big.Rat).Mul(r, new(big.Rat).SetInt(new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(column.Scale)), nil)))
		if !scaled.IsInt() || len(new(big.Int).Abs(scaled.Num()).String()) > column.Precision {
			return unsupported()
		}
		// Character transport avoids the driver's generic decimal128 path.
		// The trusted parameter expression supplies the physical decimal cast.
		return r.FloatString(column.Scale), nil
	case featuresql.StringColumn:
		if literal.Kind() != featuresql.StringLiteral || column.Collation != "" || (operator != "=" && operator != "<>" && operator != "IN") {
			return unsupported()
		}
		if column.Precision > 0 && len([]rune(literal.Text())) > column.Precision {
			return unsupported()
		}
		return literal.Text(), nil
	case featuresql.BooleanColumn:
		if literal.Kind() != featuresql.BooleanLiteral || (operator != "=" && operator != "<>" && operator != "IN") {
			return unsupported()
		}
		return literal.Text() == "true", nil
	}
	return unsupported()
}

// featureBoundedNumber normalizes decimal syntax before allocating any big
// integer powers. Only at most 38 significant digits and decimal scale 38 reach
// arithmetic. Input scanning remains bounded by the parser's 64 KiB limit.
func featureBoundedNumber(text string) (*big.Rat, bool) {
	if len(text) == 0 || len(text) > 64<<10 {
		return nil, false
	}
	negative := text[0] == '-'
	if text[0] == '-' || text[0] == '+' {
		text = text[1:]
	}
	coefficient, exponentText := text, ""
	if i := strings.IndexAny(text, "eE"); i >= 0 {
		coefficient, exponentText = text[:i], text[i+1:]
	}
	fractional := 0
	if i := strings.IndexByte(coefficient, '.'); i >= 0 {
		fractional = len(coefficient) - i - 1
		coefficient = coefficient[:i] + coefficient[i+1:]
	}
	if coefficient == "" {
		return nil, false
	}
	for _, digit := range coefficient {
		if digit < '0' || digit > '9' {
			return nil, false
		}
	}
	digits := strings.TrimLeft(coefficient, "0")
	// Zero is exact even when its exponent is outside any source range.
	if digits == "" {
		return new(big.Rat), true
	}
	normalized := strings.TrimRight(digits, "0")
	trailing := len(digits) - len(normalized)
	exponent := 0
	if exponentText != "" {
		minus := exponentText[0] == '-'
		if exponentText[0] == '-' || exponentText[0] == '+' {
			exponentText = exponentText[1:]
		}
		if exponentText == "" {
			return nil, false
		}
		// A larger exponent cannot cancel a coefficient/fraction bounded by
		// input bytes into the 38-digit source profile.
		limit := len(coefficient) + 76
		for _, digit := range exponentText {
			if digit < '0' || digit > '9' {
				return nil, false
			}
			n := int(digit - '0')
			if exponent > (limit-n)/10 {
				return nil, false
			}
			exponent = exponent*10 + n
		}
		if minus {
			exponent = -exponent
		}
	}
	scale := fractional - exponent - trailing
	if len(normalized) > 38 || scale > 38 || (scale < 0 && len(normalized)-scale > 38) {
		return nil, false
	}
	n, ok := new(big.Int).SetString(normalized, 10)
	if !ok {
		return nil, false
	}
	if negative {
		n.Neg(n)
	}
	if scale < 0 {
		n.Mul(n, new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(-scale)), nil))
		scale = 0
	}
	denominator := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(scale)), nil)
	return new(big.Rat).SetFrac(n, denominator), true
}

func featureSQLError(err error) error {
	if errors.Is(err, featuresql.ErrInvalid) {
		return featureInvalid("feature_sql", "invalid bounded SQL profile")
	}
	if errors.Is(err, featuresql.ErrUnsupported) {
		return featureUnsupported("SQL is outside bounded source profile")
	}
	return fmt.Errorf("HANA feature SQL: %w", err)
}

func resolveFeatureSQL(l *Layer, s *featureSource, plan *featuresql.Plan) error {
	id, err := featureConfigColumn(l.idField)
	if err != nil {
		return err
	}
	geometry, err := featureConfigColumn(l.geomField)
	if err != nil {
		return err
	}
	var temporal []string
	for _, field := range []string{s.Temporal.InstantField, s.Temporal.StartField, s.Temporal.EndField} {
		if field != "" {
			output, err := featureConfigColumn(field)
			if err != nil {
				return err
			}
			temporal = append(temporal, output)
		}
	}
	resolved, err := featuresql.Resolve(plan, featureSQLCatalog{s.Catalog}, featuresql.ResolveOptions{IdentityOutput: id, GeometryOutput: geometry, TemporalOutputs: temporal})
	if err != nil {
		return featureSQLError(err)
	}
	for _, projection := range resolved.Projections() {
		s.Projections = append(s.Projections, featureProjection{Output: projection.Output, Physical: projection.Column.Name})
	}
	s.BasePredicate, s.BaseArgs, _, err = featuresql.CompileWhere(resolved, featuresql.RenderOptions{
		QuoteIdentifier: quoteIdent, Placeholder: func(int) string { return "?" }, BindLiteral: bindFeatureSQLLiteral, FirstParameter: 1, Qualifier: "L",
		ParameterExpression: func(column featuresql.ColumnMetadata, _ string, placeholder string) (string, error) {
			if column.Kind == featuresql.DecimalColumn {
				if column.Precision <= 0 || column.Precision > 38 || column.Scale < 0 || column.Scale > column.Precision {
					return "", featureUnsupported("decimal comparison metadata invalid")
				}
				return fmt.Sprintf("CAST(CAST(%s AS NVARCHAR(64)) AS DECIMAL(%d,%d))", placeholder, column.Precision, column.Scale), nil
			}
			return placeholder, nil
		},
	})
	return featureSQLErrorIfPresent(err)
}

func featureSQLErrorIfPresent(err error) error {
	if err == nil {
		return nil
	}
	return featureSQLError(err)
}
