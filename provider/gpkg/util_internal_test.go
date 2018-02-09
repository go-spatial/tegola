package gpkg

import (
	"reflect"
	"testing"
)

func TestReplaceTokens(t *testing.T) {
	type tcase struct {
		qtext          string
		expected       string
		expectedTokens map[string]bool
	}

	fn := func(t *testing.T, tc tcase) {
		output, tokens := replaceTokens(tc.qtext)

		if tc.expected != output {
			t.Errorf("expected %v got %v", tc.expected, output)
			return
		}

		if !reflect.DeepEqual(tc.expectedTokens, tokens) {
			t.Errorf("expected %v got %v", tc.expectedTokens, tokens)
			return
		}
	}

	tests := map[string]tcase{
		"zoom": tcase{
			qtext: `
				SELECT
					fid, geom, featurecla, min_zoom, 22 as max_zoom, minx, miny, maxx, maxy
				FROM
					ne_110m_land t JOIN rtree_ne_110m_land_geom si ON t.fid = si.id
				WHERE
					!ZOOM!`,
			expected: `
				SELECT
					fid, geom, featurecla, min_zoom, 22 as max_zoom, minx, miny, maxx, maxy
				FROM
					ne_110m_land t JOIN rtree_ne_110m_land_geom si ON t.fid = si.id
				WHERE
					min_zoom <= ? AND max_zoom >= ?`,
			expectedTokens: map[string]bool{
				"ZOOM": true,
			},
		},
		"bbox": tcase{
			qtext: `
				SELECT
					fid, geom, featurecla, min_zoom, 22 as max_zoom, minx, miny, maxx, maxy
				FROM
					ne_110m_land t JOIN rtree_ne_110m_land_geom si ON t.fid = si.id
				WHERE
					!BBOX!`,
			expected: `
				SELECT
					fid, geom, featurecla, min_zoom, 22 as max_zoom, minx, miny, maxx, maxy
				FROM
					ne_110m_land t JOIN rtree_ne_110m_land_geom si ON t.fid = si.id
				WHERE
					minx <= ? AND maxx >= ? AND miny <= ? AND maxy >= ?`,
			expectedTokens: map[string]bool{
				"BBOX": true,
			},
		},
		"bbox zoom": tcase{
			qtext: `
				SELECT
					fid, geom, featurecla, min_zoom, 22 as max_zoom, minx, miny, maxx, maxy
				FROM
					ne_110m_land t JOIN rtree_ne_110m_land_geom si ON t.fid = si.id
				WHERE
					!BBOX! AND !ZOOM!`,
			expected: `
				SELECT
					fid, geom, featurecla, min_zoom, 22 as max_zoom, minx, miny, maxx, maxy
				FROM
					ne_110m_land t JOIN rtree_ne_110m_land_geom si ON t.fid = si.id
				WHERE
					minx <= ? AND maxx >= ? AND miny <= ? AND maxy >= ? AND min_zoom <= ? AND max_zoom >= ?`,
			expectedTokens: map[string]bool{
				"ZOOM": true,
				"BBOX": true,
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			fn(t, tc)
		})
	}
}
