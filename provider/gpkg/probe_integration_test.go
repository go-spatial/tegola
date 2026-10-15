//go:build cgo

package gpkg

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/alexeydott/tegola/provider"
	codec "github.com/alexeydott/tegola/provider/geometrycodec"
	"github.com/alexeydott/tegola/provider/test/fixture"
)

func TestIntegrationMOSMetadataOnlyProbe(t *testing.T) {
	queries := []string{}
	db := fixture.OpenSQLRows(t, fixture.SQLRows{Columns: []string{"id", "geom", "MINX", "MAXX", "MINY", "MAXY"}, QueryLog: &queries})
	layer := Layer{geomFieldname: "geom", geometryFormat: codec.FormatMOS, bboxFields: codec.DefaultBBoxFields()}
	_, c, err := probeMOSCustomSQLContract(db, &layer, "SELECT id,geom,MINX,MAXX,MINY,MAXY FROM items ORDER BY id")
	if err != nil {
		t.Fatal(err)
	}
	if len(queries) != 1 || strings.Contains(queries[0], "LIMIT 16") || strings.Contains(queries[0], "__tegola_bounds_probe") || !strings.HasSuffix(queries[0], "WHERE 1=0") {
		t.Fatalf("metadata probe %v", queries)
	}
	if c.ValidRows != 0 || c.ValidMOSRows != 0 {
		t.Fatal("metadata probe sampled rows")
	}
}

func TestIntegrationBBoxQualifierCannotAdmitCustomFeatures(t *testing.T) {
	p, _ := queryTestProvider(t, "CREATE TABLE items(id INTEGER PRIMARY KEY, geom TEXT); INSERT INTO items VALUES(1,'POINT(1 2)');", map[string]interface{}{"sql": "SELECT id,geom FROM items", "bbox_table": "items"})
	callbacks := 0
	_, err := p.QueryFeatures(context.Background(), "items", provider.FeatureQuery{Limit: 1}, func(*provider.Feature) error { callbacks++; return nil })
	if !errors.Is(err, provider.ErrUnsupported) || callbacks != 0 {
		t.Fatalf("custom feature admission %v callbacks%d", err, callbacks)
	}
}
