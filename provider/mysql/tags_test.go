package mysql

import (
	"testing"
	"time"
)

func TestCategoryFromDatabaseTypeName(t *testing.T) {
	tcs := []struct {
		dbType   string
		expected mysqlTypeCategory
	}{
		{"BIGINT", typeCategoryInt},
		{"UNSIGNED BIGINT", typeCategoryInt},
		{"TINYINT", typeCategoryInt},
		{"SMALLINT", typeCategoryInt},
		{"MEDIUMINT", typeCategoryInt},
		{"INT", typeCategoryInt},
		{"INTEGER", typeCategoryInt},
		{"INT UNSIGNED", typeCategoryInt},
		{"BOOL", typeCategoryInt},
		{"BOOLEAN", typeCategoryInt},
		{"BIT", typeCategoryInt},
		{"SERIAL", typeCategoryInt},
		{"DECIMAL", typeCategoryFloat},
		{"DECIMAL(10,2)", typeCategoryFloat},
		{"NUMERIC", typeCategoryFloat},
		{"FLOAT", typeCategoryFloat},
		{"DOUBLE", typeCategoryFloat},
		{"DOUBLE PRECISION", typeCategoryFloat},
		{"REAL", typeCategoryFloat},
		{"NEWDECIMAL", typeCategoryFloat},
		{"POINT", typeCategoryString},
		{"DATE", typeCategoryTime},
		{"DATETIME", typeCategoryTime},
		{"TIMESTAMP", typeCategoryTime},
		{"TIME", typeCategoryTime},
		{"YEAR", typeCategoryTime},
		{"TEXT", typeCategoryString},
		{"LONGTEXT", typeCategoryString},
		{"VARCHAR", typeCategoryString},
		{"CHAR", typeCategoryString},
		{"BLOB", typeCategoryString},
		{"JSON", typeCategoryString},
		{"", typeCategoryString},
		{"SOMETHINGUNKNOWN", typeCategoryString},
	}

	for _, tc := range tcs {
		if got := categoryFromDatabaseTypeName(tc.dbType); got != tc.expected {
			t.Errorf("categoryFromDatabaseTypeName(%q) = %v, expected %v", tc.dbType, got, tc.expected)
		}
	}
}

func TestConvertTagValue(t *testing.T) {
	now := time.Now()

	tcs := []struct {
		name     string
		val      interface{}
		cat      mysqlTypeCategory
		expected interface{}
		wantErr  bool
	}{
		{"nil stays nil", nil, typeCategoryInt, nil, false},
		{"typed int64 passes through", int64(42), typeCategoryInt, int64(42), false},
		{"typed float64 passes through", 3.14, typeCategoryFloat, 3.14, false},
		{"typed string passes through", "abc", typeCategoryString, "abc", false},
		{"time passes through", now, typeCategoryTime, now, false},
		{"bytes as string", []byte("hello"), typeCategoryString, "hello", false},
		{"bytes as int", []byte("123"), typeCategoryInt, int64(123), false},
		{"bytes as negative int", []byte("-7"), typeCategoryInt, int64(-7), false},
		{"bytes as huge uint", []byte("18446744073709551615"), typeCategoryInt, uint64(18446744073709551615), false},
		{"bytes as float", []byte("2.5"), typeCategoryFloat, 2.5, false},
		{"bytes as time string", []byte("2024-01-02 15:04:05"), typeCategoryTime, "2024-01-02T15:04:05Z", false},
		{"bad int bytes", []byte("notanint"), typeCategoryInt, nil, true},
		{"bad float bytes", []byte("notafloat"), typeCategoryFloat, nil, true},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			got, err := convertTagValue(tc.val, tc.cat)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got %v", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tc.cat == typeCategoryTime {
				// time.Time compared by identity is fine since passthrough
				if got != tc.expected {
					t.Errorf("got %v (%T), expected %v (%T)", got, got, tc.expected, tc.expected)
				}
				return
			}
			if got != tc.expected {
				t.Errorf("got %v (%T), expected %v (%T)", got, got, tc.expected, tc.expected)
			}
		})
	}
}

// TestConvertTagValueDecimalPrecision (audit P6-12) covers DECIMAL values
// that carry more precision than float64's 53-bit mantissa: ParseFloat used
// to silently round them, so a 20-digit DECIMAL lost digits beyond 2^53.
// Values that do not survive the float64 round trip are emitted as string
// tags with the exact text; everything else keeps the numeric tag.
func TestConvertTagValueDecimalPrecision(t *testing.T) {
	tcs := []struct {
		name     string
		val      string
		expected interface{}
	}{
		{
			"20-digit DECIMAL keeps exact digits as string",
			"12345678901234567890",
			"12345678901234567890",
		},
		{
			"20-digit DECIMAL with fraction keeps exact digits as string",
			"12345678901234567890.12",
			"12345678901234567890.12",
		},
		{
			"negative precision-losing DECIMAL is a string",
			"-12345678901234567890",
			"-12345678901234567890",
		},
		{
			"exact 2.5 stays a number",
			"2.5",
			2.5,
		},
		{
			"scale-only trailing zeros stay a number",
			"2.50",
			2.5,
		},
		{
			"binary-inexact but round-tripping 0.1 stays a number",
			"0.1",
			0.1,
		},
		{
			"exponent notation stays a number",
			"1e2",
			100.0,
		},
		{
			"2^53+1 loses precision and becomes a string",
			"9007199254740993",
			"9007199254740993",
		},
		{
			"integer within exact range stays a number",
			"42",
			42.0,
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			got, err := convertTagValue([]byte(tc.val), typeCategoryFloat)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.expected {
				t.Errorf("got %v (%T), expected %v (%T)", got, got, tc.expected, tc.expected)
			}
		})
	}
}
