package geometrycodec

import (
	"encoding/binary"
	"fmt"
	"math"
	"math/big"

	"github.com/alexeydott/geom"
	"github.com/alexeydott/tegola/mos"
	"github.com/alexeydott/tegola/provider/crsconfig"
)

// MOSStorageGeometry describes the bytes actually persisted. RawBounds use
// encoded integer coordinates in minX,maxX,minY,maxY column order.
type MOSStorageGeometry struct {
	Blob      []byte
	Geometry  geom.Geometry
	RawBounds [4]int32
}

// TransformStorageGeometry converts request XY to the pinned source definition.
// A zero input SRID is the provider's internal storage-coordinate convention.
func TransformStorageGeometry(g geom.Geometry, inputSRID, storageSRID uint64, target *crsconfig.FeatureProjection) (geom.Geometry, error) {
	switch g.(type) {
	case geom.Point, geom.MultiPoint, geom.LineString, geom.MultiLineString, geom.Polygon, geom.MultiPolygon:
	default:
		return nil, fmt.Errorf("MOS storage requires an XY geometry")
	}
	if err := ValidateFeatureSpatialGeometry(g); err != nil {
		return nil, err
	}
	if inputSRID == 0 || inputSRID == storageSRID {
		return g, nil
	}
	if target == nil {
		return nil, fmt.Errorf("storage CRS is not pinned")
	}
	definition, ok := crsconfig.CanonicalFeatureDefinition(inputSRID)
	if !ok {
		return nil, fmt.Errorf("mutation input CRS %d is unsupported", inputSRID)
	}
	source, err := crsconfig.NewFeatureProjection(definition)
	if err != nil {
		return nil, err
	}
	return TransformFeatureSpatialGeometry(g, func(xy [2]float64) ([2]float64, error) {
		ll, err := source.Inverse(xy[:])
		if err != nil {
			return [2]float64{}, err
		}
		stored, err := target.Forward(ll)
		if err != nil {
			return [2]float64{}, err
		}
		if len(stored) != 2 {
			return [2]float64{}, fmt.Errorf("storage projection returned invalid coordinate")
		}
		return [2]float64{stored[0], stored[1]}, nil
	})
}

// EncodeMOSStorage derives bounds from the encoded grid, never from unrounded
// request coordinates. Attribute-only mutations must not call this encoder.
func EncodeMOSStorage(g geom.Geometry, opts mos.Options) (MOSStorageGeometry, error) {
	var result MOSStorageGeometry
	switch g.(type) {
	case geom.Point, geom.MultiPoint, geom.LineString, geom.MultiLineString, geom.Polygon, geom.MultiPolygon:
	default:
		return result, fmt.Errorf("MOS storage requires a nonempty XY geometry")
	}
	if err := ValidateFeatureSpatialGeometry(g); err != nil {
		return result, err
	}
	if opts.UnitFactor < 0 || math.IsNaN(opts.UnitFactor) || math.IsInf(opts.UnitFactor, 0) ||
		math.IsNaN(opts.OffsetX) || math.IsNaN(opts.OffsetY) || math.IsInf(opts.OffsetX, 0) || math.IsInf(opts.OffsetY, 0) {
		return result, fmt.Errorf("invalid MOS storage scale or offset")
	}
	blob, err := mos.Encode(g, opts)
	if err != nil {
		return result, err
	}
	if err := validateEncodedMOSParts(blob); err != nil {
		return result, err
	}
	raw, err := mos.Decode(blob, mos.Options{UnitFactor: 1})
	if err != nil {
		return result, fmt.Errorf("encoded MOS geometry: %w", err)
	}
	if err := validateMOSGridGeometry(raw); err != nil {
		return result, err
	}
	bounds, err := geom.NewExtentFromGeometry(raw)
	if err != nil || bounds == nil {
		return result, fmt.Errorf("encoded MOS geometry has no bounds")
	}
	values := [4]float64{bounds.MinX(), bounds.MaxX(), bounds.MinY(), bounds.MaxY()}
	for i, v := range values {
		if math.IsNaN(v) || math.IsInf(v, 0) || v < math.MinInt32 || v > math.MaxInt32 || v != math.Trunc(v) {
			return result, fmt.Errorf("encoded MOS bound is not an int32")
		}
		result.RawBounds[i] = int32(v)
	}
	decoded, err := mos.Decode(blob, opts)
	if err != nil {
		return result, err
	}
	if err := ValidateFeatureSpatialGeometry(decoded); err != nil {
		return result, err
	}
	result.Blob, result.Geometry = blob, decoded
	return result, nil
}

// Validate each freshly encoded subobject before Decode can discard collapsed
// children. Losing a hole or one member of a multiline is a lossy write even
// when the remaining geometry is valid. Encode always uses the 12-byte header.
func validateEncodedMOSParts(blob []byte) error {
	if len(blob) < 12 {
		return fmt.Errorf("encoded MOS header is incomplete")
	}
	count := int(binary.LittleEndian.Uint16(blob[4:6]))
	position := 12 + count*4
	if position > len(blob) {
		return fmt.Errorf("encoded MOS subobjects are incomplete")
	}
	for i := 0; i < count; i++ {
		n := uint64(binary.LittleEndian.Uint32(blob[12+i*4:]))
		if n > uint64((len(blob)-position)/8) {
			return fmt.Errorf("encoded MOS points are incomplete")
		}
		points := make([][2]float64, int(n))
		for j := range points {
			points[j] = [2]float64{
				float64(int32(binary.LittleEndian.Uint32(blob[position:]))),
				float64(int32(binary.LittleEndian.Uint32(blob[position+4:]))),
			}
			position += 8
		}
		switch blob[0] {
		case mos.TypePolyline:
			if err := validateMOSGridGeometry(geom.LineString(points)); err != nil {
				return err
			}
		case mos.TypePolygon:
			if !mosGridRingHasArea(points) {
				return fmt.Errorf("MOS quantization collapses a polygon ring")
			}
		}
	}
	return nil
}

func validateMOSGridGeometry(g geom.Geometry) error {
	switch v := g.(type) {
	case geom.Point:
		return nil
	case geom.MultiPoint:
		if len(v) > 0 {
			return nil
		}
	case geom.LineString:
		for i := 1; i < len(v); i++ {
			if v[i] != v[0] {
				return nil
			}
		}
	case geom.MultiLineString:
		if len(v) == 0 {
			break
		}
		for _, line := range v {
			if err := validateMOSGridGeometry(geom.LineString(line)); err != nil {
				return err
			}
		}
		return nil
	case geom.Polygon:
		if len(v) == 0 {
			break
		}
		for _, ring := range v {
			if !mosGridRingHasArea(ring) {
				return fmt.Errorf("MOS quantization collapses a polygon ring")
			}
		}
		return nil
	case geom.MultiPolygon:
		if len(v) == 0 {
			break
		}
		for _, polygon := range v {
			if err := validateMOSGridGeometry(geom.Polygon(polygon)); err != nil {
				return err
			}
		}
		return nil
	}
	return fmt.Errorf("MOS quantization produces an empty or collapsed geometry")
}

func mosGridRingHasArea(ring [][2]float64) bool {
	if len(ring) < 3 {
		return false
	}
	// int32 products and their accumulated sum can overflow int64. Exact
	// arithmetic avoids rejecting a small ring near the integer-grid edge.
	var sum, a, b, term big.Int
	for i, point := range ring {
		next := ring[(i+1)%len(ring)]
		a.SetInt64(int64(point[0]))
		b.SetInt64(int64(next[1]))
		term.Mul(&a, &b)
		sum.Add(&sum, &term)
		a.SetInt64(int64(next[0]))
		b.SetInt64(int64(point[1]))
		term.Mul(&a, &b)
		sum.Sub(&sum, &term)
	}
	return sum.Sign() != 0
}

// MOSRawScale uses the codec's Delphi-compatible precision cap. Configurations
// above precision 10 must not select a different grid in SQL than in the blob.
func MOSRawScale(config MOSConfig) (float64, error) {
	if err := ValidateMOSPrecision(config.Precision); err != nil {
		return 0, err
	}
	if config.UnitFactor <= 0 || math.IsNaN(config.UnitFactor) || math.IsInf(config.UnitFactor, 0) {
		return 0, fmt.Errorf("invalid MOS unit factor")
	}
	scale := math.Pow(10, math.Min(config.Precision, 10)) / config.UnitFactor
	if math.IsNaN(scale) || math.IsInf(scale, 0) || scale <= 0 {
		return 0, fmt.Errorf("invalid MOS raw scale")
	}
	return scale, nil
}
