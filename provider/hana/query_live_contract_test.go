package hana_test

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"os"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/SAP/go-hdb/driver"
	"github.com/alexeydott/geom"
	"github.com/alexeydott/tegola/dict"
	"github.com/alexeydott/tegola/provider"
	"github.com/alexeydott/tegola/provider/hana"
	"github.com/alexeydott/tegola/provider/internal/querytest"
	"github.com/alexeydott/tegola/provider/test/mosfixture"
)

// This opt-in test runs the real admitted raw and native feature profiles.
// A skip is not parity or HANA snapshot evidence.
func TestFeatureQueryLiveContract(t *testing.T) {
	if os.Getenv(TESTENV) != "yes" || GetConnectionURI() == "" {
		t.Skip("live HANA feature contract requires RUN_HANA_TESTS=yes and HANA_CONNECTION_STRING")
	}
	for _, format := range []string{"wkb", "wkt", "native"} {
		t.Run(format, func(t *testing.T) {
			factory := hanaFeatureFactory(format)
			options := querytest.ProfileOptions{Custom: querytest.Options{PublicTemporalProperties: &querytest.TemporalPropertyProfile{StartField: "start_time", EndField: "end_time", Storage: querytest.UnixNanoseconds}}}
			if format == "native" {
				options.Ordinary.NativeRingOrientationEquivalent = true
				options.Custom.NativeRingOrientationEquivalent = true
				profile := &querytest.NativeProfileOptions{Backend: "hana", Version: "2.00.088.00.1760424921", Cases: []querytest.NativeProfileCase{querytest.NativeMixedCollection, querytest.NativeInvalidChildNormalization}}
				options.Ordinary.NativeProfileOutcomes = profile
				options.Custom.NativeProfileOutcomes = profile
			}
			querytest.RunProfiles(t, factory, options)
			querytest.RunDimensionalProfiles(t, factory, options)
			querytest.RunExactTemporalProfiles(t, factory, options)
			querytest.RunNullableProfiles(t, factory, options)
		})
	}
}

func hanaFeatureFactory(requestedFormat string) querytest.Factory {
	return func(t *testing.T, fixture querytest.Fixture) querytest.Instance {
		format := requestedFormat
		for _, r := range fixture.Rows {
			if r.Metadata {
				format = "mos"
			}
			if r.MalformedGeometry == "wkb" {
				format = "wkb"
			}
		}

		db, err := hana.OpenDB(GetConnectionURI())
		if err != nil {
			t.Fatal("open live HANA fixture")
		}
		t.Cleanup(func() {
			if err := db.Close(); err != nil {
				t.Error("close live HANA fixture")
			}
		})
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if fixture.NativeProfileCase != 0 {
			if requestedFormat != "native" || len(fixture.Rows) != 1 {
				t.Fatal("native outcome requires exact native fixture")
			}
			wire, err := hanaNativeFixtureWKB(fixture.Rows[0].Feature.Geometry)
			if err != nil {
				t.Fatal(err)
			}
			var version string
			if err := db.QueryRowContext(ctx, "SELECT VERSION FROM SYS.M_DATABASE").Scan(&version); err != nil {
				t.Fatal(err)
			}
			evidence := &querytest.NativeProfileEvidence{Case: fixture.NativeProfileCase, Backend: "hana", Version: version, Operation: querytest.NativeGeometryConstruction, AttemptedWire: wire}
			switch fixture.NativeProfileCase {
			case querytest.NativeMixedCollection:
				var sink any
				err := db.QueryRowContext(ctx, "SELECT ST_GeomFromWKB(HEXTOBIN(?),4326) FROM DUMMY", hex.EncodeToString(wire)).Scan(&sink)
				var typed driver.Error
				if !errors.As(err, &typed) || typed.Code() != 669 || !strings.Contains(err.Error(), "1600408") {
					t.Fatalf("mixed native actual rejection: %v", err)
				}
				evidence.Kind = querytest.NativeProfileStorageRejected
				evidence.Error = err
				evidence.SQLState = "HY000"
				evidence.VendorCode = typed.Code()
				evidence.InternalCode = 1600408
			case querytest.NativeInvalidChildNormalization:
				var exported bytes.Buffer
				lob := new(driver.Lob).SetWriter(&exported)
				if err := db.QueryRowContext(ctx, "SELECT ST_GeomFromWKB(HEXTOBIN(?),4326).ST_AsBinary() FROM DUMMY", hex.EncodeToString(wire)).Scan(lob); err != nil {
					t.Fatal(err)
				}
				evidence.Kind = querytest.NativeProfileStorageNormalized
				evidence.ExportedWire = append([]byte(nil), exported.Bytes()...)
			default:
				t.Fatal("unknown native outcome case")
			}
			return querytest.Instance{NativeProfileOutcome: evidence}
		}
		table := featureFixtureTable(t, "TEGOLA_FEATURE")
		quoted := `"` + table + `"`
		geometryType := "VARBINARY(5000)"
		if format == "wkt" {
			geometryType = "NCLOB"
		}
		tableKind := "ROW"
		if format == "native" {
			geometryType = "ST_GEOMETRY(4326)"
			tableKind = "COLUMN"
		}
		ddl := `CREATE ` + tableKind + ` TABLE ` + quoted + ` ("id" BIGINT UNIQUE, "geom" ` + geometryType + `, "name" NVARCHAR(100), "value" BIGINT, "start_time" BIGINT, "end_time" BIGINT, "MINX" DOUBLE, "MAXX" DOUBLE, "MINY" DOUBLE, "MAXY" DOUBLE,"included" INTEGER)`
		if _, err := db.ExecContext(ctx, ddl); err != nil {
			t.Fatalf("create live HANA fixture format=%s: %v", format, err)
		}
		t.Cleanup(func() {
			dropCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			if _, err := db.ExecContext(dropCtx, "DROP TABLE "+quoted); err != nil {
				t.Error("drop live HANA fixture")
			}
		})
		storage := fixture.TemporalStorage
		if storage == 0 {
			storage = querytest.UnixNanoseconds
		}
		for _, row := range fixture.Rows {
			if row.MalformedGeometry == "native" {
				var sink any
				nativeErr := db.QueryRowContext(ctx, "SELECT ST_GeomFromWKB(HEXTOBIN(?),4326) FROM DUMMY", "FF").Scan(&sink)
				var typed driver.Error
				if !errors.As(nativeErr, &typed) || typed.Code() != 669 || !strings.Contains(nativeErr.Error(), "WKB") {
					t.Fatalf("expected actual native WKB parse669, got %v", nativeErr)
				}
				return querytest.Instance{StorageRejectedNativeCorruption: &querytest.NativeStorageEvidence{Backend: "hana", Operation: querytest.NativeGeometryConstruction, Scope: querytest.MalformedNativeWire, FeatureID: row.Feature.ID, Error: nativeErr, SQLState: "HY000", VendorCode: typed.Code()}}
			}
		}
		for _, row := range fixture.Rows {
			var id, geometry, start, end any
			if !row.MissingID {
				id = int64(row.Feature.ID)
			}
			source := row.Feature.Geometry
			if row.RawEmptyGeometry != nil {
				source = row.RawEmptyGeometry
			}
			if source != nil {
				geometry, err = querytest.EncodeFixtureWKB(source)
				if format == "native" {
					geometry, err = hanaNativeFixtureWKB(source)
				}
				if format == "wkt" {
					geometry = hanaFixtureWKT(source)
				}
				if format == "mos" {
					geometry = hanaFixtureMOS(source)
				}
				if err != nil {
					t.Fatal(err)
				}
			}
			if row.EmptyGeometry {
				body := binary.LittleEndian.AppendUint32([]byte{1}, 1)
				body = binary.LittleEndian.AppendUint64(body, math.Float64bits(math.NaN()))
				geometry = binary.LittleEndian.AppendUint64(body, math.Float64bits(math.NaN()))
				if format == "wkt" {
					geometry = "POINT EMPTY"
				}
			}
			if row.MalformedGeometry != "" {
				geometry = []byte{0xff}
				if format == "wkt" {
					geometry = "not geometry"
				}
			}
			if row.Metadata {
				geometry = mosfixture.SystemInfoBlob()
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
			if row.RawTemporal != nil {
				if row.RawTemporal.Start != nil {
					start = *row.RawTemporal.Start
				}
				if row.RawTemporal.End != nil {
					end = *row.RawTemporal.End
				}
			}
			geomParameter := "?"
			if format == "native" {
				geomParameter = "ST_GeomFromWKB(HEXTOBIN(?),4326)"
				if geometry != nil {
					geometry = hex.EncodeToString(geometry.([]byte))
				}
			}
			included := 1
			if row.ExcludedBySelection {
				included = 0
			}
			if _, err := db.ExecContext(ctx, "INSERT INTO "+quoted+` ("id","geom","name","value","start_time","end_time","MINX","MAXX","MINY","MAXY","included") VALUES (? ,`+geomParameter+`,?,?,?,?,0,0,0,0,?)`, id, geometry, row.Feature.Tags["name"], row.Feature.Tags["value"], start, end, included); err != nil {
				t.Fatalf("populate live HANA fixture format=%s id=%v: %v", format, id, err)
			}
		}
		publicSet := map[string]bool{}
		for _, row := range fixture.Rows {
			for key := range row.Feature.Tags {
				publicSet[key] = true
			}
		}
		public := []string{}
		for key := range publicSet {
			public = append(public, key)
		}
		sort.Strings(public)
		if fixture.PublicFields != nil {
			public = append([]string{}, fixture.PublicFields...)
		}
		fields := append([]string(nil), public...)
		if len(fields) == 0 {
			fields = []string{"id"}
		}
		layer := map[string]any{"name": "items", "tablename": quoted, "id_fieldname": "id", "geometry_fieldname": "geom", "geometry_format": format, "geometry_type": "point", "srid": 4326, "fields": fields, "temporal_start_field": "start_time", "temporal_end_field": "end_time", "temporal_storage": "unix_nanoseconds"}
		if fixture.Spatial.Dimension != 0 {
			layer["spatial_dimension"] = map[provider.CoordinateDimension]string{provider.DimensionXY: "xy", provider.DimensionXYZ: "xyz", provider.DimensionMixedXYXYZ: "mixed_xy_xyz"}[fixture.Spatial.Dimension]
			if fixture.Spatial.VerticalCRS != "" {
				layer["vertical_crs"] = fixture.Spatial.VerticalCRS
			}
		}
		units := map[querytest.TemporalPropertyStorage]string{querytest.UnixSeconds: "unix_seconds", querytest.UnixMilliseconds: "unix_milliseconds", querytest.UnixMicroseconds: "unix_microseconds", querytest.UnixNanoseconds: "unix_nanoseconds"}
		layer["temporal_storage"] = units[storage]
		if format == "native" {
			delete(layer, "geometry_format")
			if fixture.Spatial.Dimension == 0 {
				layer["spatial_dimension"] = "xy"
			}
		}
		if format == "mos" {
			layer["mos_precision"] = float64(7)
			layer["mos_units"] = "m"
		}
		if fixture.Profile == querytest.CustomSelection {
			projection := []string{`"id"`, `"geom"`}
			for _, name := range public {
				if name != "start_time" && name != "end_time" {
					projection = append(projection, `"`+name+`"`)
				}
			}
			projection = append(projection, `"start_time"`, `"end_time"`)
			layer["feature_sql"] = "SELECT " + strings.Join(projection, ",") + " FROM " + quoted + ` WHERE "included"=1`
		}
		if fixture.InvalidTemporalMapping {
			layer["temporal_end_field"] = "missing"
		}
		p, err := hana.CreateProvider(dict.Dict{hana.ConfigKeyName: "feature_contract", hana.ConfigKeyURI: GetConnectionURI(), "layers": []map[string]any{layer}}, nil, hana.ProviderType)
		if err != nil {
			return querytest.Instance{SetupError: err}
		}
		t.Cleanup(p.Close)
		return querytest.Instance{Querier: p, Layer: "items", CountMode: querytest.OptionalCount}
	}
}

func hanaFixtureWKT(g geom.Geometry) string {
	if collection, ok := g.(geom.Collection); ok {
		if len(collection) == 0 {
			return "GEOMETRYCOLLECTION EMPTY"
		}
		v := []string{}
		for _, child := range collection {
			v = append(v, hanaFixtureWKT(child))
		}
		return "GEOMETRYCOLLECTION(" + strings.Join(v, ",") + ")"
	}
	typ := reflect.TypeOf(g).Name()
	z := strings.HasSuffix(typ, "Z")
	name := strings.ToUpper(strings.TrimSuffix(typ, "Z"))
	v := reflect.ValueOf(g)
	if v.Kind() == reflect.Slice && v.Len() == 0 {
		return name + map[bool]string{true: " Z", false: ""}[z] + " EMPTY"
	}
	var body func(reflect.Value, int) string
	body = func(v reflect.Value, level int) string {
		if v.Kind() == reflect.Array && v.Type().Elem().Kind() == reflect.Float64 {
			a := []string{}
			for i := 0; i < v.Len(); i++ {
				a = append(a, strconv.FormatFloat(v.Index(i).Float(), 'g', -1, 64))
			}
			return strings.Join(a, " ")
		}
		a := []string{}
		for i := 0; i < v.Len(); i++ {
			a = append(a, body(v.Index(i), level+1))
		}
		if (name == "POLYGON" && level == 1 || name == "MULTIPOLYGON" && level == 2) && v.Len() > 0 && !reflect.DeepEqual(v.Index(0).Interface(), v.Index(v.Len()-1).Interface()) {
			a = append(a, body(v.Index(0), level+1))
		}
		if len(a) == 0 {
			return "EMPTY"
		}
		return "(" + strings.Join(a, ",") + ")"
	}
	text := body(v, 0)
	if name == "POINT" {
		text = "(" + text + ")"
	}
	return name + map[bool]string{true: " Z", false: ""}[z] + text
}

func hanaFixtureMOS(g geom.Geometry) []byte {
	typ := byte(0)
	var parts [][][2]float64
	switch v := g.(type) {
	case geom.Point:
		typ = 2
		parts = [][][2]float64{{v}}
	case geom.MultiPoint:
		typ = 2
		parts = [][][2]float64{v}
	case geom.LineString:
		typ = 1
		parts = [][][2]float64{v}
	case geom.MultiLineString:
		typ = 1
		parts = v
	case geom.Polygon:
		parts = v
	case geom.MultiPolygon:
		for _, p := range v {
			parts = append(parts, p...)
		}
	default:
		panic(fmt.Sprintf("MOS fixture unsupported %T", g))
	}
	n := 0
	for _, p := range parts {
		n += len(p)
	}
	b := []byte{typ, 0, 0, 0}
	b = binary.LittleEndian.AppendUint16(b, uint16(len(parts)))
	b = binary.LittleEndian.AppendUint32(b, uint32(n))
	for _, p := range parts {
		b = binary.LittleEndian.AppendUint32(b, uint32(len(p)))
	}
	for _, p := range parts {
		for _, xy := range p {
			for _, x := range xy {
				scaled := x * 1e7
				if math.Trunc(scaled) != scaled || scaled < math.MinInt32 || scaled > math.MaxInt32 {
					panic("MOS fixture nonrepresentable")
				}
				b = binary.LittleEndian.AppendUint32(b, uint32(int32(scaled)))
			}
		}
	}
	return b
}
