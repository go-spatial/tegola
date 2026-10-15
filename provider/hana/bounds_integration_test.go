package hana

import (
	"errors"
	"strings"
	"testing"

	"github.com/alexeydott/geom"
	"github.com/alexeydott/tegola/dict"
	"github.com/alexeydott/tegola/provider"
	codec "github.com/alexeydott/tegola/provider/geometrycodec"
	"github.com/alexeydott/tegola/provider/test/fixture"
)

func TestIntegrationMOSBoundsQualifierPreservesFeatureMetadata(t *testing.T) {
	source := &featureSource{SRID: 4326, Temporal: provider.TemporalMapping{InstantField: "at"}, Spatial: provider.SpatialMetadata{Dimension: provider.DimensionXY}}
	layer := Layer{name: "items", srid: 4326, geometryFormat: codec.FormatMOS, bboxTable: "source.items", bboxFields: codec.DefaultBBoxFields(), mosConfig: codec.MOSConfig{UnitFactor: 1}, featureTable: "physical_items", featureSRID: 4326, feature: source}
	tile := fixture.Tile{Z: 0, X: 0, Y: 0, SRID: 4326, Bounds: geom.NewExtent([2]float64{10, 20}, [2]float64{30, 40})}
	got, err := replaceTokens(2, "SELECT id,geom FROM source.items WHERE !BBOX! ORDER BY id", &layer, geom.Point{}, 4326, tile, false)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, `"source"."items"."MAXX"`) || !strings.Contains(got, "ORDER BY id") {
		t.Fatalf("qualified predicate: %q", got)
	}
	if layer.feature != source || layer.featureTable != "physical_items" || layer.FeatureSourceSRID() != 4326 {
		t.Fatal("feature identity mutated")
	}
	mapping, err := layer.TemporalMapping()
	if err != nil || mapping.InstantField != "at" {
		t.Fatalf("temporal metadata %v %v", mapping, err)
	}
	spatial, err := layer.SpatialMetadata()
	if err != nil || spatial.Dimension != provider.DimensionXY {
		t.Fatalf("spatial metadata %v %v", spatial, err)
	}
}

func TestIntegrationBBoxQualifierDoesNotAdmitCustomFeatureSQL(t *testing.T) {
	layer := Layer{name: "items", sql: "SELECT id,geom FROM items", geometryFormat: codec.FormatMOS, bboxTable: "items", featureSRID: 4326}
	p := Provider{}
	if err := p.registerFeatureSource(&layer, dict.Dict{}, ProviderType); err != nil {
		t.Fatal(err)
	}
	if !errors.Is(layer.FeatureQuerySupported(), provider.ErrUnsupported) {
		t.Fatal("tile qualifier admitted unproven feature source")
	}
}
