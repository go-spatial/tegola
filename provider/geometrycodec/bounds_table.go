package geometrycodec

import (
	"fmt"
	"strings"

	"github.com/alexeydott/tegola/dict"
)

// ConfigKeyBBoxTable optionally qualifies bounds predicates in joined custom SQL.
const ConfigKeyBBoxTable = "bbox_table"

// ResolveBBoxTable reads the layer-only qualifier for generated bounds fields.
// It accepts a bare table/alias or schema.table, while result-column names stay
// unqualified for metadata inspection and feature attribute filtering.
func ResolveBBoxTable(layer dict.Dicter, layerName string) (string, error) {
	if layer == nil {
		return "", nil
	}
	if _, explicit := layer.Interface(ConfigKeyBBoxTable); !explicit {
		return "", nil
	}
	table, err := layer.String(ConfigKeyBBoxTable, nil)
	if err != nil {
		return "", fmt.Errorf("for layer (%v) invalid %v: %w", layerName, ConfigKeyBBoxTable, err)
	}
	table = strings.TrimSpace(table)
	parts := strings.Split(table, ".")
	if len(parts) > 2 {
		return "", fmt.Errorf("for layer (%v) invalid %v: use a table alias, table name or schema.table",
			layerName, ConfigKeyBBoxTable)
	}
	for _, part := range parts {
		if !simpleIdentifierRegexp.MatchString(part) {
			return "", fmt.Errorf("for layer (%v) invalid %v: %q is not a simple SQL identifier",
				layerName, ConfigKeyBBoxTable, part)
		}
	}
	return table, nil
}

// BoundsQuote qualifies each bounds field with a validated bbox_table value.
// Components are quoted separately even when quote treats dots as literal
// identifier characters. An empty table preserves the original quoting policy.
func BoundsQuote(table string, quote func(string) string) func(string) string {
	if table == "" {
		return quote
	}
	if quote == nil {
		quote = func(name string) string { return name }
	}
	parts := strings.Split(table, ".")
	for i, part := range parts {
		parts[i] = quote(part)
	}
	prefix := strings.Join(parts, ".") + "."
	return func(field string) string { return prefix + quote(field) }
}
