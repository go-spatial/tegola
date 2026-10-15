package hana

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/SAP/go-hdb/driver"
	"github.com/alexeydott/geom"
	"github.com/alexeydott/geom/encoding/wkb"
	"github.com/alexeydott/geom/encoding/wkt"
	"github.com/alexeydott/tegola/dict"
	"github.com/alexeydott/tegola/internal/log"
	"github.com/alexeydott/tegola/internal/sqltoken"
	"github.com/alexeydott/tegola/mos"
	"github.com/alexeydott/tegola/observability"
	"github.com/alexeydott/tegola/provider"
	"github.com/alexeydott/tegola/provider/crsconfig"
	codec "github.com/alexeydott/tegola/provider/geometrycodec"
	"github.com/alexeydott/tegola/provider/mapplgis"
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
	_ = c.pool.Close()
}

func (c connectionPoolCollector) QueryRow(query string, args ...any) *sql.Row {
	return c.pool.QueryRow(query, args...)
}

// QueryRowContext mirrors QueryRow for registration-time probes that must
// run under the inspection timeout (audit P5-16).
func (c connectionPoolCollector) QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row {
	return c.pool.QueryRowContext(ctx, query, args...)
}

func (c connectionPoolCollector) QueryContext(ctx context.Context, query string) (*sql.Rows, error) {
	return c.pool.QueryContext(ctx, query)
}

func (c connectionPoolCollector) queryContextWithArgs(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	return c.pool.QueryContext(ctx, query, args...)
}

func (c connectionPoolCollector) QueryContextWithBBox(ctx context.Context, query string, extent *geom.Extent, srid uint64, hasTileBounds bool) (*sql.Rows, error) {
	// A synthetic SRID (crs_defn) exists only on the client side: the SQL
	// was generated with a 1=1 placeholder instead of spatial predicates,
	// so no bbox parameters are bound.
	if isSyntheticCRS(srid) {
		return c.pool.QueryContext(ctx, query)
	}

	ll, ur, err := getBBoxCoordinates(extent, srid)
	if err != nil {
		return nil, err
	}

	strLL, _ := wkt.EncodeString(ll)
	lobLL := new(driver.Lob)
	lobLL.SetReader(strings.NewReader(strLL))

	strUR, _ := wkt.EncodeString(ur)
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

// resolveGeometryFormatConfig validates the geometry_format config value.
// An empty string means the provider default (native HANA geometry).
func resolveGeometryFormatConfig(config dict.Dicter) (string, error) {
	def := ""
	v, err := config.String(codec.ConfigKeyGeometryFormat, &def)
	if err != nil {
		return "", err
	}
	switch v = strings.TrimSpace(v); v {
	case "", codec.FormatWKB, codec.FormatWKT, codec.FormatMOS:
		return v, nil
	default:
		return "", fmt.Errorf("invalid %v: %q (expected one of %q, %q, %q)",
			codec.ConfigKeyGeometryFormat, v, codec.FormatWKB, codec.FormatWKT, codec.FormatMOS)
	}
}

// hanaGeometryFormats is the set of geometry_format values accepted at the
// layer level (an empty value falls back to the provider-level value).
var hanaGeometryFormats = map[string]struct{}{
	codec.FormatWKB: {},
	codec.FormatWKT: {},
	codec.FormatMOS: {},
}

// decodeGeometryValue decodes a raw geometry column value according to the
// layer's geometry format. The default (empty) format expects HANA native
// geometry already serialized via ST_AsBinary(), decoded as WKB.
func decodeGeometryValue(v any, format string, mosCfg codec.MOSConfig) (geom.Geometry, error) {
	switch format {
	case "", codec.FormatWKB:
		return codec.DecodeWKB(v)
	case codec.FormatWKT:
		return codec.DecodeWKT(v)
	case codec.FormatMOS:
		return codec.DecodeMOS(v, mosCfg)
	default:
		return nil, fmt.Errorf("unknown geometry_format: %v", format)
	}
}

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

	db := sql.OpenDB(featureCancellableConnector(connector))
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
	// audit P5-16: the version probe runs under the inspection timeout.
	vctx, vcancel := NewInspectionContext(context.Background())
	defer vcancel()
	if err := conn.QueryRowContext(vctx, `SELECT VERSION FROM "SYS"."M_DATABASE"`).Scan(&dbVersion); err != nil {
		return nil, err
	}

	majorVersion, _ := strconv.Atoi(strings.Split(dbVersion, ".")[0])
	if providerType == MVTProviderType && majorVersion < 4 {
		return nil, fmt.Errorf("MVT provider is only available in HANA Cloud")
	}

	// provider-level srid/crs_defn via the shared CRS contract; -1 keeps the
	// legacy "auto-detect from the geometry column" behavior.
	pcrs, perr := crsconfig.ResolveProvider(config, -1)
	if perr != nil {
		return nil, perr
	}
	srid := pcrs.SRID

	// provider-level geometry format / MOS settings, shared by all layers
	// that do not override them (same contract as the other providers).
	providerGeometryFormat, err := resolveGeometryFormatConfig(config)
	if err != nil {
		return nil, err
	}
	providerMOSCfg, err := codec.ResolveMOSConfig(config, nil, "")
	if err != nil {
		return nil, err
	}
	// NOTE: mos_precision/mos_units are NOT rejected here even when the
	// provider-level geometry_format is not "mos": they are provider-level
	// defaults for layers that may still select the MOS format via their own
	// layer-level geometry_format. Relevance (warn) is decided per layer,
	// after the effective layer format is known.
	if providerType == MVTProviderType {
		if verr := codec.ValidateMVTGeometryFormat(providerGeometryFormat); verr != nil {
			return nil, verr
		}
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

		// Audit P5-12: reject empty field names before they are quoted
		// into SQL at registration.
		for _, f := range fields {
			if err := validateIdentName(f); err != nil {
				return nil, fmt.Errorf("for layer (%v) %v %v: %w", i, lName, ConfigKeyFields, err)
			}
		}

		geomfld := "geom"
		geomfld, err = layer.String(ConfigKeyGeomField, &geomfld)
		if err != nil {
			return nil, fmt.Errorf("for layer (%v) %v : %w", i, lName, err)
		}
		if err := validateIdentName(geomfld); err != nil {
			return nil, fmt.Errorf("for layer (%v) %v %v: %w", i, lName, ConfigKeyGeomField, err)
		}

		idfld := ""
		idfld, err = layer.String(ConfigKeyFeatureIDField, &idfld)
		if err != nil {
			return nil, fmt.Errorf("for layer (%v) %v : %w", i, lName, err)
		}
		// An empty id field is the documented "no id column" sentinel and
		// stays valid; an explicitly empty quoted name ("") does not.
		if idfld != "" {
			if err := validateIdentName(idfld); err != nil {
				return nil, fmt.Errorf("for layer (%v) %v %v: %w", i, lName, ConfigKeyFeatureIDField, err)
			}
		}
		if idfld == geomfld {
			return nil, fmt.Errorf("for layer (%v) %v: %v (%v) and %v field (%v) is the same", i, lName, ConfigKeyGeomField, geomfld, ConfigKeyFeatureIDField, idfld)
		}

		geomType := ""
		geomType, err = layer.String(ConfigKeyGeomType, &geomType)
		if err != nil {
			return nil, fmt.Errorf("for layer (%v) %v : %w", i, lName, err)
		}

		// tablename and sql are mutually exclusive. Presence is checked
		// explicitly (like the postgis/gpkg providers) so an explicit
		// tablename equal to the layer name is still detected.
		_, errTable := layer.String(ConfigKeyTablename, nil)
		_, errSQL := layer.String(ConfigKeySQL, nil)
		tblPresent := true
		if _, ok := errTable.(dict.ErrKeyRequired); ok {
			tblPresent = false
		} else if errTable != nil {
			return nil, fmt.Errorf(
				"for %v layer (%v) %v has an error: %w",
				i,
				lName,
				ConfigKeyTablename,
				errTable,
			)
		}
		sqlPresent := true
		if _, ok := errSQL.(dict.ErrKeyRequired); ok {
			sqlPresent = false
		} else if errSQL != nil {
			return nil, fmt.Errorf(
				"for %v layer (%v) %v has an error: %w",
				i,
				lName,
				ConfigKeySQL,
				errSQL,
			)
		}

		if tblPresent && sqlPresent {
			return nil, fmt.Errorf(
				"for %v layer (%v) %v: only one of %v or %v can be specified",
				providerType,
				lName,
				i,
				ConfigKeyTablename,
				ConfigKeySQL,
			)
		}
		if !tblPresent && !sqlPresent {
			return nil, fmt.Errorf(
				"for %v layer (%v) %v: one of %v or %v must be specified",
				providerType,
				lName,
				i,
				ConfigKeyTablename,
				ConfigKeySQL,
			)
		}

		var tblName string
		if tblPresent {
			tblName, err = layer.String(ConfigKeyTablename, &lName)
			if err != nil {
				return nil, fmt.Errorf("for %v layer (%v) %v has an error: %w", i, lName, ConfigKeyTablename, err)
			}
			// Audit P5-12: reject empty table names at registration; the
			// quoted-identifier parts are validated by quoteTableName.
			if err := validateTableName(tblName); err != nil {
				return nil, fmt.Errorf("for %v layer (%v) %v: %w", i, lName, ConfigKeyTablename, err)
			}
		}

		var sql string
		if sqlPresent {
			sql, err = layer.String(ConfigKeySQL, &sql)
			if err != nil {
				return nil, fmt.Errorf("for %v layer (%v) %v has an error: %w", i, lName, ConfigKeySQL, err)
			}
		}

		// layer-level srid/crs_defn via the shared CRS contract; a layer
		// crs_defn wins over any numeric srid on the same level.
		lcrs, lerr := crsconfig.ResolveLayer(layer, srid)
		if lerr != nil {
			return nil, fmt.Errorf("for layer (%v) %v: %w", i, lName, lerr)
		}
		lsrid := lcrs.SRID

		l := Layer{
			name:           lName,
			idField:        idfld,
			geomField:      geomfld,
			srid:           uint64(lsrid),
			geometryFormat: providerGeometryFormat,
			mosConfig:      providerMOSCfg,
		}

		// layer-level geometry format / MOS overrides merge on top of the
		// provider-level values.
		layerMOSCfg, lerr := codec.ResolveMOSConfig(nil, layer, lName)
		if lerr != nil {
			return nil, lerr
		}
		layerGeometryFormat, lerr := codec.ResolveLayerGeometryFormat(providerGeometryFormat, layer, lName, hanaGeometryFormats)
		if lerr != nil {
			return nil, fmt.Errorf("for layer (%v) %w", i, lerr)
		}
		l.geometryFormat = layerGeometryFormat
		l.mosConfig = codec.MergeMOSConfig(providerMOSCfg, layerMOSCfg)
		codec.WarnAndResetMOSParams(l.geometryFormat, &l.mosConfig, lName)

		// Resolve the bounds field names (layer > provider > defaults)
		// backing the bounds-backed custom-SQL !BBOX! predicate for raw
		// (MOS) layers.
		l.bboxFields, lerr = codec.ResolveBBoxFields(config, layer, lName)
		if lerr != nil {
			return nil, fmt.Errorf("for layer (%v) %v: %w", i, lName, lerr)
		}

		// A06: canonical MapplGIS detection runs for every tablename layer
		// before the SRID resolution: a MapplGIS table is not a spatial
		// table, so a layer pointing at one must be discovered here and
		// served via the MOS path with the self-described projection.
		// Custom SQL is never auto-detected.
		if tblPresent {
			l.featureTable = tblName
			isMappl, derr := detectMapplGIS(context.Background(), p.pool, &l, tblName)
			if derr != nil {
				return nil, fmt.Errorf("for layer (%v) %v: %v", i, lName, derr)
			}
			if isMappl {
				// A02: an explicitly configured ID field is honored; the
				// contract primary key (OKEY) replaces the empty default.
				if idfld == "" {
					idfld = mapplgis.PrimaryKey
					l.idField = idfld
				}
				lsrid = int(l.srid)
				log.Debugf("layer (%v): table %v detected as MapplGIS", lName, tblName)
			}
		}

		if lsrid < 0 {
			// we try to auto detect SRID if it is not specified neither
			// for the provider nor for the layer. Native HANA ST_Geometry
			// columns can report their SRS via ST_SRID(); raw formats cannot:
			// MOS layers self-describe through the canonical structural
			// MapplGIS detector (registration-time only), while plain WKB/WKT
			// columns carry no CRS information at all.
			switch {
			case l.geometryFormat == codec.FormatMOS:
				// shared A11 enforcement (identical across providers)
				if verr := codec.ValidateMOSSQLExplicitConfig(lName, pcrs.Explicit || lcrs.Explicit); verr != nil {
					return nil, fmt.Errorf("for layer (%v) %v: %w", i, lName, verr)
				}
				return nil, fmt.Errorf(
					"for layer (%v) %v: source CRS unresolved; MOS columns carry no CRS metadata, specify %v or %v",
					i, lName, crsconfig.KeySRID, crsconfig.KeyCRSDefn,
				)
			case codec.IsRawFormat(l.geometryFormat):
				// wkb/wkt
				return nil, fmt.Errorf(
					"for layer (%v) %v: source CRS unresolved; %v columns carry no CRS metadata, specify %v or %v",
					i, lName, l.geometryFormat, crsconfig.KeySRID, crsconfig.KeyCRSDefn,
				)
			default:
				sqlQuery := sql
				if sqlQuery == "" {
					sqlQuery = fmt.Sprintf(`(SELECT * FROM %v)`, quoteTableName(tblName))
				}

				lsrid, err = getGeometryColumnSRID(p.pool, p.dbVersion, sqlQuery, geomfld)
				if err != nil {
					return nil, err
				}
				l.srid = uint64(lsrid)
			}
		}

		l.featureSRID = uint64(lsrid)
		switch {
		case isSyntheticCRS(uint64(lsrid)):
			// A synthetic SRID registered from crs_defn exists only on the
			// client side: HANA spatial predicates and MVT generation need a
			// database-side SRS, so !BBOX! degrades to 1=1 (see getBBoxFilter)
			// and filtering happens in memory on a raw geometry format.
			if verr := validateCRSFormatCompatibility(lsrid, providerType, l.geometryFormat); verr != nil {
				return nil, fmt.Errorf("for layer (%v) %v: %w", i, lName, verr)
			}
		default:
			// audit N11: the SRS lookups return errors instead of silently
			// treating an unresolvable SRS as planar.
			roundEarth, rerr := isSrsRoundEarth(p.pool, uint64(lsrid))
			if rerr != nil {
				return nil, fmt.Errorf("for layer (%v) %v: %w", i, lName, rerr)
			}
			if !roundEarth {
				break
			}
			hasPlanar, perr := hasSrsPlanarEquivalent(p.pool, uint64(lsrid))
			if perr != nil {
				return nil, fmt.Errorf("for layer (%v) %v: %w", i, lName, perr)
			}
			if !hasPlanar {
				return nil, fmt.Errorf("unable to find a planar equivalent for srid %v in layer: %v", lsrid, lName)
			}
			lsrid = int(toPlanarEquivalenSrid(uint64(lsrid)))
			l.srid = uint64(lsrid)
		}

		// MVT providers must not take the raw geometry path: their geometry
		// is MVT bytes produced by the database, not a raw feature geometry.
		if providerType == MVTProviderType {
			if verr := codec.ValidateMVTGeometryFormat(l.geometryFormat); verr != nil {
				return nil, fmt.Errorf("for layer (%v) %v: %w", i, lName, verr)
			}
		}

		if sql != "" && !isSelectQuery(sql) {
			// if it is not a SELECT query, then we assume we have a sub-query
			// (`(select ...) as foo`) which we can handle like a tablename
			tblName = sql
			sql = ""
		}

		if sql != "" {
			sql = sanitizeSQL(sql)
			// Raw custom-SQL contract: native HANA geometry requires the
			// !BBOX! token; only MOS raw format is allowed (and required)
			// to carry it, with the bounds-backed predicate builder
			// replacing the spatial predicate at query time.
			if verr := codec.HANA.ValidateRawCustomSQL(lName, l.geometryFormat, sql, bboxToken, "!BOX!"); verr != nil {
				return nil, fmt.Errorf("for layer (%v) %v: %w", i, lName, verr)
			}
			if rerr := codec.HANA.RequireBBoxCustomSQL(lName, l.geometryFormat == codec.FormatMOS, sql, bboxToken, "!BOX!"); rerr != nil {
				return nil, fmt.Errorf("for layer (%v) %v: %w", i, lName, rerr)
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
			l.bboxTable, err = codec.ResolveBBoxTable(layer, lName)
			if err != nil {
				return nil, err
			}

			// Shared probe preparation: the probe always executes the SQL
			// without a spatial filter (!BBOX!/!BOX! -> 1=1) and with
			// permissive position/zoom placeholders, applied in the ONE
			// documented order of codec.PrepareProbeSQL (7.2.2).
			inspectionSQL := codec.HANA.PrepareProbeSQL(l.sql, geomfld, idfld, geomType)

			// Storage-format probe + structural contract (A01/A02/A08):
			// it runs for explicit MOS and for inference (""), INCLUDING
			// tile-dependent SQL and layers with an explicit geometry_type
			// (A03): the structural result-column contract is always
			// validated at registration and never skipped (the >=3-sample-
			// row evidence bar is inference-only).
			if l.geometryFormat != codec.FormatWKB && l.geometryFormat != codec.FormatWKT {
				columns, contract, perr := p.probeMOSCustomSQLContract(&l, inspectionSQL)
				strict := l.geometryFormat == codec.FormatMOS
				switch {
				case perr != nil && strict:
					return nil, fmt.Errorf("layer '%v' problem probing bounds-backed MOS custom SQL: %v", lName, perr)

				case perr != nil:
					log.Warnf("layer '%v': custom SQL storage-format probe failed; format not detected: %v", lName, perr)

				default:
					// >=3 decodable MOS rows (positive MOS signature only)
					// is the sql-sample evidence bar; the structural
					// contract is fail-closed for explicit MOS and for
					// inference with MOS evidence (A02/A05).
					mosEvidence := contract.DetectsMOS()
					if strict || mosEvidence {
						resolved, boundsInResult, cerr := codec.HANA.ResolveBoundsSQLContract(lName, l.sql, geomfld, contract, l.bboxFields)
						if cerr != nil {
							return nil, fmt.Errorf("for layer (%v) %v: %w", i, lName, cerr)
						}
						if boundsInResult {
							// persist the ACTUAL result-column names (A09):
							// runtime predicates quote identifiers
							// case-sensitively.
							l.bboxFields = resolved
						} else {
							// Bounds columns are filtered on but not selected in the
							// result set: the generated !BBOX! predicate expands inside
							// the custom SQL and resolves in the query's own scope (e.g.
							// the source table's bounds columns). Keep the resolved
							// chain (layer > provider > MINX/MAXX/MINY/MAXY) and warn -
							// never fail the whole provider over this (b5ad0979
							// regression fix).
							log.Warnf("layer '%v': bounds columns (%v) are not present in the custom SQL result columns; the !BBOX! predicate resolves in the query's own scope (e.g. the source table's bounds columns). Registering with the configured bounds field names - add the bounds columns to the SELECT list to validate them at registration", lName, strings.Join(resolved[:], ", "))
						}
						if contract.GeometryField != "" {
							l.geomField = contract.GeometryField
						}
						if mosEvidence {
							l.isMapplGIS = true
							l.mapplSource = codec.MapplGISSQLSample
							if !strict {
								// inference resolves the effective format
								// to MOS (A04/A07)
								l.geometryFormat = codec.FormatMOS
							}
							log.Infof("layer '%v': bounds-backed MOS custom SQL contract detected (source %v, %v valid MOS sample rows)", lName, l.mapplSource, contract.ValidMOSRows)
						} else if !strict && geomType == "" {
							log.Warnf("layer '%v': bounds-backed MOS custom SQL sample carried %v decodable MOS rows (need %v for sql-sample tagging)", lName, contract.ValidMOSRows, codec.MinValidMOSRows)
						}
						// MOS blobs carry no CRS metadata: the source CRS
						// must be configured explicitly (A11).
						if merr := codec.ValidateMOSSQLExplicitConfig(lName, pcrs.Explicit || lcrs.Explicit); merr != nil {
							return nil, fmt.Errorf("layer '%v': %v", lName, merr)
						}
					} else {
						// inference without usable MOS evidence ends
						// "not detected"; the 3-row sample is best-effort.
						log.Warnf("layer '%v': custom SQL storage format not detected (sample columns: %v; %v decodable MOS sample rows); registering with format %q", lName, strings.Join(columns, ", "), contract.ValidMOSRows, l.geometryFormat)
					}
				}
			}
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

		if err := p.registerFeatureSource(&l, layer, providerType); err != nil {
			return nil, err
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
	// Raw geometry formats (wkb/wkt/mos) carry non-HANA values in the
	// geometry column, so ST_GeometryType-based inspection cannot work.
	// For MOS the type is derived after decoding the first real geometry.
	if l.geometryFormat == codec.FormatWKB || l.geometryFormat == codec.FormatWKT {
		return fmt.Errorf(
			"layer (%v): geometry_type is required when %v = %q; native geometry inspection cannot be used",
			l.name, codec.ConfigKeyGeometryFormat, l.geometryFormat,
		)
	}
	if l.geometryFormat == codec.FormatMOS {
		return p.inspectMOSLayerGeomType(l)
	}

	re := regexp.MustCompile(`(?i)ST_AsBinary`)
	sqlQuery := re.ReplaceAllString(l.sql, "ST_GeometryType")

	probeGeomType := ""
	if l.geomType != nil {
		probeGeomType = codec.GeomTypeName(l.geomType)
	}
	// Shared bounds-contract probe preparation (audit R1): !BBOX!/!BOX!
	// neutralize to "1=1" and zoom/position placeholders expand
	// permissively across ALL occurrences (the previous first-occurrence
	// replacement left later !ZOOM! tokens in the SQL).
	sqlQuery = codec.HANA.PrepareProbeSQL(sqlQuery, l.GeomFieldName(), l.IDFieldName(), probeGeomType)

	// The probe binds no arguments, so custom parameters cannot be
	// substituted here (the previous ReplaceParams call generated
	// placeholders that were never bound and broke the query). Strip the
	// parameter tokens for inspection; crossing our fingers that the query
	// is still valid 🤞 if not, the user will have to specify
	// `geometry_type` in the config. Only code-context tokens are
	// stripped; tokens in string literals or comments stay verbatim.
	sqlQuery = sqltoken.HANA.StripTokens(sqlQuery)

	// Shared sample window (docs/provider-contract.md): the probe reads at
	// most codec.InspectionSampleLimit rows.
	sqlQuery = codec.WrapProbeSQLTopStyle(sqlQuery)

	// The prepared probe contains no bbox placeholders, so it must run
	// without extent binding (withBBox=false).
	ctx, cancel := NewInspectionContext(context.Background())
	defer cancel()
	rows, err := getLayerRows(ctx, p.pool, sqlQuery, nil, l.SRID(), false)
	if err != nil {
		return err
	}

	defer func() { _ = rows.Close() }()

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

			// probeRawValue unwraps any typed scan target (NullString,
			// NullLob, ...) into the raw driver value so the sniff works
			// for every geometry-column representation.
			value, ok := probeRawValue(rowValues[i]).(string)
			if !ok {
				break
			}
			err := p.setLayerGeomType(l, strings.Trim(value, "ST_"))
			if err != nil {
				return err
			}

			break
		}
	}

	return rows.Err()
}

// geometryFormatName returns a human-readable name for a geometry format,
// used in error messages.
func geometryFormatName(format string) string {
	if format == "" {
		return "wkb"
	}
	return format
}

// mosProbeSQL builds the shared bounds-contract probe query for a MOS
// layer's custom SQL (audit R1): codec.PrepareProbeSQL neutralizes
// !BBOX!/!BOX! to "1=1" (never "TRUE", so tokens inside SQL function
// arguments stay syntactically valid; all occurrences, not just the first)
// and expands zoom/position placeholders permissively, then the query is
// wrapped in the shared InspectionSampleLimit sample window
// (docs/provider-contract.md). Custom parameter tokens are stripped for
// inspection; if the query cannot run without them the user must set
// geometry_type in the config.
func mosProbeSQL(l *Layer) string {
	probeGeomType := ""
	if l.geomType != nil {
		probeGeomType = codec.GeomTypeName(l.geomType)
	}
	sql := codec.HANA.PrepareProbeSQL(l.sql, l.GeomFieldName(), l.IDFieldName(), probeGeomType)
	sql = sqltoken.HANA.StripTokens(sql)
	return codec.WrapProbeSQLTopStyle(sql)
}

// inspectMOSLayerGeomType samples the first rows of the layer's SQL and
// derives the geometry type from the first decodable MOS geometry. The
// MapplGIS LayerInfo blob is metadata: rows carrying it are skipped without
// applying anything (audit A-01 — custom SQL never auto-applies system
// info; detection belongs to the registration-time table contract).
func (p Provider) inspectMOSLayerGeomType(l *Layer) error {
	sqlQuery := mosProbeSQL(l)

	// audit P5-16: the sample probe runs under the inspection timeout.
	ctx, cancel := NewInspectionContext(context.Background())
	defer cancel()
	rows, err := p.pool.QueryContext(ctx, sqlQuery)
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()

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

		if err := rows.Scan(rowValues...); err != nil {
			return fmt.Errorf("error running layer (%v) SQL (%v): %w", l.name, sqlQuery, err)
		}

		for i := range rowValues {
			if rowValues[i] == nil || fields[i].name != l.GeomFieldName() {
				continue
			}

			raw, ok := blobBytes(probeRawValue(rowValues[i]))
			if !ok {
				return fmt.Errorf("layer (%v): unexpected MOS column type %T", l.name, rowValues[i])
			}

			// system info rows are metadata, not features: skip without
			// applying anything (audit A-01)
			if mos.IsSystemInfoBlob(raw) {
				continue
			}

			// The first decodable geometry infers the geometry type; later
			// rows in the window can still carry system-info metadata.
			if l.geomType == nil {
				g, derr := codec.DecodeMOS(raw, l.mosConfig)
				if derr != nil {
					if l.mapplSource == codec.MapplGISSQLSample {
						continue
					}
					return fmt.Errorf("layer (%v): %w", l.name, derr)
				}
				l.geomType = g
			}
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	// No decodable geometry found; leave geomType unset (nil) so the layer
	// still registers and MVT encoding stays permissive.
	return nil
}

// blobBytes normalizes driver values that may arrive as []byte or string.
func blobBytes(v interface{}) ([]byte, bool) {
	switch val := v.(type) {
	case []byte:
		return val, true
	case string:
		return []byte(val), true
	case *sql.NullString:
		return []byte(val.String), true
	}
	return nil, false
}

// probeMOSCustomSQLContract takes prepared, unwrapped SQL and runs the
// common InspectSQLGeometryContract probe over REAL scanned row values:
// bounds columns presence plus at least MinValidMOSRows decodable MOS
// geometries with coordinates. The probe SQL is prepared by the shared
// codec.PrepareProbeSQL helper: the statement always
// executes without a spatial filter and the sample reads at most
// codec.InspectionSampleLimit rows. SystemInfo rows are skipped, never
// applied: SQL-sample detection carries no projection contract. The actual
// result-column names are returned so the caller can persist them (A09).
func (p Provider) probeMOSCustomSQLContract(l *Layer, probeSQL string) ([]string, codec.SQLGeometryContract, error) {
	if probeSQL == "" {
		return nil, codec.SQLGeometryContract{}, fmt.Errorf("missing probing SQL")
	}
	if l.geometryFormat == codec.FormatMOS {
		probeSQL = codec.HANA.MetadataProbeSQL(probeSQL)
	} else {
		probeSQL = codec.WrapProbeSQLTopStyle(probeSQL)
	}
	// catch-all: drop any remaining code-context !TOKEN! the shared
	// preparation could not neutralize so a leftover placeholder cannot
	// break the probe statement (tokens in string literals or comments
	// are left verbatim)
	probeSQL = sqltoken.HANA.StripTokens(probeSQL)

	// audit P5-16: the sample probe runs under the inspection timeout.
	ctx, cancel := NewInspectionContext(context.Background())
	defer cancel()
	rows, err := p.pool.QueryContext(ctx, probeSQL)
	if err != nil {
		return nil, codec.SQLGeometryContract{}, err
	}
	defer func() { _ = rows.Close() }()

	columnTypes, err := rows.ColumnTypes()
	if err != nil {
		return nil, codec.SQLGeometryContract{}, err
	}
	columns := make([]string, len(columnTypes))
	for i, ct := range columnTypes {
		columns[i] = ct.Name()
	}

	// standard HANA row scan: typed targets derived from the result-column
	// descriptions, unwrapped to raw values for the shared probe (A01)
	fdescs, err := getFieldDescriptions(l.name, "", "", columnTypes, false)
	if err != nil {
		return nil, codec.SQLGeometryContract{}, err
	}
	rowValues := make([]interface{}, len(columns))

	// decision 3: candidate rows are decoded with the configured format
	// first; the auto closure falls back to a positive MOS signature so
	// native/WKB/WKT rows never count as MOS (7.1.3)
	decode := codec.AutoRowDecode(func(value interface{}) (geom.Geometry, error) {
		if codec.IsSystemInfoValue(value) {
			// LayerInfo blob describes the layer, not a geometry:
			// skip, never decode or apply.
			return nil, fmt.Errorf("system info blob")
		}
		return decodeGeometryValue(value, l.geometryFormat, l.mosConfig)
	}, l.mosConfig)
	if l.geometryFormat == codec.FormatMOS {
		decode = nil // explicit MOS needs structural metadata, not format inference
	}

	contract, cerr := codec.InspectSQLGeometryContract(
		func() ([]interface{}, bool, error) {
			if !rows.Next() {
				return nil, false, rows.Err()
			}
			setupRowValues(fdescs, rowValues)
			if err := rows.Scan(rowValues...); err != nil {
				return nil, false, err
			}
			return probeRawValues(rowValues), true, nil
		},
		columns,
		l.GeomFieldName(),
		l.bboxFields,
		decode,
	)
	if cerr != nil {
		return columns, codec.SQLGeometryContract{}, cerr
	}
	return columns, contract, nil
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
		mapNameVal := ctx.Value(observability.ObserveCtxKey(observability.ObserveVarMapName))
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

	// per-tile copy of the layer's resolved MOS config so runtime
	// MapplGIS LayerInfo application does not mutate provider state.
	mosCfg := plyr.mosConfig

	// buffered tile extent in the layer's source CRS, used by the exact
	// in-memory filter for raw geometry formats. Planar-equivalent layer
	// SRIDs are normalized inside tileBBoxInLayerCRS.
	tileBBox, berr := tileBBoxInLayerCRS(tile, plyr.SRID())
	if berr != nil {
		return fmt.Errorf("error converting tile extent for layer (%v): %w", layer, berr)
	}

	sqlQuery, err := replaceTokens(p.dbVersion, plyr.sql, &plyr, plyr.GeomType(), plyr.SRID(), tile, true)
	if err != nil {
		return fmt.Errorf("error replacing layer tokens for layer (%v) SQL (%v): %w", layer, sqlQuery, err)
	}

	// replace configured query parameters if any
	args := make([]interface{}, 0)
	sqlQuery = params.ReplaceParamsWithDialect(sqlQuery, &args, provider.SQLDialectHANA)
	if err != nil {
		return err
	}

	if debugExecuteSQL {
		log.Debugf("TEGOLA_SQL_DEBUG:EXECUTE_SQL for layer (%v): %v", layer, sqlQuery)
	}

	now := time.Now()

	extent, _, err := getTileExtent(tile, true)
	if err != nil {
		return fmt.Errorf("error getting tile extent for layer (%v): %w", layer, err)
	}
	srid := plyr.SRID()
	var rows *sql.Rows
	if codec.IsRawFormat(plyr.geometryFormat) {
		rows, err = p.pool.queryContextWithArgs(ctx, sqlQuery, args...)
	} else {
		rows, err = p.pool.QueryContextWithBBox(ctx, sqlQuery, extent, srid, false)
	}

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
	// when using ctxErr, it's import to make sure the defer func() { _ = rows.Close() }()
	// statement happens before the error check. The context may have been
	// canceled, but rows were also returned. If we don't close the rows
	// the the provider can't clean up the pool and the process will hang
	// trying to clean itself up.
	defer func() { _ = rows.Close() }()

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

		gid, geobytes, tags, err := readRowValues(ctx, &plyr, plyr.FieldDescriptions(), rowValues)
		if err := ctxErr(ctx, err); err != nil {
			return fmt.Errorf("for layer (%v) %w", plyr.Name(), err)
		}

		// check that we have geometry data. if not, skip the feature
		if len(geobytes) == 0 {
			continue
		}

		// The MOS layer self-description blob (MapplGIS LayerInfo) is
		// metadata, never a feature. Registration-time detection already
		// finalized the layer's MOS parameters (audit A-01): no runtime
		// application, the row is simply skipped.
		if plyr.geometryFormat == codec.FormatMOS && codec.IsSystemInfoValue(geobytes) {
			continue
		}

		// decode our geometry according to the layer's geometry format
		geometry, err := decodeGeometryValue(geobytes, plyr.geometryFormat, mosCfg)
		if err != nil {
			if plyr.mapplSource == codec.MapplGISSQLSample {
				log.Warnf("layer %v: skipping undecodable auto-detected MOS geometry: %v", layer, err)
				continue
			}
			if plyr.geometryFormat == "" {
				var ugt wkb.ErrUnknownGeometryType
				if errors.As(err, &ugt) {
					rplfn := layer + ":" + plyr.GeomFieldName()
					// Only report to the log once. This is to prevent the logs from filling up if there are many geometries in the layer
					if reportedLayerFieldName == "" || reportedLayerFieldName == rplfn {
						reportedLayerFieldName = rplfn
						log.Warnf("Ignoring unsupported geometry in layer (%v). Only basic 2D geometry type are supported. Try using `ST_Force2D(%v)`.", layer, plyr.GeomFieldName())
					}
					continue
				}
			}
			return fmt.Errorf("unable to decode layer (%v) geometry field (%v) into %v where (%v = %v): %w", layer, plyr.GeomFieldName(), geometryFormatName(plyr.geometryFormat), plyr.IDFieldName(), gid, err)
		}

		// Exact in-memory bbox filter. Mandatory for raw formats (wkb/wkt/
		// mos) and synthetic CRS layers whose SQL cannot use native spatial
		// predicates; harmless for native geometry already filtered via
		// !BBOX!.
		if (codec.IsRawFormat(plyr.geometryFormat) || isSyntheticCRS(plyr.SRID())) &&
			!codec.GeometryIntersectsExtent(geometry, tileBBox) {
			continue
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
		mapNameVal := ctx.Value(observability.ObserveCtxKey(observability.ObserveVarMapName))
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
			// A missing configured layer FAILS the MVT query (part12 0.6):
			// never continue with a zero-valued Layer and a logged warning
			// (the removed upstream behaviour). This is a missing-LAYER
			// error, deliberately distinct from the bounds-contract gate:
			// custom SQL whose SELECT list omits the bounds columns still
			// registers with a WARN (b5ad0979).
			return nil, fmt.Errorf("MVT query: configured layer is not registered with the provider (a missing layer fails the query; missing bounds columns in custom SQL only warn at registration): %w", ErrLayerNotFound{LayerName: layer.Name})
		}
		if debugLayerSQL {
			log.Debugf("SQL for Layer(%v):\n%v\n", l.Name(), l.sql)
		}

		sqlQuery, err := replaceTokens(p.dbVersion, l.sql, &l, l.GeomType(), l.SRID(), tile, false)
		if err := ctxErr(ctx, err); err != nil {
			return nil, err
		}

		// replace configured query parameters if any
		sqlQuery = params.ReplaceParamsWithDialect(sqlQuery, &args, provider.SQLDialectHANA)

		now := time.Now()

		extent, _, err := getTileExtent(tile, false)
		if err != nil {
			return nil, fmt.Errorf("error getting tile extent for layer (%v): %w", l.Name(), err)
		}
		srid := l.SRID()
		rows, err := p.pool.QueryContextWithBBox(ctx, sqlQuery, extent, srid, true)

		if err := ctxErr(ctx, err); err != nil {
			return []byte{}, err
		}

		defer func() { _ = rows.Close() }()

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
		if err := rows.Err(); err != nil {
			return []byte{}, err
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

// collectMapplGISMeta gathers the schema metadata required by the canonical
// MapplGIS table contract (provider/mapplgis) from the HANA catalog views:
//   - SYS.TABLE_COLUMNS supplies the DDL column list;
//   - SYS.REFERENCES_/SYS.INDEX_COLUMNS supplies the primary key and index
//     columns in key order.
//
// The OKEY = 1 row's LINE value is parsed by the returned fetcher. All
// identifiers are matched case-insensitively downstream (mapplgis.Detect).
func collectMapplGISMeta(ctx context.Context, pool *connectionPoolCollector, tblName string) (mapplgis.TableMeta, mapplgis.SystemInfoFetcher, error) {
	var meta mapplgis.TableMeta

	qtn := quoteTableName(tblName)
	// HANA catalog views are scoped by schema name; derive it with the
	// quote-aware parser so dots inside quoted identifiers are not treated
	// as separators (audit P5-3). Unqualified tables resolve against the
	// connection's CURRENT SCHEMA and are matched accordingly. The parsed
	// parts are only used as catalog predicate arguments (bound as
	// parameters), never interpolated into SQL.
	schemaName, bareTableName, err := splitQualifiedTableName(tblName)
	if err != nil {
		return meta, nil, fmt.Errorf("invalid table name %q: %w", tblName, err)
	}

	// DDL columns.
	schemaPred := "1=1"
	args := []interface{}{}
	if schemaName != "" {
		schemaPred = `SCHEMA_NAME = ?`
		args = append(args, schemaName)
	}
	args = append(args, bareTableName)

	colRows, err := pool.pool.QueryContext(ctx, fmt.Sprintf(`
		SELECT COLUMN_NAME
		FROM SYS.TABLE_COLUMNS
		WHERE %v AND TABLE_NAME = ?
		ORDER BY POSITION`, schemaPred), args...)
	if err != nil {
		return meta, nil, fmt.Errorf("unable to list columns of table %v: %w", qtn, err)
	}
	defer func() { _ = colRows.Close() }()

	for colRows.Next() {
		var name string
		if serr := colRows.Scan(&name); serr != nil {
			return meta, nil, fmt.Errorf("unable to scan columns of table %v: %w", qtn, serr)
		}
		meta.Columns = append(meta.Columns, name)
	}
	if err := colRows.Err(); err != nil {
		return meta, nil, fmt.Errorf("error iterating columns of table %v: %w", qtn, err)
	}
	if len(meta.Columns) == 0 {
		return meta, nil, fmt.Errorf("table %v does not exist", qtn)
	}

	// Primary key columns in key order.
	pkArgs := []interface{}{}
	pkSchemaPred := "ic.SCHEMA_NAME = CURRENT_SCHEMA"
	if schemaName != "" {
		pkSchemaPred = "ic.SCHEMA_NAME = ?"
		pkArgs = append(pkArgs, schemaName)
	}
	pkArgs = append(pkArgs, bareTableName)
	pkRows, err := pool.pool.QueryContext(ctx, fmt.Sprintf(`
		SELECT ic.COLUMN_NAME
		FROM SYS.INDEX_COLUMNS ic
		JOIN SYS.INDEXES i
			ON i.SCHEMA_NAME = ic.SCHEMA_NAME AND i.INDEX_NAME = ic.INDEX_NAME
		WHERE %v AND ic.TABLE_NAME = ? AND i.CONSTRAINT = 'PRIMARY KEY'
		ORDER BY ic.POSITION`, pkSchemaPred), pkArgs...)
	if err != nil {
		return meta, nil, fmt.Errorf("unable to list primary key of table %v: %w", qtn, err)
	}
	defer func() { _ = pkRows.Close() }()

	for pkRows.Next() {
		var name string
		if serr := pkRows.Scan(&name); serr != nil {
			return meta, nil, fmt.Errorf("unable to scan primary key of table %v: %w", qtn, serr)
		}
		meta.PrimaryKeyColumns = append(meta.PrimaryKeyColumns, name)
	}
	if err := pkRows.Err(); err != nil {
		return meta, nil, fmt.Errorf("error iterating primary key of table %v: %w", qtn, err)
	}

	// Every index with its ordered column list.
	idxArgs := []interface{}{}
	idxSchemaPred := "i.SCHEMA_NAME = CURRENT_SCHEMA"
	if schemaName != "" {
		idxSchemaPred = "i.SCHEMA_NAME = ?"
		idxArgs = append(idxArgs, schemaName)
	}
	idxArgs = append(idxArgs, bareTableName)
	idxRows, err := pool.pool.QueryContext(ctx, fmt.Sprintf(`
		SELECT i.INDEX_NAME, ic.COLUMN_NAME
		FROM SYS.INDEXES i
		JOIN SYS.INDEX_COLUMNS ic
			ON ic.SCHEMA_NAME = i.SCHEMA_NAME AND ic.INDEX_NAME = i.INDEX_NAME AND ic.TABLE_NAME = i.TABLE_NAME
		WHERE %v AND i.TABLE_NAME = ?
		ORDER BY i.INDEX_NAME, ic.POSITION`, idxSchemaPred), idxArgs...)
	if err != nil {
		return meta, nil, fmt.Errorf("unable to list indexes of table %v: %w", qtn, err)
	}
	defer func() { _ = idxRows.Close() }()

	indexes := make(map[string]*mapplgis.IndexMeta)
	var order []string
	for idxRows.Next() {
		var idxName, colName string
		if serr := idxRows.Scan(&idxName, &colName); serr != nil {
			return meta, nil, fmt.Errorf("unable to scan indexes of table %v: %w", qtn, serr)
		}
		idx, ok := indexes[idxName]
		if !ok {
			idx = &mapplgis.IndexMeta{Name: idxName}
			indexes[idxName] = idx
			order = append(order, idxName)
		}
		idx.Columns = append(idx.Columns, colName)
	}
	if err := idxRows.Err(); err != nil {
		return meta, nil, fmt.Errorf("error iterating indexes of table %v: %w", qtn, err)
	}
	for _, name := range order {
		meta.Indexes = append(meta.Indexes, *indexes[name])
	}

	fetch := func() (*mos.SystemInfo, error) {
		var blob []byte
		err := pool.pool.QueryRowContext(ctx, fmt.Sprintf(
			`SELECT %v FROM %v WHERE %v = 1 AND %v IS NOT NULL LIMIT 1`,
			quoteIdentifier(mapplgis.GeometryField), qtn,
			quoteIdentifier(mapplgis.PrimaryKey), quoteIdentifier(mapplgis.GeometryField),
		)).Scan(&blob)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, nil
			}
			return nil, err
		}
		if len(blob) == 0 {
			return nil, nil
		}
		si, perr := mos.ParseSystemInfo(blob)
		if perr != nil {
			return nil, perr
		}
		return &si, nil
	}

	return meta, fetch, nil
}

// detectMapplGIS applies the canonical MapplGIS table contract to a
// tablename layer (audit A06). On detection the effective geometry format
// is authoritative MOS (A04): unset/auto resolve to MOS and an explicit
// non-MOS format is a startup conflict error. The MOS system-info applies
// the self-described projection to the layer unless the CRS config was
// explicit, so the subsequent SRID resolution sees a resolved value.
func detectMapplGIS(ctx context.Context, pool *connectionPoolCollector, l *Layer, tblName string) (bool, error) {
	// audit P5-16: the metadata probes run under the inspection timeout,
	// honoring the plumbed parent context (nil means Background).
	ctx, cancel := NewInspectionContext(ctx)
	defer cancel()
	meta, fetch, err := collectMapplGISMeta(ctx, pool, tblName)
	if err != nil {
		return false, err
	}
	info, err := mapplgis.Detect(meta, fetch)
	if err != nil {
		return false, fmt.Errorf("table %v: %w", tblName, err)
	}
	if !info.IsMapplGIS {
		return false, nil
	}

	switch l.geometryFormat {
	case "", codec.FormatMOS:
		l.geometryFormat = codec.FormatMOS
	default:
		return false, fmt.Errorf(
			"table %v is detected as a MapplGIS layer (MOS storage), but geometry_format is explicitly %q; remove the setting or use %q",
			tblName, l.geometryFormat, codec.FormatMOS,
		)
	}

	if aerr := l.mosConfig.ApplySystemInfo(&info.SystemInfo); aerr != nil {
		return false, fmt.Errorf("table %v apply MOS system info: %v", tblName, aerr)
	}
	l.isMapplGIS = true
	l.mapplSource = codec.MapplGISTableCanonical
	l.mapplSysInfo = info.SystemInfo
	if srid, applied, aerr := crsconfig.ApplySystemInfoCRS(int(l.srid), false, info.SystemInfo.Projection); aerr != nil {
		return false, fmt.Errorf("table %v apply MOS projection: %v", tblName, aerr)
	} else if applied {
		l.srid = uint64(srid)
	}
	return true, nil
}

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
