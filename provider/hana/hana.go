package hana

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/SAP/go-hdb/driver"
	"github.com/go-spatial/geom"
	"github.com/go-spatial/geom/encoding/wkb"
	"github.com/go-spatial/geom/encoding/wkt"
	"github.com/go-spatial/tegola"
	"github.com/go-spatial/tegola/dict"
	"github.com/go-spatial/tegola/internal/log"
	"github.com/go-spatial/tegola/observability"
	"github.com/go-spatial/tegola/provider"
	"github.com/prometheus/client_golang/prometheus"
)

const Name = "hana"

type connectionPoolCollector struct {
	pool *sql.DB

	// providerName the pool is created for
	// required to make metrics unique
	providerName string

	maxConnectionDesc        *prometheus.Desc
	currentConnectionsDesc   *prometheus.Desc
	availableConnectionsDesc *prometheus.Desc
}

func (c connectionPoolCollector) Close() {
	c.pool.Close()
}

func (c connectionPoolCollector) QueryRow(query string, args ...any) *sql.Row {
	return c.pool.QueryRow(query, args...)
}

func (c connectionPoolCollector) QueryContext(ctx context.Context, query string) (*sql.Rows, error) {
	return c.pool.QueryContext(ctx, query)
}

func (c connectionPoolCollector) QueryContextWithBBox(ctx context.Context, query string, extent *geom.Extent, srid uint64, hasTileBounds bool) (*sql.Rows, error) {
	ll, ur, err := getBBoxCoordinates(extent, srid)
	if err != nil {
		return nil, err
	}

	strLL, _ := wkt.Encode(ll)
	lobLL := new(driver.Lob)
	lobLL.SetReader(strings.NewReader(strLL))

	strUR, _ := wkt.Encode(ur)
	lobUR := new(driver.Lob)
	lobUR.SetReader(strings.NewReader(strUR))

	if hasTileBounds {
		strTileBounds := fmt.Sprintf("LINESTRING(%v %v, %v %v)", ll.X(), ll.Y(), ur.X(), ur.Y())
		lobBounds := new(driver.Lob)
		lobBounds.SetReader(strings.NewReader(strTileBounds))
		return c.pool.QueryContext(ctx, query, lobLL, lobUR, srid, strTileBounds)
	} else {
		return c.pool.QueryContext(ctx, query, lobLL, lobUR, srid)
	}
}

func (c connectionPoolCollector) Describe(ch chan<- *prometheus.Desc) {
	prometheus.DescribeByCollect(c, ch)
}

func (c connectionPoolCollector) Collect(ch chan<- prometheus.Metric) {
	if c.pool == nil {
		return
	}
	stat := c.pool.Stats()
	ch <- prometheus.MustNewConstMetric(
		c.maxConnectionDesc,
		prometheus.GaugeValue,
		float64(stat.MaxOpenConnections),
	)
	ch <- prometheus.MustNewConstMetric(
		c.currentConnectionsDesc,
		prometheus.GaugeValue,
		float64(stat.OpenConnections),
	)
	ch <- prometheus.MustNewConstMetric(
		c.availableConnectionsDesc,
		prometheus.GaugeValue,
		float64(stat.MaxOpenConnections-stat.OpenConnections),
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
		prefix+"hana_max_connections",
		"Max number of hana connections in the pool",
		nil,
		prometheus.Labels{"provider_name": c.providerName},
	)

	c.currentConnectionsDesc = prometheus.NewDesc(
		prefix+"hana_current_connections",
		"Current number of hana connections in the pool",
		nil,
		prometheus.Labels{"provider_name": c.providerName},
	)

	c.availableConnectionsDesc = prometheus.NewDesc(
		prefix+"hana_available_connections",
		"Current number of available hana connections in the pool",
		nil,
		prometheus.Labels{"provider_name": c.providerName},
	)

	return []observability.Collector{c}, nil
}

// Provider provides the HANA data provider.
type Provider struct {
	dbVersion uint
	name      string
	pool      *connectionPoolCollector
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
	DefaultURI             = ""
	DefaultMaxConn         = 100
	DefaultMaxConnIdleTime = "30m"
	DefaultMaxConnLifetime = "1h"
)

const (
	ConfigKeyName            = "name"
	ConfigKeyURI             = "uri"
	ConfigKeyMaxConn         = "max_connections"
	ConfigKeyMaxConnIdleTime = "max_connection_idle_time"
	ConfigKeyMaxConnLifetime = "max_connection_life_time"
	ConfigKeySRID            = "srid"
	ConfigKeyLayers          = "layers"
	ConfigKeyLayerName       = "name"
	ConfigKeyTablename       = "tablename"
	ConfigKeySQL             = "sql"
	ConfigKeyFields          = "fields"
	ConfigKeyGeomField       = "geometry_fieldname"
	ConfigKeyFeatureIDField  = "id_fieldname"
	ConfigKeyGeomType        = "geometry_type"
	ConfigKeyBuffer          = "buffer"
	ConfigKeyClipGeometry    = "clip_geometry"
)

type DataType byte

const (
	DtTinyint DataType = iota
	DtSmallint
	DtInteger
	DtBigint
	DtDecimal
	DtSmalldecimal
	DtReal
	DtDouble
	DtChar
	DtVarchar
	DtNChar
	DtNVarchar
	DtShorttext
	DtAlphanum
	DtBinary
	DtVarbinary
	DtDate
	DtTime
	DtTimestamp
	DtSeconddate
	DtClob
	DtNClob
	DtBlob
	DtText
	DtBoolean
	DtSTPoint
	DtSTGeometry
	DtUnknown
)

type FieldDescription struct {
	dataType    DataType
	name        string
	isGeometry  bool
	isFeatureId bool
}

func OpenDB(uri string) (*sql.DB, error) {
	u, err := url.Parse(uri)
	if err != nil {
		return nil, err
	}

	if u.Scheme != "hdb" {
		return nil, ErrInvalidURI{Msg: fmt.Sprintf("invalid scheme '%v'", u.Scheme)}
	}
	params := u.Query()

	supportedParams := []string{ConfigKeyMaxConn, ConfigKeyMaxConnIdleTime, ConfigKeyMaxConnLifetime, driver.DSNTimeout, driver.DSNTLSInsecureSkipVerify, driver.DSNTLSRootCAFile, driver.DSNTLSServerName}
	for k := range params {
		found := false
		for i := 0; i < len(supportedParams); i++ {
			pname := supportedParams[i]
			if k == pname {
				found = true
				break
			}
		}

		if !found {
			return nil, ErrInvalidURI{Msg: fmt.Sprintf("parameter '%v' is unknown", k)}
		}
	}

	max_conn := DefaultMaxConn
	if params.Has(ConfigKeyMaxConn) {
		max_conn, err = strconv.Atoi(params.Get(ConfigKeyMaxConn))
		if err != nil {
			return nil, ErrInvalidURI{Msg: "max_connections value is incorrect"}
		}
	}

	max_conn_idle_time, _ := time.ParseDuration(DefaultMaxConnIdleTime)
	if params.Has(ConfigKeyMaxConnIdleTime) {
		value := params.Get(ConfigKeyMaxConnIdleTime)
		max_conn_idle_time, err = time.ParseDuration(value)
		if err != nil {
			return nil, ErrInvalidURI{Msg: "max_connection_idle_time value is incorrect"}
		}
	}

	max_conn_life_time, _ := time.ParseDuration(DefaultMaxConnLifetime)
	if params.Has(ConfigKeyMaxConnLifetime) {
		value := params.Get(ConfigKeyMaxConnLifetime)
		max_conn_life_time, err = time.ParseDuration(value)
		if err != nil {
			return nil, ErrInvalidURI{Msg: "max_connection_life_time value is incorrect"}
		}
	}

	// We construct a new uri that only contains parameters supported by the HANA driver.
	// Otherwise, driver.NewDSNConnector returns an error.
	newParams := url.Values{}
	for i := 3; i < len(supportedParams); i++ {
		pname := supportedParams[i]
		if params.Has(pname) {
			value := params.Get(pname)
			if pname == driver.DSNTLSServerName && value == "host" {
				value = strings.Split(u.Host, ":")[0]
			}
			newParams.Add(pname, value)
		}
	}

	newUri := &url.URL{
		Scheme:   u.Scheme,
		User:     u.User,
		Host:     u.Host,
		RawQuery: newParams.Encode(),
	}

	connector, err := driver.NewDSNConnector(newUri.String())
	if err != nil {
		return nil, err
	}

	sv := driver.SessionVariables{"APPLICATION": "Tegola"}
	connector.SetSessionVariables(sv)

	db := sql.OpenDB(connector)
	db.SetMaxOpenConns(max_conn)
	db.SetConnMaxIdleTime(max_conn_idle_time)
	db.SetConnMaxLifetime(max_conn_life_time)

	return db, nil
}

// CreateConnection creates a connection from config values
func CreateConnection(config dict.Dicter) (*sql.DB, error) {
	uri, err := config.String(ConfigKeyURI, nil)
	if err != nil {
		return nil, err
	}

	db, err := OpenDB(uri)
	if err != nil {
		return db, err
	}
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("Failed while establishing connection: %v", err)
	}

	return db, nil
}

// CreateProvider instantiates and returns a new postgis provider or an error.
// The function will validate that the config object looks good before
// trying to create a driver. This Provider supports the following fields
// in the provided map[string]interface{} map:
//
//	uri (string): [Required] HANA database host
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
func CreateProvider(config dict.Dicter, maps []provider.Map, providerType string) (*Provider, error) {
	conn, err := CreateConnection(config)
	if err != nil {
		return nil, err
	}

	var dbVersion string
	if err := conn.QueryRow(`SELECT VERSION FROM "SYS"."M_DATABASE"`).Scan(&dbVersion); err != nil {
		return nil, err
	}

	majorVersion, _ := strconv.Atoi(strings.Split(dbVersion, ".")[0])
	if providerType == MVTProviderType && majorVersion < 4 {
		return nil, fmt.Errorf("MVT provider is only available in HANA Cloud")
	}

	srid := -1
	if srid, err = config.Int(ConfigKeySRID, &srid); err != nil {
		return nil, err
	}

	name, err := config.String(ConfigKeyName, nil)
	if err != nil {
		return nil, err
	}

	p := Provider{
		name:      name,
		dbVersion: uint(majorVersion),
		srid:      uint64(srid),
		pool:      &connectionPoolCollector{pool: conn, providerName: name},
	}

	layers, err := config.MapSlice(ConfigKeyLayers)
	if err != nil {
		return nil, err
	}

	lyrs := make(map[string]Layer)
	lyrsSeen := make(map[string]int)

	for i, layer := range layers {

		lName, err := layer.String(ConfigKeyLayerName, nil)
		if err != nil {
			return nil, fmt.Errorf("for layer (%v) we got the following error trying to get the layer's name field: %w", i, err)
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
		idfld, err = layer.String(ConfigKeyFeatureIDField, &idfld)
		if err != nil {
			return nil, fmt.Errorf("for layer (%v) %v : %w", i, lName, err)
		}
		if idfld == geomfld {
			return nil, fmt.Errorf("for layer (%v) %v: %v (%v) and %v field (%v) is the same", i, lName, ConfigKeyGeomField, geomfld, ConfigKeyFeatureIDField, idfld)
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

		if lsrid < 0 {
			// we try to auto detect SRID if it is not specified neither
			// for the provider nor for the layer.
			sqlQuery := sql
			if sqlQuery == "" {
				sqlQuery = fmt.Sprintf(`(SELECT * FROM %v)`, quoteIdentifier(tblName))
			}

			lsrid, err = getGeometryColumnSRID(p.pool, p.dbVersion, sqlQuery, geomfld)
			if err != nil {
				return nil, err
			}
		}

		if isSrsRoundEarth(p.pool, uint64(lsrid)) {
			if !hasSrsPlanarEquivalent(p.pool, uint64(lsrid)) {
				return nil, fmt.Errorf("unable to find a planar equivalent for srid %v in layer: %v", lsrid, lName)
			}
			lsrid = int(toPlanarEquivalenSrid(uint64(lsrid)))
		}

		l := Layer{
			name:      lName,
			idField:   idfld,
			geomField: geomfld,
			srid:      uint64(lsrid),
		}

		if sql != "" && !isSelectQuery(sql) {
			// if it is not a SELECT query, then we assume we have a sub-query
			// (`(select ...) as foo`) which we can handle like a tablename
			tblName = sql
			sql = ""
		}

		if sql != "" {
			sql = sanitizeSQL(sql)
			// make sure that the sql has a !BBOX! token
			if !strings.Contains(sql, bboxToken) {
				return nil, fmt.Errorf("SQL for layer (%v) %v is missing required token: %v", i, lName, bboxToken)
			}
			if !strings.Contains(sql, "*") {
				if !strings.Contains(sql, geomfld) {
					return nil, ErrGeomFieldNotFound{
						GeomFieldName: geomfld,
						LayerName:     lName,
					}
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
			if len(fields) == 0 {
				fields, err = getTableFieldNames(p.pool, &l, tblName)
				if err != nil {
					return nil, err
				}
			}

			l.sql, err = genSQL(&l, tblName, fields, true, providerType)
			if err != nil {
				return nil, fmt.Errorf("could not generate sql, for layer(%v): %w", lName, err)
			}
		}

		l.fields, err = getLayerFields(p.pool, &l, l.sql)
		if err != nil {
			return nil, err
		}

		// set the layer geom type
		if geomType != "" {
			if err = p.setLayerGeomType(&l, geomType); err != nil {
				return nil, fmt.Errorf("error fetching geometry type for layer (%v): %w", l.name, err)
			}
		} else {
			pname, err := config.String(ConfigKeyName, nil)
			if err != nil {
				return nil, err
			}

			if err = p.inspectLayerGeomType(pname, &l, maps); err != nil {
				return nil, fmt.Errorf("error fetching geometry type for layer (%v): %w", l.name, err)
			}
		}

		if providerType == MVTProviderType {
			var buffer uint = 256
			if buffer, err = layer.Uint(ConfigKeyBuffer, &buffer); err != nil {
				return nil, err
			}

			var clipGeom = true
			if clipGeom, err = layer.Bool(ConfigKeyClipGeometry, &clipGeom); err != nil {
				return nil, err
			}

			l.sql, err = genMVTSQL(&l, getFieldNames(l.fields), buffer, clipGeom)
			if err != nil {
				return nil, err
			}
		}

		if debugLayerSQL {
			log.Debugf("SQL for Layer(%v):\n%v\n", lName, l.sql)
		}

		lyrs[lName] = l
	}
	p.layers = lyrs

	// track the provider so we can clean it up later
	providers = append(providers, p)

	return &p, nil
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
func (p Provider) inspectLayerGeomType(pname string, l *Layer, maps []provider.Map) error {
	var err error

	re := regexp.MustCompile(`(?i)ST_AsBinary`)
	sqlQuery := re.ReplaceAllString(l.sql, "ST_GeometryType")

	// we only need a single result set to sniff out the geometry type
	sqlQuery = fmt.Sprintf("%v LIMIT 1", sqlQuery)

	// if a !ZOOM! token exists, all features could be filtered out so we don't have a geometry to inspect it's type.
	// address this by replacing the !ZOOM! token with an range of all values which includes all zooms,
	// in this case the query must use the following condition "scalerank IN (!ZOOM!)"
	sqlQuery = strings.Replace(sqlQuery, "!ZOOM!", "0,1,2,3,4,5,6,7,8,9,10,11,12,13,14,15,16,17,18,19,20,21,22,23,24", 1)

	// we need a tile to run our sql through the replacer
	tile := provider.NewTile(0, 0, 0, 64, tegola.WebMercator)

	withBBox := strings.Contains(l.sql, bboxToken)
	// normal replacer
	sqlQuery, err = replaceTokens(p.dbVersion, sqlQuery, l.IDFieldName(), l.GeomFieldName(), l.GeomType(), l.SRID(), tile, true)
	if err != nil {
		return err
	}

	// substitute default values to parameter
	params := extractQueryParamValues(pname, maps, l)

	args := make([]interface{}, 0)
	sqlQuery = params.ReplaceParams(sqlQuery, &args)

	if provider.ParameterTokenRegexp.MatchString(sqlQuery) {
		// remove all parameter tokens for inspection
		// crossing our fingers that the query is still valid 🤞
		// if not, the user will have to specify `geometry_type` in the config
		sqlQuery = provider.ParameterTokenRegexp.ReplaceAllString(sqlQuery, "")
	}

	extent, _ := getTileExtent(tile, false)
	rows, err := getLayerRows(p.pool, sqlQuery, extent, l.SRID(), withBBox)
	if err != nil {
		return err
	}

	defer rows.Close()

	columns, err := rows.ColumnTypes()
	if err != nil {
		return err
	}

	fields, err := getFieldDescriptions(l.Name(), l.GeomFieldName(), l.IDFieldName(), columns, false)
	if err != nil {
		return err
	}

	rowValues := make([]interface{}, len(fields))

	for rows.Next() {
		setupRowValues(fields, rowValues)

		err := rows.Scan(rowValues...)
		if err != nil {
			return fmt.Errorf("error running layer (%v) SQL (%v): %w", l, sqlQuery, err)
		}

		for i := range rowValues {
			if rowValues[i] == nil || fields[i].name != l.GeomFieldName() {
				continue
			}

			value := *(rowValues[i].(*sql.NullString))
			err := p.setLayerGeomType(l, strings.Trim(value.String, "ST_"))
			if err != nil {
				return err
			}

			break
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
func (p Provider) TileFeatures(ctx context.Context, layer string, tile provider.Tile, params provider.Params, fn func(f *provider.Feature) error) error {

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

	sqlQuery, err := replaceTokens(p.dbVersion, plyr.sql, plyr.IDFieldName(), plyr.GeomFieldName(), plyr.GeomType(), plyr.SRID(), tile, true)
	if err != nil {
		return fmt.Errorf("error replacing layer tokens for layer (%v) SQL (%v): %w", layer, sqlQuery, err)
	}

	// replace configured query parameters if any
	args := make([]interface{}, 0)
	sqlQuery = params.ReplaceParams(sqlQuery, &args)
	if err != nil {
		return err
	}

	if debugExecuteSQL {
		log.Debugf("TEGOLA_SQL_DEBUG:EXECUTE_SQL for layer (%v): %v", layer, sqlQuery)
	}

	now := time.Now()

	extent, _ := getTileExtent(tile, true)
	srid := plyr.SRID()
	rows, err := p.pool.QueryContextWithBBox(ctx, sqlQuery, extent, srid, false)

	if err := ctxErr(ctx, err); err != nil {
		return fmt.Errorf("error running layer (%v) SQL (%v): %w", layer, sqlQuery, err)
	}

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
		return fmt.Errorf("error running layer (%v) SQL (%v): %w", layer, sqlQuery, err)
	}

	rowValues := make([]interface{}, len(plyr.FieldDescriptions()))

	reportedLayerFieldName := ""
	for rows.Next() {
		// context check
		if err := ctx.Err(); err != nil {
			return err
		}

		setupRowValues(plyr.FieldDescriptions(), rowValues)

		// fetch row values
		err := rows.Scan(rowValues...)
		if err := ctxErr(ctx, err); err != nil {
			return fmt.Errorf("error running layer (%v) SQL (%v): %w", layer, sqlQuery, err)
		}

		gid, geobytes, tags, err := readRowValues(ctx, plyr.FieldDescriptions(), rowValues)
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

func (p Provider) MVTForLayers(ctx context.Context, tile provider.Tile, params provider.Params, layers []provider.Layer) ([]byte, error) {
	var mapName string

	{
		mapNameVal := ctx.Value(observability.ObserveVarMapName)
		if mapNameVal != nil {
			// if it's not convertible to a string, we will ignore it.
			mapName, _ = mapNameVal.(string)
		}
	}

	args := make([]interface{}, 0)
	var mvtBytes bytes.Buffer
	var totalSeconds float64
	totalSeconds = 0.0

	for i := range layers {
		layer := layers[i]
		if debug {
			log.Debugf("looking for layer: %v", layer)
		}
		l, ok := p.Layer(layer.Name)

		if !ok {
			// Should we error here, or have a flag so that we don't
			// spam the user?
			log.Warnf("provider layer not found %v", layer.Name)
		}
		if debugLayerSQL {
			log.Debugf("SQL for Layer(%v):\n%v\n", l.Name(), l.sql)
		}

		sqlQuery, err := replaceTokens(p.dbVersion, l.sql, l.IDFieldName(), l.GeomFieldName(), l.GeomType(), l.SRID(), tile, false)
		if err := ctxErr(ctx, err); err != nil {
			return nil, err
		}

		// replace configured query parameters if any
		sqlQuery = params.ReplaceParams(sqlQuery, &args)

		now := time.Now()

		extent, _ := getTileExtent(tile, false)
		srid := l.SRID()
		rows, err := p.pool.QueryContextWithBBox(ctx, sqlQuery, extent, srid, true)

		if err := ctxErr(ctx, err); err != nil {
			return []byte{}, err
		}

		defer rows.Close()

		if err := ctxErr(ctx, err); err != nil {
			return []byte{}, err
		}

		lob := &driver.Lob{}
		lob.SetWriter(new(bytes.Buffer))

		if rows.Next() {
			if err := ctx.Err(); err != nil {
				return []byte{}, err
			}

			err = rows.Scan(lob)
			if err := ctxErr(ctx, err); err != nil {
				return []byte{}, err
			}

			mvtBytes.Write(lob.Writer().(*bytes.Buffer).Bytes())
		} else {
			return nil, fmt.Errorf("unable to read the result set for layer (%v)", l.Name())
		}

		totalSeconds += time.Since(now).Seconds()

		if debugExecuteSQL {
			log.Debugf("%s:%s: %v", EnvSQLDebugName, EnvSQLDebugExecute, sqlQuery)
			if err != nil {
				log.Errorf("%s:%s: returned error %v", EnvSQLDebugName, EnvSQLDebugExecute, err)
			} else {
				log.Debugf("%s:%s: returned %v bytes", EnvSQLDebugName, EnvSQLDebugExecute, lob.Writer().(*bytes.Buffer).Len())
			}
		}
	}

	if p.mvtProviderQueryHistogramSeconds != nil {
		z, _, _ := tile.ZXY()
		lbls := prometheus.Labels{
			"z":        strconv.FormatUint(uint64(z), 10),
			"map_name": mapName,
		}
		p.mvtProviderQueryHistogramSeconds.With(lbls).Observe(totalSeconds)
	}

	return mvtBytes.Bytes(), nil
}

// Close will close the Provider's database connectio
func (p *Provider) Close() { p.pool.Close() }

// reference to all instantiated providers
var providers []Provider

// Cleanup will close all database connections and destroy all previously instantiated Provider instances
func Cleanup() {
	if len(providers) > 0 {
		log.Infof("cleaning up HANA providers")
	}

	for i := range providers {
		providers[i].Close()
	}

	providers = make([]Provider, 0)
}
