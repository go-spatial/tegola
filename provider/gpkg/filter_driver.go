//go:build cgo

package gpkg

import (
	"database/sql"
	"fmt"
	"math"
	"unicode/utf8"

	"github.com/alexeydott/geom"
	"github.com/alexeydott/geom/encoding/wkb"
	"github.com/mattn/go-sqlite3"
)

const featureSQLiteDriver = "tegola-gpkg-feature-sqlite3"
const featureUTF8Function = "tegola_feature_valid_utf8"
const maxFilterSourceTextBytes = 1024 * 1024

func init() {
	sql.Register(featureSQLiteDriver, &sqlite3.SQLiteDriver{ConnectHook: func(connection *sqlite3.SQLiteConn) error {
		if err := connection.RegisterFunc(featureUTF8Function, validFilterUTF8, true); err != nil {
			return err
		}
		// Standard GeoPackage RTree triggers call these functions even when
		// SQLite prepares an attribute-only UPDATE. Keep those triggers intact.
		if err := connection.RegisterFunc("ST_IsEmpty", gpkgSQLIsEmpty, true); err != nil {
			return err
		}
		for index, name := range []string{"ST_MinX", "ST_MaxX", "ST_MinY", "ST_MaxY"} {
			if err := connection.RegisterFunc(name, func(value any) (any, error) {
				data, ok := value.([]byte)
				if !ok {
					return nil, fmt.Errorf("GeoPackage geometry must be a blob")
				}
				if data == nil {
					return nil, nil
				}
				geometry, err := decodeSQLGeoPackage(data)
				if err != nil || geometry == nil {
					return nil, err
				}
				bounds, err := geometryBounds(geometry)
				if err != nil {
					return nil, err
				}
				return bounds[index], nil
			}, true); err != nil {
				return err
			}
		}
		return nil
	}})
}

func gpkgSQLIsEmpty(value any) (any, error) {
	data, ok := value.([]byte)
	if !ok {
		return nil, fmt.Errorf("GeoPackage geometry must be a blob")
	}
	if data == nil {
		return nil, nil
	}
	geometry, err := decodeSQLGeoPackage(data)
	if err != nil {
		return nil, err
	}
	if geometry == nil {
		return true, nil
	}
	points, err := geom.GetCoordinates(geometry)
	return len(points) == 0, err
}

// RTree callbacks must not accept extensions or non-finite coordinates and
// silently manufacture an index entry. Empty binary geometry is represented
// by the GeoPackage header flag, including an empty Point's NaN WKB payload.
func decodeSQLGeoPackage(data []byte) (geom.Geometry, error) {
	header, err := NewBinaryHeader(data)
	if err != nil {
		return nil, err
	}
	if !header.IsStandardGeometry() {
		return nil, fmt.Errorf("extended geopackage geometry is unsupported")
	}
	if header.IsGeometryEmpty() {
		return nil, nil
	}
	geometry, err := wkb.DecodeBytes(data[header.Size():])
	if err != nil {
		return nil, err
	}
	points, err := geom.GetCoordinates(geometry)
	if err != nil {
		return nil, err
	}
	for _, point := range points {
		if math.IsNaN(point[0]) || math.IsNaN(point[1]) || math.IsInf(point[0], 0) || math.IsInf(point[1], 0) {
			return nil, fmt.Errorf("non-finite geopackage coordinate")
		}
	}
	return geometry, nil
}

// SQL domain guards establish TEXT and a byte cap before this callback. The
// vendored generic converter copies at most that many UTF-8 bytes with GoStringN.
// UTF-16 databases are not admitted to the optional filter profile.
func validFilterUTF8(value any) bool {
	text, ok := value.(string)
	return ok && len(text) <= maxFilterSourceTextBytes && utf8.ValidString(text)
}
