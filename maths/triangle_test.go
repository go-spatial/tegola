package maths

import (
	"log"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/gdey/tbltest"
	"github.com/go-test/deep"
)

func TestGenerateEdgeMap(t *testing.T) {
	type testcase struct {
		lines   [][]Line
		edgemap EdgeMap
	}
	tests := tbltest.Cases(
		testcase{
			lines: [][]Line{
				{
					{Pt{4, 4}, Pt{4, 9}},
					{Pt{4, 9}, Pt{5, 9}},
					{Pt{5, 9}, Pt{5, 4}},
					//	Line{Pt{5, 4}, Pt{4, 4}},
				},
				{
					{Pt{3, 1}, Pt{3, 6}},
					{Pt{3, 6}, Pt{7, 6}},
					{Pt{7, 6}, Pt{7, 1}},
					//	Line{Pt{7, 1}, Pt{3, 1}},
				},
			},
			edgemap: EdgeMap{
				BBox: [4]Pt{{0 - adjustBBoxBy, 0 - adjustBBoxBy}, {7 + adjustBBoxBy, 0 - adjustBBoxBy}, {7 + adjustBBoxBy, 9 + adjustBBoxBy}, {0 - adjustBBoxBy, 9 + adjustBBoxBy}},
				Keys: []Pt{
					{0 - adjustBBoxBy, 0 - adjustBBoxBy}, {0 - adjustBBoxBy, 9 + adjustBBoxBy}, {3, 1}, {3, 6}, {4, 4}, {4, 6}, {4, 9}, {5, 4}, {5, 6}, {5, 9}, {7, 1}, {7, 6}, {7 + adjustBBoxBy, 0 - adjustBBoxBy}, {7 + adjustBBoxBy, 9 + adjustBBoxBy},
				},
				Map: map[Pt]map[Pt]bool{
					Pt{0 - adjustBBoxBy, 0 - adjustBBoxBy}: map[Pt]bool{
						Pt{7 + adjustBBoxBy, 0 - adjustBBoxBy}: false,
						Pt{0 - adjustBBoxBy, 9 + adjustBBoxBy}: false,
						Pt{3, 1}: false,
						Pt{3, 6}: false,
						Pt{4, 9}: false,
						Pt{7, 1}: false,
					},
					Pt{0 - adjustBBoxBy, 9 + adjustBBoxBy}: map[Pt]bool{
						Pt{0 - adjustBBoxBy, 0 - adjustBBoxBy}: false,
						Pt{7 + adjustBBoxBy, 9 + adjustBBoxBy}: false,
						Pt{4, 9}: false,
						Pt{5, 9}: false,
					},
					Pt{3, 1}: map[Pt]bool{
						Pt{3, 6}: true,
						Pt{7, 1}: true,
						Pt{0 - adjustBBoxBy, 0 - adjustBBoxBy}: false,
						Pt{4, 4}: false,
						Pt{4, 6}: false,
						Pt{5, 4}: false,
						Pt{7, 6}: false,
					},
					Pt{3, 6}: map[Pt]bool{
						Pt{3, 1}: true,
						Pt{4, 6}: true,
						Pt{0 - adjustBBoxBy, 0 - adjustBBoxBy}: false,
						Pt{4, 9}: false,
					},
					Pt{4, 4}: map[Pt]bool{
						Pt{4, 6}: true,
						Pt{5, 4}: true,
						Pt{3, 1}: false,
						Pt{5, 6}: false,
					},
					Pt{4, 6}: map[Pt]bool{
						Pt{3, 6}: true,
						Pt{4, 4}: true,
						Pt{4, 9}: true,
						Pt{5, 6}: true,
						Pt{3, 1}: false,
						Pt{5, 9}: false,
					},
					Pt{4, 9}: map[Pt]bool{
						Pt{4, 6}: true,
						Pt{5, 9}: true,
						Pt{0 - adjustBBoxBy, 0 - adjustBBoxBy}: false,
						Pt{0 - adjustBBoxBy, 9 + adjustBBoxBy}: false,
						Pt{3, 6}: false,
					},
					Pt{5, 4}: map[Pt]bool{
						Pt{4, 4}: true,
						Pt{5, 6}: true,
						Pt{3, 1}: false,
						Pt{7, 6}: false,
					},
					Pt{5, 6}: map[Pt]bool{
						Pt{4, 6}: true,
						Pt{5, 4}: true,
						Pt{5, 9}: true,
						Pt{7, 6}: true,
						Pt{4, 4}: false,
						Pt{7 + adjustBBoxBy, 9 + adjustBBoxBy}: false,
					},
					Pt{5, 9}: map[Pt]bool{
						Pt{4, 9}: true,
						Pt{5, 6}: true,
						Pt{0 - adjustBBoxBy, 9 + adjustBBoxBy}: false,
						Pt{7 + adjustBBoxBy, 9 + adjustBBoxBy}: false,
						Pt{4, 6}: false,
					},
					Pt{7, 1}: map[Pt]bool{
						Pt{3, 1}: true,
						Pt{7, 6}: true,
						Pt{0 - adjustBBoxBy, 0 - adjustBBoxBy}: false,
						Pt{7 + adjustBBoxBy, 0 - adjustBBoxBy}: false,
						Pt{7 + adjustBBoxBy, 9 + adjustBBoxBy}: false,
					},
					Pt{7, 6}: map[Pt]bool{
						Pt{5, 6}: true,
						Pt{7, 1}: true,
						Pt{3, 1}: false,
						Pt{5, 4}: false,
						Pt{7 + adjustBBoxBy, 9 + adjustBBoxBy}: false,
					},
					Pt{7 + adjustBBoxBy, 0 - adjustBBoxBy}: map[Pt]bool{
						Pt{0 - adjustBBoxBy, 0 - adjustBBoxBy}: false,
						Pt{7 + adjustBBoxBy, 9 + adjustBBoxBy}: false,
						Pt{7, 1}: false,
					},

					Pt{7 + adjustBBoxBy, 9 + adjustBBoxBy}: map[Pt]bool{
						Pt{0 - adjustBBoxBy, 9 + adjustBBoxBy}: false,
						Pt{5, 6}: false,
						Pt{5, 9}: false,
						Pt{7, 1}: false,
						Pt{7, 6}: false,
						Pt{7 + adjustBBoxBy, 0 - adjustBBoxBy}: false,
					},
				},
				Segments: []Line{
					{Pt{0 - adjustBBoxBy, 0 - adjustBBoxBy}, Pt{7 + adjustBBoxBy, 9 + adjustBBoxBy}},
					{Pt{7 + adjustBBoxBy, 0 - adjustBBoxBy}, Pt{7 + adjustBBoxBy, 9 + adjustBBoxBy}},
					{Pt{7 + adjustBBoxBy, 9 + adjustBBoxBy}, Pt{0 - adjustBBoxBy, 9 + adjustBBoxBy}},
					{Pt{0 - adjustBBoxBy, 9 + adjustBBoxBy}, Pt{0 - adjustBBoxBy, 0 - adjustBBoxBy}},
					{Pt{3, 1}, Pt{3, 6}},
					{Pt{3, 1}, Pt{7, 1}},
					{Pt{3, 6}, Pt{4, 6}},
					{Pt{4, 4}, Pt{4, 6}},
					{Pt{4, 4}, Pt{5, 4}},
					{Pt{4, 6}, Pt{4, 9}},
					{Pt{4, 6}, Pt{5, 6}},
					{Pt{4, 9}, Pt{5, 9}},
					{Pt{5, 4}, Pt{5, 6}},
					{Pt{5, 6}, Pt{5, 9}},
					{Pt{5, 6}, Pt{7, 6}},
					{Pt{7, 1}, Pt{7, 6}},
					{Pt{0 - adjustBBoxBy, 0 - adjustBBoxBy}, Pt{3, 1}},
					{Pt{0 - adjustBBoxBy, 0 - adjustBBoxBy}, Pt{3, 6}},
					{Pt{0 - adjustBBoxBy, 0 - adjustBBoxBy}, Pt{4, 9}},
					{Pt{0 - adjustBBoxBy, 0 - adjustBBoxBy}, Pt{7, 1}},
					{Pt{0 - adjustBBoxBy, 9 + adjustBBoxBy}, Pt{4, 9}},
					{Pt{0 - adjustBBoxBy, 9 + adjustBBoxBy}, Pt{5, 9}},
					{Pt{3, 1}, Pt{4, 4}},
					{Pt{3, 1}, Pt{4, 6}},
					{Pt{3, 1}, Pt{5, 4}},
					{Pt{3, 1}, Pt{7, 6}},
					{Pt{3, 6}, Pt{4, 9}},
					{Pt{4, 4}, Pt{5, 6}},
					{Pt{4, 6}, Pt{5, 9}},
					{Pt{5, 4}, Pt{7, 6}},
					{Pt{5, 6}, Pt{7 + adjustBBoxBy, 9 + adjustBBoxBy}},
					{Pt{5, 9}, Pt{7 + adjustBBoxBy, 9 + adjustBBoxBy}},
					{Pt{7, 1}, Pt{7 + adjustBBoxBy, 0 - adjustBBoxBy}},
					{Pt{7, 1}, Pt{7 + adjustBBoxBy, 9 + adjustBBoxBy}},
					{Pt{7, 6}, Pt{7 + adjustBBoxBy, 9 + adjustBBoxBy}},
				},
			},
		},
	)

	tests.Run(func(idx int, test testcase) {
		polygons := destructure(insureConnected(test.lines...))

		em := generateEdgeMap(polygons)
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

func TestTrianglesForEdge(t *testing.T) {

	type edgecase struct {
		pts       [2]Pt
		err       error
		triangles [2]*Triangle
	}
	T := func(pt1, pt2, pt3, pt4 Pt) edgecase {
		t1 := NewTriangle(pt1, pt2, pt3)
		t2 := NewTriangle(pt1, pt2, pt4)
		return edgecase{
			pts:       [2]Pt{pt1, pt2},
			triangles: [2]*Triangle{&t1, &t2},
		}
	}
	NT := func(pt1, pt2, pt4 Pt) edgecase {
		t2 := NewTriangle(pt1, pt2, pt4)
		return edgecase{
			pts:       [2]Pt{pt1, pt2},
			triangles: [2]*Triangle{nil, &t2},
		}
	}
	TN := func(pt1, pt2, pt3 Pt) edgecase {
		t1 := NewTriangle(pt1, pt2, pt3)
		return edgecase{
			pts:       [2]Pt{pt1, pt2},
			triangles: [2]*Triangle{&t1, nil},
		}
	}
	_, _, _ = T, NT, TN
	type testcase struct {
		lines       []Line
		triangulate bool
		edges       []edgecase
	}
	tests := tbltest.Cases(
		/*
			testcase{
				lines: []Line{
					// A
					{Pt{0, 0}, Pt{7, -2}}, // B
					{Pt{0, 0}, Pt{5, -5}}, // C
					{Pt{0, 0}, Pt{5, 1}},  // D
					{Pt{0, 0}, Pt{4, 0}},  // E
					{Pt{0, 0}, Pt{5, -3}}, // F
					{Pt{0, 0}, Pt{4, 4}},  // G
					// B
					{Pt{7, -2}, Pt{5, -5}},
					{Pt{7, -2}, Pt{5, 1}},
					{Pt{7, -2}, Pt{4, 0}},
					{Pt{7, -2}, Pt{5, -3}},
					{Pt{7, -2}, Pt{4, 4}},
					// C
					{Pt{5, -5}, Pt{5, -3}},
					// D
					{Pt{5, 1}, Pt{4, 0}},
					{Pt{5, 1}, Pt{4, 4}},
				},
				edges: []edgecase{
					// AB
					T(Pt{0, 0}, Pt{7, -2}, Pt{5, -3}, Pt{4, 0}),
					// BA
					T(Pt{7, -2}, Pt{0, 0}, Pt{4, 0}, Pt{5, -3}),
					// AC
					NT(Pt{0, 0}, Pt{5, -5}, Pt{5, -3}),
					// AD
					T(Pt{0, 0}, Pt{5, 1}, Pt{4, 0}, Pt{4, 4}),
					// AE
					T(Pt{0, 0}, Pt{4, 0}, Pt{7, -2}, Pt{5, 1}),
					// AF
					T(Pt{0, 0}, Pt{5, -3}, Pt{5, -5}, Pt{7, -2}),
					// AG
					TN(Pt{0, 0}, Pt{4, 4}, Pt{5, 1}),
					// BC
					TN(Pt{7, -2}, Pt{5, -5}, Pt{5, -3}),
				},
			},
		*/
		testcase{
			triangulate: true,
			lines: []Line{
				Line{Pt{X: 0, Y: 0}, Pt{X: 258, Y: -52}},
				Line{Pt{X: 0, Y: 0}, Pt{X: 258, Y: -51}},
				Line{Pt{X: 0, Y: 0}, Pt{X: 258, Y: -51}},
				Line{Pt{X: 0, Y: 0}, Pt{X: 258, Y: -22}},
				Line{Pt{X: 0, Y: 0}, Pt{X: 283, Y: 42}},
				Line{Pt{X: 0, Y: 0}, Pt{X: 283, Y: 42}},
				Line{Pt{X: 0, Y: 0}, Pt{X: 283, Y: 43}},
				Line{Pt{X: 0, Y: 0}, Pt{X: 283, Y: 43}},
				Line{Pt{X: 0, Y: 0}, Pt{X: 299, Y: -52}},
				Line{Pt{X: 0, Y: 0}, Pt{X: 299, Y: -52}},
				Line{Pt{X: 0, Y: 0}, Pt{X: 299, Y: -51}},
				Line{Pt{X: 0, Y: 0}, Pt{X: 299, Y: -51}},
				Line{Pt{X: 0, Y: 0}, Pt{X: 299, Y: -28}},
				Line{Pt{X: 0, Y: 0}, Pt{X: 299, Y: -27}},
				Line{Pt{X: 193, Y: 14}, Pt{X: 193, Y: 15}},
				Line{Pt{X: 193, Y: 14}, Pt{X: 217, Y: -22}},
				Line{Pt{X: 193, Y: 15}, Pt{X: 213, Y: 55}},
				Line{Pt{X: 213, Y: 55}, Pt{X: 214, Y: 55}},
				Line{Pt{X: 214, Y: 55}, Pt{X: 250, Y: 93}},
				Line{Pt{X: 217, Y: -23}, Pt{X: 217, Y: -22}},
				Line{Pt{X: 217, Y: -23}, Pt{X: 257, Y: -22}},
				Line{Pt{X: 250, Y: 93}, Pt{X: 253, Y: 89}},
				Line{Pt{X: 253, Y: 89}, Pt{X: 261, Y: 79}},
				Line{Pt{X: 257, Y: -22}, Pt{X: 258, Y: -22}},
				Line{Pt{X: 258, Y: -52}, Pt{X: 299, Y: -51}},
				Line{Pt{X: 261, Y: 79}, Pt{X: 262, Y: 79}},
				Line{Pt{X: 262, Y: 79}, Pt{X: 271, Y: 83}},
				Line{Pt{X: 263, Y: 53}, Pt{X: 264, Y: 53}},
				Line{Pt{X: 263, Y: 53}, Pt{X: 271, Y: 82}},
				Line{Pt{X: 264, Y: 53}, Pt{X: 282, Y: 53}},
				Line{Pt{X: 271, Y: 82}, Pt{X: 271, Y: 83}},
				Line{Pt{X: 282, Y: 53}, Pt{X: 283, Y: 52}},
				Line{Pt{X: 283, Y: 43}, Pt{X: 283, Y: 52}},
				Line{Pt{X: 283, Y: 43}, Pt{X: 290, Y: 42}},
				Line{Pt{X: 290, Y: 42}, Pt{X: 290, Y: 43}},
				Line{Pt{X: 290, Y: 43}, Pt{X: 295, Y: 54}},
				Line{Pt{X: 295, Y: 53}, Pt{X: 295, Y: 54}},
				Line{Pt{X: 295, Y: 53}, Pt{X: 307, Y: 55}},
				Line{Pt{X: 299, Y: -51}, Pt{X: 299, Y: -28}},
				Line{Pt{X: 299, Y: -27}, Pt{X: 324, Y: -31}},
				Line{Pt{X: 307, Y: 54}, Pt{X: 307, Y: 55}},
				Line{Pt{X: 307, Y: 54}, Pt{X: 313, Y: 47}},
				Line{Pt{X: 313, Y: 47}, Pt{X: 313, Y: 48}},
				Line{Pt{X: 313, Y: 48}, Pt{X: 315, Y: 56}},
				Line{Pt{X: 315, Y: 2}, Pt{X: 315, Y: 3}},
				Line{Pt{X: 315, Y: 2}, Pt{X: 329, Y: -18}},
				Line{Pt{X: 315, Y: 3}, Pt{X: 329, Y: 12}},
				Line{Pt{X: 315, Y: 56}, Pt{X: 316, Y: 56}},
				Line{Pt{X: 316, Y: 56}, Pt{X: 324, Y: 53}},
				Line{Pt{X: 324, Y: -31}, Pt{X: 325, Y: -31}},
				Line{Pt{X: 324, Y: 52}, Pt{X: 324, Y: 53}},
				Line{Pt{X: 324, Y: 52}, Pt{X: 330, Y: 12}},
				Line{Pt{X: 325, Y: -31}, Pt{X: 329, Y: -19}},
				Line{Pt{X: 329, Y: -19}, Pt{X: 329, Y: -18}},
				Line{Pt{X: 329, Y: 12}, Pt{X: 330, Y: 12}},
			},
			edges: []edgecase{
				// AB
				T(Pt{217, -23}, Pt{257, -22}, Pt{5, -3}, Pt{4, 0}),
			},
		},
	)

	tests.Run(func(idx int, test testcase) {
		em := generateEdgeMap(test.lines)
		log.Println("\tSegments:", len(em.Segments))
		for _, seg := range em.Segments {
			log.Println("\t\t", seg)
		}
		log.Println("\tSegments:", len(em.Segments))
		if test.triangulate {
			em.Triangulate()
		}
		log.Println("\tSegments:", len(em.Segments))
		for _, seg := range em.Segments {
			log.Println("\t\t", seg)
		}
		log.Println("\tSegments:", len(em.Segments))
		for _, ec := range test.edges {
			tr1, tr2, err := em.trianglesForEdge(ec.pts[0], ec.pts[1])
			if ec.err != nil {
				if err != ec.err {
					t.Errorf("Expected an error %v got %v", ec.err, err)
				}
				continue
			}
			if err != nil {
				t.Errorf("Got unexpected error: %v", ec.err, err)
				continue
			}
			if !(tr1.Equal(ec.triangles[0]) && tr2.Equal(ec.triangles[1])) {
				//em.Dump()
				t.Errorf("Expected: \n\t%v\n\t%v\nGot: \n\t%v\n\t%v",
					ec.triangles[0], ec.triangles[1],
					tr1, tr2,
				)
			}
		}

	})

}

func TestFindTriangles(t *testing.T) {
	type testcase struct {
		EdgeMap       *EdgeMap
		expectedGraph *TriangleGraph
	}
	type genTestCase struct {
		lines         []Line
		expectedGraph *TriangleGraph
	}
	genCases := []genTestCase{
		{
			lines: []Line{
				Line{Pt{X: 0, Y: 0}, Pt{X: 258, Y: -52}},
				Line{Pt{X: 0, Y: 0}, Pt{X: 258, Y: -51}},
				Line{Pt{X: 0, Y: 0}, Pt{X: 258, Y: -51}},
				Line{Pt{X: 0, Y: 0}, Pt{X: 258, Y: -22}},
				Line{Pt{X: 0, Y: 0}, Pt{X: 283, Y: 42}},
				Line{Pt{X: 0, Y: 0}, Pt{X: 283, Y: 42}},
				Line{Pt{X: 0, Y: 0}, Pt{X: 283, Y: 43}},
				Line{Pt{X: 0, Y: 0}, Pt{X: 283, Y: 43}},
				Line{Pt{X: 0, Y: 0}, Pt{X: 299, Y: -52}},
				Line{Pt{X: 0, Y: 0}, Pt{X: 299, Y: -52}},
				Line{Pt{X: 0, Y: 0}, Pt{X: 299, Y: -51}},
				Line{Pt{X: 0, Y: 0}, Pt{X: 299, Y: -51}},
				Line{Pt{X: 0, Y: 0}, Pt{X: 299, Y: -28}},
				Line{Pt{X: 0, Y: 0}, Pt{X: 299, Y: -27}},
				Line{Pt{X: 193, Y: 14}, Pt{X: 193, Y: 15}},
				Line{Pt{X: 193, Y: 14}, Pt{X: 217, Y: -22}},
				Line{Pt{X: 193, Y: 15}, Pt{X: 213, Y: 55}},
				Line{Pt{X: 213, Y: 55}, Pt{X: 214, Y: 55}},
				Line{Pt{X: 214, Y: 55}, Pt{X: 250, Y: 93}},
				Line{Pt{X: 217, Y: -23}, Pt{X: 217, Y: -22}},
				Line{Pt{X: 217, Y: -23}, Pt{X: 257, Y: -22}},
				Line{Pt{X: 250, Y: 93}, Pt{X: 253, Y: 89}},
				Line{Pt{X: 253, Y: 89}, Pt{X: 261, Y: 79}},
				Line{Pt{X: 257, Y: -22}, Pt{X: 258, Y: -22}},
				Line{Pt{X: 258, Y: -52}, Pt{X: 299, Y: -51}},
				Line{Pt{X: 261, Y: 79}, Pt{X: 262, Y: 79}},
				Line{Pt{X: 262, Y: 79}, Pt{X: 271, Y: 83}},
				Line{Pt{X: 263, Y: 53}, Pt{X: 264, Y: 53}},
				Line{Pt{X: 263, Y: 53}, Pt{X: 271, Y: 82}},
				Line{Pt{X: 264, Y: 53}, Pt{X: 282, Y: 53}},
				Line{Pt{X: 271, Y: 82}, Pt{X: 271, Y: 83}},
				Line{Pt{X: 282, Y: 53}, Pt{X: 283, Y: 52}},
				Line{Pt{X: 283, Y: 43}, Pt{X: 283, Y: 52}},
				Line{Pt{X: 283, Y: 43}, Pt{X: 290, Y: 42}},
				Line{Pt{X: 290, Y: 42}, Pt{X: 290, Y: 43}},
				Line{Pt{X: 290, Y: 43}, Pt{X: 295, Y: 54}},
				Line{Pt{X: 295, Y: 53}, Pt{X: 295, Y: 54}},
				Line{Pt{X: 295, Y: 53}, Pt{X: 307, Y: 55}},
				Line{Pt{X: 299, Y: -51}, Pt{X: 299, Y: -28}},
				Line{Pt{X: 299, Y: -27}, Pt{X: 324, Y: -31}},
				Line{Pt{X: 307, Y: 54}, Pt{X: 307, Y: 55}},
				Line{Pt{X: 307, Y: 54}, Pt{X: 313, Y: 47}},
				Line{Pt{X: 313, Y: 47}, Pt{X: 313, Y: 48}},
				Line{Pt{X: 313, Y: 48}, Pt{X: 315, Y: 56}},
				Line{Pt{X: 315, Y: 2}, Pt{X: 315, Y: 3}},
				Line{Pt{X: 315, Y: 2}, Pt{X: 329, Y: -18}},
				Line{Pt{X: 315, Y: 3}, Pt{X: 329, Y: 12}},
				Line{Pt{X: 315, Y: 56}, Pt{X: 316, Y: 56}},
				Line{Pt{X: 316, Y: 56}, Pt{X: 324, Y: 53}},
				Line{Pt{X: 324, Y: -31}, Pt{X: 325, Y: -31}},
				Line{Pt{X: 324, Y: 52}, Pt{X: 324, Y: 53}},
				Line{Pt{X: 324, Y: 52}, Pt{X: 330, Y: 12}},
				Line{Pt{X: 325, Y: -31}, Pt{X: 329, Y: -19}},
				Line{Pt{X: 329, Y: -19}, Pt{X: 329, Y: -18}},
				Line{Pt{X: 329, Y: 12}, Pt{X: 330, Y: 12}},
			},
		},
	}

	cases := []testcase{
		testcase{
			EdgeMap: &EdgeMap{
				BBox: [4]Pt{{-10, -10}, {17, -100}, {17, 19}, {-10, 19}},
				Keys: []Pt{
					{-10, -10}, {-10, 19}, {3, 1}, {3, 6}, {4, 4}, {4, 6}, {4, 9}, {5, 4}, {5, 6}, {5, 9}, {7, 1}, {7, 6}, {17, -10}, {17, 19},
				},
				Map: map[Pt]map[Pt]bool{
					Pt{-10, -10}: map[Pt]bool{
						Pt{17, -10}: false,
						Pt{-10, 19}: false,
						Pt{3, 1}:    false,
						Pt{3, 6}:    false,
						Pt{4, 9}:    false,
						Pt{7, 1}:    false,
					},
					Pt{-10, 19}: map[Pt]bool{
						Pt{17, 19}:   false,
						Pt{-10, -10}: false,
						Pt{4, 9}:     false,
						Pt{5, 9}:     false,
					},
					Pt{3, 1}: map[Pt]bool{
						Pt{4, 6}:     false,
						Pt{5, 4}:     false,
						Pt{7, 6}:     false,
						Pt{3, 6}:     true,
						Pt{7, 1}:     true,
						Pt{-10, -10}: false,
						Pt{4, 4}:     false,
					},
					Pt{3, 6}: map[Pt]bool{
						Pt{3, 1}:     true,
						Pt{4, 6}:     true,
						Pt{-10, -10}: false,
						Pt{4, 9}:     false,
					},
					Pt{4, 4}: map[Pt]bool{
						Pt{4, 6}: true,
						Pt{5, 4}: true,
						Pt{3, 1}: false,
						Pt{5, 6}: false,
					},
					Pt{4, 6}: map[Pt]bool{
						Pt{3, 6}: true,
						Pt{4, 4}: true,
						Pt{4, 9}: true,
						Pt{5, 6}: true,
						Pt{3, 1}: false,
						Pt{5, 9}: false,
					},
					Pt{4, 9}: map[Pt]bool{
						Pt{3, 6}:     false,
						Pt{4, 6}:     true,
						Pt{5, 9}:     true,
						Pt{-10, -10}: false,
						Pt{-10, 19}:  false,
					},
					Pt{5, 4}: map[Pt]bool{
						Pt{4, 4}: true,
						Pt{5, 6}: true,
						Pt{3, 1}: false,
						Pt{7, 6}: false,
					},
					Pt{5, 6}: map[Pt]bool{
						Pt{4, 6}:   true,
						Pt{5, 4}:   true,
						Pt{5, 9}:   true,
						Pt{7, 6}:   true,
						Pt{4, 4}:   false,
						Pt{17, 19}: false,
					},
					Pt{5, 9}: map[Pt]bool{
						Pt{4, 9}:    true,
						Pt{5, 6}:    true,
						Pt{-10, 19}: false,
						Pt{4, 6}:    false,
						Pt{17, 19}:  false,
					},
					Pt{7, 1}: map[Pt]bool{
						Pt{3, 1}:     true,
						Pt{7, 6}:     true,
						Pt{-10, -10}: false,
						Pt{17, -10}:  false,
						Pt{17, 19}:   false,
					},
					Pt{7, 6}: map[Pt]bool{
						Pt{5, 6}:   true,
						Pt{7, 1}:   true,
						Pt{3, 1}:   false,
						Pt{5, 4}:   false,
						Pt{17, 19}: false,
					},
					Pt{17, -10}: map[Pt]bool{
						Pt{-10, -10}: false,
						Pt{17, 19}:   false,
						Pt{7, 1}:     false,
					},

					Pt{17, 19}: map[Pt]bool{
						Pt{-10, 109}: false,
						Pt{5, 6}:     false,
						Pt{5, 9}:     false,
						Pt{7, 1}:     false,
						Pt{7, 6}:     false,
						Pt{17, -10}:  false,
					},
				},
				Segments: []Line{
					{Pt{-10, -10}, Pt{17, -10}},
					{Pt{17, -10}, Pt{17, 19}},
					{Pt{17, 19}, Pt{-10, 19}},
					{Pt{-10, 19}, Pt{-10, -10}},
					{Pt{3, 1}, Pt{3, 6}},
					{Pt{3, 1}, Pt{7, 1}},
					{Pt{3, 6}, Pt{4, 6}},
					{Pt{4, 4}, Pt{4, 6}},
					{Pt{4, 4}, Pt{5, 4}},
					{Pt{4, 6}, Pt{4, 9}},
					{Pt{4, 6}, Pt{5, 6}},
					{Pt{4, 9}, Pt{5, 9}},
					{Pt{5, 4}, Pt{5, 6}},
					{Pt{5, 6}, Pt{5, 9}},
					{Pt{5, 6}, Pt{7, 6}},
					{Pt{7, 1}, Pt{7, 6}},
					{Pt{-10, -10}, Pt{3, 1}},
					{Pt{-10, -10}, Pt{3, 6}},
					{Pt{-10, -10}, Pt{4, 9}},
					{Pt{-10, -10}, Pt{7, 1}},
					{Pt{-10, 19}, Pt{4, 9}},
					{Pt{-10, 19}, Pt{5, 9}},
					{Pt{3, 1}, Pt{4, 4}},
					{Pt{3, 1}, Pt{4, 6}},
					{Pt{3, 1}, Pt{5, 4}},
					{Pt{3, 1}, Pt{7, 6}},
					{Pt{3, 6}, Pt{4, 9}},
					{Pt{4, 4}, Pt{5, 6}},
					{Pt{4, 6}, Pt{5, 9}},
					{Pt{5, 4}, Pt{7, 6}},
					{Pt{5, 6}, Pt{17, 19}},
					{Pt{5, 9}, Pt{17, 19}},
					{Pt{7, 1}, Pt{17, -10}},
					{Pt{7, 1}, Pt{17, 19}},
					{Pt{7, 6}, Pt{17, 19}},
				},
			},
			expectedGraph: &TriangleGraph{},
		},
	}
	for _, acase := range genCases {
		em := generateEdgeMap(acase.lines)
		em.Triangulate()
		cases = append(cases, testcase{EdgeMap: &em, expectedGraph: acase.expectedGraph})
	}

	var tcases []tbltest.TestCase
	for _, ac := range cases {
		tcases = append(tcases, tbltest.TestCase(ac))
	}

	tests := tbltest.Cases(tcases...)

	tests.Run(func(idx int, test testcase) {

		got, err := test.EdgeMap.FindTriangles()
		log.Println(err)
		log.Printf("%+v\n", got)
		if got == nil {
			return
		}
		for i, v := range got.triangles {
			log.Println("i:", i)
			v.Dump()
		}
		log.Println("BBox:", test.EdgeMap.BBox)
	})
}

func TestPointPairs(t *testing.T) {
	type testcase struct {
		pts      []Pt
		expected [][2]Pt
		err      error
	}
	tests := tbltest.Cases(
		testcase{
			pts: []Pt{
				{1, 1}, {1, 2}, {1, 3}, {1, 4},
			},
			expected: [][2]Pt{
				{Pt{1, 1}, Pt{1, 2}},
				{Pt{1, 1}, Pt{1, 3}},
				{Pt{1, 1}, Pt{1, 4}},
				{Pt{1, 2}, Pt{1, 3}},
				{Pt{1, 2}, Pt{1, 4}},
				{Pt{1, 3}, Pt{1, 4}},
			},
		},
	)

	tests.Run(func(idx int, test testcase) {
		got, err := PointPairs(test.pts)
		if test.err != err {
			t.Error("Expected an error %v but got %v", test.err, err)
		}
		if test.err != nil && !reflect.DeepEqual(test.expected, got) {
			t.Error("Expected\n\t", test.expected, "\ngot\n\t", got)
		}
	})
}

func TestDestructure(t *testing.T) {
	type testcase struct {
		lines    [][]Line
		expected []Line
	}
	tests := tbltest.Cases(
		testcase{
			lines: [][]Line{
				{
					{Pt{3, 6}, Pt{7, 6}},
					{Pt{4, 4}, Pt{4, 9}},
				},
			},
			expected: []Line{
				{Pt{3, 6}, Pt{4, 6}},
				{Pt{4, 4}, Pt{4, 6}},
				{Pt{4, 6}, Pt{7, 6}},
				{Pt{4, 6}, Pt{4, 9}},
			},
		},
		testcase{
			lines: [][]Line{
				{
					{Pt{3, 1}, Pt{7, 1}},
					{Pt{7, 1}, Pt{7, 6}},
					{Pt{7, 6}, Pt{3, 6}},
					{Pt{3, 6}, Pt{3, 1}},
				},
				{
					{Pt{4, 4}, Pt{5, 4}},
					{Pt{5, 4}, Pt{5, 9}},
					{Pt{5, 9}, Pt{4, 9}},
					{Pt{4, 9}, Pt{4, 4}},
				},
			},
			expected: []Line{
				{Pt{3, 1}, Pt{7, 1}},
				{Pt{3, 1}, Pt{3, 6}},
				{Pt{3, 6}, Pt{4, 6}},
				{Pt{4, 4}, Pt{4, 6}},
				{Pt{4, 4}, Pt{5, 4}},
				{Pt{4, 6}, Pt{5, 6}},
				{Pt{4, 6}, Pt{4, 9}},
				{Pt{4, 9}, Pt{5, 9}},
				{Pt{5, 4}, Pt{5, 6}},
				{Pt{5, 6}, Pt{5, 9}},
				{Pt{5, 6}, Pt{7, 6}},
				{Pt{7, 1}, Pt{7, 6}},
			},
		},
		testcase{
			lines: [][]Line{
				{
					{Pt{3, 1}, Pt{7, 1}},
					{Pt{7, 1}, Pt{7, 6}},
					{Pt{7, 6}, Pt{7, 1}}, // This is a bad line. Make sure that destruct can handle this.
					{Pt{7, 6}, Pt{3, 6}},
					{Pt{3, 6}, Pt{3, 1}},
				},
				{
					{Pt{4, 4}, Pt{5, 4}},
					{Pt{5, 4}, Pt{5, 9}},
					{Pt{5, 9}, Pt{4, 9}},
					{Pt{4, 9}, Pt{4, 4}},
				},
			},
			expected: []Line{
				{Pt{3, 1}, Pt{7, 1}},
				{Pt{3, 1}, Pt{3, 6}},
				{Pt{3, 6}, Pt{4, 6}},
				{Pt{4, 4}, Pt{4, 6}},
				{Pt{4, 4}, Pt{5, 4}},
				{Pt{4, 6}, Pt{5, 6}},
				{Pt{4, 6}, Pt{4, 9}},
				{Pt{4, 9}, Pt{5, 9}},
				{Pt{5, 4}, Pt{5, 6}},
				{Pt{5, 6}, Pt{5, 9}},
				{Pt{5, 6}, Pt{7, 6}},
				{Pt{7, 1}, Pt{7, 6}},
			},
		},
	)
	tests.Run(func(idx int, test testcase) {
		got := destructure(test.lines)
		sort.Sort(ByXYLine(test.expected))
		if !reflect.DeepEqual(test.expected, got) {
			t.Error("Expected\n\t", test.expected, "\ngot\n\t", got)
		}
	})
}

func TestMakeValid(t *testing.T) {

	type testcase struct {
		lines    [][]Line
		polygons [][][]Pt
		err      error
	}

	tests := tbltest.Cases(
		testcase{
			lines: [][]Line{
				{
					{Pt{4, 4}, Pt{4, 9}},
					{Pt{4, 9}, Pt{5, 9}},
					{Pt{5, 9}, Pt{5, 4}},
					//	Line{Pt{5, 4}, Pt{4, 4}},
				},
				{
					{Pt{3, 1}, Pt{3, 6}},
					{Pt{3, 6}, Pt{7, 6}},
					{Pt{7, 6}, Pt{7, 1}},
					//	Line{Pt{7, 1}, Pt{3, 1}},
				},
			},
			polygons: [][][]Pt{
				{
					[]Pt{{3, 1}, {3, 6}, {4, 6}, {4, 4}, {5, 4}, {5, 6}, {7, 6}, {7, 1}},
				},
				{
					[]Pt{{4, 6}, {4, 9}, {5, 9}, {5, 6}},
				},
			},
		},
	)

	tests.Run(func(idx int, test testcase) {
		got, err := makeValid(test.lines...)
		if test.err != nil && err != test.err {
			t.Errorf("Expecting and err %v, got: %v", test.err, err)
			return
		}
		if diff := deep.Equal(got, test.polygons); diff != nil {
			t.Error("Points do not match: Expected\n\t", test.polygons, "\ngot\n\t", got, "\n\tdiff:\t", diff)
		}
	})

}
