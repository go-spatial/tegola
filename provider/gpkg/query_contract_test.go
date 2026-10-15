//go:build cgo

package gpkg_test

import (
	"context"
	"encoding/binary"
	"errors"
	"math"
	"reflect"
	"testing"

	"github.com/alexeydott/tegola/dict"
	"github.com/alexeydott/tegola/provider"
	"github.com/alexeydott/tegola/provider/gpkg"
	"github.com/alexeydott/tegola/provider/internal/querytest"
)

func TestFeatureQueryContract(t *testing.T) {
	querytest.Run(t, featureContractInstance)
}

func TestFeatureQueryNullableContract(t *testing.T) {
	querytest.RunNullableProfiles(t, func(t *testing.T, fixture querytest.Fixture) querytest.Instance {
		if fixture.Profile == querytest.CustomSelection {
			t.Skip("GeoPackage custom feature_sql admission is not implemented")
		}
		return featureContractInstance(t, fixture)
	}, querytest.ProfileOptions{})
}

func TestFeatureQueryFilterContract(t *testing.T) {
	// Custom SQL has an explicit Unsupported capability regression elsewhere.
	// This suite proves the admitted ordinary-table filter path only.
	querytest.RunFilterWithOptions(t, featureContractInstance, querytest.Options{})
}

func featureContractInstance(t *testing.T, fixture querytest.Fixture) querytest.Instance {
	format := "wkb"
	for _, row := range fixture.Rows {
		if row.EmptyGeometry || row.MalformedGeometry == "native" {
			format = "gpkg"
		}
		if row.Metadata {
			format = "mos"
		}
	}
	columns := "id INTEGER UNIQUE, geom BLOB, name TEXT, value INTEGER, start_time INTEGER, end_time INTEGER"
	if len(fixture.FilterFields) != 0 {
		columns += ", n INTEGER, s TEXT, b BOOLEAN"
	}
	fx := newRawFixture(t, []string{"CREATE TABLE items (" + columns + ")"})
	if format == "gpkg" {
		addFeatureMetadata(t, fx.path, "items", "geom", 4326)
	}
	layer := map[string]interface{}{
		"name": "items", "tablename": "items", "id_fieldname": "id", "geometry_fieldname": "geom",
		"geometry_format": format, "geometry_type": "point", "srid": 4326, "fields": []string{"name", "value"},
		"temporal_start_field": "start_time", "temporal_end_field": "end_time", "temporal_storage": "unix_nanoseconds",
	}
	if fixture.PublicFields != nil {
		fields := append([]string(nil), fixture.PublicFields...)
		if len(fields) == 0 {
			fields = []string{"id"}
		}
		layer["fields"] = fields
	}
	if fixture.InvalidTemporalMapping {
		layer["temporal_end_field"] = "missing"
	}
	storage := fixture.TemporalStorage
	if storage == 0 {
		storage = querytest.UnixNanoseconds
	}
	switch storage {
	case querytest.UnixSeconds:
		layer["temporal_storage"] = "unix_seconds"
	case querytest.UnixMilliseconds:
		layer["temporal_storage"] = "unix_milliseconds"
	case querytest.UnixMicroseconds:
		layer["temporal_storage"] = "unix_microseconds"
	case querytest.UnixNanoseconds:
	default:
		t.Fatal("unsupported fixture temporal storage")
	}
	tiler, err := gpkg.NewTileProvider(dict.Dict{"filepath": fx.path, "layers": []map[string]interface{}{layer}}, nil)
	if err != nil {
		return querytest.Instance{SetupError: err}
	}
	p := tiler.(*gpkg.Provider)
	t.Cleanup(func() {
		if err := p.Close(); err != nil {
			t.Error(err)
		}
	})
	var rows [][]interface{}
	for _, row := range fixture.Rows {
		var id, geometry, start, end any
		if !row.MissingID {
			id = int64(row.Feature.ID)
		}
		if row.Feature.Geometry != nil {
			geometry = wkbGeomBytes(t, row.Feature.Geometry)
			if format == "gpkg" {
				geometry = append(featureHeader(false, 4326), geometry.([]byte)...)
			}
		}
		if row.EmptyGeometry {
			body := binary.LittleEndian.AppendUint32([]byte{1}, 1)
			body = binary.LittleEndian.AppendUint64(body, math.Float64bits(math.NaN()))
			body = binary.LittleEndian.AppendUint64(body, math.Float64bits(math.NaN()))
			geometry = append(featureHeader(true, 4326), body...)
		}
		if row.MalformedGeometry != "" {
			geometry = []byte{0xff}
		}
		if row.Metadata {
			geometry = mosSystemInfoBlob(2, "", 0, false)
		}
		if row.Start != nil {
			start, err = querytest.EncodeFixtureTemporal(*row.Start, storage)
			if err != nil {
				t.Fatal(err)
			}
		}
		if row.End != nil {
			end, err = querytest.EncodeFixtureTemporal(*row.End, storage)
			if err != nil {
				t.Fatal(err)
			}
		}
		values := []interface{}{id, geometry, row.Feature.Tags["name"], row.Feature.Tags["value"], start, end}
		if len(fixture.FilterFields) != 0 {
			values = append(values, row.Feature.Tags["n"], row.Feature.Tags["s"], row.Feature.Tags["b"])
		}
		rows = append(rows, values)
	}
	insertColumns := []string{"id", "geom", "name", "value", "start_time", "end_time"}
	if len(fixture.FilterFields) != 0 {
		insertColumns = append(insertColumns, "n", "s", "b")
	}
	insertRows(t, fx.path, "items", insertColumns, rows)
	layers, err := p.Layers()
	if err != nil {
		t.Fatal(err)
	}
	return querytest.Instance{Querier: p, Layer: "items", CountMode: querytest.OptionalCount, QueryableLayer: layers[0].(provider.FeatureQueryableLayerInfo)}
}

func featureHeader(empty bool, srid uint32) []byte {
	flags := byte(1)
	if empty {
		flags |= 1 << 4
	}
	return binary.LittleEndian.AppendUint32([]byte{'G', 'P', 0, flags}, srid)
}

func addFeatureMetadata(t *testing.T, path, table, geometry string, srid int) {
	t.Helper()
	db, err := openSQLite(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			t.Error(err)
		}
	}()
	for _, ddl := range []string{
		"CREATE TABLE gpkg_contents (table_name TEXT, data_type TEXT, min_x REAL, min_y REAL, max_x REAL, max_y REAL, srs_id INTEGER)",
		"CREATE TABLE gpkg_geometry_columns (table_name TEXT, column_name TEXT, geometry_type_name TEXT, srs_id INTEGER, z INTEGER, m INTEGER)",
	} {
		if _, err := db.Exec(ddl); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := db.Exec("INSERT INTO gpkg_contents VALUES (?, 'features', -180, -90, 180, 90, ?)", table, srid); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("INSERT INTO gpkg_geometry_columns VALUES (?, ?, 'POINT', ?, 0, 0)", table, geometry, srid); err != nil {
		t.Fatal(err)
	}
}

func TestFeatureQueryFieldsAndUnsignedIDs(t *testing.T) {
	fx := newRawFixture(t, []string{"CREATE TABLE items (id INTEGER PRIMARY KEY, geom TEXT, name TEXT)"})
	insertRows(t, fx.path, "items", []string{"id", "geom", "name"}, [][]interface{}{{1, "POINT (1 2)", "one"}})
	tiler, err := gpkg.NewTileProvider(dict.Dict{"filepath": fx.path, "layers": []map[string]interface{}{{"name": "items", "tablename": "items", "id_fieldname": "id", "geometry_fieldname": "geom", "geometry_format": "wkt", "srid": 4326}}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	p := tiler.(*gpkg.Provider)
	t.Cleanup(func() { _ = p.Close() })
	var ids []uint64
	result, err := p.QueryFeatures(context.Background(), "items", provider.FeatureQuery{Limit: 10, IDs: []uint64{math.MaxUint64, 1}}, func(f *provider.Feature) error { ids = append(ids, f.ID); return nil })
	if err != nil || result.NumberReturned != 1 || !reflect.DeepEqual(ids, []uint64{1}) {
		t.Fatalf("unsigned identity: %+v %v %v", result, ids, err)
	}
	result, err = p.QueryFeatures(context.Background(), "items", provider.FeatureQuery{Limit: 10, IDs: []uint64{math.MaxUint64}}, func(*provider.Feature) error { t.Fatal("impossible identity delivered"); return nil })
	if err != nil || result.NumberMatched == nil || *result.NumberMatched != 0 {
		t.Fatalf("impossible ID: %+v %v", result, err)
	}
	_, err = p.QueryFeatures(context.Background(), "items", provider.FeatureQuery{Limit: 10, Fields: []string{"name` FROM items;--"}}, func(*provider.Feature) error { t.Fatal("invalid field delivered"); return nil })
	var invalid provider.InvalidFeatureQueryError
	if !errors.As(err, &invalid) {
		t.Fatalf("field allowlist: %v", err)
	}
}

var _ provider.FeatureQuerier = (*gpkg.Provider)(nil)
var _ provider.TemporalLayerInfo = gpkg.Layer{}
var _ provider.FeatureQueryLayerInfo = gpkg.Layer{}
