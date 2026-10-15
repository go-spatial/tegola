package mysql

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/binary"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/alexeydott/geom"
	"github.com/alexeydott/geom/encoding/wkb"
	"github.com/alexeydott/geom/encoding/wkt"
	"github.com/alexeydott/tegola"
	"github.com/alexeydott/tegola/basic"
	"github.com/alexeydott/tegola/internal/log"
	"github.com/alexeydott/tegola/mos"
	"github.com/alexeydott/tegola/provider"
	codec "github.com/alexeydott/tegola/provider/geometrycodec"
	mysqlDriver "github.com/go-sql-driver/mysql"
)

const (
	Name                 = "mysql"
	DefaultSRID          = tegola.WebMercator
	DefaultPort          = 3306
	DefaultIDFieldName   = "fid"
	DefaultGeomFieldName = "geom"
)

const (
	mysqlQueryMaxAttempts = 3
	mysqlRetryBaseDelay   = 100 * time.Millisecond
	mysqlConnMaxIdleTime  = 5 * time.Minute
	mysqlConnMaxLifetime  = 30 * time.Minute
)

// geometry column encoding formats. "auto" detects the server flavor
// (MySQL vs MariaDB) at provider startup via SELECT VERSION() and uses the
// matching native layout. "wkb" expects plain WKB (e.g. selected via
// ST_AsBinary(geom)) with no internal header. "wkt" expects WKT text
// (e.g. CHAR/VARCHAR/TEXT columns holding LINESTRING(...), or a
// ST_AsText(...) expression selected in custom SQL).
const (
	GeometryFormatAuto    = "auto"
	GeometryFormatMySQL   = "mysql"
	GeometryFormatMariaDB = "mariadb"
	GeometryFormatWKB     = "wkb"
	GeometryFormatWKT     = "wkt"
	GeometryFormatMOS     = "mos"
)

// mysqlGeometryFormats is the set of geometry_format values accepted at both
// the provider and the layer level.
var mysqlGeometryFormats = map[string]struct{}{
	GeometryFormatAuto:    {},
	GeometryFormatMySQL:   {},
	GeometryFormatMariaDB: {},
	GeometryFormatWKB:     {},
	GeometryFormatWKT:     {},
	GeometryFormatMOS:     {},
}

// config keys
const (
	ConfigKeyHost                     = "host"
	ConfigKeyPort                     = "port"
	ConfigKeyDatabase                 = "database"
	ConfigKeyUser                     = "user"
	ConfigKeyPassword                 = "password"
	ConfigKeyTLS                      = "tls"
	ConfigKeyTimeout                  = "timeout"
	ConfigKeySRID                     = "srid"
	ConfigKeyCRSDefn                  = "crs_defn"
	ConfigKeyMaxConn                  = "max_connections"
	ConfigKeyGeometryFormat           = "geometry_format"
	ConfigKeyMOSPrecision             = "mos_precision"
	ConfigKeyMOSUnits                 = "mos_units"
	ConfigKeyLayers                   = "layers"
	ConfigKeyLayerName                = "name"
	ConfigKeyTableName                = "tablename"
	ConfigKeySQL                      = "sql"
	ConfigKeyGeomIDField              = "id_fieldname"
	ConfigKeyGeomField                = "geometry_fieldname"
	ConfigKeyFields                   = "fields"
	ConfigKeyAllowLegacyTableIdentity = "allow_legacy_table_identity"
)

// MariaDB 10.7+ stores axis-order flags for reference geometries in the
// three top bits of the SRID field. Mask them off to recover the actual SRID.
const mariaDBSRIDMask = 0x1FFFFFFF

// ErrUnknownLayer denotes a layer name that is not registered on the provider
type ErrUnknownLayer struct {
	Name string
}

func (e ErrUnknownLayer) Error() string {
	return fmt.Sprintf("layer not registered with provider: %v", e.Name)
}

// decodeMySQLFormat parses the MySQL native internal geometry layout:
// [4 bytes SRID (little-endian)][1 byte byte-order][WKB (type + body)]
func decodeMySQLFormat(b []byte) (srid uint64, g geom.Geometry, err error) {
	if len(b) < 9 {
		return 0, nil, fmt.Errorf("geometry blob too short (%v bytes) for MySQL native format", len(b))
	}
	bom := b[4]
	if bom != 0 && bom != 1 {
		return 0, nil, fmt.Errorf("invalid byte-order marker (%v) for MySQL native format", bom)
	}
	srid = uint64(binary.LittleEndian.Uint32(b[0:4]))
	g, err = wkb.DecodeBytes(b[5:])
	if err != nil {
		return 0, nil, fmt.Errorf("error decoding WKB body of MySQL native geometry: %v", err)
	}
	return srid, g, nil
}

// decodeMariaDBFormat parses the MariaDB native internal geometry layout:
// [1 byte byte-order][4 bytes SRID (stored in that byte order)][WKB (type + body)]
// Note the SRID comes *after* the byte-order marker, unlike MySQL where it
// precedes it. In MariaDB 10.7+ the top three SRID bits carry axis-order
// flags for reference geometries and are masked off here.
func decodeMariaDBFormat(b []byte) (srid uint64, g geom.Geometry, err error) {
	if len(b) < 9 {
		return 0, nil, fmt.Errorf("geometry blob too short (%v bytes) for MariaDB native format", len(b))
	}
	bom := b[0]
	if bom != 0 && bom != 1 {
		return 0, nil, fmt.Errorf("invalid byte-order marker (%v) for MariaDB native format", bom)
	}
	var bo binary.ByteOrder = binary.LittleEndian
	if bom == 0 {
		bo = binary.BigEndian
	}
	srid = uint64(bo.Uint32(b[1:5]) & mariaDBSRIDMask)
	g, err = wkb.DecodeBytes(b[5:])
	if err != nil {
		return 0, nil, fmt.Errorf("error decoding WKB body of MariaDB native geometry: %v", err)
	}
	return srid, g, nil
}

// decodeGeometry decodes a geometry value read from the database according
// to the configured format: "mysql", "mariadb", "wkb", "wkt", "mos" or
// "auto". With "auto" the server flavor detected at startup (serverFlavor)
// selects the native layout. Plain WKB is accepted as a last resort, which
// covers values already converted with ST_AsBinary() in custom SQL.
// The MOS quantization settings come from the resolved provider/layer
// mosConfig (layer-level mos_precision/mos_units).
func decodeGeometry(v interface{}, format string, serverFlavor string, mosCfg codec.MOSConfig) (srid uint64, g geom.Geometry, err error) {
	switch format {
	case GeometryFormatWKT:
		g, err = codec.DecodeWKT(v)
		return 0, g, err
	case GeometryFormatMOS:
		g, err = codec.DecodeMOS(v, mosCfg)
		return 0, g, err
	}

	// all remaining formats operate on binary blobs
	b, ok := v.([]byte)
	if !ok {
		// a string may still arrive for binary formats depending on driver
		// column typing; convert before rejecting.
		if s, isStr := v.(string); isStr {
			b = []byte(s)
		} else {
			return 0, nil, fmt.Errorf("unexpected geometry column type %T, expected blob", v)
		}
	}

	switch format {
	case GeometryFormatMySQL:
		return decodeMySQLFormat(b)
	case GeometryFormatMariaDB:
		return decodeMariaDBFormat(b)
	case GeometryFormatWKB:
		g, err = wkb.DecodeBytes(b)
		return 0, g, err
	case GeometryFormatAuto, "":
		if serverFlavor == GeometryFormatMariaDB {
			if srid, g, err = decodeMariaDBFormat(b); err == nil {
				return srid, g, nil
			}
			// fall through to plain WKB as a last resort
			if g, wkbErr := wkb.DecodeBytes(b); wkbErr == nil {
				return 0, g, nil
			}
			// last resort: the blob may actually be WKT text (e.g. a
			// CHAR/TEXT geometry column stored without a native type)
			if g, wktErr := wkt.DecodeBytes(b); wktErr == nil {
				return 0, g, nil
			}
			return 0, nil, err
		}
		if srid, g, err = decodeMySQLFormat(b); err == nil {
			return srid, g, nil
		}
		if g, wkbErr := wkb.DecodeBytes(b); wkbErr == nil {
			return 0, g, nil
		}
		if g, wktErr := wkt.DecodeBytes(b); wktErr == nil {
			return 0, g, nil
		}
		return 0, nil, err
	default:
		return 0, nil, fmt.Errorf("unknown geometry_format: %v", format)
	}
}

func isRetryableConnectionError(err error) bool {
	return errors.Is(err, driver.ErrBadConn) || errors.Is(err, mysqlDriver.ErrInvalidConn)
}

func waitForMySQLRetry(ctx context.Context, attempt int) error {
	delay := mysqlRetryBaseDelay * time.Duration(1<<attempt)
	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func preferContextError(ctx context.Context, err error) error {
	if ctxErr := ctx.Err(); ctxErr != nil {
		return ctxErr
	}
	return err
}

type Provider struct {
	// database host
	Host string
	// database port
	Port int
	// database name
	Database string
	// map of layer name and corresponding layer
	layers map[string]Layer
	// reference to the database connection
	db *sql.DB
	// default SRID for the provider
	srid uint64
	// geometry column encoding: auto, mysql, mariadb or wkb
	geometryFormat string
	// detected server flavor ("mysql" or "mariadb"), used by the auto format
	serverFlavor string
	// MySQL 5.5 has no native InnoDB table-ID catalog. This explicit opt-in
	// permits metadata drift checks, not proof against external recreation.
	allowLegacyTableIdentity bool
	legacyIdentityWarning    sync.Once
	// writerMu guards cachedWriter.
	writerMu     sync.Mutex
	cachedWriter *Writer
}

func (p *Provider) Layers() ([]provider.LayerInfo, error) {
	log.Debug("attempting mysql.Layers()")

	ls := make([]provider.LayerInfo, len(p.layers))

	var i int
	for _, player := range p.layers {
		ls[i] = player
		i++
	}

	log.Debugf("returning LayerInfo array: %v", ls)

	return ls, nil
}

func (p *Provider) TileFeatures(ctx context.Context, layer string, tile provider.Tile, queryParams provider.Params, fn func(f *provider.Feature) error) error {
	var err error
	for attempt := 0; attempt < mysqlQueryMaxAttempts; attempt++ {
		err = p.tileFeaturesAttempt(
			ctx,
			layer,
			tile,
			queryParams,
			fn,
			attempt+1,
		)
		if err == nil || !isRetryableConnectionError(err) || attempt == mysqlQueryMaxAttempts-1 {
			return err
		}

		log.Warnf("mysql provider: retrying tile query after broken connection (attempt %d/%d): %v",
			attempt+1, mysqlQueryMaxAttempts, err)
		if err := waitForMySQLRetry(ctx, attempt); err != nil {
			return err
		}
	}
	return err
}

func (p *Provider) tileFeaturesAttempt(
	ctx context.Context,
	layer string,
	tile provider.Tile,
	queryParams provider.Params,
	fn func(f *provider.Feature) error,
	attempt int,
) (resultErr error) {
	diagnostics := tileQueryDiagnostics{started: time.Now(), phase: "prepare"}
	diagnostics.phaseStarted = diagnostics.started
	completed := false
	defer func() {
		if !completed {
			diagnostics.finish(p.db, layer, tile, attempt, errors.New("tile query interrupted"))
			return
		}
		diagnostics.finish(p.db, layer, tile, attempt, resultErr)
	}()
	resultErr = p.tileFeaturesAttemptQuery(ctx, layer, tile, queryParams, fn, &diagnostics)
	completed = true
	return resultErr
}

func (p *Provider) tileFeaturesAttemptQuery(
	ctx context.Context,
	layer string,
	tile provider.Tile,
	queryParams provider.Params,
	fn func(f *provider.Feature) error,
	diagnostics *tileQueryDiagnostics,
) error {
	log.Debugf("fetching layer %v", layer)

	pLayer, ok := p.layers[layer]
	if !ok {
		return ErrUnknownLayer{layer}
	}

	// read the tile extent
	tileBBox, tileSRID := tile.BufferedExtent()
	webMercatorBBox := tileBBox

	// tileSRID is assumed to always be WebMercator. Build a conservative
	// source-CRS extent before applying the provider's spatial filter.
	if pLayer.srid != tileSRID {
		sourceBBox, err := basic.FromWebMercatorExtent(pLayer.srid, tileBBox)
		if err != nil {
			return fmt.Errorf("error converting tile extent: %v ", err)
		}
		tileBBox = sourceBBox
	}

	var qtext string
	args := make([]interface{}, 0)

	if pLayer.tablename != "" {
		// If layer was specified via "tablename" in config, construct query.
		selectClause := fmt.Sprintf("SELECT %v, %v", quoteIdentifier(pLayer.idFieldname), quoteIdentifier(pLayer.geomFieldname))

		for _, tf := range pLayer.tagFieldnames {
			selectClause += fmt.Sprintf(", %v", quoteIdentifier(tf))
		}

		qtext = fmt.Sprintf("%v FROM %v WHERE %v IS NOT NULL AND !BBOX!", selectClause, quoteIdentifier(pLayer.tablename), quoteIdentifier(pLayer.geomFieldname))

		expanded, terr := replaceTokens(qtext, &pLayer, tile, tileBBox)
		if terr != nil {
			return fmt.Errorf("layer (%v): %v", pLayer.name, terr)
		}
		qtext = expanded
	} else {
		// If layer was specified via "sql" in config, collect it
		expanded, terr := replaceTokens(pLayer.sql, &pLayer, tile, tileBBox)
		if terr != nil {
			return fmt.Errorf("layer (%v): %v", pLayer.name, terr)
		}
		qtext = expanded
		qtext = queryParams.ReplaceParamsWithDialect(qtext, &args, provider.SQLDialectMySQL)
	}

	log.Debugf("qtext: %v", qtext)

	diagnostics.before = p.db.Stats()
	diagnostics.queried = true
	diagnostics.enter("query")
	rows, err := p.db.QueryContext(ctx, qtext, args...)
	diagnostics.query = time.Since(diagnostics.phaseStarted)
	if err != nil {
		err = preferContextError(ctx, err)
		if !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
			log.Errorf("err during query: %v - %v", qtext, err)
		}
		return err
	}
	diagnostics.enter("rows_decode")
	defer func() { _ = rows.Close() }()

	cols, err := rows.Columns()
	if err != nil {
		return err
	}

	// declared column types let us distinguish numeric columns the driver
	// returns as []byte (textual form) from genuine text/blob columns
	colTypes, err := rows.ColumnTypes()
	if err != nil {
		return err
	}

	var geomErr error
	geometryFormat := pLayer.geometryFormat
	// mosCfg is a per-tile copy of the layer's resolved MOS config so a
	// runtime system-info blob can adjust it without mutating the layer.
	mosCfg := pLayer.mosConfig
	inMemoryTileFilter := pLayer.deferredInspection || basic.IsSyntheticSRID(pLayer.srid)
	features := make([]provider.Feature, 0)
	var deferredRows [][]interface{}

	processRow := func(vals []interface{}) error {
		feature := provider.Feature{
			Tags: map[string]interface{}{},
		}

		// set when the row's geometry is undecodable or outside the tile:
		// the row is then skipped entirely (no feature emitted) rather than
		// being emitted with a nil geometry and SRID 0, which would fail
		// downstream reprojection and kill the whole tile.
		skipRow := false

		for i := range cols {
			// check if the context cancelled or timed out
			if ctx.Err() != nil {
				return ctx.Err()
			}
			if vals[i] == nil {
				if cols[i] == pLayer.geomFieldname {
					skipRow = true
				}
				continue
			}

			switch cols[i] {
			case pLayer.idFieldname:
				feature.ID, err = provider.ConvertFeatureID(vals[i])
				if err != nil {
					return err
				}

			case pLayer.geomFieldname:
				// A MapplGIS layer system-info blob is metadata, not
				// geometry: skip it silently. MapplGIS detection is done
				// once at registration (provider/mapplgis contract); tile
				// requests never repeat or apply it at runtime.
				if blob, ok := vals[i].([]byte); ok && mos.IsSystemInfoBlob(blob) {
					skipRow = true
					break
				}
				srid, geo, err := decodeGeometry(vals[i], geometryFormat, p.serverFlavor, mosCfg)
				if err != nil {
					// a single undecodable row (e.g. a version-prefixed or
					// otherwise non-MOS blob) must not kill the whole tile;
					// log it and skip
					log.Warnf("mysql provider: skipping undecodable geometry in layer %v (id %v): %v", pLayer.Name(), feature.ID, err)
					geomErr = err
					skipRow = true
					break
				}

				// Match startup inspection for deferred custom SQL: when no
				// CRS was configured, a native geometry header is the only
				// available source-CRS declaration. The first non-zero
				// header SRID establishes the canonical layer CRS; rows
				// carrying a different SRID are skipped with a warning
				// instead of being silently mislabeled (audit P5-9).
				if pLayer.deferredInspection && !pLayer.crsExplicit && srid > 0 {
					switch pLayer.resolveDeferredHeaderSRID(srid) {
					case deferredSRIDAdopt:
						if pLayer.srid != tileSRID {
							sourceBBox, err := basic.FromWebMercatorExtent(pLayer.srid, webMercatorBBox)
							if err != nil {
								return fmt.Errorf("convert tile extent for geometry header SRID %d: %w", srid, err)
							}
							tileBBox = sourceBBox
						} else {
							tileBBox = webMercatorBBox
						}
					case deferredSRIDSkip:
						log.Warnf("mysql provider: layer %v: skipping row with geometry header SRID %d; the layer CRS was established as SRID %d from the first sampled row (mixed SRIDs are not supported in deferred custom SQL)", pLayer.Name(), srid, pLayer.srid)
						skipRow = true
					}
					if skipRow {
						break
					}
				}

				// Raw formats (mos/wkb/wkt) cannot push an exact spatial
				// filter into SQL (MOS blobs are opaque; WKB/WKT predicates
				// are best-effort optimizations). Drop rows whose decoded
				// geometry cannot intersect the tile's buffered extent
				// (already transformed into the layer's source SRID).
				if (codec.IsRawFormat(geometryFormat) || inMemoryTileFilter) && !codec.GeometryIntersectsExtent(geo, tileBBox) {
					skipRow = true
					break
				}

				// an explicitly configured layer/provider SRID wins over the value
				// decoded from the geometry header, which can be 0 or carry
				// MariaDB axis-order flags.
				if pLayer.srid != 0 {
					feature.SRID = pLayer.srid
				} else if srid > 0 {
					feature.SRID = srid
				} else if p.srid != 0 {
					feature.SRID = p.srid
				} else {
					feature.SRID = DefaultSRID
				}

				// mixed-content policy for explicit geometry_type: permit
				// the feature but warn once per layer.
				if pLayer.geomTypeExplicit {
					codec.WarnOnceGeometryTypeMismatch(pLayer.Name(), pLayer.geomType, geo)
				}
				feature.Geometry = geo

			default:
				// Bounds fields are operational columns backing the
				// bounds-backed MOS !BBOX! predicate, not user attributes:
				// never leak them into feature tags.
				if pLayer.bboxFields.IsBBoxField(cols[i]) {
					continue
				}
				// Grab any non-nil, non-id, non-geometry column as a tag,
				// converting numeric columns the driver returns as []byte
				switch v := vals[i].(type) {
				case []uint8:
					tagVal, cerr := tagValueFromColumn(colTypes[i], v)
					if cerr != nil {
						log.Errorf("unable to convert mysql column data: %v: %v", cols[i], cerr)
						continue
					}
					if tagVal != nil {
						feature.Tags[cols[i]] = tagVal
					}
				case int64:
					feature.Tags[cols[i]] = v
				case uint64:
					// BIGINT UNSIGNED arrives as typed uint64 under the
					// binary protocol; MVT has a native uint representation,
					// matching what the []byte ParseUint fallback returns.
					feature.Tags[cols[i]] = v
				case float32:
					// FLOAT arrives as typed float32 under the binary
					// protocol; widen through its shortest decimal form so
					// the tag equals the text protocol's float64 value
					// instead of exposing float32 artifacts (1.10000002...).
					if f64, cerr := strconv.ParseFloat(strconv.FormatFloat(float64(v), 'g', -1, 32), 64); cerr == nil {
						feature.Tags[cols[i]] = f64
					}
				case string:
					feature.Tags[cols[i]] = v
				case float64:
					feature.Tags[cols[i]] = v
				case time.Time:
					feature.Tags[cols[i]] = v.Format(time.RFC3339)
				default:
					log.Errorf("unexpected type for mysql column data: %v: %T", cols[i], v)
				}
			}
		}

		// drop rows whose geometry was undecodable or outside the tile
		if skipRow {
			return nil
		}
		if feature.Geometry == nil {
			return fmt.Errorf("mysql layer %q query did not return a non-null geometry column %q", pLayer.Name(), pLayer.geomFieldname)
		}

		features = append(features, feature)
		diagnostics.features++
		return nil
	}

	for rows.Next() {
		diagnostics.rows++
		// check if the context cancelled or timed out
		if ctx.Err() != nil {
			return ctx.Err()
		}

		vals := make([]interface{}, len(cols))
		valPtrs := make([]interface{}, len(cols))
		for i := 0; i < len(cols); i++ {
			valPtrs[i] = &vals[i]
		}

		if err = rows.Scan(valPtrs...); err != nil {
			err = preferContextError(ctx, err)
			if !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
				log.Errorf("err reading row values: %v", err)
			}
			return err
		}

		if pLayer.deferredInspection {
			deferredRows = append(deferredRows, vals)
			continue
		}

		if err := processRow(vals); err != nil {
			return err
		}
	}

	if pLayer.deferredInspection {
		// Deferred rows are reprocessed only for their geometry: the
		// canonical MapplGIS detection ran once at registration, so there is
		// no runtime system-info pass over the result set anymore.
		for _, vals := range deferredRows {
			if err := processRow(vals); err != nil {
				return err
			}
		}
	}

	if err := rows.Err(); err != nil {
		return err
	}
	// if every row in this tile had an undecodable geometry the layer is
	// effectively broken (misconfigured mos_precision or foreign blob
	// format) — surface it instead of silently rendering an empty tile
	if geomErr != nil && len(features) == 0 {
		return fmt.Errorf("no decodable geometries in layer %v: %v", pLayer.Name(), geomErr)
	}

	// Do not emit features until the complete result set has been consumed.
	// If the server drops the connection while rows are being read, the caller
	// can safely retry the read without duplicating partially emitted features.
	diagnostics.enter("callback")
	for i := range features {
		if err := fn(&features[i]); err != nil {
			return err
		}
	}
	return nil
}

// Close will close the Provider's database connection
func (p *Provider) Close() error {
	return p.db.Close()
}

// quoteIdentifier wraps an identifier in backticks, escaping embedded ones.
func quoteIdentifier(name string) string {
	// qualified names ("table.column") are quoted per part so the bounds
	// fields accept table-qualified columns without breaking the SQL
	if table, column, ok := strings.Cut(name, "."); ok {
		return quoteIdentifier(table) + "." + quoteIdentifier(column)
	}
	return "`" + strings.ReplaceAll(name, "`", "``") + "`"
}

// quoteTokenIdentifier quotes an identifier substituted for an
// !ID_FIELD!/!GEOM_FIELD! SQL token (audit P5-10). Values the user already
// wrapped in a complete quote pair (backticks or double quotes) pass through
// verbatim for backward compatibility; anything else is quoted with
// quoteIdentifier, which escapes embedded backticks and quotes qualified
// names per part so hostile names cannot break out of the identifier.
func quoteTokenIdentifier(name string) string {
	if isQuotedIdentifierValue(name) {
		return name
	}
	return quoteIdentifier(name)
}

// isQuotedIdentifierValue reports whether name is already wrapped in a
// complete quote pair (backticks or double quotes). Semantics are kept
// identical to the postgis provider's token quoting (audit P5-10).
func isQuotedIdentifierValue(name string) bool {
	if len(name) < 2 {
		return false
	}
	if name[0] != '`' && name[0] != '"' {
		return false
	}
	return name[len(name)-1] == name[0]
}

// wktPolygon formats an extent as a WKT POLYGON string for use with
// ST_GeomFromText in the !BBOX! replacement.
func wktPolygon(ext *geom.Extent) string {
	f := func(v float64) string {
		return strconv.FormatFloat(v, 'f', -1, 64)
	}
	return fmt.Sprintf("POLYGON((%v %v, %v %v, %v %v, %v %v, %v %v))",
		f(ext.MinX()), f(ext.MinY()),
		f(ext.MaxX()), f(ext.MinY()),
		f(ext.MaxX()), f(ext.MaxY()),
		f(ext.MinX()), f(ext.MaxY()),
		f(ext.MinX()), f(ext.MinY()),
	)
}

// reference to all instantiated providers
var providers []*Provider
var providersMu sync.Mutex

// Cleanup will close all database connections and destroy all previously instantiated Provider instances
func Cleanup() {
	providersMu.Lock()
	defer providersMu.Unlock()

	if len(providers) > 0 {
		log.Infof("cleaning up mysql providers")
	}

	for i := range providers {
		if err := providers[i].Close(); err != nil {
			log.Errorf("err closing connection: %v", err)
		}
	}

	providers = nil
}
