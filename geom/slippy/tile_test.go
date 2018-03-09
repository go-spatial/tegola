package slippy_test

import (
	"strconv"
	"testing"

	"github.com/go-spatial/tegola"
	"github.com/go-spatial/tegola/geom"
	"github.com/go-spatial/tegola/geom/cmp"
	"github.com/go-spatial/tegola/geom/slippy"
)

func TestNewTile(t *testing.T) {
	type tcase struct {
		z, x, y  uint
		buffer   float64
		srid     uint64
		eBounds  [4]float64
		eExtent  *geom.Extent
		eBExtent *geom.Extent
	}
	fn := func(t *testing.T, tc tcase) {

		// Test the new functions.
		tile := slippy.NewTile(tc.z, tc.x, tc.y, tc.buffer, tc.srid)
		{
			gz, gx, gy := tile.ZXY()
			if gz != tc.z {
				t.Errorf("z, expected %v got %v", tc.z, gz)
			}
			if gx != tc.x {
				t.Errorf("x, expected %v got %v", tc.x, gx)
			}
			if gy != tc.y {
				t.Errorf("y, expected %v got %v", tc.y, gy)
			}
			if tile.Buffer != tc.buffer {
				t.Errorf("buffer, expected %v got %v", tc.buffer, tile.Buffer)
			}
			if tile.SRID != tc.srid {
				t.Errorf("srid, expected %v got %v", tc.srid, tile.SRID)
			}
		}
		{
			bounds := tile.Bounds()
			for i := 0; i < 4; i++ {
				if !cmp.Float64(bounds[i], tc.eBounds[i], 0.01) {
					t.Errorf("bounds[%v] , expected %v got %v", i, tc.eBounds[i], bounds[i])

				}
			}
		}
		{
			bufferedExtent, srid := tile.BufferedExtent()
			if srid != tc.srid {
				t.Errorf("buffered extent srid, expected %v got %v", tc.srid, srid)
			}

			if !cmp.BBox(tc.eBExtent, bufferedExtent) {
				t.Errorf("buffered extent, expected %v got %v", tc.eBExtent, bufferedExtent)
			}
		}
		{
			extent, srid := tile.Extent()
			if srid != tc.srid {
				t.Errorf("extent srid, expected %v got %v", tc.srid, srid)
			}

			if !cmp.BBox(tc.eExtent, extent) {
				t.Errorf("extent, expected %v got %v", tc.eExtent, extent)
			}
		}

	}
	tests := [...]tcase{
		{
			z:      2,
			x:      1,
			y:      1,
			buffer: 64,
			srid:   tegola.WebMercator,
			eExtent: geom.NewExtent(
				[2]float64{-10018754.17, 10018754.17},
				[2]float64{0, 0},
			),
			eBExtent: geom.NewExtent(
				[2]float64{-1.017529720390625e+07, 1.017529720390625e+07},
				[2]float64{156543.03390624933, -156543.03390624933},
			),
			eBounds: [4]float64{-90, 66.51, 0, 0},
		},
		{
			z:      16,
			x:      11436,
			y:      26461,
			buffer: 64,
			srid:   tegola.WebMercator,
			eExtent: geom.NewExtent(
				[2]float64{-13044437.497219238996, 3856706.6986199953},
				[2]float64{-13043826.000993041, 3856095.202393799},
			),
			eBExtent: geom.NewExtent(
				[2]float64{-1.3044447051847773e+07, 3.8567162532485295e+06},
				[2]float64{-1.3043816446364507e+07, 3.856085647765265e+06},
			),
			eBounds: [4]float64{-117.18, 32.70, -117.17, 32.70},
		},
	}
	for i, tc := range tests {
		tc := tc
		t.Run(strconv.FormatUint(uint64(i), 10), func(t *testing.T) { fn(t, tc) })
	}

}
