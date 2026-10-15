package hana_test

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/SAP/go-hdb/driver"
	"github.com/alexeydott/geom"
	"github.com/alexeydott/tegola/provider/geometrycodec"
	"github.com/alexeydott/tegola/provider/hana"
	"github.com/alexeydott/tegola/provider/internal/querytest"
)

func TestFeatureLiveNativePolygonExportEvidence(t *testing.T) {
	if os.Getenv(TESTENV) != "yes" || GetConnectionURI() == "" {
		t.Skip("live HANA")
	}
	db, err := hana.OpenDB(GetConnectionURI())
	if err != nil {
		t.Fatal(err)
	}
	defer featureFixtureClose(t, db)
	source := geom.PolygonZ{
		{{0, 0, 0}, {10, 0, 10}, {10, 10, 10}, {0, 10, 0}, {0, 0, 0}},
		{{4, 4, 4}, {6, 4, 6}, {6, 6, 6}, {4, 6, 4}, {4, 4, 4}},
	}
	wire, err := querytest.EncodeFixtureWKB(source)
	if err != nil {
		t.Fatal(err)
	}
	var exported bytes.Buffer
	lob := new(driver.Lob).SetWriter(&exported)
	if err := db.QueryRow("SELECT ST_GeomFromWKB(HEXTOBIN(?),4326).ST_AsBinary() FROM DUMMY", hex.EncodeToString(wire)).Scan(lob); err != nil {
		t.Fatal(err)
	}
	actual, err := geometrycodec.DecodeRawWKB(exported.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("declared literal: %#v", source)
	t.Logf("native export: %#v", actual)
	t.Logf("attempted ISO wire: %s", hex.EncodeToString(wire))
	t.Logf("exported ISO wire: %s", hex.EncodeToString(exported.Bytes()))
}

func TestFeatureLiveNativeInvalidChildExportEvidence(t *testing.T) {
	if os.Getenv(TESTENV) != "yes" || GetConnectionURI() == "" {
		t.Skip("live HANA")
	}
	db, err := hana.OpenDB(GetConnectionURI())
	if err != nil {
		t.Fatal(err)
	}
	defer featureFixtureClose(t, db)
	source := geom.Collection{geom.PointZ{0, 0, 0}, geom.PointZ{1, math.NaN(), 2}}
	wire, err := hanaNativeFixtureWKB(source)
	if err != nil {
		t.Fatal(err)
	}
	var exported bytes.Buffer
	lob := new(driver.Lob).SetWriter(&exported)
	if err := db.QueryRow("SELECT ST_GeomFromWKB(HEXTOBIN(?),4326).ST_AsBinary() FROM DUMMY", hex.EncodeToString(wire)).Scan(lob); err != nil {
		t.Fatal(err)
	}
	actual, err := geometrycodec.DecodeRawWKB(exported.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("declared literal: %#v", source)
	t.Logf("native export: %#v", actual)
	t.Logf("attempted ISO wire: %s", hex.EncodeToString(wire))
	t.Logf("exported ISO wire: %s", hex.EncodeToString(exported.Bytes()))
}

func TestFeatureLiveNativeMixedStorageEvidence(t *testing.T) {
	if os.Getenv(TESTENV) != "yes" || GetConnectionURI() == "" {
		t.Skip("live HANA")
	}
	db, err := hana.OpenDB(GetConnectionURI())
	if err != nil {
		t.Fatal(err)
	}
	defer featureFixtureClose(t, db)
	var version string
	if err := db.QueryRow("SELECT VERSION FROM SYS.M_DATABASE").Scan(&version); err != nil {
		t.Fatal(err)
	}
	t.Logf("server version: %s", version)
	source := geom.Collection{geom.Point{0, 0}, geom.PointZ{5, 5, 100}}
	wire, err := hanaNativeFixtureWKB(source)
	if err != nil {
		t.Fatal(err)
	}
	var sink any
	err = db.QueryRow("SELECT ST_GeomFromWKB(HEXTOBIN(?),4326) FROM DUMMY", hex.EncodeToString(wire)).Scan(&sink)
	var typed driver.Error
	if !errors.As(err, &typed) || typed.Code() != 669 {
		t.Fatalf("mixed constructor rejection: %v", err)
	}
	t.Logf("operation: SELECT ST_GeomFromWKB(HEXTOBIN(?),4326) FROM DUMMY")
	t.Logf("attempted ISO wire: %s", hex.EncodeToString(wire))
	t.Logf("constructor error SQLSTATE=HY000 vendor=%d: %v", typed.Code(), err)
}

func TestFeatureLiveNativeCanonicalCoordinateProof(t *testing.T) {
	if os.Getenv(TESTENV) != "yes" || GetConnectionURI() == "" {
		t.Skip("live HANA")
	}
	db, err := hana.OpenDB(GetConnectionURI())
	if err != nil {
		t.Fatal(err)
	}
	defer featureFixtureClose(t, db)
	for _, srid := range []uint64{4326, 1000004326} {
		for _, source := range []geom.Geometry{geom.Point{151.25, -33.75}, geom.PointZ{151.25, -33.75, 123.125}} {
			wire, err := querytest.EncodeFixtureWKB(source)
			if err != nil {
				t.Fatal(err)
			}
			var exported bytes.Buffer
			lob := new(driver.Lob).SetWriter(&exported)
			var actualSRID uint64
			if err := db.QueryRow(fmt.Sprintf("SELECT g.ST_AsBinary(),g.ST_SRID() FROM (SELECT ST_GeomFromWKB(HEXTOBIN(?),%d) AS g FROM DUMMY)", srid), hex.EncodeToString(wire)).Scan(lob, &actualSRID); err != nil {
				t.Fatal(err)
			}
			actual, err := geometrycodec.DecodeRawWKB(exported.Bytes())
			if err != nil {
				t.Fatal(err)
			}
			if actualSRID != srid || !reflect.DeepEqual(actual, source) {
				t.Fatalf("stored axes/height changed srid=%d actualSRID=%d geometry=%#v", srid, actualSRID, actual)
			}
		}
	}
}

func TestFeatureLiveNativeTypedSourceSRIDEnforcement(t *testing.T) {
	if os.Getenv(TESTENV) != "yes" || GetConnectionURI() == "" {
		t.Skip("live HANA")
	}
	db, err := hana.OpenDB(GetConnectionURI())
	if err != nil {
		t.Fatal(err)
	}
	defer featureFixtureClose(t, db)
	for _, srid := range []uint64{4326, 1000004326} {
		for _, kind := range []string{"ST_POINT", "ST_GEOMETRY"} {
			t.Run(fmt.Sprintf("%s(%d)", kind, srid), func(t *testing.T) {
				table := featureFixtureTable(t, "TEGOLA_SRID_PROOF")
				q := `"` + table + `"`
				if _, err := db.Exec(`CREATE COLUMN TABLE ` + q + ` ("id" BIGINT PRIMARY KEY,"geom" ` + fmt.Sprintf("%s(%d))", kind, srid)); err != nil {
					t.Fatal(err)
				}
				defer func() {
					if _, err := db.Exec("DROP TABLE " + q); err != nil {
						t.Error(err)
					}
				}()
				_, err := db.Exec(`INSERT INTO ` + q + ` VALUES(1,ST_GeomFromText('POINT(1 2)',3857))`)
				var typed driver.Error
				if !errors.As(err, &typed) || typed.Code() != 669 || !strings.Contains(err.Error(), "1620501") {
					t.Fatalf("typed source accepted wrong stored SRID: %v", err)
				}
				t.Logf("typed=%s(%d) wrong storedSRID3857 enforcement vendor=%d: %v", kind, srid, typed.Code(), err)
				var count int
				if err := db.QueryRow("SELECT COUNT(*) FROM " + q).Scan(&count); err != nil || count != 0 {
					t.Fatalf("wrong SRID row stored count%d: %v", count, err)
				}
			})
		}
	}
}
