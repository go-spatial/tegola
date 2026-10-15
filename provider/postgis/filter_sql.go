package postgis

import (
	"fmt"
	"strings"

	"github.com/alexeydott/tegola/provider"
)

// FeatureQueryables exposes only registration-proven public scalar columns.
func (l Layer) FeatureQueryables() (provider.FeatureQueryables, error) {
	if err := l.FeatureQuerySupported(); err != nil {
		return provider.FeatureQueryables{}, err
	}
	if l.feature.queryablesErr != nil {
		return provider.FeatureQueryables{}, l.feature.queryablesErr
	}
	if err := l.feature.queryables.Validate(); err != nil {
		return provider.FeatureQueryables{}, featureUnsupported("queryable metadata is unavailable")
	}
	return provider.NewFeatureQueryables(l.feature.queryables.Fields())
}

// Optional catalog admission cannot invalidate an otherwise admitted Core source.
func (f *featureProfile) freezeOptionalQueryables() {
	f.queryablesErr = nil
	if err := f.freezeQueryables(); err != nil {
		f.queryables = provider.FeatureQueryables{}
		f.filterColumns = nil
		f.queryablesErr = featureUnsupported("optional queryable catalog is unavailable")
	}
}

func (f *featureProfile) freezeQueryables() error {
	fields, err := f.featureFields(nil)
	if err != nil {
		return err
	}
	admitted := make([]provider.FeatureQueryable, 0, len(fields))
	lineage := make(map[string]featureColumn, len(fields))
	for _, name := range fields {
		c, ok := f.column(name)
		if !ok || c.generated != "" || c.typeSchema != "pg_catalog" {
			continue
		}
		var kind provider.QueryableType
		switch c.oid {
		case 20, 21, 23:
			kind = provider.QueryableInteger
		case 16:
			kind = provider.QueryableBoolean
		case 25, 1043:
			kind = provider.QueryableString
		default:
			continue
		}
		admitted = append(admitted, provider.FeatureQueryable{Name: name, Type: kind, Nullable: !c.notNull})
		lineage[name] = c
	}
	catalog, err := provider.NewFeatureQueryables(admitted)
	if err != nil {
		return err
	}
	f.queryables, f.filterColumns = catalog, lineage
	return nil
}

// prepareFeatureFilter compiles against the provider's own frozen catalog. The
// returned profile owns its augmented predicate/arguments; registration is untouched.
func (f *featureProfile) prepareFeatureFilter(expression *provider.FilterExpression) (*featureProfile, error) {
	if expression == nil {
		return f, nil
	}
	if f.queryablesErr != nil {
		return nil, f.queryablesErr
	}
	if err := f.queryables.Validate(); err != nil {
		return nil, featureUnsupported("queryable metadata is unavailable")
	}
	resolved, err := provider.ResolveFeatureFilter(*expression, f.queryables)
	if err != nil {
		return nil, err
	}
	args := append([]any(nil), f.whereArgs...)
	predicate, err := f.compileFeatureFilter(resolved.Expression().Root(), &args)
	if err != nil {
		return nil, err
	}
	if len(predicate) > 512*1024 || len(args) > 8192 {
		return nil, featureInvalid("filter", "compiled filter exceeds resource limits")
	}
	copy := *f
	if f.where != "" {
		copy.where = "(" + f.where + ") AND (" + predicate + ")"
	} else {
		copy.where = predicate
	}
	copy.whereArgs = args
	return &copy, nil
}

func (f *featureProfile) compileFeatureFilter(node provider.FilterNode, args *[]any) (string, error) {
	switch node.Kind {
	case provider.FilterBooleanConstant:
		if node.Boolean {
			return "TRUE", nil
		}
		return "FALSE", nil
	case provider.FilterAnd, provider.FilterOr, provider.FilterNot:
		children := make([]string, len(node.Children))
		for i, child := range node.Children {
			sql, err := f.compileFeatureFilter(child, args)
			if err != nil {
				return "", err
			}
			children[i] = "(" + sql + ")"
		}
		if node.Kind == provider.FilterNot {
			return "NOT " + children[0], nil
		}
		sep := " AND "
		if node.Kind == provider.FilterOr {
			sep = " OR "
		}
		return strings.Join(children, sep), nil
	case provider.FilterIsNull, provider.FilterIsNotNull, provider.FilterCompare:
		c, ok := f.filterColumns[node.Property]
		if !ok {
			return "", featureInvalid("filter", "unknown queryable property")
		}
		column := "l." + pgQuoteIdent(c.name)
		if node.Kind == provider.FilterIsNull {
			return column + " IS NULL", nil
		}
		if node.Kind == provider.FilterIsNotNull {
			return column + " IS NOT NULL", nil
		}
		var operator string
		switch node.Operator {
		case provider.FilterEqual:
			operator = "="
		case provider.FilterNotEqual:
			operator = "<>"
		case provider.FilterLess:
			operator = "<"
		case provider.FilterLessEqual:
			operator = "<="
		case provider.FilterGreater:
			operator = ">"
		case provider.FilterGreaterEqual:
			operator = ">="
		default:
			return "", featureInvalid("filter", "invalid comparison")
		}
		parameter := fmt.Sprintf("$%d", len(*args)+1)
		switch c.oid {
		case 20, 21, 23:
			// PostgreSQL numeric covers the bounded decimal/exponent literal domain.
			// Widen both operands exactly, preserving fractions, range and SQL UNKNOWN.
			*args = append(*args, node.Literal.Text())
			column += "::pg_catalog.numeric"
			parameter += "::pg_catalog.text::pg_catalog.numeric"
		case 16:
			*args = append(*args, node.Literal.Text() == "true")
			parameter += "::pg_catalog.bool"
		case 25, 1043:
			// UTF-8 byte order is Unicode scalar order and retains trailing spaces.
			// Neither operand inherits a session/source collation or CHAR padding.
			*args = append(*args, []byte(node.Literal.Text()))
			column = "pg_catalog.convert_to(" + column + ",'UTF8')"
			parameter += "::pg_catalog.bytea"
		default:
			return "", featureUnsupported("queryable comparison profile is unavailable")
		}
		return column + " OPERATOR(pg_catalog." + operator + ") " + parameter, nil
	default:
		return "", featureInvalid("filter", "invalid filter node")
	}
}
