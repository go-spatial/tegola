package postgis

import (
	"fmt"
	"math/big"
	"time"

	"github.com/alexeydott/tegola/provider"
	"github.com/alexeydott/tegola/provider/internal/featuresemantics"
)

func featureEpochBound(t time.Time, scale int64, ceil bool, fraction string, leap bool) *big.Int {
	return featuresemantics.EpochBound(t, scale, ceil, fraction, leap)
}

func featureInt64(value any) (int64, bool) {
	switch v := value.(type) {
	case int16:
		return int64(v), true
	case int32:
		return int64(v), true
	case int64:
		return v, true
	}
	return 0, false
}

func (p *featureProfile) matchesFeatureTemporal(values map[string]any, q *provider.TemporalConstraint) (bool, error) {
	m := p.temporal
	if m == (provider.TemporalMapping{}) {
		return true, nil
	}
	read := func(field string) (*int64, error) {
		if field == "" || values[field] == nil {
			return nil, nil
		}
		v, ok := featureInt64(values[field])
		if !ok {
			return nil, fmt.Errorf("source temporal field is not an integer")
		}
		return &v, nil
	}
	start, err := read(m.StartField)
	if err != nil {
		return false, err
	}
	end, err := read(m.EndField)
	if err != nil {
		return false, err
	}
	instant, err := read(m.InstantField)
	if err != nil {
		return false, err
	}
	if start != nil && end != nil && *start > *end {
		return false, fmt.Errorf("source temporal interval is reversed")
	}
	if q == nil {
		return true, nil
	}
	if m.InstantField != "" {
		if instant == nil {
			return true, nil
		}
		start, end = instant, instant
	}
	if q.Start != nil && end != nil && big.NewInt(*end).Cmp(featureEpochBound(*q.Start, p.temporalScale, true, q.StartSubNanosecond, q.StartLeapSecond)) < 0 {
		return false, nil
	}
	if q.End != nil && start != nil && big.NewInt(*start).Cmp(featureEpochBound(*q.End, p.temporalScale, false, q.EndSubNanosecond, q.EndLeapSecond)) > 0 {
		return false, nil
	}
	return true, nil
}
