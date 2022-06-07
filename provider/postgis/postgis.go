package postgis

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/go-spatial/geom"
	"github.com/go-spatial/geom/encoding/wkb"
	"github.com/go-spatial/tegola"
	conf "github.com/go-spatial/tegola/config"
	"github.com/go-spatial/tegola/dict"
	"github.com/go-spatial/tegola/internal/log"
	"github.com/go-spatial/tegola/observability"
	"github.com/go-spatial/tegola/provider"
	"github.com/jackc/pgproto3/v2"
	"github.com/jackc/pgtype"
	gofrs "github.com/jackc/pgtype/ext/gofrs-uuid"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
)

const Name = "postgis"

type connectionPoolCollector struct {
	*pgxpool.Pool
	maxConnectionDesc        *prometheus.Desc
	currentConnectionsDesc   *prometheus.Desc
	availableConnectionsDesc *prometheus.Desc
}

func (c connectionPoolCollector) Describe(ch chan<- *prometheus.Desc) {
	prometheus.DescribeByCollect(c, ch)
}

func (c connectionPoolCollector) Collect(ch chan<- prometheus.Metric) {
	if c.Pool == nil {
		return
	}
	stat := c.Pool.Stat()
	ch <- prometheus.MustNewConstMetric(
		c.maxConnectionDesc,
		prometheus.GaugeValue,
		float64(stat.MaxConns()),
	)
	ch <- prometheus.MustNewConstMetric(
		c.currentConnectionsDesc,
		prometheus.GaugeValue,
		float64(stat.AcquiredConns()),
	)
	ch <- prometheus.MustNewConstMetric(
		c.availableConnectionsDesc,
		prometheus.GaugeValue,
		float64(stat.MaxConns()-stat.AcquiredConns()),
	)
}

func (c *connectionPoolCollector) Collectors(prefix string, _ func(configKey string) map[string]interface{}) ([]observability.Collector, error) {
	if c == nil {
		return nil, nil
	}
	if prefix != "" && !strings.HasSuffix(prefix, "_") {
		prefix = prefix + "_"
	}

	c.maxConnectionDesc = prometheus.NewDesc(
		prefix+"postgres_max_connections",
		"Max number of postgres connections in the pool",
		nil,
		nil,
	)

	c.currentConnectionsDesc = prometheus.NewDesc(
		prefix+"postgres_current_connections",
		"Current number of postgres connections in the pool",
		nil,
		nil,
	)

	c.availableConnectionsDesc = prometheus.NewDesc(
		prefix+"postgres_available_connections",
		"Current number of available postgres connections in the pool",
		nil,
		nil,
	)
	return []observability.Collector{c}, nil
}

// Provider provides the postgis data provider.
type Provider struct {
	config pgxpool.Config
	pool   *connectionPoolCollector
	// map of layer name and corresponding sql
	layers     map[string]Layer
	srid       uint64
	firstLayer string

	// collectorsRegistered keeps track if we have already collectorsRegistered these collectors
	// as the Collectors function will be called for each map and layer, but
	// we are going to assign those during runtime, instead of at registration
	// time; so we will only return these collectors on the first call.
	collectorsRegistered bool

	// Collectors for Query times
	mvtProviderQueryHistogramSeconds *prometheus.HistogramVec
	queryHistogramSeconds            *prometheus.HistogramVec
}

func (p *Provider) Collectors(prefix string, cfgFn func(configKey string) map[string]interface{}) ([]observability.Collector, error) {
	if p.collectorsRegistered {
		return nil, nil
	}

	buckets := []float64{.1, 1, 5, 20}
	collectors, err := p.pool.Collectors(prefix, cfgFn)
	if err != nil {
		return nil, err
	}

	p.mvtProviderQueryHistogramSeconds = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    prefix + "_mvt_provider_sql_query_seconds",
			Help:    "A histogram of query time for sql for mvt providers",
			Buckets: buckets,
		},
		[]string{"map_name", "z"},
	)

	p.queryHistogramSeconds = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    prefix + "_provider_sql_query_seconds",
			Help:    "A histogram of query time for sql for providers",
			Buckets: buckets,
		},
		[]string{"map_name", "layer_name", "z"},
	)

	p.collectorsRegistered = true
	return append(collectors, p.mvtProviderQueryHistogramSeconds, p.queryHistogramSeconds), nil
}

const (
	// We quote the field and table names to prevent colliding with postgres keywords.
	stdSQL = `SELECT %[1]v FROM %[2]v WHERE "%[3]v" && ` + conf.BboxToken
	mvtSQL = `SELECT %[1]v FROM %[2]v`

	// SQL to get the column names, without hitting the information_schema. Though it might be better to hit the information_schema.
	fldsSQL = `SELECT * FROM %[1]v LIMIT 0;`
)

const (
	DefaultURI             = ""
	DefaultPort            = 5432
	DefaultSRID            = tegola.WebMercator
	DefaultMaxConn         = 100
	DefaultMaxConnIdleTime = "30m"
	DefaultMaxConnLifetime = "1h"
	DefaultSSLMode         = "prefer"
	DefaultSSLKey          = ""
	DefaultSSLCert         = ""
)

const (
	ConfigKeyURI             = "uri"
	ConfigKeyHost            = "host"
	ConfigKeyPort            = "port"
	ConfigKeyDB              = "database"
	ConfigKeyUser            = "user"
	ConfigKeyPassword        = "password"
	ConfigKeySSLMode         = "ssl_mode"
	ConfigKeySSLKey          = "ssl_key"
	ConfigKeySSLCert         = "ssl_cert"
	ConfigKeySSLRootCert     = "ssl_root_cert"
	ConfigKeyMaxConn         = "max_connections"
	ConfigKeyMaxConnIdleTime = "max_connection_idle_time"
	ConfigKeyMaxConnLifetime = "max_connection_lifetime"
	ConfigKeySRID            = "srid"
	ConfigKeyLayers          = "layers"
	ConfigKeyLayerName       = "name"
	ConfigKeyTablename       = "tablename"
	ConfigKeySQL             = "sql"
	ConfigKeyFields          = "fields"
	ConfigKeyGeomField       = "geometry_fieldname"
	ConfigKeyGeomIDField     = "id_fieldname"
	ConfigKeyGeomType        = "geometry_type"
)

// isSelectQuery is a regexp to check if a query starts with `SELECT`,
// case-insensitive and ignoring any preceding whitespace and SQL comments.
var isSelectQuery = regexp.MustCompile(`(?i)^((\s*)(--.*\n)?)*select`)

type hstoreOID struct {
	OID     uint32
	hasInit bool
}

// validateURI validates for minimum requirements for a valid postgresql uri
func validateURI(u string) error {
	uri, err := url.Parse(u)
	if err != nil {
		return ErrInvalidURI{Err: err}
	}

	if uri.Scheme != "postgres" && uri.Scheme != "postgresql" {
		return ErrInvalidURI{
			Msg: fmt.Sprintf("invalid connection scheme (%v)", uri.Scheme),
		}
	}

	if uri.User == nil {
		return ErrInvalidURI{Msg: "auth credentials missing"}
	}

	host, port, err := net.SplitHostPort(uri.Host)
	if err != nil {
		return ErrInvalidURI{
			Err: fmt.Errorf("splitting host port error: %w", err),
		}
	}

	if host == "" {
		return ErrInvalidURI{
			Msg: fmt.Sprintf("address %v:%v: missing host in address", host, port),
		}
	}

	if uri.Path == "" {
		return ErrInvalidURI{Msg: "missing database"}
	}

	return nil
}

// TODO: (iwpnd) to be removed/refactored in v0.17.0
// BuildURI creates a database URI from config
func BuildURI(config dict.Dicter) (*url.URL, *url.Values, error) {

	sslmode := DefaultSSLMode
	sslmode, err := config.String(ConfigKeySSLMode, &sslmode)
	if err != nil {
		return nil, nil, err
	}

	uri := DefaultURI
	uri, err = config.String(ConfigKeyURI, &uri)
	if err != nil {
		return nil, nil, err
	}

	// if uri is set in the config, we add sslmode and return early
	if uri != "" {
		if err := validateURI(uri); err != nil {
			return nil, nil, err
		}

		parsedUri, err := url.Parse(uri)
		if err != nil {
			return nil, nil, err
		}

		// parse query to make sure sslmode is attached
		parsedQuery, err := url.ParseQuery(parsedUri.RawQuery)
		if err != nil {
			return &url.URL{}, nil, err
		}

		if ok := parsedQuery.Get("sslmode"); ok == "" {
			parsedQuery.Add("sslmode", sslmode)
		}

		parsedUri.RawQuery = parsedQuery.Encode()

		return parsedUri, &parsedQuery, nil
	}

	host, err := config.String(ConfigKeyHost, nil)
	if err != nil {
		return nil, nil, err
	}

	port := DefaultPort
	if port, err = config.Int(ConfigKeyPort, &port); err != nil {
		return nil, nil, err
	}

	db, err := config.String(ConfigKeyDB, nil)
	if err != nil {
		return nil, nil, err
	}

	user, err := config.String(ConfigKeyUser, nil)
	if err != nil {
		return nil, nil, err
	}

	password, err := config.String(ConfigKeyPassword, nil)
	if err != nil {
		return nil, nil, err
	}

	maxcon := DefaultMaxConn
	if maxcon, err = config.Int(ConfigKeyMaxConn, &maxcon); err != nil {
		return nil, nil, err
	}

	idletime := DefaultMaxConnIdleTime
	if idletime, err = config.String(ConfigKeyMaxConnIdleTime, &idletime); err != nil {
		return nil, nil, err
	}

	lifetime := DefaultMaxConnLifetime
	if lifetime, err = config.String(ConfigKeyMaxConnLifetime, &lifetime); err != nil {
		return nil, nil, err
	}

	params := &url.Values{}
	params.Add("sslmode", sslmode)
	params.Add("pool_max_conns", fmt.Sprintf("%v", maxcon))
	params.Add("pool_max_conn_lifetime", lifetime)
	params.Add("pool_max_conn_idle_time", idletime)

	if uri == "" {
		log.Warn("Connecting to PostGIS with connection parameters is deprecated. Use 'uri' instead.")
	}

	u := &url.URL{
		Scheme:   "postgres",
		Host:     fmt.Sprintf("%v:%v", host, port),
		User:     url.UserPassword(user, password),
		Path:     db,
		RawQuery: params.Encode(),
	}

	return u, params, nil
}

// BuildDBConfig build db config with defaults
func BuildDBConfig(uri string) (*pgxpool.Config, error) {
	dbconfig, err := pgxpool.ParseConfig(uri)
	if err != nil {
		return nil, err
	}

	dbconfig.ConnConfig.LogLevel = pgx.LogLevelWarn
	dbconfig.ConnConfig.RuntimeParams = map[string]string{
		"default_transaction_read_only": "TRUE",
		"application_name":              "tegola",
	}

	var hstore hstoreOID

	dbconfig.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
		// The AfterConnect call runs everytime a new connection is acquired,
		// including everytime the connection pool expands. The hstore OID
		// is not constant, so we lookup the OID once per provider and store it.
		// Extensions have to be registered for every new connection.

		if !hstore.hasInit {
			row := conn.QueryRow(ctx, "SELECT oid FROM pg_type WHERE typname = 'hstore';")
			if err = row.Scan(&hstore.OID); err != nil {
				switch {
				case errors.Is(err, pgx.ErrNoRows):
					// do nothing, because query can be empty if hstore is not installed
					break
				default:
					return fmt.Errorf("error fetching hstore oid: %w", err)
				}
			}
			hstore.hasInit = true
		}

		// dont register hstore data type if hstore extension is not installed
		if hstore.OID != 0 {
			conn.ConnInfo().RegisterDataType(pgtype.DataType{
				Value: &pgtype.Hstore{},
				Name:  "hstore",
				OID:   hstore.OID,
			})
		}

		// register UUID type, see https://github.com/jackc/pgx/wiki/UUID-Support
		conn.ConnInfo().RegisterDataType(pgtype.DataType{
			Value: &gofrs.UUID{},
			Name:  "uuid",
			OID:   pgtype.UUIDOID,
		})

		return nil
	}

	return dbconfig, nil
}

// CreateProvider instantiates and returns a new postgis provider or an error.
// The function will validate that the config object looks good before
// trying to create a driver. This Provider supports the following fields
// in the provided map[string]interface{} map:
//
//	host (string): [Required] postgis database host
//	port (int): [Required] postgis database port (required)
//	database (string): [Required] postgis database name
//	user (string): [Required] postgis database user
//	password (string): [Required] postgis database password
//	srid (int): [Optional] The default SRID for the provider. Defaults to WebMercator (3857) but also supports WGS84 (4326)
//	max_connections : [Optional] The max connections to maintain in the connection pool. Default is 100. 0 means no max.
//	layers (map[string]struct{})  — This is map of layers keyed by the layer name. supports the following properties
//
//		name (string): [Required] the name of the layer. This is used to reference this layer from map layers.
//		tablename (string): [*Required] the name of the database table to query against. Required if sql is not defined.
//		geometry_fieldname (string): [Optional] the name of the filed which contains the geometry for the feature. defaults to geom
//		id_fieldname (string): [Optional] the name of the feature id field. defaults to gid
//		fields ([]string): [Optional] a list of fields to include alongside the feature. Can be used if sql is not defined.
//		srid (int): [Optional] the SRID of the layer. Supports 3857 (WebMercator) or 4326 (WGS84).
//		sql (string): [*Required] custom SQL to use use. Required if tablename is not defined. Supports the following tokens:
//
//			!BBOX! - [Required] will be replaced with the bounding box of the tile before the query is sent to the database.
//			!ZOOM! - [Optional] will be replaced with the "Z" (zoom) value of the requested tile.
func CreateProvider(config dict.Dicter, providerType string) (*Provider, error) {
	uri, params, err := BuildURI(config)
	if err != nil {
		return nil, err
	}

	sslmode := params.Get("sslmode")

	sslkey := DefaultSSLKey
	sslkey, err = config.String(ConfigKeySSLKey, &sslkey)
	if err != nil {
		return nil, err
	}

	sslcert := DefaultSSLCert
	sslcert, err = config.String(ConfigKeySSLCert, &sslcert)
	if err != nil {
		return nil, err
	}

	sslrootcert := DefaultSSLCert
	sslrootcert, err = config.String(ConfigKeySSLRootCert, &sslrootcert)
	if err != nil {
		return nil, err
	}

	dbconfig, err := BuildDBConfig(uri.String())
	if err != nil {
		return nil, fmt.Errorf("failed while building db config: %v", err)
	}

	srid := DefaultSRID
	if srid, err = config.Int(ConfigKeySRID, &srid); err != nil {
		return nil, err
	}

	if err = ConfigTLS(sslmode, sslkey, sslcert, sslrootcert, dbconfig); err != nil {
		return nil, err
	}

	p := Provider{
		srid:   uint64(srid),
		config: *dbconfig,
	}

	pool, err := pgxpool.ConnectConfig(context.Background(), &p.config)
	if err != nil {
		return nil, fmt.Errorf("failed while creating connection pool: %v", err)
	}
	p.pool = &connectionPoolCollector{Pool: pool}

	layers, err := config.MapSlice(ConfigKeyLayers)
	if err != nil {
		return nil, err
	}

	lyrs := make(map[string]Layer)
	lyrsSeen := make(map[string]int)

	for i, layer := range layers {

		lName, err := layer.String(ConfigKeyLayerName, nil)
		if err != nil {
			return nil, fmt.Errorf("for layer (%v) we got the following error trying to get the layer's name field: %v", i, err)
		}

		if j, ok := lyrsSeen[lName]; ok {
			return nil, fmt.Errorf("%v layer name is duplicated in both layer %v and layer %v", lName, i, j)
		}

		lyrsSeen[lName] = i
		if i == 0 {
			p.firstLayer = lName
		}

		fields, err := layer.StringSlice(ConfigKeyFields)
		if err != nil {
			return nil, fmt.Errorf("for layer (%v) %v %v field had the following error: %w", i, lName, ConfigKeyFields, err)
		}

		geomfld := "geom"
		geomfld, err = layer.String(ConfigKeyGeomField, &geomfld)
		if err != nil {
			return nil, fmt.Errorf("for layer (%v) %v : %w", i, lName, err)
		}

		idfld := ""
		idfld, err = layer.String(ConfigKeyGeomIDField, &idfld)
		if err != nil {
			return nil, fmt.Errorf("for layer (%v) %v : %w", i, lName, err)
		}
		if idfld == geomfld {
			return nil, fmt.Errorf("for layer (%v) %v: %v (%v) and %v field (%v) is the same", i, lName, ConfigKeyGeomField, geomfld, ConfigKeyGeomIDField, idfld)
		}

		geomType := ""
		geomType, err = layer.String(ConfigKeyGeomType, &geomType)
		if err != nil {
			return nil, fmt.Errorf("for layer (%v) %v : %w", i, lName, err)
		}

		var tblName string
		tblName, err = layer.String(ConfigKeyTablename, &lName)
		if err != nil {
			return nil, fmt.Errorf("for %v layer (%v) %v has an error: %w", i, lName, ConfigKeyTablename, err)
		}

		var sql string
		sql, err = layer.String(ConfigKeySQL, &sql)
		if err != nil {
			return nil, fmt.Errorf("for %v layer (%v) %v has an error: %w", i, lName, ConfigKeySQL, err)
		}

		if tblName != lName && sql != "" {
			log.Debugf("both %v and %v field are specified for layer (%v) %v, using only %[2]v field.", ConfigKeyTablename, ConfigKeySQL, i, lName)
		}

		var lsrid = srid
		if lsrid, err = layer.Int(ConfigKeySRID, &lsrid); err != nil {
			return nil, err
		}

		l := Layer{
			name:      lName,
			idField:   idfld,
			geomField: geomfld,
			srid:      uint64(lsrid),
		}

		if sql != "" && !isSelectQuery.MatchString(sql) {
			// if it is not a SELECT query, then we assume we have a sub-query
			// (`(select ...) as foo`) which we can handle like a tablename
			tblName = sql
			sql = ""
		}

		if sql != "" {
			// convert !BOX! (MapServer) and !bbox! (Mapnik) to !BBOX! for compatibility
			sql := strings.Replace(strings.Replace(sql, "!BOX!", "!BBOX!", -1), "!bbox!", "!BBOX!", -1)
			// make sure that the sql has a !BBOX! token
			if !strings.Contains(sql, conf.BboxToken) {
				return nil, fmt.Errorf("SQL for layer (%v) %v is missing required token: %v", i, lName, conf.BboxToken)
			}
			if !strings.Contains(sql, "*") {
				if !strings.Contains(sql, geomfld) {
					return nil, fmt.Errorf("SQL for layer (%v) %v does not contain the geometry field: %v", i, lName, geomfld)
				}
				if !strings.Contains(sql, idfld) {
					return nil, fmt.Errorf("SQL for layer (%v) %v does not contain the id field for the geometry: %v", i, lName, sql)
				}
			}

			l.sql = sql
		} else {
			// Tablename and Fields will be used to build the query.
			// We need to do some work. We need to check to see Fields contains the geom and gid fields
			// and if not add them to the list. If Fields list is empty/nil we will use '*' for the field list.
			l.sql, err = genSQL(&l, p.pool, tblName, fields, true, providerType)
			if err != nil {
				return nil, fmt.Errorf("could not generate sql, for layer(%v): %w", lName, err)
			}
		}

		if debugLayerSQL {
			log.Debugf("SQL for Layer(%v):\n%v\n", lName, l.sql)
		}

		// set the layer geom type
		if geomType != "" {
			if err = p.setLayerGeomType(&l, geomType); err != nil {
				return nil, fmt.Errorf("error fetching geometry type for layer (%v): %w", l.name, err)
			}
		} else {
			if err = p.inspectLayerGeomType(&l); err != nil {
				return nil, fmt.Errorf("error fetching geometry type for layer (%v): %w", l.name, err)
			}
		}

		lyrs[lName] = l
	}
	p.layers = lyrs

	// track the provider so we can clean it up later
	providers = append(providers, p)

	return &p, nil
}

// ConfigTLS is derived from github.com/jackc/pgx configTLS (https://github.com/jackc/pgx/blob/master/conn.go)
func ConfigTLS(sslMode string, sslKey string, sslCert string, sslRootCert string, cc *pgxpool.Config) error {

	switch sslMode {
	case "disable":
		cc.ConnConfig.TLSConfig = nil
		return nil
	case "allow":
		cc.ConnConfig.TLSConfig = &tls.Config{InsecureSkipVerify: true}
	case "prefer":
		cc.ConnConfig.TLSConfig = &tls.Config{InsecureSkipVerify: true}
	case "require":
		cc.ConnConfig.TLSConfig = &tls.Config{InsecureSkipVerify: true}
	case "verify-ca", "verify-full":
		cc.ConnConfig.TLSConfig = &tls.Config{
			ServerName: cc.ConnConfig.Host,
		}
	default:
		return ErrInvalidSSLMode(sslMode)
	}

	if sslRootCert != "" {
		caCertPool := x509.NewCertPool()

		caCert, err := ioutil.ReadFile(sslRootCert)
		if err != nil {
			return fmt.Errorf("unable to read CA file (%q): %w", sslRootCert, err)
		}

		if !caCertPool.AppendCertsFromPEM(caCert) {
			return fmt.Errorf("unable to add CA to cert pool")
		}

		cc.ConnConfig.TLSConfig.RootCAs = caCertPool
		cc.ConnConfig.TLSConfig.ClientCAs = caCertPool
	}

	if (sslCert == "") != (sslKey == "") {
		return fmt.Errorf("both 'sslcert' and 'sslkey' are required")
	} else if sslCert != "" { // we must have both now
		cert, err := tls.LoadX509KeyPair(sslCert, sslKey)
		if err != nil {
			return fmt.Errorf("unable to read cert: %w", err)
		}

		cc.ConnConfig.TLSConfig.Certificates = []tls.Certificate{cert}
	}

	return nil
}

// setLayerGeomType sets the geomType field on the layer to one of point,
// linestring, polygon, multipoint, multilinestring, multipolygon or
// geometrycollection
func (p Provider) setLayerGeomType(l *Layer, geomType string) error {
	switch strings.ToLower(geomType) {
	case "point":
		l.geomType = geom.Point{}
	case "linestring":
		l.geomType = geom.LineString{}
	case "polygon":
		l.geomType = geom.Polygon{}
	case "multipoint":
		l.geomType = geom.MultiPoint{}
	case "multilinestring":
		l.geomType = geom.MultiLineString{}
	case "multipolygon":
		l.geomType = geom.MultiPolygon{}
	case "geometrycollection":
		l.geomType = geom.Collection{}
	default:
		return fmt.Errorf("unsupported geometry_type (%v) for layer (%v)", geomType, l.name)
	}
	return nil
}

// inspectLayerGeomType sets the geomType field on the layer by running the SQL
// and reading the geom type in the result set
func (p Provider) inspectLayerGeomType(l *Layer) error {
	var err error

	// we want to know the geom type instead of returning the geom data so we modify the SQL
	// TODO (arolek): this strategy wont work if remove the requirement of wrapping ST_AsBinary(geom) in the SQL statements.
	//
	// https://github.com/go-spatial/tegola/issues/180
	//
	// case insensitive search

	re := regexp.MustCompile(`(?i)ST_AsBinary`)
	sql := re.ReplaceAllString(l.sql, "ST_GeometryType")

	re = regexp.MustCompile(`(?i)(ST_AsMVTGeom\(.*\))`)
	if re.MatchString(sql) {
		sql = fmt.Sprintf("SELECT ST_GeometryType(%v) FROM (%v) as q", l.geomField, sql)
	}

	// we only need a single result set to sniff out the geometry type
	sql = fmt.Sprintf("%v LIMIT 1", sql)

	// if a !ZOOM! token exists, all features could be filtered out so we don't have a geometry to inspect it's type.
	// address this by replacing the !ZOOM! token with an ANY statement which includes all zooms
	sql = strings.Replace(sql, "!ZOOM!", "ANY('{0,1,2,3,4,5,6,7,8,9,10,11,12,13,14,15,16,17,18,19,20,21,22,23,24}')", 1)

	// we need a tile to run our sql through the replacer
	tile := provider.NewTile(0, 0, 0, 64, tegola.WebMercator)

	// normal replacer
	sql, err = replaceTokens(sql, l, tile, true)
	if err != nil {
		return err
	}

	// TODO (bemyak): Figure out what is this
	// remove all parameter tokens for inspection
	sql = stripParams(sql)

	rows, err := p.pool.Query(context.Background(), sql)
	if err != nil {
		return err
	}
	defer rows.Close()

	// fetch rows FieldDescriptions. this gives us the OID for the data types returned to aid in decoding
	fdescs := rows.FieldDescriptions()
	for rows.Next() {

		vals, err := rows.Values()
		if err != nil {
			return fmt.Errorf("error running SQL: %v ; %w", sql, err)
		}

		// iterate the values returned from our row, sniffing for the geomField or st_geometrytype field name
		for i, v := range vals {
			switch string(fdescs[i].Name) {
			case l.geomField, "st_geometrytype":
				switch v {
				case "ST_Point":
					l.geomType = geom.Point{}
				case "ST_LineString":
					l.geomType = geom.LineString{}
				case "ST_Polygon":
					l.geomType = geom.Polygon{}
				case "ST_MultiPoint":
					l.geomType = geom.MultiPoint{}
				case "ST_MultiLineString":
					l.geomType = geom.MultiLineString{}
				case "ST_MultiPolygon":
					l.geomType = geom.MultiPolygon{}
				case "ST_GeometryCollection":
					l.geomType = geom.Collection{}
				default:
					return fmt.Errorf("layer (%v) returned unsupported geometry type (%v)", l.name, v)
				}
			}
		}
	}

	return rows.Err()
}

// Layer fetches an individual layer from the provider, if it's configured
// if no name is provider, the first layer is returned
func (p *Provider) Layer(name string) (Layer, bool) {
	if name == "" {
		return p.layers[p.firstLayer], true
	}

	layer, ok := p.layers[name]
	return layer, ok
}

// Layers returns meta data about the various layers which are configured with the provider
func (p Provider) Layers() ([]provider.LayerInfo, error) {
	var ls []provider.LayerInfo

	for i := range p.layers {
		ls = append(ls, p.layers[i])
	}

	return ls, nil
}

// TileFeatures adheres to the provider.Tiler interface
func (p Provider) TileFeatures(ctx context.Context, layer string, tile provider.Tile, fn func(f *provider.Feature) error) error {

	var mapName string
	{
		mapNameVal := ctx.Value(observability.ObserveVarMapName)
		if mapNameVal != nil {
			// if it's not convertible to a string, we will ignore it.
			mapName, _ = mapNameVal.(string)
		}
	}
	// fetch the provider layer
	plyr, ok := p.Layer(layer)
	if !ok {
		return ErrLayerNotFound{layer}
	}

	sql, err := replaceTokens(plyr.sql, &plyr, tile, true)
	if err := ctxErr(ctx, err); err != nil {
		return fmt.Errorf("error replacing layer tokens for layer (%v) SQL (%v): %w", layer, sql, err)
	}

	// replace configured query parameters if any
	args := make([]interface{}, 0)
	sql = replaceParams(queryParams, sql, &args)
	if err != nil {
		return err
	}

	if debugExecuteSQL {
		log.Debugf("TEGOLA_SQL_DEBUG:EXECUTE_SQL for layer (%v): %v with args %v", layer, sql, args)
	}

	// context check
	if err := ctx.Err(); err != nil {
		return err
	}

	now := time.Now()
	rows, err := p.pool.Query(ctx, sql, args...)
	if p.queryHistogramSeconds != nil {
		z, _, _ := tile.ZXY()
		lbls := prometheus.Labels{
			"z":          strconv.FormatUint(uint64(z), 10),
			"map_name":   mapName,
			"layer_name": layer,
		}
		p.queryHistogramSeconds.With(lbls).Observe(time.Since(now).Seconds())
	}
	// when using ctxErr, it's import to make sure the defer rows.Close()
	// statement happens before the error check. The context may have been
	// canceled, but rows were also returned. If we don't close the rows
	// the the provider can't clean up the pool and the process will hang
	// trying to clean itself up.
	defer rows.Close()
	if err := ctxErr(ctx, err); err != nil {
		return fmt.Errorf("error running layer (%v) SQL (%v) with args %v: %w", layer, sql, args, err)
	}

	// fieldDescriptions
	var fdescs []pgproto3.FieldDescription
	reportedLayerFieldName := ""
	for rows.Next() {
		// context check
		if err := ctx.Err(); err != nil {
			return err
		}

		// fetch rows FieldDescriptions. this gives us the OID for the data types
		// returned to aid in decoding. This only needs to be done once.
		if fdescs == nil {
			fdescs = rows.FieldDescriptions()
			// loop our field descriptions looking for the geometry field
			var geomFieldFound bool
			for i := range fdescs {
				if string(fdescs[i].Name) == plyr.GeomFieldName() {
					geomFieldFound = true
					break
				}
			}
			if !geomFieldFound {
				return ErrGeomFieldNotFound{
					GeomFieldName: plyr.GeomFieldName(),
					LayerName:     plyr.Name(),
				}
			}
		}

		// fetch row values
		vals, err := rows.Values()
		if err := ctxErr(ctx, err); err != nil {
			return fmt.Errorf("error running layer (%v) SQL (%v): %w", layer, sql, err)
		}

		gid, geobytes, tags, err := decipherFields(ctx, plyr.GeomFieldName(), plyr.IDFieldName(), fdescs, vals)
		if err := ctxErr(ctx, err); err != nil {
			return fmt.Errorf("for layer (%v) %w", plyr.Name(), err)
		}

		// check that we have geometry data. if not, skip the feature
		if len(geobytes) == 0 {
			continue
		}

		// decode our WKB
		geometry, err := wkb.DecodeBytes(geobytes)
		if err != nil {
			switch err.(type) {
			case wkb.ErrUnknownGeometryType:
				rplfn := layer + ":" + plyr.GeomFieldName()
				// Only report to the log once. This is to prevent the logs from filling up if there are many geometries in the layer
				if reportedLayerFieldName == "" || reportedLayerFieldName == rplfn {
					reportedLayerFieldName = rplfn
					log.Warnf("Ignoring unsupported geometry in layer (%v). Only basic 2D geometry type are supported. Try using `ST_Force2D(%v)`.", layer, plyr.GeomFieldName())
				}
				continue
			default:
				return fmt.Errorf("unable to decode layer (%v) geometry field (%v) into wkb where (%v = %v): %w", layer, plyr.GeomFieldName(), plyr.IDFieldName(), gid, err)
			}
		}

		feature := provider.Feature{
			ID:       gid,
			Geometry: geometry,
			SRID:     plyr.SRID(),
			Tags:     tags,
		}

		// pass the feature to the provided callback
		if err = fn(&feature); err != nil {
			return err
		}
	}

	return rows.Err()
}

func (p Provider) MVTForLayers(ctx context.Context, tile provider.Tile, layers []provider.Layer) ([]byte, error) {
	var (
		err     error
		sqls    = make([]string, 0, len(layers))
		mapName string
	)

	{
		mapNameVal := ctx.Value(observability.ObserveVarMapName)
		if mapNameVal != nil {
			// if it's not convertible to a string, we will ignore it.
			mapName, _ = mapNameVal.(string)
		}
	}

	args := make([]interface{}, 0)

	for i := range layers {
		if debug {
			log.Debugf("looking for layer: %v", layers[i])
		}
		l, ok := p.Layer(layers[i].Name)
		if !ok {
			// Should we error here, or have a flag so that we don't
			// spam the user?
			log.Warnf("provider layer not found %v", layers[i].Name)
		}
		if debugLayerSQL {
			log.Debugf("SQL for Layer(%v):\n%v\nargs:%v\n", l.Name(), l.sql, args)
		}
		sql, err := replaceTokens(l.sql, &l, tile, false)
		if err := ctxErr(ctx, err); err != nil {
			return nil, err
		}

		// replace configured query parameters if any
		sql = replaceParams(params, sql, &args)

		// ref: https://postgis.net/docs/ST_AsMVT.html
		// bytea ST_AsMVT(any_element row, text name, integer extent, text geom_name, text feature_id_name)

		var featureIDName string

		if l.IDFieldName() == "" {
			featureIDName = "NULL"
		} else {
			featureIDName = fmt.Sprintf(`'%s'`, l.IDFieldName())
		}

		sqls = append(sqls, fmt.Sprintf(
			`(SELECT ST_AsMVT(q,'%s',%d,'%s',%s) AS data FROM (%s) AS q)`,
			layers[i].MVTName,
			tegola.DefaultExtent,
			l.GeomFieldName(),
			featureIDName,
			sql,
		))
	}
	subsqls := strings.Join(sqls, "||")
	fsql := fmt.Sprintf(`SELECT (%s) AS data`, subsqls)
	var data pgtype.Bytea
	if debugExecuteSQL {
		log.Debugf("%s:%s: %v", EnvSQLDebugName, EnvSQLDebugExecute, fsql)
	}
	{
		now := time.Now()
		err = p.pool.QueryRow(ctx, fsql, args...).Scan(&data)
		if p.mvtProviderQueryHistogramSeconds != nil {
			z, _, _ := tile.ZXY()
			// TODO (bemyak): add params as labels to prometheus?
			lbls := prometheus.Labels{
				"z":        strconv.FormatUint(uint64(z), 10),
				"map_name": mapName,
			}
			p.mvtProviderQueryHistogramSeconds.With(lbls).Observe(time.Since(now).Seconds())
		}
	}

	if debugExecuteSQL {
		log.Debugf("%s:%s: %v", EnvSQLDebugName, EnvSQLDebugExecute, fsql)
		if err != nil {
			log.Errorf("%s:%s: returned error %v", EnvSQLDebugName, EnvSQLDebugExecute, err)
		} else {
			log.Debugf("%s:%s: returned %v bytes", EnvSQLDebugName, EnvSQLDebugExecute, len(data.Bytes))
		}
	}

	// data may have garbage in it.
	if err := ctxErr(ctx, err); err != nil {
		return []byte{}, err
	}
	return data.Bytes, nil
}

// Close will close the Provider's database connectio
func (p *Provider) Close() { p.pool.Close() }

// reference to all instantiated providers
var providers []Provider

// Cleanup will close all database connections and destroy all previously instantiated Provider instances
func Cleanup() {
	if len(providers) > 0 {
		log.Infof("cleaning up postgis providers")
	}

	for i := range providers {
		providers[i].Close()
	}

	providers = make([]Provider, 0)
}
