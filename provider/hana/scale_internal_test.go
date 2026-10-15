package hana

import (
	"fmt"
	"math"
	"strconv"
	"strings"
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
					w, h, s, err := tileScale(tile, srid)
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
	// Independent decimal reference values for the WGS84 ellipsoid parallel.
	for _, reference := range []struct{ latitude, meters float64 }{
		{0, 111319.49079327357}, {45, 78846.83509397811},
		{60, 55800.00157243613}, {-60, 55800.00157243613}, {80, 19393.485528132147},
	} {
		latitude := reference.latitude
		for _, size := range []uint{256, 512} {
			t.Run(fmt.Sprintf("%g/%d", latitude, size), func(t *testing.T) {
				extent := &geom.Extent{10, latitude - 1, 12, latitude + 1}
				tile := sizedScaleTile{Tile: fixture.Tile{Bounds: extent, SRID: 4326}, width: size, height: size}
				w, h, s, err := tileScale(tile, 4326)
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
				for _, srid := range []uint64{4326, 1000004326, 4087, feet} {
					w, h, s, err := tileScale(tile, srid)
					if err != nil {
						t.Fatal(err)
					}
					factor := 1.0
					meters := 111319.49079327358
					if srid == 4326 || srid == 1000004326 {
						phi := centerLat * math.Pi / 180
						// Both the geographic and HANA planar alias resolve to WGS84.
						const flattening = 1 / 298.257223563
						const e2 = flattening * (2 - flattening)
						meters *= math.Cos(phi) / math.Sqrt(1-e2*math.Sin(phi)*math.Sin(phi))
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
			if _, _, _, err := tileScale(tile, tc.layerCRS); err == nil {
				t.Fatal("expected scale error")
			}
		})
	}
}

func TestReplaceScaleTokensWebMercatorCompatibility(t *testing.T) {
	layer := &Layer{name: "metric", srid: 3857}
	for _, z := range []slippy.Zoom{0, 2, 11, 22} {
		t.Run(fmt.Sprint(z), func(t *testing.T) {
			tile := provider.NewTile(z, 0, 0, 64, 3857)
			extent, _ := tile.Extent()
			oldWidth := (extent.MaxX() - extent.MinX()) / 256
			oldHeight := (extent.MaxY() - extent.MinY()) / 256
			want := fmt.Sprintf("SELECT %.8f, %.8f, %.8f", oldWidth, oldHeight, oldWidth/0.00028)
			for _, buffered := range []bool{false, true} {
				got, err := replaceTokens(4, "SELECT !pixel_width!, !PIXEL_HEIGHT!, !scale_denominator!", layer, nil, layer.SRID(), tile, buffered)
				if err != nil {
					t.Fatal(err)
				}
				if got != want {
					t.Errorf("SQL = %q, want byte-exact legacy SQL %q", got, want)
				}
			}
		})
	}
}

func TestReplaceScaleTokensCRSAndContexts(t *testing.T) {
	feet, err := basic.RegisterProj4Defn("+proj=eqc +lat_ts=0 +datum=WGS84 +units=ft")
	if err != nil {
		t.Fatal(err)
	}
	for _, srid := range []uint64{4326, 1000004326, feet, 4087} {
		t.Run(fmt.Sprint(srid), func(t *testing.T) {
			layer := &Layer{name: "source", srid: srid}
			tile := sizedScaleTile{Tile: provider.NewTile(8, 128, 80, 64, 3857), width: 512, height: 1024}
			w, h, s, err := tileScale(tile, srid)
			if err != nil {
				t.Fatal(err)
			}
			protected := ", '!pixel_width!', \"!PIXEL_HEIGHT!\", '!scale_denominator!', '!pixel_width!', '!SCALE_DENOMINATOR!', '!pixel_height!' -- !PIXEL_WIDTH!\n/* !scale_denominator! */"
			sql := "SELECT !pixel_width!, !PIXEL_HEIGHT!, !scale_denominator!" + protected
			want := fmt.Sprintf("SELECT %.8f, %.8f, %.8f", w, h, s) + protected
			for _, buffered := range []bool{false, true} {
				got, err := replaceTokens(4, sql, layer, nil, layer.SRID(), tile, buffered)
				if err != nil {
					t.Fatal(err)
				}
				if got != want {
					t.Errorf("SQL = %q, want %q", got, want)
				}
			}
		})
	}
}

func TestReplaceScaleTokensUnknownCRS(t *testing.T) {
	layer := &Layer{name: "unknown", srid: 999999}
	tile := provider.NewTile(8, 128, 80, 64, 3857)
	for _, token := range []string{pixelWidthToken, pixelHeightToken, scaleDenominatorToken} {
		got, err := replaceTokens(4, "SELECT "+token, layer, nil, layer.SRID(), tile, false)
		if err == nil || got != "" || !strings.Contains(err.Error(), "layer (unknown) scale tokens") {
			t.Errorf("expected contextual error and empty SQL, got %q, %v", got, err)
		}
	}
	for _, sql := range []string{"SELECT 1", "SELECT '!PIXEL_WIDTH!', \"!pixel_height!\" -- !scale_denominator!\n/* !PIXEL_WIDTH! */"} {
		got, err := replaceTokens(4, sql, layer, nil, layer.SRID(), tile, false)
		if err != nil || got != sql {
			t.Errorf("protected/unused tokens must not need CRS metadata: got %q, %v", got, err)
		}
	}
}

func TestReplaceScaleTokenGeographicExamples(t *testing.T) {
	// Log operator-visible examples without a database; the old denominator
	// ignores latitude, while the new one follows the tile center parallel.
	layer := &Layer{name: "geographic", srid: 4326}
	for _, tc := range []struct {
		z    slippy.Zoom
		x, y uint
	}{{2, 1, 1}, {11, 1070, 676}} {
		tile := provider.NewTile(tc.z, tc.x, tc.y, 64, 3857)
		extent, _ := tile.Extent()
		oldWidth := (extent.MaxX() - extent.MinX()) / 256
		sql, err := replaceTokens(4, "!PIXEL_WIDTH!,!SCALE_DENOMINATOR!", layer, nil, layer.SRID(), tile, false)
		if err != nil {
			t.Fatal(err)
		}
		values := strings.Split(sql, ",")
		newScale, err := strconv.ParseFloat(values[1], 64)
		if err != nil || newScale >= oldWidth/0.00028 {
			t.Fatalf("expected latitude-adjusted scale below legacy scale: %v, %v", newScale, err)
		}
		t.Logf("z=%d x=%d y=%d old width=%.8f m scale=%.8f; new width=%s degrees scale=%s", tc.z, tc.x, tc.y, oldWidth, oldWidth/0.00028, values[0], values[1])
	}
}
