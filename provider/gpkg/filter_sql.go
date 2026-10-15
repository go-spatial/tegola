//go:build cgo

package gpkg

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"math/big"
	"reflect"
	"strings"

	"github.com/alexeydott/tegola/provider"
	codec "github.com/alexeydott/tegola/provider/geometrycodec"
)

func (l Layer) FeatureQueryables() (provider.FeatureQueryables, error) {
	if err := l.FeatureQuerySupported(); err != nil {
		return provider.FeatureQueryables{}, err
	}
	if l.filterError != nil {
		return provider.FeatureQueryables{}, l.filterError
	}
	if l.filterProfile == nil {
		return provider.FeatureQueryables{}, filterUnsupported()
	}
	return provider.NewFeatureQueryables(l.filterProfile.catalog.Fields())
}

func filterUnsupported() error {
	return fmt.Errorf("gpkg optional queryable profile unavailable: %w", provider.ErrUnsupported)
}

type filterSchemaReader interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func inspectFilterSchema(ctx context.Context, reader filterSchemaReader, table string) (profile *featureFilterProfile, err error) {
	profile = &featureFilterProfile{}
	if err := reader.QueryRowContext(ctx, "PRAGMA encoding").Scan(&profile.encoding); err != nil {
		return nil, err
	}
	if profile.encoding != "UTF-8" {
		return nil, filterUnsupported()
	}
	if err := reader.QueryRowContext(ctx, "SELECT sql FROM sqlite_schema WHERE type='table' AND name=?", table).Scan(&profile.schemaSQL); err != nil {
		return nil, err
	}
	if strings.Contains(strings.ToUpper(profile.schemaSQL), "CREATE VIRTUAL TABLE") {
		return nil, filterUnsupported()
	}
	rows, err := reader.QueryContext(ctx, `SELECT name,type,"notnull",pk,hidden FROM pragma_table_xinfo(?) ORDER BY cid`, table)
	if err != nil {
		return nil, err
	}
	defer func() { err = errors.Join(err, rows.Close()) }()
	for rows.Next() {
		var column featureFilterColumn
		if err := rows.Scan(&column.name, &column.declaration, &column.notNull, &column.primary, &column.hidden); err != nil {
			return nil, err
		}
		profile.schemaColumns = append(profile.schemaColumns, column)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return profile, nil
}

func (p *Provider) initializeFilterCatalog(layer *Layer) {
	ctx, cancel := codec.NewInspectionContext()
	defer cancel()
	profile, err := inspectFilterSchema(ctx, p.db, layer.tablename)
	if err == nil {
		err = profile.freeze(layer)
	}
	if err != nil {
		layer.filterProfile = nil
		layer.filterError = filterUnsupported()
		return
	}
	layer.filterProfile, layer.filterError = profile, nil
}

func (profile *featureFilterProfile) freeze(layer *Layer) error {
	public, err := resolveQueryFields(layer, nil)
	if err != nil {
		return err
	}
	fields := make([]provider.FeatureQueryable, 0)
	columns := make(map[string]featureFilterColumn)
	for _, c := range profile.schemaColumns {
		if c.hidden != 0 || !containsExact(public, c.name) {
			continue
		}
		// SQLite declarations may carry parameters ("TEXT(15)") or aliases
		// ("DOUBLE PRECISION"); only the normalized base name is admitted.
		switch normalizeFilterColumnType(c.declaration) {
		case "INTEGER", "INT", "BIGINT", "SMALLINT", "MEDIUMINT", "TINYINT", "INT2", "INT8":
			c.kind = provider.QueryableInteger
		case "REAL", "DOUBLE", "DOUBLE PRECISION", "FLOAT", "NUMERIC", "DECIMAL":
			c.kind = provider.QueryableNumber
		case "BOOLEAN", "BOOL":
			c.kind = provider.QueryableBoolean
		case "TEXT", "CHAR", "CHARACTER", "VARCHAR", "NCHAR", "NATIVE CHARACTER", "NVARCHAR", "CLOB":
			c.kind = provider.QueryableString
		default:
			continue
		}
		fields = append(fields, provider.FeatureQueryable{Name: c.name, Type: c.kind, Nullable: c.notNull == 0})
		columns[c.name] = c
	}
	catalog, err := provider.NewFeatureQueryables(fields)
	if err != nil {
		return err
	}
	profile.catalog, profile.columns = catalog, columns
	return nil
}

// normalizeFilterColumnType reduces a SQLite column type declaration to its
// base name: parameters ("TEXT(15)"), letter case and surrounding space are
// removed. Only exact base names are admitted by the caller; anything else
// stays unqueryable.
func normalizeFilterColumnType(declaration string) string {
	t := strings.ToUpper(strings.TrimSpace(declaration))
	if i := strings.IndexByte(t, '('); i >= 0 {
		t = strings.TrimSpace(t[:i])
	}
	return t
}

func containsExact(values []string, value string) bool {
	for _, candidate := range values {
		if candidate == value {
			return true
		}
	}
	return false
}

func (l *Layer) prepareFeatureFilter(expression *provider.FilterExpression) (string, []any, error) {
	if expression == nil {
		return "", nil, nil
	}
	if _, err := l.FeatureQueryables(); err != nil {
		return "", nil, err
	}
	resolved, err := provider.ResolveFeatureFilter(*expression, l.filterProfile.catalog)
	if err != nil {
		return "", nil, err
	}
	args := []any{}
	predicate, err := l.filterProfile.compile(resolved.Expression().Root(), &args)
	if err != nil {
		return "", nil, err
	}
	if len(predicate) > 512*1024 || len(args) > 8192 {
		return "", nil, invalidQuery("filter", "compiled filter exceeds limits")
	}
	return predicate, args, nil
}

// Domain proof is a same-snapshot SQL scan, not a Go materialization. It runs
// before the client predicate so invalid stored values cannot be filtered away.
func (profile *featureFilterProfile) verify(ctx context.Context, tx *sql.Tx, table string) error {
	actual, err := inspectFilterSchema(ctx, tx, table)
	if err != nil {
		return err
	}
	if actual.schemaSQL != profile.schemaSQL || actual.encoding != profile.encoding || !reflect.DeepEqual(actual.schemaColumns, profile.schemaColumns) {
		return invalidQuery("source", "queryable schema changed")
	}
	invalid := make([]string, 0, len(profile.columns))
	// Iterate the detached sorted public catalog for deterministic SQL.
	for _, field := range profile.catalog.Fields() {
		c := profile.columns[field.Name]
		source := "l." + quoteIdent(c.name)
		domain := "typeof(" + source + ")='integer'"
		switch c.kind {
		case provider.QueryableNumber:
			// NUMERIC/DECIMAL affinity can retain exact int64 values or REAL
			// values. Both storage classes must be finite before filtering.
			maximum := fmt.Sprintf("%.17g", math.MaxFloat64)
			domain = "typeof(" + source + ") IN ('integer','real') AND " +
				source + ">=-" + maximum + " AND " + source + "<=" + maximum
		case provider.QueryableBoolean:
			domain += " AND " + source + " IN (0,1)"
		case provider.QueryableString:
			// CASE is lazy in SQLite. Only bounded TEXT reaches the Go callback;
			// BLOB length measures all bytes, including embedded NUL.
			domain = "CASE WHEN typeof(" + source + ")='text' AND length(CAST(" + source + " AS BLOB))<=" +
				fmt.Sprint(maxFilterSourceTextBytes) + " THEN " + featureUTF8Function + "(" + source + ") ELSE 0 END"
		}
		invalid = append(invalid, "("+source+" IS NOT NULL AND NOT ("+domain+"))")
	}
	if len(invalid) == 0 {
		return nil
	}
	var bad bool
	statement := "SELECT EXISTS(SELECT 1 FROM " + quoteIdent(table) + " l WHERE " + balancedFilterSQL(invalid, " OR ") + " LIMIT 1)"
	if err := tx.QueryRowContext(ctx, statement).Scan(&bad); err != nil {
		return err
	}
	if bad {
		return invalidQuery("source", "queryable scalar domain invalid")
	}
	return nil
}

func (profile *featureFilterProfile) compile(node provider.FilterNode, args *[]any) (string, error) {
	switch node.Kind {
	case provider.FilterBooleanConstant:
		if node.Boolean {
			return "1", nil
		}
		return "0", nil
	case provider.FilterAnd, provider.FilterOr, provider.FilterNot:
		children := make([]string, len(node.Children))
		for i, child := range node.Children {
			value, err := profile.compile(child, args)
			if err != nil {
				return "", err
			}
			children[i] = "(" + value + ")"
		}
		if node.Kind == provider.FilterNot {
			return "NOT " + children[0], nil
		}
		operator := " AND "
		if node.Kind == provider.FilterOr {
			operator = " OR "
		}
		return balancedFilterSQL(children, operator), nil
	case provider.FilterCompare, provider.FilterIsNull, provider.FilterIsNotNull:
		c, ok := profile.columns[node.Property]
		if !ok {
			return "", invalidQuery("filter", "unknown queryable property")
		}
		source := "l." + quoteIdent(c.name)
		if node.Kind == provider.FilterIsNull {
			return source + " IS NULL", nil
		}
		if node.Kind == provider.FilterIsNotNull {
			return source + " IS NOT NULL", nil
		}
		operator, err := featureFilterOperator(node.Operator)
		if err != nil {
			return "", err
		}
		switch c.kind {
		case provider.QueryableString:
			*args = append(*args, []byte(node.Literal.Text()))
			return "CAST(" + source + " AS BLOB) " + operator + " CAST(? AS BLOB)", nil
		case provider.QueryableBoolean:
			value := int64(0)
			if node.Literal.Text() == "true" {
				value = 1
			}
			*args = append(*args, value)
			return source + " " + operator + " ?", nil
		case provider.QueryableInteger:
			return compileFilterInteger(source, node.Operator, node.Literal, args)
		case provider.QueryableNumber:
			return compileFilterNumber(source, node.Operator, node.Literal, args)
		default:
			return "", filterUnsupported()
		}
	default:
		return "", invalidQuery("filter", "invalid filter node")
	}
}

func balancedFilterSQL(values []string, operator string) string {
	if len(values) == 1 {
		return values[0]
	}
	middle := len(values) / 2
	return "(" + balancedFilterSQL(values[:middle], operator) + operator + balancedFilterSQL(values[middle:], operator) + ")"
}

func featureFilterOperator(operator provider.FilterCompareOperator) (string, error) {
	switch operator {
	case provider.FilterEqual:
		return "=", nil
	case provider.FilterNotEqual:
		return "<>", nil
	case provider.FilterLess:
		return "<", nil
	case provider.FilterLessEqual:
		return "<=", nil
	case provider.FilterGreater:
		return ">", nil
	case provider.FilterGreaterEqual:
		return ">=", nil
	default:
		return "", invalidQuery("filter", "invalid comparison")
	}
}

func compileFilterInteger(source string, operator provider.FilterCompareOperator, literal provider.FilterLiteral, args *[]any) (string, error) {
	rational, ok := literal.Number()
	if !ok {
		return "", invalidQuery("filter", "invalid numeric literal")
	}
	minimum := new(big.Int).Neg(new(big.Int).Lsh(big.NewInt(1), 63))
	maximum := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 63), big.NewInt(1))
	floor := new(big.Int).Div(rational.Num(), rational.Denom())
	ceil := new(big.Int).Set(floor)
	if !rational.IsInt() {
		ceil.Add(ceil, big.NewInt(1))
	}
	bound := new(big.Int)
	direction := ""
	fold := func(value bool) string {
		constant := "0"
		if value {
			constant = "1"
		}
		return "CASE WHEN " + source + " IS NULL THEN NULL ELSE " + constant + " END"
	}
	switch operator {
	case provider.FilterEqual, provider.FilterNotEqual:
		if !rational.IsInt() || floor.Cmp(minimum) < 0 || floor.Cmp(maximum) > 0 {
			return fold(operator == provider.FilterNotEqual), nil
		}
		bound.Set(floor)
		direction, _ = featureFilterOperator(operator)
	case provider.FilterLess:
		bound.Sub(ceil, big.NewInt(1))
		direction = "<="
	case provider.FilterLessEqual:
		bound.Set(floor)
		direction = "<="
	case provider.FilterGreater:
		bound.Add(floor, big.NewInt(1))
		direction = ">="
	case provider.FilterGreaterEqual:
		bound.Set(ceil)
		direction = ">="
	default:
		return "", invalidQuery("filter", "invalid comparison")
	}
	if direction == "<=" {
		if bound.Cmp(minimum) < 0 {
			return fold(false), nil
		}
		if bound.Cmp(maximum) >= 0 {
			return fold(true), nil
		}
	} else if direction == ">=" {
		if bound.Cmp(maximum) > 0 {
			return fold(false), nil
		}
		if bound.Cmp(minimum) <= 0 {
			return fold(true), nil
		}
	}
	*args = append(*args, bound.Int64())
	return source + " " + direction + " ?", nil
}

// compileFilterNumber dispatches by actual SQLite storage, preserving integer
// precision in NUMERIC/DECIMAL columns. Arguments follow SQL branch order.
func compileFilterNumber(source string, operator provider.FilterCompareOperator, literal provider.FilterLiteral, args *[]any) (string, error) {
	integer, err := compileFilterInteger(source, operator, literal, args)
	if err != nil {
		return "", err
	}
	real, err := compileFilterReal(source, operator, literal, args)
	if err != nil {
		return "", err
	}
	return "CASE WHEN typeof(" + source + ")='integer' THEN (" + integer + ") ELSE (" + real + ") END", nil
}

// compileFilterReal compiles a comparison against a REAL storage value.
// SQLite stores REAL values as float64, so the comparison must be exact in
// float64 terms. The CQL2 numeric literal is an exact rational, and binding
// its nearest float64 would change semantics whenever the literal is not
// representable: 9007199254740993 would match a stored 9007199254740992,
// and 1e-4096 would underflow to zero and match a stored zero. Instead the
// literal is replaced by directed float64 bounds: < and > bind the bound on
// the literal's side, <= and >= bind the bound on the value's side, = folds
// to false and <> to true when the literal is not representable. A validated
// CQL2 numeric literal is a finite rational, so NaN can never arise here.
// Literals beyond the float64 range fold to constants: every finite stored
// value is below a +infinite literal and above a -infinite one. NULL never
// matches, mirroring the integer path.
func compileFilterReal(source string, operator provider.FilterCompareOperator, literal provider.FilterLiteral, args *[]any) (string, error) {
	rational, ok := literal.Number()
	if !ok {
		return "", invalidQuery("filter", "invalid numeric literal")
	}
	fold := func(value bool) string {
		constant := "0"
		if value {
			constant = "1"
		}
		return "CASE WHEN " + source + " IS NULL THEN NULL ELSE " + constant + " END"
	}
	maxFloat := new(big.Rat).SetFloat64(math.MaxFloat64)
	switch {
	case rational.Cmp(maxFloat) > 0:
		// The literal exceeds every finite float64.
		switch operator {
		case provider.FilterEqual, provider.FilterGreater, provider.FilterGreaterEqual:
			return fold(false), nil
		case provider.FilterNotEqual, provider.FilterLess, provider.FilterLessEqual:
			return fold(true), nil
		}
		return "", invalidQuery("filter", "invalid comparison")
	case rational.Cmp(new(big.Rat).Neg(maxFloat)) < 0:
		// Symmetric: the literal is below every finite float64.
		switch operator {
		case provider.FilterEqual, provider.FilterLess, provider.FilterLessEqual:
			return fold(false), nil
		case provider.FilterNotEqual, provider.FilterGreater, provider.FilterGreaterEqual:
			return fold(true), nil
		}
		return "", invalidQuery("filter", "invalid comparison")
	}
	nearest, _ := new(big.Float).SetPrec(256).SetRat(rational).Float64()
	exact := new(big.Rat).SetFloat64(nearest)
	// cmp reports where the nearest float64 sits relative to the exact
	// literal: 0 means the literal is representable, -1 that the nearest
	// rounded down, +1 that it rounded up.
	cmp := exact.Cmp(rational)
	// lo is the greatest float64 <= literal, hi the smallest float64 >= it.
	lo, hi := nearest, nearest
	if cmp > 0 {
		lo = math.Nextafter(nearest, math.Inf(-1))
	} else if cmp < 0 {
		hi = math.Nextafter(nearest, math.Inf(1))
	}
	switch operator {
	case provider.FilterEqual:
		if cmp != 0 {
			return fold(false), nil
		}
	case provider.FilterNotEqual:
		if cmp != 0 {
			return fold(true), nil
		}
	case provider.FilterLess:
		*args = append(*args, hi)
		return source + " < ?", nil
	case provider.FilterLessEqual:
		*args = append(*args, lo)
		return source + " <= ?", nil
	case provider.FilterGreater:
		*args = append(*args, lo)
		return source + " > ?", nil
	case provider.FilterGreaterEqual:
		*args = append(*args, hi)
		return source + " >= ?", nil
	default:
		return "", invalidQuery("filter", "invalid comparison")
	}
	*args = append(*args, nearest)
	direction, err := featureFilterOperator(operator)
	if err != nil {
		return "", err
	}
	return source + " " + direction + " ?", nil
}
