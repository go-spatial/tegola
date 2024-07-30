package hana

import (
	"testing"

	"github.com/go-spatial/tegola"
	"github.com/go-spatial/tegola/provider"
)

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
			sql, err := replaceTokens(tc.dbVersion, tc.sql, tc.layer.IDFieldName(), tc.layer.GeomFieldName(), tc.layer.GeomType(), tc.layer.SRID(), tc.tile, true)
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
