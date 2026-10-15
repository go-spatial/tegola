package wfs

import (
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/alexeydott/tegola/dict"
	"github.com/alexeydott/tegola/ogc/features"
	"github.com/alexeydott/tegola/provider"
	"github.com/alexeydott/tegola/provider/postgis"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Exercise the actual PostgreSQL scalar catalog, SQL binding and persisted IDs.
// A private schema avoids collisions with concurrent provider-package fixtures.
func TestNativePostGISTypedFES(t *testing.T) {
	dsn := os.Getenv("TEGOLA_REVIEW_POSTGIS_DSN")
	if dsn == "" {
		t.Skip("TEGOLA_REVIEW_POSTGIS_DSN is unset")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	schema := fmt.Sprintf("fes_native_%d", time.Now().UnixNano())
	if _, err := pool.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if _, err := pool.Exec(ctx, "DROP SCHEMA "+schema+" CASCADE"); err != nil {
			t.Error(err)
		}
	})
	table := schema + ".sites"
	_, err = pool.Exec(ctx, "CREATE TABLE "+table+` (
 fid bigint PRIMARY KEY, geom text, code text, active boolean,
 n bigint, amount numeric, day date, moment timestamptz);
 INSERT INTO `+table+` VALUES
 (1,'POINT (1 2)','123',true,9007199254740993,12345678901234567890.12345678901234567890,'2026-10-04','2026-10-04T08:00:00.123456Z'),
 (2,'POINT (1 2)','00123',false,9007199254740992,12345678901234567890.12345678901234567891,'2026-10-05','2026-10-04T08:00:00.123457Z');`)
	if err != nil {
		t.Fatal(err)
	}
	tiler, err := postgis.NewTileProvider(dict.Dict{"name": "fes_native", "uri": dsn, "layers": []map[string]any{{
		"name": "sites", "tablename": table, "id_fieldname": "fid", "geometry_fieldname": "geom",
		"geometry_type": "point", "geometry_format": "wkt", "srid": 4326,
		"fields": []string{"code", "active", "n"},
	}}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	p := tiler.(*postgis.Provider)
	t.Cleanup(p.Close)
	layers, err := p.Layers()
	if err != nil {
		t.Fatal(err)
	}
	service, err := features.NewService([]features.CollectionSource{{ID: "sites", Layer: layers[0], Querier: p}})
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"amount", "day", "moment"} {
		t.Run("unsupported_native_"+field, func(t *testing.T) {
			tiler, err := postgis.NewTileProvider(dict.Dict{"name": "fes_rejected", "uri": dsn, "layers": []map[string]any{{
				"name": "sites", "tablename": table, "id_fieldname": "fid", "geometry_fieldname": "geom",
				"geometry_type": "point", "geometry_format": "wkt", "srid": 4326, "fields": []string{field},
			}}}, nil)
			if err != nil {
				t.Fatal(err)
			}
			p := tiler.(*postgis.Provider)
			defer p.Close()
			layers, err := p.Layers()
			if err != nil {
				t.Fatal(err)
			}
			_, err = features.NewService([]features.CollectionSource{{ID: "sites", Layer: layers[0], Querier: p}})
			if !errors.Is(err, provider.ErrUnsupported) {
				t.Fatalf("native %s must be rejected by the source output profile: %v", field, err)
			}
		})
	}
	for _, tc := range []struct{ property, literal, id string }{
		{"code", "123", "sites.1"},
		{"code", "00123", "sites.2"},
		{"active", "true", "sites.1"},
		{"active", "false", "sites.2"},
		{"n", "9007199254740993", "sites.1"},
	} {
		t.Run(tc.property+"="+tc.literal, func(t *testing.T) {
			filter := `<fes:Filter xmlns:fes="http://www.opengis.net/fes/2.0" xmlns:app="http://example.com/tegola/sites"><fes:PropertyIsEqualTo><fes:ValueReference>app:` + tc.property + `</fes:ValueReference><fes:Literal>` + tc.literal + `</fes:Literal></fes:PropertyIsEqualTo></fes:Filter>`
			req, ex := ParseGetFeatureKVP(V202, map[string]string{"typename": "app:sites", "filter": filter})
			if len(ex) > 0 {
				t.Fatal(ex)
			}
			raw, ex := ExecuteGetFeature(ctx, service, req)
			if len(ex) > 0 {
				t.Fatal(ex)
			}
			var collection struct {
				Members []struct {
					Feature struct {
						ID string `xml:"id,attr"`
					} `xml:",any"`
				} `xml:"member"`
			}
			if err := xml.Unmarshal([]byte(raw), &collection); err != nil {
				t.Fatal(err)
			}
			if len(collection.Members) != 1 || collection.Members[0].Feature.ID != tc.id {
				t.Fatalf("want only %s, got %s", tc.id, raw)
			}
		})
	}
}
