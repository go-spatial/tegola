package featuresql

import (
	"fmt"
	"strings"
)

func CompileWhere(plan *ResolvedPlan, options RenderOptions) (string, []any, int, error) {
	if plan == nil || options.FirstParameter <= 0 || options.QuoteIdentifier == nil ||
		options.Placeholder == nil || options.BindLiteral == nil {
		return "", nil, options.FirstParameter, invalid("render options", 0)
	}
	if plan.relation.Schema == "" || plan.relation.Name == "" || plan.relation.CatalogIdentity == "" ||
		len(plan.projections) == 0 {
		return "", nil, options.FirstParameter, invalid("empty resolved plan", 0)
	}
	next := options.FirstParameter
	var args []any
	if !plan.hasPredicate {
		return "1=1", args, next, nil
	}
	var render func(resolvedPredicate, int) (string, error)
	render = func(node resolvedPredicate, depth int) (string, error) {
		if depth > maxDepth {
			return "", invalid("render depth", 0)
		}
		syntax := node.syntax
		if syntax.kind == And || syntax.kind == Or || syntax.kind == Not {
			expected := 2
			if syntax.kind == Not {
				expected = 1
			}
			if len(node.children) != expected {
				return "", invalid("Boolean form", 0)
			}
			left, err := render(node.children[0], depth+1)
			if err != nil {
				return "", err
			}
			if syntax.kind == Not {
				return "(NOT " + left + ")", nil
			}
			right, err := render(node.children[1], depth+1)
			if err != nil {
				return "", err
			}
			op := " AND "
			if syntax.kind == Or {
				op = " OR "
			}
			return "(" + left + op + right + ")", nil
		}
		column := options.QuoteIdentifier(node.column.Name)
		if column == "" {
			return "", invalid("quoted column", 0)
		}
		if options.Qualifier != "" {
			qual := options.QuoteIdentifier(options.Qualifier)
			if qual == "" {
				return "", invalid("quoted qualifier", 0)
			}
			column = qual + "." + column
		}
		if syntax.kind == IsNull {
			if len(syntax.literals) != 0 || len(node.children) != 0 {
				return "", invalid("NULL form", 0)
			}
			op := " IS NULL"
			if syntax.negated {
				op = " IS NOT NULL"
			}
			return "(" + column + op + ")", nil
		}
		if syntax.kind != Compare && syntax.kind != In {
			return "", invalid("predicate form", 0)
		}
		if (syntax.kind == Compare && len(syntax.literals) != 1) ||
			(syntax.kind == In && (len(syntax.literals) == 0 || len(syntax.literals) > maxIn)) {
			return "", invalid("literal arity", 0)
		}
		op := syntax.operator
		if syntax.kind == Compare {
			switch op {
			case "=", "<>", "!=", "<", "<=", ">", ">=":
			default:
				return "", invalid("comparison form", 0)
			}
		} else if op != "IN" {
			return "", invalid("IN form", 0)
		}
		placeholders := make([]string, 0, len(syntax.literals))
		for _, literal := range syntax.literals {
			if literal.kind > BooleanLiteral {
				return "", invalid("literal form", 0)
			}
			value, err := options.BindLiteral(node.column, op, literal)
			if err != nil {
				return "", fmt.Errorf("feature sql typed literal: %w", err)
			}
			if next == int(^uint(0)>>1) {
				return "", invalid("parameter ordinal overflow", 0)
			}
			placeholder := options.Placeholder(next)
			if placeholder == "" {
				return "", invalid("placeholder", 0)
			}
			if options.ParameterExpression != nil {
				placeholder, err = options.ParameterExpression(node.column, op, placeholder)
				if err != nil {
					return "", fmt.Errorf("feature sql parameter expression: %w", err)
				}
				if strings.TrimSpace(placeholder) == "" {
					return "", invalid("empty parameter expression", 0)
				}
			}
			args = append(args, value)
			placeholders = append(placeholders, placeholder)
			next++
		}
		if syntax.kind == Compare {
			return "(" + column + " " + op + " " + placeholders[0] + ")", nil
		}
		if syntax.negated {
			op = "NOT IN"
		}
		return "(" + column + " " + op + " (" + strings.Join(placeholders, ",") + "))", nil
	}
	statement, err := render(plan.predicate, 1)
	if err != nil {
		return "", nil, options.FirstParameter, err
	}
	return statement, args, next, nil
}
