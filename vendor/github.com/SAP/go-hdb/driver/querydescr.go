package driver

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/SAP/go-hdb/driver/internal/protocol/scanner"
)

// queryKind is the query type of a database statement.
type queryKind int

func (k queryKind) String() string {
	keyword, ok := queryKindKeyword[k]
	if ok {
		return keyword
	}
	return fmt.Sprintf("cmdKind(%d)", k)
}

// Query kind constants.
const (
	qkUnknown queryKind = iota
	qkCall
	qkSelect
	qkInsert
	qkUpdate
	qkUpsert
	qkCreate
	qkDrop
	qkSet
	qkID
)

var (
	queryKindKeyword = map[queryKind]string{
		qkUnknown: "unknown",
		qkCall:    "call",
		qkSelect:  "select",
		qkInsert:  "insert",
		qkUpdate:  "update",
		qkUpsert:  "upsert",
		qkCreate:  "create",
		qkDrop:    "drop",
		qkSet:     "set",
		qkID:      "id",
	}
	queryKeywordKind = map[string]queryKind{}
)

func init() {
	// build cmdKeywordKind from cmdKindKeyword
	for k, v := range queryKindKeyword {
		queryKeywordKind[v] = k
	}
}

func encodeID(id uint64) string {
	return fmt.Sprintf("%s %s", queryKindKeyword[qkID], strconv.FormatUint(id, 10))
}

type invalidQueryError struct {
	query string
}

func (e *invalidQueryError) Error() string {
	return fmt.Sprintf("ivalid query parameter: %s", e.query)
}

var errEmptyQuery = errors.New("query parameter is empty")

const (
	bulkQuery = "bulk"
)

// queryDescr represents a query descriptor of a database statement.
type queryDescr struct {
	query  string
	kind   queryKind
	isBulk bool
	id     uint64
}

func (d *queryDescr) String() string {
	return fmt.Sprintf("query: %s kind: %s isBulk: %t", d.query, d.kind, d.isBulk)
}

// NewQueryDescr returns a new QueryDescr instance.
func newQueryDescr(query string, sc *scanner.Scanner) (*queryDescr, error) {
	if query == "" {
		return nil, errEmptyQuery
	}

	d := &queryDescr{query: query}

	sc.Reset(query)

	// first token
	token, start, end := sc.Next()

	if token != scanner.Identifier {
		return nil, &invalidQueryError{query: query}
	}

	if strings.ToLower(query[start:end]) == bulkQuery {
		d.isBulk = true
		_, start, end = sc.Next()
	}

	// kind
	keyword := strings.ToLower(query[start:end])

	d.kind = qkUnknown
	kind, ok := queryKeywordKind[keyword]
	if ok {
		d.kind = kind
	}

	// command
	d.query = query[start:] // cut off whitespaces and bulk

	// result set id query
	if d.kind == qkID {
		token, start, end = sc.Next()
		if token != scanner.Number {
			return nil, &invalidQueryError{query: query}
		}
		var err error
		d.id, err = strconv.ParseUint(query[start:end], 10, 64)
		if err != nil {
			return nil, err
		}
	}

	// TODO potentially scan variables for named parameters

	return d, nil
}
