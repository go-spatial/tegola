package geometrycodec

import (
	"strconv"
	"strings"

	"github.com/alexeydott/tegola/internal/sqltoken"
)

// MetadataProbeSQL inspects the columns of prepared SQL without sampling rows.
// MySQL 5.5 can materialize derived tables despite an outer WHERE 1=0, so use
// a direct LIMIT 0 where the SELECT's outer clauses are unambiguous. Complex
// forms and executable comments retain the conservative metadata wrapper.
func (d SQLDialect) MetadataProbeSQL(sql string) string {
	if d.scanner == sqltoken.MySQL {
		if query, ok := mysqlLimitedProbeSQL(sql, 0); ok {
			return query
		}
	}
	return MetadataProbeSQL(sql)
}

// SampleProbeSQL bounds prepared SQL for registration-time row inspection.
// Direct MySQL limits avoid materializing the entire original query before
// applying a sample window. Existing smaller limits and offsets are preserved.
func (d SQLDialect) SampleProbeSQL(sql string) string {
	if d.scanner == sqltoken.MySQL {
		if query, ok := mysqlLimitedProbeSQL(sql, InspectionSampleLimit); ok {
			return query
		}
	}
	if d.scanner == sqltoken.HANA {
		return WrapProbeSQLTopStyle(sql)
	}
	return WrapProbeSQL(sql)
}

type metadataSQLToken struct {
	text       string
	start, end int
}

// mysqlMetadataTokens keeps outer words and punctuation, but never interprets
// strings, identifiers or nested SELECT clauses as an outer LIMIT.
func mysqlMetadataTokens(sql string) ([]metadataSQLToken, bool) {
	tokens := []metadataSQLToken{}
	var depth int
	for _, segment := range sqltoken.MySQL.Scan(sql) {
		if segment.Kind == sqltoken.Comment {
			comment := sql[segment.Start:segment.End]
			if strings.HasPrefix(comment, "/*!") || strings.HasPrefix(comment, "/*M!") {
				return nil, false
			}
			continue
		}
		if segment.Kind != sqltoken.Code {
			if depth == 0 {
				tokens = append(tokens, metadataSQLToken{text: "?", start: segment.Start, end: segment.End})
			}
			continue
		}
		for i := segment.Start; i < segment.End; {
			start := i
			c := sql[i]
			i++
			if c <= ' ' {
				continue
			}
			if c == ')' {
				depth--
				if depth < 0 {
					return nil, false
				}
			}
			if metadataIdentifierByte(c) {
				for i < segment.End && metadataIdentifierByte(sql[i]) {
					i++
				}
			}
			if depth == 0 {
				tokens = append(tokens, metadataSQLToken{
					text: strings.ToUpper(sql[start:i]), start: start, end: i,
				})
			}
			if c == '(' {
				depth++
			}
		}
	}
	return tokens, depth == 0
}

func metadataIdentifierByte(c byte) bool {
	return c >= 128 || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' ||
		c >= '0' && c <= '9' || c == '_' || c == '$'
}

func metadataDecimal(token string) bool {
	if token == "" {
		return false
	}
	for i := range token {
		if token[i] < '0' || token[i] > '9' {
			return false
		}
	}
	return true
}

func mysqlLimitedProbeSQL(sql string, maxRows uint64) (string, bool) {
	tokens, ok := mysqlMetadataTokens(sql)
	if !ok || len(tokens) == 0 || tokens[0].text != "SELECT" {
		return "", false
	}
	insert := len(sql)
	if tokens[len(tokens)-1].text == ";" {
		insert = tokens[len(tokens)-1].start
		tokens = tokens[:len(tokens)-1]
	}
	// Registration must not introduce direct row-locking execution. Such
	// queries retain the conservative wrapper, including their original tail.
	limit := -1
	for i, token := range tokens {
		switch token.text {
		case ";", "FOR", "LOCK", "INTO", "PROCEDURE":
			return "", false
		case "LIMIT":
			if limit >= 0 {
				return "", false
			}
			limit = i
		}
	}
	if limit < 0 {
		return sql[:insert] + "\nLIMIT " + strconv.FormatUint(maxRows, 10) + "\n" + sql[insert:], true
	}
	clause := tokens[limit+1:]
	count := 0
	switch {
	case len(clause) == 1 && metadataDecimal(clause[0].text):
	case len(clause) == 3 && metadataDecimal(clause[0].text) && metadataDecimal(clause[2].text):
		switch clause[1].text {
		case ",":
			count = 2
		case "OFFSET":
		default:
			return "", false
		}
	default:
		return "", false
	}
	rows, err := strconv.ParseUint(clause[count].text, 10, 64)
	if err != nil {
		return "", false
	}
	if rows <= maxRows {
		return sql, true
	}
	return sql[:clause[count].start] + strconv.FormatUint(maxRows, 10) + sql[clause[count].end:], true
}
