package featuresql

import (
	"fmt"
	"reflect"
)

func Resolve(plan *Plan, catalog CatalogResolver, options ResolveOptions) (*ResolvedPlan, error) {
	if plan == nil || catalog == nil || nilCatalog(catalog) ||
		options.IdentityOutput == "" || options.GeometryOutput == "" {
		return nil, invalid("resolve options", 0)
	}
	if len(plan.relation.parts) == 0 || len(plan.relation.parts) > 2 || len(plan.projections) == 0 {
		return nil, invalid("empty parsed plan", 0)
	}
	relation, err := catalog.ResolveRelation(plan.Relation())
	if err != nil {
		return nil, fmt.Errorf("feature sql catalog relation: %w", err)
	}
	if !relation.PhysicalTable || !relation.Deterministic {
		return nil, unsupported(0)
	}
	if relation.Schema == "" || relation.Name == "" || relation.CatalogIdentity == "" {
		return nil, invalid("unfrozen relation metadata", 0)
	}
	qualifier := plan.relation.parts[len(plan.relation.parts)-1]
	if plan.hasAlias {
		qualifier = plan.alias
	}
	qualifierKey, err := catalog.QualifierKey(qualifier)
	if err != nil {
		return nil, fmt.Errorf("feature sql qualifier: %w", err)
	}
	if qualifierKey == "" {
		return nil, invalid("qualifier metadata", 0)
	}
	resolved := &ResolvedPlan{relation: relation, hasPredicate: plan.hasPredicate}
	lineage := map[string]ColumnMetadata{}
	resolveColumn := func(source, qual Identifier, qualified bool) (ColumnMetadata, error) {
		if qualified {
			key, err := catalog.QualifierKey(qual)
			if err != nil {
				return ColumnMetadata{}, fmt.Errorf("feature sql qualifier: %w", err)
			}
			if key != qualifierKey {
				return ColumnMetadata{}, invalid("unrelated qualifier", 0)
			}
		}
		column, err := catalog.ResolveColumn(relation, source)
		if err != nil {
			return ColumnMetadata{}, fmt.Errorf("feature sql catalog column: %w", err)
		}
		if column.Name == "" || column.Kind > NativeGeometryColumn {
			return ColumnMetadata{}, invalid("column metadata", 0)
		}
		if previous, ok := lineage[column.Name]; ok && previous != column {
			return ColumnMetadata{}, invalid("inconsistent column lineage", 0)
		}
		lineage[column.Name] = column
		return column, nil
	}
	outputs := map[string]ResolvedProjection{}
	for _, projection := range plan.projections {
		column, err := resolveColumn(projection.source, projection.qualifier, projection.qualified)
		if err != nil {
			return nil, err
		}
		output, err := catalog.OutputKey(projection.output)
		if err != nil {
			return nil, fmt.Errorf("feature sql output label: %w", err)
		}
		if output == "" {
			return nil, invalid("output metadata", 0)
		}
		if _, exists := outputs[output]; exists {
			return nil, invalid("duplicate output label", 0)
		}
		value := ResolvedProjection{Output: output, Column: column}
		outputs[output] = value
		resolved.projections = append(resolved.projections, value)
	}
	identity, ok := outputs[options.IdentityOutput]
	if !ok {
		return nil, invalid("identity output absent", 0)
	}
	if identity.Column.Generated {
		return nil, unsupported(0)
	}
	if identity.Column.Kind != IntegerColumn || !identity.Column.SingleColumnUnique {
		return nil, unsupported(0)
	}
	switch identity.Column.Bits {
	case 8, 16, 24, 32, 64:
	default:
		return nil, unsupported(0)
	}
	geometry, ok := outputs[options.GeometryOutput]
	if !ok {
		return nil, invalid("geometry output absent", 0)
	}
	if geometry.Column.Generated {
		return nil, unsupported(0)
	}
	seenTemporal := map[string]bool{}
	for _, output := range options.TemporalOutputs {
		column, ok := outputs[output]
		if !ok {
			return nil, invalid("temporal output absent", 0)
		}
		if column.Column.Generated {
			return nil, unsupported(0)
		}
		if seenTemporal[column.Column.Name] {
			return nil, invalid("temporal lineage overlap", 0)
		}
		seenTemporal[column.Column.Name] = true
	}
	resolved.identity = identity
	resolved.geometry = geometry
	var resolvePredicate func(Predicate) (resolvedPredicate, error)
	resolvePredicate = func(node Predicate) (resolvedPredicate, error) {
		out := resolvedPredicate{syntax: clonePredicate(node)}
		switch node.kind {
		case Compare, IsNull, In:
			column, err := resolveColumn(node.column, node.qualifier, node.qualified)
			if err != nil {
				return out, err
			}
			out.column = column
		case And, Or, Not:
			for _, child := range node.children {
				value, err := resolvePredicate(child)
				if err != nil {
					return out, err
				}
				out.children = append(out.children, value)
			}
		default:
			return out, invalid("predicate form", 0)
		}
		return out, nil
	}
	if plan.hasPredicate {
		resolved.predicate, err = resolvePredicate(plan.predicate)
		if err != nil {
			return nil, err
		}
	}
	return resolved, nil
}
func nilCatalog(c CatalogResolver) bool {
	v := reflect.ValueOf(c)
	switch v.Kind() {
	case reflect.Pointer, reflect.Map, reflect.Slice, reflect.Interface, reflect.Func, reflect.Chan:
		return v.IsNil()
	}
	return false
}
