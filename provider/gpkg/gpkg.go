//go:build cgo

package gpkg

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/alexeydott/geom"
	"github.com/alexeydott/geom/encoding/wkb"
	"github.com/alexeydott/tegola"
	"github.com/alexeydott/tegola/basic"
	"github.com/alexeydott/tegola/internal/log"
	"github.com/alexeydott/tegola/provider"
	codec "github.com/alexeydott/tegola/provider/geometrycodec"
)

const (
	Name                 = "gpkg"
	DefaultSRID          = tegola.WebMercator
	DefaultIDFieldName   = "fid"
	DefaultGeomFieldName = "geom"
)

// config keys
const (
	ConfigKeyFilePath    = "filepath"
	ConfigKeySRID        = "srid"
	ConfigKeyCRSDefn     = "crs_defn"
	ConfigKeyLayers      = "layers"
	ConfigKeyLayerName   = "name"
	ConfigKeyTableName   = "tablename"
	ConfigKeySQL         = "sql"
	ConfigKeyGeomIDField = "id_fieldname"
	ConfigKeyGeomField   = "geometry_fieldname"
	ConfigKeyFields      = "fields"
)

// Geometry format values accepted by the gpkg provider's geometry_format
// setting. "" and "gpkg" select the native GeoPackage binary layout.
const (
	GeometryFormatGPKG = "gpkg"
	GeometryFormatWKB  = codec.FormatWKB
	GeometryFormatWKT  = codec.FormatWKT
	GeometryFormatMOS  = codec.FormatMOS
)

// resolveGeometryFormat validates the geometry_format value.
func resolveGeometryFormat(v string) (string, error) {
	switch v {
	case "", GeometryFormatGPKG, GeometryFormatWKB, GeometryFormatWKT, GeometryFormatMOS:
		return v, nil
	default:
		return "", fmt.Errorf("invalid %v: %q (expected one of %q, %q, %q, %q)",
			codec.ConfigKeyGeometryFormat, v,
			GeometryFormatGPKG, GeometryFormatWKB, GeometryFormatWKT, GeometryFormatMOS)
	}
}

// gpkgGeometryFormats is the set of geometry_format values accepted at the
// layer level (an empty value falls back to the provider-level value).
var gpkgGeometryFormats = map[string]struct{}{
	GeometryFormatGPKG: {},
	GeometryFormatWKB:  {},
	GeometryFormatWKT:  {},
	GeometryFormatMOS:  {},
}

func decodeGeometry(bytes []byte) (*BinaryHeader, geom.Geometry, error) {
	h, err := NewBinaryHeader(bytes)
	if err != nil {
		log.Errorf("error decoding geometry header: %v", err)
		return h, nil, err
	}

	// audit N15: a GeoPackage binary header with the empty-geometry flag
	// set carries no WKB tail (the remainder is usually zero-length):
	// honor the flag instead of feeding it to the WKB decoder. The header
	// is still returned so callers can read the SRS id; the nil geometry
	// marks the row as geometry-less.
	if h.IsGeometryEmpty() {
		return h, nil, nil
	}

	geo, err := wkb.DecodeBytes(bytes[h.Size():])
	if err != nil {
		log.Errorf("error decoding geometry: %v", err)
		return h, nil, err
	}

	return h, geo, nil
}

// decodeGeometryValue decodes a geometry column value according to the
// layer's geometry format. Only the default GeoPackage binary layout carries
// a GeoPackageBinaryHeader; wkb/wkt/mos values are passed through the shared
// codec, which returns no header.
func decodeGeometryValue(v interface{}, format string, mosCfg codec.MOSConfig) (*BinaryHeader, geom.Geometry, error) {
	if format == "" || format == GeometryFormatGPKG {
		geomData, ok := v.([]byte)
		if !ok {
			return nil, nil, errors.New("unexpected column type for geom field. expected blob")
		}
		return decodeGeometry(geomData)
	}

	switch format {
	case GeometryFormatWKB:
		geo, err := codec.DecodeWKB(v)
		if err != nil {
			return nil, nil, err
		}
		return nil, geo, nil
	case GeometryFormatWKT:
		geo, err := codec.DecodeWKT(v)
		if err != nil {
			return nil, nil, err
		}
		return nil, geo, nil
	case GeometryFormatMOS:
		geo, err := codec.DecodeMOS(v, mosCfg)
		if err != nil {
			return nil, nil, err
		}
		return nil, geo, nil
	default:
		return nil, nil, fmt.Errorf("unknown geometry_format: %v", format)
	}
}

// quoteIdent quotes a SQLite identifier, escaping embedded backticks so a
// crafted config value cannot break out of the quoted name.
func quoteIdent(name string) string {
	return "`" + strings.ReplaceAll(name, "`", "``") + "`"
}

// rawBoundsSQL builds a coarse SQL bounds filter from the raw bounds columns
// detected at registration for a non-GPKG geometry format, mirroring the
// MySQL provider's MOS bounds filter. The stored values use the layer's
// stored coordinate units: quantized MOS units for the MOS format (scaled by
// 10^precision / unit factor), otherwise the layer CRS coordinates that the
// source extent is already expressed in. An empty result means the filter
// cannot be applied and filtering must happen in memory after decoding.
//
// The predicate shape and MOS raw scaling are shared with the other raw
// providers through codec.BuildBoundsPredicate; the [4]string order here is
// minx/maxx/miny/maxy.
func rawBoundsSQL(l *Layer, extent *geom.Extent) string {
	if l.boundFieldnames == nil || extent == nil {
		return ""
	}
	mode := codec.BoundsSourceCRS
	if l.geometryFormat == codec.FormatMOS {
		mode = codec.BoundsMOSRaw
	}
	predicate, err := codec.BuildBoundsPredicate(
		codec.BBoxFields{
			l.boundFieldnames[0], l.boundFieldnames[1],
			l.boundFieldnames[2], l.boundFieldnames[3],
		},
		extent, mode, l.mosConfig, quoteIdent,
	)
	if err != nil {
		return ""
	}
	return predicate
}

type Provider struct {
	// path to the geopackage file
	Filepath string
	// layers maps names to layer definitions finalized by NewTileProvider,
	// including MOS system-info and CRS metadata. The map and its layer
	// definitions are read-only after registration.
	layers map[string]*Layer
	// reference to the database connection
	db *sql.DB
	// default SRID for the provider
	srid uint64
	// mu guards warn-once bookkeeping below; tile requests run concurrently.
	mu sync.Mutex
	// warned tracks warn-once keys (unexpected column types, late
	// system-info rows) so a warning is emitted once instead of per row.
	warned map[string]struct{}
	// writerMu guards the cached mutation Writer. One Writer (and one
	// write sqlite pool) per Provider: creating a fresh Writer per
	// transaction would leak sql.DB pools.
	writerMu     sync.Mutex
	cachedWriter *Writer
}

// ErrUnknownLayer denotes a layer name that is not registered on the provider.
type ErrUnknownLayer struct {
	Name string
}

func (e ErrUnknownLayer) Error() string {
	return fmt.Sprintf("layer not registered with provider: %v", e.Name)
}

func (p *Provider) Layers() ([]provider.LayerInfo, error) {
	log.Debug("attempting gpkg.Layers()")

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
	log.Debugf("fetching layer %v", layer)

	pLayer, ok := p.layers[layer]
	if !ok {
		return ErrUnknownLayer{layer}
	}

	// Tile requests never repeat discovery (audit A-06): registration
	// finalized all MOS system-info metadata, so no runtime pre-query
	// runs here.

	// read the tile extent
	tileBBox, tileSRID := tile.BufferedExtent()

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
	var err error
	args := make([]interface{}, 0)

	if pLayer.tablename != "" {
		if pLayer.geometryFormat != "" && pLayer.geometryFormat != GeometryFormatGPKG {
			// Non-native geometry formats (wkb/wkt/mos) live in plain
			// tables without a GeoPackage binary header or RTree index.
			// When the table carries raw bounds columns (minx/maxx/miny/
			// maxy, detected at registration) they act as a coarse SQL
			// filter; otherwise the whole geometry column is scanned and
			// the exact in-memory filter below is the only one.
			selectClause := fmt.Sprintf("SELECT l.%v, l.%v", quoteIdent(pLayer.idFieldname), quoteIdent(pLayer.geomFieldname))

			for _, tf := range pLayer.tagFieldnames {
				selectClause += fmt.Sprintf(", l.%v", quoteIdent(tf))
			}

			where := fmt.Sprintf("l.%v IS NOT NULL", quoteIdent(pLayer.geomFieldname))
			if bboxSQL := rawBoundsSQL(pLayer, tileBBox); bboxSQL != "" {
				where += " AND " + bboxSQL
			}
			qtext = fmt.Sprintf("%v FROM %v l WHERE %v", selectClause, quoteIdent(pLayer.tablename), where)
		} else {
			// If layer was specified via "tablename" in config, construct
			// query. The spatial pre-filter plan was detected and cached at
			// registration (UPSTREAM debt 0.6b): exactly one SQL round trip
			// per tile request, never a per-request existence probe.
			rtreeTablename := fmt.Sprintf("rtree_%v_%v", pLayer.tablename, pLayer.geomFieldname)

			selectClause := fmt.Sprintf("SELECT l.%v, l.%v", quoteIdent(pLayer.idFieldname), quoteIdent(pLayer.geomFieldname))

			for _, tf := range pLayer.tagFieldnames {
				selectClause += fmt.Sprintf(", l.%v", quoteIdent(tf))
			}

			switch pLayer.tileQueryPlan {
			case planBBox:
				// No RTree shadow table: filter on the table's bounds
				// columns instead. The !BBOX! expansion names the bounds
				// columns unqualified, resolving against the single table.
				qtext = fmt.Sprintf("%v FROM %v l WHERE l.%v IS NOT NULL AND !BBOX! ORDER BY l.%v, l.rowid", selectClause, quoteIdent(pLayer.tablename), quoteIdent(pLayer.geomFieldname), quoteIdent(pLayer.idFieldname))

				// bounds predicate build errors are fail-closed (A12): surface
				// them instead of silently running unfiltered SQL.
				qtext, err = replaceTokens(qtext, pLayer, tile, tileBBox)
				if err != nil {
					return err
				}
			case planScan:
				// Neither RTree nor bounds columns: scan the table with no
				// spatial pre-filter; the mandatory in-memory exact filter
				// below is the only one. No !BBOX! token here, so
				// replaceTokens (whose bounds predicate build is lazy) is
				// not needed - matching the raw-format branch.
				qtext = fmt.Sprintf("%v FROM %v l WHERE l.%v IS NOT NULL ORDER BY l.%v, l.rowid", selectClause, quoteIdent(pLayer.tablename), quoteIdent(pLayer.geomFieldname), quoteIdent(pLayer.idFieldname))
			default: // planRTree
				// l - layer table, si - spatial index. The RTree's id is the
				// table rowid (audit P5-5): join on l.rowid directly so the
				// join is deterministic regardless of the configured id
				// column (which only feeds feature IDs and is verified at
				// registration). ORDER BY keeps row order deterministic so
				// concurrent requests stream identical rows (stable MVT
				// output); rowid breaks ties for non-unique id columns.
				qtext = fmt.Sprintf("%v FROM %v l JOIN %v si ON l.rowid = si.id WHERE l.%v IS NOT NULL AND !BBOX! ORDER BY l.%v, l.rowid", selectClause, quoteIdent(pLayer.tablename), quoteIdent(rtreeTablename), quoteIdent(pLayer.geomFieldname), quoteIdent(pLayer.idFieldname))

				// bounds predicate build errors are fail-closed (A12): surface
				// them instead of silently running unfiltered SQL.
				qtext, err = replaceTokens(qtext, pLayer, tile, tileBBox)
				if err != nil {
					return err
				}
			}
		}
	} else {
		// If layer was specified via "sql" in config, collect it
		qtext, err = replaceTokens(pLayer.sql, pLayer, tile, tileBBox)
		if err != nil {
			return err
		}
		qtext = queryParams.ReplaceParamsWithDialect(qtext, &args, provider.SQLDialectSQLite)
	}

	log.Debugf("qtext: %v", qtext)

	// QueryContext stops the SQLite query when the request context is
	// cancelled instead of running it to completion.
	rows, err := p.db.QueryContext(ctx, qtext, args...)
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return err
		}
		log.Errorf("err during query: %v - %v", qtext, err)
		return err
	}
	defer func() { _ = rows.Close() }()

	cols, err := rows.Columns()
	if err != nil {
		return err
	}

	for rows.Next() {
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
			log.Errorf("err reading row values: %v", err)
			return err
		}

		decoded, err := p.decodeRow(
			ctx,
			pLayer,
			cols,
			vals,
			rowDecodePolicy{tolerateAutoDetectedMOS: true},
		)
		if err != nil {
			return err
		}

		if !decoded.Representable || decoded.HadAbsentGeometry || decoded.Feature.Geometry == nil {
			continue
		}

		feature := decoded.Feature

		// Exact in-memory filter. Mandatory for wkb/wkt/mos formats whose
		// queries cannot use the RTree join and for planScan native layers
		// (no spatial pre-filter in SQL at all); harmless for the planRTree
		// and planBBox native plans that already filtered via !BBOX!.
		if !codec.GeometryIntersectsExtent(feature.Geometry, tileBBox) {
			continue
		}

		// pass the feature to the provided call back
		if err = fn(&feature); err != nil {
			return err
		}
	}

	if err := rows.Err(); err != nil {
		return err
	}
	return nil
}

// warnOnce logs a warning only the first time it is called with a given key.
// Guarded by mu because tile requests run concurrently.
func (p *Provider) warnOnce(key string, format string, args ...interface{}) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.warned == nil {
		p.warned = make(map[string]struct{})
	}
	if _, ok := p.warned[key]; ok {
		return
	}
	p.warned[key] = struct{}{}
	log.Warnf(format, args...)
}

// Close will close the Provider's database connection
func (p *Provider) Close() error {
	p.writerMu.Lock()
	defer p.writerMu.Unlock()
	var writeErr error
	if p.cachedWriter != nil && p.cachedWriter.db != nil {
		writeErr = p.cachedWriter.db.Close()
	}
	return errors.Join(p.db.Close(), writeErr)
}

func geomNameToGeom(name string) (geom.Geometry, error) {
	switch name {
	case "POINT":
		return geom.Point{}, nil
	case "LINESTRING":
		return geom.LineString{}, nil
	case "POLYGON":
		return geom.Polygon{}, nil
	case "MULTIPOINT":
		return geom.MultiPoint{}, nil
	case "MULTILINESTRING":
		return geom.MultiLineString{}, nil
	case "MULTIPOLYGON":
		return geom.MultiPolygon{}, nil
	case "GEOMETRYCOLLECTION":
		return geom.Collection{}, nil
	case "GEOMETRY":
		return nil, nil
	}

	return nil, fmt.Errorf("unsupported geometry type: %v", name)
}
