package plyg

import (
	"context"
	"fmt"
	"log"
	"reflect"
	"testing"

	"github.com/gdey/tbltest"
	"github.com/terranodo/tegola/maths"
	"github.com/terranodo/tegola/maths/hitmap"
)

func ringDiff(got, expected *RingCol, cmpYEdges bool) (bool, string) {
	if got == nil && expected != nil {
		return true, "got is nil, and expected is not nil."
	}
	if expected == nil && got != nil {
		return true, "got is not nil, and expected is nil."
	}

	gRingsDiff := "Rings are different:\n"
	gRingsDiff += fmt.Sprintf("Got: (%v)", len(got.Rings))
	for i := range got.Rings {
		gRingsDiff += fmt.Sprintf("\n\t%v", got.Rings[i])
	}
	gRingsDiff += fmt.Sprintf("\nExpected: (%v)", len(expected.Rings))
	for i := range expected.Rings {
		gRingsDiff += fmt.Sprintf("\n\t%v", expected.Rings[i])
	}

	if !reflect.DeepEqual(got.Rings, expected.Rings) {
		return true, gRingsDiff
	}
	if got.X1 != expected.X1 {
		return true, fmt.Sprintf("X1s are different:\nGot:\n\t%v\nExpected:\n\t%v", got.X1, expected.X1)

	}
	if got.X2 != expected.X2 {
		return true, fmt.Sprintf("X2s are different:\nGot:\n\t%v\nExpected:\n\t%v", got.X2, expected.X2)

	}
	if !cmpYEdges {
		return false, ""
	}
	if !reflect.DeepEqual(got.Y1s, expected.Y1s) {
		return true, fmt.Sprintf("Y1s are different:\nGot:\n\t%v\nExpected:\n\t%v", got.Y1s, expected.Y1s)
	}
	if !reflect.DeepEqual(got.Y2s, expected.Y2s) {
		return true, fmt.Sprintf("Y1s are different:\nGot:\n\t%v\nExpected:\n\t%v", got.Y2s, expected.Y2s)
	}

	return false, ""
}

func TestBuildRingCol(t *testing.T) {
	type testcase struct {
		desc   string
		hm     hitmap.Interface
		icols  [2][]maths.Pt
		pt2my  map[maths.Pt]int64
		testYs bool
		Col    RingCol
	}

	tests := tbltest.Cases(
		testcase{
			desc:   "Simple Rectangle",
			testYs: true,
			hm:     hitmap.AllwaysInside,
			icols: [2][]maths.Pt{
				[]maths.Pt{{0, 0}, {0, 1}},
				[]maths.Pt{{1, 0}, {1, 1}},
			},
			pt2my: map[maths.Pt]int64{},
			Col: RingCol{
				X1: 0,
				X2: 1,
				Rings: []Ring{
					{
						Label:  maths.Inside,
						Points: []maths.Pt{{0, 0}, {1, 0}, {1, 1}, {0, 1}},
					},
				},
				Y1s: []YEdge{
					{
						Y:     0,
						Descs: []RingDesc{{0, 0, maths.Inside}},
					},
					{
						Y:     1,
						Descs: []RingDesc{{0, 3, maths.Inside}},
					},
				},
				Y2s: []YEdge{
					{
						Y:     0,
						Descs: []RingDesc{{0, 1, maths.Inside}},
					},
					{
						Y:     1,
						Descs: []RingDesc{{0, 2, maths.Inside}},
					},
				},
			},
		},
		testcase{
			desc: "Simple Rectangle with constrined rightward line.",
			hm:   hitmap.AllwaysInside,
			icols: [2][]maths.Pt{
				[]maths.Pt{{0, 0}, {0, 1}},
				[]maths.Pt{{1, 0}, {1, 1}},
			},
			pt2my: map[maths.Pt]int64{
				{0, 0}: 100,
			},
			Col: RingCol{
				X1: 0,
				X2: 1,
				Rings: []Ring{
					{
						Label:  maths.Inside,
						Points: []maths.Pt{{0, 0}, {1, 0}, {1, 1}, {0, 1}},
					},
				},
			},
		},
		testcase{
			desc: "Simple Rectangle with constrined rightward line 1.",
			hm:   hitmap.AllwaysInside,
			icols: [2][]maths.Pt{
				[]maths.Pt{{1, 0}, {1, 1}},
				[]maths.Pt{{2, 0}, {2, 1}},
			},
			pt2my: map[maths.Pt]int64{
				{1, 0}: 100,
			},
			Col: RingCol{
				X1: 1,
				X2: 2,
				Rings: []Ring{
					{
						Label:  maths.Inside,
						Points: []maths.Pt{{1, 0}, {2, 0}, {2, 1}, {1, 1}},
					},
				},
			},
		},
		testcase{ // special case.
			desc: "Empty column (all outside) should be empty",
			hm:   new(hitmap.M), // Everything will be marked as outside
			icols: [2][]maths.Pt{
				[]maths.Pt{{0, 0}, {0, 1}, {0, 8}, {0, 9}},
				[]maths.Pt{{1, 0}, {1, 1}, {1, 2}, {1, 4}, {1, 5}, {1, 7}, {1, 8}, {1, 9}},
			},
			pt2my: map[maths.Pt]int64{
				{0, 0}: 0,
				{0, 1}: 100,
				{0, 8}: 800,
				{0, 9}: 900,
			},
			Col: RingCol{
				X1: 0,
				X2: 1,
			},
		},
		testcase{
			desc: "Number Eight col 0",
			hm: new(hitmap.M).AppendSegment(
				hitmap.NewSegmentFromRing(maths.Inside, []maths.Pt{{0, 1}, {4, 1}, {4, 8}, {0, 8}}),
				hitmap.NewSegmentFromRing(maths.Outside, []maths.Pt{{1, 2}, {3, 2}, {3, 4}, {1, 4}}),
				hitmap.NewSegmentFromRing(maths.Outside, []maths.Pt{{1, 5}, {3, 5}, {3, 7}, {1, 7}}),
			),
			icols: [2][]maths.Pt{
				[]maths.Pt{{0, 0}, {0, 1}, {0, 8}, {0, 9}},
				[]maths.Pt{{1, 0}, {1, 1}, {1, 2}, {1, 4}, {1, 5}, {1, 7}, {1, 8}, {1, 9}},
			},
			pt2my: map[maths.Pt]int64{
				{0, 0}: 0,
				{0, 1}: 100,
				{0, 8}: 800,
				{0, 9}: 900,
			},
			Col: RingCol{
				X1: 0,
				X2: 1,
				Rings: []Ring{
					{
						Label:  maths.Outside,
						Points: []maths.Pt{{0, 0}, {1, 0}, {1, 1}, {0, 1}},
					},
					{
						Label:  maths.Inside,
						Points: []maths.Pt{{0, 1}, {1, 1}, {1, 2}, {1, 4}, {1, 5}, {1, 7}, {1, 8}, {0, 8}},
					},
					{
						Label:  maths.Outside,
						Points: []maths.Pt{{0, 8}, {1, 8}, {1, 9}, {0, 9}},
					},
				},
			},
		},
	)
	tests.Run(func(idx int, test testcase) {
		log.Printf("Running %v (%v)", idx, test.desc)
		//var ys [2][]YEdge
		col1 := BuildRingCol(context.Background(), test.hm, test.icols[0], test.icols[1], test.pt2my)

		if ok, reason := ringDiff(&col1, &test.Col, test.testYs); ok {
			t.Errorf("For %v (%v) %v", idx, test.desc, reason)
		}
	})

}

func TestMerge2AdjecentRings(t *testing.T) {
	type testcase struct {
		desc   string
		hm     hitmap.Interface
		icols  [2][2][]maths.Pt
		pt2my  [2]map[maths.Pt]int64
		testYs bool
		Col    RingCol
	}
	// cases {{{1
	tests := tbltest.Cases(
		// Case Simple 2 {{{2
		testcase{
			desc: "Simple 2 Rectangle merge",
			hm:   hitmap.AllwaysInside,
			icols: [2][2][]maths.Pt{
				[2][]maths.Pt{
					[]maths.Pt{{0, 0}, {0, 1}},
					[]maths.Pt{{1, 0}, {1, 1}},
				},
				[2][]maths.Pt{
					[]maths.Pt{{1, 0}, {1, 1}},
					[]maths.Pt{{2, 0}, {2, 1}},
				},
			},
			pt2my: [2]map[maths.Pt]int64{
				map[maths.Pt]int64{
					{0, 0}: 100,
				},
				map[maths.Pt]int64{
					{1, 0}: 100,
				},
			},
			testYs: true,
			Col: RingCol{
				X1: 0,
				X2: 2,
				Rings: []Ring{
					{
						Label:  maths.Inside,
						Points: []maths.Pt{{0, 0}, {2, 0}, {2, 1}, {0, 1}},
					},
				},
				Y1s: []YEdge{
					{
						Y:     0,
						Descs: []RingDesc{{0, 0, maths.Inside}},
					},
					{
						Y:     1,
						Descs: []RingDesc{{0, 3, maths.Inside}},
					},
				},
				Y2s: []YEdge{
					{
						Y:     0,
						Descs: []RingDesc{{0, 1, maths.Inside}},
					},
					{
						Y:     1,
						Descs: []RingDesc{{0, 2, maths.Inside}},
					},
				},
			},
		},
		// case PackMan case 8
		testcase{
			desc: "PacMan case 8",
			hm: new(hitmap.M).AppendSegment(
				hitmap.NewSegmentFromRing(maths.Inside, []maths.Pt{{0, 1}, {1, 1}, {1, 2}}),
				hitmap.NewSegmentFromRing(maths.Inside, []maths.Pt{{0, 3}, {1, 2}, {2, 3}}),
			),
			icols: [2][2][]maths.Pt{
				[2][]maths.Pt{
					[]maths.Pt{{0, 0}, {0, 1}, {0, 3}},
					[]maths.Pt{{1, 0}, {1, 1}, {1, 2}, {1, 3}},
				},
				[2][]maths.Pt{
					[]maths.Pt{{1, 0}, {1, 1}, {1, 2}, {1, 3}},
					[]maths.Pt{{2, 0}, {2, 3}},
				},
			},
			pt2my: [2]map[maths.Pt]int64{
				map[maths.Pt]int64{
					{0, 1}: 200,
				},
				map[maths.Pt]int64{
					{1, 2}: 300,
				},
			},
			testYs: true,
			Col: RingCol{
				X1: 0,
				X2: 2,
				Rings: []Ring{
					{
						Label:  maths.Outside,
						Points: []maths.Pt{{0, 0}, {2, 0}, {2, 3}, {1, 2}, {1, 1}, {0, 1}},
					},
					{
						Label:  maths.Inside,
						Points: []maths.Pt{{0, 1}, {1, 1}, {1, 2}},
					},
					{
						Label:  maths.Outside,
						Points: []maths.Pt{{0, 1}, {1, 2}, {0, 3}},
					},
					{
						Label:  maths.Inside,
						Points: []maths.Pt{{0, 3}, {1, 2}, {2, 3}},
					},
				},
				Y1s: []YEdge{
					{
						Y:     0,
						Descs: []RingDesc{{0, 0, maths.Outside}},
					},
					{
						Y: 1,
						Descs: []RingDesc{
							{0, 5, maths.Outside},
							{1, 0, maths.Inside},
							{2, 0, maths.Outside},
						},
					},
					{
						Y: 3,
						Descs: []RingDesc{
							{2, 2, maths.Outside},
							{3, 0, maths.Inside},
						},
					},
				},
				Y2s: []YEdge{
					{
						Y:     0,
						Descs: []RingDesc{{0, 1, maths.Outside}},
					},
					{
						Y: 3,
						Descs: []RingDesc{
							{0, 2, maths.Outside},
							{3, 2, maths.Inside},
						},
					},
				},
			},
		},
		// Case The Letter E {{{2
		testcase{
			desc: "The Letter E",
			// HM {{{3
			hm: new(hitmap.M).AppendSegment(
				hitmap.NewSegmentFromRing(maths.Inside, []maths.Pt{{0, 1}, {4, 1}, {4, 8}, {0, 8}}),
				hitmap.NewSegmentFromRing(maths.Outside, []maths.Pt{{1, 2}, {3, 2}, {3, 4}, {1, 4}}),
				hitmap.NewSegmentFromRing(maths.Outside, []maths.Pt{{1, 5}, {3, 5}, {3, 7}, {1, 7}}),
			),
			// Points {{{3
			icols: [2][2][]maths.Pt{
				[2][]maths.Pt{
					[]maths.Pt{{0, 0}, {0, 1}, {0, 8}, {0, 9}},
					[]maths.Pt{{1, 0}, {1, 1}, {1, 2}, {1, 4}, {1, 5}, {1, 7}, {1, 8}, {1, 9}},
				},
				[2][]maths.Pt{
					[]maths.Pt{{1, 0}, {1, 1}, {1, 2}, {1, 4}, {1, 5}, {1, 7}, {1, 8}, {1, 9}},
					[]maths.Pt{{3, 0}, {3, 1}, {3, 2}, {3, 4}, {3, 5}, {3, 7}, {3, 8}, {3, 9}},
				},
			},
			// MaxY Point Map {{{3
			pt2my: [2]map[maths.Pt]int64{
				map[maths.Pt]int64{
					{0, 0}: 0,
					{0, 1}: 100,
					{0, 8}: 800,
					{0, 9}: 900,
				},
				map[maths.Pt]int64{
					{1, 0}: 0,
					{1, 1}: 100,
					{1, 2}: 200,
					{1, 4}: 400,
					{1, 5}: 500,
					{1, 7}: 700,
					{1, 8}: 800,
					{1, 9}: 900,
				},
			},
			testYs: true,
			// Col {{{3
			Col: RingCol{
				X1: 0,
				X2: 3,
				// Rings {{{4
				Rings: []Ring{
					{
						Label:  maths.Outside,
						Points: []maths.Pt{{0, 0}, {3, 0}, {3, 1}, {0, 1}},
					},
					{
						Label:  maths.Inside,
						Points: []maths.Pt{{0, 1}, {3, 1}, {3, 2}, {1, 2}, {1, 4}, {3, 4}, {3, 5}, {1, 5}, {1, 7}, {3, 7}, {3, 8}, {0, 8}},
					},
					{
						Label:  maths.Outside,
						Points: []maths.Pt{{0, 8}, {3, 8}, {3, 9}, {0, 9}},
					},
					{
						Label:  maths.Outside,
						Points: []maths.Pt{{1, 2}, {3, 2}, {3, 4}, {1, 4}},
					},
					{
						Label:  maths.Outside,
						Points: []maths.Pt{{1, 5}, {3, 5}, {3, 7}, {1, 7}},
					},
				},
				// Y1s {{{4
				Y1s: []YEdge{
					{
						Y:     0,
						Descs: []RingDesc{{0, 0, maths.Outside}},
					},
					{
						Y: 1,
						Descs: []RingDesc{
							{0, 3, maths.Outside},
							{1, 0, maths.Inside},
						},
					},
					{
						Y: 8,
						Descs: []RingDesc{
							{1, 11, maths.Inside},
							{2, 0, maths.Outside},
						},
					},
					{
						Y:     9,
						Descs: []RingDesc{{2, 3, maths.Outside}},
					},
				},
				// Y2s {{{4
				Y2s: []YEdge{
					{
						Y:     0,
						Descs: []RingDesc{{0, 1, maths.Outside}},
					},
					{
						Y: 1,
						Descs: []RingDesc{
							{0, 2, maths.Outside},
							{1, 1, maths.Inside},
						},
					},
					{
						Y: 2,
						Descs: []RingDesc{
							{1, 2, maths.Inside},
							{3, 1, maths.Outside},
						},
					},
					{
						Y: 4,
						Descs: []RingDesc{
							{1, 5, maths.Inside},
							{3, 2, maths.Outside},
						},
					},
					{
						Y: 5,
						Descs: []RingDesc{
							{1, 6, maths.Inside},
							{4, 1, maths.Outside},
						},
					},
					{
						Y: 7,
						Descs: []RingDesc{
							{1, 9, maths.Inside},
							{4, 2, maths.Outside},
						},
					},
					{
						Y: 8,
						Descs: []RingDesc{
							{1, 10, maths.Inside},
							{2, 1, maths.Outside},
						},
					},
					{
						Y: 9,
						Descs: []RingDesc{
							{2, 2, maths.Outside},
						},
					},
				},
			},
		},
		// }}}2
		// Case The Letter K {{{2
		testcase{
			desc: "The letter K",
			// HM {{{3
			hm: new(hitmap.M).AppendSegment(
				hitmap.NewSegmentFromRing(
					maths.Inside,
					[]maths.Pt{{0, 0}, {2, 0}, {2, 4}, {0, 4}},
				),
				hitmap.NewSegmentFromRing(
					maths.Outside,
					[]maths.Pt{{1, 2}, {2, 1}, {2, 3}},
				),
			),
			// Points {{{3
			icols: [2][2][]maths.Pt{
				[2][]maths.Pt{
					[]maths.Pt{{0, 0}, {0, 4}},
					[]maths.Pt{{1, 0}, {1, 2}, {1, 4}},
				},
				[2][]maths.Pt{
					[]maths.Pt{{1, 0}, {1, 2}, {1, 4}},
					[]maths.Pt{{2, 0}, {2, 1}, {2, 3}, {2, 4}},
				},
			},
			// MaxY {{{3
			pt2my: [2]map[maths.Pt]int64{
				map[maths.Pt]int64{
					{1, 2}: 300,
				},
				map[maths.Pt]int64{
					{1, 2}: 300,
				},
			},
			testYs: true,
			// Col {{{3
			Col: RingCol{
				X1: 0,
				X2: 2,
				// Rings {{{4
				Rings: []Ring{
					{
						Label:  maths.Inside,
						Points: []maths.Pt{{0, 0}, {2, 0}, {2, 1}, {1, 2}, {2, 3}, {2, 4}, {0, 4}},
					},
					{
						Label:  maths.Outside,
						Points: []maths.Pt{{1, 2}, {2, 1}, {2, 3}},
					},
				},
				// Y1s {{{4
				Y1s: []YEdge{
					{
						Y:     0,
						Descs: []RingDesc{{0, 0, maths.Inside}},
					},
					{
						Y:     4,
						Descs: []RingDesc{{0, 6, maths.Inside}},
					},
				},
				// Y2s {{{4
				Y2s: []YEdge{
					{
						Y:     0,
						Descs: []RingDesc{{0, 1, maths.Inside}},
					},
					{
						Y: 1,
						Descs: []RingDesc{
							{0, 2, maths.Inside},
							{1, 1, maths.Outside},
						},
					},
					{
						Y: 3,
						Descs: []RingDesc{
							{0, 4, maths.Inside},
							{1, 2, maths.Outside},
						},
					},
					{
						Y:     4,
						Descs: []RingDesc{{0, 5, maths.Inside}},
					},
				},
			},
		},

		// }}}2
	)
	// cases }}}1
	tests.Run(func(idx int, test testcase) {
		log.Printf("Running %v (%v)", idx, test.desc)
		col1 := BuildRingCol(context.Background(), test.hm, test.icols[0][0], test.icols[0][1], test.pt2my[0])
		col2 := BuildRingCol(context.Background(), test.hm, test.icols[1][0], test.icols[1][1], test.pt2my[1])
		log.Printf("Col1: %v", col1.String())
		log.Printf("Col2: %v", col2.String())

		mcol := merge2AdjectRC(col1, col2)
		if ok, reason := ringDiff(&mcol, &test.Col, test.testYs); ok {
			t.Errorf("For %v (%v) %v", idx, test.desc, reason)
		}
	})
}

func _TestMerge2AR1(t *testing.T) {
	type testcase struct {
		ColsFile string
		Col      RingCol
	}
	// cases {{{1
	tests := tbltest.Cases(
		testcase{
			//ColsFile: "testdata/9297b97c-bbbf-4f8d-a285-3adb99de4163",
			//ColsFile: "testdata/62254d3e-10a4-4b71-912b-48d3c7168a9c",
			//ColsFile: "testdata/c23a7a28-743f-4fef-b335-0e0d1fd94126",
			//ColsFile: "testdata/9ac4d011-c9b6-4d68-b128-584a5992f1f7",

			ColsFile: "testdata/66223d1e-18a4-4d84-bde0-950a4ec9a8c5",
		},
	)
	tests.Run(func(idx int, test testcase) {
		log.Printf("Running %v (%v)", idx, test.ColsFile)
		cols := LoadCols(test.ColsFile)
		writeOutSVG(test.ColsFile, cols, nil)
		if len(cols) < 2 {
			panic("Expected the cols to be two or more.")
		}

		log.Println("Col1", cols[0].String())
		log.Println("Col2", cols[1].String())

		mcol := merge2AdjectRC(cols[0], cols[1])
		if ok, reason := ringDiff(&mcol, &test.Col, true); ok {
			t.Errorf("For %v (%v) %v", idx, test.ColsFile, reason)
		}
	})
}
