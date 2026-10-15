package geometrycodec

import "testing"

func TestMySQLMetadataProbeSQL(t *testing.T) {
	tests := []struct {
		name, sql, want string
	}{
		{
			name: "view join keeps original projection",
			sql:  "SELECT c.id, c.geom, v.label FROM chambers c LEFT JOIN chamber_view v ON c.id=v.id WHERE 1=1",
			want: "SELECT c.id, c.geom, v.label FROM chambers c LEFT JOIN chamber_view v ON c.id=v.id WHERE 1=1\nLIMIT 0\n",
		},
		{name: "limit", sql: "select geom from t limit 16", want: "select geom from t limit 0"},
		{name: "offset comma", sql: "SELECT geom FROM t LIMIT 10, 16;", want: "SELECT geom FROM t LIMIT 10, 0;"},
		{name: "offset keyword", sql: "SELECT geom FROM t LIMIT 16 OFFSET 10", want: "SELECT geom FROM t LIMIT 0 OFFSET 10"},
		{
			name: "comments in limit",
			sql:  "SELECT geom FROM t LIMIT /* count */ 16 OFFSET /* offset */ 10; -- end",
			want: "SELECT geom FROM t LIMIT /* count */ 0 OFFSET /* offset */ 10; -- end",
		},
		{
			name: "protected limits and parentheses",
			sql:  "SELECT 'LIMIT 7)', `LIMIT`, \"(\" FROM t # LIMIT 10",
			want: "SELECT 'LIMIT 7)', `LIMIT`, \"(\" FROM t # LIMIT 10\nLIMIT 0\n",
		},
		{
			name: "nested limit",
			sql:  "SELECT geom FROM (SELECT geom FROM t LIMIT 16) x",
			want: "SELECT geom FROM (SELECT geom FROM t LIMIT 16) x\nLIMIT 0\n",
		},
		{
			name: "union",
			sql:  "SELECT geom FROM t UNION ALL SELECT geom FROM u LIMIT 16",
			want: "SELECT geom FROM t UNION ALL SELECT geom FROM u LIMIT 0",
		},
		{
			name: "union without limit",
			sql:  "SELECT geom FROM t UNION SELECT geom FROM u",
			want: "SELECT geom FROM t UNION SELECT geom FROM u\nLIMIT 0\n",
		},
		{
			name: "semicolon before trailing comment",
			sql:  "SELECT geom FROM t; -- LIMIT 16",
			want: "SELECT geom FROM t\nLIMIT 0\n; -- LIMIT 16",
		},
		{
			name: "identifier boundaries",
			sql:  "SELECT my_limit, $limit, limit$ FROM t",
			want: "SELECT my_limit, $limit, limit$ FROM t\nLIMIT 0\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := MySQL.MetadataProbeSQL(tt.sql); got != tt.want {
				t.Fatalf("got %q; want %q", got, tt.want)
			}
		})
	}
}

func TestMySQLMetadataProbeFallback(t *testing.T) {
	for _, sql := range []string{
		"SELECT geom FROM t FOR UPDATE;",
		"SELECT geom FROM t LIMIT 16 LOCK IN SHARE MODE",
		"SELECT geom FROM t /*! LIMIT 16 */",
		"SELECT geom FROM t /*M! LIMIT 16 */",
		"SELECT geom FROM t LIMIT ?",
		"SELECT geom FROM t LIMIT '16'",
		"SELECT geom FROM t LIMIT 16 FOR UPDATE SKIP LOCKED",
		"(SELECT geom FROM t LIMIT 16)",
		"WITH q AS (SELECT geom FROM t) SELECT geom FROM q",
		"SELECT geom FROM t; SELECT geom FROM u",
		"SELECT geom FROM (SELECT geom FROM t",
	} {
		t.Run(sql, func(t *testing.T) {
			if got, want := MySQL.MetadataProbeSQL(sql), MetadataProbeSQL(sql); got != want {
				t.Fatalf("got %q; want fallback %q", got, want)
			}
		})
	}
}

func TestMetadataProbeOtherDialects(t *testing.T) {
	const sql = "SELECT geom FROM t LIMIT 12"
	for _, dialect := range []SQLDialect{SQLite, PostgreSQL, HANA, Legacy} {
		if got, want := dialect.MetadataProbeSQL(sql), MetadataProbeSQL(sql); got != want {
			t.Fatalf("got %q; want %q", got, want)
		}
	}
}

func TestMySQLSampleProbeSQL(t *testing.T) {
	tests := []struct {
		name, sql, want string
	}{
		{
			name: "direct sample preserves columns",
			sql:  "SELECT id, geom, label FROM t LEFT JOIN v USING (id)",
			want: "SELECT id, geom, label FROM t LEFT JOIN v USING (id)\nLIMIT 16\n",
		},
		{name: "smaller limit", sql: "SELECT geom FROM t LIMIT 3", want: "SELECT geom FROM t LIMIT 3"},
		{name: "zero limit", sql: "SELECT geom FROM t LIMIT 0", want: "SELECT geom FROM t LIMIT 0"},
		{name: "cap limit", sql: "SELECT geom FROM t LIMIT 500", want: "SELECT geom FROM t LIMIT 16"},
		{
			name: "preserve comma offset",
			sql:  "SELECT geom FROM t LIMIT 200, 500;",
			want: "SELECT geom FROM t LIMIT 200, 16;",
		},
		{
			name: "preserve keyword offset",
			sql:  "SELECT geom FROM t LIMIT 500 OFFSET 200",
			want: "SELECT geom FROM t LIMIT 16 OFFSET 200",
		},
		{
			name: "preserve smaller offset limit",
			sql:  "SELECT geom FROM t LIMIT 3 OFFSET 200",
			want: "SELECT geom FROM t LIMIT 3 OFFSET 200",
		},
		{
			name: "preserve nested limit",
			sql:  "SELECT geom FROM (SELECT geom FROM t LIMIT 500) q LIMIT 100",
			want: "SELECT geom FROM (SELECT geom FROM t LIMIT 500) q LIMIT 16",
		},
		{
			name: "preserve comments and lock",
			sql:  "SELECT geom FROM t LIMIT /* sample */ 500 FOR UPDATE; -- end",
			want: WrapProbeSQL("SELECT geom FROM t LIMIT /* sample */ 500 FOR UPDATE; -- end"),
		},
		{
			name: "union sample",
			sql:  "SELECT geom FROM t UNION ALL SELECT geom FROM u LIMIT 200",
			want: "SELECT geom FROM t UNION ALL SELECT geom FROM u LIMIT 16",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := MySQL.SampleProbeSQL(tt.sql); got != tt.want {
				t.Fatalf("got %q; want %q", got, tt.want)
			}
		})
	}
}

func TestSampleProbeFallback(t *testing.T) {
	const sql = "SELECT geom FROM t /*! LIMIT 500 */"
	for _, dialect := range []SQLDialect{MySQL, SQLite, PostgreSQL, Legacy} {
		if got, want := dialect.SampleProbeSQL(sql), WrapProbeSQL(sql); got != want {
			t.Fatalf("got %q; want %q", got, want)
		}
	}
	if got, want := HANA.SampleProbeSQL(sql), WrapProbeSQLTopStyle(sql); got != want {
		t.Fatalf("got %q; want %q", got, want)
	}
}
