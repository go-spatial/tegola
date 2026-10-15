// Package feature holds the neutral feature services shared by the WFS
// and OGC API Features Part 4 adapters: schema descriptors, identity,
// authorization policy, query coordination and mutation coordination.
// See ADR-0010.
package feature

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/alexeydott/tegola/provider"
)

// ValueState distinguishes the four states a property can be in:
// absent from the payload, explicit JSON null, empty value, or a value.
// The states must not be collapsed: e.g. JSON merge-patch null removes a
// member and is not an instruction to store SQL NULL.
type ValueState int

const (
	ValueAbsent ValueState = iota
	ValueNull
	ValueEmpty
	ValuePresent
)

// LogicalType is the schema-level type of a property. Integer and decimal
// values never pass through float64.
type LogicalType int

const (
	TypeInteger LogicalType = iota
	TypeDecimal
	TypeString
	TypeBoolean
	TypeDateTime
	TypeGeometry
)

func (t LogicalType) String() string {
	switch t {
	case TypeInteger:
		return "integer"
	case TypeDecimal:
		return "decimal"
	case TypeString:
		return "string"
	case TypeBoolean:
		return "boolean"
	case TypeDateTime:
		return "datetime"
	case TypeGeometry:
		return "geometry"
	default:
		return "unknown"
	}
}

// TypedValue carries a property value with its logical type. Integer is
// int64; Decimal is the canonical decimal string (never float64);
// String/boolean/datetime are string/bool/string (RFC3339).
type TypedValue struct {
	Type    LogicalType
	State   ValueState
	Integer int64
	Decimal string
	String  string
	Boolean bool
}

// PropertyDescriptor describes one public property and its storage mapping.
type PropertyDescriptor struct {
	// Name is the public property name.
	Name string
	// Column is the storage column name. Public names are never turned
	// into SQL identifiers without this mapping.
	Column string
	Type   LogicalType
	// Nullable, Required, ReadOnly, WriteOnly, HasDefault describe the
	// write contract. ReadOnly properties are rejected as input.
	Nullable   bool
	Required   bool
	ReadOnly   bool
	WriteOnly  bool
	HasDefault bool
	// MaxLength applies to strings; AllowedValues, when non-empty,
	// restricts the value set.
	MaxLength     int
	AllowedValues []string
}

// GeometryDimension is XY, XYZ, XYM or XYZM.
type GeometryDimension string

const (
	DimXY   GeometryDimension = "XY"
	DimXYZ  GeometryDimension = "XYZ"
	DimXYM  GeometryDimension = "XYM"
	DimXYZM GeometryDimension = "XYZM"
)

// GeometryDescriptor describes the geometry property.
type GeometryDescriptor struct {
	// Nullable reports whether an explicit null geometry is valid input.
	Nullable bool
	// Name is the public geometry property name ("geometry" for GeoJSON).
	Name string
	// Column is the storage geometry column.
	Column string
	// Type is the admitted simple-feature type: point, linestring,
	// polygon, multipoint, multilinestring, multipolygon.
	Type      string
	Dimension GeometryDimension
	// SRID is the storage SRID.
	SRID uint64
}

// RevisionStrategy describes how optimistic concurrency is proven.
type RevisionStrategy struct {
	// Column, when set, names the explicit revision column maintained by
	// the backend (or its trigger). Empty means the profile does not
	// prove revision-based concurrency.
	Column string
}

// SchemaDescriptor is the single source for input validation, XSD / JSON
// Schema generation and editor forms. See ADR-0011.
type SchemaDescriptor struct {
	Collection string
	// IDColumn is the storage primary-key column backing the public ID.
	IDColumn   string
	Properties []PropertyDescriptor
	Geometry   GeometryDescriptor
	Revision   RevisionStrategy
	// SchemaVersion identifies this descriptor for caches and ETags.
	SchemaVersion string
}

// Property returns the descriptor for a public property name.
func (s *SchemaDescriptor) Property(name string) (PropertyDescriptor, bool) {
	for _, p := range s.Properties {
		if p.Name == name {
			return p, true
		}
	}
	return PropertyDescriptor{}, false
}

// ValidateInputValue checks one input value against its descriptor.
// Unknown properties, read-only properties and type violations are
// rejected; absent vs null vs empty vs default are preserved.
func (s *SchemaDescriptor) ValidateInputValue(name string, v TypedValue) error {
	p, ok := s.Property(name)
	if !ok {
		return fmt.Errorf("unknown property %q", name)
	}
	if p.ReadOnly {
		return fmt.Errorf("property %q is read-only", name)
	}
	if v.State == ValueAbsent || v.State == ValueNull {
		if v.State == ValueNull && !p.Nullable {
			return fmt.Errorf("property %q does not accept null", name)
		}
		return nil
	}
	if v.Type != p.Type {
		return fmt.Errorf("property %q expects %s, got %s", name, p.Type, v.Type)
	}
	switch p.Type {
	case TypeString:
		if p.MaxLength > 0 && len(v.String) > p.MaxLength {
			return fmt.Errorf("property %q exceeds max length %d", name, p.MaxLength)
		}
		if len(p.AllowedValues) > 0 {
			allowed := false
			for _, a := range p.AllowedValues {
				if a == v.String {
					allowed = true
					break
				}
			}
			if !allowed {
				return fmt.Errorf("property %q value not in allowed set", name)
			}
		}
	case TypeDateTime:
		if !validRFC3339(v.String) {
			return fmt.Errorf("property %q must be an RFC3339 timestamp", name)
		}
	case TypeDecimal:
		if !isCanonicalDecimal(v.Decimal) {
			return fmt.Errorf("property %q is not a valid decimal", name)
		}
	}
	if v.State == ValueEmpty && p.Required && !p.Nullable {
		return fmt.Errorf("property %q is required and cannot be empty", name)
	}
	return nil
}

// ValidateRequired checks that all required properties are present in the
// input set (used by Insert/Replace; Update may be partial).
func (s *SchemaDescriptor) ValidateRequired(present map[string]TypedValue) error {
	for _, p := range s.Properties {
		if !p.Required || p.ReadOnly {
			continue
		}
		v, ok := present[p.Name]
		if !ok || v.State == ValueAbsent {
			if p.HasDefault {
				continue
			}
			return fmt.Errorf("required property %q is missing", p.Name)
		}
		if v.State == ValueNull && !p.Nullable {
			return fmt.Errorf("required property %q is null", p.Name)
		}
	}
	return nil
}

func isCanonicalDecimal(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return false
	}
	i := 0
	if s[0] == '+' || s[0] == '-' {
		i = 1
	}
	digits := 0
	dot := false
	for ; i < len(s); i++ {
		c := s[i]
		if c == '.' {
			if dot {
				return false
			}
			dot = true
			continue
		}
		if c < '0' || c > '9' {
			return false
		}
		digits++
	}
	return digits > 0
}

var rfc3339Input = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}[Tt]\d{2}:\d{2}:\d{2}(?:\.\d+)?(?:[Zz]|[+-](?:[01]\d|2[0-3]):[0-5]\d)$`)

func validRFC3339(value string) bool {
	if !rfc3339Input.MatchString(value) {
		return false
	}
	// RFC3339 permits lowercase separators. Keep the original mutation value;
	// normalize only the validation copy, as the read query parser does.
	normalized := value[:10] + "T" + value[11:]
	if strings.HasSuffix(normalized, "z") {
		normalized = normalized[:len(normalized)-1] + "Z"
	}
	leap := normalized[17:19] == "60"
	if leap {
		normalized = normalized[:17] + "59" + normalized[19:]
	}
	parsed, err := time.Parse(time.RFC3339Nano, normalized)
	if err != nil {
		return false
	}
	return !leap || provider.IsPositiveLeapSecondPredecessor(parsed)
}
