package windingorder

import "testing"

func TestAttributeMethods(t *testing.T) {

	fn := func(val WindingOrder, isClockwise bool) {
		if val.IsClockwise() != isClockwise {
			t.Errorf("is clockwise, expected %v got %v", isClockwise, val.IsClockwise())
		}
		if val.IsCounterClockwise() == isClockwise {
			t.Errorf("is counter clockwise, expected %v got %v", !isClockwise, val.IsClockwise())
		}
		var cw, ncw = Clockwise, CounterClockwise
		if !isClockwise {
			cw = CounterClockwise
			ncw = Clockwise
		}
		if val.Not() != ncw {
			t.Errorf("not, expected %v got %v", ncw, val.Not())
		}
		if val.Not().Not() != cw {
			t.Errorf("not not, expected %v got %v", cw, val.Not().Not())
		}
		str := "clockwise"
		if !isClockwise {
			str = "counter clockwise"
		}
		if val.String() != str {
			t.Errorf("string, expected %v got %v", val.String(), str)
		}
	}
	fn(Clockwise, true)
	fn(CounterClockwise, false)
}

func TestOfPoints(t *testing.T) {
	type tcase struct {
		pts   [][2]float64
		order WindingOrder
	}
	fn := func(t *testing.T, tc tcase) {
		got := OfPoints(tc.pts...)
		if got != tc.order {
			t.Errorf("OfPoints, expected %v got %v", tc.order, got)
		}
	}
	tests := map[string]tcase{
		"simple points": {
			pts: [][2]float64{
				{0, 0}, {10, 0}, {10, 10}, {0, 10},
			},
			order: Clockwise,
		},
		"counter simple points": {
			pts: [][2]float64{
				{0, 10}, {10, 10}, {10, 0}, {0, 0},
			},
			order: CounterClockwise,
		},
	}
	for name, tc := range tests {
		tc := tc
		t.Run(name, func(t *testing.T) { fn(t, tc) })
	}
}
