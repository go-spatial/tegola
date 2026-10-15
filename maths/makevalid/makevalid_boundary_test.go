package makevalid

import (
	"context"
	"testing"

	"github.com/alexeydott/geom"
	"github.com/alexeydott/tegola/maths"
	"github.com/alexeydott/tegola/maths/hitmap"
)

func TestMakeValidPreservesBoundaryCoincidentWithClipbox(t *testing.T) {
	clipbox := geom.NewExtent([2]float64{0, 0}, [2]float64{10, 10})
	lines := [][]maths.Line{{
		maths.NewLine(0, 0, 10, 0),
		maths.NewLine(10, 0, 10, 10),
		maths.NewLine(10, 10, 0, 10),
		maths.NewLine(0, 10, 0, 0),
	}}

	hm := hitmap.NewFromLines(lines)
	got, err := MakeValid(context.Background(), &hm, clipbox, lines...)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) == 0 {
		t.Fatal("polygon coincident with clipbox was discarded")
	}

	hasBottomBoundary := false
	for _, polygon := range got {
		for _, ring := range polygon {
			for i := range ring {
				next := ring[(i+1)%len(ring)]
				if ring[i].Y == 0 && next.Y == 0 {
					hasBottomBoundary = true
				}
			}
		}
	}
	if !hasBottomBoundary {
		t.Fatalf("polygon lost its boundary on the clipbox edge: %#v", got)
	}
}
