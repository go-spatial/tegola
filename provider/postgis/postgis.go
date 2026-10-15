package postgis

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/alexeydott/geom"
	"github.com/alexeydott/geom/encoding/wkb"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/alexeydott/tegola"
	"github.com/alexeydott/tegola/basic"
	conf "github.com/alexeydott/tegola/config"
	"github.com/alexeydott/tegola/dict"
	"github.com/alexeydott/tegola/internal/log"
	"github.com/alexeydott/tegola/internal/sqltoken"
	"github.com/alexeydott/tegola/mos"
	"github.com/alexeydott/tegola/observability"
	"github.com/alexeydott/tegola/provider"
	"github.com/alexeydott/tegola/provider/crsconfig"
	codec "github.com/alexeydott/tegola/provider/geometrycodec"
	"github.com/alexeydott/tegola/provider/mapplgis"
	"github.com/jackc/pgx/v5"
)

const Name = "postgis"

const (
	// We quote the field and table names to prevent colliding with postgres keywords.
	stdSQL = `SELECT %[1]v FROM %[2]v WHERE %[3]v && ` + conf.BboxToken
	mvtSQL = `SELECT %[1]v FROM %[2]v`

	// SQL to get the column names, without hitting the information_schema.
	// Though it might be better to hit the information_schema.
	fldsSQL = `SELECT * FROM %[1]v LIMIT 0;`
)

const (
	DefaultSRID                       = tegola.WebMercator
	DefaultSSLMode                    = "prefer"
	DefaultSSLKey                     = ""
	DefaultSSLCert                    = ""
	DefaultApplicationName            = "tegola"
	DefaultDefaultTransactionReadOnly = "TRUE"
)

const (
	ConfigKeyName                       = "name"
	ConfigKeyURI                        = "uri"
	ConfigKeySSLMode                    = "ssl_mode"
	ConfigKeySSLKey                     = "ssl_key"
	ConfigKeySSLCert                    = "ssl_cert"
	ConfigKeySSLRootCert                = "ssl_root_cert"
	ConfigKeySRID                       = "srid"
	ConfigKeyCRSDefn                    = "crs_defn"
	ConfigKeyLayers                     = "layers"
	ConfigKeyLayerName                  = "name"
	ConfigKeyTablename                  = "tablename"
	ConfigKeySQL                        = "sql"
	ConfigKeyFields                     = "fields"
	ConfigKeyGeomField                  = "geometry_fieldname"
	ConfigKeyGeomIDField                = "id_fieldname"
	ConfigKeyGeomType                   = "geometry_type"
	ConfigKeyApplicationName            = "application_name"
	ConfigKeyDefaultTransactionReadOnly = "default_transaction_read_only"
	ConfigKeyPoolMinConns               = "pool_min_conns"
	ConfigKeyPoolMinIdleConns           = "pool_min_idle_conns"
	ConfigKeyPoolMaxConns               = "pool_max_conns"
	ConfigKeyPoolMaxConnLifeTime        = "pool_max_conn_lifetime"
	// canonical spelling; ConfigKeyPoolMaxConnIdleTime (missing the
	// separating underscore) is kept as a deprecated alias
	ConfigKeyPoolMaxConnIdleTimeCanonical = "pool_max_conn_idle_time"
	ConfigKeyPoolMaxConnIdleTime          = "pool_max_conn_idletime"
	ConfigKeyPoolHealthCheckPeriod        = "pool_health_check_period"
	ConfigKeyPoolMaxConnLifeTimeJitter    = "pool_max_conn_lifetime_jitter"
)

var (
	// isSelectQuery is a regexp to check if a query starts with `SELECT`,
	// case-insensitive and ignoring any preceding whitespace and SQL comments.
	isSelectQuery = regexp.MustCompile(`(?i)^((\s*)(--.*\n)?)*select`)

	// reference to all instantiated providers
	providers []*Provider
)

// Provider provides the postgis data provider.
type Provider struct {
	config pgxpool.Config
	name   string
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

	// R08: cached mutation writer (with mutex).
	writerMu     sync.Mutex
	cachedWriter *Writer

	// Collectors for Query times
	mvtProviderQueryHistogramSeconds *prometheus.HistogramVec
	queryHistogramSeconds            *prometheus.HistogramVec
}

func (p *Provider) Collectors(
	prefix string,
	cfgFn func(configKey string) map[string]any,
) ([]observability.Collector, error) {
	if p.collectorsRegistered {
		return nil, nil
	}

	buckets := []float64{.1, 1, 5, 20}
	c, err := p.pool.Collectors(prefix, cfgFn)
	if err != nil {
		return nil, err
	}

	// a constant label ensures that the metrics are unique
	// this allows the registration of multiple providers in the same
	// config.
	// Additional label names will be appended to the constant labels.
	p.mvtProviderQueryHistogramSeconds = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:        prefix + "_mvt_provider_sql_query_seconds",
			Help:        "A histogram of query time for sql for mvt providers",
			Buckets:     buckets,
			ConstLabels: prometheus.Labels{"provider_name": p.name},
		},
		[]string{"map_name", "z"},
	)

	p.queryHistogramSeconds = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:        prefix + "_provider_sql_query_seconds",
			Help:        "A histogram of query time for sql for providers",
			Buckets:     buckets,
			ConstLabels: prometheus.Labels{"provider_name": p.name},
		},
		[]string{"map_name", "layer_name", "z"},
	)

	p.collectorsRegistered = true
	return append(c, p.mvtProviderQueryHistogramSeconds, p.queryHistogramSeconds), nil
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
func (p *Provider) Layers() ([]provider.LayerInfo, error) {
	ls := []provider.LayerInfo{}

	for i := range p.layers {
		ls = append(ls, p.layers[i])
	}

	return ls, nil
}

// TileFeatures adheres to the provider.Tiler any
// resolveGeometryFormatConfig validates the geometry_format config value.
// An empty string means the provider default (native PostGIS geometry).
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

// postgisGeometryFormats is the set of geometry_format values accepted at
// the layer level (an empty value falls back to the provider-level value).
var postgisGeometryFormats = map[string]struct{}{
	codec.FormatWKB: {},
	codec.FormatWKT: {},
	codec.FormatMOS: {},
}

// decodeGeometryValue decodes a raw geometry column value according to the
// layer's geometry format. The default (empty) format expects PostGIS native
// geometry already serialized via ST_AsBinary, decoded as WKB.
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

func (p *Provider) TileFeatures(
	ctx context.Context,
	layer string,
	tile provider.Tile,
	params provider.Params,
	fn func(f *provider.Feature) error,
) error {
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

	// buffered tile extent in WebMercator, used by the exact in-memory
	// filter for raw geometry formats.
	webMercatorBBox, _ := tile.BufferedExtent()
	tileBBox := webMercatorBBox
	if plyr.SRID() != tegola.WebMercator {
		sourceBBox, berr := basic.FromWebMercatorExtent(plyr.SRID(), tileBBox)
		if berr != nil {
			return fmt.Errorf("error converting tile extent for layer (%v): %w", layer, berr)
		}
		tileBBox = sourceBBox
	}

	sql, err := replaceTokens(plyr.sql, &plyr, tile, true)
	if err := ctxErr(ctx, err); err != nil {
		return fmt.Errorf(
			"error replacing layer tokens for layer (%v) SQL (%v): %w",
			layer,
			sql,
			err,
		)
	}

	// replace configured query parameters if any
	args := make([]any, 0)
	sql = params.ReplaceParamsWithDialect(sql, &args, provider.SQLDialectPostgreSQL)
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
	// the provider can't clean up the pool and the process will hang
	// trying to clean itself up.
	defer rows.Close()
	if err := ctxErr(ctx, err); err != nil {
		return fmt.Errorf(
			"error running layer (%v) SQL (%v) with args %v: %w",
			layer,
			sql,
			args,
			err,
		)
	}

	// fieldDescriptions
	var fdescs []pgconn.FieldDescription

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

		gid, geobytes, tags, err := decipherFields(
			ctx,
			plyr.GeomFieldName(),
			plyr.IDFieldName(),
			&plyr.bboxFields,
			fdescs,
			vals,
		)
		if err := ctxErr(ctx, err); err != nil {
			return fmt.Errorf("for layer (%v) %w", plyr.Name(), err)
		}

		// check that we have geometry data. if not, skip the feature
		if len(geobytes) == 0 {
			continue
		}

		// The MOS layer self-description blob (MapplGIS LayerInfo) is
		// metadata, never a feature. Registration-time detection already
		// finalized the layer's MOS parameters and CRS (audit A-01): no
		// runtime application, the row is simply skipped.
		if plyr.geometryFormat == codec.FormatMOS && codec.IsSystemInfoValue(geobytes) {
			continue
		}

		// decode our geometry according to the layer's geometry format
		geometry, err := decodeGeometryValue(geobytes, plyr.geometryFormat, plyr.mosConfig)
		if err != nil {
			if plyr.mapplSource == codec.MapplGISSQLSample {
				log.Warnf("layer %v: skipping undecodable auto-detected MOS geometry: %v", layer, err)
				continue
			}
			if plyr.geometryFormat == "" {
				var ugt wkb.ErrUnknownGeometryType
				if errors.As(err, &ugt) {
					rplfn := layer + ":" + plyr.GeomFieldName()
					// Only report to the log once.
					// This is to prevent the logs from filling up if there are many geometries in the layer
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
		// mos) whose SQL cannot use native spatial predicates; harmless for
		// native geometry already filtered via !BBOX!.
		if !codec.GeometryIntersectsExtent(geometry, tileBBox) {
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

// geometryFormatName returns a human-readable name for a geometry format,
// used in error messages.
func geometryFormatName(format string) string {
	if format == "" {
		return "wkb"
	}
	return format
}

// splitTableName splits an optionally schema-qualified table name into its
// schema and table parts; the PostGIS default schema "public" is assumed
// when no qualifier is present. Quoted identifiers ("...") are understood:
// the split only happens at top-level dots, "" escapes are unquoted and
// trailing garbage after a closing quote is rejected, so names containing
// dots or quotes resolve correctly and cannot smuggle SQL into the
// metadata lookups (audit N7). Unquoted input keeps the historical
// first-dot split.
func splitTableName(tbl string) (schema, table string, err error) {
	if !strings.Contains(tbl, `"`) {
		// no quoted identifiers: keep the historical first-dot split
		if i := strings.Index(tbl, "."); i >= 0 {
			return tbl[:i], tbl[i+1:], nil
		}
		return "public", tbl, nil
	}

	parts, perr := parseIdentParts(tbl)
	if perr != nil {
		return "", "", perr
	}
	switch len(parts) {
	case 1:
		return "public", parts[0], nil
	case 2:
		return parts[0], parts[1], nil
	default:
		return "", "", fmt.Errorf("table name %q has more than two parts", tbl)
	}
}

// parseIdentParts splits a possibly qualified identifier at top-level dots,
// unquoting each "..." part ("" -> "). Bare parts pass through verbatim but
// may not contain quotes; a quoted part must be followed by a separator or
// the end of the input, and empty parts are rejected.
func parseIdentParts(s string) ([]string, error) {
	orig := s
	var parts []string
	for {
		var part string
		if strings.HasPrefix(s, `"`) {
			s = s[1:]
			var b strings.Builder
			closed := false
			for len(s) > 0 && !closed {
				switch {
				case s[0] == '"' && len(s) > 1 && s[1] == '"':
					b.WriteByte('"')
					s = s[2:]
				case s[0] == '"':
					s = s[1:]
					closed = true
				default:
					b.WriteByte(s[0])
					s = s[1:]
				}
			}
			if !closed {
				return nil, fmt.Errorf("unterminated quoted identifier in %q", orig)
			}
			part = b.String()
			if part == "" {
				return nil, fmt.Errorf("empty identifier in %q", orig)
			}
			if s != "" && s[0] != '.' {
				return nil, fmt.Errorf("unexpected characters after quoted identifier in %q", orig)
			}
		} else {
			var raw string
			if i := strings.IndexByte(s, '.'); i >= 0 {
				raw, s = s[:i], s[i:]
			} else {
				raw, s = s, ""
			}
			if raw == "" {
				return nil, fmt.Errorf("empty identifier in %q", orig)
			}
			if strings.Contains(raw, `"`) {
				return nil, fmt.Errorf("unexpected quote in identifier %q", raw)
			}
			part = raw
		}
		parts = append(parts, part)
		if s == "" {
			return parts, nil
		}
		// both branches leave a leading '.' separator (the quoted branch
		// has already rejected anything else)
		s = s[1:]
		if s == "" {
			return nil, fmt.Errorf("trailing dot in table name %q", orig)
		}
	}
}

// inferTableSRID looks up the source SRID of a native geometry column in the
// PostGIS spatial metadata. It backs the documented source-SRID auto-detect
// for table layers and is only consulted when neither the provider nor the
// layer CRS was configured explicitly. Unknown tables and mixed-SRID columns
// produce a controlled error instead of a silent default.
// findSRIDQuery builds the PostGIS Find_SRID metadata lookup. The schema,
// table and geometry column names travel as query parameters and never
// enter the SQL text, so names containing quotes, dots, semicolons or
// other special characters cannot inject SQL (audit N7).
func findSRIDQuery(schema, table, geomField string) (string, []any) {
	return `SELECT Find_SRID($1, $2, $3)`, []any{schema, table, geomField}
}

func inferTableSRID(ctx context.Context, pool *connectionPoolCollector, schema, table, geomField string) (uint64, error) {
	var srid int
	query, args := findSRIDQuery(schema, table, geomField)
	err := pool.QueryRow(ctx, query, args...).Scan(&srid)
	if err != nil {
		return 0, err
	}
	if srid <= 0 {
		return 0, fmt.Errorf("Find_SRID returned invalid SRID %v", srid)
	}
	return uint64(srid), nil
}

// mapplGISSystemInfoSQL builds the OKEY = 1 system-info fetch statement for
// a MapplGIS table. All identifiers are quoted via pgQuoteIdent (embedded
// quotes doubled), so adversarial schema/table names cannot inject SQL
// (audit N7).
func mapplGISSystemInfoSQL(schema, table string) string {
	return fmt.Sprintf(
		`SELECT %[1]v FROM %[2]v.%[3]v WHERE "OKEY" = 1 AND %[1]v IS NOT NULL LIMIT 1`,
		pgQuoteIdent(mapplgis.GeometryField),
		pgQuoteIdent(schema),
		pgQuoteIdent(table),
	)
}

// collectMapplGISMeta gathers the schema metadata required by the canonical
// MapplGIS table contract (provider/mapplgis) from the PostgreSQL catalogs:
//   - information_schema.columns supplies the DDL column list;
//   - pg_constraint (contype = 'p') supplies the primary key columns in
//     key order;
//   - pg_index/pg_attribute supplies every index with its ordered column
//     list.
//
// The OKEY = 1 row's LINE value is parsed by the returned fetcher. All
// identifiers are matched case-insensitively downstream (mapplgis.Detect).
func collectMapplGISMeta(ctx context.Context, pool *connectionPoolCollector, schema, table string) (mapplgis.TableMeta, mapplgis.SystemInfoFetcher, error) {
	var meta mapplgis.TableMeta

	// DDL columns.
	colRows, err := pool.Query(ctx, `
		SELECT column_name
		FROM information_schema.columns
		WHERE table_schema = $1 AND table_name = $2
		ORDER BY ordinal_position`, schema, table)
	if err != nil {
		return meta, nil, fmt.Errorf("unable to list columns of table %v.%v: %w", schema, table, err)
	}
	for colRows.Next() {
		var name string
		if serr := colRows.Scan(&name); serr != nil {
			colRows.Close()
			return meta, nil, fmt.Errorf("unable to scan columns of table %v.%v: %w", schema, table, serr)
		}
		meta.Columns = append(meta.Columns, name)
	}
	if err := colRows.Err(); err != nil {
		colRows.Close()
		return meta, nil, fmt.Errorf("error iterating columns of table %v.%v: %w", schema, table, err)
	}
	colRows.Close()
	if len(meta.Columns) == 0 {
		return meta, nil, fmt.Errorf("table %v.%v does not exist", schema, table)
	}

	// Primary key columns in key order. pg_index.indkey is an int2vector:
	// cast to text and split in Go so the query stays compatible with old
	// Postgres versions that lack unnest(...) WITH ORDINALITY in joins.
	pkRows, err := pool.Query(ctx, `
		SELECT i.indkey::text
		FROM pg_index i
		JOIN pg_class c ON c.oid = i.indrelid
		JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE n.nspname = $1 AND c.relname = $2 AND i.indisprimary`, schema, table)
	if err != nil {
		return meta, nil, fmt.Errorf("unable to list primary key of table %v.%v: %w", schema, table, err)
	}
	var primaryIndkeys []string
	for pkRows.Next() {
		var indkey string
		if serr := pkRows.Scan(&indkey); serr != nil {
			pkRows.Close()
			return meta, nil, fmt.Errorf("unable to scan primary key of table %v.%v: %w", schema, table, serr)
		}
		primaryIndkeys = append(primaryIndkeys, indkey)
	}
	if err := pkRows.Err(); err != nil {
		pkRows.Close()
		return meta, nil, fmt.Errorf("error iterating primary key of table %v.%v: %w", schema, table, err)
	}
	pkRows.Close()
	for _, indkey := range primaryIndkeys {
		names, err := indexColumnNames(ctx, pool, schema, table, indkey)
		if err != nil {
			return meta, nil, err
		}
		meta.PrimaryKeyColumns = append(meta.PrimaryKeyColumns, names...)
	}

	// Every index with its ordered column list.
	idxRows, err := pool.Query(ctx, `
		SELECT c2.relname, i.indkey::text
		FROM pg_index i
		JOIN pg_class c ON c.oid = i.indrelid
		JOIN pg_namespace n ON n.oid = c.relnamespace
		JOIN pg_class c2 ON c2.oid = i.indexrelid
		WHERE n.nspname = $1 AND c.relname = $2
		ORDER BY c2.relname`, schema, table)
	if err != nil {
		return meta, nil, fmt.Errorf("unable to list indexes of table %v.%v: %w", schema, table, err)
	}
	indexes := make(map[string]*mapplgis.IndexMeta)
	var order []string
	type indexDescriptor struct{ name, indkey string }
	var descriptors []indexDescriptor
	for idxRows.Next() {
		var idxName, indkey string
		if serr := idxRows.Scan(&idxName, &indkey); serr != nil {
			idxRows.Close()
			return meta, nil, fmt.Errorf("unable to scan indexes of table %v.%v: %w", schema, table, serr)
		}
		descriptors = append(descriptors, indexDescriptor{idxName, indkey})
	}
	if err := idxRows.Err(); err != nil {
		idxRows.Close()
		return meta, nil, fmt.Errorf("error iterating indexes of table %v.%v: %w", schema, table, err)
	}
	idxRows.Close()
	for _, descriptor := range descriptors {
		names, err := indexColumnNames(ctx, pool, schema, table, descriptor.indkey)
		if err != nil {
			return meta, nil, err
		}
		idx, ok := indexes[descriptor.name]
		if !ok {
			idx = &mapplgis.IndexMeta{Name: descriptor.name}
			indexes[descriptor.name] = idx
			order = append(order, descriptor.name)
		}
		idx.Columns = append(idx.Columns, names...)
	}
	for _, name := range order {
		meta.Indexes = append(meta.Indexes, *indexes[name])
	}

	fetch := func() (*mos.SystemInfo, error) {
		var blob []byte
		err := pool.QueryRow(ctx, mapplGISSystemInfoSQL(schema, table)).Scan(&blob)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
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

// indexColumnNames resolves a pg_index.indkey int2vector (its text form,
// e.g. "1 4") to the ordered column names of the table. Expression keys
// (0 entries) are skipped — they never match the plain-column MapplGIS
// contract.
func indexColumnNames(ctx context.Context, pool *connectionPoolCollector, schema, table, indkey string) ([]string, error) {
	var names []string
	for _, part := range strings.Fields(indkey) {
		attnum, err := strconv.Atoi(part)
		if err != nil {
			return nil, fmt.Errorf("table %v.%v: malformed index key %q: %w", schema, table, indkey, err)
		}
		if attnum <= 0 {
			continue
		}
		var name string
		err = pool.QueryRow(ctx, `
			SELECT a.attname
			FROM pg_class c
			JOIN pg_namespace n ON n.oid = c.relnamespace
			JOIN pg_attribute a ON a.attrelid = c.oid AND a.attnum = $3
			WHERE n.nspname = $1 AND c.relname = $2`, schema, table, attnum).Scan(&name)
		if err != nil {
			return nil, fmt.Errorf("table %v.%v: unable to resolve index column %v: %w", schema, table, attnum, err)
		}
		names = append(names, name)
	}
	return names, nil
}

// detectMapplGIS applies the canonical MapplGIS table contract to a
// tablename layer (audit A06). On detection the effective geometry format
// is authoritative MOS (A04): unset/auto resolve to MOS and an explicit
// non-MOS format is a startup conflict error. The MOS system-info is
// applied to the layer; the projection becomes the layer SRID unless the
// config was explicit.
func detectMapplGIS(ctx context.Context, pool *connectionPoolCollector, l *Layer, schema, table string) (bool, error) {
	meta, fetch, err := collectMapplGISMeta(ctx, pool, schema, table)
	if err != nil {
		return false, err
	}
	info, err := mapplgis.Detect(meta, fetch)
	if err != nil {
		return false, fmt.Errorf("table %v.%v: %w", schema, table, err)
	}
	if !info.IsMapplGIS {
		return false, nil
	}

	switch l.geometryFormat {
	case "", codec.FormatMOS:
		l.geometryFormat = codec.FormatMOS
	default:
		return false, fmt.Errorf(
			"table %v.%v is detected as a MapplGIS layer (MOS storage), but geometry_format is explicitly %q; remove the setting or use %q",
			schema, table, l.geometryFormat, codec.FormatMOS,
		)
	}

	if aerr := l.mosConfig.ApplySystemInfo(&info.SystemInfo); aerr != nil {
		return false, fmt.Errorf("table %v.%v apply MOS system info: %v", schema, table, aerr)
	}
	l.isMapplGIS = true
	l.mapplSource = codec.MapplGISTableCanonical
	l.mapplSysInfo = info.SystemInfo
	if srid, applied, aerr := crsconfig.ApplySystemInfoCRS(int(l.srid), l.crsExplicit, info.SystemInfo.Projection); aerr != nil {
		return false, fmt.Errorf("table %v.%v apply MOS projection: %v", schema, table, aerr)
	} else if applied {
		l.srid = uint64(srid)
		l.crsExplicit = true
	}
	return true, nil
}

// mosProbeSQL builds the shared bounds-contract probe query for a MOS
// layer's custom SQL (audit R1). The SQL is prepared with the documented
// codec.PrepareProbeSQL token order — !BBOX!/!BOX! neutralize to "1=1"
// (never "TRUE", so tokens inside SQL function arguments stay syntactically
// valid) and zoom/position placeholders expand permissively — then wrapped
// in the shared InspectionSampleLimit sample window
// (docs/provider-contract.md). Custom parameter tokens are stripped for
// inspection; if the query cannot run without them the user must set
// geometry_type in the config.
func mosProbeSQL(l *Layer) string {
	probeGeomType := ""
	if l.geomType != nil {
		probeGeomType = codec.GeomTypeName(l.geomType)
	}
	sql := codec.PostgreSQL.PrepareProbeSQL(l.sql, l.geomField, l.idField, probeGeomType)
	sql = sqltoken.PostgreSQL.StripTokens(sql)
	return codec.WrapProbeSQL(sql)
}

// inspectMOSLayerGeomType samples the first rows of the layer's SQL and
// derives the geometry type from the first decodable MOS geometry.
func (p *Provider) inspectMOSLayerGeomType(l *Layer) error {
	probeSQL := mosProbeSQL(l)

	ctx, cancel := codec.NewInspectionContext()
	defer cancel()

	rows, err := p.pool.Query(ctx, probeSQL)
	if err != nil {
		return err
	}
	defer rows.Close()

	return inspectMOSGeomTypeRows(l, probeSQL, rows)
}

// inspectMOSGeomTypeRows derives the layer geometry type from sampled probe
// rows. The MapplGIS LayerInfo blob is metadata: rows carrying it are
// skipped without applying anything (audit A-01 — custom SQL never
// auto-applies system info; detection belongs to the registration-time
// table contract).
func inspectMOSGeomTypeRows(l *Layer, probeSQL string, rows pgx.Rows) error {
	fdescs := rows.FieldDescriptions()
	for rows.Next() {
		vals, err := rows.Values()
		if err != nil {
			return fmt.Errorf("error running SQL: %v ; %w", probeSQL, err)
		}

		for i := range vals {
			if string(fdescs[i].Name) != l.geomField || vals[i] == nil {
				continue
			}

			raw, ok := vals[i].([]byte)
			if !ok {
				return fmt.Errorf("layer (%v): unexpected MOS column type %T", l.name, vals[i])
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

// LayerFields returns a map of field names to their types for a given layer.
// It executes a sample query (LIMIT 0) to get column information without fetching data.
func (p *Provider) LayerFields(ctx context.Context, layerName string) (map[string]any, error) {
	plyr, ok := p.Layer(layerName)
	if !ok {
		return nil, ErrLayerNotFound{layerName}
	}

	// Use a dummy tile to replace tokens in the SQL
	dummyTile := provider.NewTile(0, 0, 0, 256, 3857)
	sql, err := replaceTokens(plyr.sql, &plyr, dummyTile, true)
	if err != nil {
		return nil, fmt.Errorf("error replacing layer tokens for layer (%s): %w", layerName, err)
	}

	// Wrap the query to get column information without fetching rows
	sql = fmt.Sprintf("SELECT * FROM (%s) AS subquery LIMIT 0", sql)

	rows, err := p.pool.Query(ctx, sql)
	if err != nil {
		return nil, fmt.Errorf("error querying fields for layer (%s): %w", layerName, err)
	}
	defer rows.Close()

	fields := make(map[string]any)
	fdescs := rows.FieldDescriptions()

	for _, desc := range fdescs {
		fieldName := desc.Name

		// Skip geometry and ID fields as they're not attributes
		if fieldName == plyr.GeomFieldName() || fieldName == plyr.IDFieldName() {
			continue
		}

		// Map PostgreSQL OID types to simple type names
		fieldType := postgresTypeToString(desc.DataTypeOID)
		fields[fieldName] = fieldType
	}

	return fields, nil
}

// postgresTypeToString converts PostgreSQL OID types to simple type strings
func postgresTypeToString(oid uint32) string {
	// Common PostgreSQL type OIDs using pgtype constants
	switch oid {
	case pgtype.BoolOID:
		return "Boolean"
	case pgtype.Int8OID, pgtype.Int2OID, pgtype.Int4OID, pgtype.OIDOID:
		return "Number"
	case pgtype.Float4OID, pgtype.Float8OID, pgtype.NumericOID:
		return "Number"
	case pgtype.QCharOID, pgtype.NameOID, pgtype.TextOID, pgtype.BPCharOID, pgtype.VarcharOID:
		return "String"
	case pgtype.TimestampOID, pgtype.TimestamptzOID:
		return "String"
	case pgtype.JSONOID, pgtype.JSONBOID:
		return "String"
	default:
		return "String"
	}
}

func (p *Provider) MVTForLayers(
	ctx context.Context,
	tile provider.Tile,
	params provider.Params,
	layers []provider.Layer,
) ([]byte, error) {
	var (
		err     error
		sqls    = make([]string, 0, len(layers))
		mapName string
	)

	{
		mapNameVal := ctx.Value(observability.ObserveCtxKey(observability.ObserveVarMapName))
		if mapNameVal != nil {
			// if it's not convertible to a string, we will ignore it.
			mapName, _ = mapNameVal.(string)
		}
	}

	args := make([]any, 0)

	for i := range layers {
		if debug {
			log.Debugf("looking for layer: %v", layers[i])
		}
		l, ok := p.Layer(layers[i].Name)
		if !ok {
			// A missing configured layer FAILS the MVT query (part12 0.6):
			// never continue with a zero-valued Layer and a logged warning
			// (the removed upstream behaviour). This is a missing-LAYER
			// error, deliberately distinct from the bounds-contract gate:
			// custom SQL whose SELECT list omits the bounds columns still
			// registers with a WARN (b5ad0979).
			return nil, fmt.Errorf("MVT query: configured layer is not registered with the provider (a missing layer fails the query; missing bounds columns in custom SQL only warn at registration): %w", ErrLayerNotFound{LayerName: layers[i].Name})
		}
		if debugLayerSQL {
			log.Debugf("SQL for Layer(%v):\n%v\nargs:%v\n", l.Name(), l.sql, args)
		}
		sql, err := replaceTokens(l.sql, &l, tile, false)
		if err := ctxErr(ctx, err); err != nil {
			return nil, err
		}

		// replace configured query parameters if any
		sql = params.ReplaceParamsWithDialect(sql, &args, provider.SQLDialectPostgreSQL)

		// ref: https://postgis.net/docs/ST_AsMVT.html
		// bytea ST_AsMVT(any_element row, text name, integer extent, text geom_name, text feature_id_name)
		// name/geom/feature-id arguments are single-quoted SQL literals with
		// embedded quotes escaped (audit P5-8)
		sqls = append(sqls, buildMVTLayerSQL(layers[i].MVTName, l.GeomFieldName(), l.IDFieldName(), sql))
	}

	subsqls := strings.Join(sqls, "||")

	fsql := fmt.Sprintf(`SELECT (%s) AS data`, subsqls)

	var data []byte

	if debugExecuteSQL {
		log.Debugf("%s:%s: %v", EnvSQLDebugName, EnvSQLDebugExecute, fsql)
	}
	{
		now := time.Now()
		err = p.pool.QueryRow(ctx, fsql, args...).Scan(&data)
		if p.mvtProviderQueryHistogramSeconds != nil {
			z, _, _ := tile.ZXY()
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
			log.Debugf("%s:%s: returned %v bytes", EnvSQLDebugName, EnvSQLDebugExecute, len(data))
		}
	}

	// data may have garbage in it.
	if err := ctxErr(ctx, err); err != nil {
		return []byte{}, err
	}

	return data, nil
}

// Close will close the Provider's database connectio
func (p *Provider) Close() { p.pool.Close() }

// setLayerGeomType sets the geomType field on the layer to one of point,
// linestring, polygon, multipoint, multilinestring, multipolygon or
// geometrycollection
func (p *Provider) setLayerGeomType(l *Layer, geomType string) error {
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

// probeMOSCustomSQLContract takes prepared, unwrapped SQL and runs the
// common InspectSQLGeometryContract probe over REAL scanned row values:
// bounds columns presence plus at least MinValidMOSRows decodable MOS
// geometries with coordinates. The probe SQL is prepared by the shared
// probe helpers: raw bounds predicates are neutralized, native
// spatial operands use a source-CRS world envelope, and the sample reads at most
// codec.InspectionSampleLimit rows. SystemInfo rows are skipped, never
// applied: SQL-sample detection carries no projection contract. The actual
// result-column names are returned so the caller can persist them (A09).
func (p *Provider) probeMOSCustomSQLContract(l *Layer, probeSQL string) ([]string, codec.SQLGeometryContract, error) {
	if probeSQL == "" {
		return nil, codec.SQLGeometryContract{}, fmt.Errorf("missing probing SQL")
	}
	if l.geometryFormat == codec.FormatMOS {
		probeSQL = codec.PostgreSQL.MetadataProbeSQL(probeSQL)
	} else {
		probeSQL = codec.WrapProbeSQL(probeSQL)
	}
	// catch-all: drop any remaining code-context !TOKEN! the shared
	// preparation could not neutralize so a leftover placeholder cannot
	// break the probe statement (tokens in string literals or comments
	// are left verbatim)
	probeSQL = sqltoken.PostgreSQL.StripTokens(probeSQL)

	// 7.2.3: the probe statement carries no query arguments; no args —
	// never pass nil here: pgx variadic treats a lone nil as one query
	// argument ("expected 0 arguments, got 1")
	ctx, cancel := codec.NewInspectionContext()
	defer cancel()

	rows, err := p.pool.Query(ctx, probeSQL)
	if err != nil {
		return nil, codec.SQLGeometryContract{}, err
	}
	defer rows.Close()
	return probeSQLContractRows(l, rows)
}

// probeSQLContractRows runs the common InspectSQLGeometryContract probe over
// an already-executed result set of REAL row values (rows.Values). Split
// from probeMOSCustomSQLContract so the row contract can be regression
// tested with in-memory rows (audit A01).
func probeSQLContractRows(l *Layer, rows pgx.Rows) ([]string, codec.SQLGeometryContract, error) {
	fdescs := rows.FieldDescriptions()
	columns := make([]string, len(fdescs))
	for i, fd := range fdescs {
		columns[i] = string(fd.Name)
	}

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
			vals, err := rows.Values()
			if err != nil {
				return nil, false, err
			}
			return vals, true, nil
		},
		columns,
		l.geomField,
		l.bboxFields,
		decode,
	)
	if cerr != nil {
		return columns, codec.SQLGeometryContract{}, cerr
	}
	return columns, contract, nil
}

// inspectLayerGeomType sets the geomType field on the layer by running the SQL
// and reading the geom type in the result set
func (p *Provider) inspectLayerGeomType(pname string, l *Layer, maps []provider.Map) error {
	// Raw geometry formats (wkb/wkt/mos) carry non-PostGIS values in the
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

	// we want to know the geom type instead of returning the geom data so we modify the SQL
	// TODO (arolek): this strategy wont work if remove the requirement of wrapping ST_AsBinary(geom) in the SQL statements.
	//
	// https://github.com/alexeydott/tegola/issues/180
	//
	// case insensitive search

	probeSQL, args, err := geomTypeProbeSQL(l, extractQueryParamValues(pname, maps, l))
	if err != nil {
		return err
	}

	ctx, cancel := codec.NewInspectionContext()
	defer cancel()

	rows, err := p.pool.Query(ctx, probeSQL, args...)
	if err != nil {
		return err
	}
	defer rows.Close()

	return inspectGeomTypeRows(l, probeSQL, rows)
}

// geomTypeProbeSQL builds the shared bounds-contract probe query for native
// geometry-type inspection (audit R1). The ST_AsBinary → ST_GeometryType
// rewrite keeps the historical sniffing strategy (go-spatial/tegola#180);
// token neutralization then follows the documented codec.PrepareProbeSQL
// order: native !BBOX!/!BOX! use typed source-CRS world envelopes;
// zoom/position placeholders expand permissively (all occurrences, not just
// the first) — before custom parameters are substituted with live $N
// arguments. The query is wrapped in the shared InspectionSampleLimit
// sample window (docs/provider-contract.md).
func geomTypeProbeSQL(l *Layer, params provider.Params) (string, []any, error) {
	re := regexp.MustCompile(`(?i)ST_AsBinary`)
	sql := re.ReplaceAllString(l.sql, "ST_GeometryType")

	re = regexp.MustCompile(`(?i)(ST_AsMVTGeom\(.*\))`)
	if re.MatchString(sql) {
		sql = fmt.Sprintf("SELECT ST_GeometryType(%v) FROM (%v) as q", l.geomField, sql)
	}

	probeGeomType := ""
	if l.geomType != nil {
		probeGeomType = codec.GeomTypeName(l.geomType)
	}
	sql, err := prepareLayerProbeSQL(sql, l, probeGeomType)
	if err != nil {
		return "", nil, err
	}

	// substitute default values for custom parameters; the generated $N
	// placeholders are passed to the probe query as live arguments
	args := make([]any, 0)
	sql = params.ReplaceParamsWithDialect(sql, &args, provider.SQLDialectPostgreSQL)

	// remove all remaining code-context parameter tokens for inspection
	// (tokens in string literals or comments are left verbatim)
	// crossing our fingers that the query is still valid 🤞
	// if not, the user will have to specify `geometry_type` in the config
	sql = sqltoken.PostgreSQL.StripTokens(sql)

	return codec.WrapProbeSQL(sql), args, nil
}

// inspectGeomTypeRows sniffs the geometry type from sampled probe rows,
// matching either the geometry column (ST_GeometryType-rewritten values) or
// the st_geometrytype result column produced by the AsMVTGeom wrap.
func inspectGeomTypeRows(l *Layer, probeSQL string, rows pgx.Rows) error {
	// fetch rows FieldDescriptions. this gives us the OID for the data types returned to aid in decoding
	fdescs := rows.FieldDescriptions()
	for rows.Next() {
		vals, err := rows.Values()
		if err != nil {
			return fmt.Errorf("error running SQL: %v ; %w", probeSQL, err)
		}

		// iterate the values returned from our row, sniffing for the geomField or st_geometrytype field name
		for i, v := range vals {
			switch string(fdescs[i].Name) {
			case l.geomField, "st_geometrytype":
				switch v {
				case nil:
					// SQL NULL carries no type evidence. Keep sampling so a
					// nullable geometry column can be inferred from later rows.
					continue
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
					return fmt.Errorf(
						"layer (%v) returned unsupported geometry type (%v)",
						l.name,
						v,
					)
				}
			}
		}
	}

	return rows.Err()
}

// CreateProvider instantiates and returns a new PostGIS provider or an error.
//
// Connection configuration is resolved using either a PostgreSQL connection
// URI or environment variables, with environment variables taking precedence.
//
// The provider requires:
//
//   - name (string): [Required] unique name of the provider.
//   - uri (string): [Required unless environment mode is used] full PostgreSQL
//     connection URI (postgres:// or postgresql://).
//
// Connection resolution rules:
//
//  1. If one or more PostgreSQL environment variables relevant to connection
//     configuration (e.g. PGHOST, PGPORT, PGDATABASE, PGUSER, PGPASSWORD,
//     PGSSLMODE, etc.) are present, the provider operates in environment mode.
//     In this mode, connection parameters are derived from the environment
//     and the configured URI (if provided) is ignored.
//
//  2. If no environment trigger variables are present, the configured URI
//     is used.
//
// In environment mode, strict validation is applied to the resolved
// configuration to ensure host, port, database, and user are set. If the
// configuration is incomplete, provider initialization fails with a
// descriptive error.
//
// When multiple PostGIS providers are defined within a single Tegola
// configuration, using explicit URIs is strongly recommended. Environment
// variables apply process-wide and will override all PostGIS providers
// uniformly, potentially leading to unintended shared connections.
//
// Additional provider configuration:
//
//   - srid (int): [Optional] default SRID for the provider. Defaults to
//     WebMercator (3857). Any numeric SRID is supported; use crs_defn for a
//     full PROJ.4 definition. See docs/crs.md.
//
//   - max_connections (int): [Optional] maximum number of connections in the
//     pool. Default is 100. A value of 0 disables the limit.
//
//   - layers (map[string]struct{}): map of layers keyed by layer name. Each
//     layer supports:
//
//   - name (string): [Required] name of the layer.
//
//   - tablename (string): [Required if sql is not defined] database table
//     to query. tablename and sql are mutually exclusive.
//
//   - geometry_fieldname (string): [Optional] geometry column name.
//     Defaults to "geom".
//
//   - id_fieldname (string): [Optional] feature ID column. Defaults to
//     empty (no id column selected; MVT feature IDs are left unset).
//
//   - fields ([]string): [Optional] additional fields to include if sql
//     is not defined.
//
//   - srid (int): [Optional] layer SRID; supports any numeric SRID
//     available in spatial_ref_sys.
//
//   - sql (string): [Required if tablename is not defined] custom SQL
//     statement. The SQL must include required tokens where applicable.
func CreateProvider(
	config dict.Dicter,
	maps []provider.Map,
	providerType string,
) (*Provider, error) {
	c := newDefaultConnector(config)
	pool, pgxCfg, _, err := c.Connect(context.Background())
	if err != nil {
		return nil, err
	}
	// Built-in projected CRS definitions must be registered before layer SQL
	// generation converts WebMercator extents into source-CRS bounds.
	basic.RegisterBuiltinProj4SRIDs()

	// provider-level srid/crs_defn via the shared CRS contract.
	pcrs, perr := crsconfig.ResolveProvider(config, DefaultSRID)
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
	if isMVT(providerType) {
		if verr := codec.ValidateMVTGeometryFormat(providerGeometryFormat); verr != nil {
			return nil, verr
		}
	}

	name, err := config.String(ConfigKeyName, nil)
	if err != nil {
		return nil, err
	}

	p := Provider{
		srid:   uint64(srid),
		config: *pgxCfg,
		name:   name,
	}

	p.pool = &connectionPoolCollector{Pool: pool, providerName: name}

	layers, err := config.MapSlice(ConfigKeyLayers)
	if err != nil {
		return nil, err
	}

	lyrs := make(map[string]Layer)
	lyrsSeen := make(map[string]int)

	for i, layer := range layers {
		lName, err := layer.String(ConfigKeyLayerName, nil)
		if err != nil {
			return nil, fmt.Errorf(
				"for layer (%v) we got the following error trying to get the layer's name field: %w",
				i,
				err,
			)
		}

		if j, ok := lyrsSeen[lName]; ok {
			return nil, fmt.Errorf(
				"%v layer name is duplicated in both layer %v and layer %v",
				lName,
				i,
				j,
			)
		}

		lyrsSeen[lName] = i

		if i == 0 {
			p.firstLayer = lName
		}

		fields, err := layer.StringSlice(ConfigKeyFields)
		if err != nil {
			return nil, fmt.Errorf(
				"for layer (%v) %v %v field had the following error: %w",
				i,
				lName,
				ConfigKeyFields,
				err,
			)
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
			return nil, fmt.Errorf(
				"for layer (%v) %v: %v (%v) and %v field (%v) is the same",
				i,
				lName,
				ConfigKeyGeomField,
				geomfld,
				ConfigKeyGeomIDField,
				idfld,
			)
		}

		geomType := ""
		geomType, err = layer.String(ConfigKeyGeomType, &geomType)
		if err != nil {
			return nil, fmt.Errorf("for layer (%v) %v : %w", i, lName, err)
		}

		// tablename and sql are mutually exclusive. Presence is checked
		// explicitly (like the gpkg/mysql providers) so an explicit tablename
		// equal to the layer name is still detected.
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
				return nil, fmt.Errorf(
					"for %v layer (%v) %v has an error: %w",
					i,
					lName,
					ConfigKeyTablename,
					err,
				)
			}
		}

		var sql string
		if sqlPresent {
			sql, err = layer.String(ConfigKeySQL, &sql)
			if err != nil {
				return nil, fmt.Errorf(
					"for %v layer (%v) %v has an error: %w",
					i,
					lName,
					ConfigKeySQL,
					err,
				)
			}
		}

		// layer-level srid/crs_defn via the shared CRS contract; a layer
		// crs_defn wins over any numeric srid on the same level.
		lcrs, err := crsconfig.ResolveLayer(layer, srid)
		if err != nil {
			return nil, fmt.Errorf("for layer (%v) %v: %w", i, lName, err)
		}
		lsrid := lcrs.SRID

		l := Layer{
			tablename: tblName,
			name:      lName,
			idField:   idfld,
			geomField: geomfld,
			srid:      uint64(lsrid),
			// explicit CRS at either config level suppresses any
			// source-provided projection (MOS system-info blob)
			crsExplicit:    pcrs.Explicit || lcrs.Explicit,
			geometryFormat: providerGeometryFormat,
			mosConfig:      providerMOSCfg,
		}

		// layer-level geometry format / MOS overrides merge on top of the
		// provider-level values.
		layerMOSCfg, lerr := codec.ResolveMOSConfig(nil, layer, lName)
		if lerr != nil {
			return nil, lerr
		}
		layerGeometryFormat, lerr := codec.ResolveLayerGeometryFormat(providerGeometryFormat, layer, lName, postgisGeometryFormats)
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
		// MVT providers must not take the raw geometry path: their geometry
		// is MVT bytes produced by the database, not a raw feature geometry.
		if isMVT(providerType) {
			if verr := codec.ValidateMVTGeometryFormat(l.geometryFormat); verr != nil {
				return nil, fmt.Errorf("for layer (%v) %v: %w", i, lName, verr)
			}
		}

		if sql != "" && !isSelectQuery.MatchString(sql) {
			// if it is not a SELECT query, then we assume we have a sub-query
			// (`(select ...) as foo`) which we can handle like a tablename
			tblName = sql
			sql = ""
		}

		// PostGIS metadata source-SRID auto-detect (docs/crs.md): for native
		// table layers with no explicit provider/layer CRS, the SRID
		// recorded in the spatial metadata wins over the documented 3857
		// default. Custom SQL and raw formats cannot be introspected this
		// way and keep the documented default.
		if tblPresent && !sqlPresent && !pcrs.Explicit && !lcrs.Explicit && !isMVT(providerType) && !codec.IsRawFormat(l.geometryFormat) {
			schema, table, serr := splitTableName(tblName)
			if serr != nil {
				return nil, fmt.Errorf("for layer (%v) %v: invalid table name %q: %w", i, lName, tblName, serr)
			}
			dctx, dcancel := codec.NewInspectionContext()
			detected, derr := inferTableSRID(dctx, p.pool, schema, table, geomfld)
			dcancel()
			if derr != nil {
				return nil, fmt.Errorf(
					"for layer (%v) %v: unable to auto-detect source SRID from PostGIS metadata; set srid or crs_defn explicitly: %w",
					i, lName, derr,
				)
			}
			l.srid = detected
		}

		// A02: id_fieldname explicit tracking. A detected MapplGIS table
		// overrides the default ID field with the contract primary key
		// (OKEY); an explicit value is honored.
		idFieldExplicit := idfld != ""
		var tblSchema, tblTable string
		if tblPresent && !sqlPresent {
			var serr error
			tblSchema, tblTable, serr = splitTableName(tblName)
			if serr != nil {
				return nil, fmt.Errorf("for layer (%v) %v: invalid table name %q: %w", i, lName, tblName, serr)
			}
		}

		// A06: canonical MapplGIS detection runs for every tablename layer
		// before the sql/tbl branch: a MapplGIS table is not a PostGIS
		// spatial table, so a layer pointing at one must be discovered here
		// and served via the MOS path. Custom SQL is never auto-detected.
		if tblSchema != "" {
			mctx, mcancel := codec.NewInspectionContext()
			isMappl, derr := detectMapplGIS(mctx, p.pool, &l, tblSchema, tblTable)
			mcancel()
			if derr != nil {
				return nil, fmt.Errorf("for layer (%v) %v: %v", i, lName, derr)
			}
			if isMappl {
				if !idFieldExplicit {
					idfld = mapplgis.PrimaryKey
					l.idField = idfld
				}
				log.Debugf("layer (%v): table %v.%v detected as MapplGIS", lName, tblSchema, tblTable)
			}
		}

		if sql != "" {
			// convert !BOX! (MapServer) and !bbox! (Mapnik) to !BBOX! for
			// compatibility; only code-context occurrences are rewritten
			sql := sqltoken.PostgreSQL.MapTokens(sql, func(token string) string {
				if token == "!BOX!" || token == "!bbox!" {
					return conf.BboxToken
				}
				return token
			})
			// make sure that the sql has a !BBOX! token (code context)
			if !sqltoken.PostgreSQL.ContainsToken(sql, conf.BboxToken) {
				return nil, fmt.Errorf(
					"SQL for layer (%v) %v is missing required token: %v",
					i,
					lName,
					conf.BboxToken,
				)
			}
			if !strings.Contains(sql, "*") {
				if !strings.Contains(sql, geomfld) {
					return nil, fmt.Errorf(
						"SQL for layer (%v) %v does not contain the geometry field: %v",
						i,
						lName,
						geomfld,
					)
				}
				if !strings.Contains(sql, idfld) {
					return nil, fmt.Errorf(
						"SQL for layer (%v) %v does not contain the id field for the geometry: %v",
						i,
						lName,
						sql,
					)
				}
			}

			// check all tokens are valid (code-context occurrences only)
			for _, token := range sqltoken.PostgreSQL.CodeTokens(sql) {
				if _, ok := conf.ReservedTokens[token.Text]; !ok {
					return nil, fmt.Errorf(
						"SQL for layer (%v) %v references an unknown token %s: %v",
						i,
						lName,
						token.Text,
						sql,
					)
				}
			}

			// Raw custom-SQL contract: native PostGIS geometry still uses
			// the native-spatial !BBOX! envelope; only MOS raw format is
			// allowed (and required) to carry it, with the bounds-backed
			// predicate builder replacing the envelope at query time.
			if verr := codec.PostgreSQL.ValidateRawCustomSQL(lName, l.geometryFormat, sql, conf.BboxToken, "!BOX!"); verr != nil {
				return nil, fmt.Errorf("for layer (%v) %v: %w", i, lName, verr)
			}
			if rerr := codec.PostgreSQL.RequireBBoxCustomSQL(lName, l.geometryFormat == codec.FormatMOS, sql, conf.BboxToken, "!BOX!"); rerr != nil {
				return nil, fmt.Errorf("for layer (%v) %v: %w", i, lName, rerr)
			}

			// Common storage-format probe (decisions 3+6): the probe
			// uses boolean predicates for raw bounds and a world envelope
			// for native spatial operands, then inspects
			// REAL scanned row values for the bounds columns plus
			// decodable geometries. It runs for every format that can
			// carry bounds columns or raw geometry and doubles as the
			// registration-time structural check for tile-dependent SQL
			// (A08): result-column checks never depend on the sample rows.
			boundsBacked := l.geometryFormat == codec.FormatMOS
			if l.geometryFormat != codec.FormatWKB && l.geometryFormat != codec.FormatWKT {
				inspectionSQL, err := prepareLayerProbeSQL(sql, &l, geomType)
				if err != nil {
					return nil, err
				}
				columns, contract, perr := p.probeMOSCustomSQLContract(&l, inspectionSQL)
				// Empty geometry_format also permits positive MOS detection.
				// Such SQL uses a boolean bounds token, not a native geometry
				// operand. Retry the raw contract only after the typed probe
				// fails; format selection still requires decoded MOS evidence.
				if perr != nil && l.geometryFormat == "" {
					rawSQL := codec.PostgreSQL.PrepareProbeSQL(sql, geomfld, idfld, geomType)
					if rawSQL != inspectionSQL {
						columns, contract, perr = p.probeMOSCustomSQLContract(&l, rawSQL)
					}
				}

				switch {
				case perr != nil && boundsBacked:
					return nil, fmt.Errorf("layer '%v' problem probing bounds-backed custom SQL: %v", lName, perr)
				case perr != nil:
					log.Warnf("layer '%v': custom SQL storage-format probe failed (%v); format not detected", lName, perr)
				default:
					// decision 3: MOS evidence needs a positive MOS
					// signature AND >=3 decodable MOS sample rows; native
					// rows never count (7.1.3)
					mosEvidence := contract.DetectsMOS()
					if boundsBacked || mosEvidence {
						// A02: the structural contract is a startup error,
						// validated AFTER format resolution so an
						// auto-detected MOS layer cannot skip it (A05)
						resolved, boundsInResult, verr := codec.PostgreSQL.ResolveBoundsSQLContract(lName, sql, l.geomField, contract, l.bboxFields)
						if verr != nil {
							return nil, fmt.Errorf("layer (%v) %v: %w", i, lName, verr)
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
					}
					if mosEvidence {
						// decision 4: tagging happens only on a successful
						// MOS sample; SystemInfo rows were never applied
						l.isMapplGIS = true
						l.mapplSource = codec.MapplGISSQLSample
						log.Infof("layer '%v': bounds-backed MOS custom SQL contract detected (source %v, %v valid MOS sample rows)", lName, l.mapplSource, contract.ValidMOSRows)
						if l.geometryFormat == "" {
							l.geometryFormat = codec.FormatMOS
							log.Infof("layer '%v': switched geometry_format to %v (sql-sample detected)", lName, codec.FormatMOS)
						}
					}
					if l.geometryFormat == codec.FormatMOS {
						// decision 5: MOS columns carry no CRS metadata;
						// explicit srid/crs_defn is required
						if verr := codec.ValidateMOSSQLExplicitConfig(lName, l.crsExplicit); verr != nil {
							return nil, fmt.Errorf("layer (%v) %v: %w", i, lName, verr)
						}
					} else if !boundsBacked && !mosEvidence {
						// documented empty/auto policy: no usable sample ->
						// format not detected; register with the configured
						// format (7.2.1: report the actual sample columns)
						log.Warnf("layer '%v': custom SQL storage format not detected (sample columns: %v; %v decodable MOS sample rows); registering with format %q", lName, strings.Join(columns, ", "), contract.ValidMOSRows, l.geometryFormat)
					}
				}
			}

			l.sql = sql
			l.bboxTable, err = codec.ResolveBBoxTable(layer, lName)
			if err != nil {
				return nil, err
			}
		} else {
			// Tablename and Fields will be used to build the query.
			// We need to do some work. We need to check to see Fields contains the geom and gid fields
			// and if not add them to the list. If Fields list is empty/nil we will use '*' for the field list.
			l.sql, err = genSQL(&l, p.pool, tblName, append([]string(nil), fields...), true, providerType)
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
				return nil, fmt.Errorf(
					"error fetching geometry type for layer (%v): %w",
					l.name,
					err,
				)
			}
		} else {
			pname, err := config.String(ConfigKeyName, nil)
			if err != nil {
				return nil, err
			}

			if err = p.inspectLayerGeomType(pname, &l, maps); err != nil {
				return nil, fmt.Errorf("error fetching geometry type for layer (%v): %w\nif custom parameters are used, remember to set %s for the provider", l.name, err, ConfigKeyGeomType)
			}
		}

		if err := p.registerFeatureProfile(layer, &l, tblName, tblPresent && !sqlPresent, isMVT(providerType)); err != nil {
			return nil, fmt.Errorf("layer %s feature metadata: %w", lName, err)
		}
		lyrs[lName] = l
	}
	p.layers = lyrs

	// audit P6-19: raw geometry formats without a bounds-backed filter are
	// fully scanned on every tile request; warn once per affected layer.
	for _, msg := range rawGeometryBoundsWarnings(p.layers) {
		log.Warn(msg)
	}

	// track the provider so we can clean it up later
	providers = append(providers, &p)

	return &p, nil
}

// Cleanup will close all database connections and destroy all previously instantiated Provider instances
func Cleanup() {
	if len(providers) > 0 {
		log.Infof("cleaning up postgis providers")
	}

	for i := range providers {
		providers[i].Close()
	}

	providers = nil
}
