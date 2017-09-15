package validate

import (
	"log"
	"testing"

	"github.com/terranodo/tegola/basic"
)

func TestMakePolygon(t *testing.T) {

}

type MakeMPValidTestCaseType struct {
	Desc     string
	Bench    bool
	Original basic.MultiPolygon
	Expected basic.MultiPolygon
	Err      error
}

var makevalidmpTestcases = []MakeMPValidTestCaseType{
	{
		Bench: true,
		Original: basic.MultiPolygon{ // basic.MultiPolygon len(01).
			basic.Polygon{ // basic.Polygon len(000001) polygon(00).
				basic.Line{ // basic.Line len(000006) direction(clockwise) line(00).
					{2516.000000, 1438.000000}, {2528.000000, 1483.000000}, {2661.000000, 1578.000000}, {2656.000000, 1586.000000}, {2653.000000, 1590.000000}, {2500.000000, 1435.000000}, // 000000 — 000005
				},
			},
		},
		Expected: basic.MultiPolygon{ // basic.MultiPolygon len(03).
			basic.Polygon{ // basic.Polygon len(000001) polygon(00).
				basic.Line{ // basic.Line len(000004) direction(clockwise) line(00).
					{2593.000000, 1529.000000}, {2661.000000, 1578.000000}, {2656.000000, 1586.000000}, {2653.000000, 1590.000000}, // 000000 — 000003
				},
			},
			basic.Polygon{ // basic.Polygon len(000001) polygon(01).
				basic.Line{ // basic.Line len(000003) direction(clockwise) line(00).
					{2499.000000, 1435.000000}, {2516.000000, 1438.000000}, {2520.000000, 1456.000000}, // 000000 — 000002
				},
			},
			basic.Polygon{ // basic.Polygon len(000001) polygon(02).
				basic.Line{ // basic.Line len(000003) direction(clockwise) line(00).
					{2520.000000, 1456.000000}, {2593.000000, 1529.000000}, {2528.000000, 1483.000000}, // 000000 — 000002
				},
			},
		},
	},
}

func TestMakeMultiPolygonValid(t *testing.T) {

}
func BenchmarkMakeMultiPolygonValid001(b *testing.B) {
	log.Println("Running benchmark test ", b.N)
	for n := 0; n < b.N; n++ {
		MakeMultiPolygonValid(makevalidmpTestcases[0].Original)
	}
}
