package postgis

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"os"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alexeydott/geom"
	"github.com/alexeydott/tegola/dict"
	"github.com/alexeydott/tegola/provider"
	"github.com/alexeydott/tegola/provider/internal/querytest"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var featureFixtureSequence atomic.Uint64

func TestFeaturePostGISLiveVersions(t *testing.T) {
	if os.Getenv("RUN_POSTGIS_TESTS") != "yes" {
		t.Skip("live PostGIS version probe is opt-in")
	}
	if os.Getenv("PGURI") == "" {
		t.Fatal("live version probe requires explicit connection environment")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	db, err := pgxpool.New(ctx, os.Getenv("PGURI"))
	if err != nil {
		t.Fatal("cannot initialize live version probe")
	}
	defer db.Close()
	var server, extension string
	if err := db.QueryRow(ctx, "SELECT current_setting('server_version'),extversion FROM pg_catalog.pg_extension WHERE extname='postgis'").Scan(&server, &extension); err != nil {
		var databaseError *pgconn.PgError
		if errors.As(err, &databaseError) {
			t.Fatalf("live version probe failed with SQLSTATE %s", databaseError.Code)
		}
		t.Fatalf("live PostgreSQL/PostGIS version probe failed (%T)", err)
	}
	t.Logf("PostgreSQL %s; PostGIS %s", server, extension)
}

func featureLiveDatabase(t *testing.T) (*pgxpool.Pool, string) {
	t.Helper()
	if os.Getenv("RUN_POSTGIS_TESTS") != "yes" {
		t.Skip("live PostGIS unavailable: set RUN_POSTGIS_TESTS=yes and PGURI for an isolated writable test database")
	}
	uri := os.Getenv("PGURI")
	if uri == "" {
		t.Fatal("live feature tests require explicit PGURI")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	db, err := pgxpool.New(ctx, uri)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Ping(ctx); err != nil {
		db.Close()
		t.Fatal(err)
	}
	schema := fmt.Sprintf("feature_query_%d_%d", time.Now().UnixNano(), featureFixtureSequence.Add(1))
	if _, err := db.Exec(ctx, "CREATE SCHEMA "+pgQuoteIdent(schema)); err != nil {
		db.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if _, err := db.Exec(cleanup, "DROP SCHEMA "+pgQuoteIdent(schema)+" CASCADE"); err != nil {
			t.Error(err)
		}
		db.Close()
	})
	return db, schema
}

func TestFeaturePostGISLiveContract(t *testing.T) {
	db, schema := featureLiveDatabase(t)
	factory := func(t *testing.T, fixture querytest.Fixture) querytest.Instance {
		custom := fixture.Profile == querytest.CustomSelection
		if fixture.PublicFields == nil {
			t.Fatal("live fixture must declare its public property schema")
		}
		publicFields := make([]string, 0, len(fixture.PublicFields))
		for _, name := range fixture.PublicFields {
			if name == "start_time" || name == "end_time" {
				continue
			}
			if name != "name" && name != "value" {
				t.Fatal("fixture property lacks a declared source column")
			}
			publicFields = append(publicFields, name)
		}
		for _, row := range fixture.Rows {
			if row.MalformedGeometry != "native" {
				continue
			}
			// Attempt the literal malformed body through the real native constructor.
			// Native rejection is distinct from corrupt bytes returned by a source query.
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			var extensionSchema string
			if err := db.QueryRow(ctx, `SELECT n.nspname FROM pg_catalog.pg_extension e JOIN pg_catalog.pg_namespace n ON n.oid=e.extnamespace WHERE e.extname='postgis'`).Scan(&extensionSchema); err != nil {
				t.Fatal(err)
			}
			_, err := db.Exec(ctx, "SELECT "+pgQuoteIdent(extensionSchema)+".ST_GeomFromWKB($1::bytea)", []byte{0xff})
			var parseError *pgconn.PgError
			if !errors.As(err, &parseError) || (parseError.Code != "XX000" && parseError.Code != "22023") {
				t.Fatalf("malformed native wire did not produce a geometry parser error: %v", err)
			}
			return querytest.Instance{StorageRejectedNativeCorruption: &querytest.NativeStorageEvidence{
				Backend: "postgis", Operation: querytest.NativeGeometryConstruction,
				Scope: querytest.MalformedNativeWire, FeatureID: row.Feature.ID,
				Error: err, SQLState: parseError.Code,
			}}
		}
		table := fmt.Sprintf("items_%d", featureFixtureSequence.Add(1))
		qualified := pgQuoteIdent(schema) + "." + pgQuoteIdent(table)
		columns := "id bigint UNIQUE,geom bytea,name text,value bigint,start_time bigint,end_time bigint,selection_flag integer"
		tileTable := qualified
		if custom {
			columns = "source_id bigint UNIQUE,source_geom bytea,source_name text,source_value bigint,source_start bigint,source_end bigint,selection_flag integer"
			tileTable = pgQuoteIdent(schema) + "." + pgQuoteIdent(table+"_tiles")
			if _, err := db.Exec(context.Background(), "CREATE TABLE "+tileTable+" (id bigint UNIQUE,geom bytea,name text,value bigint)"); err != nil {
				t.Fatal(err)
			}
		}
		if _, err := db.Exec(context.Background(), "CREATE TABLE "+qualified+" ("+columns+")"); err != nil {
			t.Fatal(err)
		}
		format := "wkb"
		storage := fixture.TemporalStorage
		if storage == 0 {
			storage = querytest.UnixNanoseconds
		}
		storageName := ""
		switch storage {
		case querytest.UnixSeconds:
			storageName = "unix_seconds"
		case querytest.UnixMilliseconds:
			storageName = "unix_milliseconds"
		case querytest.UnixMicroseconds:
			storageName = "unix_microseconds"
		case querytest.UnixNanoseconds:
			storageName = "unix_nanoseconds"
		default:
			t.Fatal("unknown fixture temporal storage")
		}
		for _, row := range fixture.Rows {
			if row.Metadata {
				format = "mos"
			}
		}
		for _, row := range fixture.Rows {
			var id, geometry, start, end any
			if !row.MissingID {
				id = int64(row.Feature.ID)
			}
			literalGeometry := row.Feature.Geometry
			if row.RawEmptyGeometry != nil {
				literalGeometry = row.RawEmptyGeometry
			}
			if literalGeometry != nil {
				bytes, err := querytest.EncodeFixtureWKB(literalGeometry)
				if err != nil {
					t.Fatal(err)
				}
				geometry = bytes
			}
			if row.EmptyGeometry {
				bytes := binary.LittleEndian.AppendUint32([]byte{1}, 1)
				bytes = binary.LittleEndian.AppendUint64(bytes, math.Float64bits(math.NaN()))
				bytes = binary.LittleEndian.AppendUint64(bytes, math.Float64bits(math.NaN()))
				geometry = bytes
			}
			if row.MalformedGeometry == "wkb" {
				geometry = []byte{0xff}
			}
			if row.Metadata {
				metadata := make([]byte, 64)
				copy(metadata, []byte{5, 'V', 'e', 'r', ' ', '1'})
				geometry = metadata
			}
			if row.RawTemporal == nil && row.Start != nil {
				value, err := querytest.EncodeFixtureTemporal(*row.Start, storage)
				if err != nil {
					t.Fatal(err)
				}
				start = value
			}
			if row.RawTemporal == nil && row.End != nil {
				value, err := querytest.EncodeFixtureTemporal(*row.End, storage)
				if err != nil {
					t.Fatal(err)
				}
				end = value
			}
			if row.RawTemporal != nil {
				start, end = nil, nil
				if row.RawTemporal.Start != nil {
					start = *row.RawTemporal.Start
				}
				if row.RawTemporal.End != nil {
					end = *row.RawTemporal.End
				}
			}
			selectionFlag := int32(1)
			if row.ExcludedBySelection {
				selectionFlag = 0
			}
			if _, err := db.Exec(context.Background(), "INSERT INTO "+qualified+" VALUES ($1,$2,$3,$4,$5,$6,$7)", id, geometry, row.Feature.Tags["name"], row.Feature.Tags["value"], start, end, selectionFlag); err != nil {
				t.Fatal(err)
			}
		}
		conf := map[string]any{"name": "items", "tablename": tileTable, "id_fieldname": "id", "geometry_fieldname": "geom", "geometry_format": format, "geometry_type": "point", "srid": 4326, "temporal_start_field": "start_time", "temporal_end_field": "end_time", "temporal_storage": storageName}
		if fixture.Spatial.Dimension != provider.DimensionUnknown {
			switch fixture.Spatial.Dimension {
			case provider.DimensionXY:
				conf["spatial_dimension"] = "xy"
			case provider.DimensionXYZ:
				conf["spatial_dimension"] = "xyz"
			case provider.DimensionMixedXYXYZ:
				conf["spatial_dimension"] = "mixed_xy_xyz"
			default:
				t.Fatal("unknown dimensional fixture declaration")
			}
			if fixture.Spatial.VerticalCRS != "" {
				conf["vertical_crs"] = fixture.Spatial.VerticalCRS
			}
		}
		if fixture.InvalidTemporalMapping {
			conf["temporal_end_field"] = "missing"
		}
		if custom {
			projections := []string{"f.source_id AS id", "f.source_geom AS geom"}
			for _, name := range publicFields {
				projections = append(projections, "f."+pgQuoteIdent("source_"+name)+" AS "+pgQuoteIdent(name))
			}
			projections = append(projections, "f.source_start AS start_time", "f.source_end AS end_time")
			conf["feature_sql"] = "SELECT " + strings.Join(projections, ",") + " FROM " + qualified + " f WHERE f.selection_flag = 1"
		} else {
			conf["fields"] = publicFields
			if len(publicFields) == 0 {
				// Empty config fields means all. Selecting only mandatory identity and
				// geometry implements the fixture's explicit empty public schema.
				conf["fields"] = []string{"id", "geom"}
			}
		}
		var expectedFields []string
		if !custom {
			expectedFields = append([]string(nil), conf["fields"].([]string)...)
		}
		tiler, err := NewTileProvider(dict.Dict{"name": "live_feature_contract", "uri": os.Getenv("PGURI"), "layers": []map[string]any{conf}}, nil)
		if !custom && len(expectedFields) > 0 && !reflect.DeepEqual(conf["fields"], expectedFields) {
			t.Fatal("tile SQL generation mutated the configured public field names")
		}
		if err != nil {
			return querytest.Instance{SetupError: err}
		}
		p := tiler.(*Provider)
		t.Cleanup(func() { p.pool.Close() })
		return querytest.Instance{Querier: p, Layer: "items", CountMode: querytest.UnknownCount}
	}
	options := querytest.ProfileOptions{Custom: querytest.Options{PublicTemporalProperties: &querytest.TemporalPropertyProfile{
		StartField: "start_time", EndField: "end_time", Storage: querytest.UnixNanoseconds,
	}}}
	querytest.RunProfiles(t, factory, options)
	querytest.RunDimensionalProfiles(t, factory, options)
	querytest.RunExactTemporalProfiles(t, factory, options)
	querytest.RunNullableProfiles(t, factory, options)
}

func TestFeaturePostGISNativeXYZLive(t *testing.T) {
	db, schema := featureLiveDatabase(t)
	qualified := pgQuoteIdent(schema) + `."native_items"`
	if _, err := db.Exec(context.Background(), "CREATE TABLE "+qualified+" (id bigint PRIMARY KEY,geom geometry(PointZ,4326),name text)"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(context.Background(), "INSERT INTO "+qualified+" VALUES (1,ST_GeomFromText('POINT Z(15 0 7)',4326),'height')"); err != nil {
		t.Fatal(err)
	}
	conf := map[string]any{"name": "items", "tablename": qualified, "id_fieldname": "id", "geometry_fieldname": "geom", "geometry_type": "point", "srid": 4326, "vertical_crs": provider.CRS84h}
	tiler, err := NewTileProvider(dict.Dict{"name": "live_native_features", "uri": os.Getenv("PGURI"), "layers": []map[string]any{conf}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	p := tiler.(*Provider)
	defer p.pool.Close()
	var received []provider.Feature
	result, err := p.QueryFeatures(context.Background(), "items", provider.FeatureQuery{Limit: 1, Bounds3D: []provider.Extent3D{{14, -1, 7, 16, 1, 7}}, BoundsSRID: 4326, BoundsVerticalCRS: provider.CRS84h}, func(feature *provider.Feature) error { received = append(received, *feature); return nil })
	if err != nil || result.NumberReturned != 1 || len(received) != 1 {
		t.Fatal(result, received, err)
	}
	point, ok := received[0].Geometry.(geom.PointZ)
	if !ok || point != (geom.PointZ{15, 0, 7}) {
		t.Fatalf("native ISO Z lost: %#v", received[0].Geometry)
	}
}

func TestFeaturePostGISSnapshotMutationAndDDLLive(t *testing.T) {
	db, schema := featureLiveDatabase(t)
	qualified := pgQuoteIdent(schema) + `."snapshot_items"`
	if _, err := db.Exec(context.Background(), "CREATE TABLE "+qualified+" (id bigint PRIMARY KEY,geom text,name text)"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(context.Background(), "INSERT INTO "+qualified+" SELECT n,'POINT(0 0)','before' FROM generate_series(1,300) n"); err != nil {
		t.Fatal(err)
	}
	conf := map[string]any{"name": "items", "tablename": qualified, "id_fieldname": "id", "geometry_fieldname": "geom", "geometry_format": "wkt", "geometry_type": "point", "srid": 4326}
	tiler, err := NewTileProvider(dict.Dict{"name": "live_snapshot_features", "uri": os.Getenv("PGURI"), "layers": []map[string]any{conf}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	p := tiler.(*Provider)
	defer p.pool.Close()
	callbacks := 0
	result, err := p.QueryFeatures(context.Background(), "items", provider.FeatureQuery{Limit: 400}, func(feature *provider.Feature) error {
		callbacks++
		if callbacks == 1 {
			if _, err := db.Exec(context.Background(), "INSERT INTO "+qualified+" VALUES (301,'POINT(0 0)','after')"); err != nil {
				return err
			}
			if _, err := db.Exec(context.Background(), "UPDATE "+qualified+" SET name='after' WHERE id=300"); err != nil {
				return err
			}
			ddlCtx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
			defer cancel()
			_, ddlErr := db.Exec(ddlCtx, "ALTER TABLE "+qualified+" ADD COLUMN intrusion integer")
			if ddlErr == nil || ddlCtx.Err() == nil {
				return fmt.Errorf("source DDL was not blocked under the query snapshot")
			}
		}
		if feature.Tags["name"] != "before" {
			return fmt.Errorf("snapshot included later mutation")
		}
		return nil
	})
	if err != nil || result.NumberReturned != 300 || callbacks != 300 {
		t.Fatal(result, callbacks, err)
	}
	if _, err := db.Exec(context.Background(), "DROP TABLE "+qualified); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(context.Background(), "CREATE TABLE "+qualified+" (id bigint PRIMARY KEY,geom text,name text)"); err != nil {
		t.Fatal(err)
	}
	callbacks = 0
	_, err = p.QueryFeatures(context.Background(), "items", provider.FeatureQuery{Limit: 1}, func(*provider.Feature) error { callbacks++; return nil })
	var corrupt provider.FeatureDataError
	if !errors.As(err, &corrupt) || callbacks != 0 {
		t.Fatal("replacement relation admitted", callbacks, err)
	}
}
