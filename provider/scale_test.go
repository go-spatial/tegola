package provider_test

import (
	"fmt"
	"math"
	"testing"

	"github.com/alexeydott/geom"
	"github.com/alexeydott/geom/slippy"
	"github.com/alexeydott/tegola/basic"
	"github.com/alexeydott/tegola/provider"
	"github.com/alexeydott/tegola/provider/test/fixture"
)

type sizedScaleTile struct {
	provider.Tile
	width, height uint
}

func (t sizedScaleTile) PixelSize() (uint, uint) { return t.width, t.height }

func assertScaleClose(t *testing.T, name string, got, want float64) {
	t.Helper()
	if math.IsNaN(got) || math.Abs(got-want) > math.Max(1, math.Abs(want))*1e-10 {
		t.Errorf("%s = %.12g, want %.12g", name, got, want)
	}
}

func TestTileScaleUnitsAndSizes(t *testing.T) {
	basic.RegisterBuiltinProj4SRIDs()
	for _, tc := range []struct {
		name, defn string
		srid       uint64
		factor     float64
	}{
		{name: "WebMercator", srid: 3857, factor: 1},
		{name: "UTM", srid: 32631, factor: 1},
		{name: "WorldMercator", srid: 3395, factor: 1},
		{name: "feet", defn: "+proj=utm +zone=31 +datum=WGS84 +units=ft", factor: 0.3048},
		{name: "survey feet", defn: "+proj=utm +zone=31 +datum=WGS84 +units=us-ft", factor: 1200.0 / 3937},
		{name: "kilometers", defn: "+proj=utm +zone=31 +datum=WGS84 +units=km", factor: 1000},
		{name: "custom units", defn: "+proj=utm +zone=31 +datum=WGS84 +to_meter=2.5", factor: 2.5},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srid := tc.srid
			if tc.defn != "" {
				var err error
				srid, err = basic.RegisterProj4Defn(tc.defn)
				if err != nil {
					t.Fatal(err)
				}
			}
			for _, size := range [][2]uint{{256, 256}, {512, 512}, {1024, 512}} {
				t.Run(fmt.Sprint(size), func(t *testing.T) {
					extent := &geom.Extent{1000 / tc.factor, 2000 / tc.factor, 2024 / tc.factor, 4048 / tc.factor}
					tile := sizedScaleTile{Tile: fixture.Tile{Bounds: extent, SRID: srid}, width: size[0], height: size[1]}
					w, h, s, err := provider.TileScale(tile, srid)
					if err != nil {
						t.Fatal(err)
					}
					assertScaleClose(t, "width", w, 1024/tc.factor/float64(size[0]))
					assertScaleClose(t, "height", h, 2048/tc.factor/float64(size[1]))
					assertScaleClose(t, "denominator", s, 1024/float64(size[0])/0.00028)
				})
			}
		})
	}
}

func TestTileScaleGeographic(t *testing.T) {
	// Independent 50-digit decimal reference calculations using the WGS84
	// inverse flattening 298.257223563 and local parallel curvature.
	for _, reference := range []struct{ latitude, meters float64 }{
		{0, 111319.49079327357}, {45, 78846.83509397811},
		{60, 55800.00157243613}, {-60, 55800.00157243613}, {80, 19393.485528132147},
	} {
		latitude := reference.latitude
		for _, size := range []uint{256, 512} {
			t.Run(fmt.Sprintf("%g/%d", latitude, size), func(t *testing.T) {
				extent := &geom.Extent{10, latitude - 1, 12, latitude + 1}
				tile := sizedScaleTile{Tile: fixture.Tile{Bounds: extent, SRID: 4326}, width: size, height: size}
				w, h, s, err := provider.TileScale(tile, 4326)
				if err != nil {
					t.Fatal(err)
				}
				assertScaleClose(t, "width degrees", w, 2/float64(size))
				assertScaleClose(t, "height degrees", h, 2/float64(size))
				assertScaleClose(t, "denominator", s, 2/float64(size)*reference.meters/0.00028)
			})
		}
	}
}

func TestTileScaleReprojectsSlippyTiles(t *testing.T) {
	basic.RegisterBuiltinProj4SRIDs()
	feet, err := basic.RegisterProj4Defn("+proj=eqc +lat_ts=0 +datum=WGS84 +units=ft")
	if err != nil {
		t.Fatal(err)
	}
	for _, z := range []slippy.Zoom{0, 2, 8, 16} {
		for _, size := range []uint{256, 512} {
			t.Run(fmt.Sprintf("%d/%d", z, size), func(t *testing.T) {
				y := uint(0)
				if z > 0 {
					y = 1 << (z - 1)
				}
				tile := sizedScaleTile{Tile: provider.NewTile(z, 0, y, 64, 3857), width: size, height: size}
				extent, _ := tile.Extent()
				centerLat := math.Atan(math.Sinh((extent.MinY()+extent.MaxY())/2/6378137)) * 180 / math.Pi
				minLat := math.Atan(math.Sinh(extent.MinY()/6378137)) * 180 / math.Pi
				maxLat := math.Atan(math.Sinh(extent.MaxY()/6378137)) * 180 / math.Pi
				degreesPerPixel := 360 / math.Exp2(float64(z)) / float64(size)
				for _, srid := range []uint64{4326, 4087, feet} {
					w, h, s, err := provider.TileScale(tile, srid)
					if err != nil {
						t.Fatal(err)
					}
					factor := 1.0
					meters := 111319.49079327358
					if srid == 4326 {
						phi := centerLat * math.Pi / 180
						meters *= math.Cos(phi) / math.Sqrt(1-0.0066943799901413165*math.Sin(phi)*math.Sin(phi))
					} else {
						factor = 111319.49079327358
						if srid == feet {
							factor /= 0.3048
						}
					}
					assertScaleClose(t, "width", w, degreesPerPixel*factor)
					assertScaleClose(t, "height", h, (maxLat-minLat)/float64(size)*factor)
					assertScaleClose(t, "denominator", s, degreesPerPixel*meters/0.00028)
				}
			})
		}
	}
}

func TestTileScaleErrors(t *testing.T) {
	for _, tc := range []struct {
		name              string
		extent            *geom.Extent
		tileCRS, layerCRS uint64
		size              uint
	}{
		{"unknown layer", &geom.Extent{0, 0, 1, 1}, 3857, 999999, 256},
		{"unknown matching CRS", &geom.Extent{0, 0, 1, 1}, 999999, 999999, 256},
		{"unknown tile", &geom.Extent{0, 0, 1, 1}, 999999, 3857, 256},
		{"unsupported tile conversion", &geom.Extent{0, 0, 1, 1}, 4326, 3857, 256},
		{"nil extent", nil, 3857, 3857, 256},
		{"nonfinite extent", &geom.Extent{0, 0, math.Inf(1), 1}, 3857, 3857, 256},
		{"zero width", &geom.Extent{0, 0, 0, 1}, 3857, 3857, 256},
		{"zero height", &geom.Extent{0, 0, 1, 0}, 3857, 3857, 256},
		{"inverted extent", &geom.Extent{1, 0, 0, 1}, 3857, 3857, 256},
		{"zero pixels", &geom.Extent{0, 0, 1, 1}, 3857, 3857, 0},
		{"invalid latitude", &geom.Extent{0, 89, 1, 91}, 4326, 4326, 256},
		{"overflow", &geom.Extent{-math.MaxFloat64, 0, math.MaxFloat64, 1}, 3857, 3857, 256},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tile := sizedScaleTile{Tile: fixture.Tile{Bounds: tc.extent, SRID: tc.tileCRS}, width: tc.size, height: tc.size}
			if _, _, _, err := provider.TileScale(tile, tc.layerCRS); err == nil {
				t.Fatal("expected scale error")
			}
		})
	}
}

func TestTileScaleSourceEllipsoid(t *testing.T) {
	for _, tc := range []struct {
		name, defn string
		meters     float64
	}{
		{"Bessel", "+proj=longlat +ellps=bessel +towgs84=41,-107.6,-93", 55793.108216124725},
		{"custom", "+proj=longlat +a=6378200 +rf=298.3 +towgs84=23.92,-141.27,-80.9,0,-0.35,-0.82,-0.12", 55800.53258131389},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srid, err := basic.RegisterProj4Defn(tc.defn)
			if err != nil {
				t.Fatal(err)
			}
			tile := sizedScaleTile{Tile: fixture.Tile{Bounds: &geom.Extent{10, 59, 12, 61}, SRID: srid}, width: 512, height: 256}
			w, h, s, err := provider.TileScale(tile, srid)
			if err != nil {
				t.Fatal(err)
			}
			assertScaleClose(t, "width degrees", w, 2.0/512)
			assertScaleClose(t, "height degrees", h, 2.0/256)
			assertScaleClose(t, "source ellipsoid denominator", s, 2.0/512*tc.meters/0.00028)
		})
	}
}

func TestTileScaleUsesTransformedGeographicLatitude(t *testing.T) {
	srid, err := basic.RegisterProj4Defn("+proj=longlat +ellps=bessel +towgs84=41,-107.6,-93")
	if err != nil {
		t.Fatal(err)
	}
	// PROJ 9.5.1 maps this WGS84 (37.6,55.7) center to source latitude
	// 55.69966788487392. The Bessel parallel there is 62868.02857200517 m/deg.
	const x, y = 4185612.8538270863, 7498924.477653493
	tile := sizedScaleTile{Tile: fixture.Tile{
		Bounds: &geom.Extent{x - 10000, y - 10000, x + 10000, y + 10000}, SRID: 3857,
	}, width: 512, height: 256}
	w, _, scale, err := provider.TileScale(tile, srid)
	if err != nil {
		t.Fatal(err)
	}
	assertScaleClose(t, "source center meters per degree", scale*0.00028/w, 62868.02857200517)
}
