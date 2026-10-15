package gpkg

import (
	"strings"
	"testing"

	"github.com/alexeydott/geom"
	"github.com/alexeydott/tegola"
	"github.com/alexeydott/tegola/provider"
	codec "github.com/alexeydott/tegola/provider/geometrycodec"
)

// TestIdentTokenQuoting pins the cross-provider !ID_FIELD!/!GEOM_FIELD!
// token quoting contract (audit P5-10, shorthand "!ID!/!GEOM!"): unquoted values are quoted per identifier part,
// values wrapped in one complete identifier quote pair (double quote or
// backtick) pass through verbatim, and hostile values can never break
// out of the quoted identifier. The matrix
// goes through replaceTokens so it fails against the pre-fix raw
// substitution.
// TestSQLStringLiteral pins the audit P5-8 helper contract: values are
// wrapped as single-quoted literals with inner single quotes doubled,
// e.g. for the CreateRTreeIndex hint arguments in registration errors.
func TestSQLStringLiteral(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"gpkgTestPoints", "'gpkgTestPoints'"},
		{"we'ird", "'we''ird'"},
		{"'", "''''"},
		{"", "''"},
	}
	for _, tc := range tests {
		if got := sqlStringLiteral(tc.in); got != tc.want {
			t.Fatalf("sqlStringLiteral(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestIdentTokenQuoting(t *testing.T) {
	type tcase struct {
		value    string
		expected string
	}
	tests := map[string]tcase{
		"plain":                              {value: "feature_id", expected: "`feature_id`"},
		"qualified":                          {value: "schema.table.col", expected: "`schema`.`table`.`col`"},
		"already-quoted":                     {value: `"my.col"`, expected: `"my.col"`},
		"already-escaped":                    {value: "`a``b`", expected: "`a``b`"},
		"backtick-quoted":                    {value: "`my.col`", expected: "`my.col`"},
		"single-quoted-is-not-an-identifier": {value: "'my.col'", expected: "`'my.col'`"},
		"quote-shaped-single":                {value: "'x';DROP'", expected: "`'x';DROP'`"},
		"hostile-semi":                       {value: "a;b--", expected: "`a;b--`"},
		"hostile-quote":                      {value: `a"b`, expected: "`a\"b`"},
		"quote-shaped":                       {value: "`x`;DROP`", expected: "```x``;DROP```"},
		"mixed-qualified":                    {value: `"my schema".col`, expected: "\"my schema\".`col`"},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			layer := Layer{idFieldname: tc.value, geomFieldname: tc.value}
			tile := provider.NewTile(0, 0, 0, 0, tegola.WebMercator)
			ext, _ := tile.BufferedExtent()
			out, err := replaceTokens("SELECT !ID_FIELD!, !GEOM_FIELD! FROM t", &layer, tile, ext)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			want := "SELECT " + tc.expected + ", " + tc.expected + " FROM t"
			if out != want {
				t.Errorf("value %q:\n want %v\n  got %v", tc.value, want, out)
			}
		})
	}
}

func TestReplaceTokens(t *testing.T) {
	type tcase struct {
		qtext    string
		layer    Layer
		tile     provider.Tile
		extent   *geom.Extent
		expected string
	}

	fn := func(tc tcase) func(*testing.T) {
		return func(t *testing.T) {
			if tc.tile == nil {
				tc.tile = provider.NewTile(0, 0, 0, 0, tegola.WebMercator)
			}
			if tc.extent == nil {
				tc.extent, _ = tc.tile.BufferedExtent()
			}
			output, err := replaceTokens(tc.qtext, &tc.layer, tc.tile, tc.extent)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if tc.expected != output {
				t.Errorf("expected %v\n got\n %v", tc.expected, output)
				return
			}
		}
	}

	tests := map[string]tcase{
		"zoom": {
			qtext: `
				SELECT
					fid, geom, featurecla, min_zoom, 22 as max_zoom, minx, miny, maxx, maxy
				FROM
					ne_110m_land t JOIN rtree_ne_110m_land_geom si ON t.fid = si.id
				WHERE
					min_zoom <= !ZOOM! AND max_zoom >= !ZOOM!`,
			tile: provider.NewTile(9, 0, 0, 0, tegola.WebMercator),
			expected: `
				SELECT
					fid, geom, featurecla, min_zoom, 22 as max_zoom, minx, miny, maxx, maxy
				FROM
					ne_110m_land t JOIN rtree_ne_110m_land_geom si ON t.fid = si.id
				WHERE
					min_zoom <= 9 AND max_zoom >= 9`,
		},
		"bbox": {
			qtext: `
				SELECT
					fid, geom, featurecla, min_zoom, 22 as max_zoom, minx, miny, maxx, maxy
				FROM
					ne_110m_land t JOIN rtree_ne_110m_land_geom si ON t.fid = si.id
				WHERE
					!BBOX!`,
			extent: &geom.Extent{
				-180, -85.0511,
				180, 85.0511,
			},
			expected: `
				SELECT
					fid, geom, featurecla, min_zoom, 22 as max_zoom, minx, miny, maxx, maxy
				FROM
					ne_110m_land t JOIN rtree_ne_110m_land_geom si ON t.fid = si.id
				WHERE
					` + "`MAXX` >= -180 AND `MINX` <= 180 AND `MAXY` >= -85.0511 AND `MINY` <= 85.0511",
		},
		"bbox zoom": {
			qtext: `
				SELECT
					fid, geom, featurecla, min_zoom, 22 as max_zoom, minx, miny, maxx, maxy
				FROM
					ne_110m_land t JOIN rtree_ne_110m_land_geom si ON t.fid = si.id
				WHERE
					!BBOX! AND min_zoom = !ZOOM!`,
			extent: &geom.Extent{
				-180, -85.0511,
				180, 85.0511,
			},
			tile: provider.NewTile(3, 0, 0, 0, tegola.WebMercator),
			expected: `
				SELECT
					fid, geom, featurecla, min_zoom, 22 as max_zoom, minx, miny, maxx, maxy
				FROM
					ne_110m_land t JOIN rtree_ne_110m_land_geom si ON t.fid = si.id
				WHERE
					` + "`MAXX` >= -180 AND `MINX` <= 180 AND `MAXY` >= -85.0511 AND `MINY` <= 85.0511 AND min_zoom = 3",
		},
		"tile coordinates, scale and layer metadata": {
			qtext: `SELECT !id_field!, !geom_field!, '!geom_type!',
				!zoom!, !x!, !y!, !z!,
				!pixel_width!, !pixel_height!, !scale_denominator!`,
			layer: Layer{
				idFieldname:   "feature_id",
				srid:          3857,
				geomFieldname: "shape",
				geomType:      geom.Point{},
			},
			tile: provider.NewTile(11, 1070, 676, 64, tegola.WebMercator),
			// P5-10: !ID_FIELD!/!GEOM_FIELD! substitute quoted identifiers.
			// SQL-context-aware substitution (UPSTREAM 2.6): '!geom_type!'
			// sits inside a string literal and stays verbatim instead of
			// being interpolated.
			expected: "SELECT `feature_id`, `shape`, '!geom_type!',\n" +
				"				11, 1070, 676, 11,\n" +
				"				76.43702829, 76.43702829, 272989.38673277",
		},
	}

	for name, tc := range tests {
		t.Run(name, fn(tc))
	}
}

// SQL-context-aware token substitution (UPSTREAM 2.6): tokens inside
// string literals (mind quote doubling and backslash escapes), quoted
// identifiers and comments stay verbatim and are not uppercased; only SQL
// code-context tokens substitute.
func TestReplaceTokensProtectedContexts(t *testing.T) {
	layer := Layer{
		idFieldname:   "feature_id",
		srid:          3857,
		geomFieldname: "shape",
		geomType:      geom.Point{},
	}
	tile := provider.NewTile(11, 1070, 676, 64, tegola.WebMercator)
	extent, _ := tile.BufferedExtent()

	qtext := "SELECT '!zoom!', `!x!`, \"!y!\", [!z!]\n" +
		"FROM t WHERE z = !zoom! -- !BBOX!\n" +
		"/* !id_field! */ AND a = 'it''s \\!x!' AND b = '\\!y!'"
	want := "SELECT '!zoom!', `!x!`, \"!y!\", [!z!]\n" +
		"FROM t WHERE z = 11 -- !BBOX!\n" +
		"/* !id_field! */ AND a = 'it''s \\!x!' AND b = '\\!y!'"

	out, err := replaceTokens(qtext, &layer, tile, extent)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != want {
		t.Fatalf("protected contexts must stay verbatim:\n got %q\nwant %q", out, want)
	}
}

func TestTrimTrailingSemicolon(t *testing.T) {
	if got := trimTrailingSemicolon(" SELECT * FROM features ;  "); got != "SELECT * FROM features" {
		t.Fatalf("trimTrailingSemicolon() = %q", got)
	}
}

// P6-16: GeoPackage files must be opened read-only, with a busy timeout.
func TestSQLiteReadOnlyDSN(t *testing.T) {
	got := sqliteReadOnlyDSN(`C:\data\file.gpkg`)
	want := `file:C:\data\file.gpkg?mode=ro&_busy_timeout=5000`
	if got != want {
		t.Fatalf("sqliteReadOnlyDSN() = %q, want %q", got, want)
	}
}

// A12: bounds predicate build errors must fail closed (returned to the
// caller), never silently fall back to 1=1.
func TestReplaceTokensFailClosed(t *testing.T) {
	tile := provider.NewTile(0, 0, 0, 0, tegola.WebMercator)
	layer := Layer{}
	out, err := replaceTokens("SELECT * FROM t WHERE !BBOX!", &layer, tile, nil)
	if err == nil {
		t.Fatalf("expected bounds predicate build error to surface (A12), got output %q", out)
	}
}

// TestBuildBBoxPredicateContract pins the two bounds-predicate modes for
// custom SQL (audit A06) and the verbatim quoting of the resolved bounds
// column names (audit A09):
//   - custom MOS SQL filters over raw quantized bounds columns
//     (BoundsMOSRaw);
//   - custom geometry_format=gpkg SQL filters in the source CRS
//     (BoundsSourceCRS, no MOS scaling).
func TestBuildBBoxPredicateContract(t *testing.T) {
	extent := geom.NewExtent([2]float64{10, 20}, [2]float64{30, 40})

	mosLayer := &Layer{
		geometryFormat: codec.FormatMOS,
		bboxFields:     codec.BBoxFields{"minx", "maxx", "miny", "maxy"},
		mosConfig:      codec.MOSConfig{Precision: 1, UnitFactor: 1},
	}
	pred, err := buildBBoxPredicate(mosLayer, extent)
	if err != nil {
		t.Fatalf("mos predicate: %v", err)
	}
	// raw quantized values (x10^precision) and actual column names quoted
	// verbatim
	want := "`maxx` >= 100 AND `minx` <= 300 AND `maxy` >= 200 AND `miny` <= 400"
	if pred != want {
		t.Errorf("MOS custom SQL predicate: want %q, got %q", want, pred)
	}

	// (the "gpkg" value of GeometryFormatGPKG, inlined because the
	// constant lives in the cgo-gated gpkg.go)
	gpkgLayer := &Layer{
		geometryFormat: "gpkg",
		bboxFields:     codec.DefaultBBoxFields(),
	}
	pred, err = buildBBoxPredicate(gpkgLayer, extent)
	if err != nil {
		t.Fatalf("gpkg predicate: %v", err)
	}
	want = "`MAXX` >= 10 AND `MINX` <= 30 AND `MAXY` >= 20 AND `MINY` <= 40"
	if pred != want {
		t.Errorf("gpkg custom SQL predicate must run in the source CRS without MOS scaling: want %q, got %q", want, pred)
	}
}

// TestReplaceTokensPreservesTrailingClauses covers the regression matrix
// row "WHERE ... AND !BBOX! ORDER BY ... -> ORDER BY preserved": token
// replacement must keep trailing clauses verbatim.
func TestReplaceTokensPreservesTrailingClauses(t *testing.T) {
	tile := provider.NewTile(0, 0, 0, 0, tegola.WebMercator)
	extent, _ := tile.BufferedExtent()
	layer := &Layer{bboxFields: codec.DefaultBBoxFields()}
	out, err := replaceTokens("SELECT * FROM t WHERE kind = 'a' AND !BBOX! ORDER BY id DESC", layer, tile, extent)
	if err != nil {
		t.Fatalf("replaceTokens: %v", err)
	}
	if !strings.Contains(out, "ORDER BY id DESC") {
		t.Errorf("ORDER BY must be preserved: %q", out)
	}
	if strings.Contains(out, "!BBOX!") {
		t.Errorf("!BBOX! must be replaced: %q", out)
	}
}
