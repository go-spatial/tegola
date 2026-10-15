package simplify_test

import (
	"reflect"
	"testing"

	"github.com/alexeydott/tegola"
	"github.com/alexeydott/tegola/basic"
	"github.com/alexeydott/tegola/maths/simplify"
)

func denseRectangle(x, y, width, height float64, reverse, closed bool) basic.Line {
	line := basic.Line{{x, y}, {x, y + height/2}, {x, y + height}, {x + width/2, y + height}, {x + width, y + height}, {x + width, y + height/2}, {x + width, y}, {x + width/2, y}}
	if reverse {
		for i, j := 0, len(line)-1; i < j; i, j = i+1, j-1 {
			line[i], line[j] = line[j], line[i]
		}
	}
	if closed {
		line = append(line, line[0])
	}
	return line
}

func signedArea(line basic.Line) float64 {
	var area float64
	for i, a := range line {
		b := line[(i+1)%len(line)]
		area += a[0]*b[1] - b[0]*a[1]
	}
	return area / 2
}

func TestSimplifyComplexPolygonReduction(t *testing.T) {
	for _, closed := range []bool{false, true} {
		shell := denseRectangle(0, 0, 100, 100, false, closed)
		hole := denseRectangle(20, 20, 60, 60, true, closed)
		island := denseRectangle(40, 40, 20, 20, false, closed)
		separate := denseRectangle(200, 0, 100, 100, false, closed)
		for _, tc := range []struct {
			name string
			in   basic.MultiPolygon
		}{
			{"holes", basic.MultiPolygon{{shell, hole}}},
			{"disjoint components", basic.MultiPolygon{{shell}, {separate}}},
			{"island within hole", basic.MultiPolygon{{shell, hole}, {island}, {separate}}},
		} {
			t.Run(tc.name+map[bool]string{true: " closed", false: " open"}[closed], func(t *testing.T) {
				before := make(basic.MultiPolygon, len(tc.in))
				for i, p := range tc.in {
					before[i] = basic.ClonePolygon(p)
				}
				var got basic.MultiPolygon
				if len(tc.in) == 1 {
					got = basic.MultiPolygon{simplify.SimplifyGeometry(tc.in[0], 1).(basic.Polygon)}
				} else {
					got = simplify.SimplifyGeometry(tc.in, 1).(basic.MultiPolygon)
				}
				if !reflect.DeepEqual(before, tc.in) {
					t.Fatal("input changed")
				}
				if len(got) != len(before) {
					t.Fatal("component lost")
				}
				for i, p := range got {
					if len(p) != len(before[i]) {
						t.Fatal("ring lost")
					}
					for j, line := range p {
						wantCount := 4
						if closed {
							wantCount++
						}
						if len(line) < wantCount || len(line) > wantCount+1 || len(line) >= len(before[i][j]) {
							t.Errorf("ring %d/%d has %d points, want %d", i, j, len(line), wantCount)
						}
						if (line[0] == line[len(line)-1]) != closed {
							t.Error("closure representation changed")
						}
						if signedArea(line)*signedArea(before[i][j]) <= 0 {
							t.Error("winding changed")
						}
						if signedArea(line) != signedArea(before[i][j]) {
							t.Error("collinear reduction changed area")
						}
					}
				}
				got[0][0][0][0] += 999
				if !reflect.DeepEqual(before, tc.in) {
					t.Fatal("output aliases input")
				}
			})
		}
	}
}

func TestSimplifyPreservesComponentRelationships(t *testing.T) {
	bulge := basic.Line{{0, 0}, {0, 100}, {100, 100}, {110, 50}, {100, 0}}
	stranded := basic.Line{{102, 48}, {107, 50}, {102, 52}}
	notch := basic.Line{{0, 0}, {0, 100}, {100, 100}, {90, 50}, {100, 0}}
	outside := basic.Line{{94, 48}, {98, 50}, {94, 52}}
	touching := basic.Line{{100, 45}, {105, 50}, {100, 55}}
	outer := denseRectangle(-20, -20, 160, 160, false, false)
	for _, tc := range []struct {
		name string
		in   tegola.Geometry
	}{
		{"hole stranded outside shell", basic.Polygon{bulge, stranded}},
		{"components become overlapping", basic.MultiPolygon{{notch}, {outside}}},
		{"components acquire contact", basic.MultiPolygon{{notch}, {touching}}},
		{"holes become overlapping", basic.Polygon{outer, notch, outside}},
		{"island stranded by shrinking hole", basic.MultiPolygon{{outer, bulge}, {stranded}}},
		{"nested holes", basic.Polygon{outer, denseRectangle(10, 10, 80, 80, true, false), denseRectangle(30, 30, 10, 10, true, false)}},
		{"self intersecting shell", basic.Polygon{{{0, 0}, {100, 100}, {0, 100}, {100, 0}, {50, -10}}}},
		{"existing boundary contact", basic.MultiPolygon{{denseRectangle(0, 0, 100, 100, false, false)}, {touching}}},
		{"existing component containment", basic.MultiPolygon{{outer}, {notch}}},
		{"hole outside shell", basic.Polygon{notch, stranded}},
		{"degenerate hole", basic.Polygon{outer, {{20, 20}, {30, 20}, {40, 20}}}},
		{"short hole", basic.Polygon{outer, {{20, 20}, {30, 20}}}},
		{"empty component", basic.MultiPolygon{{outer}, nil}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := simplify.SimplifyGeometry(tc.in, 15)
			if !reflect.DeepEqual(got, tc.in) {
				t.Fatalf("unsafe candidate must retain original: got %#v, want %#v", got, tc.in)
			}
		})
	}
}

// These candidates remove a non-collinear bulge, rather than merely redundant
// collinear vertices. A nearby independent component does not require fallback.
func TestSimplifyComplexPolygonRemovesBends(t *testing.T) {
	shell := basic.Line{{0, 0}, {0, 100}, {100, 100}, {110, 50}, {100, 0}}
	hole := basic.Line{{40, 40}, {60, 40}, {50, 60}}
	neighbor := basic.Line{{110, 60}, {115, 65}, {112, 70}}
	polygon := simplify.SimplifyGeometry(basic.Polygon{shell, hole}, 15).(basic.Polygon)
	multi := simplify.SimplifyGeometry(basic.MultiPolygon{{shell, hole}, {neighbor}}, 15).(basic.MultiPolygon)
	want := basic.Polygon{{{0, 0}, {0, 100}, {100, 100}, {100, 0}}, hole}
	if !reflect.DeepEqual(polygon, want) {
		t.Fatalf("safe shell bend was not reduced: %#v", polygon)
	}
	if !reflect.DeepEqual(multi, basic.MultiPolygon{want, {neighbor}}) {
		t.Fatalf("safe component bend was not reduced: %#v", multi)
	}
}
