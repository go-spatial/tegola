package hana_test

import (
	"context"
	"encoding/hex"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/alexeydott/tegola/dict"
	"github.com/alexeydott/tegola/provider"
	"github.com/alexeydott/tegola/provider/hana"
	"github.com/alexeydott/tegola/provider/internal/querytest"
)

func TestFeatureFilterLiveContract(t *testing.T) {
	if os.Getenv(TESTENV) != "yes" || GetConnectionURI() == "" {
		t.Skip("live HANA filters require explicit connection configuration")
	}
	for _, format := range []string{"wkb", "wkt", "mos", "native"} {
		t.Run(format, func(t *testing.T) {
			options := querytest.ProfileOptions{Custom: querytest.Options{PublicTemporalProperties: &querytest.TemporalPropertyProfile{StartField: "start_time", EndField: "end_time", Storage: querytest.UnixSeconds}}}
			querytest.RunFilterProfiles(t, hanaFilterFactory(format), options)
		})
	}
}

func hanaFilterFactory(format string) querytest.Factory {
	return func(t *testing.T, fixture querytest.Fixture) querytest.Instance {
		t.Helper()
		db, err := hana.OpenDB(GetConnectionURI())
		if err != nil {
			t.Fatal("open live HANA filter fixture")
		}
		t.Cleanup(func() {
			if err := db.Close(); err != nil {
				t.Error("close filter fixture")
			}
		})
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		quoted := `"` + featureFixtureTable(t, "TEGOLA_FILTER") + `"`
		kind, geometryType := "ROW", "VARBINARY(5000)"
		if format == "wkt" {
			geometryType = "NCLOB"
		}
		if format == "native" {
			kind, geometryType = "COLUMN", "ST_GEOMETRY(4326)"
		}
		if _, err := db.ExecContext(ctx, "CREATE "+kind+" TABLE "+quoted+` ("id" BIGINT UNIQUE,"geom" `+geometryType+`,"n" BIGINT,"s" NVARCHAR(5000),"b" BOOLEAN,"start_time" BIGINT,"end_time" BIGINT,"included" INTEGER)`); err != nil {
			t.Fatalf("create filter fixture: %v", err)
		}
		t.Cleanup(func() {
			dropCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			if _, err := db.ExecContext(dropCtx, "DROP TABLE "+quoted); err != nil {
				t.Error("drop filter fixture")
			}
		})
		for _, row := range fixture.Rows {
			var geometry, start, end any
			if row.Feature.Geometry != nil {
				wire, err := querytest.EncodeFixtureWKB(row.Feature.Geometry)
				if err != nil {
					t.Fatal(err)
				}
				geometry = wire
				if format == "wkt" {
					geometry = hanaFixtureWKT(row.Feature.Geometry)
				}
				if format == "mos" {
					geometry = hanaFixtureMOS(row.Feature.Geometry)
				}
				if format == "native" {
					geometry = hex.EncodeToString(wire)
				}
			}
			if row.Start != nil {
				start, err = querytest.EncodeFixtureTemporal(*row.Start, querytest.UnixSeconds)
				if err != nil {
					t.Fatal(err)
				}
			}
			if row.End != nil {
				end, err = querytest.EncodeFixtureTemporal(*row.End, querytest.UnixSeconds)
				if err != nil {
					t.Fatal(err)
				}
			}
			included := 1
			if row.ExcludedBySelection {
				included = 0
			}
			parameter := "?"
			if format == "native" {
				parameter = "ST_GeomFromWKB(HEXTOBIN(?),4326)"
			}
			if _, err := db.ExecContext(ctx, "INSERT INTO "+quoted+` VALUES (?,`+parameter+`,?,?,?,?,?,?)`, int64(row.Feature.ID), geometry, row.Feature.Tags["n"], row.Feature.Tags["s"], row.Feature.Tags["b"], start, end, included); err != nil {
				t.Fatalf("insert filter fixture: %v", err)
			}
		}
		fields := append([]string(nil), fixture.PublicFields...)
		layer := map[string]any{"name": "items", "tablename": quoted, "id_fieldname": "id", "geometry_fieldname": "geom", "geometry_format": format, "geometry_type": "point", "srid": 4326, "fields": fields, "temporal_start_field": "start_time", "temporal_end_field": "end_time", "temporal_storage": "unix_seconds"}
		if format == "native" {
			delete(layer, "geometry_format")
			layer["spatial_dimension"] = "xy"
		}
		if format == "mos" {
			layer["mos_precision"] = float64(7)
			layer["mos_units"] = "m"
		}
		if fixture.Profile == querytest.CustomSelection {
			projection := []string{`"id"`, `"geom"`}
			for _, field := range fields {
				if field != "start_time" && field != "end_time" {
					projection = append(projection, `"`+field+`"`)
				}
			}
			projection = append(projection, `"start_time"`, `"end_time"`)
			layer["feature_sql"] = "SELECT " + strings.Join(projection, ",") + " FROM " + quoted + ` WHERE "included"=1`
		}
		p, err := hana.CreateProvider(dict.Dict{hana.ConfigKeyName: "filter_contract", hana.ConfigKeyURI: GetConnectionURI(), "layers": []map[string]any{layer}}, nil, hana.ProviderType)
		if err != nil {
			return querytest.Instance{SetupError: err}
		}
		t.Cleanup(p.Close)
		layers, err := p.Layers()
		if err != nil || len(layers) != 1 {
			t.Fatal("actual filter layer metadata unavailable")
		}
		queryable, ok := layers[0].(provider.FeatureQueryableLayerInfo)
		if !ok {
			t.Fatal("actual filter layer lacks optional metadata")
		}
		return querytest.Instance{Querier: p, Layer: "items", CountMode: querytest.OptionalCount, QueryableLayer: queryable}
	}
}
