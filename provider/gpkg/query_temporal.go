//go:build cgo

package gpkg

import (
	"math/big"
	"strings"
	"time"

	"github.com/alexeydott/tegola/provider"
	"github.com/alexeydott/tegola/provider/internal/featuresemantics"
)

// epochBound preserves precision beyond UnixNano's representable date range.
func epochBound(t time.Time, scale int64, ceil bool) *big.Int {
	return exactEpochBound(t, scale, ceil, "", false)
}

// exactEpochBound rounds against POSIX source coordinates without truncating
// query fractions. An inserted leap second has no integer POSIX instant: its
// lower bound is the next ordinary second, and its upper bound the prior tick.
func exactEpochBound(t time.Time, scale int64, ceil bool, fraction string, leap bool) *big.Int {
	return featuresemantics.EpochBound(t, scale, ceil, fraction, leap)
}

func integerBoundSQL(field, operator string, bound *big.Int, args *[]any) string {
	if bound.IsInt64() {
		*args = append(*args, bound.Int64())
		return field + operator + "?"
	}
	// Stored endpoints are signed int64. Simplify out-of-range comparisons.
	truth := (operator == ">=" && bound.Sign() < 0) || (operator == "<=" && bound.Sign() > 0)
	if truth {
		return "1"
	}
	return "0"
}

func temporalPredicate(layer *Layer, temporal *provider.TemporalConstraint, args *[]any) string {
	m := layer.temporalMapping
	if temporal == nil || m == (provider.TemporalMapping{}) {
		return "1"
	}
	var comparisons, invalid []string
	if m.InstantField != "" {
		field := "l." + quoteIdent(m.InstantField)
		invalid = append(invalid, "("+field+" IS NOT NULL AND typeof("+field+")<>'integer')")
		if temporal.Start != nil {
			comparisons = append(comparisons, integerBoundSQL(field, ">=", exactEpochBound(*temporal.Start, layer.temporalScale, true, temporal.StartSubNanosecond, temporal.StartLeapSecond), args))
		}
		if temporal.End != nil {
			comparisons = append(comparisons, integerBoundSQL(field, "<=", exactEpochBound(*temporal.End, layer.temporalScale, false, temporal.EndSubNanosecond, temporal.EndLeapSecond), args))
		}
		return "(" + strings.Join(invalid, " OR ") + " OR " + field + " IS NULL OR (" + strings.Join(comparisons, " AND ") + "))"
	}
	start, end := "l."+quoteIdent(m.StartField), "l."+quoteIdent(m.EndField)
	invalid = []string{
		"(" + start + " IS NOT NULL AND typeof(" + start + ")<>'integer')",
		"(" + end + " IS NOT NULL AND typeof(" + end + ")<>'integer')",
		"(" + start + " IS NOT NULL AND " + end + " IS NOT NULL AND " + start + ">" + end + ")",
	}
	if temporal.Start != nil {
		comparisons = append(comparisons, "("+end+" IS NULL OR "+integerBoundSQL(end, ">=", exactEpochBound(*temporal.Start, layer.temporalScale, true, temporal.StartSubNanosecond, temporal.StartLeapSecond), args)+")")
	}
	if temporal.End != nil {
		comparisons = append(comparisons, "("+start+" IS NULL OR "+integerBoundSQL(start, "<=", exactEpochBound(*temporal.End, layer.temporalScale, false, temporal.EndSubNanosecond, temporal.EndLeapSecond), args)+")")
	}
	return "(" + strings.Join(invalid, " OR ") + " OR (" + strings.Join(comparisons, " AND ") + "))"
}

func validateTemporalRow(layer *Layer, columns []string, values []any) error {
	m := layer.temporalMapping
	if m == (provider.TemporalMapping{}) {
		return nil
	}
	var start, end *int64
	for i, column := range columns {
		if column != m.InstantField && column != m.StartField && column != m.EndField {
			continue
		}
		if values[i] == nil {
			continue
		}
		value, ok := values[i].(int64)
		if !ok {
			return invalidQuery("temporal", "source value is not an INTEGER")
		}
		if column == m.StartField {
			v := value
			start = &v
		}
		if column == m.EndField {
			v := value
			end = &v
		}
	}
	if start != nil && end != nil && *start > *end {
		return invalidQuery("temporal", "source interval is reversed")
	}
	return nil
}
