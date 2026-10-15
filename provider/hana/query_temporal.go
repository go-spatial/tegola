package hana

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
		return "1=1"
	}
	return "1=0"
}

func (s *featureSource) physical(output string) string {
	for _, projection := range s.Projections {
		if projection.Output == output {
			return projection.Physical
		}
	}
	return ""
}

func temporalPredicate(s *featureSource, temporal *provider.TemporalConstraint, args *[]any) string {
	m := s.Temporal
	if temporal == nil || m == (provider.TemporalMapping{}) {
		return "1=1"
	}
	field := func(output string) string { return "l." + quoteIdent(s.physical(output)) }
	var conditions []string
	if m.InstantField != "" {
		instant := field(m.InstantField)
		if temporal.Start != nil {
			conditions = append(conditions, integerBoundSQL(instant, ">=", exactEpochBound(*temporal.Start, s.TemporalScale, true, temporal.StartSubNanosecond, temporal.StartLeapSecond), args))
		}
		if temporal.End != nil {
			conditions = append(conditions, integerBoundSQL(instant, "<=", exactEpochBound(*temporal.End, s.TemporalScale, false, temporal.EndSubNanosecond, temporal.EndLeapSecond), args))
		}
		return "(" + instant + " IS NULL OR (" + strings.Join(conditions, " AND ") + "))"
	}
	start, end := field(m.StartField), field(m.EndField)
	if temporal.Start != nil {
		conditions = append(conditions, "("+end+" IS NULL OR "+integerBoundSQL(end, ">=", exactEpochBound(*temporal.Start, s.TemporalScale, true, temporal.StartSubNanosecond, temporal.StartLeapSecond), args)+")")
	}
	if temporal.End != nil {
		conditions = append(conditions, "("+start+" IS NULL OR "+integerBoundSQL(start, "<=", exactEpochBound(*temporal.End, s.TemporalScale, false, temporal.EndSubNanosecond, temporal.EndLeapSecond), args)+")")
	}
	// Invalid intervals remain candidates so source corruption cannot disappear
	// behind a nonmatching time predicate.
	return "((" + start + " IS NOT NULL AND " + end + " IS NOT NULL AND " + start + ">" + end + ") OR (" + strings.Join(conditions, " AND ") + "))"
}

func validateTemporalValues(s *featureSource, values map[string]any) error {
	m := s.Temporal
	var start, end *int64
	for _, output := range []string{m.InstantField, m.StartField, m.EndField} {
		if output == "" || values[output] == nil {
			continue
		}
		value, ok := featureInteger(values[output])
		if !ok {
			return featureInvalid("temporal", "source value is not an integral storage value")
		}
		if output == m.StartField {
			v := value
			start = &v
		}
		if output == m.EndField {
			v := value
			end = &v
		}
	}
	if start != nil && end != nil && *start > *end {
		return featureInvalid("temporal", "source interval is reversed")
	}
	return nil
}

func featureInteger(value any) (int64, bool) {
	switch v := value.(type) {
	case int64:
		return v, true
	case int32:
		return int64(v), true
	case int16:
		return int64(v), true
	case uint8:
		return int64(v), true
	}
	return 0, false
}
