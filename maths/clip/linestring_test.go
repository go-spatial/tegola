package clip

import (
	"testing"

	"github.com/go-spatial/geom"
	"github.com/go-spatial/geom/cmp"
	"github.com/go-spatial/tegola"
	"github.com/go-spatial/tegola/basic"
)

var testExtents = []geom.Extent{
	/* 00 */ {0, 0, 10, 10},
	/* 01 */ {2, 2, 9, 9},
	/* 02 */ {-1, -1, 11, 11},
	/* 03 */ {-2, -2, 12, 12},
	/* 04 */ {-3, -3, 13, 13},

	/* 05 */ {-4, -4, 14, 14},
	/* 06 */ {5, 1, 7, 3},
	/* 07 */ {0, 5, 2, 7},
	/* 08 */ {0, 5, 2, 7},
	/* 09 */ {5, 2, 11, 9},

	/* 10 */ {-1, -1, 11, 11},
	/* 11 */ {0, 0, 4096, 4096},
}

func TestLineString(t *testing.T) {

	type tcase struct {
		extent   *geom.Extent
		linestr  tegola.LineString
		expected []basic.Line
		err      error
	}

	fn := func(t *testing.T, tc tcase) {
		t.Parallel()
		got, gerr := LineString(tc.linestr, tc.extent)
		switch {
		case tc.err != nil && gerr == nil:
			t.Errorf("expected error, expected %v, got nil", tc.err.Error())
			return
		case tc.err != nil && gerr != nil && tc.err.Error() != gerr.Error():
			t.Errorf("unexpected error, expected %v, got %v", tc.err.Error(), gerr.Error())
			return

		case tc.err == nil && gerr != nil:
			t.Errorf("unexpected error, expected nil, got %v", gerr.Error())
			return
		}
		if tc.err != nil {
			// we are expecting an error nothing more.
			return
		}
		if len(tc.expected) != len(got) {
			t.Errorf("number of lines, expected %v got %v", len(tc.expected), len(got))
			t.Errorf("expected: %v", tc.expected)
			t.Errorf("got     : %v", got)
			return
		}
		for i := range tc.expected {
			if !cmp.LineStringEqual(tc.expected[i].AsGeomLineString(), got[i].AsGeomLineString()) {
				t.Errorf("line %v,\n\texpected %#v\n\tgot %#v", i, tc.expected[i], got[i])
			}
		}
	}
	tests := map[string]tcase{
		"0": {
			extent:  &testExtents[0],
			linestr: basic.NewLine(-2, 1, 2, 1, 2, 2, -1, 2, -1, 11, 2, 11, 2, 4, 4, 4, 4, 13, -2, 13),
			expected: []basic.Line{
				basic.NewLine(0, 1, 2, 1, 2, 2, 0, 2),
				basic.NewLine(2, 10, 2, 4, 4, 4, 4, 10),
			},
		},
		"1": {
			extent:  &testExtents[0],
			linestr: basic.NewLine(-2, 1, 12, 1, 12, 2, -1, 2, -1, 11, 2, 11, 2, 4, 4, 4, 4, 13, -2, 13),
			expected: []basic.Line{
				basic.NewLine(0, 1, 10, 1),
				basic.NewLine(10, 2, 0, 2),
				basic.NewLine(2, 10, 2, 4, 4, 4, 4, 10),
			},
		},
		"2": {
			extent:  &testExtents[0],
			linestr: basic.NewLine(-3, 1, -3, 9, 11, 9, 11, 2, 5, 2, 5, 8, -1, 8, -1, 4, 3, 4, 3, 1),
			expected: []basic.Line{
				basic.NewLine(0, 9, 10, 9),
				basic.NewLine(10, 2, 5, 2, 5, 8, 0, 8),
				basic.NewLine(0, 4, 3, 4, 3, 1),
			},
		},
		"3": {
			extent:  &testExtents[1],
			linestr: basic.NewLine(-3, 1, -3, 9, 11, 9, 11, 2, 5, 2, 5, 8, -1, 8, -1, 4, 3, 4, 3, 1),
			expected: []basic.Line{
				basic.NewLine(2, 9, 9, 9),
				basic.NewLine(9, 2, 5, 2, 5, 8, 2, 8),
				basic.NewLine(2, 4, 3, 4, 3, 2),
			},
		},
		"4": {
			extent:  &geom.Extent{0, 0, 11, 11},
			linestr: basic.NewLine(-3, 1, -3, 9, 11, 9, 11, 2, 5, 2, 5, 8, -1, 8, -1, 4, 3, 4, 3, 1),
			expected: []basic.Line{
				basic.NewLine(0, 9, 11, 9, 11, 2, 5, 2, 5, 8, 0, 8),
				basic.NewLine(0, 4, 3, 4, 3, 1),
			},
		},
		"5": {
			extent:  &testExtents[3],
			linestr: basic.NewLine(-3, 1, -3, 9, 11, 9, 11, 2, 5, 2, 5, 8, -1, 8, -1, 4, 3, 4, 3, 1),
			expected: []basic.Line{
				basic.NewLine(-2, 9, 11, 9, 11, 2, 5, 2, 5, 8, -1, 8, -1, 4, 3, 4, 3, 1),
			},
		},
		"6": {
			extent:  &testExtents[4],
			linestr: basic.NewLine(-3, 1, -3, 9, 11, 9, 11, 2, 5, 2, 5, 8, -1, 8, -1, 4, 3, 4, 3, 1),
			expected: []basic.Line{
				basic.NewLine(-3, 1, -3, 9, 11, 9, 11, 2, 5, 2, 5, 8, -1, 8, -1, 4, 3, 4, 3, 1),
			},
		},
		"7": {
			extent:  &testExtents[5],
			linestr: basic.NewLine(-3, 1, -3, 9, 11, 9, 11, 2, 5, 2, 5, 8, -1, 8, -1, 4, 3, 4, 3, 1),
			expected: []basic.Line{
				basic.NewLine(-3, 1, -3, 9, 11, 9, 11, 2, 5, 2, 5, 8, -1, 8, -1, 4, 3, 4, 3, 1),
			},
		},
		"8": {
			extent:  &testExtents[6],
			linestr: basic.NewLine(-3, 1, -3, 9, 11, 9, 11, 2, 5, 2, 5, 8, -1, 8, -1, 4, 3, 4, 3, 1),
			expected: []basic.Line{
				basic.NewLine(7, 2, 5, 2, 5, 3),
			},
		},
		"9": {
			extent:   &testExtents[7],
			linestr:  basic.NewLine(-3, 1, -3, 9, 11, 9, 11, 2, 5, 2, 5, 8, -1, 8, -1, 4, 3, 4, 3, 1),
			expected: nil,
		},
		"10": {
			extent:   &testExtents[8],
			linestr:  basic.NewLine(-3, 1, -3, 9, 11, 9, 11, 2, 5, 2, 5, 8, -1, 8, -1, 4, 3, 4, 3, 1),
			expected: nil,
		},
		"11": {
			extent:  &testExtents[9],
			linestr: basic.NewLine(-3, 1, -3, 9, 11, 9, 11, 2, 5, 2, 5, 8, -1, 8, -1, 4, 3, 4, 3, 1),
			expected: []basic.Line{
				basic.NewLine(5, 9, 11, 9, 11, 2, 5, 2, 5, 8),
			},
		},
		"12": {
			extent:   &testExtents[9],
			linestr:  basic.NewLine(-3, 1, -3, 10, 12, 10, 12, 1, 4, 1, 4, 8, -1, 8, -1, 4, 3, 4, 3, 1),
			expected: nil,
		},
		"13": {
			extent:  &testExtents[0],
			linestr: basic.NewLine(-3, -3, -3, 10, 12, 10, 12, 1, 4, 1, 4, 8, -1, 8, -1, 4, 3, 4, 3, 3),
			expected: []basic.Line{
				basic.NewLine(0, 10, 10, 10),
				basic.NewLine(10, 1, 4, 1, 4, 8, 0, 8),
				basic.NewLine(0, 4, 3, 4, 3, 3),
			},
		},
		"14": {
			extent:  &testExtents[10],
			linestr: basic.NewLine(-1, -1, 12, -1, 12, 12, -1, 12),
			expected: []basic.Line{
				basic.NewLine(-1, -1, 11, -1),
			},
		},
		"15": {
			extent: &testExtents[11],

			linestr: basic.NewLine(
				7848, 19609, 7340, 18835, 6524, 17314, 6433, 17163, 5178, 15057, 5147, 15006, 4680, 14226, 3861, 12766, 2471, 10524, 2277, 10029, 1741, 8281, 1655, 8017, 1629, 7930, 1437, 7368, 973, 5481,
				325, 4339, -497, 3233,
				-1060, 2745, -1646, 2326, -1883, 2156, -2002, 2102, -2719, 1774, -3638, 1382, -3795, 1320, -5225, 938, -6972, 295, -7672, -88, -8243, -564, -8715,
				-1112, -9019, -1573, -9235, -2067, -9293, -2193, -9408, -2570, -9823, -4630, -10118, -5927, -10478, -7353, -10909, -8587, -11555, -9743, -11837, -10005, -12277, -10360, -13748,
				-11189, -14853, -12102, -15806, -12853, -16711, -13414),
			expected: []basic.Line{
				basic.NewLine(144.397830, 4096, 0, 3901.712895),
			},
		},
		"empty line": {
			extent:  &testExtents[11],
			linestr: basic.Line{},
		},
	}
	for name, tc := range tests {
		tc := tc
		t.Run(name, func(t *testing.T) { fn(t, tc) })
	}
}
