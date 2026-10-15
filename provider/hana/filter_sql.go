package hana

import (
	"context"
	"fmt"
	"math/big"
	"strings"

	"github.com/alexeydott/tegola/provider"
)

// This revision has actual Unicode/binary-parameter and BOOLEAN profile proof.
// Other revisions retain Core publication without claiming this capability.
const featureFilterServerVersion = "2.00.088.00.1760424921"

func (l Layer) FeatureQueryables() (provider.FeatureQueryables, error) {
	if err := l.FeatureQuerySupported(); err != nil {
		return provider.FeatureQueryables{}, err
	}
	if l.feature.FilterError != nil {
		return provider.FeatureQueryables{}, l.feature.FilterError
	}
	if err := l.feature.Queryables.Validate(); err != nil {
		return provider.FeatureQueryables{}, featureUnsupported("queryable metadata unavailable")
	}
	return provider.NewFeatureQueryables(l.feature.Queryables.Fields())
}

func registerFeatureQueryables(ctx context.Context, db featureCatalogReader, source *featureSource) {
	var version string
	if err := db.QueryRowContext(ctx, "SELECT VERSION FROM SYS.M_DATABASE").Scan(&version); err != nil {
		source.FilterError = featureUnsupported("filter server profile unavailable")
		return
	}
	source.FilterError = buildFeatureQueryables(source, version)
}

func buildFeatureQueryables(source *featureSource, version string) error {
	source.Queryables = provider.FeatureQueryables{}
	source.FilterColumns, source.FilterVersion = nil, ""
	if version != featureFilterServerVersion {
		return featureUnsupported("filter server revision is unproved")
	}
	fields := make([]provider.FeatureQueryable, 0)
	bindings := make(map[string]featureColumn)
	for _, projection := range source.Projections {
		if source.Private[projection.Physical] || (source.Public != nil && !source.Public[projection.Output]) {
			continue
		}
		column, exists := source.Catalog.column(projection.Physical)
		if !exists || column.Hidden || column.Masked || featureComputed(column) {
			continue
		}
		kind, admitted := featureQueryableType(column)
		if !admitted {
			continue
		}
		if len(fields) >= provider.MaxQueryableFields {
			return featureUnsupported("queryable catalog exceeds its field limit")
		}
		fields = append(fields, provider.FeatureQueryable{Name: projection.Output, Type: kind, Nullable: column.Nullable})
		bindings[projection.Output] = column
	}
	catalog, err := provider.NewFeatureQueryables(fields)
	if err != nil {
		return featureUnsupported("queryable catalog is not representable")
	}
	source.Queryables, source.FilterColumns, source.FilterVersion = catalog, bindings, version
	return nil
}

func featureQueryableType(column featureColumn) (provider.QueryableType, bool) {
	switch column.Type {
	case "TINYINT", "SMALLINT", "INTEGER", "BIGINT":
		return provider.QueryableInteger, true
	case "DECIMAL":
		return provider.QueryableNumber, column.Length > 0 && column.Length <= 38 && column.Scale >= 0 && column.Scale <= column.Length
	case "BOOLEAN":
		return provider.QueryableBoolean, true
	case "NVARCHAR":
		return provider.QueryableString, column.Length > 0 && column.Length <= 5000
	default:
		return 0, false
	}
}

func compileFeatureFilter(source *featureSource, expression *provider.FilterExpression) (string, []any, error) {
	if expression == nil {
		return "", nil, nil
	}
	layer := Layer{feature: source}
	catalog, err := layer.FeatureQueryables()
	if err != nil {
		return "", nil, err
	}
	resolved, err := provider.ResolveFeatureFilter(*expression, catalog)
	if err != nil {
		return "", nil, err
	}
	var args []any
	statement, err := compileFeatureFilterNode(source, resolved.Expression().Root(), &args)
	return statement, args, err
}

func compileFeatureFilterNode(source *featureSource, node provider.FilterNode, args *[]any) (string, error) {
	switch node.Kind {
	case provider.FilterBooleanConstant:
		if node.Boolean {
			return "(1=1)", nil
		}
		return "(1=0)", nil
	case provider.FilterAnd, provider.FilterOr:
		parts := make([]string, len(node.Children))
		for i, child := range node.Children {
			var err error
			parts[i], err = compileFeatureFilterNode(source, child, args)
			if err != nil {
				return "", err
			}
		}
		join := " AND "
		if node.Kind == provider.FilterOr {
			join = " OR "
		}
		return "(" + strings.Join(parts, join) + ")", nil
	case provider.FilterNot:
		child, err := compileFeatureFilterNode(source, node.Children[0], args)
		return "(NOT " + child + ")", err
	}
	column, exists := source.FilterColumns[node.Property]
	if !exists {
		return "", featureInvalid("filter", "unknown queryable property")
	}
	physical := "l." + quoteIdent(column.Name)
	switch node.Kind {
	case provider.FilterIsNull:
		return "(" + physical + " IS NULL)", nil
	case provider.FilterIsNotNull:
		return "(" + physical + " IS NOT NULL)", nil
	case provider.FilterCompare:
		op, err := featureFilterOperator(node.Operator)
		if err != nil {
			return "", err
		}
		switch column.Type {
		case "NVARCHAR":
			*args = append(*args, []byte(node.Literal.Text()))
			return "(STRTOBIN(" + physical + ",'UTF-8') " + op + " ?)", nil
		case "BOOLEAN":
			value := int64(0)
			if node.Literal.Text() == "true" {
				value = 1
			}
			*args = append(*args, value)
			return "((CASE WHEN " + physical + " IS NULL THEN NULL WHEN " + physical + "=TRUE THEN 1 ELSE 0 END) " + op + " CAST(? AS INTEGER))", nil
		default:
			return compileFeatureNumericFilter(column, physical, node.Operator, node.Literal, args)
		}
	default:
		return "", featureInvalid("filter", "unsupported filter node")
	}
}

func featureFilterOperator(operator provider.FilterCompareOperator) (string, error) {
	switch operator {
	case provider.FilterEqual:
		return "=", nil
	case provider.FilterNotEqual:
		return "<>", nil
	case provider.FilterLess:
		return "<", nil
	case provider.FilterLessEqual:
		return "<=", nil
	case provider.FilterGreater:
		return ">", nil
	case provider.FilterGreaterEqual:
		return ">=", nil
	default:
		return "", featureInvalid("filter", "unknown comparison operator")
	}
}

func featureNumericDomain(column featureColumn) (minimum, maximum, scale *big.Int, err error) {
	scale = big.NewInt(1)
	bits := uint(0)
	switch column.Type {
	case "TINYINT":
		return big.NewInt(0), big.NewInt(255), scale, nil
	case "SMALLINT":
		bits = 16
	case "INTEGER":
		bits = 32
	case "BIGINT":
		bits = 64
	case "DECIMAL":
		if column.Length < 1 || column.Length > 38 || column.Scale < 0 || column.Scale > column.Length {
			return nil, nil, nil, featureUnsupported("decimal filter metadata invalid")
		}
		scale.Exp(big.NewInt(10), big.NewInt(column.Scale), nil)
		maximum = new(big.Int).Sub(new(big.Int).Exp(big.NewInt(10), big.NewInt(column.Length), nil), big.NewInt(1))
		return new(big.Int).Neg(new(big.Int).Set(maximum)), maximum, scale, nil
	default:
		return nil, nil, nil, featureUnsupported("numeric filter storage unproved")
	}
	limit := new(big.Int).Lsh(big.NewInt(1), bits-1)
	return new(big.Int).Neg(new(big.Int).Set(limit)), new(big.Int).Sub(limit, big.NewInt(1)), scale, nil
}

func featureNullFold(physical string, value bool) string {
	bit := "0"
	if value {
		bit = "1"
	}
	return "(CASE WHEN " + physical + " IS NULL THEN NULL ELSE " + bit + " END = 1)"
}

func compileFeatureNumericFilter(column featureColumn, physical string, operator provider.FilterCompareOperator, literal provider.FilterLiteral, args *[]any) (string, error) {
	minimum, maximum, scale, err := featureNumericDomain(column)
	if err != nil {
		return "", err
	}
	number, ok := literal.Number()
	if !ok {
		return "", featureInvalid("filter", "numeric literal required")
	}
	number.Mul(number, new(big.Rat).SetInt(scale))
	threshold := new(big.Int)
	remainder := new(big.Int)
	threshold.QuoRem(number.Num(), number.Denom(), remainder)
	if operator == provider.FilterEqual || operator == provider.FilterNotEqual {
		if remainder.Sign() != 0 || threshold.Cmp(minimum) < 0 || threshold.Cmp(maximum) > 0 {
			return featureNullFold(physical, operator == provider.FilterNotEqual), nil
		}
	} else {
		ceiling := operator == provider.FilterLess || operator == provider.FilterGreaterEqual
		if ceiling && remainder.Sign() > 0 {
			threshold.Add(threshold, big.NewInt(1))
		} else if !ceiling && remainder.Sign() < 0 {
			threshold.Sub(threshold, big.NewInt(1))
		}
		low := featureNumericComparison(minimum.Cmp(threshold), operator)
		high := featureNumericComparison(maximum.Cmp(threshold), operator)
		if low == high {
			return featureNullFold(physical, low), nil
		}
	}
	op, err := featureFilterOperator(operator)
	if err != nil {
		return "", err
	}
	if column.Type == "DECIMAL" {
		bound := new(big.Rat).SetFrac(threshold, scale).FloatString(int(column.Scale))
		*args = append(*args, bound)
		cast := fmt.Sprintf("CAST(CAST(? AS NVARCHAR(64)) AS DECIMAL(%d,%d))", column.Length, column.Scale)
		return "(" + physical + " " + op + " " + cast + ")", nil
	}
	*args = append(*args, threshold.Int64())
	return "(" + physical + " " + op + " CAST(? AS " + column.Type + "))", nil
}

func featureNumericComparison(comparison int, operator provider.FilterCompareOperator) bool {
	switch operator {
	case provider.FilterLess:
		return comparison < 0
	case provider.FilterLessEqual:
		return comparison <= 0
	case provider.FilterGreater:
		return comparison > 0
	case provider.FilterGreaterEqual:
		return comparison >= 0
	default:
		return false
	}
}
