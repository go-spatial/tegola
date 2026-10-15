package mysql

import (
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"

	"github.com/alexeydott/tegola/provider"
)

type featureFilterColumn struct {
	column                 featureColumn
	kind                   provider.QueryableType
	bits, precision, scale int
	unsigned               bool
}

func (l Layer) FeatureQueryables() (provider.FeatureQueryables, error) {
	if err := l.FeatureQuerySupported(); err != nil {
		return provider.FeatureQueryables{}, err
	}
	if l.feature.queryableError != nil {
		return provider.FeatureQueryables{}, l.feature.queryableError
	}
	if err := l.feature.queryableCatalog.Validate(); err != nil {
		return provider.FeatureQueryables{}, featureUnsupported("queryable catalog unavailable")
	}
	return provider.NewFeatureQueryables(l.feature.queryableCatalog.Fields())
}

// This optional capability never disables a previously admitted Core source.
func (f *featureProfile) initializeFilterCatalog() {
	fields := make([]provider.FeatureQueryable, 0)
	columns := make(map[string]featureFilterColumn)
	for _, name := range f.properties {
		source, ok := f.sourceColumn(name)
		if !ok {
			continue
		}
		column, ok := f.schema.column(source)
		if !ok || generatedColumn(column) {
			continue
		}
		c := featureFilterColumn{column: column}
		c.bits, c.unsigned = integralColumn(column)
		switch {
		case c.bits != 0:
			c.kind = provider.QueryableInteger
		case strings.EqualFold(column.dataType, "bit") && strings.EqualFold(column.columnType, "bit(1)"):
			c.kind = provider.QueryableBoolean
		case strings.EqualFold(column.dataType, "decimal") || strings.EqualFold(column.dataType, "numeric"):
			meta := featureColumnMetadata(column)
			if meta.Precision < 1 || meta.Precision > 65 || meta.Scale < 0 || meta.Scale > 30 || meta.Scale > meta.Precision {
				continue
			}
			c.kind, c.precision, c.scale = provider.QueryableNumber, meta.Precision, meta.Scale
			c.unsigned = strings.Contains(strings.ToLower(column.columnType), "unsigned")
		case strings.EqualFold(column.characterSet, "utf8mb4"):
			switch strings.ToLower(column.dataType) {
			case "varchar", "tinytext", "text", "mediumtext", "longtext":
				c.kind = provider.QueryableString
			}
		}
		if c.kind == provider.QueryableInvalid {
			continue
		}
		fields = append(fields, provider.FeatureQueryable{Name: name, Type: c.kind, Nullable: column.nullable == "YES"})
		columns[name] = c
	}
	catalog, err := provider.NewFeatureQueryables(fields)
	if err != nil {
		f.queryableCatalog, f.queryableColumns = provider.FeatureQueryables{}, nil
		f.queryableError = featureUnsupported("optional queryable catalog unavailable")
		return
	}
	f.queryableCatalog, f.queryableColumns, f.queryableError = catalog, columns, nil
}

func (f *featureProfile) prepareFeatureFilter(expression *provider.FilterExpression) (*featureProfile, error) {
	if expression == nil {
		return f, nil
	}
	if f.queryableError != nil {
		return nil, f.queryableError
	}
	if err := f.queryableCatalog.Validate(); err != nil {
		return nil, featureUnsupported("queryable catalog unavailable")
	}
	resolved, err := provider.ResolveFeatureFilter(*expression, f.queryableCatalog)
	if err != nil {
		return nil, err
	}
	args := append([]any(nil), f.filterArgs...)
	predicate, err := f.compileFeatureFilter(resolved.Expression().Root(), &args)
	if err != nil {
		return nil, err
	}
	if len(predicate) > 512*1024 || len(args) > 8192 {
		return nil, featureInvalid("filter", "compiled filter exceeds limits")
	}
	copy := *f
	copy.filterArgs = args
	if f.filter == "" {
		copy.filter = predicate
	} else {
		copy.filter = "(" + f.filter + ") AND (" + predicate + ")"
	}
	return &copy, nil
}

func (f *featureProfile) compileFeatureFilter(node provider.FilterNode, args *[]any) (string, error) {
	switch node.Kind {
	case provider.FilterBooleanConstant:
		if node.Boolean {
			return "1", nil
		}
		return "0", nil
	case provider.FilterAnd, provider.FilterOr, provider.FilterNot:
		children := make([]string, len(node.Children))
		for i, child := range node.Children {
			value, err := f.compileFeatureFilter(child, args)
			if err != nil {
				return "", err
			}
			children[i] = "(" + value + ")"
		}
		if node.Kind == provider.FilterNot {
			return "NOT " + children[0], nil
		}
		operator := " AND "
		if node.Kind == provider.FilterOr {
			operator = " OR "
		}
		return balancedFilterSQL(children, operator), nil
	case provider.FilterIsNull, provider.FilterIsNotNull, provider.FilterCompare:
		c, ok := f.queryableColumns[node.Property]
		if !ok {
			return "", featureInvalid("filter", "unknown queryable property")
		}
		source := "l." + featureQuoteIdentifier(c.column.name)
		if node.Kind == provider.FilterIsNull {
			return source + " IS NULL", nil
		}
		if node.Kind == provider.FilterIsNotNull {
			return source + " IS NOT NULL", nil
		}
		operator, err := featureFilterOperator(node.Operator)
		if err != nil {
			return "", err
		}
		switch c.kind {
		case provider.QueryableString:
			*args = append(*args, hex.EncodeToString([]byte(node.Literal.Text())))
			return "CAST(" + source + " AS BINARY) " + operator + " UNHEX(?)", nil
		case provider.QueryableBoolean:
			value := "0"
			if node.Literal.Text() == "true" {
				value = "1"
			}
			*args = append(*args, value)
			return "CAST(" + source + " AS UNSIGNED) " + operator + " CAST(? AS UNSIGNED)", nil
		case provider.QueryableInteger, provider.QueryableNumber:
			return c.compileNumber(source, node.Operator, node.Literal, args)
		default:
			return "", featureUnsupported("queryable comparison unavailable")
		}
	default:
		return "", featureInvalid("filter", "invalid filter node")
	}
}

func balancedFilterSQL(values []string, operator string) string {
	if len(values) == 1 {
		return values[0]
	}
	middle := len(values) / 2
	return "(" + balancedFilterSQL(values[:middle], operator) + operator + balancedFilterSQL(values[middle:], operator) + ")"
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
		return "", featureInvalid("filter", "invalid comparison")
	}
}

// Directed lattice rounding compares exact literals without narrowing to float
// or allowing a DECIMAL cast to round an unrepresentable operand.
func (c featureFilterColumn) compileNumber(source string, operator provider.FilterCompareOperator, literal provider.FilterLiteral, args *[]any) (string, error) {
	rational, ok := literal.Number()
	if !ok {
		return "", featureInvalid("filter", "invalid numeric literal")
	}
	scale := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(c.scale)), nil)
	rational.Mul(rational, new(big.Rat).SetInt(scale))
	minimum, maximum := new(big.Int), new(big.Int)
	if c.kind == provider.QueryableNumber {
		maximum.Exp(big.NewInt(10), big.NewInt(int64(c.precision)), nil).Sub(maximum, big.NewInt(1))
		if !c.unsigned {
			minimum.Neg(maximum)
		}
	} else if c.unsigned {
		maximum.Lsh(big.NewInt(1), uint(c.bits)).Sub(maximum, big.NewInt(1))
	} else {
		maximum.Lsh(big.NewInt(1), uint(c.bits-1))
		minimum.Neg(maximum)
		maximum.Sub(maximum, big.NewInt(1))
	}
	floor := new(big.Int).Div(rational.Num(), rational.Denom())
	ceil := new(big.Int).Set(floor)
	if !rational.IsInt() {
		ceil.Add(ceil, big.NewInt(1))
	}
	bound := new(big.Int)
	direction := ""
	fold := func(value bool) string {
		constant := "0"
		if value {
			constant = "1"
		}
		return "CASE WHEN " + source + " IS NULL THEN NULL ELSE " + constant + " END"
	}
	switch operator {
	case provider.FilterEqual, provider.FilterNotEqual:
		if !rational.IsInt() || floor.Cmp(minimum) < 0 || floor.Cmp(maximum) > 0 {
			return fold(operator == provider.FilterNotEqual), nil
		}
		bound.Set(floor)
		direction, _ = featureFilterOperator(operator)
	case provider.FilterLess:
		bound.Sub(ceil, big.NewInt(1))
		direction = "<="
	case provider.FilterLessEqual:
		bound.Set(floor)
		direction = "<="
	case provider.FilterGreater:
		bound.Add(floor, big.NewInt(1))
		direction = ">="
	case provider.FilterGreaterEqual:
		bound.Set(ceil)
		direction = ">="
	default:
		return "", featureInvalid("filter", "invalid comparison")
	}
	if direction == "<=" {
		if bound.Cmp(minimum) < 0 {
			return fold(false), nil
		}
		if bound.Cmp(maximum) >= 0 {
			return fold(true), nil
		}
	} else if direction == ">=" {
		if bound.Cmp(maximum) > 0 {
			return fold(false), nil
		}
		if bound.Cmp(minimum) <= 0 {
			return fold(true), nil
		}
	}
	value, cast := bound.String(), "SIGNED"
	if c.unsigned {
		// MySQL 8.4 infers a signed integer parameter for CAST(? AS
		// UNSIGNED), narrowing string operands above INT64_MAX before the
		// cast. DECIMAL(20,0) admits every uint64 exactly without that path.
		cast = "DECIMAL(20,0)"
	}
	if c.kind == provider.QueryableNumber {
		negative := bound.Sign() < 0
		digits := new(big.Int).Abs(bound).String()
		for len(digits) <= c.scale {
			digits = "0" + digits
		}
		if c.scale > 0 {
			digits = digits[:len(digits)-c.scale] + "." + digits[len(digits)-c.scale:]
		}
		if negative {
			digits = "-" + digits
		}
		value, cast = digits, fmt.Sprintf("DECIMAL(%d,%d)", c.precision, c.scale)
	}
	*args = append(*args, value)
	return source + " " + direction + " CAST(? AS " + cast + ")", nil
}
