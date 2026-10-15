package feature

import (
	"fmt"
	"math"
	"strconv"

	"github.com/alexeydott/geom"
	"github.com/alexeydott/geom/encoding/wkb"
)

func uint64ToString(n uint64) string {
	return strconv.FormatUint(n, 10)
}

func stringify(v interface{}) string {
	return fmt.Sprintf("%v", v)
}

// formatDecimal renders a float64 with enough digits to round-trip.
// Provider real columns arrive as float64; downstream code treats the
// text as decimal and never re-parses it into float64 for storage.
func formatDecimal(f float64) string {
	if math.IsNaN(f) || math.IsInf(f, 0) {
		return "0"
	}
	return strconv.FormatFloat(f, 'g', -1, 64)
}

// encodeGeometryWKB encodes the original storage geometry as WKB.
// The vendored encoder preserves Z/M dimensions; unsupported types are
// an error, never a silent drop.
func encodeGeometryWKB(g geom.Geometry) ([]byte, error) {
	bs, err := wkb.EncodeBytes(g)
	if err != nil {
		return nil, fmt.Errorf("WKB encode: %w", err)
	}
	return bs, nil
}
