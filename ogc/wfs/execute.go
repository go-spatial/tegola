package wfs

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/alexeydott/geom/encoding/wkb"
	"github.com/alexeydott/tegola/feature"
	"github.com/alexeydott/tegola/internal/log"
	"github.com/alexeydott/tegola/ogc/wfs/gml"
	"github.com/alexeydott/tegola/provider"
)

// actionToMutation converts one parsed Transaction action into neutral
// mutations (Update/Delete with an ID filter expand to one mutation per
// ID; document order is preserved).
func actionToMutation(v Version, schema *feature.SchemaDescriptor, act TransactionAction) ([]provider.Mutation, error) {
	switch act.Op {
	case provider.MutationInsert:
		m, err := insertActionToMutation(v, schema, act)
		if err != nil {
			return nil, err
		}
		return []provider.Mutation{m}, nil
	case provider.MutationReplace:
		m, err := insertActionToMutation(v, schema, act)
		if err != nil {
			return nil, err
		}
		// Replace needs the target ID from the feature's FID.
		if len(act.FilterIDs) != 1 {
			return nil, fmt.Errorf("Replace requires exactly one target feature ID")
		}
		m.Op = provider.MutationReplace
		m.FeatureID = act.FilterIDs[0]
		return []provider.Mutation{m}, nil
	case provider.MutationUpdate:
		return updateActionToMutations(v, schema, act)
	case provider.MutationDelete:
		return deleteActionToMutations(act)
	default:
		return nil, fmt.Errorf("unknown action")
	}
}

func insertActionToMutation(v Version, schema *feature.SchemaDescriptor, act TransactionAction) (provider.Mutation, error) {
	m := provider.Mutation{Op: provider.MutationInsert, Collection: act.TypeName, Properties: map[string]provider.MutationValue{}}
	for name, literal := range act.Properties {
		if name == schema.Geometry.Name {
			if err := validateNullGeometry(schema, literal, act.NullProperties[name]); err != nil {
				return m, err
			}
			if act.FeatureXML != "" {
				return m, &provider.MutationError{Kind: provider.MutationErrMalformedInput, Reason: "multiple geometry properties"}
			}
			m.GeometryAbsent = true
			continue
		}
		desc, ok := schema.Property(name)
		if !ok {
			return m, &provider.MutationError{Kind: provider.MutationErrSchemaViolation, Reason: fmt.Sprintf("unknown property %q", name)}
		}
		mv, err := transactionPropertyValue(desc, literal, act.NullProperties[name])
		if err != nil {
			return m, &provider.MutationError{Kind: provider.MutationErrMalformedInput, Reason: fmt.Sprintf("property %q: %v", name, err)}
		}
		m.Properties[name] = mv
	}
	if act.FeatureXML != "" {
		// A07: axis order determined from srsName in the GML, not WFS version.
		g, srid, err := gml.ParseGeometryWithSRID(act.FeatureXML, "")
		if err != nil {
			return m, &provider.MutationError{Kind: provider.MutationErrMalformedInput, Reason: fmt.Sprintf("invalid GML geometry: %v", err)}
		}
		raw, err := wkb.EncodeBytes(g)
		if err != nil {
			return m, &provider.MutationError{Kind: provider.MutationErrMalformedInput, Reason: fmt.Sprintf("WKB encode: %v", err)}
		}
		m.GeometryWKB = raw
		m.GeometrySRID = srid
	}
	return m, nil
}

func updateActionToMutations(v Version, schema *feature.SchemaDescriptor, act TransactionAction) ([]provider.Mutation, error) {
	if len(act.FilterIDs) == 0 {
		return nil, &provider.MutationError{Kind: provider.MutationErrMalformedInput, Reason: "Update without a feature ID filter is not supported"}
	}
	props := map[string]provider.MutationValue{}
	var geomWKB []byte
	var geomSRID uint64
	var geomNull bool
	for name, literal := range act.Properties {
		if name == schema.Geometry.Name {
			if act.NullProperties[name] {
				if err := validateNullGeometry(schema, literal, true); err != nil {
					return nil, err
				}
				geomNull = true
				continue
			}
			// Geometry replacement: <Value> carries raw GML.
			// A07: axis order from srsName in the GML.
			g, srid, err := gml.ParseGeometryWithSRID(literal, "")
			if err != nil {
				return nil, &provider.MutationError{Kind: provider.MutationErrMalformedInput, Reason: fmt.Sprintf("invalid GML geometry: %v", err)}
			}
			raw, err := wkb.EncodeBytes(g)
			if err != nil {
				return nil, &provider.MutationError{Kind: provider.MutationErrMalformedInput, Reason: fmt.Sprintf("WKB encode: %v", err)}
			}
			geomWKB = raw
			geomSRID = srid
			continue
		}
		desc, ok := schema.Property(name)
		if !ok {
			return nil, &provider.MutationError{Kind: provider.MutationErrSchemaViolation, Reason: fmt.Sprintf("unknown property %q", name)}
		}
		mv, err := transactionPropertyValue(desc, literal, act.NullProperties[name])
		if err != nil {
			return nil, &provider.MutationError{Kind: provider.MutationErrMalformedInput, Reason: fmt.Sprintf("property %q: %v", name, err)}
		}
		props[name] = mv
	}
	var out []provider.Mutation
	for _, id := range act.FilterIDs {
		out = append(out, provider.Mutation{
			Op:             provider.MutationUpdate,
			Collection:     act.TypeName,
			FeatureID:      id,
			Properties:     props,
			GeometryWKB:    geomWKB,
			GeometryAbsent: geomNull,
			GeometrySRID:   geomSRID,
		})
	}
	return out, nil
}

// validateNullGeometry distinguishes explicit XML nil from an empty geometry value.
func validateNullGeometry(schema *feature.SchemaDescriptor, literal string, isNull bool) error {
	if !isNull || literal != "" {
		return &provider.MutationError{Kind: provider.MutationErrMalformedInput, Reason: "geometry property requires GML or explicit xsi:nil"}
	}
	if !schema.Geometry.Nullable {
		return &provider.MutationError{Kind: provider.MutationErrSchemaViolation, Reason: "geometry is required and does not accept null"}
	}
	return nil
}

func deleteActionToMutations(act TransactionAction) ([]provider.Mutation, error) {
	if len(act.FilterIDs) == 0 {
		return nil, &provider.MutationError{Kind: provider.MutationErrMalformedInput, Reason: "Delete without a feature ID filter is not supported"}
	}
	var out []provider.Mutation
	for _, id := range act.FilterIDs {
		out = append(out, provider.Mutation{
			Op:         provider.MutationDelete,
			Collection: act.TypeName,
			FeatureID:  id,
		})
	}
	return out, nil
}

// literalToMutationValue converts an XML text literal to a neutral value
// using the schema type. Empty text is the empty value, not null.
func literalToMutationValue(t feature.LogicalType, literal string) (provider.MutationValue, error) {
	if t != feature.TypeString {
		literal = strings.TrimSpace(literal)
	}
	switch t {
	case feature.TypeInteger:
		n, err := strconv.ParseInt(literal, 10, 64)
		if err != nil {
			return provider.MutationValue{}, fmt.Errorf("not an integer: %q", literal)
		}
		return provider.MutationValue{Kind: provider.MutationValueInteger, Integer: n}, nil
	case feature.TypeDecimal:
		if n, err := strconv.ParseFloat(literal, 64); err != nil || math.IsNaN(n) || math.IsInf(n, 0) {
			return provider.MutationValue{}, fmt.Errorf("not a number: %q", literal)
		}
		return provider.MutationValue{Kind: provider.MutationValueDecimal, Decimal: literal}, nil
	case feature.TypeString:
		mv := provider.MutationValue{Kind: provider.MutationValueString, String: literal}
		if literal == "" {
			mv.Empty = true
		}
		return mv, nil
	case feature.TypeBoolean:
		switch literal {
		case "true", "1":
			return provider.MutationValue{Kind: provider.MutationValueBoolean, Boolean: true}, nil
		case "false", "0":
			return provider.MutationValue{Kind: provider.MutationValueBoolean, Boolean: false}, nil
		default:
			return provider.MutationValue{}, fmt.Errorf("not a boolean: %q", literal)
		}
	case feature.TypeDateTime:
		return provider.MutationValue{Kind: provider.MutationValueString, String: literal}, nil
	default:
		return provider.MutationValue{}, fmt.Errorf("unsupported type")
	}
}

// ExecuteTransaction runs parsed actions through the coordinator in
// document order, in one native transaction.
func ExecuteTransaction(ctx context.Context, coord *feature.MutationCoordinator, schemas func(string) (*feature.SchemaDescriptor, error), v Version, actions []TransactionAction, principal feature.Principal) ([]TransactionResult, error) {
	results, _, err := ExecuteTransactionWithReceipt(ctx, coord, schemas, v, actions, principal)
	return results, err
}

// ExecuteTransactionWithReceipt preserves the commit outcome and correlation ID
// even when execution fails or the database acknowledgement is lost.
func ExecuteTransactionWithReceipt(ctx context.Context, coord *feature.MutationCoordinator, schemas func(string) (*feature.SchemaDescriptor, error), v Version, actions []TransactionAction, principal feature.Principal) ([]TransactionResult, provider.CommitReceipt, error) {
	notCommitted := provider.CommitReceipt{Status: provider.CommitNotCommitted}
	var mutations []provider.Mutation
	var owners []TransactionAction // parallel to mutations for result mapping
	for _, act := range actions {
		schema, err := schemas(act.TypeName)
		if err != nil {
			return nil, notCommitted, err
		}
		ms, err := actionToMutation(v, schema, act)
		if err != nil {
			return nil, notCommitted, err
		}
		for _, m := range ms {
			mutations = append(mutations, m)
			owners = append(owners, act)
		}
	}
	outcomes, receipt, err := coord.ExecuteAll(ctx, principal, mutations)
	if err != nil && receipt.Status != provider.CommitCommitted {
		return nil, receipt, err
	}
	if receipt.Status != provider.CommitCommitted {
		return nil, receipt, &provider.MutationError{Kind: provider.MutationErrCommitUnknown, Reason: "commit outcome unknown"}
	}
	if err != nil {
		log.Error("WFS transaction committed with auxiliary failure", "transaction", receipt.TransactionID, "error", err)
	}
	results := make([]TransactionResult, 0, len(outcomes))
	for i, o := range outcomes {
		results = append(results, TransactionResult{
			Op:        owners[i].Op,
			TypeName:  owners[i].TypeName,
			FeatureID: o.FeatureID,
			Affected:  o.Affected,
		})
	}
	return results, receipt, nil
}

func transactionPropertyValue(desc feature.PropertyDescriptor, literal string, isNull bool) (provider.MutationValue, error) {
	if isNull {
		if !desc.Nullable {
			return provider.MutationValue{}, fmt.Errorf("property is not nullable")
		}
		return provider.MutationValue{Null: true}, nil
	}
	return literalToMutationValue(desc.Type, literal)
}
