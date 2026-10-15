package server

import (
	"context"
	"encoding/json"
	"math"
	"net/url"
	"slices"
	"sync/atomic"
	"testing"

	"github.com/alexeydott/geom"
	"github.com/alexeydott/tegola/ogc/features"
	"github.com/alexeydott/tegola/provider"
)

type crsAcceptanceLayer struct {
	protocolLayer
	dimension provider.CoordinateDimension
}

func (l crsAcceptanceLayer) SpatialMetadata() (provider.SpatialMetadata, error) {
	m := provider.SpatialMetadata{Dimension: l.dimension}
	if l.dimension != provider.DimensionXY {
		m.VerticalCRS = provider.CRS84h
	}
	return m, nil
}
func (l crsAcceptanceLayer) FeatureCRSDefinition() (provider.FeatureCRSDefinition, error) {
	m, _ := l.SpatialMetadata()
	code := "4326"
	if l.dimension != provider.DimensionXY {
		code = "4979"
	}
	return provider.FeatureCRSDefinition{HorizontalSRID: 4326, Definition: "+proj=longlat +datum=WGS84 +no_defs", CanonicalAuthority: "EPSG", CanonicalCode: code, Spatial: m}, nil
}

type crsAcceptanceQuery struct {
	calls atomic.Int64
	shape geom.Geometry
}

func (p *crsAcceptanceQuery) QueryFeatures(ctx context.Context, _ string, q provider.FeatureQuery, fn func(*provider.Feature) error) (provider.FeatureQueryResult, error) {
	p.calls.Add(1)
	if err := ctx.Err(); err != nil {
		return provider.FeatureQueryResult{}, err
	}
	if err := fn(&provider.Feature{ID: 10, SRID: 4326, Geometry: p.shape, Tags: map[string]any{"name": "asymmetric"}}); err != nil {
		return provider.FeatureQueryResult{}, err
	}
	one := uint64(1)
	return provider.FeatureQueryResult{NumberMatched: &one, NumberReturned: 1}, nil
}

func crsPoint(t *testing.T, response protocolResponse, item bool) []float64 {
	t.Helper()
	var point struct{ Coordinates []float64 }
	var feature struct{ Geometry json.RawMessage }
	if item {
		if err := json.Unmarshal(response.body, &feature); err != nil {
			t.Fatal(err)
		}
	} else {
		var page struct {
			Features []struct{ Geometry json.RawMessage }
		}
		if err := json.Unmarshal(response.body, &page); err != nil || len(page.Features) != 1 {
			t.Fatal("expected one literal feature", err)
		}
		feature.Geometry = page.Features[0].Geometry
	}
	if err := json.Unmarshal(feature.Geometry, &point); err != nil {
		t.Fatal(err)
	}
	return point.Coordinates
}

func assertCRSPosition(t *testing.T, got, want []float64, tolerance float64) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("coordinate dimension=%d expected=%d", len(got), len(want))
	}
	for i := range want {
		if math.IsNaN(got[i]) || math.IsInf(got[i], 0) || math.Abs(got[i]-want[i]) > tolerance {
			t.Fatalf("ordinate%d=%g expected%g tolerance%g", i, got[i], want[i], tolerance)
		}
	}
}

func TestFeatureCRSProtocolLiteralOutputAndRejection(t *testing.T) {
	p := &crsAcceptanceQuery{shape: geom.Point{15, 30}}
	srv := protocolServer(t, p, crsAcceptanceLayer{dimension: provider.DimensionXY}, "/stage")
	for _, path := range []string{"/stage/features/collections", "/stage/features/collections/public"} {
		r := protocolRequest(t, srv, "GET", path)
		var metadata struct {
			CRS         []string `json:"crs"`
			Storage     string   `json:"storageCrs"`
			Collections []struct {
				CRS     []string `json:"crs"`
				Storage string   `json:"storageCrs"`
			}
		}
		if err := json.Unmarshal(r.body, &metadata); err != nil {
			t.Fatal(err)
		}
		if len(metadata.Collections) == 1 {
			metadata.CRS = metadata.Collections[0].CRS
			metadata.Storage = metadata.Collections[0].Storage
		}
		if r.status != 200 || len(metadata.CRS) == 0 || metadata.CRS[0] != features.CRS84 || metadata.Storage != features.CRS84 || !slices.Contains(metadata.CRS, metadata.Storage) || slices.Contains(metadata.CRS, features.CRS84h) || p.calls.Load() != 0 {
			t.Fatal("applicable default/storage discovery changed or invoked provider query")
		}
	}
	for _, tc := range []struct {
		uri       string
		want      []float64
		tolerance float64
	}{
		{features.CRS84, []float64{15, 30}, 1e-6},
		{"http://www.opengis.net/def/crs/EPSG/0/4326", []float64{30, 15}, 1e-6},
		{"http://www.opengis.net/def/crs/EPSG/0/3857", []float64{1669792.3618991037, 3503549.843504374}, .15},
	} {
		for _, item := range []bool{false, true} {
			path := "/stage/features/collections/public/items"
			if item {
				path += "/10"
			}
			r := protocolRequest(t, srv, "GET", path+"?crs="+url.QueryEscape(tc.uri))
			if r.status != 200 || r.header.Get("Content-Crs") != "<"+tc.uri+">" {
				t.Fatalf("literal CRS %s item=%v status/header %d %q body=%s", tc.uri, item, r.status, r.header.Get("Content-Crs"), r.body)
			}
			assertCRSPosition(t, crsPoint(t, r, item), tc.want, tc.tolerance)
		}
	}
	for _, item := range []bool{false, true} {
		path := "/stage/features/collections/public/items"
		if item {
			path += "/10"
		}
		r := protocolRequest(t, srv, "GET", path)
		if r.status != 200 || r.header.Get("Content-Crs") != "<"+features.CRS84+">" {
			t.Fatal("default CRS header missing")
		}
		assertCRSPosition(t, crsPoint(t, r, item), []float64{15, 30}, 1e-6)
	}
	before := p.calls.Load()
	for _, query := range []string{"crs=EPSG:4326", "crs=", "crs=unknown", "crs=" + url.QueryEscape(features.CRS84h), "crs=" + url.QueryEscape("http://www.opengis.net/def/crs/EPSG/0/32633"), "bbox=29,14,31,16&bbox-crs=unknown", "bbox-crs=" + url.QueryEscape(features.CRS84), "crs=unknown&crs=unknown"} {
		r := protocolRequest(t, srv, "GET", "/stage/features/collections/public/items?"+query)
		if r.status != 400 || p.calls.Load() != before {
			t.Fatal("invalid CRS reached provider I/O")
		}
	}
}

func TestFeatureCRSProtocolHeightAxes(t *testing.T) {
	p := &crsAcceptanceQuery{shape: geom.PointZ{15, 30, 23}}
	srv := protocolServer(t, p, crsAcceptanceLayer{dimension: provider.DimensionXYZ}, "/")
	for _, tc := range []struct {
		uri  string
		want []float64
	}{{features.CRS84h, []float64{15, 30, 23}}, {"http://www.opengis.net/def/crs/EPSG/0/4979", []float64{30, 15, 23}}} {
		for _, item := range []bool{false, true} {
			path := "/features/collections/public/items"
			if item {
				path += "/10"
			}
			r := protocolRequest(t, srv, "GET", path+"?crs="+url.QueryEscape(tc.uri))
			if r.status != 200 || r.header.Get("Content-Crs") != "<"+tc.uri+">" {
				t.Fatal("height CRS response missing")
			}
			assertCRSPosition(t, crsPoint(t, r, item), tc.want, 1e-6)
		}
	}
	before := p.calls.Load()
	if r := protocolRequest(t, srv, "GET", "/features/collections/public/items?crs="+url.QueryEscape(features.CRS84)); r.status != 400 || p.calls.Load() != before {
		t.Fatal("height-dropping CRS admitted")
	}
	if r := protocolRequest(t, srv, "GET", "/features/collections/public/items"); r.status != 200 || r.header.Get("Content-Crs") != "<"+features.CRS84h+">" {
		t.Fatal("height default header missing")
	}
}

func TestFeatureCRSProtocolMixedMissingHeight(t *testing.T) {
	p := &crsAcceptanceQuery{shape: geom.Collection{geom.Point{15, 30}, geom.PointZ{15, 30, 23}}}
	srv := protocolServer(t, p, crsAcceptanceLayer{dimension: provider.DimensionMixedXYXYZ}, "/")
	r := protocolRequest(t, srv, "GET", "/features/collections/public/items?crs="+url.QueryEscape("http://www.opengis.net/def/crs/EPSG/0/4979"))
	var page struct {
		Features []struct {
			Geometry struct {
				Type       string
				Geometries []struct {
					Type        string
					Coordinates []float64
				}
			}
		}
	}
	if err := json.Unmarshal(r.body, &page); err != nil {
		t.Fatal(err)
	}
	if r.status != 200 || r.header.Get("Content-Crs") != "<http://www.opengis.net/def/crs/EPSG/0/4979>" || len(page.Features) != 1 || page.Features[0].Geometry.Type != "GeometryCollection" || len(page.Features[0].Geometry.Geometries) != 2 {
		t.Fatal("mixed collection structure/header changed")
	}
	for i, want := range [][]float64{{30, 15}, {30, 15, 23}} {
		child := page.Features[0].Geometry.Geometries[i]
		if child.Type != "Point" {
			t.Fatal("mixed child family changed")
		}
		assertCRSPosition(t, child.Coordinates, want, 1e-6)
	}
	metadata := protocolRequest(t, srv, "GET", "/features/collections/public")
	var object map[string]json.RawMessage
	if err := json.Unmarshal(metadata.body, &object); err != nil {
		t.Fatal(err)
	}
	if _, claimed := object["storageCrs"]; claimed {
		t.Fatal("mixed source claimed uniform dimensional storage CRS")
	}
}
