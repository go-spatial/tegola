package featuresql

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"testing"
)

type testCatalog struct {
	dialect        Dialect
	columns        map[string]ColumnMetadata
	relation       RelationMetadata
	qualifierExact bool
	calls          map[string]int
	unstable       bool
}

func TestFeaturesqlParameterExpression(t *testing.T) {
	r := testResolve(t, "SELECT id,geom FROM items WHERE id=1 AND at IN (2,3)", testMetadata(SQLite))
	opts := testRenderOptions()
	opts.ParameterExpression = func(col ColumnMetadata, op, placeholder string) (string, error) {
		if col.Kind != IntegerColumn || (op != "=" && op != "IN") {
			return "", ErrUnsupported
		}
		return "CAST(" + placeholder + " AS DECIMAL(20,0))", nil
	}
	sql, args, next, err := CompileWhere(r, opts)
	want := `(("f"."id" = CAST($3 AS DECIMAL(20,0))) AND ("f"."at" IN (CAST($4 AS DECIMAL(20,0)),CAST($5 AS DECIMAL(20,0)))))`
	if err != nil || sql != want || next != 6 || !reflect.DeepEqual(args, []any{int64(1), int64(2), int64(3)}) {
		t.Fatal("cast parameter order", sql, args, next, err)
	}
	for _, cause := range []error{context.Canceled, context.DeadlineExceeded, ErrUnsupported} {
		failed := opts
		failed.ParameterExpression = func(ColumnMetadata, string, string) (string, error) { return "", cause }
		statement, values, n, err := CompileWhere(r, failed)
		if !errors.Is(err, cause) || statement != "" || len(values) != 0 || n != 3 {
			t.Fatal("expression error chain", err)
		}
	}
	empty := opts
	empty.ParameterExpression = func(ColumnMetadata, string, string) (string, error) { return " ", nil }
	if _, _, _, err := CompileWhere(r, empty); !errors.Is(err, ErrInvalid) {
		t.Fatal("empty expression", err)
	}
	bad := testResolve(t, "SELECT id,geom FROM items WHERE id='badtype'", testMetadata(SQLite))
	called := false
	guard := opts
	guard.ParameterExpression = func(ColumnMetadata, string, string) (string, error) { called = true; return "CAST($3 AS BIGINT)", nil }
	if _, _, _, err := CompileWhere(bad, guard); !errors.Is(err, ErrUnsupported) || called {
		t.Fatal("cast preceded exact binding", err)
	}
	var wg sync.WaitGroup
	issues := make(chan string)
	for range 16 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s, a, n, e := CompileWhere(r, opts)
			if e != nil || s != want || n != 6 || !reflect.DeepEqual(a, args) {
				issues <- "parallel expression differs"
			}
		}()
	}
	go func() { wg.Wait(); close(issues) }()
	for issue := range issues {
		t.Error(issue)
	}
	p, err := Parse("SELECT id,geom FROM items", SQLite)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Resolve(p, testMetadata(SQLite), ResolveOptions{"id", "geom", []string{"id"}}); err != nil {
		t.Fatal("valid ID instant temporal overlap rejected", err)
	}
}

func testMetadata(d Dialect) *testCatalog {
	return &testCatalog{dialect: d, relation: RelationMetadata{Schema: "public", Name: "items", CatalogIdentity: "table:7", PhysicalTable: true, Deterministic: true}, columns: map[string]ColumnMetadata{
		"id":   {Name: "id", Kind: IntegerColumn, Bits: 64, SingleColumnUnique: true, Nullable: true},
		"geom": {Name: "geom", Kind: BinaryColumn}, "name": {Name: "name", Kind: StringColumn, Collation: "binary"},
		"at": {Name: "at", Kind: IntegerColumn, Bits: 64}, "end": {Name: "end", Kind: IntegerColumn, Bits: 64},
		"flag": {Name: "flag", Kind: BooleanColumn}, "amount": {Name: "amount", Kind: DecimalColumn, Precision: 38, Scale: 10},
	}, calls: map[string]int{}}
}
func (c *testCatalog) ResolveRelation(n Name) (RelationMetadata, error) {
	if len(n.Parts()) == 0 {
		return RelationMetadata{}, ErrInvalid
	}
	return c.relation, nil
}
func (c *testCatalog) ResolveColumn(_ RelationMetadata, i Identifier) (ColumnMetadata, error) {
	key := i.Name()
	if !i.Quoted() {
		key = strings.ToLower(key)
	}
	col, ok := c.columns[key]
	if !ok {
		return ColumnMetadata{}, fmt.Errorf("catalog column unavailable: %w", ErrInvalid)
	}
	c.calls[key]++
	if c.unstable && c.calls[key] > 1 {
		col.Nullable = !col.Nullable
	}
	return col, nil
}
func (c *testCatalog) OutputKey(i Identifier) (string, error) {
	if i.Quoted() {
		return i.Name(), nil
	}
	if c.dialect == HANA {
		return strings.ToUpper(i.Name()), nil
	}
	return strings.ToLower(i.Name()), nil
}
func (c *testCatalog) QualifierKey(i Identifier) (string, error) {
	if c.qualifierExact || i.Quoted() {
		return i.Name(), nil
	}
	return strings.ToLower(i.Name()), nil
}
func testResolve(t *testing.T, text string, c *testCatalog) *ResolvedPlan {
	t.Helper()
	plan, err := Parse(text, c.dialect)
	if err != nil {
		t.Fatal(err)
	}
	id, geo := "id", "geom"
	if c.dialect == HANA {
		id, geo = "ID", "GEOM"
	}
	resolved, err := Resolve(plan, c, ResolveOptions{IdentityOutput: id, GeometryOutput: geo})
	if err != nil {
		t.Fatal(err)
	}
	return resolved
}
func testRenderOptions() RenderOptions {
	return RenderOptions{
		QuoteIdentifier: func(name string) string { return `"` + strings.ReplaceAll(name, `"`, `""`) + `"` },
		Placeholder:     func(n int) string { return fmt.Sprintf("$%d", n) },
		BindLiteral: func(col ColumnMetadata, _ string, lit Literal) (any, error) {
			switch col.Kind {
			case IntegerColumn:
				if lit.Kind() != NumberLiteral {
					return nil, ErrUnsupported
				}
				if col.Unsigned {
					return strconv.ParseUint(lit.Text(), 10, 64)
				}
				return strconv.ParseInt(lit.Text(), 10, 64)
			case StringColumn:
				if lit.Kind() != StringLiteral {
					return nil, ErrUnsupported
				}
				return lit.Text(), nil
			case BooleanColumn:
				if lit.Kind() != BooleanLiteral {
					return nil, ErrUnsupported
				}
				return lit.Text() == "true", nil
			case DecimalColumn:
				if lit.Kind() != NumberLiteral {
					return nil, ErrUnsupported
				}
				return lit.Text(), nil
			default:
				return nil, ErrUnsupported
			}
		}, FirstParameter: 3, Qualifier: "f",
	}
}

func TestFeaturesqlDialectAndTypedRendering(t *testing.T) {
	for _, d := range []Dialect{SQLite, MySQL, PostgreSQL, HANA} {
		t.Run(fmt.Sprint(d), func(t *testing.T) {
			c := testMetadata(d)
			resolved := testResolve(t, "SELECT f.id AS id, f.geom AS geom, f.name FROM public.items AS f WHERE NOT (flag = TRUE OR at < -1) AND name IN ('a''b','c') AND amount >= 18446744073709551615.0000000001", c)
			sql, args, next, err := CompileWhere(resolved, testRenderOptions())
			if err != nil {
				t.Fatal(err)
			}
			wantSQL := `((NOT (("f"."flag" = $3) OR ("f"."at" < $4))) AND ("f"."name" IN ($5,$6))) AND ("f"."amount" >= $7)`
			// Outer binary AND is explicit; static expected token order is independent
			// of traversal implementation and does not execute generated SQL.
			wantSQL = "(" + wantSQL + ")"
			if sql != wantSQL || next != 8 || !reflect.DeepEqual(args, []any{true, int64(-1), "a'b", "c", "18446744073709551615.0000000001"}) {
				t.Fatalf("SQL %s args%#v next%d", sql, args, next)
			}
		})
	}
}
func TestFeaturesqlPredicatesAndFullUnsigned(t *testing.T) {
	c := testMetadata(MySQL)
	id := c.columns["id"]
	id.Unsigned = true
	c.columns["id"] = id
	r := testResolve(t, "SELECT id,geom FROM items WHERE id != 18446744073709551615 AND name IS NOT NULL AND at NOT IN (-1,0,1)", c)
	sql, args, next, err := CompileWhere(r, testRenderOptions())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(sql, " != $3") || !strings.Contains(sql, " IS NOT NULL") || !strings.Contains(sql, " NOT IN ($4,$5,$6)") || next != 7 || !reflect.DeepEqual(args, []any{uint64(^uint64(0)), int64(-1), int64(0), int64(1)}) {
		t.Fatalf("%s %#v %d", sql, args, next)
	}
	for _, operator := range []string{"=", "<>", "!=", "<", "<=", ">", ">="} {
		r := testResolve(t, "SELECT id,geom FROM items WHERE at "+operator+" 1", testMetadata(SQLite))
		if _, _, _, err := CompileWhere(r, testRenderOptions()); err != nil {
			t.Fatal(operator, err)
		}
	}
	r = testResolve(t, "SELECT id,geom FROM items", testMetadata(SQLite))
	sql, args, next, err = CompileWhere(r, testRenderOptions())
	if err != nil || sql != "1=1" || len(args) != 0 || next != 3 {
		t.Fatalf("absent WHERE %s %v %d %v", sql, args, next, err)
	}
}
func TestFeaturesqlInvalidAndUnsupported(t *testing.T) {
	invalids := []string{"", "SELECT id, FROM items", "SELECT id,geom FROM items;", "SELECT id,geom FROM items --comment", "SELECT id,geom FROM items /*comment*/", "SELECT id,geom FROM items WHERE name='secret\\escape'", "SELECT id,geom FROM items WHERE id=?", "SELECT id,geom FROM items WHERE id=$1", "SELECT id,geom FROM items WHERE name=!BBOX!", "SELECT id,geom FROM items WHERE id=NULL", "SELECT id,geom FROM items WHERE id IN ()", "SELECT id,geom FROM items WHERE id IN (NULL)", "SELECT id,geom FROM items WHERE id=1e", "SELECT id,geom FROM items WHERE id=0x10", "SELECT id,geom FROM items WHERE id=1 garbage", "SELECT id,geom FROM a.b.c", "SELECT id,geom FROM items WHERE (id=1", "SELECT id,geom FROM items WHERE id=1)"}
	for _, input := range invalids {
		if _, err := Parse(input, PostgreSQL); err == nil || !errors.Is(err, ErrInvalid) {
			t.Errorf("invalid input classified incorrectly: %v", err)
		}
	}
	unsupportedInputs := []string{"WITH x AS (SELECT id FROM items) SELECT id FROM x", "SELECT DISTINCT id,geom FROM items", "SELECT * FROM items", "SELECT COALESCE(id,0),geom FROM items", "SELECT id,geom FROM items JOIN other ON items.id=other.id", "SELECT id,geom FROM items UNION SELECT id,geom FROM other", "SELECT id,geom FROM items ORDER BY id", "SELECT id,geom FROM items LIMIT 10", "SELECT id,geom FROM (SELECT id,geom FROM items) x", "SELECT id,geom FROM items WHERE id IN (SELECT id FROM other)"}
	for _, input := range unsupportedInputs {
		if _, err := Parse(input, PostgreSQL); err == nil || !errors.Is(err, ErrUnsupported) {
			t.Errorf("unsupported input classified incorrectly: %v", err)
		}
	}
	_, err := Parse("SELECT id,geom FROM items WHERE name='"+strings.Repeat("secret", 12000)+"'", SQLite)
	if err == nil || len(err.Error()) > 150 || strings.Contains(err.Error(), "secret") {
		t.Fatal("error echoed input", err)
	}
	for _, tc := range []struct {
		d     Dialect
		text  string
		valid bool
	}{{MySQL, "SELECT `id`,`geom` FROM `items`", true}, {MySQL, `SELECT "id",geom FROM items`, false}, {PostgreSQL, "SELECT `id`,geom FROM items", false}, {HANA, `SELECT "id","geom" FROM "items"`, true}, {SQLite, "SELECT `id`,geom FROM items", true}} {
		_, err := Parse(tc.text, tc.d)
		if (err == nil) != tc.valid {
			t.Fatal("dialect quoting", tc.d, err)
		}
	}
}

func TestFeaturesqlResourceBounds(t *testing.T) {
	base := "SELECT id,geom FROM items"
	for _, size := range []int{maxBytes, maxBytes + 1} {
		_, err := Parse(base+strings.Repeat(" ", size-len(base)), SQLite)
		if (err == nil) != (size == maxBytes) {
			t.Fatal("byte bound", size, err)
		}
	}
	for _, count := range []int{maxTokens, maxTokens + 1} {
		_, err := lex(strings.Repeat("a ", count), SQLite)
		if (err == nil) != (count == maxTokens) {
			t.Fatal("token bound", count, err)
		}
	}
	for _, count := range []int{maxProjections, maxProjections + 1} {
		projection := strings.TrimSuffix(strings.Repeat("id,", count), ",")
		_, err := Parse("SELECT "+projection+" FROM items", SQLite)
		if (err == nil) != (count == maxProjections) {
			t.Fatal("projection bound", count, err)
		}
	}
	for _, count := range []int{maxIn, maxIn + 1} {
		values := strings.TrimSuffix(strings.Repeat("1,", count), ",")
		_, err := Parse(base+" WHERE id IN ("+values+")", SQLite)
		if (err == nil) != (count == maxIn) {
			t.Fatal("IN bound", count, err)
		}
	}
	for _, count := range []int{31, 32} {
		for _, predicate := range []string{strings.Repeat("NOT ", count) + "id=1", strings.Repeat("(", count) + "id=1" + strings.Repeat(")", count)} {
			_, err := Parse(base+" WHERE "+predicate, SQLite)
			if (err == nil) != (count == 31) {
				t.Fatal("unary/group depth", count, err)
			}
		}
	}
	for _, count := range []int{32, 33} {
		for _, join := range []string{" AND ", " OR "} {
			predicate := strings.TrimSuffix(strings.Repeat("id=1"+join, count), join)
			_, err := Parse(base+" WHERE "+predicate, SQLite)
			if (err == nil) != (count == 32) {
				t.Fatal("binary depth", count, err)
			}
		}
	}
}

func TestFeaturesqlMacroAndNULRejection(t *testing.T) {
	for _, text := range []string{
		"SELECT id,geom FROM items WHERE name='!bbox!'",
		"SELECT id,geom FROM items WHERE name='!SCALE_DENOMINATOR!'",
		"SELECT id,geom FROM items WHERE name='contains\x00nul'",
		`SELECT "!ZOOM!",geom FROM items`,
	} {
		if _, err := Parse(text, SQLite); !errors.Is(err, ErrInvalid) {
			t.Fatal("macro or NUL admitted", err)
		}
	}
}

func TestFeaturesqlCatalogAdmission(t *testing.T) {
	for _, tc := range []struct {
		name    string
		sql     string
		change  func(*testCatalog)
		options ResolveOptions
		class   error
	}{
		{"nonunique", "SELECT id,geom FROM items", func(c *testCatalog) { v := c.columns["id"]; v.SingleColumnUnique = false; c.columns["id"] = v }, ResolveOptions{"id", "geom", nil}, ErrUnsupported},
		{"noninteger", "SELECT id,geom FROM items", func(c *testCatalog) { v := c.columns["id"]; v.Kind = DecimalColumn; c.columns["id"] = v }, ResolveOptions{"id", "geom", nil}, ErrUnsupported},
		{"generatedID", "SELECT id,geom FROM items", func(c *testCatalog) { v := c.columns["id"]; v.Generated = true; c.columns["id"] = v }, ResolveOptions{"id", "geom", nil}, ErrUnsupported},
		{"generatedGeometry", "SELECT id,geom FROM items", func(c *testCatalog) { v := c.columns["geom"]; v.Generated = true; c.columns["geom"] = v }, ResolveOptions{"id", "geom", nil}, ErrUnsupported},
		{"view", "SELECT id,geom FROM items", func(c *testCatalog) { c.relation.PhysicalTable = false }, ResolveOptions{"id", "geom", nil}, ErrUnsupported},
		{"unfrozen", "SELECT id,geom FROM items", func(c *testCatalog) { c.relation.CatalogIdentity = "" }, ResolveOptions{"id", "geom", nil}, ErrInvalid},
		{"wrongQualifier", "SELECT x.id,geom FROM items f", func(*testCatalog) {}, ResolveOptions{"id", "geom", nil}, ErrInvalid},
		{"duplicateOutput", "SELECT id,geom,name AS ID FROM items", func(*testCatalog) {}, ResolveOptions{"id", "geom", nil}, ErrInvalid},
		{"missingID", "SELECT geom FROM items", func(*testCatalog) {}, ResolveOptions{"id", "geom", nil}, ErrInvalid},
		{"missingTime", "SELECT id,geom FROM items", func(*testCatalog) {}, ResolveOptions{"id", "geom", []string{"at"}}, ErrInvalid},
		{"timeSamePhysical", "SELECT id,geom,at AS a,at AS b FROM items", func(*testCatalog) {}, ResolveOptions{"id", "geom", []string{"a", "b"}}, ErrInvalid},
		{"generatedTime", "SELECT id,geom,at FROM items", func(c *testCatalog) { v := c.columns["at"]; v.Generated = true; c.columns["at"] = v }, ResolveOptions{"id", "geom", []string{"at"}}, ErrUnsupported},
		{"inconsistentLineage", "SELECT id,geom FROM items WHERE id=1", func(c *testCatalog) { c.unstable = true }, ResolveOptions{"id", "geom", nil}, ErrInvalid},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := testMetadata(SQLite)
			tc.change(c)
			p, err := Parse(tc.sql, SQLite)
			if err != nil {
				t.Fatal(err)
			}
			_, err = Resolve(p, c, tc.options)
			if !errors.Is(err, tc.class) {
				t.Fatal("admission classification", err)
			}
		})
	}
	for _, nullable := range []bool{false, true} {
		c := testMetadata(SQLite)
		v := c.columns["id"]
		v.Nullable = nullable
		c.columns["id"] = v
		r := testResolve(t, "SELECT id,geom,at,end FROM items", c)
		if r.Identity().Column.Nullable != nullable {
			t.Fatal("nullable metadata lost")
		}
	}
	c := testMetadata(MySQL)
	c.qualifierExact = true
	p, err := Parse("SELECT F.id, f.geom FROM items f", MySQL)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Resolve(p, c, ResolveOptions{"id", "geom", nil}); !errors.Is(err, ErrInvalid) {
		t.Fatal("qualifiers incorrectly used output-label folding", err)
	}
	p, err = Parse(`SELECT "id" AS "Identity",geom FROM items`, PostgreSQL)
	if err != nil {
		t.Fatal(err)
	}
	r, err := Resolve(p, testMetadata(PostgreSQL), ResolveOptions{"Identity", "geom", nil})
	if err != nil || r.Identity().Output != "Identity" {
		t.Fatal("quoted output lost", err)
	}
}

func TestFeaturesqlOwnershipAndConcurrentRender(t *testing.T) {
	p, err := Parse("SELECT id,geom,name FROM items WHERE id=1 AND name='value'", SQLite)
	if err != nil {
		t.Fatal(err)
	}
	parts := p.Relation().Parts()
	parts[0].name = "wrong"
	projections := p.Projections()
	projections[0].source.name = "wrong"
	predicate, _ := p.Predicate()
	children := predicate.Children()
	children[0].column.name = "wrong"
	literals := children[1].Literals()
	literals[0].text = "wrong"
	c := testMetadata(SQLite)
	r, err := Resolve(p, c, ResolveOptions{"id", "geom", nil})
	if err != nil {
		t.Fatal(err)
	}
	rp := r.Projections()
	rp[0].Column.Name = "wrong"
	col := c.columns["id"]
	col.Name = "wrong"
	c.columns["id"] = col
	want, args, next, err := CompileWhere(r, testRenderOptions())
	if err != nil {
		t.Fatal(err)
	}
	if want != `(("f"."id" = $3) AND ("f"."name" = $4))` || !reflect.DeepEqual(args, []any{int64(1), "value"}) || next != 5 {
		t.Fatal("detached state altered", want, args, next)
	}
	var wg sync.WaitGroup
	issues := make(chan string)
	for range 16 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			actual, values, n, err := CompileWhere(r, testRenderOptions())
			if err != nil || actual != want || n != next || !reflect.DeepEqual(values, args) {
				issues <- "concurrent render differs"
			}
		}()
	}
	go func() { wg.Wait(); close(issues) }()
	for issue := range issues {
		t.Error(issue)
	}
}

func TestFeaturesqlZeroAndRenderFailures(t *testing.T) {
	var nilCatalog *testCatalog
	for _, plan := range []*Plan{nil, {}} {
		if _, err := Resolve(plan, testMetadata(SQLite), ResolveOptions{"id", "geom", nil}); !errors.Is(err, ErrInvalid) {
			t.Fatal("zero parsed plan", err)
		}
	}
	p, err := Parse("SELECT id,geom FROM items", SQLite)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Resolve(p, nilCatalog, ResolveOptions{"id", "geom", nil}); !errors.Is(err, ErrInvalid) {
		t.Fatal("typed nil catalog", err)
	}
	for _, plan := range []*ResolvedPlan{nil, {}} {
		if _, _, _, err := CompileWhere(plan, testRenderOptions()); !errors.Is(err, ErrInvalid) {
			t.Fatal("zero resolved plan", err)
		}
	}
	r := testResolve(t, "SELECT id,geom FROM items WHERE id=1", testMetadata(SQLite))
	cause := errors.New("binder failed")
	opts := testRenderOptions()
	opts.BindLiteral = func(ColumnMetadata, string, Literal) (any, error) { return nil, cause }
	sql, args, next, err := CompileWhere(r, opts)
	if !errors.Is(err, cause) || sql != "" || len(args) != 0 || next != 3 {
		t.Fatal("binder chain/partial result", err)
	}
	for _, change := range []func(*RenderOptions){func(o *RenderOptions) { o.FirstParameter = 0 }, func(o *RenderOptions) { o.FirstParameter = int(^uint(0) >> 1) }, func(o *RenderOptions) { o.QuoteIdentifier = nil }, func(o *RenderOptions) { o.Placeholder = func(int) string { return "" } }, func(o *RenderOptions) { o.BindLiteral = nil }} {
		opts := testRenderOptions()
		change(&opts)
		if _, _, _, err := CompileWhere(r, opts); !errors.Is(err, ErrInvalid) {
			t.Fatal("render options", err)
		}
	}
	r = testResolve(t, "SELECT id,geom FROM items WHERE id='badtype'", testMetadata(SQLite))
	if _, _, _, err := CompileWhere(r, testRenderOptions()); !errors.Is(err, ErrUnsupported) {
		t.Fatal("type combination accepted", err)
	}
}
