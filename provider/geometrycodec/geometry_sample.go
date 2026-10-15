package geometrycodec

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/alexeydott/tegola/internal/sqltoken"
)

// MaxInspectionGeometryBytes bounds a registration sample cell, independently
// of the row-count window. The extra SQL byte detects truncation before decode.
const MaxInspectionGeometryBytes = 64 << 20

var mysqlSampleIdentifier = regexp.MustCompile("^(?:`(?:``|[^`])+`|[A-Za-z_][A-Za-z0-9_$]*)(?:\\s*\\.\\s*(?:`(?:``|[^`])+`|[A-Za-z_][A-Za-z0-9_$]*))*$")
var mysqlSampleAlias = regexp.MustCompile("(?i)\\s+(?:AS\\s+)?(`(?:``|[^`])+`|[A-Za-z_][A-Za-z0-9_$]*)$")

func sampleQuote(name string) string {
	return "`" + strings.ReplaceAll(name, "`", "``") + "`"
}

func sampleName(name string) string {
	name = strings.TrimSpace(name)
	if strings.HasPrefix(name, "`") && strings.HasSuffix(name, "`") {
		return strings.ReplaceAll(name[1:len(name)-1], "``", "`")
	}
	return name
}

// MySQLGeometrySampleSQL preserves direct identifier-only SELECTs without
// transferring unrelated result cells. Expressions, distinct/aggregate/window
// queries, ordering and ambiguous projections use a geometry-only derived
// projection instead. It never rewrites the operator's runtime tile SQL.
func MySQLGeometrySampleSQL(sql, geometryField string) string {
	expression := "__tegola_geometry_probe." + sampleQuote(geometryField)
	if direct, suffix, ok := mysqlDirectSampleGeometry(sql, geometryField); ok {
		return MySQL.SampleProbeSQL("SELECT " + boundedSampleExpression(direct, geometryField) + suffix)
	}
	return fmt.Sprintf("SELECT %s FROM (%s) AS __tegola_geometry_probe LIMIT %d",
		boundedSampleExpression(expression, geometryField), probeBody(sql), InspectionSampleLimit)
}

func boundedSampleExpression(expression, field string) string {
	return fmt.Sprintf("LEFT(CAST(%s AS BINARY), %d) AS %s", expression, MaxInspectionGeometryBytes+1, sampleQuote(field))
}

func mysqlDirectSampleGeometry(sql, field string) (string, string, bool) {
	tokens, ok := mysqlMetadataTokens(sql)
	if !ok || len(tokens) < 3 || tokens[0].text != "SELECT" {
		return "", "", false
	}
	from := -1
	for i, token := range tokens {
		switch token.text {
		case "DISTINCT", "DISTINCTROW", "ALL", "GROUP", "HAVING", "UNION", "WINDOW", "OVER", "ORDER", "FOR", "LOCK", "INTO", "PROCEDURE":
			return "", "", false
		case "FROM":
			if from >= 0 {
				return "", "", false
			}
			from = i
		case ";":
			if i != len(tokens)-1 {
				return "", "", false
			}
		}
	}
	if from < 0 {
		return "", "", false
	}
	projection := sql[tokens[0].end:tokens[from].start]
	// Commas inside quoted identifiers must not split a projection.
	var parts []string
	start := 0
	for _, segment := range MySQL.scanner.Scan(projection) {
		if segment.Kind != sqltoken.Code {
			continue
		}
		for i := segment.Start; i < segment.End; i++ {
			if projection[i] == ',' {
				parts = append(parts, projection[start:i])
				start = i + 1
			}
		}
	}
	parts = append(parts, projection[start:])
	geometry := ""
	for _, part := range parts {
		identifier := strings.TrimSpace(part)
		output := ""
		if match := mysqlSampleAlias.FindStringSubmatchIndex(identifier); match != nil {
			candidate := strings.TrimSpace(identifier[:match[0]])
			// Whitespace before the final qualified identifier is not an
			// alias (for example, table . column). Strip a suffix only when
			// the remaining expression independently proves an identifier.
			if mysqlSampleIdentifier.MatchString(candidate) {
				output = sampleName(identifier[match[2]:match[3]])
				identifier = candidate
			}
		}
		if !mysqlSampleIdentifier.MatchString(identifier) {
			return "", "", false
		}
		if output == "" {
			// The final dot outside backticks separates the output name.
			last := 0
			for _, segment := range MySQL.scanner.Scan(identifier) {
				if segment.Kind != sqltoken.Code {
					continue
				}
				if dot := strings.LastIndex(identifier[segment.Start:segment.End], "."); dot >= 0 {
					last = segment.Start + dot + 1
				}
			}
			output = sampleName(identifier[last:])
		}
		if strings.EqualFold(output, field) {
			if geometry != "" {
				return "", "", false
			}
			geometry = identifier
		}
	}
	return geometry, " " + sql[tokens[from].start:], geometry != ""
}
