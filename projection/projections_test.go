package projection

import (
	"math"
	"math/rand"
	"testing"

	"github.com/go-spatial/geom/slippy"

	"github.com/go-spatial/geom"
)

func pointIsClose(pt geom.Point, cmp geom.Point, tolerance float64) bool {
	return math.Abs(pt.X()-cmp.X()) < tolerance && math.Abs(pt.Y()-cmp.Y()) < tolerance
}

func TestProjection(t *testing.T) {
	type tcase struct {
		input        geom.Point
		ssrid        uint64
		dsrid        uint64
		expectedGeom geom.Point
		tolerance    float64
		expectedErr  error
	}

	fn := func(tc tcase) func(t *testing.T) {
		return func(t *testing.T) {
			output, err := ConvertGeom(tc.dsrid, tc.ssrid, tc.input)
			if pt, ok := output.(geom.Point); ok {
				var tol = tc.tolerance

				if tol == 0 {
					tol = 0.0001
				}
				if !pointIsClose(tc.expectedGeom, pt, tol) {
					t.Errorf("testcase (%v) failed. output (%v) not close enough to expected (%v)", t.Name(), output, tc.expectedGeom)
				}
			} else {
				panic("only accepting points for projection tests")
			}
			if tc.expectedErr != nil {
				if err == nil {
					t.Errorf("testcase (%v) failed. expected error %v", t.Name(), tc.expectedErr)
				}

				if tc.expectedErr.Error() != err.Error() {
					t.Errorf("testcase (%v) failed. error (%v) does not match expected error (%v)", t.Name(), err, tc.expectedErr)
				}
			} else {
				if err != nil {
					t.Errorf("testcase (%v) failed. received unexpected error (%v)", t.Name(), err)
				}
			}
		}
	}

	tests := map[string]tcase{
		"identity point (4326)": {
			input:        geom.Point{0, 0},
			ssrid:        4326,
			dsrid:        4326,
			expectedGeom: geom.Point{0, 0},
		},
		"identity point (3857)": {
			input:        geom.Point{0, 0},
			ssrid:        3857,
			dsrid:        3857,
			expectedGeom: geom.Point{0, 0},
		},
		"3857 SW extent": {
			input:        geom.Point{-20037508.3427, -20037471.2051},
			ssrid:        3857,
			dsrid:        4326,
			expectedGeom: geom.Point{-180, -85.0511},
		},
		"3857 NW extent": {
			input:        geom.Point{-20037508.3427, 20037471.2051},
			ssrid:        3857,
			dsrid:        4326,
			expectedGeom: geom.Point{-180, 85.0511},
		},
		"4326 NE extent": {
			input:        geom.Point{180, -85.0511},
			ssrid:        4326,
			dsrid:        3857,
			expectedGeom: geom.Point{20037508.3427, -20037471.2051},
			tolerance:    1,
		},
		"4326 NE full extent": {
			input:        geom.Point{180, -85.06},
			ssrid:        4326,
			dsrid:        3857,
			expectedGeom: geom.Point{20037508.34, -20048966.10},
			tolerance:    1,
		},
		"3857 NE full extent": {
			input:        geom.Point{20037508.3427, -20048966.10},
			ssrid:        3857,
			dsrid:        4326,
			expectedGeom: geom.Point{180, -85.06},
		},
	}

	for name, tc := range tests {
		t.Run(name, fn(tc))
	}
}

func BenchmarkTo3857Projections(b *testing.B) {
	src := rand.NewSource(42)
	xwid := slippy.SupportedProjections[4326].NativeExtents.XSpan()
	ywid := slippy.SupportedProjections[4326].NativeExtents.YSpan()

	datas := make([]geom.Point, 0, b.N)
	for n := 0; n < b.N; n++ {
		r := rand.New(src)
		x := r.Float64()*xwid - xwid/2
		y := r.Float64()*ywid - ywid/2
		datas = append(datas, geom.Point{x, y})
	}

	b.ReportAllocs()
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		ConvertGeom(3857, 4326, datas[n])
	}
}

func BenchmarkTo4326Projections(b *testing.B) {
	src := rand.NewSource(42)
	xwid := slippy.SupportedProjections[3857].NativeExtents.XSpan()
	ywid := slippy.SupportedProjections[3857].NativeExtents.YSpan()

	datas := make([]geom.Point, 0, b.N)
	for n := 0; n < b.N; n++ {
		r := rand.New(src)
		x := r.Float64()*xwid - xwid/2
		y := r.Float64()*ywid - ywid/2
		datas = append(datas, geom.Point{x, y})
	}

	b.ReportAllocs()
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		ConvertGeom(4326, 3857, datas[n])
	}
}
