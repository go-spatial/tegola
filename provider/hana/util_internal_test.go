package hana

import (
	"testing"

	"github.com/alexeydott/tegola"
	"github.com/alexeydott/tegola/basic"
	"github.com/alexeydott/tegola/provider"
	codec "github.com/alexeydott/tegola/provider/geometrycodec"
)

func TestValidateCRSFormatCompatibility(t *testing.T) {
	type tcase struct {
		srid           int
		providerType   string
		geometryFormat string
		expectedErr    string
	}

	fn := func(tc tcase) func(t *testing.T) {
		return func(t *testing.T) {
			err := validateCRSFormatCompatibility(tc.srid, tc.providerType, tc.geometryFormat)
			if tc.expectedErr == "" {
				if err != nil {
					t.Errorf("unexpected error, Expected nil Got %v", err)
				}
				return
			}
			if err == nil {
				t.Errorf("expected error, Expected %v Got nil", tc.expectedErr)
				return
			}
			if err.Error() != tc.expectedErr {
				t.Errorf("incorrect error,\n Expected \n \t%v\n Got \n \t%v", tc.expectedErr, err.Error())
			}
		}
	}

	synthetic := int(basic.SyntheticSRIDMin + 1000)

	tests := map[string]tcase{
		"non-synthetic srid, native format": {
			srid: 3857,
		},
		"non-synthetic srid, raw format": {
			srid:           3857,
			geometryFormat: "wkb",
		},
		"synthetic srid, wkb ok": {
			srid:           synthetic,
			geometryFormat: "wkb",
		},
		"synthetic srid, wkt ok": {
			srid:           synthetic,
			geometryFormat: "wkt",
		},
		"synthetic srid, mos ok": {
			srid:           synthetic,
			geometryFormat: "mos",
		},
		"synthetic srid, empty format rejected": {
			srid:        synthetic,
			expectedErr: `crs_defn (synthetic SRID) requires geometry_format = "wkb", "wkt", or "mos"; native HANA ST_Geometry columns use a database-side SRS`,
		},
		"synthetic srid, mvt rejected": {
			srid:           synthetic,
			providerType:   "mvt_hana",
			geometryFormat: "wkb",
			expectedErr:    `crs_defn (synthetic SRID) is not supported for MVT providers`,
		},
		"planar equivalent of round-earth is not synthetic": {
			srid:           int(1000000000 + 4326),
			geometryFormat: "",
		},
	}

	for name, tc := range tests {
		t.Run(name, fn(tc))
	}
}

func TestReplaceTokens(t *testing.T) {
	type tcase struct {
		dbVersion uint
		sql       string
		tile      provider.Tile
		expected  string
		layer     Layer
	}

	fn := func(tc tcase) func(t *testing.T) {
		return func(t *testing.T) {
			sql, err := replaceTokens(tc.dbVersion, tc.sql, &tc.layer, tc.layer.GeomType(), tc.layer.SRID(), tc.tile, true)
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
		"replace BBOX for HANA 1": {
			dbVersion: 1,
			sql:       "SELECT * FROM foo WHERE !BBOX!",
			layer:     Layer{srid: tegola.WebMercator, geomField: "geom"},
			tile:      provider.NewTile(2, 1, 1, 64, tegola.WebMercator),
			expected:  `SELECT * FROM foo WHERE "geom".ST_IntersectsRect(NEW ST_POINT($1, $3), NEW ST_POINT($2, $3)) = 1`,
		},
		"replace BBOX": {
			dbVersion: 4,
			sql:       "SELECT * FROM foo WHERE !BBOX!",
			layer:     Layer{srid: tegola.WebMercator, geomField: "geom"},
			tile:      provider.NewTile(2, 1, 1, 64, tegola.WebMercator),
			expected:  `SELECT * FROM foo WHERE "geom".ST_IntersectsRectPlanar(NEW ST_POINT($1, $3), NEW ST_POINT($2, $3)) = 1`,
		},
		"replace BBOX for round-earth with planar equivalent": {
			dbVersion: 4,
			sql:       "SELECT * FROM foo WHERE !BBOX!",
			layer:     Layer{srid: 1000004326, geomField: "geom"},
			tile:      provider.NewTile(2, 1, 1, 64, tegola.WebMercator),
			expected:  `SELECT * FROM foo WHERE "geom".ST_SRID($3).ST_IntersectsRectPlanar(NEW ST_POINT($1, $3), NEW ST_POINT($2, $3)) = 1`,
		},
		"replace BBOX with != in query": {
			dbVersion: 4,
			sql:       "SELECT * FROM foo WHERE !BBOX! AND bar != 42",
			layer:     Layer{srid: tegola.WebMercator, geomField: "geom"},
			tile:      provider.NewTile(2, 1, 1, 64, tegola.WebMercator),
			expected:  `SELECT * FROM foo WHERE "geom".ST_IntersectsRectPlanar(NEW ST_POINT($1, $3), NEW ST_POINT($2, $3)) = 1 AND bar != 42`,
		},
		"replace BBOX and ZOOM 1": {
			dbVersion: 4,
			sql:       "SELECT id, scalerank=!ZOOM! FROM foo WHERE !BBOX!",
			layer:     Layer{srid: tegola.WebMercator, geomField: "geom"},
			tile:      provider.NewTile(2, 1, 1, 64, tegola.WebMercator),
			expected:  `SELECT id, scalerank=2 FROM foo WHERE "geom".ST_IntersectsRectPlanar(NEW ST_POINT($1, $3), NEW ST_POINT($2, $3)) = 1`,
		},
		"replace BBOX and ZOOM 2": {
			dbVersion: 4,
			sql:       "SELECT id, scalerank=!ZOOM! FROM foo WHERE !BBOX!",
			layer:     Layer{srid: tegola.WebMercator, geomField: "geom"},
			tile:      provider.NewTile(16, 11241, 26168, 64, tegola.WebMercator),
			expected:  `SELECT id, scalerank=16 FROM foo WHERE "geom".ST_IntersectsRectPlanar(NEW ST_POINT($1, $3), NEW ST_POINT($2, $3)) = 1`,
		},
		"replace pixel_width/height and scale_denominator": {
			dbVersion: 4,
			sql:       "SELECT id, !pixel_width! as width, !pixel_height! as height, !scale_denominator! as scale_denom FROM foo WHERE !BBOX!",
			layer:     Layer{srid: tegola.WebMercator, geomField: "geom"},
			tile:      provider.NewTile(11, 1070, 676, 64, tegola.WebMercator),
			expected:  `SELECT id, 76.43702829 as width, 76.43702829 as height, 272989.38673277 as scale_denom FROM foo WHERE "geom".ST_IntersectsRectPlanar(NEW ST_POINT($1, $3), NEW ST_POINT($2, $3)) = 1`,
		},
	}

	for name, tc := range tests {
		t.Run(name, fn(tc))
	}
}

// SQL-context-aware token substitution (UPSTREAM 2.6): tokens inside
// string literals, quoted identifiers and comments stay verbatim and are
// not uppercased; only SQL code-context tokens substitute.
func TestReplaceTokensProtectedContexts(t *testing.T) {
	layer := Layer{srid: tegola.WebMercator, geomField: "geom"}
	tile := provider.NewTile(2, 1, 1, 64, tegola.WebMercator)

	sql := "SELECT '!BBOX!', '!x!', \"!y!\", '!z!'\n" +
		"FROM foo WHERE z = !ZOOM! -- !zoom!\n" +
		"/* !ID_FIELD! */"
	want := "SELECT '!BBOX!', '!x!', \"!y!\", '!z!'\n" +
		"FROM foo WHERE z = 2 -- !zoom!\n" +
		"/* !ID_FIELD! */"

	out, err := replaceTokens(4, sql, &layer, layer.GeomType(), layer.SRID(), tile, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != want {
		t.Fatalf("protected contexts must stay verbatim:\n got %q\nwant %q", out, want)
	}
}

// TestGenSQLRawFormat covers the R3-08 provider-level regression for the
// HANA SQL generation matrix: raw geometry columns (wkb / wkt / mos) must
// bypass ST_Geometry wrappers and spatial predicates, while native format
// keeps them.
func TestGenSQLRawFormat(t *testing.T) {
	type tcase struct {
		name           string
		geometryFormat string
		expectedSQL    string
	}

	tests := []tcase{
		{
			name:           "native format uses spatial predicate",
			geometryFormat: "",
			expectedSQL:    `SELECT "id", "geom".ST_AsBinary()  AS "geom", "id" FROM "tbl" WHERE !BBOX!`,
		},
		{
			name:           "mos format selects raw blob without bbox predicate",
			geometryFormat: "mos",
			expectedSQL:    `SELECT "id", "geom" AS "geom", "id" FROM "tbl" WHERE "geom" IS NOT NULL`,
		},
		{
			name:           "wkb format selects raw value without bbox predicate",
			geometryFormat: "wkb",
			expectedSQL:    `SELECT "id", "geom" AS "geom", "id" FROM "tbl" WHERE "geom" IS NOT NULL`,
		},
		{
			name:           "wkt format selects raw value without bbox predicate",
			geometryFormat: "wkt",
			expectedSQL:    `SELECT "id", "geom" AS "geom", "id" FROM "tbl" WHERE "geom" IS NOT NULL`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			l := &Layer{
				name:           "layer",
				geomField:      "geom",
				idField:        "id",
				srid:           tegola.WebMercator,
				geometryFormat: tc.geometryFormat,
			}
			sql, err := genSQL(l, "tbl", []string{"id", "geom"}, false, ProviderType)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if sql != tc.expectedSQL {
				t.Errorf("incorrect sql,\n Expected \n \t%v\n Got \n \t%v", tc.expectedSQL, sql)
			}
		})
	}
}

// TestGenSQLIdFieldAlsoTag pins the postgis provider contract "the id has
// to be parsed once but it can also be a tag": when the configured fields
// already contain the id field, genSQL must still append the id column to
// the SELECT list so the row carries it twice. readRowValues consumes the
// first occurrence as the feature id and the second occurrence becomes a
// tags entry (see TestReadRowValuesRepeatedIDOccurrenceIsTag).
// Pre-fix: the id column was emitted only once and an explicitly
// configured id field never reached the tags map
// (tablename query with fields and id as field regression).
func TestGenSQLIdFieldAlsoTag(t *testing.T) {
	l := &Layer{
		name:      "layer",
		geomField: "geom",
		idField:   "id",
		srid:      tegola.WebMercator,
	}
	sql, err := genSQL(l, "tbl", []string{"id", "scalerank"}, false, ProviderType)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := `SELECT "id", "scalerank", "geom".ST_AsBinary()  AS "geom", "id" FROM "tbl" WHERE !BBOX!`
	if sql != expected {
		t.Errorf("incorrect sql,\n Expected \n \t%v\n Got \n \t%v", expected, sql)
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

// TestTransformSRID covers the SRID normalization contract for client-side
// coordinate transforms: planar-equivalent layer SRIDs only label the
// round-earth SRID of a HANA geometry column and must be mapped back to the
// effective (base) SRID before any registered CRS conversion. Everything
// else, including synthetic crs_defn SRIDs, passes through unchanged.
func TestTransformSRID(t *testing.T) {
	synthetic := basic.SyntheticSRIDMin + 1000

	tests := map[string]struct{ srid, expected uint64 }{
		"planar equivalent of wgs84":    {PLANAR_SRID_OFFSET + 4326, 4326},
		"planar equivalent of webmerc":  {PLANAR_SRID_OFFSET + tegola.WebMercator, tegola.WebMercator},
		"plain webmercator":             {tegola.WebMercator, tegola.WebMercator},
		"plain wgs84":                   {4326, 4326},
		"synthetic srid passes through": {synthetic, synthetic},
		"zero srid passes through":      {0, 0},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			got := transformSRID(tc.srid)
			if got != tc.expected {
				t.Errorf("transformSRID(%v) = %v, want %v", tc.srid, got, tc.expected)
			}
		})
	}
}

// TestTileBBoxInLayerCRSPlanarEquivalentSRID ensures the in-memory tile
// extent conversion for raw geometry formats uses the effective SRID: a
// planar-equivalent layer SRID must convert exactly like its base SRID
// instead of failing with "don't know how to convert from 1000004326".
func TestTileBBoxInLayerCRSPlanarEquivalentSRID(t *testing.T) {
	tile := provider.NewTile(2, 1, 1, 64, tegola.WebMercator)

	base, err := tileBBoxInLayerCRS(tile, 4326)
	if err != nil {
		t.Fatalf("unexpected error for base SRID: %v", err)
	}

	planar, err := tileBBoxInLayerCRS(tile, PLANAR_SRID_OFFSET+4326)
	if err != nil {
		t.Fatalf("planar-equivalent SRID must convert like its base SRID, got error: %v", err)
	}
	if *planar != *base {
		t.Errorf("extent mismatch for planar-equivalent SRID,\n Expected \n \t%v\n Got \n \t%v", base, planar)
	}

	// WebMercator layers return the untransformed buffered extent.
	raw, _ := tile.BufferedExtent()
	wm, err := tileBBoxInLayerCRS(tile, tegola.WebMercator)
	if err != nil {
		t.Fatalf("unexpected error for webmercator: %v", err)
	}
	if *wm != *raw {
		t.Errorf("webmercator extent must be unchanged,\n Expected \n \t%v\n Got \n \t%v", raw, wm)
	}
}

// TestReplaceTokensMOSPlanarEquivalentSRID ensures the MOS raw-token branch
// of replaceTokens converts the tile extent with the effective SRID: a
// planar-equivalent layer SRID must produce the same !BBOX! predicate as its
// base SRID instead of failing the conversion.
func TestReplaceTokensMOSPlanarEquivalentSRID(t *testing.T) {
	mkLayer := func(srid uint64) Layer {
		return Layer{
			name:           "mos",
			geomField:      "geom",
			srid:           srid,
			geometryFormat: codec.FormatMOS,
			bboxFields:     codec.DefaultBBoxFields(),
			mosConfig:      codec.MOSConfig{Precision: 2, UnitFactor: 1},
		}
	}
	const sql = "SELECT * FROM foo WHERE !BBOX!"
	tile := provider.NewTile(2, 1, 1, 64, tegola.WebMercator)

	baseLayer := mkLayer(4326)
	want, err := replaceTokens(2, sql, &baseLayer, baseLayer.GeomType(), baseLayer.SRID(), tile, true)
	if err != nil {
		t.Fatalf("unexpected error for base SRID: %v", err)
	}

	planarLayer := mkLayer(PLANAR_SRID_OFFSET + 4326)
	got, err := replaceTokens(2, sql, &planarLayer, planarLayer.GeomType(), planarLayer.SRID(), tile, true)
	if err != nil {
		t.Fatalf("planar-equivalent SRID must not break MOS token replacement, got error: %v", err)
	}
	if got != want {
		t.Errorf("predicate mismatch for planar-equivalent SRID,\n Expected \n \t%v\n Got \n \t%v", want, got)
	}
}
