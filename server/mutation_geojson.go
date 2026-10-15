package server

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"

	"github.com/alexeydott/geom"
	"github.com/alexeydott/geom/encoding/wkb"
	"github.com/alexeydott/tegola/feature"
	"github.com/alexeydott/tegola/provider"
)

// geoJSONFeature is the parsed mutation input.
type geoJSONFeature struct {
	Geometry    geom.Geometry
	HasGeometry bool
	// GeometryNull is explicit JSON null geometry.
	GeometryNull bool
	Properties   map[string]interface{}
}

// parseGeoJSONFeature parses a GeoJSON Feature body for mutation input.
// Duplicate keys are rejected (no silent split-brain); numbers keep
// their literal text via json.Decoder.UseNumber.
func parseGeoJSONFeature(body []byte) (*geoJSONFeature, error) {
	if err := rejectDuplicateKeys(body); err != nil {
		return nil, err
	}
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.UseNumber()
	var raw map[string]interface{}
	if err := dec.Decode(&raw); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}
	if dec.More() {
		return nil, fmt.Errorf("trailing data after JSON document")
	}
	if t, _ := raw["type"].(string); t != "Feature" {
		return nil, fmt.Errorf("expected a GeoJSON Feature")
	}
	if _, ok := raw["crs"]; ok {
		return nil, fmt.Errorf("GeoJSON crs member is not supported; input must use CRS84")
	}
	out := &geoJSONFeature{Properties: map[string]interface{}{}}
	if gv, ok := raw["geometry"]; ok {
		if gv == nil {
			out.GeometryNull = true
		} else {
			gm, ok := gv.(map[string]interface{})
			if !ok {
				return nil, fmt.Errorf("geometry must be an object or null")
			}
			g, err := parseGeoJSONGeometry(gm)
			if err != nil {
				return nil, err
			}
			out.Geometry = g
			out.HasGeometry = true
		}
	}
	if pv, ok := raw["properties"]; ok && pv != nil {
		pm, ok := pv.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("properties must be an object or null")
		}
		out.Properties = pm
	}
	return out, nil
}

func parseGeoJSONGeometry(gm map[string]interface{}) (geom.Geometry, error) {
	t, _ := gm["type"].(string)
	coords, _ := gm["coordinates"]
	switch t {
	case "Point":
		c, err := toCoord(coords)
		return geom.Point(c), err
	case "MultiPoint":
		l, err := toPositions(coords)
		return geom.MultiPoint(l), err
	case "LineString":
		return toLineString(coords)
	case "MultiLineString":
		return toMultiLineString(coords)
	case "Polygon":
		return toPolygon(coords)
	case "MultiPolygon":
		return toMultiPolygon(coords)
	default:
		return nil, fmt.Errorf("unsupported geometry type %q", t)
	}
}

func toCoord(v interface{}) ([2]float64, error) {
	arr, ok := v.([]interface{})
	if !ok || len(arr) < 2 {
		return [2]float64{}, fmt.Errorf("invalid coordinate")
	}
	// A19: Reject XYZ on XY-only profile instead of silently truncating Z.
	// XY-only is the current supported profile; XYZ requires explicit support.
	if len(arr) > 2 {
		return [2]float64{}, fmt.Errorf("3D coordinates not supported: got %d ordinates, expected 2 (XY-only profile)", len(arr))
	}
	x, err := toFloat(arr[0])
	if err != nil {
		return [2]float64{}, err
	}
	y, err := toFloat(arr[1])
	if err != nil {
		return [2]float64{}, err
	}
	return [2]float64{x, y}, nil
}

func toFloat(v interface{}) (float64, error) {
	switch n := v.(type) {
	case json.Number:
		return n.Float64()
	case float64:
		return n, nil
	default:
		return 0, fmt.Errorf("not a number")
	}
}

func toLineString(v interface{}) (geom.LineString, error) {
	points, err := toPositions(v)
	if err != nil {
		return nil, err
	}
	if len(points) < 2 {
		return nil, fmt.Errorf("linestring needs at least 2 points, got %d", len(points))
	}
	return points, nil
}

func toPositions(v interface{}) (geom.LineString, error) {
	arr, ok := v.([]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid coordinate sequence")
	}
	out := make(geom.LineString, 0, len(arr))
	for _, c := range arr {
		cc, err := toCoord(c)
		if err != nil {
			return nil, err
		}
		out = append(out, cc)
	}
	return out, nil
}

func toPolygon(v interface{}) (geom.Polygon, error) {
	arr, ok := v.([]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid polygon")
	}
	if len(arr) == 0 {
		return nil, fmt.Errorf("polygon needs at least one ring")
	}
	out := make(geom.Polygon, 0, len(arr))
	for i, r := range arr {
		ring, err := toLineString(r)
		if err != nil {
			return nil, err
		}
		// A19: validate ring cardinality and closure.
		if len(ring) < 4 {
			return nil, fmt.Errorf("polygon ring %d needs at least 4 points, got %d", i, len(ring))
		}
		if ring[0] != ring[len(ring)-1] {
			return nil, fmt.Errorf("polygon ring %d not closed: first %v != last %v", i, ring[0], ring[len(ring)-1])
		}
		out = append(out, ring)
	}
	return out, nil
}

func toMultiLineString(v interface{}) (geom.MultiLineString, error) {
	arr, ok := v.([]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid multilinestring")
	}
	out := make(geom.MultiLineString, 0, len(arr))
	for _, l := range arr {
		line, err := toLineString(l)
		if err != nil {
			return nil, err
		}
		out = append(out, line)
	}
	return out, nil
}

func toMultiPolygon(v interface{}) (geom.MultiPolygon, error) {
	arr, ok := v.([]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid multipolygon")
	}
	out := make(geom.MultiPolygon, 0, len(arr))
	for _, p := range arr {
		poly, err := toPolygon(p)
		if err != nil {
			return nil, err
		}
		out = append(out, poly)
	}
	return out, nil
}

// mutationInputToProvider converts parsed GeoJSON properties to neutral
// mutation values using the schema's logical types. JSON numbers keep
// their literal text: integers never pass through float64.
func mutationInputToProvider(schema *feature.SchemaDescriptor, props map[string]interface{}) (map[string]provider.MutationValue, error) {
	out := make(map[string]provider.MutationValue, len(props))
	for name, raw := range props {
		desc, ok := schema.Property(name)
		if !ok {
			return nil, fmt.Errorf("unknown property %q", name)
		}
		mv, err := jsonValueToMutation(desc.Type, raw)
		if err != nil {
			return nil, fmt.Errorf("property %q: %w", name, err)
		}
		out[name] = mv
	}
	return out, nil
}

func jsonValueToMutation(t feature.LogicalType, raw interface{}) (provider.MutationValue, error) {
	if raw == nil {
		return provider.MutationValue{Null: true}, nil
	}
	switch t {
	case feature.TypeInteger:
		switch n := raw.(type) {
		case json.Number:
			s := n.String()
			i, err := strconv.ParseInt(s, 10, 64)
			if err != nil {
				return provider.MutationValue{}, fmt.Errorf("not an integer: %q", s)
			}
			return provider.MutationValue{Kind: provider.MutationValueInteger, Integer: i}, nil
		default:
			return provider.MutationValue{}, fmt.Errorf("expected integer")
		}
	case feature.TypeDecimal:
		switch n := raw.(type) {
		case json.Number:
			return provider.MutationValue{Kind: provider.MutationValueDecimal, Decimal: n.String()}, nil
		default:
			return provider.MutationValue{}, fmt.Errorf("expected number")
		}
	case feature.TypeString:
		s, ok := raw.(string)
		if !ok {
			return provider.MutationValue{}, fmt.Errorf("expected string")
		}
		mv := provider.MutationValue{Kind: provider.MutationValueString, String: s}
		if s == "" {
			mv.Empty = true
		}
		return mv, nil
	case feature.TypeBoolean:
		b, ok := raw.(bool)
		if !ok {
			return provider.MutationValue{}, fmt.Errorf("expected boolean")
		}
		return provider.MutationValue{Kind: provider.MutationValueBoolean, Boolean: b}, nil
	case feature.TypeDateTime:
		s, ok := raw.(string)
		if !ok {
			return provider.MutationValue{}, fmt.Errorf("expected datetime string")
		}
		return provider.MutationValue{Kind: provider.MutationValueString, String: s}, nil
	default:
		return provider.MutationValue{}, fmt.Errorf("unsupported type")
	}
}

// applyMergePatch implements RFC 7396 JSON Merge Patch on the target
// document. null removes a member; arrays are replaced wholesale.
// A20: RFC 7396 JSON Merge Patch. Null in patch means delete.
// When patch value is a map but target is not, the patch map is
// processed (nulls removed) rather than copied verbatim.
func applyMergePatch(target map[string]interface{}, patch map[string]interface{}) map[string]interface{} {
	out := make(map[string]interface{}, len(target))
	for k, v := range target {
		out[k] = v
	}
	for k, pv := range patch {
		if pv == nil {
			delete(out, k)
			continue
		}
		if pm, ok := pv.(map[string]interface{}); ok {
			if tm, ok := out[k].(map[string]interface{}); ok {
				out[k] = applyMergePatch(tm, pm)
			} else {
				// A20: target not a map; process patch map to strip nulls.
				out[k] = applyMergePatch(map[string]interface{}{}, pm)
			}
			continue
		}
		out[k] = pv
	}
	return out
}

// canonicalFeatureBytes renders a deterministic canonical form of a
// feature for strong ETag computation: sorted property keys, fixed
// structure. The ETag is bound to this exact representation.
func canonicalFeatureBytes(id uint64, geometryJSON []byte, props map[string]interface{}) []byte {
	keys := make([]string, 0, len(props))
	for k := range props {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var buf bytes.Buffer
	buf.WriteString(`{"id":`)
	buf.WriteString(strconv.FormatUint(id, 10))
	buf.WriteString(`,"geometry":`)
	if len(geometryJSON) == 0 {
		buf.WriteString(`null`)
	} else {
		buf.Write(geometryJSON)
	}
	buf.WriteString(`,"properties":{`)
	for i, k := range keys {
		if i > 0 {
			buf.WriteByte(',')
		}
		kb, _ := json.Marshal(k)
		buf.Write(kb)
		buf.WriteByte(':')
		vb, _ := json.Marshal(props[k])
		buf.Write(vb)
	}
	buf.WriteString("}}")
	return buf.Bytes()
}

// strongETag returns the quoted strong ETag for a feature representation.
func strongETag(canonical []byte) string {
	sum := sha256.Sum256(canonical)
	return `"` + hex.EncodeToString(sum[:]) + `"`
}

// geometryToWKB encodes a parsed geometry to WKB for the mutation.
func geometryToWKB(g geom.Geometry) ([]byte, error) {
	return wkb.EncodeBytes(g)
}

// jsonRawToMap decodes a raw JSON object into a map.
func jsonRawToMap(raw json.RawMessage) map[string]interface{} {
	var out map[string]interface{}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	if err := dec.Decode(&out); err != nil {
		return nil
	}
	return out
}

// jsonEqual compares two JSON values by their canonical encoding.
func jsonEqual(a, b interface{}) bool {
	ab, err := json.Marshal(a)
	if err != nil {
		return false
	}
	bb, err := json.Marshal(b)
	if err != nil {
		return false
	}
	return bytes.Equal(ab, bb)
}

// geometryEqual reports whether the current GeoJSON geometry encodes to
// the same WKB. It is a change-detection helper, not a topology check.
func geometryEqual(current json.RawMessage, wkbBytes []byte) bool {
	if len(current) == 0 || string(current) == "null" {
		return wkbBytes == nil
	}
	gm := jsonRawToMap(current)
	if gm == nil {
		return false
	}
	g, err := parseGeoJSONGeometry(gm)
	if err != nil {
		return false
	}
	cur, err := wkb.EncodeBytes(g)
	if err != nil {
		return false
	}
	return bytes.Equal(cur, wkbBytes)
}

// rejectDuplicateKeys walks the JSON document token by token and rejects
// any object that repeats a key. encoding/json silently keeps the last
// value; for mutation input that split-brain is a correctness hazard.
func rejectDuplicateKeys(body []byte) error {
	if !json.Valid(body) {
		return fmt.Errorf("expected one complete JSON document")
	}
	dec := json.NewDecoder(bytes.NewReader(body))
	// One frame per open container: keys seen (nil for arrays) and whether
	// the next string token is a key (objects only).
	type frame struct {
		keys      map[string]struct{}
		expectKey bool
	}
	var stack []frame
	for {
		tok, err := dec.Token()
		if err != nil {
			if err.Error() == "EOF" {
				break
			}
			return fmt.Errorf("invalid JSON: %w", err)
		}
		switch v := tok.(type) {
		case json.Delim:
			switch v {
			case '{':
				stack = append(stack, frame{keys: map[string]struct{}{}, expectKey: true})
			case '[':
				stack = append(stack, frame{})
			case '}', ']':
				stack = stack[:len(stack)-1]
				// A container just ended as a value; in a parent object
				// the next string is a key again.
				if n := len(stack); n > 0 && stack[n-1].keys != nil {
					stack[n-1].expectKey = true
				}
			}
		case string:
			if n := len(stack); n > 0 && stack[n-1].keys != nil {
				if stack[n-1].expectKey {
					if _, dup := stack[n-1].keys[v]; dup {
						return fmt.Errorf("duplicate key %q", v)
					}
					stack[n-1].keys[v] = struct{}{}
					stack[n-1].expectKey = false
				} else {
					// String value ended; next string in this object is a key.
					stack[n-1].expectKey = true
				}
			}
		default:
			// Scalar value ended; in a parent object the next string is
			// a key again.
			if n := len(stack); n > 0 && stack[n-1].keys != nil {
				stack[n-1].expectKey = true
			}
		}
	}
	return nil
}
