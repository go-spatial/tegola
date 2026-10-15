package hana

import (
	"bytes"
	"context"
	"database/sql"
	"log/slog"
	"strings"
	"testing"
)

// captureHanaWarns runs f with the default slog logger replaced by one
// writing WARN+ records into a buffer and returns the captured text.
// internal/log's Warnf routes through slog's default logger.
func captureHanaWarns(t *testing.T, f func()) string {
	t.Helper()
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn})))
	defer slog.SetDefault(prev)
	f()
	return buf.String()
}

// TestReadRowValuesExtractsNumericID pins the audit P6-10/P6-11 hana part:
// numeric id columns must feed the feature id (previously they fell into
// the tags map and every feature got duplicate ID 0), and the id value is
// not duplicated into the tags.
func TestReadRowValuesExtractsNumericID(t *testing.T) {
	l := &Layer{name: "lyr"}
	descriptions := []FieldDescription{
		{dataType: DtBigint, name: "gid", isFeatureId: true},
		{dataType: DtNVarchar, name: "name"},
	}
	rowValues := []interface{}{
		&sql.NullInt64{Int64: 42, Valid: true},
		&sql.NullString{String: "value", Valid: true},
	}

	gid, _, tags, err := readRowValues(context.Background(), l, descriptions, rowValues)
	if err != nil {
		t.Fatalf("readRowValues errored = %v", err)
	}
	if gid != 42 {
		t.Errorf("gid = %d, expected 42 (numeric id columns feed the feature id)", gid)
	}
	if _, dup := tags["gid"]; dup {
		t.Errorf("tags must not duplicate the id column, got %v", tags)
	}
	if tags["name"] != "value" {
		t.Errorf("tags = %v, expected name=value kept", tags)
	}
}

// TestReadRowValuesStringID sanity: a string id column keeps feeding the
// feature id and never lands in tags.
func TestReadRowValuesStringID(t *testing.T) {
	l := &Layer{name: "lyr"}
	descriptions := []FieldDescription{
		{dataType: DtNVarchar, name: "gid", isFeatureId: true},
		{dataType: DtNVarchar, name: "name"},
	}
	rowValues := []interface{}{
		&sql.NullString{String: "42", Valid: true},
		&sql.NullString{String: "value", Valid: true},
	}

	gid, _, tags, err := readRowValues(context.Background(), l, descriptions, rowValues)
	if err != nil {
		t.Fatalf("readRowValues errored = %v", err)
	}
	if gid != 42 {
		t.Errorf("gid = %d, expected 42", gid)
	}
	if _, dup := tags["gid"]; dup {
		t.Errorf("tags must not duplicate the id column, got %v", tags)
	}
}

// TestReadRowValuesRepeatedIDOccurrenceIsTag pins the postgis provider
// contract "the id has to be parsed once but it can also be a tag": the
// row carries the id column twice (genSQL always appends it — see
// TestGenSQLIdFieldAlsoTag), the first occurrence feeds the feature id
// and the second occurrence falls through into the tags map. This is what
// makes an explicitly configured id field appear in both the feature id
// and the tags ("tablename query with fields and id as field").
func TestReadRowValuesRepeatedIDOccurrenceIsTag(t *testing.T) {
	l := &Layer{name: "lyr"}
	descriptions := []FieldDescription{
		{dataType: DtBigint, name: "id", isFeatureId: true},
		{dataType: DtNVarchar, name: "scalerank"},
		{dataType: DtBigint, name: "id"},
	}
	rowValues := []interface{}{
		&sql.NullInt64{Int64: 42, Valid: true},
		&sql.NullString{String: "X", Valid: true},
		&sql.NullInt64{Int64: 42, Valid: true},
	}

	gid, _, tags, err := readRowValues(context.Background(), l, descriptions, rowValues)
	if err != nil {
		t.Fatalf("readRowValues errored = %v", err)
	}
	if gid != 42 {
		t.Errorf("gid = %d, expected 42 (first id occurrence feeds the feature id)", gid)
	}
	if tags["id"] != int64(42) {
		t.Errorf("tags[id] = %v, expected 42 (second id occurrence becomes a tag)", tags["id"])
	}
	if tags["scalerank"] != "X" {
		t.Errorf("tags = %v, expected scalerank=X kept", tags)
	}
}

// TestReadRowValuesNullIDSkipsRow pins the audit P6-11 hana part: a NULL
// feature id must not silently become duplicate ID 0 - the row is skipped
// (empty result, caller's len(geom)==0 continue seam) with a warning.
// Pre-fix: the row was emitted with ID 0 and no warning.
func TestReadRowValuesNullIDSkipsRow(t *testing.T) {
	l := &Layer{name: "lyr"}
	for name, idVal := range map[string]interface{}{
		"null string id":   &sql.NullString{Valid: false},
		"null bigint id":   &sql.NullInt64{Valid: false},
		"null smallint id": &sql.NullInt16{Valid: false},
	} {
		t.Run(name, func(t *testing.T) {
			descriptions := []FieldDescription{
				{dataType: DtBigint, name: "gid", isFeatureId: true},
				{dataType: DtNVarchar, name: "name"},
			}
			rowValues := []interface{}{
				idVal,
				&sql.NullString{String: "value", Valid: true},
			}

			var (
				gid   uint64
				geom  []byte
				tags  map[string]interface{}
				rerr  error
				warns string
			)
			warns = captureHanaWarns(t, func() {
				gid, geom, tags, rerr = readRowValues(context.Background(), l, descriptions, rowValues)
			})
			if rerr != nil {
				t.Fatalf("readRowValues errored = %v", rerr)
			}
			if gid != 0 || geom != nil || tags != nil {
				t.Errorf("NULL id row must be skipped entirely (0, nil, nil), got (%d, %v, %v)", gid, geom, tags)
			}
			if !strings.Contains(warns, "NULL id field") || !strings.Contains(warns, "skipping feature") {
				t.Errorf("expected NULL-id skip warning, got: %s", warns)
			}
		})
	}
}

// TestConvertToUInt64Delegation pins the audit P6-10 conversion contract:
// values are converted through the shared provider.ConvertFeatureID (byte
// ids included), sql.NullString and int16 scan-target shapes are adapted,
// and NULL/garbage values error instead of becoming ID 0.
func TestConvertToUInt64Delegation(t *testing.T) {
	if got, err := convertToUInt64([]byte("42")); err != nil || got != 42 {
		t.Errorf("convertToUInt64([]byte(\"42\")) = %d, %v, expected 42, nil", got, err)
	}
	if got, err := convertToUInt64(sql.NullString{String: "7", Valid: true}); err != nil || got != 7 {
		t.Errorf("convertToUInt64(NullString 7) = %d, %v, expected 7, nil", got, err)
	}
	if got, err := convertToUInt64(int16(9)); err != nil || got != 9 {
		t.Errorf("convertToUInt64(int16 9) = %d, %v, expected 9, nil", got, err)
	}
	if _, err := convertToUInt64(sql.NullString{Valid: false}); err == nil {
		t.Error("convertToUInt64(NULL NullString) must error, got nil")
	}
	if _, err := convertToUInt64("abc"); err == nil {
		t.Error("convertToUInt64(\"abc\") must error, got nil")
	}
}
