package postgis

import (
	"fmt"

	"github.com/go-spatial/tegola/dict"
	"github.com/jackc/pgx"
)

type featureTableMetaData struct {
	geomColname string
	geomSrid    int64
	primaryKey  string
	propCols    []string
}

func propertycols(conn *pgx.Conn, tablename string, md *featureTableMetaData) (propcols []string, err error) {
	// --- Get table property columns (currently all columns except pk & geometry)
	sql := fmt.Sprintf("SELECT * FROM %v LIMIT 1;", tablename)

	rows, err := conn.Query(sql)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	fds := rows.FieldDescriptions()
	propcols = make([]string, 0, 15)
	for _, fd := range fds {
		// Skip primary key & geom column for use as properties
		if md.primaryKey == fd.Name || md.geomColname == fd.Name {
			continue
		}
		propcols = append(propcols, fd.Name)
	}

	return propcols, nil
}

func metadata(conn *pgx.Conn) (md map[string]*featureTableMetaData, err error) {
	// --- Get geometry metadata for all feature tables
	sql := "SELECT f_table_name, f_geometry_column, srid FROM geometry_columns ORDER BY f_table_name;"

	rows, err := conn.Query(sql)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	md = make(map[string]*featureTableMetaData)

	for rows.Next() {
		var tablename string
		var geomcol string
		var srid int64
		err = rows.Scan(&tablename, &geomcol, &srid)
		if err != nil {
			return nil, fmt.Errorf("error running SQL: %v ; %v", sql, err)
		}

		md[tablename] = &featureTableMetaData{}
		md[tablename].geomColname = geomcol
		md[tablename].geomSrid = srid
	}

	// --- Get table primary keys
	sql = `SELECT t.table_name, c.column_name
          FROM information_schema.key_column_usage AS c
          LEFT JOIN information_schema.table_constraints AS t
          ON t.constraint_name = c.constraint_name
          WHERE t.constraint_type = 'PRIMARY KEY';`
	rows, err = conn.Query(sql)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var tablename string
		var pk string
		rows.Scan(&tablename, &pk)
		md[tablename].primaryKey = pk
	}

	// --- Get table property columns
	for tablename, tablemd := range md {
		tablemd.propCols, err = propertycols(conn, tablename, tablemd)
		if err != nil {
			return nil, fmt.Errorf("problem getting property columns: %v", err)
		}
	}

	return md, nil
}

func AutoConfig(connstr string) (dict.Dicter, error) {
	cc, err := pgx.ParseConnectionString(connstr)
	if err != nil {
		return nil, err
	}

	conn, err := pgx.Connect(cc)
	if err != nil {
		return nil, fmt.Errorf("unable to connect: %v", err)
	}
	defer conn.Close()

	md, err := metadata(conn)
	if err != nil {
		return nil, fmt.Errorf("problem getting metadata: %v", err)
	}

	conf := make(map[string]interface{})
	conf["name"] = "autoconfd_postgis"
	conf["type"] = Name
	conf[ConfigKeyHost] = cc.Host
	conf[ConfigKeyPort] = int64(cc.Port)
	conf[ConfigKeyDB] = cc.Database
	conf[ConfigKeyUser] = cc.User
	conf[ConfigKeyPassword] = cc.Password

	conf[ConfigKeyLayers] = make([]map[string]interface{}, 0, len(md))
	for tablename, tablemd := range md {
		// *** TODO: Currently can't handle zeroes in srid field, like in the osm database.
		if tablemd.geomSrid == 0 {
			continue
		}
		// layer config
		lconf := make(map[string]interface{})
		lconf[ConfigKeyLayerName] = tablename
		lconf[ConfigKeyTablename] = tablename
		if md[tablename].primaryKey != "" {
			lconf[ConfigKeyGeomIDField] = tablemd.primaryKey
		}
		lconf[ConfigKeySRID] = tablemd.geomSrid
		lconf[ConfigKeyGeomField] = tablemd.geomColname
		lconf[ConfigKeyFields] = tablemd.propCols

		conf[ConfigKeyLayers] = append(conf["layers"].([]map[string]interface{}), lconf)
	}

	// TODO: Setting the srid at this level doesn't make sense, as it can be different for different layers
	clayers := conf[ConfigKeyLayers].([]map[string]interface{})
	if len(clayers) == 0 {
		conf[ConfigKeySRID] = int64(0)
	} else {
		// Use the SRID of the first layer
		conf[ConfigKeySRID] = clayers[0][ConfigKeySRID]
	}
	dconf := dict.Dict(conf)
	return dconf, nil
}
