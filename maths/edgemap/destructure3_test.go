package edgemap

import (
	"log"
	"testing"

	"github.com/gdey/tbltest"
	//"github.com/go-test/deep"
	"github.com/terranodo/tegola/maths"
	"github.com/terranodo/tegola/maths/hitmap"
)

func TestDestructure3(t *testing.T) {
	type testcase struct {
		adjustbb float64
		lines    [][]maths.Line
	}

	tests := tbltest.Cases(
		testcase{
			adjustbb: 1.0,
			lines: [][]maths.Line{
				{
					{maths.Pt{3, 1}, maths.Pt{7, 1}},
					{maths.Pt{7, 1}, maths.Pt{7, 6}},
					{maths.Pt{7, 6}, maths.Pt{3, 6}},
					{maths.Pt{3, 6}, maths.Pt{3, 1}},
				},
				{

					{maths.Pt{4, 4}, maths.Pt{4, 9}},
					{maths.Pt{4, 9}, maths.Pt{5, 9}},
					{maths.Pt{5, 9}, maths.Pt{5, 4}},
					{maths.Pt{5, 4}, maths.Pt{4, 4}},
				},
			},
		},
	)
	tests.Run(func(idx int, test testcase) {
		hm := hitmap.NewFromLines(test.lines)
		log.Printf("hm: %#v", hm)
		var lines []maths.Line
		for i := range test.lines {
			lines = append(lines, test.lines[i]...)
		}
		triangles, bbox := destructure3(hm, test.adjustbb, lines)
		for i := range triangles {
			log.Printf("Triangle[%v]: %v", i, triangles[i])
		}
		log.Printf("bbox: %#v", bbox)
		/*

			if diff := deep.Equal(got, test.polygons); diff != nil {
				t.Error("(", idx, ") Points do not match: Expected\n\t", test.polygons, "\ngot\n\t", got, "\n\tdiff:\t", diff)
			}
		*/
	})

}
