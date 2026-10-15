package features

import (
	"strings"
	"testing"

	"github.com/alexeydott/tegola/provider"
)

func TestCustomApplicationCRSCatalog(t *testing.T) {
	const definition = "+proj=etmerc +ellps=bessel +towgs84=1,2,3,0,0,0,0 +lon_0=9 +lat_0=50 +k_0=1 +x_0=0 +y_0=0 +units=m +no_defs"
	storage, err := NewApplicationCRS(definition, 340000001, 2)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(storage.URI(), "urn:uuid:") || storage.Definition().Datum == "WGS84" || storage.projection.CanonicalSRID() != 0 {
		t.Fatal("custom CRS claims canonical datum or authority")
	}
	catalog, err := NewCollectionCRS(storage, provider.SpatialMetadata{Dimension: provider.DimensionXY})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := catalog.ValidateOutput(storage.URI()); err != nil {
		t.Fatal(err)
	}
	if _, err := catalog.ValidateOutput(epsgURI(0)); err == nil {
		t.Fatal("EPSG zero advertised")
	}
	if _, err := NewApplicationCRS(definition, 340000001, 3); err == nil {
		t.Fatal("custom height admitted")
	}
}
