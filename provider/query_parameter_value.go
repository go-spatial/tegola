package provider

import (
	"fmt"
	"strings"

	"github.com/alexeydott/tegola/internal/sqltoken"
)

// Query parameter holds normalized parameter data ready to be inserted in the
// final query
type QueryParameterValue struct {
	// Token to replace e.g., !TOKEN!
	Token string
	// SQL expression to be inserted. Contains "?" that will be replaced with an
	//  ordinal argument e.g., "$1"
	SQL string
	// Value that will be passed to the final query in arguments list
	Value interface{}
	// Raw parameter and value for debugging and monitoring
	RawParam string
	// RawValue will be "" if the param wasn't passed and defaults were used
	RawValue string
}

type Params map[string]QueryParameterValue

// ReplaceParams substitutes configured query parameter tokens for their values
// within the provided SQL string. Only tokens in SQL code context are
// substituted: token-looking text inside string literals, quoted identifiers,
// or comments is left verbatim.
func (params Params) ReplaceParams(sql string, args *[]interface{}) string {
	return params.ReplaceParamsWithDialect(sql, args, SQLDialectLegacy)
}

// SQLDialect selects the lexical rules used for parameter substitution.
type SQLDialect = sqltoken.Dialect

const (
	SQLDialectLegacy     = sqltoken.Legacy
	SQLDialectPostgreSQL = sqltoken.PostgreSQL
	SQLDialectMySQL      = sqltoken.MySQL
	SQLDialectSQLite     = sqltoken.SQLite
	SQLDialectHANA       = sqltoken.HANA
)

// ReplaceParamsWithDialect substitutes executable parameters under backend SQL rules.
func (params Params) ReplaceParamsWithDialect(sql string, args *[]interface{}, dialect SQLDialect) string {
	if params == nil {
		return sql
	}

	var (
		cache = make(map[string]string)
		sb    strings.Builder
	)

	for _, tok := range dialect.CodeTokens(sql) {
		resultSQL, ok := cache[tok.Text]
		if ok {
			// Already have it cached, replace the token and move on.
			sql = dialect.ReplaceToken(sql, tok.Text, resultSQL)
			continue
		}

		param, ok := params[tok.Text]
		if !ok {
			// Unknown token, ignoring
			continue
		}

		sb.Reset()
		sb.Grow(len(param.SQL))
		argFound := false

		// Replace every `?` in the param's SQL with a positional argument
		for _, c := range param.SQL {
			if c != '?' {
				sb.WriteRune(c)
				continue
			}

			if !argFound {
				*args = append(*args, param.Value)
				argFound = true
			}
			sb.WriteString(fmt.Sprintf("$%d", len(*args)))
		}

		resultSQL = sb.String()
		cache[tok.Text] = resultSQL
		sql = dialect.ReplaceToken(sql, tok.Text, resultSQL)
	}

	return sql
}
