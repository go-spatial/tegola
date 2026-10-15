package postgis

import (
	"context"
	"math"
	"strings"
	"testing"

	"github.com/alexeydott/tegola"
	"github.com/alexeydott/tegola/basic"
	"github.com/alexeydott/tegola/dict"
	"github.com/alexeydott/tegola/internal/ttools"
	"github.com/alexeydott/tegola/provider"
	"github.com/alexeydott/tegola/provider/geometrycodec"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

// TestGenSQLRawFormat covers the R3-08 provider-level regression for the
// PostGIS SQL generation matrix: raw geometry columns (wkb / wkt / mos) must
// bypass ST_AsBinary and spatial predicates, while native format keeps them.
func TestGenSQLRawFormat(t *testing.T) {
	type tcase struct {
		name           string
		geometryFormat string
		expectedSQL    string
	}

	tests := []tcase{
		{
			name:           "native format uses bbox predicate",
			geometryFormat: "",
			expectedSQL:    `SELECT "id", ST_AsBinary("geom") AS "geom", "id" FROM tbl WHERE "geom" && !BBOX!`,
		},
		{
			name:           "mos format selects raw blob without bbox predicate",
			geometryFormat: geometrycodec.FormatMOS,
			expectedSQL:    `SELECT "id", "geom" AS "geom", "id" FROM tbl WHERE "geom" IS NOT NULL`,
		},
		{
			name:           "wkb format selects raw value without bbox predicate",
			geometryFormat: geometrycodec.FormatWKB,
			expectedSQL:    `SELECT "id", "geom" AS "geom", "id" FROM tbl WHERE "geom" IS NOT NULL`,
		},
		{
			name:           "wkt format selects raw value without bbox predicate",
			geometryFormat: geometrycodec.FormatWKT,
			expectedSQL:    `SELECT "id", "geom" AS "geom", "id" FROM tbl WHERE "geom" IS NOT NULL`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			l := &Layer{
				name:           "layer",
				geomField:      "geom",
				idField:        "id",
				srid:           3857,
				geometryFormat: tc.geometryFormat,
			}
			sql, err := genSQL(l, nil, "tbl", []string{"id", "geom"}, false, ProviderType)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if sql != tc.expectedSQL {
				t.Errorf("incorrect sql,\n Expected \n \t%v\n Got \n \t%v", tc.expectedSQL, sql)
			}
		})
	}
}

func TestReplaceTokens(t *testing.T) {
	type tcase struct {
		sql      string
		tile     provider.Tile
		expected string
		layer    Layer
	}

	fn := func(tc tcase) func(t *testing.T) {
		return func(t *testing.T) {
			sql, err := replaceTokens(tc.sql, &tc.layer, tc.tile, true)
			if err != nil {
				t.Errorf("unexpected error, Expected nil Got %v", err)
				return
			}

			if sql != tc.expected {
				t.Errorf("incorrect sql,\n Expected \n \t%v\n Got \n \t%v", tc.expected, sql)
				return
			}
		}
	}

	tests := map[string]tcase{
		"replace BBOX": {
			sql:      "SELECT * FROM foo WHERE geom && !BBOX!",
			layer:    Layer{srid: tegola.WebMercator},
			tile:     provider.NewTile(2, 1, 1, 64, tegola.WebMercator),
			expected: "SELECT * FROM foo WHERE geom && ST_MakeEnvelope(-10175297.20532266,-156543.03392804,156543.03392804,10175297.20532266,3857)",
		},
		"replace BBOX with != in query": {
			sql:      "SELECT * FROM foo WHERE geom && !BBOX! AND bar != 42",
			layer:    Layer{srid: tegola.WebMercator},
			tile:     provider.NewTile(2, 1, 1, 64, tegola.WebMercator),
			expected: "SELECT * FROM foo WHERE geom && ST_MakeEnvelope(-10175297.20532266,-156543.03392804,156543.03392804,10175297.20532266,3857) AND bar != 42",
		},
		"replace BBOX and ZOOM 1": {
			sql:      "SELECT id, scalerank=!ZOOM! FROM foo WHERE geom && !BBOX!",
			layer:    Layer{srid: tegola.WebMercator},
			tile:     provider.NewTile(2, 1, 1, 64, tegola.WebMercator),
			expected: "SELECT id, scalerank=2 FROM foo WHERE geom && ST_MakeEnvelope(-10175297.20532266,-156543.03392804,156543.03392804,10175297.20532266,3857)",
		},
		"replace BBOX and ZOOM 2": {
			sql:      "SELECT id, scalerank=!ZOOM! FROM foo WHERE geom && !BBOX!",
			layer:    Layer{srid: tegola.WebMercator},
			tile:     provider.NewTile(16, 11241, 26168, 64, tegola.WebMercator),
			expected: "SELECT id, scalerank=16 FROM foo WHERE geom && ST_MakeEnvelope(-13163688.81778845,4035254.04260249,-13163058.21230510,4035884.64808584,3857)",
		},
		"replace pixel_width/height and scale_denominator": {
			sql:      "SELECT id, !pixel_width! as width, !pixel_height! as height, !scale_denominator! as scale_denom FROM foo WHERE geom && !BBOX!",
			layer:    Layer{srid: tegola.WebMercator},
			tile:     provider.NewTile(11, 1070, 676, 64, tegola.WebMercator),
			expected: "SELECT id, 76.43702829 as width, 76.43702829 as height, 272989.38673277 as scale_denom FROM foo WHERE geom && ST_MakeEnvelope(899816.69697309,6789748.34851564,919996.07244038,6809927.72398292,3857)",
		},
	}

	for name, tc := range tests {
		t.Run(name, fn(tc))
	}
}

// TestReplaceTokensQuotesIdentifierTokens (audit P5-10) pins identifier
// quoting for !ID_FIELD!/!GEOM_FIELD! substitution in the postgis provider:
// plain and
// qualified names are quoted per part, values already wrapped in a complete
// quote pair pass through verbatim (backward compatibility), and hostile
// names cannot break out of the quoted identifier.
func TestReplaceTokensQuotesIdentifierTokens(t *testing.T) {
	tile := provider.NewTile(0, 0, 0, 0, tegola.WebMercator)

	cases := []struct {
		name      string
		idField   string
		geomField string
		want      string
	}{
		{"plain name", "fid", "geom", `SELECT "fid", "geom" FROM t`},
		{"qualified name", "t.fid", "db.t.geom", `SELECT "t"."fid", "db"."t"."geom" FROM t`},
		{"already double quoted", `"fid"`, `"t"."geom"`, `SELECT "fid", "t"."geom" FROM t`},
		{"already backtick quoted", "`fid`", "`geom`", "SELECT `fid`, `geom` FROM t"},
		{"hostile input", `we"ird`, `ge"om`, `SELECT "we""ird", "ge""om" FROM t`},
		{"unbalanced quote", `"fid`, `ge"om"`, `SELECT """fid", "ge""om""" FROM t`},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			sql, err := replaceTokens(
				"SELECT !ID_FIELD!, !GEOM_FIELD! FROM t",
				&Layer{idField: c.idField, geomField: c.geomField, srid: tegola.WebMercator},
				tile,
				false,
			)
			if err != nil {
				t.Fatalf("replaceTokens returned error: %v", err)
			}
			if sql != c.want {
				t.Fatalf("identifier tokens not quoted safely:\n got %q\nwant %q", sql, c.want)
			}
		})
	}
}

func TestReplaceTokensSyntheticSRIDUsesDatabaseSRIDZero(t *testing.T) {
	srid, err := basic.RegisterProj4Defn("+proj=merc +lon_0=0 +k_0=1 +x_0=0 +y_0=0 +ellps=WGS84 +datum=WGS84 +units=m +no_defs")
	if err != nil {
		t.Fatalf("registering synthetic CRS: %v", err)
	}

	sql, err := replaceTokens(
		"SELECT * FROM foo WHERE geom && !BBOX!",
		&Layer{srid: srid},
		provider.NewTile(2, 1, 1, 64, tegola.WebMercator),
		true,
	)
	if err != nil {
		t.Fatalf("replaceTokens returned error: %v", err)
	}
	if !strings.Contains(sql, ",0)") {
		t.Fatalf("synthetic CRS envelope must use database SRID 0, got %q", sql)
	}
}

func TestGenSQLSyntheticSRIDUsesDatabaseSRIDZero(t *testing.T) {
	srid, err := basic.RegisterProj4Defn("+proj=merc +lon_0=0 +k_0=1 +x_0=0 +y_0=0 +ellps=WGS84 +datum=WGS84 +units=m +no_defs")
	if err != nil {
		t.Fatalf("registering synthetic CRS: %v", err)
	}

	got, err := genSQL(
		&Layer{geomField: "geom", idField: "gid", srid: srid},
		nil,
		"roads",
		[]string{"gid", "geom"},
		true,
		ProviderType,
	)
	if err != nil {
		t.Fatalf("genSQL returned error: %v", err)
	}
	if !strings.Contains(got, `ST_SetSRID("geom",0) && !BBOX!`) {
		t.Fatalf("generated SQL does not isolate synthetic SRID: %q", got)
	}
}

// Regression for the flat-identifier quoting audit item: generated table SQL
// double-quotes the geometry identifier, so a mixed-case column created as
// "Geom" keeps its case-sensitive name instead of being folded to lowercase.
func TestGenSQLMixedCaseGeometryColumnIsQuoted(t *testing.T) {
	srid, err := basic.RegisterProj4Defn("+proj=merc +lon_0=0 +k_0=1 +x_0=0 +y_0=0 +ellps=WGS84 +datum=WGS84 +units=m +no_defs")
	if err != nil {
		t.Fatalf("registering synthetic CRS: %v", err)
	}

	got, err := genSQL(
		&Layer{geomField: "Geom", idField: "gid", srid: srid},
		nil,
		"roads",
		[]string{"gid", "Geom"},
		true,
		ProviderType,
	)
	if err != nil {
		t.Fatalf("genSQL returned error: %v", err)
	}
	if !strings.Contains(got, `ST_AsBinary("Geom") AS "Geom"`) {
		t.Fatalf("mixed-case geometry column must be quoted in the select clause: %q", got)
	}
	if !strings.Contains(got, `ST_SetSRID("Geom",0) && !BBOX!`) {
		t.Fatalf("mixed-case geometry column must be quoted in the synthetic SRID predicate: %q", got)
	}
}

func TestUppercaseTokens(t *testing.T) {
	type tcase struct {
		str      string
		expected string
	}

	fn := func(tc tcase) func(t *testing.T) {
		return func(t *testing.T) {
			out := uppercaseTokens(tc.str)

			if out != tc.expected {
				t.Errorf("expected \n \t%v\n out \n \t%v", tc.expected, out)
				return
			}
		}
	}

	tests := map[string]tcase{
		"uppercase tokens": {
			str:      "this !lower! case !STrInG! should uppercase !TOKENS!",
			expected: "this !LOWER! case !STRING! should uppercase !TOKENS!",
		},
		"no tokens": {
			str:      "no token",
			expected: "no token",
		},
		"empty string": {
			str:      "",
			expected: "",
		},
		"unclosed token": {
			str:      "unclosed !token",
			expected: "unclosed !token",
		},
	}

	for name, tc := range tests {
		t.Run(name, fn(tc))
	}
}

// SQL-context-aware token substitution (UPSTREAM 2.6): uppercaseTokens
// normalizes code-context tokens only; token text inside string literals,
// quoted identifiers and comments is left byte-identical.
func TestUppercaseTokensProtectedContexts(t *testing.T) {
	str := "SELECT '!x!', '!y!' -- !z!\nFROM t WHERE !zoom! = 1"
	want := "SELECT '!x!', '!y!' -- !z!\nFROM t WHERE !ZOOM! = 1"
	if out := uppercaseTokens(str); out != want {
		t.Fatalf("protected contexts must not be uppercased:\n got %q\nwant %q", out, want)
	}
}

func TestDecipherFields(t *testing.T) {
	ttools.ShouldSkip(t, TESTENV)

	ctx := t.Context()

	type tcase struct {
		sql              string
		expectedRowCount int
		expectedTags     map[string]any
	}

	uri := ttools.GetEnvDefault("PGURI", "postgres://postgres:postgres@localhost:5432/tegola?sslmode=disable")
	c := newDefaultConnector(dict.Dict{"uri": uri})

	pool, _, _, err := c.Connect(ctx)
	if err != nil {
		t.Fatalf("unable to connect: %s", err)
	}
	defer pool.Close()

	fn := func(tc tcase) func(t *testing.T) {
		return func(t *testing.T) {
			rows, err := pool.Query(context.Background(), tc.sql)
			if err != nil {
				t.Errorf("Error performing query: %v", err)
				return
			}
			defer rows.Close()

			var rowCount int
			for rows.Next() {
				geoFieldname := "geom"
				idFieldname := "id"
				descriptions := rows.FieldDescriptions()

				vals, err := rows.Values()
				if err != nil {
					t.Errorf("unexpected error reading row Values: %v", err)
					return
				}

				_, _, tags, err := decipherFields(
					context.TODO(),
					geoFieldname,
					idFieldname,
					nil,
					descriptions,
					vals,
				)
				if err != nil {
					t.Errorf("unexpected error running decipherFileds: %v", err)
					return
				}

				if len(tags) != len(tc.expectedTags) {
					t.Errorf(
						"got (%v): %#v, expected (%v): %#v",
						len(tags),
						tags,
						len(tc.expectedTags),
						tc.expectedTags,
					)
					return
				}

				for k, v := range tags {
					if tc.expectedTags[k] != v {
						t.Errorf(
							"missing or bad value for tag %v: %v (%T) != %v (%T)",
							k,
							v,
							v,
							tc.expectedTags[k],
							tc.expectedTags[k],
						)
						return
					}
				}

				rowCount++
			}
			if rows.Err() != nil {
				t.Errorf("unexpected err: %v", rows.Err())
				return
			}

			if rowCount != tc.expectedRowCount {
				t.Errorf("invalid row count. expected %v. got %v", tc.expectedRowCount, rowCount)
				return
			}
		}
	}

	tests := map[string]tcase{
		"tags with hstore": {
			sql:              "SELECT name, extra_text, extra_int, properties FROM test_tags_table WHERE id = 1;",
			expectedRowCount: 1,
			expectedTags: map[string]any{
				"name":        "Polygon A",
				"count":       "42",
				"enabled":     "true",
				"price":       "19.99",
				"description": "example polygon A",
				"extra_text":  "Additional info A",
				"extra_int":   int64(100),
			},
		},
		"tags with uuid": {
			sql:              "SELECT uuid FROM test_tags_table WHERE id = 1;",
			expectedRowCount: 1,
			expectedTags: map[string]any{
				"uuid": "550e8400-e29b-41d4-a716-446655440000",
			},
		},
		// NOTE: should they or should they not?
		"tags do not contain primary key": {
			sql:              "SELECT id FROM test_tags_table WHERE id = 2;",
			expectedRowCount: 1,
			expectedTags:     map[string]any{},
		},
	}

	for name, tc := range tests {
		t.Run(name, fn(tc))
	}
}

// TestDecipherFieldsGeometryValueTypes (audit P6-2) covers the geometry value
// types decipherFields must accept. pgx returns Go strings (not []byte) for
// text/varchar columns, so geometry_format="wkt" layers reading WKT from a
// text column broke with "unable to convert geometry field into bytes".
// Both []byte and string values must be accepted; anything else errors.
func TestDecipherFieldsGeometryValueTypes(t *testing.T) {
	ctx := t.Context()
	descriptions := []pgconn.FieldDescription{
		{Name: "geom", DataTypeOID: pgtype.TextOID},
		{Name: "id", DataTypeOID: pgtype.Int8OID},
	}
	const wkt = "POINT(1 2)"

	type tcase struct {
		geomValue any
		wantGeom  []byte
		wantErr   bool
	}

	tests := map[string]tcase{
		"geometry as []byte": {
			geomValue: []byte(wkt),
			wantGeom:  []byte(wkt),
		},
		"geometry as string": {
			geomValue: wkt,
			wantGeom:  []byte(wkt),
		},
		"geometry as unsupported type errors": {
			geomValue: int64(3),
			wantErr:   true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			gid, geom, _, err := decipherFields(
				ctx,
				"geom",
				"id",
				nil,
				descriptions,
				[]any{tc.geomValue, int64(7)},
			)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got none (geom=%q)", geom)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if gid != 7 {
				t.Errorf("gid: expected 7, got %v", gid)
			}
			if string(geom) != string(tc.wantGeom) {
				t.Errorf("geom: expected %q, got %q", tc.wantGeom, geom)
			}
		})
	}
}

// TestGIDRejectsInvalidIDs (audit P6-10) pins the feature-ID contract at the
// PostGIS decipherFields call site: negative, fractional and out-of-range IDs
// are rejected with an error instead of being wrapped (-1 → 2^64-1) or
// truncated (1.5 → 1). gId shares the rules with provider.ConvertFeatureID.
func TestGIDRejectsInvalidIDs(t *testing.T) {
	for _, val := range []any{
		float64(-1),
		float64(1.5),
		math.NaN(),
		math.Pow(2, 64),
		int64(-2),
		int32(-3),
		"-7",
		nil,
	} {
		got, err := gId(val)
		if err == nil {
			t.Fatalf("gId(%v (%T)) = %d, want a rejection error", val, val, got)
		}
		if got != 0 {
			t.Fatalf("gId(%v) = %d alongside error, want 0", val, got)
		}
	}

	if got, err := gId(float64(3)); err != nil || got != 3 {
		t.Fatalf("gId(3.0) = %d, %v, want 3, nil", got, err)
	}
	if got, err := gId("123"); err != nil || got != 123 {
		t.Fatalf("gId(%q) = %d, %v, want 123, nil", "123", got, err)
	}
}
