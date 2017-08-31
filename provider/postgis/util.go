package postgis

import (
	"fmt"
	"log"
	"strconv"
	"strings"

	"github.com/jackc/pgx"
	"github.com/terranodo/tegola"
	"github.com/terranodo/tegola/basic"
)

// genSQL will fill in the SQL field of a layer given a pool, and list of fields.
func genSQL(l *layer, pool *pgx.ConnPool, tblname string, flds []string) (sql string, err error) {

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
func replaceTokens(plyr *layer, tile tegola.Tile) (string, error) {

	textent := tile.BoundingBox()

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

func transfromVal(valType pgx.Oid, val interface{}) (interface{}, error) {
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
