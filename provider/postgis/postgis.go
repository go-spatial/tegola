package postgis

import (
	"fmt"
	"log"
	"strings"

	"github.com/jackc/pgx"

	"github.com/terranodo/tegola"
	"github.com/terranodo/tegola/mvt"
	"github.com/terranodo/tegola/mvt/provider"
	"github.com/terranodo/tegola/util/dict"
	"github.com/terranodo/tegola/wkb"
)

// layer holds information about a query.
type layer struct {
	// The Name of the layer
	Name string
	// The SQL to use. !BBOX! token will be replaced by the envelope
	SQL string
	// The ID field name, this will default to 'gid' if not set to something other then empty string.
	IDFieldName string
	// The Geometery field name, this will default to 'geom' if not set to soemthing other then empty string.
	GeomFieldName string
}

// Provider provides the postgis data provider.
type Provider struct {
	config pgx.ConnPoolConfig
	pool   *pgx.ConnPool
	layers map[string]layer // map of layer name and corrosponding sql
	srid   int
}

// DEFAULT sql for get geometries,
const BBOX = "!BBOX!"
const stdSQL = `
SELECT %[1]v
FROM
	%[2]v
WHERE
	%[3]v && ` + BBOX

// SQL to get the column names, without hitting the information_schema. Though it might be better to hit the information_schema.
const fldsSQL = "SELECT * FROM %[1]v LIMIT 0;"

const Name = "postgis"
const DefaultPort = 5432
const DefaultSRID = 3857
const DefaultMaxConn = 5

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
func (l *layer) genSQL(pool *pgx.ConnPool, tblname string, flds []string) (err error) {

	if len(flds) == 0 {
		// We need to hit the database to see what the fields are.
		rows, err := pool.Query(fmt.Sprintf(fldsSQL, tblname))
		if err != nil {
			return err
		}
		defer rows.Close()
		fdescs := rows.FieldDescriptions()
		if len(fdescs) == 0 {
			return fmt.Errorf("No fields were returned for table %v", tblname)
		}
		for i, _ := range fdescs {
			flds = append(flds, fdescs[i].Name)
		}

	}
	var fgeom int = -1
	var fgid bool
	for i, f := range flds {
		if f == l.GeomFieldName {
			fgeom = i
		}
		if f == l.IDFieldName {
			fgid = true
		}
	}

	if fgeom == -1 {
		flds = append(flds, fmt.Sprintf("ST_AsBinary(%v)", l.GeomFieldName))
	} else {
		flds[fgeom] = fmt.Sprintf("ST_AsBinary(%v)", l.GeomFieldName)
	}
	if !fgid {
		flds = append(flds, l.IDFieldName)
	}
	selectClause := strings.Join(flds, ",")
	l.SQL = fmt.Sprintf(stdSQL, selectClause, tblname, l.GeomFieldName)
	log.Printf("The SQL for layer %v is %v.", l.Name, l.SQL)
	return nil
}

// NewProvider Setups and returns a new postgis provide or an error; if something
// is wrong. The function will validate that the config object looks good before
// trying to create a driver. This means that the Provider expects the following
// fields to exists in the provided map[string]interface{} map.
// host string — the host to connect to.
// port uint16 — the port to connect on.
// database string — the database name
// user string — the user name
// password string — the Password
// max_connections *uint8 // Default is 5 if nil, 0 means no max.
// layers map[string]struct{ — This is map of layers keyed by the layer name.
//     tablename string || sql string — This is the sql to use or the tablename to use with the default query.
//     fields []string — This is a list, if this is nil or empty we will get all fields.
//     geometry_fieldname string — This is the field name of the geometry, if it's an empty string or nil, it will defaults to 'geom'.
//     id_fieldname string — This is the field name for the id property, if it's an empty string or nil, it will defaults to 'gid'.
//  }
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

	var srid = int(DefaultSRID)
	if srid, err = c.Int(ConfigKeySRID, &srid); err != nil {
		return nil, err
	}

	p := Provider{
		srid: srid,
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
			return nil, fmt.Errorf("For layer(%v) we got the following error trying to get the layer's name field: %v", i, err)
		}
		if j, ok := lyrsSeen[lname]; ok {
			return nil, fmt.Errorf("%v layer name is duplicated in both layer %v and layer %v", lname, i, j)
		}
		lyrsSeen[lname] = i

		fields, err := vc.StringSlice(ConfigKeyFields)
		if err != nil {
			return nil, fmt.Errorf("For layer(%v) %v %v field had the following error: %v", i, lname, ConfigKeyFields, err)
		}
		geomfld := "geom"
		geomfld, err = vc.String(ConfigKeyGeomField, &geomfld)
		if err != nil {
			return nil, fmt.Errorf("For layer(%v) %v : %v", i, lname, err)
		}
		idfld := "gid"
		idfld, err = vc.String(ConfigKeyGeomIDField, &idfld)
		if err != nil {
			return nil, fmt.Errorf("For layer(%v) %v : %v", i, lname, err)
		}
		if idfld == geomfld {
			return nil, fmt.Errorf("For layer(%v) %v: %v (%v) and %v field (%v) is the same!", i, lname, ConfigKeyGeomField, geomfld, ConfigKeyGeomIDField, idfld)
		}

		var tblName string
		tblName, err = vc.String(ConfigKeyTablename, &tblName)
		if err != nil {
			return nil, fmt.Errorf("for %v layer(%v) %v has an error: %v", i, lname, ConfigKeyTablename, err)
		}
		var sql string

		sql, err = vc.String(ConfigKeySQL, &sql)

		if err != nil {
			return nil, fmt.Errorf("for %v layer(%v) %v has an error: %v", i, lname, ConfigKeySQL, err)
		}

		if tblName == "" && sql == "" {
			return nil, fmt.Errorf("The %v or %v field for layer(%v) %v must be specified.", ConfigKeyTablename, ConfigKeySQL, i, lname)
		}
		if tblName != "" && sql != "" {
			log.Printf("Both %v and %v field are specified for layer(%v) %v, using only %v field.", ConfigKeyTablename, ConfigKeySQL, i, lname)
		}

		l := layer{
			Name:          lname,
			IDFieldName:   idfld,
			GeomFieldName: geomfld,
		}
		if sql != "" {
			// We need to make sure that the sql has a BBOX for the bounding box env.
			if !strings.Contains(sql, BBOX) {
				return nil, fmt.Errorf("SQL for layer(%v) %v does not contain "+BBOX+", entry.", i, lname)
			}
			if !strings.Contains(sql, "*") {
				if !strings.Contains(sql, geomfld) {
					return nil, fmt.Errorf("SQL for layer(%v) %v does not contain the geometry field: %v", i, lname, geomfld)
				}
				if !strings.Contains(sql, idfld) {
					return nil, fmt.Errorf("SQL for layer(%v) %v does not contain the id field for the geometry: %v", i, lname, idfld)
				}
			}
			l.SQL = sql
		} else {
			// Tablename and Fields will be used to
			// We need to do some work. We need to check to see Fields contains the geom and gid fields
			// and if not add them to the list. If Fields list is empty/nil we will use '*' for the field
			// list.
			l.genSQL(p.pool, tblName, fields)
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

func (p Provider) MVTLayer(layerName string, tile tegola.Tile, tags map[string]interface{}) (layer *mvt.Layer, err error) {
	textent := tile.Extent()
	bbox := fmt.Sprintf("ST_MakeEnvelope(%v,%v,%v,%v,%v)", textent.Minx, textent.Miny, textent.Maxx, textent.Maxy, p.srid)
	plyr, ok := p.layers[layerName]
	if !ok {
		return nil, fmt.Errorf("Don't know of the layer %v", layerName)
	}
	sql := strings.Replace(plyr.SQL, BBOX, bbox, -1)

	layer = new(mvt.Layer)
	layer.Name = layerName

	rows, err := p.pool.Query(sql)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	fdescs := rows.FieldDescriptions()
	var geobytes []byte

	for rows.Next() {
		var geom tegola.Geometry
		var gid uint64
		vals, err := rows.Values()
		if err != nil {
			return nil, fmt.Errorf("Got an error trying to run SQL: %v ; %v", sql, err)
		}
		gtags := make(map[string]interface{})
		for i, v := range vals {
			switch fdescs[i].Name {
			case plyr.GeomFieldName:
				if geobytes, ok = v.([]byte); !ok {
					return nil, fmt.Errorf("Was unable to convert geometry field(%v) into bytes for layer %v.", plyr.GeomFieldName, layerName)
				}
				if geom, err = wkb.DecodeBytes(geobytes); err != nil {
					return nil, fmt.Errorf("Was unable to decode geometry field(%v) into wkb for layer %v.", plyr.GeomFieldName, layerName)
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
					return nil, fmt.Errorf("Unable to convert geometry ID field(%v) into a uint64 for layer %v", plyr.IDFieldName, layerName)
				}
			default:
				gtags[fdescs[i].Name] = vals[i]
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
