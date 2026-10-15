package hana_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/alexeydott/geom"
	"github.com/alexeydott/tegola"
	"github.com/alexeydott/tegola/dict"
	"github.com/alexeydott/tegola/provider"
	"github.com/alexeydott/tegola/provider/hana"
	"github.com/alexeydott/tegola/provider/internal/querytest"
)

func TestFeatureLiveRawTileStartupPreservesNativeCheck(t *testing.T) {
	if os.Getenv(TESTENV) != "yes" || GetConnectionURI() == "" {
		t.Skip("live HANA")
	}
	db, err := hana.OpenDB(GetConnectionURI())
	if err != nil {
		t.Fatal(err)
	}
	defer featureFixtureClose(t, db)
	for _, c := range []struct{ format, mode string }{
		{"wkb", "auto"}, {"wkt", "auto"}, {"mos", "auto"}, {"", "auto"},
		{"mos", "bbox"}, {"", "native"},
	} {
		t.Run("format="+c.format+"/"+c.mode, func(t *testing.T) {
			format := c.format
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			table := featureFixtureTable(t, "TEGOLA_TILE_STARTUP")
			q := `"` + table + `"`
			typ := "BLOB"
			var value any
			wire, err := querytest.EncodeFixtureWKB(geom.Point{1, 2})
			if err != nil {
				t.Fatal(err)
			}
			value = wire
			if format == "wkt" {
				typ = "NCLOB"
				value = "POINT(1 2)"
			}
			if format == "mos" {
				value = hanaFixtureMOS(geom.Point{1, 2})
			}
			if c.mode == "native" {
				typ = "ST_POINT(4326)"
			}
			tableType := "ROW"
			if c.mode == "native" {
				tableType = "COLUMN"
			}
			if _, err := db.ExecContext(ctx, `CREATE `+tableType+` TABLE `+q+` ("id" BIGINT PRIMARY KEY,"geom" `+typ+`,"minx" DOUBLE,"maxx" DOUBLE,"miny" DOUBLE,"maxy" DOUBLE)`); err != nil {
				t.Fatal(err)
			}
			defer func() {
				cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cleanupCancel()
				if _, err := db.ExecContext(cleanupCtx, "DROP TABLE "+q); err != nil {
					t.Error(err)
				}
			}()
			insert := `INSERT INTO ` + q + ` VALUES(1,?,0,0,0,0)`
			if c.mode == "native" {
				insert = `INSERT INTO ` + q + ` VALUES(1,NEW ST_POINT('POINT(1 2)',4326),0,0,0,0)`
				value = nil
			}
			var insertArgs []any
			if value != nil {
				insertArgs = []any{value}
			}
			if _, err := db.ExecContext(ctx, insert, insertArgs...); err != nil {
				t.Fatal(err)
			}
			layer := map[string]any{"name": "items", "tablename": q, "id_fieldname": "id", "geometry_fieldname": "geom", "geometry_type": "point", "srid": 4326}
			if format != "" {
				layer["geometry_format"] = format
			}
			if format == "mos" {
				layer["mos_precision"] = 7
				layer["mos_units"] = "m"
			}
			if c.mode == "bbox" {
				delete(layer, "tablename")
				layer["sql"] = `SELECT "id","geom","minx","maxx","miny","maxy" FROM ` + q + ` WHERE !BBOX!`
				layer["bbox_minx_fieldname"] = "minx"
				layer["bbox_maxx_fieldname"] = "maxx"
				layer["bbox_miny_fieldname"] = "miny"
				layer["bbox_maxy_fieldname"] = "maxy"
			}
			p, err := hana.CreateProvider(dict.Dict{hana.ConfigKeyName: "tile", hana.ConfigKeyURI: GetConnectionURI(), "layers": []map[string]any{layer}}, nil, hana.ProviderType)
			if format == "" && c.mode != "native" {
				if p != nil {
					p.Close()
				}
				if err == nil {
					t.Fatal("native registration admitted VARBINARY geometry")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			defer p.Close()
			calls := 0
			err = p.TileFeatures(ctx, "items", provider.NewTile(0, 0, 0, 0, tegola.WebMercator), nil, func(f *provider.Feature) error {
				calls++
				if f.ID != 1 {
					t.Fatal("tile source identity changed")
				}
				return nil
			})
			if err != nil || calls != 1 {
				t.Fatalf("raw tile startup/query %s calls%d: %v", format, calls, err)
			}
			calls = 0
			err = p.TileFeatures(ctx, "items", provider.NewTile(1, 0, 0, 0, tegola.WebMercator), nil, func(*provider.Feature) error {
				calls++
				return nil
			})
			if err != nil || calls != 0 {
				t.Fatalf("outside tile extent was not clipped: calls%d %v", calls, err)
			}
		})
	}
}
