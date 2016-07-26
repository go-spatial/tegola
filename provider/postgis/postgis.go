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

	maxcon := int64(DefaultMaxConn)
	if maxcon, err = c.Int64(ConfigKeyMaxConn, &maxcon); err != nil {
		return nil, err
	}

	var srid = int64(DefaultSRID)
	if srid, err = c.Int64(ConfigKeySRID, &srid); err != nil {
		return nil, err
	}

	layers, ok := c[ConfigKeyLayers].([]map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("Expected %v to be a []map[string]interface{}. Value is of type %T", ConfigKeyLayers, c[ConfigKeyLayers])
	}

	lyrs := make(map[string]layer)
	zerostr := ""

	seenlyrs := make(map[string]int)

	for i, v := range layers {

		vc := dict.M(v)

		lname, err := vc.String(ConfigKeyLayerName, nil)
		if err != nil {
			return nil, fmt.Errorf("for %v layer(%v)  has an error: %v", i, ConfigKeyLayerName, err)
		}

		// Check to see if we have seen this name before.
		if at, ok := seenlyrs[lname]; ok {
			return nil, fmt.Errorf("Already saw %v(%v) for layer(%v) at Layer(%v) ", ConfigKeyLayerName, lname, i, at)
		}
		seenlyrs[lname] = i

		tblName, err := vc.String(ConfigKeyTablename, &zerostr)
		if err != nil {
			return nil, fmt.Errorf("for %v layer(%v) %v has an error: %v", i, lname, ConfigKeyTablename, err)
		}
		sql, err := vc.String(ConfigKeySQL, &zerostr)
		if err != nil {
			return nil, fmt.Errorf("for %v layer(%v) %v has an error: %v", i, lname, ConfigKeySQL, err)
		}
		if tblName == "" && sql == "" {
			return nil, fmt.Errorf("The %v or %v field for layer(%v) %v must be specified.", ConfigKeyTablename, ConfigKeySQL, i, lname)
		}
		if tblName != "" && sql != "" {
			log.Printf("Both %v and %v field are specified for layer(%v) %v, using only %[2]v field.", ConfigKeyTablename, ConfigKeySQL, i, lname)
		}

		fields, err := vc.StringSlice(ConfigKeyFields)
		if err != nil {
			return nil, fmt.Errorf("For layer(%v) %v %v field had the following error: %v", i, lname, ConfigKeyFields, err)
		}
		fld := "geom"
		geomfld, err := vc.String(ConfigKeyGeomField, &fld)
		if err != nil {
			return nil, fmt.Errorf("For layer(%v) %v : %v", i, lname, err)
		}
		fld = "gid"
		idfld, err := vc.String(ConfigKeyGeomIDField, &fld)
		if err != nil {
			return nil, fmt.Errorf("For layer(%v) %v : %v", i, lname, err)
		}
		if idfld == geomfld {
			return nil, fmt.Errorf("For layer(%v) %v: %v (%v) and %v field (%v) is the same!", i, lname, ConfigKeyGeomField, geomfld, ConfigKeyGeomIDField, idfld)
		}
		var lsql string
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
			lsql = sql
		} else {
			// Tablename and Fields will be used to
			// We need to do some work. We need to check to see Fields contains the geom and gid fields
			// and if not add them to the list. If Fields list is empty/nil we will use '*' for the field
			// list.
			selectClause := "*"
			if len(fields) != 0 {
				var fgeom, fgid bool
				for _, f := range fields {
					if f == geomfld {
						fgeom = true
					}
					if f == idfld {
						fgid = true
					}
				}
				if !fgeom {
					fields = append(fields, geomfld)
				}
				if !fgid {
					fields = append(fields, idfld)
				}
				selectClause = strings.Join(fields, ",")
			}
			lsql = fmt.Sprintf(stdSQL, selectClause, tblName, geomfld)
		}
		lyrs[lname] = layer{
			SQL:           lsql,
			IDFieldName:   idfld,
			GeomFieldName: geomfld,
		}
	}
	p := Provider{
		srid:   int(srid),
		layers: lyrs,
		config: pgx.ConnPoolConfig{
			ConnConfig: pgx.ConnConfig{
				Host:     host,
				Port:     uint16(port),
				Database: db,
				User:     user,
				Password: password,
			},
			MaxConnections: int(maxcon),
		},
	}
	if p.pool, err = pgx.NewConnPool(p.config); err != nil {
		return nil, fmt.Errorf("Failed while creating connection pool: %v", err)
	}

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
				if gid, ok = v.(uint64); !ok {
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
