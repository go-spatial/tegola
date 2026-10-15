package mysql

import (
	"context"
	"database/sql"
	"encoding/binary"
	"errors"
	"math"
	"slices"
	"strings"
	"testing"

	"github.com/alexeydott/tegola/dict"
	"github.com/alexeydott/tegola/provider"
	"github.com/alexeydott/tegola/provider/internal/querytest"
	mysqlDriver "github.com/go-sql-driver/mysql"
)

func TestFeatureLiveCommonContract(t *testing.T) {
	for _, flavor := range []string{GeometryFormatMySQL, GeometryFormatMariaDB} {
		t.Run(flavor, func(t *testing.T) {
			// Gate once before creating the common matrix: unavailable servers are
			// reported as live skips, never as reference-adapter parity.
			_, _ = featureLiveDatabase(t, flavor)
			factory := func(t *testing.T, fixture querytest.Fixture) querytest.Instance {
				return featureLiveContractInstance(t, flavor, fixture)
			}
			options := querytest.ProfileOptions{Custom: querytest.Options{PublicTemporalProperties: &querytest.TemporalPropertyProfile{
				StartField: "start_time", EndField: "end_time", Storage: querytest.UnixSeconds}}}
			querytest.RunProfiles(t, factory, options)
			querytest.RunDimensionalProfiles(t, factory, options)
			querytest.RunExactTemporalProfiles(t, factory, options)
			querytest.RunNullableProfiles(t, factory, options)
			querytest.RunFilterProfiles(t, factory, options)
		})
	}
}

func featureLiveContractInstance(t *testing.T, flavor string, fixture querytest.Fixture) querytest.Instance {
	t.Helper()
	custom := fixture.Profile == querytest.CustomSelection
	storage := fixture.TemporalStorage
	if storage == 0 {
		storage = querytest.UnixNanoseconds
		if custom {
			storage = querytest.UnixSeconds
		}
	}
	db, connection := featureLiveDatabase(t, flavor)
	format := GeometryFormatWKB
	for _, row := range fixture.Rows {
		if row.Metadata {
			format = GeometryFormatMOS
		}
		if row.MalformedGeometry == "native" {
			return featureLiveNativeStorageEvidence(t, db, flavor, row.Feature.ID)
		}
	}
	columns := "id BIGINT UNSIGNED UNIQUE, geom LONGBLOB, name TEXT, value BIGINT, start_time BIGINT, end_time BIGINT, selection_flag INT NOT NULL"
	if custom {
		columns = "source_id BIGINT UNSIGNED UNIQUE, source_geom LONGBLOB, source_name TEXT, source_value BIGINT, source_start BIGINT, source_end BIGINT, selection_flag INT NOT NULL"
		if format == GeometryFormatMOS {
			columns += ", MINX DOUBLE, MAXX DOUBLE, MINY DOUBLE, MAXY DOUBLE"
		}
	}
	if len(fixture.FilterFields) != 0 {
		if custom {
			columns += ", source_n BIGINT, source_s TEXT CHARACTER SET utf8mb4 COLLATE utf8mb4_bin, source_b BIT(1)"
		} else {
			columns += ", n BIGINT, s TEXT CHARACTER SET utf8mb4 COLLATE utf8mb4_bin, b BIT(1)"
		}
	}
	table := featureLiveTable(t, db, columns)
	layer := map[string]any{"name": "items", "tablename": table, "id_fieldname": "id", "geometry_fieldname": "geom",
		"geometry_format": format, "geometry_type": "point", "srid": 4326, "fields": []string{"name", "value"},
		"temporal_start_field": "start_time", "temporal_end_field": "end_time", "temporal_storage": "unix_nanoseconds"}
	if fixture.PublicFields != nil {
		fields := slices.Clone(fixture.PublicFields)
		if len(fields) == 0 {
			fields = []string{"id"} // Identity is private; an explicit empty public set.
		}
		layer["fields"] = fields
	}
	if custom {
		projection := featureLiveContractProjection(fixture.PublicFields, len(fixture.FilterFields) != 0)
		layer["feature_sql"] = "SELECT " + strings.Join(projection, ",") + " FROM " + featureQuoteIdentifier(table) + " s WHERE s.selection_flag=1"
		// Legacy tile registration targets the physical table independently.
		// Raw custom output names are configured via a separate trusted tile
		// SELECT, so tile metadata probes never address nonexistent columns.
		delete(layer, "tablename")
		layer["sql"] = "SELECT source_id AS id,source_geom AS geom,source_name AS name,source_value AS value FROM " + featureQuoteIdentifier(table)
		if format == GeometryFormatMOS {
			layer["sql"] = "SELECT source_id AS id,source_geom AS geom,source_name AS name,source_value AS value,MINX,MAXX,MINY,MAXY FROM " + featureQuoteIdentifier(table) + " WHERE !BBOX!"
		}
		layer["temporal_storage"] = "unix_seconds"
	}
	if fixture.Spatial.Dimension != provider.DimensionUnknown {
		if format != GeometryFormatWKB {
			t.Fatal("dimensional fixture must use explicit raw WKB, not native or MOS storage")
		}
		switch fixture.Spatial.Dimension {
		case provider.DimensionXY:
			layer["spatial_dimension"] = "xy"
		case provider.DimensionXYZ:
			layer["spatial_dimension"] = "xyz"
		case provider.DimensionMixedXYXYZ:
			layer["spatial_dimension"] = "mixed_xy_xyz"
		default:
			t.Fatal("unsupported fixture source dimension")
		}
		if fixture.Spatial.VerticalCRS != "" {
			layer["vertical_crs"] = fixture.Spatial.VerticalCRS
		}
	}
	switch storage {
	case querytest.UnixSeconds:
		layer["temporal_storage"] = "unix_seconds"
	case querytest.UnixMilliseconds:
		layer["temporal_storage"] = "unix_milliseconds"
	case querytest.UnixMicroseconds:
		layer["temporal_storage"] = "unix_microseconds"
	case querytest.UnixNanoseconds:
		layer["temporal_storage"] = "unix_nanoseconds"
	default:
		t.Fatal("unsupported fixture temporal storage")
	}
	if fixture.InvalidTemporalMapping {
		// A syntactically partial mapping must fail in the real constructor.
		delete(layer, "temporal_end_field")
	}
	config := dict.Dict{}
	for key, value := range connection {
		config[key] = value
	}
	config["layers"] = []map[string]any{layer}
	tiler, err := NewTileProvider(config, nil)
	if err != nil {
		return querytest.Instance{SetupError: err}
	}
	p := tiler.(*Provider)
	t.Cleanup(func() {
		if err := p.Close(); err != nil {
			t.Error(err)
		}
	})
	if err := p.layers["items"].FeatureQuerySupported(); err != nil {
		t.Fatalf("real contract profile requires eligible source and catalog visibility: %v", err)
	}
	for _, row := range fixture.Rows {
		var id, geometry, start, end any
		if !row.MissingID {
			id = row.Feature.ID
		}
		sourceGeometry := row.Feature.Geometry
		if row.RawEmptyGeometry != nil {
			sourceGeometry = row.RawEmptyGeometry
		}
		if sourceGeometry != nil {
			geometry, err = querytest.EncodeFixtureWKB(sourceGeometry)
			if err != nil {
				t.Fatal(err)
			}
		}
		if row.EmptyGeometry {
			body := binary.LittleEndian.AppendUint32([]byte{1}, 1)
			body = binary.LittleEndian.AppendUint64(body, math.Float64bits(math.NaN()))
			geometry = binary.LittleEndian.AppendUint64(body, math.Float64bits(math.NaN()))
		}
		if row.MalformedGeometry == "wkb" {
			geometry = []byte{0xff}
		}
		if row.Metadata {
			body := make([]byte, 64)
			copy(body, []byte{5, 'V', 'e', 'r', ' ', '1'})
			geometry = body
		}
		if row.RawTemporal != nil {
			if fixture.TemporalStorage == 0 {
				t.Fatal("raw temporal source requires explicit storage unit")
			}
			if row.RawTemporal.Start != nil {
				start = *row.RawTemporal.Start
			}
			if row.RawTemporal.End != nil {
				end = *row.RawTemporal.End
			}
		} else {
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
		}
		selectionFlag := 1
		if row.ExcludedBySelection {
			selectionFlag = 0
		}
		insertColumns := "id,geom,name,value,start_time,end_time,selection_flag"
		if custom {
			insertColumns = "source_id,source_geom,source_name,source_value,source_start,source_end,selection_flag"
		}
		values := []any{id, geometry, row.Feature.Tags["name"], row.Feature.Tags["value"], start, end, selectionFlag}
		placeholders := "?,?,?,?,?,?,?"
		if len(fixture.FilterFields) != 0 {
			if custom {
				insertColumns += ",source_n,source_s,source_b"
			} else {
				insertColumns += ",n,s,b"
			}
			values = append(values, row.Feature.Tags["n"], row.Feature.Tags["s"], row.Feature.Tags["b"])
			placeholders += ",?,?,?"
		}
		if _, err := db.Exec("INSERT INTO "+featureQuoteIdentifier(table)+" ("+insertColumns+") VALUES ("+placeholders+")", values...); err != nil {
			t.Fatal(err)
		}
	}
	return querytest.Instance{Querier: p, Layer: "items", CountMode: querytest.OptionalCount, QueryableLayer: p.layers["items"]}
}

func featureLiveContractProjection(publicFields []string, filtered bool) []string {
	projection := []string{"s.source_id AS id", "s.source_geom AS geom"}
	for _, field := range []struct{ source, output string }{{"source_name", "name"}, {"source_value", "value"}} {
		if publicFields == nil || slices.Contains(publicFields, field.output) {
			projection = append(projection, "s."+field.source+" AS "+field.output)
		}
	}
	// Temporal mapping requires these direct outputs even in an empty fixture
	// whose declared public property union has no rows/keys. No oracle rows or
	// property maps are altered; nonempty custom fixtures declare both aliases.
	projection = append(projection, "s.source_start AS start_time", "s.source_end AS end_time")
	if filtered {
		projection = append(projection, "s.source_n AS n", "s.source_s AS s", "s.source_b AS b")
	}
	return projection
}

func featureLiveNativeStorageEvidence(t *testing.T, db *sql.DB, flavor string, id uint64) querytest.Instance {
	t.Helper()
	operation := querytest.NativeGeometryConstruction
	var geometry []byte
	err := db.QueryRowContext(context.Background(), "SELECT ST_GeomFromWKB(?)", []byte{0xff}).Scan(&geometry)
	expected := uint16(3037) // MySQL ER_GIS_INVALID_DATA.
	expectedState := "22023"
	if flavor == GeometryFormatMariaDB {
		// MariaDB construction can return SQL NULL with a warning. The table
		// insertion path must instead reject the literal malformed native wire.
		operation = querytest.NativeGeometryInsertion
		table := featureLiveTable(t, db, "id BIGINT PRIMARY KEY, geom GEOMETRY")
		_, err = db.ExecContext(context.Background(), "INSERT INTO "+featureQuoteIdentifier(table)+" VALUES (?,?)", id, []byte{0xff})
		expected = 1416 // ER_CANT_CREATE_GEOMETRY_OBJECT.
		expectedState = "22003"
	}
	var driverError *mysqlDriver.MySQLError
	if !errors.As(err, &driverError) || driverError.Number != expected || string(driverError.SQLState[:]) != expectedState {
		t.Fatalf("malformed native wire requires the expected GIS parse error %d, got %v", expected, err)
	}
	return querytest.Instance{StorageRejectedNativeCorruption: &querytest.NativeStorageEvidence{
		Backend: flavor, Operation: operation, Scope: querytest.MalformedNativeWire, FeatureID: id,
		Error: err, SQLState: string(driverError.SQLState[:]), VendorCode: int(driverError.Number)}}
}
