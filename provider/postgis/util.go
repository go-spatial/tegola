package postgis

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"

	"github.com/jackc/pgx"
	"github.com/terranodo/tegola"
	"github.com/terranodo/tegola/basic"
	"github.com/terranodo/tegola/provider"
)

// genSQL will fill in the SQL field of a layer given a pool, and list of fields.
func genSQL(l *Layer, pool *pgx.ConnPool, tblname string, flds []string) (sql string, err error) {

	if len(flds) == 0 {
		// We need to hit the database to see what the fields are.
		rows, err := pool.Query(fmt.Sprintf(fldsSQL, tblname))
		if err != nil {
			return "", err
		}
		defer rows.Close()

		fdescs := rows.FieldDescriptions()
		if len(fdescs) == 0 {
			return "", fmt.Errorf("No fields were returned for table %v", tblname)
		}
		//	to avoid field names possibly colliding with Postgres keywords,
		//	we wrap the field names in quotes
		for i, _ := range fdescs {
			flds = append(flds, fdescs[i].Name)
		}
	}
	for i := range flds {
		flds[i] = fmt.Sprintf(`"%v"`, flds[i])
	}

	var fgeom int = -1
	var fgid bool
	for i, f := range flds {
		if f == `"`+l.geomField+`"` {
			fgeom = i
		}
		if f == `"`+l.idField+`"` {
			fgid = true
		}
	}

	//	to avoid field names possibly colliding with Postgres keywords,
	//	we wrap the field names in quotes
	if fgeom == -1 {
		flds = append(flds, fmt.Sprintf(`ST_AsBinary("%v") AS "%[1]v"`, l.geomField))
	} else {
		flds[fgeom] = fmt.Sprintf(`ST_AsBinary("%v") AS "%[1]v"`, l.geomField)
	}

	if !fgid {
		flds = append(flds, fmt.Sprintf(`"%v"`, l.idField))
	}

	selectClause := strings.Join(flds, ", ")

	return fmt.Sprintf(stdSQL, selectClause, tblname, l.geomField), nil
}

const (
	bboxToken = "!BBOX!"
	zoomToken = "!ZOOM!"
)

//	replaceTokens replaces tokens in the provided SQL string
//
//	!BBOX! - the bounding box of the tile
//	!ZOOM! - the tile Z value
func replaceTokens(plyr *Layer, tile *tegola.Tile) (string, error) {

	textent, err := tile.BufferedBoundingBox()
	if err != nil {
		return "", nil
	}

	minGeo, err := basic.FromWebMercator(plyr.srid, basic.Point{textent.Minx, textent.Miny})
	if err != nil {
		return "", fmt.Errorf("Error trying to convert tile point: %v ", err)
	}
	maxGeo, err := basic.FromWebMercator(plyr.srid, basic.Point{textent.Maxx, textent.Maxy})
	if err != nil {
		return "", fmt.Errorf("Error trying to convert tile point: %v ", err)
	}

	minPt, maxPt := minGeo.AsPoint(), maxGeo.AsPoint()

	bbox := fmt.Sprintf("ST_MakeEnvelope(%v,%v,%v,%v,%v)", minPt.X(), minPt.Y(), maxPt.X(), maxPt.Y(), plyr.srid)

	//	replace query string tokens
	tokenReplacer := strings.NewReplacer(
		bboxToken, bbox,
		zoomToken, strconv.Itoa(tile.Z),
	)

	return tokenReplacer.Replace(plyr.sql), nil
}

//	replaceTokens replaces tokens in the provided SQL string
//
//	!BBOX! - the bounding box of the tile
//	!ZOOM! - the tile Z value
func replaceTokens2(plyr *Layer, tile provider.Tile) (string, error) {

	//	TODO: make sure the tile returns the buffered bounds
	textent, _ := tile.BufferedExtent()

	//	TODO: leverage helper functions for minx / miny to make this easier to follow
	//	TODO: it's currently assumed the tile will always be in WebMercator. Need to support different projections
	minGeo, err := basic.FromWebMercator(plyr.srid, basic.Point{textent[0][0], textent[0][1]})
	if err != nil {
		return "", fmt.Errorf("Error trying to convert tile point: %v ", err)
	}
	maxGeo, err := basic.FromWebMercator(plyr.srid, basic.Point{textent[1][0], textent[1][1]})
	if err != nil {
		return "", fmt.Errorf("Error trying to convert tile point: %v ", err)
	}

	minPt, maxPt := minGeo.AsPoint(), maxGeo.AsPoint()

	bbox := fmt.Sprintf("ST_MakeEnvelope(%v,%v,%v,%v,%v)", minPt.X(), minPt.Y(), maxPt.X(), maxPt.Y(), plyr.srid)

	//	replace query string tokens
	tokenReplacer := strings.NewReplacer(
		bboxToken, bbox,
		zoomToken, strconv.FormatUint(tile.Z(), 10),
	)

	return tokenReplacer.Replace(plyr.sql), nil
}

func transformVal(valType pgx.Oid, val interface{}) (interface{}, error) {
	switch valType {
	default:
		switch vt := val.(type) {
		default:
			log.Printf("%v type is not supported. (Expected it to be a stringer type)", valType)
			return nil, fmt.Errorf("%v type is not supported. (Expected it to be a stringer type)", valType)
		case fmt.Stringer:
			return vt.String(), nil
		case string:
			return vt, nil
		}
	case pgx.BoolOid, pgx.ByteaOid, pgx.TextOid, pgx.OidOid, pgx.VarcharOid, pgx.JsonbOid:
		return val, nil
	case pgx.Int8Oid, pgx.Int2Oid, pgx.Int4Oid, pgx.Float4Oid, pgx.Float8Oid:
		switch vt := val.(type) {
		case int8:
			return int64(vt), nil
		case int16:
			return int64(vt), nil
		case int32:
			return int64(vt), nil
		case int64, uint64:
			return vt, nil
		case uint8:
			return int64(vt), nil
		case uint16:
			return int64(vt), nil
		case uint32:
			return int64(vt), nil
		case float32:
			return float64(vt), nil
		case float64:
			return vt, nil
		default: // should never happen.
			return nil, fmt.Errorf("%v type is not supported. (should never happen)", valType)
		}
	case pgx.DateOid, pgx.TimestampOid, pgx.TimestampTzOid:
		return fmt.Sprintf("%v", val), nil
	}
}

func decipherFields(ctx context.Context, geoFieldname, idFieldname string, descriptions []pgx.FieldDescription, values []interface{}) (gid uint64, geom []byte, tags map[string]interface{}, err error) {
	tags = make(map[string]interface{})
	var desc pgx.FieldDescription
	var ok bool

	for i, v := range values {
		// Do a quick check
		if err := ctx.Err(); err != nil {
			return 0, nil, nil, err
		}
		// Skip nil values.
		if values[i] == nil {
			continue
		}
		desc = descriptions[i]
		switch desc.Name {
		case geoFieldname:
			if geom, ok = v.([]byte); !ok {
				return 0, nil, nil, fmt.Errorf("Unable to convert geometry field (%v) into bytes.", geoFieldname)
			}
		case idFieldname:
			gid, err = gId(v)
		default:
			switch desc.DataTypeName {
			// hstore is a special case
			case "hstore":
				// parse our Hstore values into keys and values
				keys, values, err := pgx.ParseHstore(v.(string))
				if err != nil {
					return gid, geom, tags, fmt.Errorf("Unable to parse Hstore err: %v", err)
				}
				for i, k := range keys {
					// if the value is Valid (i.e. not null) then add it to our tags map.
					if values[i].Valid {
						//	we need to check if the key already exists. if it does, then don't overwrite it
						if _, ok := tags[k]; !ok {
							tags[k] = values[i].String
						}
					}
				}
				continue
			case "numeric":
				num, err := strconv.ParseFloat(v.(string), 64)
				if err != nil {
					return 0, nil, nil, fmt.Errorf("Unable to parse numeric (%v) to float64 err: %v", v.(string), err)
				}
				tags[desc.Name] = num
				continue
			default:
				value, err := transformVal(desc.DataType, v)
				if err != nil {
					return gid, geom, tags, fmt.Errorf("Unable to convert field[%v] (%v) of type (%v - %v) to a suitable value.: [[ %T  :: %[5]t ]]", i, desc.Name, desc.DataType, desc.DataTypeName, v)
				}
				tags[desc.Name] = value
			}
		}
	}
	return gid, geom, tags, err
}

func gId(v interface{}) (gid uint64, err error) {
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
	default:
		return gid, fmt.Errorf("Unable to convert field into a uint64.")
	}
}
