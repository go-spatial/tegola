package postgis

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/jackc/pgx"

	"github.com/terranodo/tegola"
	"github.com/terranodo/tegola/basic"
	"github.com/terranodo/tegola/mvt"
	"github.com/terranodo/tegola/mvt/provider"
	"github.com/terranodo/tegola/util/dict"
	"github.com/terranodo/tegola/wkb"
)

// Provider provides the postgis data provider.
type Provider struct {
	config pgx.ConnPoolConfig
	pool   *pgx.ConnPool
	// map of layer name and corrosponding sql
	layers map[string]layer
	srid   int
}

// layer holds information about a query.
type layer struct {
	// The Name of the layer
	Name string
	// The SQL to use when querying PostGIS for this layer
	SQL string
	// The ID field name, this will default to 'gid' if not set to something other then empty string.
	IDFieldName string
	// The Geometery field name, this will default to 'geom' if not set to soemthing other then empty string.
	GeomFieldName string
	// The SRID that the data in the table is stored in. This will default to WebMercator
	SRID int
}

const (
	bboxToken = "!BBOX!"
	zoomToken = "!ZOOM!"
)

const (
	// We quote the field and table names to prevent colliding with postgres keywords.
	stdSQL = `SELECT %[1]v FROM %[2]v WHERE "%[3]v" && ` + bboxToken

	// SQL to get the column names, without hitting the information_schema. Though it might be better to hit the information_schema.
	fldsSQL = `SELECT * FROM %[1]v LIMIT 0;`
)

const Name = "postgis"

const (
	DefaultPort    = 5432
	DefaultSRID    = tegola.WebMercator
	DefaultMaxConn = 5
)

const (
	ConfigKeyHost        = "host"
	ConfigKeyPort        = "port"
	ConfigKeyDB          = "database"
	ConfigKeyUser        = "user"
	ConfigKeyPassword    = "password"
	ConfigKeyMaxConn     = "max_connection"
	ConfigKeySRID        = "srid"
	ConfigKeyLayers      = "layers"
	ConfigKeyLayerName   = "name"
	ConfigKeyTablename   = "tablename"
	ConfigKeySQL         = "sql"
	ConfigKeyFields      = "fields"
	ConfigKeyGeomField   = "geometry_fieldname"
	ConfigKeyGeomIDField = "id_fieldname"
)

func init() {
	provider.Register(Name, NewProvider)
}

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
		if f == `"`+l.GeomFieldName+`"` {
			fgeom = i
		}
		if f == `"`+l.IDFieldName+`"` {
			fgid = true
		}
	}

	//	to avoid field names possibly colliding with Postgres keywords,
	//	we wrap the field names in quotes
	if fgeom == -1 {
		flds = append(flds, fmt.Sprintf(`ST_AsBinary("%v") AS "%[1]v"`, l.GeomFieldName))
	} else {
		flds[fgeom] = fmt.Sprintf(`ST_AsBinary("%v") AS "%[1]v"`, l.GeomFieldName)
	}

	if !fgid {
		flds = append(flds, fmt.Sprintf(`"%v"`, l.IDFieldName))
	}

	selectClause := strings.Join(flds, ", ")

	return fmt.Sprintf(stdSQL, selectClause, tblname, l.GeomFieldName), nil
}

//	NewProvider Setups and returns a new postgis provider or an error; if something
//	is wrong. The function will validate that the config object looks good before
//	trying to create a driver. This means that the Provider expects the following
//	fields to exists in the provided map[string]interface{} map:
//
//		host (string) — the host to connect to.
// 		port (uint16) — the port to connect on.
//		database (string) — the database name
//		user (string) — the user name
//		password (string) — the Password
//		max_connections (*uint8) // Default is 5 if nil, 0 means no max.
//		layers (map[string]struct{})  — This is map of layers keyed by the layer name.
//     		tablename (string || sql string) — This is the sql to use or the tablename to use with the default query.
//     		fields ([]string) — This is a list, if this is nil or empty we will get all fields.
//     		geometry_fieldname (string) — This is the field name of the geometry, if it's an empty string or nil, it will defaults to 'geom'.
//     		id_fieldname (string) — This is the field name for the id property, if it's an empty string or nil, it will defaults to 'gid'.
//
func NewProvider(config map[string]interface{}) (mvt.Provider, error) {
	// Validate the config to make sure it has the values I care about and the types for those values.
	c := dict.M(config)

	host, err := c.String(ConfigKeyHost, nil)
	if err != nil {
		return nil, err
	}

	db, err := c.String(ConfigKeyDB, nil)
	if err != nil {
		return nil, err
	}

	user, err := c.String(ConfigKeyUser, nil)
	if err != nil {
		return nil, err
	}

	password, err := c.String(ConfigKeyPassword, nil)
	if err != nil {
		return nil, err
	}

	port := int64(DefaultPort)
	if port, err = c.Int64(ConfigKeyPort, &port); err != nil {
		return nil, err
	}

	maxcon := int(DefaultMaxConn)
	if maxcon, err = c.Int(ConfigKeyMaxConn, &maxcon); err != nil {
		return nil, err
	}

	var srid = int64(DefaultSRID)
	if srid, err = c.Int64(ConfigKeySRID, &srid); err != nil {
		return nil, err
	}

	p := Provider{
		srid: int(srid),
		config: pgx.ConnPoolConfig{
			ConnConfig: pgx.ConnConfig{
				Host:     host,
				Port:     uint16(port),
				Database: db,
				User:     user,
				Password: password,
			},
			MaxConnections: maxcon,
		},
	}

	if p.pool, err = pgx.NewConnPool(p.config); err != nil {
		return nil, fmt.Errorf("Failed while creating connection pool: %v", err)
	}

	layers, ok := c[ConfigKeyLayers].([]map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("Expected %v to be a []map[string]interface{}", ConfigKeyLayers)
	}

	lyrs := make(map[string]layer)
	lyrsSeen := make(map[string]int)

	for i, v := range layers {
		vc := dict.M(v)

		lname, err := vc.String(ConfigKeyLayerName, nil)
		if err != nil {
			return nil, fmt.Errorf("For layer (%v) we got the following error trying to get the layer's name field: %v", i, err)
		}
		if j, ok := lyrsSeen[lname]; ok {
			return nil, fmt.Errorf("%v layer name is duplicated in both layer %v and layer %v", lname, i, j)
		}
		lyrsSeen[lname] = i

		fields, err := vc.StringSlice(ConfigKeyFields)
		if err != nil {
			return nil, fmt.Errorf("For layer (%v) %v %v field had the following error: %v", i, lname, ConfigKeyFields, err)
		}

		geomfld := "geom"
		geomfld, err = vc.String(ConfigKeyGeomField, &geomfld)
		if err != nil {
			return nil, fmt.Errorf("For layer (%v) %v : %v", i, lname, err)
		}

		idfld := "gid"
		idfld, err = vc.String(ConfigKeyGeomIDField, &idfld)
		if err != nil {
			return nil, fmt.Errorf("For layer (%v) %v : %v", i, lname, err)
		}
		if idfld == geomfld {
			return nil, fmt.Errorf("For layer (%v) %v: %v (%v) and %v field (%v) is the same!", i, lname, ConfigKeyGeomField, geomfld, ConfigKeyGeomIDField, idfld)
		}

		var tblName string
		tblName, err = vc.String(ConfigKeyTablename, &lname)
		if err != nil {
			return nil, fmt.Errorf("for %v layer(%v) %v has an error: %v", i, lname, ConfigKeyTablename, err)
		}

		var sql string
		sql, err = vc.String(ConfigKeySQL, &sql)
		if err != nil {
			return nil, fmt.Errorf("for %v layer(%v) %v has an error: %v", i, lname, ConfigKeySQL, err)
		}

		if tblName != lname && sql != "" {
			log.Printf("Both %v and %v field are specified for layer(%v) %v, using only %[2]v field.", ConfigKeyTablename, ConfigKeySQL, i, lname)
		}

		var lsrid = srid
		if lsrid, err = vc.Int64(ConfigKeySRID, &lsrid); err != nil {
			return nil, err
		}

		l := layer{
			Name:          lname,
			IDFieldName:   idfld,
			GeomFieldName: geomfld,
			SRID:          int(lsrid),
		}
		if sql != "" {
			// make sure that the sql has a !BBOX! token
			if !strings.Contains(sql, bboxToken) {
				return nil, fmt.Errorf("SQL for layer (%v) %v does not contain "+bboxToken+", entry.", i, lname)
			}
			if !strings.Contains(sql, "*") {
				if !strings.Contains(sql, geomfld) {
					return nil, fmt.Errorf("SQL for layer (%v) %v does not contain the geometry field: %v", i, lname, geomfld)
				}
				if !strings.Contains(sql, idfld) {
					return nil, fmt.Errorf("SQL for layer (%v) %v does not contain the id field for the geometry: %v", i, lname, idfld)
				}
			}
			l.SQL = sql
		} else {
			// Tablename and Fields will be used to
			// We need to do some work. We need to check to see Fields contains the geom and gid fields
			// and if not add them to the list. If Fields list is empty/nil we will use '*' for the field
			// list.
			l.SQL, err = genSQL(&l, p.pool, tblName, fields)
			if err != nil {
				return nil, fmt.Errorf("Could not generate sql, for layer(%v): %v", lname, err)
			}
		}
		if strings.Contains(os.Getenv("SQL_DEBUG"), "LAYER_SQL") {
			log.Printf("SQL for Layer(%v):\n%v\n", lname, l.SQL)
		}
		lyrs[lname] = l
	}
	p.layers = lyrs

	return p, nil
}

func (p Provider) LayerNames() (names []string) {
	for k, _ := range p.layers {
		names = append(names, k)
	}
	return names
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
		default: // should never happen.
			return nil, fmt.Errorf("%v type is not supported. (should never happen)", valType)
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
		}
	case pgx.DateOid, pgx.TimestampOid, pgx.TimestampTzOid:
		return fmt.Sprintf("%v", val), nil
	}
}

func (p Provider) MVTLayer(layerName string, tile tegola.Tile, tags map[string]interface{}) (layer *mvt.Layer, err error) {

	plyr, ok := p.layers[layerName]
	if !ok {
		return nil, fmt.Errorf("Don't know of the layer %v", layerName)
	}

	sql, err := replaceTokens(&plyr, tile)
	if err != nil {
		return nil, fmt.Errorf("Got the following error (%v) running this sql (%v)", err, sql)
	}

	rows, err := p.pool.Query(sql)
	if err != nil {
		return nil, fmt.Errorf("Got the following error (%v) running this sql (%v)", err, sql)
	}
	defer rows.Close()

	//	fetch rows FieldDescriptions. this gives us the OID for the data types returned to aid in decoding
	fdescs := rows.FieldDescriptions()
	var geobytes []byte

	//	new mvt.Layer
	layer = new(mvt.Layer)
	layer.Name = layerName

	var count int
	// var didEnd bool
	//log.Printf("Running SQL:\n%v", sql)

	for rows.Next() {
		count++
		var geom tegola.Geometry
		var gid uint64

		//	fetch row values
		vals, err := rows.Values()
		if err != nil {
			return nil, fmt.Errorf("Got an error trying to run SQL: %v ; %v", sql, err)
		}
		//	holds our encoded tags
		gtags := make(map[string]interface{})

		//	iterate the values returned from our row
		for i, v := range vals {
			switch fdescs[i].Name {
			case plyr.GeomFieldName:
				if geobytes, ok = v.([]byte); !ok {
					return nil, fmt.Errorf("Was unable to convert geometry field (%v) into bytes for layer (%v)", plyr.GeomFieldName, layerName)
				}
				//	decode our WKB
				if geom, err = wkb.DecodeBytes(geobytes); err != nil {
					return nil, fmt.Errorf("Was unable to decode geometry field (%v) into wkb for layer (%v)", plyr.GeomFieldName, layerName)
				}
				// TODO: Need to move this from being the responsiblity of the provider to the responsibility of the feature. But that means a feature should know
				// how the points are encoded.
				if plyr.SRID != DefaultSRID {
					// We need to convert our points to Webmercator.
					g, err := basic.ToWebMercator(plyr.SRID, geom)
					if err != nil {
						return nil, fmt.Errorf("Was unable to transform geometry to webmercator from SRID (%v) for layer (%v)", plyr.SRID, layerName)
					}
					geom = g.Geometry
				}
			case plyr.IDFieldName:
				switch aval := v.(type) {
				case int64:
					gid = uint64(aval)
				case uint64:
					gid = aval
				case int:
					gid = uint64(aval)
				case uint:
					gid = uint64(aval)
				case int8:
					gid = uint64(aval)
				case uint8:
					gid = uint64(aval)
				case int16:
					gid = uint64(aval)
				case uint16:
					gid = uint64(aval)
				case int32:
					gid = uint64(aval)
				case uint32:
					gid = uint64(aval)
				default:
					return nil, fmt.Errorf("Unable to convert geometry ID field (%v) into a uint64 for layer (%v)", plyr.IDFieldName, layerName)
				}
			default:
				if vals[i] == nil {
					// We want to skip all nil values.
					continue
				}

				//	hstore is a special case
				if fdescs[i].DataTypeName == "hstore" {
					//	parse our Hstore values into keys and values
					keys, values, err := pgx.ParseHstore(v.(string))
					if err != nil {
						return nil, fmt.Errorf("Unable to parse Hstore err: %v", err)
					}

					for i, k := range keys {
						//	if the value is Valid (i.e. not null) then add it to our gtags map
						if values[i].Valid {
							gtags[k] = values[i].String
						}
					}
					continue
				}

				value, err := transfromVal(fdescs[i].DataType, vals[i])
				if err != nil {
					return nil, fmt.Errorf("Unable to convert field[%v] (%v) of type (%v - %v) to a suitable value.: [[ %T  :: %[5]t ]]", i, fdescs[i].Name, fdescs[i].DataType, fdescs[i].DataTypeName, vals[i])
				}
				gtags[fdescs[i].Name] = value
			}
		}

		for k, v := range tags {
			// If tags does not exists, then let's add it.
			if _, ok = gtags[k]; !ok {
				gtags[k] = v
			}
		}

		// Add features to Layer
		layer.AddFeatures(mvt.Feature{
			ID:       &gid,
			Tags:     gtags,
			Geometry: geom,
		})
	}
	/*
		didEnd = true
		log.Printf("Got %v rows running:\n%v\nDid complete %v\n", count, sql, didEnd)
	*/
	return layer, err
}

//	replaceTokens replaces tokens in the provided SQL string
//
//	!BBOX! - the bounding box of the tile
//	!ZOOM! - the tile Z value
func replaceTokens(plyr *layer, tile tegola.Tile) (string, error) {

	textent := tile.BoundingBox()

	minGeo, err := basic.FromWebMercator(plyr.SRID, basic.Point{textent.Minx, textent.Miny})
	if err != nil {
		return "", fmt.Errorf("Error trying to convert tile point: %v ", err)
	}
	maxGeo, err := basic.FromWebMercator(plyr.SRID, basic.Point{textent.Maxx, textent.Maxy})
	if err != nil {
		return "", fmt.Errorf("Error trying to convert tile point: %v ", err)
	}

	minPt, maxPt := minGeo.AsPoint(), maxGeo.AsPoint()

	bbox := fmt.Sprintf("ST_MakeEnvelope(%v,%v,%v,%v,%v)", minPt.X(), minPt.Y(), maxPt.X(), maxPt.Y(), plyr.SRID)

	//	replace query string tokens
	tokenReplacer := strings.NewReplacer(
		bboxToken, bbox,
		zoomToken, strconv.Itoa(tile.Z),
	)

	return tokenReplacer.Replace(plyr.SQL), nil
}
