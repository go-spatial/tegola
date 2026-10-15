// Package featuresemantics contains backend-independent feature operations.
package featuresemantics

import (
	"math/big"
	"strings"
	"time"
)

// EpochBound returns a detached integer source bound. Callers must validate the
// positive scale (1, 1000, 1000000 or 1000000000) and TemporalConstraint fraction
// and leap evidence before calling. ceil selects a lower inclusive bound;
// otherwise it selects an upper inclusive bound. The result may exceed int64.
//
// Inserted leap seconds have no POSIX integer instant: the lower bound is the
// next ordinary second and the upper bound is the preceding source tick.
func EpochBound(t time.Time, scale int64, ceil bool, fraction string, leap bool) *big.Int {
	if leap {
		value := new(big.Int).Add(big.NewInt(t.Unix()), big.NewInt(1))
		value.Mul(value, big.NewInt(scale))
		if !ceil {
			value.Sub(value, big.NewInt(1))
		}
		return value
	}
	value := new(big.Int).Mul(big.NewInt(t.Unix()), big.NewInt(scale))
	numerator := new(big.Int).Mul(big.NewInt(int64(t.Nanosecond())), big.NewInt(scale))
	quotient, remainder := new(big.Int), new(big.Int)
	quotient.QuoRem(numerator, big.NewInt(1000000000), remainder)
	value.Add(value, quotient)
	if ceil && (remainder.Sign() != 0 || strings.Trim(fraction, "0") != "") {
		value.Add(value, big.NewInt(1))
	}
	return value
}
