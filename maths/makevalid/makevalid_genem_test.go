package makevalid

import (
	"strings"
	"testing"

	"github.com/gdey/tbltest"
	"github.com/go-test/deep"
	"github.com/terranodo/tegola/maths"
	"github.com/terranodo/tegola/maths/edgemap"
)

func TestGenerateEdgeMap(t *testing.T) {
	type testcase struct {
		lines   [][]maths.Line
		edgemap edgemap.EM
	}
	tests := tbltest.Cases(
		testcase{
			lines: [][]maths.Line{
				{
					{maths.Pt{4, 4}, maths.Pt{4, 9}},
					{maths.Pt{4, 9}, maths.Pt{5, 9}},
					{maths.Pt{5, 9}, maths.Pt{5, 4}},
				},
				{
					{maths.Pt{3, 1}, maths.Pt{3, 6}},
					{maths.Pt{3, 6}, maths.Pt{7, 6}},
					{maths.Pt{7, 6}, maths.Pt{7, 1}},
				},
			},
			edgemap: edgemap.EM{
				BBox: [4]maths.Pt{{0 - adjustBBoxBy, 0 - adjustBBoxBy}, {7 + adjustBBoxBy, 0 - adjustBBoxBy}, {7 + adjustBBoxBy, 9 + adjustBBoxBy}, {0 - adjustBBoxBy, 9 + adjustBBoxBy}},
				Keys: []maths.Pt{
					{0 - adjustBBoxBy, 0 - adjustBBoxBy}, {0 - adjustBBoxBy, 9 + adjustBBoxBy}, {3, 1}, {3, 6}, {4, 4}, {4, 6}, {4, 9}, {5, 4}, {5, 6}, {5, 9}, {7, 1}, {7, 6}, {7 + adjustBBoxBy, 0 - adjustBBoxBy}, {7 + adjustBBoxBy, 9 + adjustBBoxBy},
				},
				Map: map[maths.Pt]map[maths.Pt]bool{
					maths.Pt{0 - adjustBBoxBy, 0 - adjustBBoxBy}: map[maths.Pt]bool{
						maths.Pt{7 + adjustBBoxBy, 0 - adjustBBoxBy}: false,
						maths.Pt{0 - adjustBBoxBy, 9 + adjustBBoxBy}: false,
						maths.Pt{3, 1}:                               false,
						maths.Pt{3, 6}:                               false,
						maths.Pt{4, 9}:                               false,
						maths.Pt{7, 1}:                               false,
					},
					maths.Pt{0 - adjustBBoxBy, 9 + adjustBBoxBy}: map[maths.Pt]bool{
						maths.Pt{0 - adjustBBoxBy, 0 - adjustBBoxBy}: false,
						maths.Pt{7 + adjustBBoxBy, 9 + adjustBBoxBy}: false,
						maths.Pt{4, 9}:                               false,
						maths.Pt{5, 9}:                               false,
					},
					maths.Pt{3, 1}: map[maths.Pt]bool{
						maths.Pt{3, 6}:                               true,
						maths.Pt{7, 1}:                               true,
						maths.Pt{0 - adjustBBoxBy, 0 - adjustBBoxBy}: false,
						maths.Pt{4, 4}:                               false,
						maths.Pt{4, 6}:                               false,
						maths.Pt{5, 4}:                               false,
						maths.Pt{7, 6}:                               false,
					},
					maths.Pt{3, 6}: map[maths.Pt]bool{
						maths.Pt{3, 1}:                               true,
						maths.Pt{4, 6}:                               true,
						maths.Pt{0 - adjustBBoxBy, 0 - adjustBBoxBy}: false,
						maths.Pt{4, 9}:                               false,
					},
					maths.Pt{4, 4}: map[maths.Pt]bool{
						maths.Pt{4, 6}: true,
						maths.Pt{5, 4}: true,
						maths.Pt{3, 1}: false,
						maths.Pt{5, 6}: false,
					},
					maths.Pt{4, 6}: map[maths.Pt]bool{
						maths.Pt{3, 6}: true,
						maths.Pt{4, 4}: true,
						maths.Pt{4, 9}: true,
						maths.Pt{5, 6}: true,
						maths.Pt{3, 1}: false,
						maths.Pt{5, 9}: false,
					},
					maths.Pt{4, 9}: map[maths.Pt]bool{
						maths.Pt{4, 6}:                               true,
						maths.Pt{5, 9}:                               true,
						maths.Pt{0 - adjustBBoxBy, 0 - adjustBBoxBy}: false,
						maths.Pt{0 - adjustBBoxBy, 9 + adjustBBoxBy}: false,
						maths.Pt{3, 6}:                               false,
					},
					maths.Pt{5, 4}: map[maths.Pt]bool{
						maths.Pt{4, 4}: true,
						maths.Pt{5, 6}: true,
						maths.Pt{3, 1}: false,
						maths.Pt{7, 6}: false,
					},
					maths.Pt{5, 6}: map[maths.Pt]bool{
						maths.Pt{4, 6}:                               true,
						maths.Pt{5, 4}:                               true,
						maths.Pt{5, 9}:                               true,
						maths.Pt{7, 6}:                               true,
						maths.Pt{4, 4}:                               false,
						maths.Pt{7 + adjustBBoxBy, 9 + adjustBBoxBy}: false,
					},
					maths.Pt{5, 9}: map[maths.Pt]bool{
						maths.Pt{4, 9}:                               true,
						maths.Pt{5, 6}:                               true,
						maths.Pt{0 - adjustBBoxBy, 9 + adjustBBoxBy}: false,
						maths.Pt{7 + adjustBBoxBy, 9 + adjustBBoxBy}: false,
						maths.Pt{4, 6}:                               false,
					},
					maths.Pt{7, 1}: map[maths.Pt]bool{
						maths.Pt{3, 1}:                               true,
						maths.Pt{7, 6}:                               true,
						maths.Pt{0 - adjustBBoxBy, 0 - adjustBBoxBy}: false,
						maths.Pt{7 + adjustBBoxBy, 0 - adjustBBoxBy}: false,
						maths.Pt{7 + adjustBBoxBy, 9 + adjustBBoxBy}: false,
					},
					maths.Pt{7, 6}: map[maths.Pt]bool{
						maths.Pt{5, 6}:                               true,
						maths.Pt{7, 1}:                               true,
						maths.Pt{3, 1}:                               false,
						maths.Pt{5, 4}:                               false,
						maths.Pt{7 + adjustBBoxBy, 9 + adjustBBoxBy}: false,
					},
					maths.Pt{7 + adjustBBoxBy, 0 - adjustBBoxBy}: map[maths.Pt]bool{
						maths.Pt{0 - adjustBBoxBy, 0 - adjustBBoxBy}: false,
						maths.Pt{7 + adjustBBoxBy, 9 + adjustBBoxBy}: false,
						maths.Pt{7, 1}:                               false,
					},

					maths.Pt{7 + adjustBBoxBy, 9 + adjustBBoxBy}: map[maths.Pt]bool{
						maths.Pt{0 - adjustBBoxBy, 9 + adjustBBoxBy}: false,
						maths.Pt{5, 6}:                               false,
						maths.Pt{5, 9}:                               false,
						maths.Pt{7, 1}:                               false,
						maths.Pt{7, 6}:                               false,
						maths.Pt{7 + adjustBBoxBy, 0 - adjustBBoxBy}: false,
					},
				},
				Segments: []maths.Line{
					{maths.Pt{0 - adjustBBoxBy, 0 - adjustBBoxBy}, maths.Pt{7 + adjustBBoxBy, 0 - adjustBBoxBy}},
					{maths.Pt{7 + adjustBBoxBy, 0 - adjustBBoxBy}, maths.Pt{7 + adjustBBoxBy, 9 + adjustBBoxBy}},
					{maths.Pt{7 + adjustBBoxBy, 9 + adjustBBoxBy}, maths.Pt{0 - adjustBBoxBy, 9 + adjustBBoxBy}},
					{maths.Pt{0 - adjustBBoxBy, 9 + adjustBBoxBy}, maths.Pt{0 - adjustBBoxBy, 0 - adjustBBoxBy}},
					{maths.Pt{3, 1}, maths.Pt{3, 6}},
					{maths.Pt{3, 1}, maths.Pt{7, 1}},
					{maths.Pt{3, 6}, maths.Pt{4, 6}},
					{maths.Pt{4, 4}, maths.Pt{4, 6}},
					{maths.Pt{4, 4}, maths.Pt{5, 4}},
					{maths.Pt{4, 6}, maths.Pt{4, 9}},
					{maths.Pt{4, 6}, maths.Pt{5, 6}},
					{maths.Pt{4, 9}, maths.Pt{5, 9}},
					{maths.Pt{5, 4}, maths.Pt{5, 6}},
					{maths.Pt{5, 6}, maths.Pt{5, 9}},
					{maths.Pt{5, 6}, maths.Pt{7, 6}},
					{maths.Pt{7, 1}, maths.Pt{7, 6}},
					{maths.Pt{0 - adjustBBoxBy, 0 - adjustBBoxBy}, maths.Pt{3, 1}},
					{maths.Pt{0 - adjustBBoxBy, 0 - adjustBBoxBy}, maths.Pt{3, 6}},
					{maths.Pt{0 - adjustBBoxBy, 0 - adjustBBoxBy}, maths.Pt{4, 9}},
					{maths.Pt{0 - adjustBBoxBy, 0 - adjustBBoxBy}, maths.Pt{7, 1}},
					{maths.Pt{0 - adjustBBoxBy, 9 + adjustBBoxBy}, maths.Pt{4, 9}},
					{maths.Pt{0 - adjustBBoxBy, 9 + adjustBBoxBy}, maths.Pt{5, 9}},
					{maths.Pt{3, 1}, maths.Pt{4, 4}},
					{maths.Pt{3, 1}, maths.Pt{4, 6}},
					{maths.Pt{3, 1}, maths.Pt{5, 4}},
					{maths.Pt{3, 1}, maths.Pt{7, 6}},
					{maths.Pt{3, 6}, maths.Pt{4, 9}},
					{maths.Pt{4, 4}, maths.Pt{5, 6}},
					{maths.Pt{4, 6}, maths.Pt{5, 9}},
					{maths.Pt{5, 4}, maths.Pt{7, 6}},
					{maths.Pt{5, 6}, maths.Pt{7 + adjustBBoxBy, 9 + adjustBBoxBy}},
					{maths.Pt{5, 9}, maths.Pt{7 + adjustBBoxBy, 9 + adjustBBoxBy}},
					{maths.Pt{7, 1}, maths.Pt{7 + adjustBBoxBy, 0 - adjustBBoxBy}},
					{maths.Pt{7, 1}, maths.Pt{7 + adjustBBoxBy, 9 + adjustBBoxBy}},
					{maths.Pt{7, 6}, maths.Pt{7 + adjustBBoxBy, 9 + adjustBBoxBy}},
				},
			},
		},
	)

	tests.Run(func(idx int, test testcase) {
		polygons := edgemap.Destructure(edgemap.InsureConnected(test.lines...))

		//		em := generateEdgeMap(polygons)
		em := edgemap.New(polygons)
		em.Triangulate()
		// Check the keys first:
		if diff := deep.Equal(em.Keys, test.edgemap.Keys); diff != nil {
			t.Error("Keys do not match: Expected\n\t", test.edgemap.Keys, "\ngot\n\t", em.Keys, "\n\tdiff:\t", diff)
		}
		// Check the Map:
		if diff := deep.Equal(em.Map, test.edgemap.Map); diff != nil {
			t.Error("Map do not match: Expected\n\t", test.edgemap.Map, "\ngot\n\t", em.Map, "\n\tdiff:\t", strings.Join(diff, "\n\t\t"))
		}
		// Check the Segments:
		if diff := deep.Equal(em.Segments, test.edgemap.Segments); diff != nil {
			t.Error("Segments do not match: Expected\n\t", test.edgemap.Segments, "\ngot\n\t", em.Segments, "\n\tdiff:\t", diff)
		}
		// Check BBox
		if diff := deep.Equal(em.BBox, test.edgemap.BBox); diff != nil {
			t.Error("BBox do not match: Expected\n\t", test.edgemap.BBox, "\ngot\n\t", em.BBox, "\n\tdiff:\t", strings.Join(diff, "\n\t\t"))
		}
	})
}
