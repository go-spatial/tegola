package mysql

import (
	"fmt"
	"github.com/alexeydott/tegola/basic"
	"github.com/alexeydott/tegola/provider"
	"math"
	"strings"
	"testing"
)

type scaleSizedTile struct{ provider.Tile }

func (scaleSizedTile) PixelSize() (uint, uint) { return 512, 1024 }

func TestScaleTokensSourceCRS(t *testing.T) {
	feet, err := basic.RegisterProj4Defn("+proj=eqc +datum=WGS84 +units=ft")
	if err != nil {
		t.Fatal(err)
	}
	geographic, err := basic.RegisterProj4Defn("+proj=longlat +datum=WGS84")
	if err != nil {
		t.Fatal(err)
	}
	tile := scaleSizedTile{provider.NewTile(2, 1, 1, 64, 3857)}
	extent, _ := tile.Extent()
	minLat := math.Atan(math.Sinh(extent.MinY()/6378137)) * 180 / math.Pi
	maxLat := math.Atan(math.Sinh(extent.MaxY()/6378137)) * 180 / math.Pi
	centerLat := math.Atan(math.Sinh((extent.MinY() + extent.MaxY()) / 2 / 6378137))
	for _, srid := range []uint64{3857, 4326, geographic, feet} {
		t.Run(fmt.Sprint(srid), func(t *testing.T) {
			layer := &Layer{name: "scale", srid: srid}
			w, h := 90.0/512, (maxLat-minLat)/1024
			scale := w * 6378137 * math.Pi / 180 * math.Cos(centerLat) / math.Sqrt(1-0.0066943799901413165*math.Sin(centerLat)*math.Sin(centerLat)) / 0.00028
			if srid == 3857 {
				w, h = (extent.MaxX()-extent.MinX())/512, (extent.MaxY()-extent.MinY())/1024
				scale = w / 0.00028
			} else if srid == feet {
				w *= 6378137 * math.Pi / 180 / 0.3048
				h *= 6378137 * math.Pi / 180 / 0.3048
				scale = w * 0.3048 / 0.00028
			}
			query := "SELECT !pixel_width!, !PIXEL_HEIGHT!, !SCALE_DENOMINATOR!, '!pixel_width!'"
			got, err := replaceTokens(query, layer, tile, nil)
			if err != nil {
				t.Fatal(err)
			}
			var actualW, actualH, actualScale float64
			if _, err := fmt.Sscanf(got, "SELECT %f, %f, %f,", &actualW, &actualH, &actualScale); err != nil {
				t.Fatal(err)
			}
			for i, want := range []float64{w, h, scale} {
				actual := []float64{actualW, actualH, actualScale}[i]
				if math.Abs(actual-want) > math.Max(5e-9, math.Abs(want)*1e-12) {
					t.Fatalf("value %d = %.12g, want %.12g; SQL %s", i, actual, want, got)
				}
			}
			if !strings.HasSuffix(got, ", '!pixel_width!'") {
				t.Fatalf("protected token changed: %s", got)
			}
		})
	}
}

func TestDeferredScaleTokensRequireResolvedCRS(t *testing.T) {
	tile := provider.NewTile(3, 2, 2, 0, 3857)
	provisional := Layer{name: "deferred", srid: 3857, deferredInspection: true}
	for _, token := range []string{"!PIXEL_WIDTH!", "!pixel_height!", "!SCALE_DENOMINATOR!"} {
		sql, err := replaceTokens("SELECT "+token, &provisional, tile, nil)
		if err == nil || sql != "" || !strings.Contains(err.Error(), "configure srid or crs_defn") {
			t.Fatalf("unresolved %s: sql=%q err=%v", token, sql, err)
		}
	}
	for _, query := range []string{"SELECT !ZOOM!", "SELECT '!PIXEL_WIDTH!' -- !SCALE_DENOMINATOR!"} {
		if _, err := replaceTokens(query, &provisional, tile, nil); err != nil {
			t.Fatalf("unused scale requires no CRS: %v", err)
		}
	}
	explicit := provisional
	explicit.crsExplicit = true
	explicit.srid = 4326
	query := "SELECT !PIXEL_WIDTH!, !PIXEL_HEIGHT!, !SCALE_DENOMINATOR!"
	expected, err := replaceTokens(query, &explicit, tile, nil)
	if err != nil {
		t.Fatal(err)
	}
	resolved := provisional
	if action := resolved.resolveDeferredHeaderSRID(4326); action != deferredSRIDAdopt {
		t.Fatalf("header action=%v", action)
	}
	actual, err := replaceTokens(query, &resolved, tile, nil)
	if err != nil || actual != expected {
		t.Fatalf("resolved header sql=%q err=%v, want explicit CRS sql=%q", actual, err, expected)
	}
	// Adopting the same SRID still establishes units, even though no transform
	// change is necessary. A zero header must not unlock provisional units.
	same := provisional
	same.resolveDeferredHeaderSRID(3857)
	if _, err := replaceTokens(query, &same, tile, nil); err != nil {
		t.Fatal(err)
	}
	zero := provisional
	zero.resolveDeferredHeaderSRID(0)
	if _, err := replaceTokens(query, &zero, tile, nil); err == nil {
		t.Fatal("zero header must not resolve scale units")
	}
}
