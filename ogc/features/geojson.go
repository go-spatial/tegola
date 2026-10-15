package features

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"math/big"

	"github.com/alexeydott/geom"
	codec "github.com/alexeydott/tegola/provider/geometrycodec"
)

// Feature contains detached GeoJSON geometry and JSON-domain properties.
// Numeric properties use json.Number so large integers survive serialization.
type Feature struct {
	Type       string          `json:"type"`
	ID         uint64          `json:"id"`
	Geometry   json.RawMessage `json:"geometry"`
	Properties map[string]any  `json:"properties"`
	// A30: GeometryTyped carries the typed geometry, bypassing the GeoJSON
	// intermediate when set. The WFS GML encoder uses this directly instead
	// of parsing Geometry. Not serialized to JSON.
	GeometryTyped geom.Geometry `json:"-"`
}

// FeatureCollection is a bounded page; HasMore is service metadata, not GeoJSON.
type FeatureCollection struct {
	Type           string    `json:"type"`
	Features       []Feature `json:"features"`
	NumberReturned uint64    `json:"numberReturned"`
	NumberMatched  *uint64   `json:"numberMatched,omitempty"`
	HasMore        bool      `json:"-"`
}

// WriteGeoJSON serializes a complete page before writing; encoding failures write
// no bytes. Callers must resolve/marshal pages before committing HTTP headers.
func WriteGeoJSON(ctx context.Context, w io.Writer, page FeatureCollection) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if nilInterface(w) {
		return fmt.Errorf("features: nil GeoJSON writer")
	}
	if page.Type != "FeatureCollection" || page.NumberReturned != uint64(len(page.Features)) {
		return fmt.Errorf("features: invalid GeoJSON page metadata")
	}
	if page.NumberMatched != nil && *page.NumberMatched < page.NumberReturned {
		return fmt.Errorf("features: invalid matched count")
	}
	for _, feature := range page.Features {
		if err := ctx.Err(); err != nil {
			return err
		}
		if feature.Type != "Feature" {
			return fmt.Errorf("features: invalid GeoJSON feature type")
		}
	}
	if page.Features == nil {
		page.Features = []Feature{}
	}
	raw, err := json.Marshal(page)
	if err != nil {
		return fmt.Errorf("features: marshal page: %w", err)
	}
	for len(raw) != 0 {
		if err := ctx.Err(); err != nil {
			return err
		}
		chunk := raw
		if len(chunk) > 32*1024 {
			chunk = chunk[:32*1024]
		}
		n, err := w.Write(chunk)
		if err != nil {
			return fmt.Errorf("features: write page: %w", err)
		}
		if n != len(chunk) {
			return fmt.Errorf("features: write page: %w", io.ErrShortWrite)
		}
		raw = raw[n:]
	}
	return ctx.Err()
}

func encodeGeometry(geometry geom.Geometry) (json.RawMessage, error) {
	if geometry == nil {
		return json.RawMessage("null"), nil
	}
	var value any
	switch shape := geometry.(type) {
	case geom.Point:
		value = map[string]any{"type": "Point", "coordinates": shape}
	case geom.PointZ:
		value = map[string]any{"type": "Point", "coordinates": shape}
	case geom.MultiPointZ:
		value = map[string]any{"type": "MultiPoint", "coordinates": shape}
	case geom.LineStringZ:
		if len(shape) < 2 {
			return nil, fmt.Errorf("invalid XYZ LineString coordinate count")
		}
		value = map[string]any{"type": "LineString", "coordinates": shape}
	case geom.MultiLineStringZ:
		for _, line := range shape {
			if len(line) < 2 {
				return nil, fmt.Errorf("invalid XYZ MultiLineString coordinate count")
			}
		}
		value = map[string]any{"type": "MultiLineString", "coordinates": shape}
	case geom.PolygonZ:
		if err := normalizePolygonZ(shape); err != nil {
			return nil, err
		}
		value = map[string]any{"type": "Polygon", "coordinates": shape}
	case codec.MultiPolygonZ:
		for _, polygon := range shape {
			if err := normalizePolygonZ(geom.PolygonZ(polygon)); err != nil {
				return nil, err
			}
		}
		value = map[string]any{"type": "MultiPolygon", "coordinates": shape}
	case geom.MultiPoint:
		value = map[string]any{"type": "MultiPoint", "coordinates": shape}
	case geom.LineString:
		if len(shape) < 2 {
			return nil, fmt.Errorf("invalid LineString coordinate count")
		}
		value = map[string]any{"type": "LineString", "coordinates": shape}
	case geom.MultiLineString:
		for _, line := range shape {
			if len(line) < 2 {
				return nil, fmt.Errorf("invalid MultiLineString coordinate count")
			}
		}
		value = map[string]any{"type": "MultiLineString", "coordinates": shape}
	case geom.Polygon:
		if err := normalizePolygon(shape); err != nil {
			return nil, err
		}
		value = map[string]any{"type": "Polygon", "coordinates": shape}
	case geom.MultiPolygon:
		for _, polygon := range shape {
			if err := normalizePolygon(geom.Polygon(polygon)); err != nil {
				return nil, err
			}
		}
		value = map[string]any{"type": "MultiPolygon", "coordinates": shape}
	case geom.Collection:
		geometries := make([]json.RawMessage, 0, len(shape))
		for _, child := range shape {
			if child == nil {
				return nil, fmt.Errorf("null child in GeometryCollection")
			}
			encoded, err := encodeGeometry(child)
			if err != nil {
				return nil, err
			}
			geometries = append(geometries, encoded)
		}
		value = map[string]any{"type": "GeometryCollection", "geometries": geometries}
	default:
		return nil, fmt.Errorf("unsupported GeoJSON geometry %T", geometry)
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	return raw, nil
}

// normalizePolygonZ closes response-owned rings without changing heights.
// Prefer XY winding for ordinary surfaces; vertical surfaces use the first
// nonzero projected exterior area in YZ, then ZX, as a deterministic in-plane
// orientation. Holes use the same frame and opposite orientation.
func normalizePolygonZ(polygon geom.PolygonZ) error {
	if err := codec.ValidateFeatureSpatialGeometry(polygon); err != nil {
		return err
	}
	if len(polygon) == 0 {
		return fmt.Errorf("XYZ polygon has no rings")
	}
	axes := [2]int{0, 1}
	project := func(ring [][3]float64) [][2]float64 {
		out := make([][2]float64, len(ring))
		for i, p := range ring {
			out[i] = [2]float64{p[axes[0]], p[axes[1]]}
		}
		if len(out) != 0 && ring[0] != ring[len(ring)-1] {
			out = append(out, out[0])
		}
		return out
	}
	for _, pair := range [][2]int{{0, 1}, {1, 2}, {2, 0}} {
		axes = pair
		if ringAreaSign(project(polygon[0])) != 0 {
			break
		}
	}
	for i, ring := range polygon {
		vertices := make(map[[3]float64]struct{}, len(ring))
		for _, p := range ring {
			vertices[p] = struct{}{}
		}
		if len(vertices) < 3 {
			return fmt.Errorf("XYZ polygon ring has fewer than three distinct positions")
		}
		if ring[0] != ring[len(ring)-1] {
			ring = append(ring, ring[0])
			polygon[i] = ring
		}
		orientation := ringAreaSign(project(ring))
		if orientation == 0 {
			return fmt.Errorf("zero XYZ polygon surface area")
		}
		if i == 0 && orientation < 0 || i != 0 && orientation > 0 {
			for left, right := 0, len(ring)-1; left < right; left, right = left+1, right-1 {
				ring[left], ring[right] = ring[right], ring[left]
			}
		}
	}
	return nil
}

// normalizePolygon operates only on the response-owned copy. RFC 7946 exterior
// rings are counterclockwise and interior rings clockwise.
func normalizePolygon(polygon geom.Polygon) error {
	if len(polygon) == 0 {
		return fmt.Errorf("polygon has no rings")
	}
	for i, ring := range polygon {
		// The shared WKB decoder represents closure implicitly. Restore the
		// repeated vertex on this detached copy for RFC 7946 serialization.
		vertices := make(map[[2]float64]struct{}, len(ring))
		for _, point := range ring {
			if !finite(point[0]) || !finite(point[1]) {
				return fmt.Errorf("nonfinite polygon coordinate")
			}
			vertices[point] = struct{}{}
		}
		if len(vertices) < 3 {
			return fmt.Errorf("polygon ring has fewer than three distinct positions")
		}
		if ring[0] != ring[len(ring)-1] {
			ring = append(ring, ring[0])
			polygon[i] = ring
		}
		orientation := ringAreaSign(ring)
		if orientation == 0 {
			return fmt.Errorf("zero polygon area")
		}
		reverse := (i == 0 && orientation < 0) || (i != 0 && orientation > 0)
		if reverse {
			for left, right := 0, len(ring)-1; left < right; left, right = left+1, right-1 {
				ring[left], ring[right] = ring[right], ring[left]
			}
		}
	}
	return nil
}

// ringAreaSign translates coordinates before summation to avoid cancellation of
// large absolute products. The conservative error estimate includes coordinate
// subtraction, multiplication and summation. Uncertain signs use exact rational
// arithmetic on the original finite IEEE values, including subnormal triangles.
func ringAreaSign(ring [][2]float64) int {
	origin := ring[0]
	epsilon := math.Nextafter(1, 2) - 1
	var area, uncertainty float64
	for i := 1; i < len(ring); i++ {
		a, b := ring[i-1], ring[i]
		ax, ay := a[0]-origin[0], a[1]-origin[1]
		bx, by := b[0]-origin[0], b[1]-origin[1]
		axError := epsilon*(math.Abs(a[0])+math.Abs(origin[0])) + math.SmallestNonzeroFloat64
		ayError := epsilon*(math.Abs(a[1])+math.Abs(origin[1])) + math.SmallestNonzeroFloat64
		bxError := epsilon*(math.Abs(b[0])+math.Abs(origin[0])) + math.SmallestNonzeroFloat64
		byError := epsilon*(math.Abs(b[1])+math.Abs(origin[1])) + math.SmallestNonzeroFloat64
		positive, negative := ax*by, bx*ay
		term := positive - negative
		uncertainty += axError*math.Abs(by) + byError*math.Abs(ax) + axError*byError
		uncertainty += bxError*math.Abs(ay) + ayError*math.Abs(bx) + bxError*ayError
		uncertainty += epsilon * (2*(math.Abs(positive)+math.Abs(negative)) + math.Abs(area) + math.Abs(term))
		uncertainty += math.SmallestNonzeroFloat64
		area += term
	}
	if finite(area) && finite(uncertainty) && math.Abs(area) > 4*uncertainty {
		if area < 0 {
			return -1
		}
		return 1
	}
	var exact big.Rat
	for i := 1; i < len(ring); i++ {
		var ax, ay, bx, by, positive, negative big.Rat
		ax.SetFloat64(ring[i-1][0])
		ay.SetFloat64(ring[i-1][1])
		bx.SetFloat64(ring[i][0])
		by.SetFloat64(ring[i][1])
		positive.Mul(&ax, &by)
		negative.Mul(&bx, &ay)
		positive.Sub(&positive, &negative)
		exact.Add(&exact, &positive)
	}
	return exact.Sign()
}
