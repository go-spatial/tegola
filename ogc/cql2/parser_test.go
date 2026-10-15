package cql2

import (
	"errors"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/alexeydott/tegola/provider"
)

func TestParseBasicLiteralValues(t *testing.T) {
	cases := []struct {
		name, input, property, text string
		kind                        provider.FilterScalarType
		operator                    provider.FilterCompareOperator
	}{
		{name: "string", input: "name = 'Alpha'", property: "name", text: "Alpha", kind: provider.FilterString, operator: provider.FilterEqual},
		{name: "empty", input: "name <> ''", property: "name", text: "", kind: provider.FilterString, operator: provider.FilterNotEqual},
		{name: "double quote", input: `name = 'O''Brien'`, property: "name", text: "O'Brien", kind: provider.FilterString, operator: provider.FilterEqual},
		{name: "backslash quote", input: `name = 'O\'Brien'`, property: "name", text: "O'Brien", kind: provider.FilterString, operator: provider.FilterEqual},
		{name: "control escapes", input: `name = '\a\b\t\n\v\f\r\\'`, property: "name", text: "\a\b\t\n\v\f\r\\", kind: provider.FilterString, operator: provider.FilterEqual},
		{name: "injection is literal", input: `name = 'x'' OR TRUE -- ; /* */'`, property: "name", text: "x' OR TRUE -- ; /* */", kind: provider.FilterString, operator: provider.FilterEqual},
		{name: "unicode", input: `имя = 'é😀'`, property: "имя", text: "é😀", kind: provider.FilterString, operator: provider.FilterEqual},
		{name: "quoted keyword", input: `"AND" = 1`, property: "AND", text: "1", kind: provider.FilterNumber, operator: provider.FilterEqual},
		{name: "dot alias", input: `source.public = 1`, property: "source.public", text: "1", kind: provider.FilterNumber, operator: provider.FilterEqual},
		{name: "nonkeyword prefix", input: `S_status = 1`, property: "S_status", text: "1", kind: provider.FilterNumber, operator: provider.FilterEqual},
		{name: "full uint64", input: `n_uint > 18446744073709551614`, property: "n_uint", text: "18446744073709551614", kind: provider.FilterNumber, operator: provider.FilterGreater},
		{name: "exact decimal", input: `n <= -1.2500e+2`, property: "n", text: "-1.2500e+2", kind: provider.FilterNumber, operator: provider.FilterLessEqual},
		{name: "negative zero", input: `n >= -0`, property: "n", text: "-0", kind: provider.FilterNumber, operator: provider.FilterGreaterEqual},
		{name: "leading decimal", input: `n < +.5`, property: "n", text: "+.5", kind: provider.FilterNumber, operator: provider.FilterLess},
		{name: "boolean", input: `b = TrUe`, property: "b", text: "true", kind: provider.FilterBoolean, operator: provider.FilterEqual},
		{name: "date", input: `d = DATE('2024-02-29')`, property: "d", text: "2024-02-29", kind: provider.FilterDate, operator: provider.FilterEqual},
		{name: "timestamp tail", input: `t = TIMESTAMP('2026-10-02T00:00:00.0000000001Z')`, property: "t", text: "2026-10-02T00:00:00.0000000001Z", kind: provider.FilterTimestamp, operator: provider.FilterEqual},
		{name: "leap timestamp", input: `t = TIMESTAMP('2016-12-31T23:59:60.5Z')`, property: "t", text: "2016-12-31T23:59:60.5Z", kind: provider.FilterTimestamp, operator: provider.FilterEqual},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			expression, err := Parse(tc.input)
			if err != nil {
				t.Fatal(err)
			}
			root := expression.Root()
			if root.Kind != provider.FilterCompare || root.Property != tc.property || root.Operator != tc.operator || root.Literal.Type() != tc.kind || root.Literal.Text() != tc.text {
				t.Fatalf("unexpected parsed descriptor: %#v", root)
			}
			if tc.name == "timestamp tail" {
				instant, ok := root.Literal.Instant()
				if !ok || instant.SubNanosecond != "1" || instant.LeapSecond {
					t.Fatal("tail lost")
				}
			}
			if tc.name == "leap timestamp" {
				instant, ok := root.Literal.Instant()
				if !ok || !instant.LeapSecond {
					t.Fatal("leap flag lost")
				}
			}
		})
	}
}
func TestParseLogicalPrecedenceAndNull(t *testing.T) {
	expression, err := Parse(`NOT n IS NULL OR n >= 7 AND (b = true OR FALSE)`)
	if err != nil {
		t.Fatal(err)
	}
	root := expression.Root()
	if root.Kind != provider.FilterOr || root.Children[0].Kind != provider.FilterNot || root.Children[0].Children[0].Kind != provider.FilterIsNull || root.Children[1].Kind != provider.FilterAnd || root.Children[1].Children[1].Kind != provider.FilterOr {
		t.Fatal("logical precedence/grouping lost")
	}
	for _, tc := range []struct {
		input string
		kind  provider.FilterKind
		value bool
	}{{"TRUE", provider.FilterBooleanConstant, true}, {"false", provider.FilterBooleanConstant, false}, {`"NULL" IS NOT NULL`, provider.FilterIsNotNull, false}} {
		e, err := Parse(tc.input)
		if err != nil {
			t.Fatal(err)
		}
		if e.Root().Kind != tc.kind || e.Root().Boolean != tc.value {
			t.Fatal("boolean/null primary lost")
		}
	}
	copy := expression.Root()
	copy.Children[0].Children[0].Property = "changed"
	if expression.Root().Children[0].Children[0].Property != "n" {
		t.Fatal("root retained mutable caller tree")
	}
}
func TestParseInvalidAndUnsupported(t *testing.T) {
	invalidInputs := []string{"", " ", "n = NULL", "n IS 1", "n =", "n = 'unterminated", "n = '\\q'", `"" = 1`, `"bad name" = 1`, "n = 1 trailing", "n=1; DROP TABLE x", "n=1 -- comment", "n=1 /* comment */", "n != 1", "n = 1e+", "n = 1.2.3", "n = NaN", "n = 'x\x00'", "n = '\xff'", `d=DATE('2023-02-29')`, `t=TIMESTAMP('2026-10-02T00:00:00+00:00')`, `t=TIMESTAMP('2026-10-02T00:00:00z')`, `t=TIMESTAMP('2016-12-30T23:59:60Z')`, "(TRUE", "TRUE)", "TRUE AND"}
	for _, input := range invalidInputs {
		t.Run("invalid", func(t *testing.T) {
			_, err := Parse(input)
			var invalid provider.InvalidFeatureQueryError
			if err == nil || !errors.As(err, &invalid) || invalid.Field != "filter" {
				t.Fatalf("expected typed invalid for %q: %v", input, err)
			}
		})
	}
	unsupportedInputs := []string{`n LIKE 'x'`, `n IN (1,2)`, `n BETWEEN 1 AND 2`, `n = other`, `1 = n`, `'x' = n`, `TRUE = FALSE`, `S_INTERSECTS(geom,POINT(0 0))`, `n = abs(1)`, `n * 2 = 1`, `n = 1 + 2`, `n = 1-2`, `n NOT IN (1)`, `CASEI(s) = 'x'`}
	for _, input := range unsupportedInputs {
		t.Run("unsupported", func(t *testing.T) {
			_, err := Parse(input)
			var invalid provider.InvalidFeatureQueryError
			if !errors.As(err, &invalid) || !errors.Is(err, provider.ErrUnsupported) {
				t.Fatalf("expected joined unsupported/clienterror for %q: %v", input, err)
			}
		})
	}
	secret := strings.Repeat("fictional-sensitive-value", 600)
	_, err := Parse("n = '" + secret)
	if err == nil || strings.Contains(err.Error(), "fictional-sensitive-value") || len(err.Error()) > 256 {
		t.Fatal("request echoed or unbounded error")
	}
}
func balancedBoolean(leaves int) string {
	if leaves == 1 {
		return "TRUE"
	}
	half := leaves / 2
	return "(" + balancedBoolean(half) + " AND " + balancedBoolean(leaves-half) + ")"
}
func TestParseResourceBoundaries(t *testing.T) {
	valid := []string{
		strings.Repeat(" ", maxInputBytes-4) + "TRUE",
		strings.Repeat("NOT ", 31) + "TRUE",
		strings.Repeat("(", 31) + "TRUE" + strings.Repeat(")", 31),
		"n = '" + strings.Repeat("x", maxLiteralBytes) + "'",
		strings.Repeat("x", maxPropertyBytes) + " = 1",
		"n = " + strings.Repeat("1", 1024),
		"n = 1e4096",
		"NOT " + balancedBoolean(2048),
	}
	for _, input := range valid {
		if _, err := Parse(input); err != nil {
			t.Fatalf("within bound rejected len=%d: %v", len(input), err)
		}
	}
	invalid := []string{
		strings.Repeat(" ", maxInputBytes-3) + "TRUE",
		strings.Repeat("NOT ", 32) + "TRUE",
		strings.Repeat("(", 32) + "TRUE" + strings.Repeat(")", 32),
		"n = '" + strings.Repeat("x", maxLiteralBytes+1) + "'",
		strings.Repeat("x", maxPropertyBytes+1) + " = 1",
		"n = " + strings.Repeat("1", 1025),
		"n = 1e4097",
		"NOT NOT " + balancedBoolean(2048),
		strings.Repeat("TRUE AND ", maxTokens) + "TRUE",
	}
	for _, input := range invalid {
		if _, err := Parse(input); err == nil {
			t.Fatalf("over bound accepted len=%d", len(input))
		}
	}
	// Repeated literal bytes are bounded even when every individual token fits.
	input := "n='" + strings.Repeat("x", 16384) + "' AND n='" + strings.Repeat("x", 16384) + "' AND n='" + strings.Repeat("x", 16384) + "' AND n='" + strings.Repeat("x", 16384) + "'"
	if _, err := Parse(input); err == nil {
		t.Fatal("total input/budget overflow accepted")
	}
}
func TestParseConcurrentIndependentCalls(t *testing.T) {
	input := `NOT n IS NULL AND (s = 'O''Brien' OR b = TRUE)`
	expected, err := Parse(input)
	if err != nil {
		t.Fatal(err)
	}
	var group sync.WaitGroup
	issues := make(chan error, 8)
	for range 8 {
		group.Add(1)
		go func() {
			defer group.Done()
			for range 20 {
				value, err := Parse(input)
				if err != nil {
					issues <- err
					return
				}
				if !reflect.DeepEqual(value.Root(), expected.Root()) {
					issues <- errors.New("concurrent parse changed tree")
					return
				}
				copy := value.Root()
				copy.Children[0].Kind = provider.FilterBooleanConstant
			}
		}()
	}
	group.Wait()
	close(issues)
	for err := range issues {
		t.Error(err)
	}
}
func FuzzParse(f *testing.F) {
	for _, seed := range []string{"TRUE", "NOT n IS NULL", `name = 'O\'Brien'`, `"AND" = 1`, `t = TIMESTAMP('2016-12-31T23:59:60.0000000001Z')`, "n = 1e4096", "n=1;DROP", "n=1 -- comment", "n = NULL", "(TRUE AND FALSE)", "\xff", "n='\\q'", strings.Repeat("NOT ", 32) + "TRUE"} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, input string) {
		expression, err := Parse(input)
		if err != nil {
			var invalid provider.InvalidFeatureQueryError
			if !errors.As(err, &invalid) || len(err.Error()) > 512 {
				t.Fatal("untyped/unbounded error")
			}
			return
		}
		if err := expression.Validate(); err != nil {
			t.Fatal("accepted expression invalid", err)
		}
		again, err := Parse(input)
		if err != nil || !reflect.DeepEqual(expression.Root(), again.Root()) {
			t.Fatal("nondeterministic parse")
		}
	})
}

func TestASCIIKeywordsPreserveUnicodeAliases(t *testing.T) {
	for _, name := range []string{"falſe", "Iſ", "LIKE", "ſ_INTERSECTS", "TRUſ"} {
		value, err := Parse(name + " = 1")
		if err != nil {
			t.Fatal("valid Unicode alias rejected", err)
		}
		if value.Root().Kind != provider.FilterCompare || value.Root().Property != name {
			t.Fatal("Unicode alias reinterpreted as keyword")
		}
	}
	for _, input := range []string{"falſe", "TRUſ", "n Iſ NULL"} {
		if _, err := Parse(input); err == nil {
			t.Fatal("Unicode spelling reinterpreted as ASCII keyword")
		}
	}
	value, err := Parse("TRUe")
	if err != nil || value.Root().Kind != provider.FilterBooleanConstant || !value.Root().Boolean {
		t.Fatal("ASCII case insensitive keyword lost", err)
	}
	if _, err := Parse("\u1fff = 1"); err == nil {
		t.Fatal("identifier outside AnnexB range admitted")
	}
	if _, err := Parse("\u1ffe = 1"); err != nil {
		t.Fatal("identifier AnnexB boundary rejected", err)
	}
}
