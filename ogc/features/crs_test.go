package features

import (
	"errors"
	"math"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/alexeydott/tegola/provider"
	"github.com/alexeydott/tegola/provider/crsconfig"
)

func TestCRSExactIdentifiers(t *testing.T) {
	for _, uri := range []string{CRS84, CRS84h, "http://www.opengis.net/def/crs/EPSG/0/4326", "http://www.opengis.net/def/crs/EPSG/0/4979", "http://www.opengis.net/def/crs/EPSG/0/3857", "http://www.opengis.net/def/crs/EPSG/0/32633", "http://www.opengis.net/def/crs/EPSG/0/32760"} {
		c, err := ResolveCRS(uri)
		if err != nil || c.URI() != uri {
			t.Fatalf("%s: %v", uri, err)
		}
	}
	for _, uri := range []string{"", "EPSG:4326", " https://www.opengis.net/def/crs/EPSG/0/4326", "http://www.opengis.net/def/crs/EPSG/0/04326", "http://www.opengis.net/def/crs/EPSG/0/340000001", "http://www.opengis.net/def/crs/EPSG/0/28407", strings.Repeat("x", 257), "urn:uuid:1234"} {
		_, err := ResolveCRS(uri)
		var invalid provider.InvalidFeatureQueryError
		if !errors.As(err, &invalid) {
			t.Fatalf("identifier accepted: %q (%v)", uri, err)
		}
	}
}

func TestCRSAxesAndOwnership(t *testing.T) {
	c, _ := ResolveCRS("http://www.opengis.net/def/crs/EPSG/0/4979")
	input := []float64{30, 15, 70}
	got, err := c.ToInternalPosition(input)
	if err != nil || !reflect.DeepEqual(got, []float64{15, 30, 70}) {
		t.Fatalf("%v %v", got, err)
	}
	wire, _ := c.FromInternalPosition(got)
	if !reflect.DeepEqual(wire, input) {
		t.Fatal(wire)
	}
	got[0] = 99
	if input[0] != 30 {
		t.Fatal("input mutated")
	}
	d := c.Definition()
	d.Axes[0].Name = "changed"
	if c.Definition().Axes[0].Name != "latitude" {
		t.Fatal("metadata aliased")
	}
	xy, err := c.FromInternalXY([]float64{15, 30})
	if err != nil || !reflect.DeepEqual(xy, []float64{30, 15}) {
		t.Fatal("mixed horizontal axes", xy, err)
	}
	for _, p := range [][]float64{{1, 2}, {1, 2, math.NaN()}, {1, 2, math.Inf(1)}} {
		if _, err := c.ToInternalPosition(p); err == nil {
			t.Fatal("invalid coordinate accepted")
		}
	}
}

func TestCRSApplicationIdentity(t *testing.T) {
	p, _ := crsconfig.NewHeightProjection(3857)
	a, err := NewApplicationCRS(p.Definition(), 3857, 3)
	if err != nil {
		t.Fatal(err)
	}
	b, err := NewApplicationCRS(p.Definition(), 340000001, 3)
	if err != nil {
		t.Fatal(err)
	}
	if a.URI() != b.URI() || len(a.Definition().SHA256) != 64 || !strings.HasPrefix(a.URI(), "urn:uuid:") {
		t.Fatal("identity depends on process SRID")
	}
	if a.URI()[23] != '8' {
		t.Fatalf("wrong UUID version: %s", a.URI())
	}
	xy, _ := NewApplicationCRS(p.Definition(), 3857, 2)
	if xy.URI() == a.URI() {
		t.Fatal("dimension omitted from identity")
	}
	if _, err := ResolveCRS(a.URI()); err == nil {
		t.Fatal("application leaked into global registry")
	}
	spelling, err := NewApplicationCRS(p.Definition()+" ", 3857, 3)
	if err != nil || spelling.URI() == a.URI() || spelling.Definition().Definition != p.Definition()+" " {
		t.Fatal("equivalent mathematical spelling lost raw identity", err)
	}
	if _, err := NewApplicationCRS(p.Definition()+" +axis=neu", 3857, 3); !errors.Is(err, provider.ErrUnsupported) {
		t.Fatal("unproven axis profile accepted", err)
	}
	if _, err := NewApplicationCRS(strings.Repeat("x", MaxCRSDefinitionBytes+1), 3857, 3); err == nil {
		t.Fatal("definition bound ignored")
	}
}

func TestCRSCollectionRoles(t *testing.T) {
	for _, dimension := range []provider.CoordinateDimension{provider.DimensionXY, provider.DimensionXYZ, provider.DimensionMixedXYXYZ} {
		spatial := provider.SpatialMetadata{Dimension: dimension}
		uri := CRS84
		if dimension != provider.DimensionXY {
			uri = CRS84h
			spatial.VerticalCRS = CRS84h
		}
		source, _ := ResolveCRS(uri)
		catalog, err := NewCollectionCRS(source, spatial)
		if err != nil {
			t.Fatal(err)
		}
		if catalog.DefaultURI() != uri {
			t.Fatal("wrong default")
		}
		if dimension == provider.DimensionMixedXYXYZ {
			if catalog.StorageURI() != "" {
				t.Fatal("mixed falsely uniform")
			}
		} else if catalog.StorageURI() != uri {
			t.Fatal("storage missing")
		}
		if _, err := catalog.ValidateBounds("", 4); err != nil {
			t.Fatal(err)
		}
		if _, err := catalog.ValidateBounds("", 6); err != nil {
			t.Fatal(err)
		}
		if _, err := catalog.ValidateBounds(CRS84h, 4); err == nil {
			t.Fatal("dimension mismatch accepted")
		}
		if _, err := catalog.ValidateOutput(""); err != nil {
			t.Fatal(err)
		}
		incompatible := CRS84h
		if dimension != provider.DimensionXY {
			incompatible = CRS84
		}
		if _, err := catalog.ValidateOutput(incompatible); err == nil {
			t.Fatal("loss/invention accepted")
		}
		uris := catalog.URIs()
		uris[0] = "changed"
		if catalog.URIs()[0] != uri {
			t.Fatal("catalog aliased")
		}
		for _, public := range catalog.URIs() {
			if _, err := catalog.Resolve(public); err != nil {
				t.Fatal(err)
			}
			if _, err := catalog.ValidateOutput(public); err != nil {
				t.Fatal("catalog advertised an incompatible output", public, err)
			}
		}
		if _, err := catalog.ValidateBounds(incompatible, 2*source.Dimension()); err == nil {
			t.Fatal("unlisted cross-dimensional explicit bbox URI accepted")
		}
	}
}

func TestCRSCollectionApplicationAndCollision(t *testing.T) {
	p, _ := crsconfig.NewHeightProjection(32633)
	source, _ := NewApplicationCRS(p.Definition(), 340000001, 3)
	catalog, err := NewCollectionCRS(source, provider.SpatialMetadata{Dimension: provider.DimensionXYZ, VerticalCRS: CRS84h})
	if err != nil {
		t.Fatal(err)
	}
	resolved, err := catalog.Resolve(source.URI())
	if err != nil || resolved.Definition().SHA256 != source.Definition().SHA256 {
		t.Fatal("application unresolved", err)
	}
	if catalog.StorageURI() != source.URI() {
		t.Fatal("storage not included")
	}
	if len(catalog.URIs()) != 4 {
		t.Fatal("unproven far-zone targets published", catalog.URIs())
	}
	farProjection, _ := crsconfig.NewHeightProjection(32760)
	far, _ := NewApplicationCRS(farProjection.Definition(), 32760, 3)
	if _, err := catalog.Resolve(far.URI()); err == nil {
		t.Fatal("unproven far-zone target published")
	}
	forged := source
	forged.definition.URI = CRS84h
	if _, err := NewCollectionCRS(forged, provider.SpatialMetadata{Dimension: provider.DimensionXYZ, VerticalCRS: CRS84h}); err == nil {
		t.Fatal("identity collision accepted")
	}
}

func TestCRSCanonicalNumericalFixtures(t *testing.T) {
	// Independently fixed Web Mercator and UTM33N coordinates for lon15,lat30.
	for _, tc := range []struct {
		srid uint64
		want []float64
		tol  float64
	}{{3857, []float64{1669792.3618991035, 3503549.843504374}, 1e-6}, {32633, []float64{500000, 3318785.3525812067}, 1e-5}} {
		c, _ := ResolveCRS(epsgURI(tc.srid))
		got, err := c.ForwardXY([]float64{15, 30})
		if err != nil {
			t.Fatal(err)
		}
		for i := range got {
			if math.Abs(got[i]-tc.want[i]) > tc.tol {
				t.Fatalf("%d: %v", tc.srid, got)
			}
		}
		back, err := c.InverseXY(got)
		if err != nil || math.Abs(back[0]-15) > 1e-9 || math.Abs(back[1]-30) > 1e-9 {
			t.Fatalf("inverse %v %v", back, err)
		}
	}
}

func TestCRSConcurrentDetached(t *testing.T) {
	c, _ := ResolveCRS(epsgURI(3857))
	var wg sync.WaitGroup
	for n := 0; n < 16; n++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 20; i++ {
				input := []float64{15, 30}
				got, err := c.ForwardXY(input)
				if err != nil || len(got) != 2 || input[0] != 15 {
					t.Error("projection failed or input changed", err)
				}
			}
		}()
	}
	wg.Wait()
}

func TestCRSZeroAndSourceProfiles(t *testing.T) {
	var zero CRS
	if _, err := zero.ForwardXY([]float64{1, 2}); err == nil {
		t.Fatal("zero usable")
	}
	var empty CollectionCRS
	if _, err := empty.Resolve(CRS84); !errors.Is(err, provider.ErrUnsupported) {
		t.Fatal(err)
	}
	xy, _ := ResolveCRS(CRS84)
	if _, err := NewCollectionCRS(xy, provider.SpatialMetadata{Dimension: provider.DimensionXYZ, VerticalCRS: CRS84h}); err == nil {
		t.Fatal("source dimension guessed")
	}
}

func TestCRSProjectionDomainFailure(t *testing.T) {
	c, _ := ResolveCRS(epsgURI(3857))
	if _, err := c.ForwardXY([]float64{15, 90}); err == nil {
		t.Fatal("Mercator pole silently returned coordinates")
	}
	if _, err := c.ForwardXY([]float64{math.NaN(), 30}); err == nil {
		t.Fatal("nonfinite source accepted")
	}
}
