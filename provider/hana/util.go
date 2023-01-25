package hana

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/hex"
	"fmt"
	"math"
	"math/big"
	"regexp"
	"strconv"
	"strings"

	"github.com/SAP/go-hdb/driver"
	"github.com/go-spatial/geom"
	"github.com/go-spatial/tegola"
	"github.com/go-spatial/tegola/basic"
	"github.com/go-spatial/tegola/internal/env"
	"github.com/go-spatial/tegola/provider"
)

const (
	bboxToken             = "!BBOX!"
	zoomToken             = "!ZOOM!"
	xToken                = "!X!"
	yToken                = "!Y!"
	zToken                = "!Z!"
	scaleDenominatorToken = "!SCALE_DENOMINATOR!"
	pixelWidthToken       = "!PIXEL_WIDTH!"
	pixelHeightToken      = "!PIXEL_HEIGHT!"
	idFieldToken          = "!ID_FIELD!"
	geomFieldToken        = "!GEOM_FIELD!"
	geomTypeToken         = "!GEOM_TYPE!"
)

// isSelectQuery is a regexp to check if a query starts with `SELECT`,
// case-insensitive and ignoring any preceeding whitespace and SQL comments.
var isSelectQueryRe = regexp.MustCompile(`(?i)^((\s*)(--.*\n)?)*select`)

func isSelectQuery(sql string) bool {
	return isSelectQueryRe.MatchString(sql)
}

func quoteIdentifier(name string) string {
	if strings.Index(name, `"`) == 0 {
		return name
	}
	return fmt.Sprintf(`"%v"`, name)
}

func quoteTableName(name string) string {
	if strings.Contains(name, " ") {
		return name
	}

	strs := strings.Split(name, ".")
	nstrs := len(strs)
	if nstrs == 1 {
		return quoteIdentifier(strs[0])
	}

	ret := ""
	for i, s := range strs {
		ret = ret + quoteIdentifier(s)
		if i != nstrs-1 {
			ret = ret + "."
		}
	}

	return ret
}

func hasSrsPlanarEquivalent(pool *connectionPoolCollector, srid uint64) bool {
	var numSRIDs int = 0
	sql := "SELECT COUNT(*) FROM SYS.ST_SPATIAL_REFERENCE_SYSTEMS WHERE SRS_ID = ?"
	_ = pool.QueryRow(sql, toPlanarEquivalenSrid(srid)).Scan(&numSRIDs)
	return numSRIDs > 0
}

func isSrsRoundEarth(pool *connectionPoolCollector, srid uint64) bool {
	if srid == tegola.WGS84 {
		return true
	}

	sql := "SELECT TO_BOOLEAN(ROUND_EARTH) FROM SYS.ST_SPATIAL_REFERENCE_SYSTEMS WHERE SRS_ID = $1"
	var ret bool = false
	_ = pool.QueryRow(sql, srid).Scan(&ret)
	return ret
}

func genGeomField(name string, providerType string) string {
	return fmt.Sprintf(`%v.ST_AsBinary()  AS %[1]v`, quoteIdentifier(name))
}

func getLayerSQL(tblname string) string {
	quotedTblName := quoteTableName(tblname)
	return fmt.Sprintf(`SELECT * FROM %[1]v LIMIT 0;`, quotedTblName)
}

func getLayerRows(pool *connectionPoolCollector, sql string, extent *geom.Extent, srid uint64, withBBox bool) (*sql.Rows, error) {
	ctx := context.Background()
	if withBBox {
		rows, err := pool.QueryContextWithBBox(ctx, sql, extent, srid, false)
		if err := ctxErr(ctx, err); err != nil {
			return nil, err
		}
		return rows, nil
	} else {
		rows, err := pool.QueryContext(ctx, sql)
		if err := ctxErr(ctx, err); err != nil {
			return nil, err
		}
		return rows, nil
	}
}

func getLayerFields(pool *connectionPoolCollector, l *Layer, sql string) ([]FieldDescription, error) {
	withBBox := strings.Contains(sql, bboxToken)

	//	if a subquery is set in the 'sql' config the subquery is set to the layer's
	//	'tablename' param. because of this case normal SQL token replacement needs to be
	//	applied to tablename SQL generation
	tile := provider.NewTile(18, 0, 0, 64, tegola.WebMercator)
	sql, err := replaceTokens(2, sql, l.IDFieldName(), l.GeomFieldName(), l.GeomType(), l.SRID(), tile, false)
	if err != nil {
		return nil, err
	}

	extent, _ := getTileExtent(tile, false)
	rows, err := getLayerRows(pool, sql, extent, l.SRID(), withBBox)
	if err != nil {
		return nil, err
	}

	columns, err := rows.ColumnTypes()
	if err != nil {
		return nil, err
	}

	return getFieldDescriptions(l.Name(), l.GeomFieldName(), l.IDFieldName(), columns, true)
}

func getFieldNames(fields []FieldDescription) []string {
	var fieldNames []string
	for i := range fields {
		fieldNames = append(fieldNames, fields[i].name)
	}
	return fieldNames
}

func getTableFieldNames(pool *connectionPoolCollector, l *Layer, tblName string) ([]string, error) {
	fields, err := getLayerFields(pool, l, getLayerSQL(tblName))
	if err != nil {
		return nil, err
	}
	if len(fields) == 0 {
		return nil, fmt.Errorf("no fields were returned for table %v", tblName)
	}

	return getFieldNames(fields), nil
}

func genSQL(l *Layer, tblName string, fieldNames []string, buffer bool, providerType string) (sql string, err error) {
	fgeom := -1
	fid := -1

	for i, f := range fieldNames {
		if f == l.idField {
			fid = i
		} else if f == l.geomField {
			fgeom = i
		}
		fieldNames[i] = quoteIdentifier(fieldNames[i])
	}

	if fgeom == -1 {
		fieldNames = append(fieldNames, genGeomField(l.geomField, providerType))
	} else {
		fieldNames[fgeom] = genGeomField(l.geomField, providerType)
	}

	if fid == -1 && l.idField != "" {
		fieldNames = append(fieldNames, quoteIdentifier(l.idField))
	}

	stdSQL := `SELECT %[1]v FROM %[2]v WHERE ` + bboxToken

	return fmt.Sprintf(stdSQL, strings.Join(fieldNames, ", "), quoteTableName(tblName)), nil
}

func genMVTSQL(l *Layer, fields []string, buffer uint, clipGeometry bool) (sql string, err error) {
	var flds []string
	for i := range fields {
		if l.GeomFieldName() != fields[i] {
			flds = append(flds, quoteIdentifier(fields[i]))
		}
	}

	geomFieldName := quoteIdentifier(l.GeomFieldName())

	// ref: https://help.sap.com/docs/HANA_CLOUD_DATABASE/bc9e455fe75541b8a248b4c09b086cf5/8cd683c4bb664fd8a71fc3f19ffa7e42.html
	// BLOB  ST_AsMVT(expression_list Expression List, layer_name NCLOB, extent INT, geom_name NCLOB, feature_id_name NCLOB)

	var clip string = "TRUE"
	if !clipGeometry {
		clip = "FALSE"
	}

	if len(flds) == 0 {
		sql = fmt.Sprintf(`SELECT ST_AsMVT(%v.ST_AsMVTGeom(bounds => NEW ST_LINESTRING($4, $3), buffer => %v, clipgeom => %v) AS %v, layer_name => '%v', geom_name => '%v') FROM (%v)`, geomFieldName, buffer, clip, geomFieldName, l.Name(), l.GeomFieldName(), l.sql)
	} else {
		if l.IDFieldName() != "" {
			sql = fmt.Sprintf(`SELECT ST_AsMVT(%v, %v.ST_AsMVTGeom(bounds => NEW ST_LINESTRING($4, $3), buffer => %v, clipgeom => %v) AS %v, layer_name => '%v', geom_name => '%v', feature_id_name => '%v') FROM (%v)`, strings.Join(flds, ","), geomFieldName, buffer, clip, geomFieldName, l.Name(), l.GeomFieldName(), l.IDFieldName(), l.sql)
		} else {
			sql = fmt.Sprintf(`SELECT ST_AsMVT(%v, %v.ST_AsMVTGeom(bounds => NEW ST_LINESTRING($4, $3), buffer => %v, clipgeom => %v) AS %v, layer_name => '%v', geom_name => '%v') FROM (%v)`, strings.Join(flds, ","), geomFieldName, buffer, clip, geomFieldName, l.Name(), l.GeomFieldName(), l.sql)
		}
	}
	return sql, nil
}

const (
	PLANAR_SRID_OFFSET = 1000000000
)

func isPlanarEquivalentSrid(srid uint64) bool {
	return srid >= PLANAR_SRID_OFFSET
}

func toPlanarEquivalenSrid(srid uint64) uint64 {
	return PLANAR_SRID_OFFSET + srid
}

func fromWebMercator(srid uint64, geometry geom.Geometry) (geom.Geometry, error) {
	if isPlanarEquivalentSrid(srid) {
		return basic.FromWebMercator(srid-PLANAR_SRID_OFFSET, geometry)
	}

	return basic.FromWebMercator(srid, geometry)
}

func getBBoxCoordinates(extent *geom.Extent, srid uint64) (geom.Point, geom.Point, error) {
	// TODO: it's currently assumed the tile will always be in WebMercator. Need to support different projections
	minGeo, err := fromWebMercator(srid, geom.Point{extent.MinX(), extent.MinY()})
	if err != nil {
		return geom.Point{}, geom.Point{}, fmt.Errorf("Error trying to convert tile point: %w ", err)
	}

	maxGeo, err := fromWebMercator(srid, geom.Point{extent.MaxX(), extent.MaxY()})
	if err != nil {
		return geom.Point{}, geom.Point{}, fmt.Errorf("Error trying to convert tile point: %w ", err)
	}

	return minGeo.(geom.Point), maxGeo.(geom.Point), nil
}

func getBBoxFilter(dbVersion uint, geomField string, srid uint64) string {
	if dbVersion == 1 {
		if isPlanarEquivalentSrid(srid) {
			return fmt.Sprintf("%v.ST_SRID($3).ST_IntersectsRect(NEW ST_POINT($1, $3), NEW ST_POINT($2, $3)) = 1", quoteIdentifier(geomField))
		} else {
			return fmt.Sprintf("%v.ST_IntersectsRect(NEW ST_POINT($1, $3), NEW ST_POINT($2, $3)) = 1", quoteIdentifier(geomField))
		}
	} else {
		if isPlanarEquivalentSrid(srid) {
			return fmt.Sprintf("%v.ST_SRID($3).ST_IntersectsRectPlanar(NEW ST_POINT($1, $3), NEW ST_POINT($2, $3)) = 1", quoteIdentifier(geomField))
		} else {
			return fmt.Sprintf("%v.ST_IntersectsRectPlanar(NEW ST_POINT($1, $3), NEW ST_POINT($2, $3)) = 1", quoteIdentifier(geomField))
		}
	}
}

func getGeometryColumnSRID(pool *connectionPoolCollector, dbVersion uint, sql string, geomFieldName string) (srid int, err error) {
	sqlQuery := sanitizeSQL(sql)
	sqlQuery = strings.Replace(sqlQuery, bboxToken, "1=1", -1)

	sqlQuery = fmt.Sprintf("SELECT %[1]v.ST_SRID() FROM %[2]v WHERE %[1]v IS NOT NULL LIMIT 1", quoteIdentifier(geomFieldName), sqlQuery)
	err = pool.QueryRow(sqlQuery).Scan(&srid)
	return srid, err
}

func getTileExtent(tile provider.Tile, withBuffer bool) (*geom.Extent, uint64) {
	if withBuffer {
		extent, srid := tile.BufferedExtent()

		minx := math.Max(-20037508.3427892, extent[0])
		miny := math.Max(-20037508.3427892, extent[1])
		maxx := math.Min(20037508.3427892, extent[2])
		maxy := math.Min(20037508.3427892, extent[3])

		return geom.NewExtent([2]float64{minx, miny}, [2]float64{maxx, maxy}), srid
	}

	return tile.Extent()
}

func sanitizeSQL(sql string) string {
	// convert !BOX! (MapServer) and !bbox! (Mapnik) to !BBOX! for compatibility
	return strings.Replace(strings.Replace(sql, "!BOX!", bboxToken, -1), "!bbox!", bboxToken, -1)
}

// replaceTokens replaces tokens in the provided SQL string
//
// !ZOOM! - the tile Z value
// !X! - the tile X value
// !Y! - the tile Y value
// !Z! - the tile Z value
// !SCALE_DENOMINATOR! - scale denominator, assuming 90.7 DPI (i.e. 0.28mm pixel size)
// !PIXEL_WIDTH! - the pixel width in meters, assuming 256x256 tiles
// !PIXEL_HEIGHT! - the pixel height in meters, assuming 256x256 tiles
// !GEOM_FIELD! - the geom field name
// !GEOM_TYPE! - the geom field type if defined otherwise ""
func replaceTokens(dbVersion uint, sql string, idFieldName string, geomFieldName string, geomFieldType geom.Geometry, srid uint64, tile provider.Tile, withBuffer bool) (string, error) {
	var (
		geoType string
	)

	extent, _ := getTileExtent(tile, false)
	// TODO: Always convert to meter if we support different projections
	pixelWidth := (extent.MaxX() - extent.MinX()) / 256
	pixelHeight := (extent.MaxY() - extent.MinY()) / 256
	scaleDenominator := pixelWidth / 0.00028 /* px size in m */

	if geomFieldType != nil {
		geoType = fmt.Sprintf("%v", geomFieldType)
	}

	// replace query string tokens
	z, x, y := tile.ZXY()
	tokenReplacer := strings.NewReplacer(
		bboxToken, getBBoxFilter(dbVersion, geomFieldName, srid),
		zoomToken, strconv.FormatUint(uint64(z), 10),
		zToken, strconv.FormatUint(uint64(z), 10),
		xToken, strconv.FormatUint(uint64(x), 10),
		yToken, strconv.FormatUint(uint64(y), 10),
		idFieldToken, idFieldName,
		geomFieldToken, geomFieldName,
		geomTypeToken, geoType,
		scaleDenominatorToken, strconv.FormatFloat(scaleDenominator, 'f', -1, 64),
		pixelWidthToken, strconv.FormatFloat(pixelWidth, 'f', -1, 64),
		pixelHeightToken, strconv.FormatFloat(pixelHeight, 'f', -1, 64),
	)

	uppercaseTokenSQL := uppercaseTokens(sql)

	return tokenReplacer.Replace(uppercaseTokenSQL), nil
}

func getFieldDescriptions(layerName, geomFieldname, idFieldname string, columns []*sql.ColumnType, checkFieldType bool) ([]FieldDescription, error) {
	list := make([]FieldDescription, 0, len(columns))

	var geomFieldFound bool
	var idFieldFound bool
	for i := range columns {
		column := columns[i]
		fieldName := column.Name()

		isIdField := false
		isGeometryField := false

		var dataType DataType
		switch column.DatabaseTypeName() {
		case "BOOLEAN":
			dataType = DtBoolean
			break
		case "TINYINT":
			dataType = DtTinyint
			break
		case "SMALLINT":
			dataType = DtSmallint
			break
		case "INTEGER":
			dataType = DtInteger
			break
		case "BIGINT":
			dataType = DtBigint
			break
		case "DECIMAL", "FIXED8", "FIXED12", "FIXED16":
			precision, _, _ := column.DecimalSize()
			if precision <= 16 {
				dataType = DtSmalldecimal
			} else {
				dataType = DtDecimal
			}
			break
		case "SMALLDECIMAL":
			dataType = DtSmalldecimal
			break
		case "REAL":
			dataType = DtReal
			break
		case "DOUBLE":
			dataType = DtDouble
			break
		case "CHAR":
			dataType = DtChar
			break
		case "VARCHAR":
			dataType = DtVarchar
			break
		case "NCHAR":
			dataType = DtNChar
			break
		case "NVARCHAR":
			dataType = DtNVarchar
			break
		case "SHORTTEXT":
			dataType = DtShorttext
			break
		case "ALPHANUM":
			dataType = DtAlphanum
			break
		case "BINARY":
			dataType = DtBinary
			break
		case "VARBINARY":
			dataType = DtVarbinary
			break
		case "DATE", "DAYDATE":
			dataType = DtDate
			break
		case "TIME", "SECONDTIME":
			dataType = DtTime
			break
		case "TIMESTAMP", "LONGDATE":
			dataType = DtTimestamp
			break
		case "SECONDDATE":
			dataType = DtSeconddate
			break
		case "BLOB":
			dataType = DtBlob
			break
		case "CLOB":
			dataType = DtClob
			break
		case "NCLOB":
			dataType = DtNClob
			break
		case "TEXT":
			dataType = DtText
			break
		case "STGEOMETRY":
			dataType = DtSTGeometry
			break
		case "STPOINT":
			dataType = DtSTPoint
			break
		default:
			dataType = DtUnknown
		}

		if !geomFieldFound && fieldName == geomFieldname {
			if !checkFieldType || (dataType == DtSTGeometry || dataType == DtSTPoint || dataType == DtBlob) {
				geomFieldFound = true
				isGeometryField = true
			}
		} else if !idFieldFound && fieldName == idFieldname {
			if !checkFieldType || (dataType == DtTinyint || dataType == DtSmallint || dataType == DtInteger || dataType == DtBigint) {
				idFieldFound = true
				isIdField = true
			}
		}

		list = append(list, FieldDescription{
			dataType:    dataType,
			name:        fieldName,
			isFeatureId: isIdField,
			isGeometry:  isGeometryField})
	}

	if !geomFieldFound && geomFieldname != "" {
		return nil, ErrGeomFieldNotFound{
			GeomFieldName: geomFieldname,
			LayerName:     layerName,
		}
	}

	return list, nil
}

func setupRowValues(descriptions []FieldDescription, rowValues []interface{}) {
	for i := range rowValues {
		switch descriptions[i].dataType {
		case DtBoolean:
			rowValues[i] = new(sql.NullBool)
			break
		case DtTinyint:
			rowValues[i] = new(sql.NullByte)
			break
		case DtSmallint:
			rowValues[i] = new(sql.NullInt16)
			break
		case DtInteger:
			rowValues[i] = new(sql.NullInt32)
			break
		case DtBigint:
			rowValues[i] = new(sql.NullInt64)
			break
		case DtDecimal, DtSmalldecimal:
			rowValues[i] = &driver.NullDecimal{Decimal: new(driver.Decimal)}
			break
		case DtReal, DtDouble:
			rowValues[i] = new(sql.NullFloat64)
			break
		case DtDate, DtTime, DtTimestamp, DtSeconddate:
			rowValues[i] = new(sql.NullTime)
			break
		case DtBinary, DtVarbinary:
			rowValues[i] = new(driver.NullBytes)
			break
		case DtBlob, DtClob, DtNClob, DtText:
			rowValues[i] = &driver.NullLob{Lob: new(driver.Lob).SetWriter(new(bytes.Buffer))}
			break
		case DtChar, DtNChar, DtNVarchar, DtVarchar, DtShorttext, DtAlphanum:
			rowValues[i] = new(sql.NullString)
			break
		case DtSTGeometry, DtSTPoint:
			rowValues[i] = new(sql.NullString)
			break
		default:
			rowValues[i] = new(interface{})
			break
		}
	}
}

func readRowValues(ctx context.Context, descriptions []FieldDescription, rowValues []interface{}) (gid uint64, geom []byte, tags map[string]interface{}, err error) {
	var idFieldParsed bool
	tags = make(map[string]interface{})

	for i := range rowValues {
		// do a quick check
		if err := ctx.Err(); err != nil {
			return 0, nil, nil, err
		}

		// skip nil values.
		if rowValues[i] == nil {
			continue
		}

		desc := descriptions[i]
		fieldName := desc.name

		switch desc.dataType {
		case DtBoolean:
			boolValue := *(rowValues[i].(*sql.NullBool))
			if boolValue.Valid {
				tags[fieldName] = boolValue.Bool
			}
			break
		case DtTinyint:
			byteValue := *(rowValues[i].(*sql.NullByte))
			if byteValue.Valid {
				tags[fieldName] = byteValue.Byte
			}
			break
		case DtSmallint:
			int16Value := *(rowValues[i].(*sql.NullInt16))
			if int16Value.Valid {
				tags[fieldName] = int16Value.Int16
			}
			break
		case DtInteger:
			int32Value := *(rowValues[i].(*sql.NullInt32))
			if int32Value.Valid {
				tags[fieldName] = int32Value.Int32
			}
			break
		case DtBigint:
			int64Value := *(rowValues[i].(*sql.NullInt64))
			if int64Value.Valid {
				tags[fieldName] = int64Value.Int64
			}
			break
		case DtDecimal:
			decimalValue := *(rowValues[i].(*driver.NullDecimal))
			if decimalValue.Valid {
				r := (*big.Rat)(decimalValue.Decimal)
				f, _ := r.Float64()
				tags[fieldName] = f
			}
			break
		case DtSmalldecimal:
			decimalValue := *(rowValues[i].(*driver.NullDecimal))
			if decimalValue.Valid {
				r := (*big.Rat)(decimalValue.Decimal)
				f, _ := r.Float32()
				tags[fieldName] = f
			}
			break
		case DtReal, DtDouble:
			float64Value := *(rowValues[i].(*sql.NullFloat64))
			if float64Value.Valid {
				if desc.dataType == DtReal {
					tags[fieldName] = float32(float64Value.Float64)
				} else {
					tags[fieldName] = float64Value.Float64
				}
			}
			break
		case DtDate, DtTime, DtTimestamp, DtSeconddate:
			timeValue := *(rowValues[i].(*sql.NullTime))
			if timeValue.Valid {
				switch desc.dataType {
				case DtDate:
					tags[fieldName] = timeValue.Time.Format("2006-01-02")
					break
				case DtTime:
					tags[fieldName] = timeValue.Time.Format("15:04:05")
					break
				case DtTimestamp:
					tags[fieldName] = timeValue.Time.Format("2006-01-02T15:04:05.000")
					break
				case DtSeconddate:
					tags[fieldName] = timeValue.Time.Format("2006-01-02T15:04:05")
					break
				}
			}
			break
		case DtNVarchar, DtVarchar, DtShorttext, DtAlphanum, DtChar, DtNChar:
			strValue := *(rowValues[i].(*sql.NullString))
			if strValue.Valid {
				if !idFieldParsed && desc.isFeatureId {
					gid, err = convertToUInt64(strValue.String)
					if err != nil {
						return 0, nil, nil, err
					}
					idFieldParsed = true
					break
				} else {
					tags[fieldName] = strValue.String
				}
			}
			break
		case DtBinary, DtVarbinary:
			binValue := *(rowValues[i].(*driver.NullBytes))
			if binValue.Valid {
				tags[fieldName] = hex.EncodeToString(binValue.Bytes[:])
			}
			break
		case DtBlob, DtClob, DtNClob, DtText:
			lobValue := *(rowValues[i].(*driver.NullLob))
			if lobValue.Valid {
				writer := lobValue.Lob.Writer()
				data := writer.(*bytes.Buffer).Bytes()
				dataLen := writer.(*bytes.Buffer).Len()
				if dataLen > 0 {
					if desc.isGeometry {
						geom = make([]byte, dataLen)
						copy(geom, data)
					} else {
						if desc.dataType == DtBlob {
							tags[fieldName] = hex.EncodeToString(data[0:dataLen])
						} else {
							tags[fieldName] = string(data[0:dataLen])
						}
					}
				}
			}
			break
		case DtSTGeometry, DtSTPoint:
			strValue := *(rowValues[i].(*sql.NullString))
			if strValue.Valid {
				if desc.isGeometry {
					geom, err = hex.DecodeString(strValue.String)
					if err != nil {
						return 0, nil, nil, fmt.Errorf("unable to decode geometry binary string in field '%v'", fieldName)
					}
				} else {
					tags[fieldName] = strValue.String
				}
			}
			break
		default:
			return 0, nil, nil, fmt.Errorf("data type is unsupported in field '%v'", fieldName)
		}
	}

	return gid, geom, tags, nil
}

func convertToUInt64(v interface{}) (intv uint64, err error) {
	switch aval := v.(type) {
	case float64:
		return uint64(aval), nil
	case int64:
		return uint64(aval), nil
	case uint64:
		return aval, nil
	case uint:
		return uint64(aval), nil
	case int8:
		return uint64(aval), nil
	case uint8:
		return uint64(aval), nil
	case uint16:
		return uint64(aval), nil
	case int32:
		return uint64(aval), nil
	case uint32:
		return uint64(aval), nil
	case string:
		return strconv.ParseUint(aval, 10, 64)
	case sql.NullString:
		return strconv.ParseUint(aval.String, 10, 64)
	default:
		return intv, fmt.Errorf("unable to convert field into a uint64")
	}
}

// extractQueryParamValues finds default values for SQL tokens and constructs query parameter values out of them
func extractQueryParamValues(pname string, maps []provider.Map, layer *Layer) provider.Params {
	result := make(provider.Params, 0)

	expectedMapName := fmt.Sprintf("%s.%s", pname, layer.name)
	for _, m := range maps {
		for _, l := range m.Layers {
			if l.ProviderLayer == env.String(expectedMapName) {
				for _, p := range m.Parameters {
					pv, err := p.ToDefaultValue()
					if err == nil {
						result[p.Token] = pv
					}
				}
			}
		}
	}

	return result
}

var tokenRe = regexp.MustCompile("![a-zA-Z0-9_-]+!")

//	uppercaseTokens converts all !tokens! to uppercase !TOKENS!. Tokens can
//	contain alphanumerics, dash and underline chars.
func uppercaseTokens(str string) string {
	return tokenRe.ReplaceAllStringFunc(str, strings.ToUpper)
}

// ctxErr will check if the supplied context has an error (i.e. context canceled)
// and if so, return that error, else return the supplied error. This is useful
// as not all of Go's stdlib has adopted error wrapping so context.Canceled
// errors are not always easy to capture.
func ctxErr(ctx context.Context, err error) error {
	if ctxErr := ctx.Err(); ctxErr != nil {
		return ctxErr
	}

	return err
}
