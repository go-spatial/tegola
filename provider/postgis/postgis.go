package postgis

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/jackc/pgx"

	"context"

	"github.com/terranodo/tegola"
	"github.com/terranodo/tegola/basic"
	"github.com/terranodo/tegola/mvt"
	"github.com/terranodo/tegola/mvt/provider"
	"github.com/terranodo/tegola/util/dict"
	"github.com/terranodo/tegola/wkb"
)

const Name = "postgis"

// Provider provides the postgis data provider.
type Provider struct {
	config pgx.ConnPoolConfig
	pool   *pgx.ConnPool
	// map of layer name and corrosponding sql
	layers map[string]layer
	srid   int
}

const (
	// We quote the field and table names to prevent colliding with postgres keywords.
	stdSQL = `SELECT %[1]v FROM %[2]v WHERE "%[3]v" && ` + bboxToken

	// SQL to get the column names, without hitting the information_schema. Though it might be better to hit the information_schema.
	fldsSQL = `SELECT * FROM %[1]v LIMIT 0;`
)

const (
	DefaultPort    = 5432
	DefaultSRID    = tegola.WebMercator
	DefaultMaxConn = 100
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
			name:      lname,
			idField:   idfld,
			geomField: geomfld,
			srid:      int(lsrid),
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
			l.sql = sql
		} else {
			// Tablename and Fields will be used to
			// We need to do some work. We need to check to see Fields contains the geom and gid fields
			// and if not add them to the list. If Fields list is empty/nil we will use '*' for the field
			// list.
			l.sql, err = genSQL(&l, p.pool, tblName, fields)
			if err != nil {
				return nil, fmt.Errorf("Could not generate sql, for layer(%v): %v", lname, err)
			}
		}
		if strings.Contains(os.Getenv("SQL_DEBUG"), "LAYER_SQL") {
			log.Printf("SQL for Layer(%v):\n%v\n", lname, l.sql)
		}

		//	set the layer geom type
		if err = p.layerGeomType(&l); err != nil {
			return nil, fmt.Errorf("error fetching geometry type for layer (%v): %v", l.name, err)
		}

		lyrs[lname] = l
	}
	p.layers = lyrs

	return p, nil
}

//	layerGeomType sets the geomType field on the layer by running the SQL and reading the geom type in the result set
func (p Provider) layerGeomType(l *layer) error {
	var err error

	//	we need a tile to run our sql through the replacer
	tile := tegola.Tile{Z: 0, X: 0, Y: 0}

	sql, err := replaceTokens(l, tile)
	if err != nil {
		return err
	}

	//	we want to know the geom type instead of returning the geom data so we modify the SQL
	//	TODO: this strategy wont work if remove the requirement of wrapping ST_AsBinary(geom) in the SQL statements.
	sql = strings.Replace(strings.ToLower(sql), "st_asbinary", "st_geometrytype", 1)

	//	we only need a single result set to sniff out the geometry type
	sql = fmt.Sprintf("%v LIMIT 1", sql)

	rows, err := p.pool.Query(sql)
	if err != nil {
		return err
	}
	defer rows.Close()

	//	fetch rows FieldDescriptions. this gives us the OID for the data types returned to aid in decoding
	fdescs := rows.FieldDescriptions()
	for rows.Next() {

		vals, err := rows.Values()
		if err != nil {
			return fmt.Errorf("error running SQL: %v ; %v", sql, err)
		}

		//	iterate the values returned from our row, sniffing for the geomField or st_geometrytype field name
		for i, v := range vals {
			switch fdescs[i].Name {
			case l.geomField, "st_geometrytype":
				switch v {
				case "ST_Point":
					l.geomType = basic.Point{}
				case "ST_LineString":
					l.geomType = basic.Line{}
				case "ST_Polygon":
					l.geomType = basic.Polygon{}
				case "ST_MultiPoint":
					l.geomType = basic.MultiPoint{}
				case "ST_MultiLineString":
					l.geomType = basic.MultiLine{}
				case "ST_MultiPolygon":
					l.geomType = basic.MultiPolygon{}
				case "ST_GeometryCollection":
					l.geomType = basic.Collection{}
				default:
					return fmt.Errorf("layer (%v) returned unsupported geometry type (%v)", l.name, v)
				}
			}
		}
	}

	return nil
}

func (p Provider) Layers() ([]mvt.LayerInfo, error) {
	var ls []mvt.LayerInfo

	for i := range p.layers {
		ls = append(ls, p.layers[i])
	}

	return ls, nil
}

func (p Provider) MVTLayer(ctx context.Context, layerName string, tile tegola.Tile, tags map[string]interface{}) (layer *mvt.Layer, err error) {
	plyr, ok := p.layers[layerName]
	if !ok {
		return nil, fmt.Errorf("Don't know of the layer %v", layerName)
	}

	//	replace the various tokens we support (i.e. !BBOX!, !ZOOM!) with balues
	sql, err := replaceTokens(&plyr, tile)
	if err != nil {
		return nil, err
	}

	// do a quick context check:
	if ctx.Err() != nil {
		return nil, mvt.ErrCanceled
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

	for rows.Next() {
		// do a quick context check:
		if ctx.Err() != nil {
			return nil, mvt.ErrCanceled
		}

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
			// do a quick context check:
			if ctx.Err() != nil {
				return nil, mvt.ErrCanceled
			}

			switch fdescs[i].Name {
			case plyr.geomField:
				if geobytes, ok = v.([]byte); !ok {
					return nil, fmt.Errorf("Was unable to convert geometry field (%v) into bytes for layer (%v)", plyr.geomField, layerName)
				}
				//	decode our WKB
				if geom, err = wkb.DecodeBytes(geobytes); err != nil {
					return nil, fmt.Errorf("Was unable to decode geometry field (%v) into wkb for layer (%v)", plyr.geomField, layerName)
				}
				// TODO: Need to move this from being the responsiblity of the provider to the responsibility of the feature. But that means a feature should know
				// how the points are encoded.
				// log.Printf("layer SRID %v Default: %v\n", plyr.SRID, DefaultSRID)
				if plyr.srid != DefaultSRID {
					// We need to convert our points to Webmercator.
					g, err := basic.ToWebMercator(plyr.srid, geom)
					if err != nil {
						return nil, fmt.Errorf("Was unable to transform geometry to webmercator from SRID (%v) for layer (%v)", plyr.srid, layerName)
					}
					geom = g.Geometry
				}
			case plyr.idField:
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
					return nil, fmt.Errorf("Unable to convert geometry ID field (%v) into a uint64 for layer (%v)", plyr.idField, layerName)
				}
			default:
				if vals[i] == nil {
					// We want to skip all nil values.
					continue
				}

				//	hstore
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

				//	decimal support
				//	pgx returns numeric datatypes as strings so we need to handle parsing them ourselves
				//	https://github.com/jackc/pgx/issues/56
				if fdescs[i].DataTypeName == "numeric" {
					num, err := strconv.ParseFloat(v.(string), 64)
					if err != nil {
						return nil, fmt.Errorf("Unable to parse numeric (%v) to float64 err: %v", v.(string), err)
					}

					gtags[fdescs[i].Name] = num
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

	return layer, err
}
