package postgis

import "testing"

// TestBuildMVTLayerSQLEscapesSingleQuotes (audit P5-8): the layer name,
// geometry-field name and feature-id name passed to ST_AsMVT are
// single-quoted SQL literals; embedded single quotes must be escaped by
// doubling so names like we'ird cannot break out of the literal (or inject
// SQL). Normal names keep the exact historical SQL shape.
func TestBuildMVTLayerSQLEscapesSingleQuotes(t *testing.T) {
	t.Parallel()

	tcs := []struct {
		name     string
		mvtName  string
		geomName string
		idName   string
		inner    string
		expected string
	}{
		{
			name:     "normal names pin historical shape",
			mvtName:  "mylayer",
			geomName: "geom",
			idName:   "fid",
			inner:    "SELECT fid, geom FROM t",
			expected: `(SELECT ST_AsMVT(q,'mylayer',4096,'geom','fid') AS data FROM (SELECT fid, geom FROM t) AS q)`,
		},
		{
			name:     "quote in layer name",
			mvtName:  "we'ird",
			geomName: "geom",
			idName:   "fid",
			inner:    "SELECT 1",
			expected: `(SELECT ST_AsMVT(q,'we''ird',4096,'geom','fid') AS data FROM (SELECT 1) AS q)`,
		},
		{
			name:     "quotes in geom and id names",
			mvtName:  "l",
			geomName: "ge'om",
			idName:   "it's",
			inner:    "SELECT 1",
			expected: `(SELECT ST_AsMVT(q,'l',4096,'ge''om','it''s') AS data FROM (SELECT 1) AS q)`,
		},
		{
			name:     "empty feature id stays NULL",
			mvtName:  "l",
			geomName: "geom",
			idName:   "",
			inner:    "SELECT 1",
			expected: `(SELECT ST_AsMVT(q,'l',4096,'geom',NULL) AS data FROM (SELECT 1) AS q)`,
		},
		{
			name:     "breakout attempt is neutralized",
			mvtName:  "x') AS data FROM (SELECT 1)--",
			geomName: "geom",
			idName:   "fid",
			inner:    "SELECT 1",
			expected: `(SELECT ST_AsMVT(q,'x'') AS data FROM (SELECT 1)--',4096,'geom','fid') AS data FROM (SELECT 1) AS q)`,
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := buildMVTLayerSQL(tc.mvtName, tc.geomName, tc.idName, tc.inner)
			if got != tc.expected {
				t.Errorf("buildMVTLayerSQL:\ngot:  %v\nwant: %v", got, tc.expected)
			}
		})
	}
}

// TestSQLStringLiteralEscapesSingleQuotes (audit P5-8): shared literal
// helper semantics.
func TestSQLStringLiteralEscapesSingleQuotes(t *testing.T) {
	t.Parallel()

	if got, want := sqlStringLiteral("we'ird"), "'we''ird'"; got != want {
		t.Errorf("we'ird: want %v, got %v", want, got)
	}
	if got, want := escapeSQLStringLiteral("it's"), "it''s"; got != want {
		t.Errorf("escape only: want %v, got %v", want, got)
	}
}
