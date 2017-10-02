package edgemap

import (
	"testing"

	"github.com/terranodo/tegola/maths"
)

func BenchmarkTriangulatePolyB(b *testing.B) {

	var em *EM
	b.ReportAllocs()
	polygons := destructure(insureConnected(
		[]maths.Line{
			maths.Line{maths.Pt{X: 2784, Y: 960}, maths.Pt{X: 2838, Y: 994}},
			maths.Line{maths.Pt{X: 2838, Y: 994}, maths.Pt{X: 2853, Y: 975}},
			maths.Line{maths.Pt{X: 2853, Y: 975}, maths.Pt{X: 2856, Y: 975}},
			maths.Line{maths.Pt{X: 2856, Y: 975}, maths.Pt{X: 2857, Y: 977}},
			maths.Line{maths.Pt{X: 2857, Y: 977}, maths.Pt{X: 2857, Y: 980}},
			maths.Line{maths.Pt{X: 2857, Y: 980}, maths.Pt{X: 2735, Y: 936}},
			maths.Line{maths.Pt{X: 2735, Y: 936}, maths.Pt{X: 2734, Y: 934}},
			maths.Line{maths.Pt{X: 2734, Y: 934}, maths.Pt{X: 2739, Y: 930}},
			maths.Line{maths.Pt{X: 2739, Y: 930}, maths.Pt{X: 2782, Y: 959}},
			maths.Line{maths.Pt{X: 2782, Y: 959}, maths.Pt{X: 2785, Y: 953}},
			maths.Line{maths.Pt{X: 2785, Y: 953}, maths.Pt{X: 2781, Y: 949}},
			maths.Line{maths.Pt{X: 2781, Y: 949}, maths.Pt{X: 2786, Y: 938}},
			maths.Line{maths.Pt{X: 2786, Y: 938}, maths.Pt{X: 2759, Y: 913}},
			maths.Line{maths.Pt{X: 2759, Y: 913}, maths.Pt{X: 2763, Y: 908}},
			maths.Line{maths.Pt{X: 2763, Y: 908}, maths.Pt{X: 2766, Y: 908}},
			maths.Line{maths.Pt{X: 2766, Y: 908}, maths.Pt{X: 2770, Y: 911}},
			maths.Line{maths.Pt{X: 2770, Y: 911}, maths.Pt{X: 2770, Y: 914}},
			maths.Line{maths.Pt{X: 2770, Y: 914}, maths.Pt{X: 2778, Y: 924}},
			maths.Line{maths.Pt{X: 2778, Y: 924}, maths.Pt{X: 2792, Y: 933}},
			maths.Line{maths.Pt{X: 2792, Y: 933}, maths.Pt{X: 2800, Y: 919}},
			maths.Line{maths.Pt{X: 2800, Y: 919}, maths.Pt{X: 2809, Y: 907}},
			maths.Line{maths.Pt{X: 2809, Y: 907}, maths.Pt{X: 2808, Y: 904}},
			maths.Line{maths.Pt{X: 2808, Y: 904}, maths.Pt{X: 2805, Y: 902}},
			maths.Line{maths.Pt{X: 2805, Y: 902}, maths.Pt{X: 2808, Y: 895}},
			maths.Line{maths.Pt{X: 2808, Y: 895}, maths.Pt{X: 2811, Y: 894}},
			maths.Line{maths.Pt{X: 2811, Y: 894}, maths.Pt{X: 2818, Y: 910}},
			maths.Line{maths.Pt{X: 2818, Y: 910}, maths.Pt{X: 2784, Y: 960}},
		},
	))

	for n := 0; n < b.N; n++ {
		em = New(polygons)
		em.Triangulate()
	}

	edgemap = em
}
