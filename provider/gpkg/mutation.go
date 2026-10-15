//go:build cgo

package gpkg

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"sync"

	"github.com/alexeydott/geom"
	"github.com/alexeydott/tegola/internal/log"
	"github.com/alexeydott/tegola/mos"
	"github.com/alexeydott/tegola/provider"
	pa "github.com/alexeydott/tegola/provider/audit"
	codec "github.com/alexeydott/tegola/provider/geometrycodec"
)

// sqliteWritableDSN opens the GeoPackage for writing. busy_timeout keeps
// writers patient; synchronous/foreign_keys are left at their safe
// defaults (never disabled for speed).
func sqliteWritableDSN(path string) string {
	return "file:" + path + "?_busy_timeout=10000&_foreign_keys=on"
}

// Writer adds the provider.MutationProvider contract to the GPKG provider.
// It is created lazily and only used when a collection passes write
// admission; the read provider stays read-only.
type Writer struct {
	provider *Provider
	mu       sync.Mutex
	db       *sql.DB
	dbOnce   sync.Once
	dbErr    error
	// mappings caches admission results per layer.
	mappings map[string]*writeMapping
}

// MutationWriter returns the provider.MutationProvider contract for this
// provider. No connection is opened until BeginFeatureTx; the read path
// stays on its read-only handle.
func (p *Provider) MutationWriter() provider.MutationProvider {
	return p.writer()
}

// CurrentRevision implements provider.RevisionReader via the writer (R01).
// Returns "", nil when the writer has no write DB (read-only provider).
func (p *Provider) CurrentRevision(ctx context.Context, layer string, featureID uint64) (string, error) {
	w, ok := p.MutationWriter().(*Writer)
	if !ok {
		return "", nil
	}
	return w.CurrentRevision(ctx, layer, featureID)
}

// DescribeWritable implements provider.MutationProvider via the writer.
func (p *Provider) DescribeWritable(ctx context.Context, layer string) (provider.WriteDescriptor, error) {
	return p.MutationWriter().DescribeWritable(ctx, layer)
}

// BeginFeatureTx implements provider.MutationProvider via the writer.
func (p *Provider) BeginFeatureTx(ctx context.Context, options provider.TxOptions) (provider.FeatureTx, error) {
	return p.MutationWriter().BeginFeatureTx(ctx, options)
}

// DescribeSchema implements provider.SchemaProvider via the writer.
func (p *Provider) DescribeSchema(ctx context.Context, layer string) (provider.SchemaDescriptor, error) {
	w, ok := p.MutationWriter().(*Writer)
	if !ok {
		return provider.SchemaDescriptor{}, &provider.MutationError{Kind: provider.MutationErrUnsupportedCapability, Reason: "schema not available"}
	}
	return w.DescribeSchema(ctx, layer)
}

func (p *Provider) writer() *Writer {
	p.writerMu.Lock()
	defer p.writerMu.Unlock()
	if p.cachedWriter == nil {
		p.cachedWriter = &Writer{provider: p, mappings: make(map[string]*writeMapping)}
	}
	return p.cachedWriter
}

// writeMapping is the admission result for one layer.
type writeMapping struct {
	createUnsupportedReason string
	layer                   *Layer
	table                   string
	idColumn                string
	geomColumn              string
	geomFormat              string // gpkg, wkb, wkt, mos
	geomType                string // point, linestring, polygon, ...
	geomSRID                uint64
	bboxColumns             [4]string
	mosOpts                 mos.Options                          // quantization for MOS encoding
	columns                 map[string]provider.ColumnDescriptor // by column name
	writable                map[string]string                    // public name -> column
	readOnly                []string
	domain                  string
}

// DescribeWritable implements provider.MutationProvider admission
// (ADR-0013). Only plain tablename layers with a single INTEGER PRIMARY
// KEY, a supported geometry encoding and no custom SQL are admitted.
func (w *Writer) DescribeWritable(ctx context.Context, layer string) (provider.WriteDescriptor, error) {
	m, err := w.mapping(ctx, layer)
	if err != nil {
		return provider.WriteDescriptor{}, err
	}
	// Defensive copies: callers must not be able to mutate the cached
	// admission result through the returned descriptor.
	writable := make(map[string]string, len(m.writable))
	for k, v := range m.writable {
		writable[k] = v
	}
	readOnly := append([]string(nil), m.readOnly...)
	wd := provider.WriteDescriptor{
		CreateUnsupportedReason: m.createUnsupportedReason,
		Layer:                   layer,
		Table:                   m.table,
		IDColumn:                m.idColumn,
		GeometryColumn:          m.geomColumn,
		GeometryType:            m.geomType,
		GeometrySRID:            m.geomSRID,
		WritableColumns:         writable,
		ReadOnlyColumns:         readOnly,
		Domain:                  m.domain,
	}
	return wd, nil
}

// DescribeSchema implements provider.SchemaProvider.
func (w *Writer) DescribeSchema(ctx context.Context, layer string) (provider.SchemaDescriptor, error) {
	m, err := w.mapping(ctx, layer)
	if err != nil {
		return provider.SchemaDescriptor{}, err
	}
	sd := provider.SchemaDescriptor{
		Layer:    layer,
		Table:    m.table,
		IDColumn: m.idColumn,
		Geometry: provider.GeometryColumnDescriptor{
			Name:     m.geomColumn,
			Type:     m.geomType,
			SRID:     m.geomSRID,
			Nullable: m.columns[m.geomColumn].Nullable,
		},
	}
	for _, col := range m.columns {
		_, writable := m.writable[col.Name]
		published := writable || col.Name == m.idColumn || col.Name == m.geomColumn
		for _, name := range m.readOnly {
			if name == col.Name {
				published = true
			}
		}
		if published {
			sd.Columns = append(sd.Columns, col)
		}
	}
	return sd, nil
}

func (w *Writer) mapping(ctx context.Context, layer string) (*writeMapping, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if m, ok := w.mappings[layer]; ok {
		return m, nil
	}
	l, ok := w.provider.layers[layer]
	if !ok {
		return nil, &provider.MutationError{Kind: provider.MutationErrNotFound, Reason: fmt.Sprintf("layer %q not registered", layer)}
	}
	m, err := admitLayer(w.provider.Filepath, l)
	if err != nil {
		return nil, err
	}
	w.mappings[layer] = m
	return m, nil
}

// admitLayer runs write admission for one GPKG layer.
func admitLayer(filepath string, l *Layer) (*writeMapping, error) {
	deny := func(reason string) (*writeMapping, error) {
		return nil, &provider.MutationError{Kind: provider.MutationErrUnsupportedCapability, Reason: reason}
	}
	if l.boundFieldnames != nil && l.geometryFormat != "mos" {
		return deny("writes to layers with derived bounds columns require MOS storage")
	}
	if l.sql != "" {
		return deny(fmt.Sprintf("layer %q uses custom SQL and is read-only", l.name))
	}
	if l.tablename == "" {
		return deny(fmt.Sprintf("layer %q has no table mapping", l.name))
	}
	format := l.geometryFormat
	if format == "" {
		format = "gpkg"
	}
	if format != "gpkg" && format != "wkb" && format != "wkt" && format != "mos" {
		return deny(fmt.Sprintf("layer %q has unsupported geometry format %q", l.name, format))
	}
	db, err := sql.Open(featureSQLiteDriver, sqliteWritableDSN(filepath))
	if err != nil {
		return nil, fmt.Errorf("write admission: open: %w", err)
	}
	defer func() { _ = db.Close() }()

	if format == "mos" {
		var registered int
		err := db.QueryRow(`SELECT COUNT(*) FROM gpkg_geometry_columns WHERE table_name=? AND column_name=?`, l.tablename, l.geomFieldname).Scan(&registered)
		if err != nil && !isNoSuchTable(err) {
			return nil, fmt.Errorf("MOS geometry metadata: %w", err)
		}
		if registered != 0 {
			return deny("MOS storage cannot replace a registered GeoPackage binary geometry column")
		}
	}
	if format == "gpkg" {
		var z, m int
		if err := db.QueryRow(`SELECT z,m FROM gpkg_geometry_columns WHERE table_name=? AND column_name=?`, l.tablename, l.geomFieldname).Scan(&z, &m); err != nil {
			return deny("GeoPackage geometry dimension metadata is unavailable")
		}
		if z != 0 || m != 0 {
			return deny("writes require an XY-only GeoPackage geometry column")
		}
	}
	cols, pkCols, err := tableColumnsAndPK(db, l.tablename)
	if err != nil {
		return deny(fmt.Sprintf("layer %q: %v", l.name, err))
	}
	// Starting profile: single INTEGER PRIMARY KEY (rowid alias).
	if len(pkCols) != 1 {
		return deny(fmt.Sprintf("layer %q: write requires a single-column primary key", l.name))
	}
	if err := checkIntegerPK(db, l.tablename, pkCols[0]); err != nil {
		return nil, err
	}
	colDesc, err := describeColumns(db, l.tablename, cols)
	if err != nil {
		return nil, err
	}
	geomCol := l.geomFieldname
	if _, ok := colDesc[geomCol]; !ok {
		return deny(fmt.Sprintf("layer %q: geometry column %q not found", l.name, geomCol))
	}
	geomType := normalizeGeomType(l.geomType)
	m := &writeMapping{
		layer:      l,
		table:      l.tablename,
		idColumn:   pkCols[0],
		geomColumn: geomCol,
		geomFormat: format,
		geomType:   geomType,
		geomSRID:   l.srid,
		mosOpts: mos.Options{
			Precision:  l.mosConfig.Precision,
			UnitFactor: l.mosConfig.UnitFactor,
		},
		columns:  colDesc,
		writable: make(map[string]string),
		domain:   "gpkg:" + filepath,
	}
	// Bounds are native MOS integer ticks, maintained atomically with geometry.
	boundsCols := map[string]bool{}
	if format == "mos" {
		fields := l.bboxFields
		if fields == (codec.BBoxFields{}) {
			fields = codec.DefaultBBoxFields()
		}
		if l.boundFieldnames != nil {
			fields = codec.BBoxFields(*l.boundFieldnames)
		}
		present := 0
		for i, name := range fields {
			for actual, descriptor := range colDesc {
				if !strings.EqualFold(name, actual) {
					continue
				}
				present++
				upper := strings.ToUpper(descriptor.Type)
				numeric := strings.Contains(upper, "INT") || (!strings.Contains(upper, "CHAR") && !strings.Contains(upper, "CLOB") && !strings.Contains(upper, "TEXT") && !strings.Contains(upper, "BLOB") && (strings.Contains(upper, "REAL") || strings.Contains(upper, "FLOA") || strings.Contains(upper, "DOUB") || strings.Contains(upper, "NUMERIC") || strings.Contains(upper, "DECIMAL")))
				if !numeric || descriptor.IsGenerated || boundsCols[actual] || actual == geomCol || actual == m.idColumn || (colDesc[geomCol].Nullable && !descriptor.Nullable) {
					return deny("MOS bounds require distinct writable numeric columns compatible with geometry nullability")
				}
				boundsCols[actual] = true
				m.bboxColumns[i] = actual
			}
		}
		if present == 4 && colDesc[geomCol].IsGenerated {
			return deny("MOS bounds maintenance requires a writable geometry column")
		}
		if present != 0 && present != 4 {
			return deny("MOS bounds mapping must contain all four columns")
		}
		if present == 0 && fields != codec.DefaultBBoxFields() {
			return deny("configured MOS bounds columns do not exist")
		}
	}
	if l.isMapplGIS {
		if m.bboxColumns[0] == "" || !strings.EqualFold(m.idColumn, "OKEY") || !strings.EqualFold(m.geomColumn, "LINE") {
			return deny("canonical MapplGIS writes require native identity, geometry and complete bounds mapping")
		}
		m.createUnsupportedReason = "canonical MapplGIS creation requires MUID, ObjectType and style initialization not supplied by this provider"
	}
	// Public writable properties: every non-PK, non-geometry column.
	// Bounds backing columns are excluded (derived data). Iterate the
	// table_xinfo descriptors (not table_info) so generated columns are
	// seen and forced read-only.
	public := map[string]bool{}
	for _, field := range l.tagFieldnames {
		public[field] = true
	}
	for c, d := range colDesc {
		if c == m.idColumn || c == m.geomColumn || boundsCols[c] || (len(l.tagFieldnames) > 0 && !public[c]) {
			continue
		}
		if l.isMapplGIS && (strings.EqualFold(c, "MUID") || strings.EqualFold(c, "ObjectStyle") || strings.EqualFold(c, "ObjectType")) {
			m.readOnly = append(m.readOnly, c)
			continue
		}
		if d.IsGenerated {
			m.readOnly = append(m.readOnly, c)
			continue
		}
		m.writable[c] = c
	}
	m.readOnly = append(m.readOnly, m.idColumn)
	return m, nil
}

func checkIntegerPK(db *sql.DB, table, pk string) error {
	rows, err := db.Query(fmt.Sprintf("PRAGMA table_info(%v);", quoteIdent(table)))
	if err != nil {
		return &provider.MutationError{Kind: provider.MutationErrUnsupportedCapability, Reason: fmt.Sprintf("table %q: %v", table, err)}
	}
	defer func() { _ = rows.Close() }()
	found := false
	for rows.Next() {
		var cid int
		var name, ctype string
		var notNull int
		var dflt sql.NullString
		var pkPos int
		if err := rows.Scan(&cid, &name, &ctype, &notNull, &dflt, &pkPos); err != nil {
			return err
		}
		if name == pk {
			// Rowid alias rule: declared type exactly INTEGER, sole PK.
			if strings.ToUpper(strings.TrimSpace(ctype)) != "INTEGER" {
				return &provider.MutationError{
					Kind:   provider.MutationErrUnsupportedCapability,
					Reason: fmt.Sprintf("table %q: write requires INTEGER PRIMARY KEY, got %q", table, ctype),
				}
			}
			found = true
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if found {
		return nil
	}
	return &provider.MutationError{Kind: provider.MutationErrNotFound, Reason: fmt.Sprintf("table %q: pk column %q not found", table, pk)}
}

func describeColumns(db *sql.DB, table string, cols []string) (map[string]provider.ColumnDescriptor, error) {
	out := make(map[string]provider.ColumnDescriptor, len(cols))
	// table_xinfo exposes hidden generated columns (hidden=2 virtual,
	// hidden=3 stored) that table_info hides. Generated columns are
	// always read-only: INSERT/UPDATE must reject them.
	rows, err := db.Query(fmt.Sprintf("PRAGMA table_xinfo(%v);", quoteIdent(table)))
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var cid int
		var name, ctype string
		var notNull int
		var dflt sql.NullString
		var pk, hidden int
		if err := rows.Scan(&cid, &name, &ctype, &notNull, &dflt, &pk, &hidden); err != nil {
			return nil, err
		}
		out[name] = provider.ColumnDescriptor{
			Name:        name,
			Type:        ctype,
			Nullable:    notNull == 0,
			IsDefault:   dflt.Valid,
			IsGenerated: hidden >= 2,
		}
	}
	return out, rows.Err()
}

func normalizeGeomType(g geom.Geometry) string {
	switch g.(type) {
	case geom.Point:
		return "point"
	case geom.LineString:
		return "linestring"
	case geom.Polygon:
		return "polygon"
	case geom.MultiPoint:
		return "multipoint"
	case geom.MultiLineString:
		return "multilinestring"
	case geom.MultiPolygon:
		return "multipolygon"
	default:
		return "geometry"
	}
}

// BeginFeatureTx implements provider.MutationProvider.
// CurrentRevision implements provider.RevisionReader (A03).
func (w *Writer) CurrentRevision(ctx context.Context, layer string, featureID uint64) (string, error) {
	l, ok := w.provider.layers[layer]
	if !ok {
		return "", nil
	}
	// SQLite INTEGER identities cannot represent the upper half of uint64.
	// Match feature reads without narrowing an impossible ID into a negative one.
	if featureID > math.MaxInt64 {
		return "0.0", nil
	}
	layer = l.tablename
	var rev, incarnation int64
	err := w.provider.db.QueryRowContext(ctx, `SELECT revision, incarnation FROM tegola_revisions WHERE collection = ? AND feature_id = ?`, layer, int64(featureID)).Scan(&rev, &incarnation)
	if err != nil {
		// R01/R09: distinguish missing row (revision 0) from
		// storage errors (including missing table). Do not mask
		// errors as "0".
		if errors.Is(err, sql.ErrNoRows) {
			return "0.0", nil
		}
		if pa.IsMissingTable(err) {
			return "0.0", nil
		}
		return "", err
	}
	return strconv.FormatInt(incarnation, 10) + "." + strconv.FormatInt(rev, 10), nil
}

func (w *Writer) BeginFeatureTx(ctx context.Context, options provider.TxOptions) (provider.FeatureTx, error) {
	// A13: db init via Once (no mutex held during BeginTx).
	w.dbOnce.Do(func() {
		db, err := sql.Open(featureSQLiteDriver, sqliteWritableDSN(w.provider.Filepath))
		if err != nil {
			w.dbErr = fmt.Errorf("begin tx: open: %w", err)
			return
		}
		db.SetMaxOpenConns(1)
		w.db = db
	})
	if w.dbErr != nil {
		return nil, w.dbErr
	}
	db := w.db

	// BeginTx without holding w.mu (mapping needs it in Apply).
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	// W13: ensure audit tables exist (SQLite DDL in tx is safe; R09
	// migration gate applies to MySQL/PostgreSQL where DDL breaks tx).
	if err := ensureAuditTables(ctx, tx); err != nil {
		_ = tx.Rollback()
		return nil, err
	}
	return &featureTx{writer: w, tx: tx, actor: options.Actor, requestID: options.RequestID}, nil
}

// featureTx is one native SQLite transaction.
type featureTx struct {
	failed error
	writer *Writer
	tx     *sql.Tx
	done   bool
	// modified tracks tables touched by this transaction for GeoPackage
	// metadata maintenance (gpkg_contents.last_change, RTree) at commit.
	modified map[string]*tableModification
	// W13 audit context
	actor     string
	requestID string
}

// tableModification records row-level changes for one table.
type tableModification struct {
	mosStorage bool
	table      string
	geomCol    string
	// geomChanged maps row id -> new bounds; nil bounds means the row was
	// deleted (drop the RTree entry).
	geomChanged map[uint64]*[4]float64
}

func (t *featureTx) mapping(ctx context.Context, layer string) (*writeMapping, error) {
	// A37: use request context, not Background.
	return t.writer.mapping(ctx, layer)
}

// Apply implements provider.FeatureTx.
func (t *featureTx) Apply(ctx context.Context, m provider.Mutation) (result provider.MutationOutcome, applyErr error) {
	if t.failed != nil {
		return result, t.failed
	}
	defer func() {
		if applyErr != nil {
			if _, classified := provider.AsMutationError(applyErr); !classified {
				applyErr = &provider.MutationError{Kind: provider.MutationErrExecutionFailed, Reason: applyErr.Error()}
			}
			t.failed = applyErr
		}
	}()
	if t.done {
		return provider.MutationOutcome{}, &provider.MutationError{Kind: provider.MutationErrMalformedInput, Reason: "transaction already finished"}
	}
	mp, err := t.mapping(ctx, m.Collection)
	if err != nil {
		return provider.MutationOutcome{}, err
	}
	if mp.layer.isMapplGIS && m.Op != provider.MutationInsert && m.GeometryWKB != nil {
		if err := t.validateCanonicalGeometry(ctx, mp, m); err != nil {
			return provider.MutationOutcome{}, err
		}
	}
	var outcome provider.MutationOutcome
	switch m.Op {
	case provider.MutationInsert:
		if mp.createUnsupportedReason != "" {
			return provider.MutationOutcome{}, &provider.MutationError{Kind: provider.MutationErrUnsupportedCapability, Reason: mp.createUnsupportedReason}
		}
		outcome, err = t.insert(ctx, mp, m)
	case provider.MutationReplace:
		outcome, err = t.replace(ctx, mp, m)
	case provider.MutationUpdate:
		outcome, err = t.update(ctx, mp, m)
	case provider.MutationDelete:
		outcome, err = t.delete(ctx, mp, m)
	default:
		return provider.MutationOutcome{}, &provider.MutationError{Kind: provider.MutationErrMalformedInput, Reason: "unknown op"}
	}
	if err != nil {
		return provider.MutationOutcome{}, err
	}
	{
		// A03: revision check + bump inside the data transaction.
		bump, rerr := pa.CheckAndBumpRevisionSQL(ctx, t.tx, mp.revisionCollection(), outcome.FeatureID, m.IfRevision, "sqlite")
		if rerr != nil {
			return provider.MutationOutcome{}, rerr
		}
		if m.Op == provider.MutationDelete {
			if err := pa.BumpIncarnationOnDelete(ctx, t.tx, mp.revisionCollection(), outcome.FeatureID, "sqlite"); err != nil {
				return provider.MutationOutcome{}, err
			}
			bump.NewInc++
			bump.New = 0
		}
		// Revision is incarnation.revision.
		outcome.Revision = formatIncarnationRevision(bump.NewInc, bump.New)
		outcome.RevisionBefore = formatIncarnationRevision(bump.OldInc, bump.Old)
	}
	// W13: audit + outbox in the same transaction as the data
	entry := auditEntryFor(m.Collection, m.Op, outcome, t.actor, t.requestID, "")
	if aerr := recordAuditTx(ctx, t.tx, entry, outboxEventType(m.Op)); aerr != nil {
		return provider.MutationOutcome{}, aerr
	}
	return outcome, nil
}

func (t *featureTx) Commit(ctx context.Context) (provider.CommitReceipt, error) {
	if t.done {
		return provider.CommitReceipt{}, &provider.MutationError{Kind: provider.MutationErrMalformedInput, Reason: "transaction already finished"}
	}
	t.done = true
	if t.failed != nil {
		_ = t.tx.Rollback()
		return provider.CommitReceipt{Status: provider.CommitNotCommitted}, t.failed
	}
	if err := t.maintainGPKGMetadata(ctx); err != nil {
		_ = t.tx.Rollback()
		return provider.CommitReceipt{Status: provider.CommitNotCommitted}, &provider.MutationError{
			Kind:   provider.MutationErrExecutionFailed,
			Reason: fmt.Sprintf("gpkg metadata maintenance failed: %v", err),
		}
	}
	if err := t.tx.Commit(); err != nil {
		return provider.CommitReceipt{Status: provider.CommitUnknown}, &provider.MutationError{
			Kind:   provider.MutationErrCommitUnknown,
			Reason: fmt.Sprintf("commit failed: %v", err),
		}
	}
	return provider.CommitReceipt{Status: provider.CommitCommitted}, nil
}

// maintainGPKGMetadata updates GeoPackage bookkeeping for tables touched
// by this transaction: gpkg_contents.last_change and the RTree spatial
// index (when present). Runs inside the transaction, before commit.
func (t *featureTx) maintainGPKGMetadata(ctx context.Context) error {
	for _, mod := range t.modified {
		if err := t.maintainRTree(ctx, mod); err != nil {
			return err
		}
		if err := t.touchContents(ctx, mod); err != nil {
			return err
		}
	}
	return nil
}

// isNoSuchTable reports whether err is SQLite "no such table".
func isNoSuchTable(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "no such table")
}

// touchContents bumps gpkg_contents.last_change for a feature table.
// Tables without a gpkg_contents row (plain SQLite tables) are skipped.
func (t *featureTx) touchContents(ctx context.Context, mod *tableModification) error {
	table := mod.table
	var n int
	if err := t.tx.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM gpkg_contents WHERE table_name = ?`, table).Scan(&n); err != nil {
		// A33: distinguish "no gpkg_contents table" (plain SQLite, skip)
		// from real storage errors (propagate).
		if isNoSuchTable(err) {
			return nil
		}
		return err
	}
	if n == 0 {
		return nil
	}
	// Raw MOS is an auxiliary storage profile, not the registered GPKG geometry.
	// Its changes must not overwrite extents belonging to another native column.
	if mod.mosStorage {
		_, err := t.tx.ExecContext(ctx, "UPDATE gpkg_contents SET last_change=strftime('%Y-%m-%dT%H:%M:%fZ','now') WHERE table_name=?", table)
		return err
	}
	extent := "min_x=NULL,max_x=NULL,min_y=NULL,max_y=NULL"
	rtree := "rtree_" + mod.table + "_" + mod.geomCol
	var hasRTree int
	if err := t.tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?`, rtree).Scan(&hasRTree); err != nil {
		return err
	}
	if hasRTree > 0 {
		extent = fmt.Sprintf("min_x=(SELECT MIN(minx) FROM %s),max_x=(SELECT MAX(maxx) FROM %s),min_y=(SELECT MIN(miny) FROM %s),max_y=(SELECT MAX(maxy) FROM %s)", quoteIdent(rtree), quoteIdent(rtree), quoteIdent(rtree), quoteIdent(rtree))
	}
	_, err := t.tx.ExecContext(ctx, "UPDATE gpkg_contents SET last_change=strftime('%Y-%m-%dT%H:%M:%fZ','now'),"+extent+" WHERE table_name=?", table)
	return err
}

// maintainRTree syncs the RTree index for changed rows. Tables without an
// RTree index table are skipped. Row-level triggers (created by
// GeoPackage writers) would already have fired; this covers the
// trigger-less case by reconciling entries for the touched rows.
func (t *featureTx) maintainRTree(ctx context.Context, mod *tableModification) error {
	rtree := "rtree_" + mod.table + "_" + mod.geomCol
	var name string
	if err := t.tx.QueryRowContext(ctx,
		`SELECT name FROM sqlite_master WHERE type='table' AND name=?`, rtree).Scan(&name); err != nil {
		// A33: ErrNoRows means no RTree (skip); other errors propagate.
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		return err
	}
	for id, bounds := range mod.geomChanged {
		if _, err := t.tx.ExecContext(ctx,
			fmt.Sprintf(`DELETE FROM %s WHERE id = ?`, quoteIdent(rtree)), id); err != nil {
			return err
		}
		if bounds == nil {
			continue // deleted row: entry removed above
		}
		if _, err := t.tx.ExecContext(ctx,
			fmt.Sprintf(`INSERT INTO %s (id, minx, maxx, miny, maxy) VALUES (?, ?, ?, ?, ?)`, quoteIdent(rtree)),
			id, bounds[0], bounds[1], bounds[2], bounds[3]); err != nil {
			return err
		}
	}
	return nil
}

// recordMod marks a table as touched by this transaction.
func (t *featureTx) recordMod(mp *writeMapping, id uint64, bounds *[4]float64, geomTouched bool) {
	if t.modified == nil {
		t.modified = make(map[string]*tableModification)
	}
	mod, ok := t.modified[mp.table]
	if !ok {
		mod = &tableModification{mosStorage: mp.geomFormat == "mos", table: mp.table, geomCol: mp.geomColumn, geomChanged: make(map[uint64]*[4]float64)}
		t.modified[mp.table] = mod
	}
	if geomTouched {
		mod.geomChanged[id] = bounds
	}
}

// geometryBounds computes the 2D envelope [minx, maxx, miny, maxy] of a
// decoded geometry for RTree maintenance.
func geometryBounds(g geom.Geometry) ([4]float64, error) {
	var b [4]float64
	pts, err := geom.GetCoordinates(g)
	if err != nil {
		return b, err
	}
	if len(pts) == 0 {
		return b, fmt.Errorf("empty geometry has no bounds")
	}
	b = [4]float64{pts[0][0], pts[0][0], pts[0][1], pts[0][1]}
	for _, p := range pts[1:] {
		if p[0] < b[0] {
			b[0] = p[0]
		}
		if p[0] > b[1] {
			b[1] = p[0]
		}
		if p[1] < b[2] {
			b[2] = p[1]
		}
		if p[1] > b[3] {
			b[3] = p[1]
		}
	}
	return b, nil
}

func (t *featureTx) Rollback(ctx context.Context) error {
	if t.done {
		return nil
	}
	t.done = true
	if err := t.tx.Rollback(); err != nil {
		log.Logger().Error("wfs tx rollback failed", "error", err)
		return err
	}
	return nil
}

// selectRow reads the current stored row inside the transaction for
// existence, revision checks and post-image validation.
func (t *featureTx) selectRow(ctx context.Context, mp *writeMapping, id uint64) (map[string]interface{}, bool, error) {
	cols := []string{quoteIdent(mp.idColumn), quoteIdent(mp.geomColumn)}
	for pub := range mp.writable {
		cols = append(cols, quoteIdent(mp.writable[pub]))
	}
	q := fmt.Sprintf("SELECT %s FROM %s WHERE %s = ?", strings.Join(cols, ", "), quoteIdent(mp.table), quoteIdent(mp.idColumn))
	rows, err := t.tx.QueryContext(ctx, q, id)
	if err != nil {
		return nil, false, fmt.Errorf("select row: %w", err)
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		return nil, false, nil
	}
	names := append([]string{mp.idColumn, mp.geomColumn}, writablePublicNames(mp)...)
	vals := make([]interface{}, len(names))
	ptrs := make([]interface{}, len(names))
	for i := range vals {
		ptrs[i] = &vals[i]
	}
	if err := rows.Scan(ptrs...); err != nil {
		return nil, false, fmt.Errorf("scan row: %w", err)
	}
	// LayerSystemInfo records are metadata, even in a noncanonical MOS table.
	// Classify the row inside this transaction before any mutation can touch it.
	if mp.geomFormat == "mos" && codec.IsSystemInfoValue(vals[1]) {
		return nil, false, nil
	}
	out := make(map[string]interface{}, len(names))
	for i, n := range names {
		out[n] = vals[i]
	}
	return out, true, rows.Err()
}

func writablePublicNames(mp *writeMapping) []string {
	names := make([]string, 0, len(mp.writable))
	for pub := range mp.writable {
		names = append(names, pub)
	}
	return names
}

// formatRevision renders a revision counter for ETag/If-Match use.
func formatRevision(n int64) string {
	// R01: negative means revisions not migrated -> no revision ETag.
	if n < 0 {
		return ""
	}
	return strconv.FormatInt(n, 10)
}

// formatIncarnationRevision renders "incarnation.revision" (A38).
func formatIncarnationRevision(inc, rev int64) string {
	if rev < 0 {
		return ""
	}
	return strconv.FormatInt(inc, 10) + "." + strconv.FormatInt(rev, 10)
}

func (m *writeMapping) revisionCollection() string { return m.table }
