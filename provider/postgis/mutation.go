package postgis

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math/rand"
	"sort"
	"strings"
	"sync"
	"time"

	"crypto/sha256"
	"encoding/hex"
	"github.com/alexeydott/tegola/mos"
	"strconv"

	"github.com/alexeydott/tegola/provider"
	pa "github.com/alexeydott/tegola/provider/audit"
	"github.com/jackc/pgx/v5"
)

// Writer implements provider.MutationProvider for PostGIS.
type Writer struct {
	provider *Provider
	mu       sync.Mutex
	mappings map[string]*writeMapping
}

func (p *Provider) MutationWriter() provider.MutationProvider {
	return p.writer()
}

// CurrentRevision implements provider.RevisionReader via the writer (R01).
// Returns "", nil when the writer has no write DB (read-only provider).
func (p *Provider) CurrentRevision(ctx context.Context, layer string, featureID uint64) (string, error) {
	w, ok := p.MutationWriter().(*Writer)
	if !ok || w.provider.pool == nil {
		return "", nil
	}
	return w.CurrentRevision(ctx, layer, featureID)
}

func (p *Provider) DescribeWritable(ctx context.Context, layer string) (provider.WriteDescriptor, error) {
	return p.MutationWriter().DescribeWritable(ctx, layer)
}

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

// domainID returns an opaque identifier for the physical PostgreSQL
// database this provider instance is connected to.
//
// A02: the domain must identify the physical DB (host+dbname), not a
// schema or table. Two provider instances pointing at different databases
// must never share a domain even if schema names match; tables in
// different schemas of the SAME database share one domain because
// PostgreSQL can cover them in a single native transaction.
func (p *Provider) domainID() string {
	cc := p.config.ConnConfig
	// host|port|database uniquely identifies the physical DB.
	// User/password are excluded: same DB, different roles = same domain.
	sum := sha256.Sum256([]byte(cc.Host + "|" + strconv.Itoa(int(cc.Port)) + "|" + cc.Database))
	return "postgis:" + hex.EncodeToString(sum[:])[:16]
}

func (p *Provider) writer() *Writer {
	// R08: cache the writer so mappings persist across calls.
	p.writerMu.Lock()
	defer p.writerMu.Unlock()
	if p.cachedWriter == nil {
		p.cachedWriter = &Writer{provider: p, mappings: make(map[string]*writeMapping)}
	}
	return p.cachedWriter
}

type writeMapping struct {
	createUnsupportedReason string
	schema                  string
	table                   string
	idColumn                string
	geomColumn              string
	geomFormat              string // mos, wkb, wkt, "" (postgis native)
	geomType                string
	geomSRID                uint64
	mosOpts                 mos.Options
	columns                 map[string]provider.ColumnDescriptor
	writable                map[string]string
	readOnly                []string
	domain                  string
	// A32: bounds columns (minx,maxx,miny,maxy); empty if not configured.
	bboxFields [4]string
	// A38: schema fingerprint for incarnation (hash of columns+PK).
	schemaFingerprint string
}

func (w *Writer) DescribeWritable(ctx context.Context, layer string) (provider.WriteDescriptor, error) {
	m, err := w.mapping(ctx, layer)
	if err != nil {
		return provider.WriteDescriptor{}, err
	}
	writable := make(map[string]string, len(m.writable))
	for k, v := range m.writable {
		writable[k] = v
	}
	return provider.WriteDescriptor{
		Layer:                   layer,
		Table:                   m.table,
		IDColumn:                m.idColumn,
		GeometryColumn:          m.geomColumn,
		GeometryType:            m.geomType,
		GeometrySRID:            m.geomSRID,
		WritableColumns:         writable,
		ReadOnlyColumns:         append([]string(nil), m.readOnly...),
		Domain:                  m.domain,
		CreateUnsupportedReason: m.createUnsupportedReason,
	}, nil
}

func (w *Writer) mapping(ctx context.Context, layer string) (*writeMapping, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if m, ok := w.mappings[layer]; ok {
		return m, nil
	}
	l, ok := w.provider.layers[layer]
	if !ok {
		return nil, &provider.MutationError{Kind: provider.MutationErrNotFound, Reason: fmt.Sprintf("layer %q not found", layer)}
	}
	m, err := admitLayer(ctx, w.provider, &l)
	if err != nil {
		return nil, err
	}
	w.mappings[layer] = m
	return m, nil
}

// mappingCached returns the cached mapping without pool queries (A12).
// Use inside Apply (tx holds a pool connection); fail if not admitted.
func (w *Writer) mappingCached(layer string) (*writeMapping, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if m, ok := w.mappings[layer]; ok {
		return m, nil
	}
	return nil, &provider.MutationError{Kind: provider.MutationErrExecutionFailed, Reason: fmt.Sprintf("layer %q not admitted before tx (call DescribeWritable first)", layer)}
}

// CurrentRevision implements provider.RevisionReader (A03).
func (w *Writer) CurrentRevision(ctx context.Context, layer string, featureID uint64) (string, error) {
	l, ok := w.provider.layers[layer]
	if !ok {
		return "", nil
	}
	schema, table := "public", l.tablename
	if i := strings.LastIndex(table, "."); i >= 0 {
		schema, table = table[:i], table[i+1:]
	}
	layer = quoteIdent(schema) + "." + quoteIdent(table)
	var rev, incarnation int64
	err := w.provider.pool.QueryRow(ctx, `SELECT revision, incarnation FROM tegola_revisions WHERE collection = $1 AND feature_id = $2`, layer, featureID).Scan(&rev, &incarnation)
	if err != nil {
		// R01/R09: distinguish missing row (revision 0) from missing
		// table (revisions not migrated -> "", nil for hash fallback)
		// and real storage errors.
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
	// R09: verify service schema version BEFORE opening the data tx.
	// No DDL or missing-table probes inside the tx (aborts in PostgreSQL).
	// Run migrations manually via provider/audit Migrate before write traffic.
	if err := w.checkServiceSchema(ctx); err != nil {
		return nil, &provider.MutationError{Kind: provider.MutationErrExecutionFailed, Reason: fmt.Sprintf("service schema: %v", err)}
	}
	// Explicitly request read-write: the provider defaults
	// default_transaction_read_only=TRUE for query workloads.
	tx, err := w.provider.pool.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadWrite})
	if err != nil {
		return nil, &provider.MutationError{Kind: provider.MutationErrExecutionFailed, Reason: fmt.Sprintf("begin: %v", err)}
	}
	// A01: No DDL in data transaction. Audit tables must be created via
	// migration before write traffic (see provider/audit/sql.go).
	// A34: unique txID for audit/outbox.
	txID := fmt.Sprintf("%d-%d", time.Now().UTC().UnixNano(), rand.Int63())
	return &featureTx{writer: w, tx: tx, actor: options.Actor, reqID: options.RequestID, txID: txID}, nil
}

// checkServiceSchema verifies the tegola service schema version.
// R09: explicit fail before write, not silent missing-table inside tx.
func (w *Writer) checkServiceSchema(ctx context.Context) error {
	// Use a dedicated connection for the version check.
	conn, err := w.provider.pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("acquire: %w", err)
	}
	defer conn.Release()
	var v int
	err = conn.QueryRow(ctx, `SELECT COALESCE(MAX(version),0) FROM tegola_schema_version`).Scan(&v)
	if err != nil {
		return fmt.Errorf("not migrated (run provider/audit Migrate): %w", err)
	}
	if v != pa.CurrentSchemaVersion {
		return fmt.Errorf("unsupported service schema version %d", v)
	}
	return nil
}

// splitStmts splits DDL on semicolons.
func splitStmts(ddl string) []string {
	var out []string
	for _, s := range strings.Split(ddl, ";") {
		s = strings.TrimSpace(s)
		if s != "" {
			out = append(out, s)
		}
	}
	return out
}

func deny(reason string) (*writeMapping, error) {
	return nil, &provider.MutationError{Kind: provider.MutationErrUnsupportedCapability, Reason: reason}
}

func admitLayer(ctx context.Context, p *Provider, l *Layer) (*writeMapping, error) {
	// Only tablename layers are writable; user-provided custom SQL is read-only.
	// (l.sql may contain auto-generated SQL for tablename layers; the
	// authoritative signal is the configured tablename.)
	if l.tablename == "" {
		return deny(fmt.Sprintf("layer %q uses custom SQL and is read-only", l.name))
	}
	// Table name may be schema-qualified.
	schema, table := "public", l.tablename
	if i := strings.LastIndex(table, "."); i >= 0 {
		schema, table = table[:i], table[i+1:]
	}
	q := `SELECT column_name, data_type, is_nullable, column_default, is_generated, is_identity FROM information_schema.columns WHERE table_schema = $1 AND table_name = $2 ORDER BY ordinal_position`
	rows, err := p.pool.Query(ctx, q, schema, table)
	if err != nil {
		return nil, &provider.MutationError{Kind: provider.MutationErrExecutionFailed, Reason: fmt.Sprintf("inspect: %v", err)}
	}
	defer rows.Close()
	cols := map[string]provider.ColumnDescriptor{}
	for rows.Next() {
		var name, dtype, nullable, generated, identity string
		var defaultValue sql.NullString
		if err := rows.Scan(&name, &dtype, &nullable, &defaultValue, &generated, &identity); err != nil {
			return nil, &provider.MutationError{Kind: provider.MutationErrExecutionFailed, Reason: fmt.Sprintf("inspect: %v", err)}
		}
		cols[name] = provider.ColumnDescriptor{Name: name, Type: dtype, Nullable: nullable == "YES", IsDefault: defaultValue.Valid, IsGenerated: generated == "ALWAYS" || identity == "YES"}
	}
	if err := rows.Err(); err != nil {
		return nil, &provider.MutationError{Kind: provider.MutationErrExecutionFailed, Reason: fmt.Sprintf("inspect: %v", err)}
	}
	for _, field := range l.bboxFields {
		if _, exists := cols[field]; field != "" && exists {
			return deny("writes to layers with derived bounds columns are not supported")
		}
	}
	// A08: PK via pg_constraint. Must be exactly one column; composite
	// PKs are rejected at admission (not silently truncated to first col).
	pkRows, err := p.pool.Query(ctx, `
		SELECT a.attname FROM pg_index i
		JOIN pg_attribute a ON a.attrelid = i.indrelid AND a.attnum = ANY(i.indkey)
		WHERE i.indrelid = $1::regclass AND i.indisprimary
		ORDER BY array_position(i.indkey, a.attnum)`, schema+"."+table)
	if err != nil {
		return nil, &provider.MutationError{Kind: provider.MutationErrExecutionFailed, Reason: fmt.Sprintf("inspect pk: %v", err)}
	}
	var pkCols []string
	for pkRows.Next() {
		var c string
		if err := pkRows.Scan(&c); err != nil {
			pkRows.Close()
			return nil, &provider.MutationError{Kind: provider.MutationErrExecutionFailed, Reason: fmt.Sprintf("inspect pk: %v", err)}
		}
		pkCols = append(pkCols, c)
	}
	pkRows.Close()
	if len(pkCols) == 0 {
		return deny(fmt.Sprintf("layer %q: no primary key found", l.name))
	}
	if len(pkCols) > 1 {
		return deny(fmt.Sprintf("layer %q: composite primary key %v not supported (need single-column PK)", l.name, pkCols))
	}
	pkCol := pkCols[0]
	// Integer PK check.
	var pkType string
	err = p.pool.QueryRow(ctx, `SELECT data_type FROM information_schema.columns WHERE table_schema=$1 AND table_name=$2 AND column_name=$3`,
		schema, table, pkCol).Scan(&pkType)
	if err != nil {
		return nil, &provider.MutationError{Kind: provider.MutationErrExecutionFailed, Reason: fmt.Sprintf("inspect pk: %v", err)}
	}
	pt := strings.ToLower(pkType)
	isInt := pt == "integer" || pt == "bigint" || pt == "smallint" || strings.HasPrefix(pt, "int")
	if !isInt {
		return deny(fmt.Sprintf("layer %q: primary key %q is not integer", l.name, pkCol))
	}
	geomCol := l.geomField
	if geomCol == "" {
		geomCol = "geom"
	}
	if _, ok := cols[geomCol]; !ok {
		return deny(fmt.Sprintf("layer %q: geometry column %q not found", l.name, geomCol))
	}
	idCol := l.idField
	if idCol == "" {
		idCol = "gid"
	}
	if idCol != pkCol {
		// id_fieldname should match the PK; warn via deny if mismatch.
		return deny(fmt.Sprintf("layer %q: id field %q is not the primary key %q", l.name, idCol, pkCol))
	}
	format := l.geometryFormat
	if format == "" {
		format = "postgis"
	}
	m := &writeMapping{
		schema:     schema,
		table:      table,
		idColumn:   pkCol,
		geomColumn: geomCol,
		geomFormat: format,
		geomType:   normalizeGeomType(l.geomType),
		geomSRID:   l.srid,
		// A32: bounds columns for derived bounds maintenance.
		bboxFields: [4]string{l.bboxFields[0], l.bboxFields[1], l.bboxFields[2], l.bboxFields[3]},
		mosOpts: mos.Options{
			Precision:  l.mosConfig.Precision,
			UnitFactor: l.mosConfig.UnitFactor,
		},
		columns:  cols,
		writable: make(map[string]string),
		domain:   p.domainID(),
	}
	// A default alone need not generate fresh IDs (for example DEFAULT 1).
	// Admit identity or a plain nextval expression with a catalog dependency
	// on an actual sequence; arbitrary default expressions fail closed.
	var generatedPK bool
	err = p.pool.QueryRow(ctx, `
        SELECT a.attidentity <> '' OR COALESCE(
            pg_get_expr(d.adbin, d.adrelid) ~ $3 AND EXISTS (
                SELECT 1 FROM pg_depend dep
                JOIN pg_class seq ON seq.oid = dep.refobjid AND seq.relkind = 'S'
                WHERE dep.classid = 'pg_attrdef'::regclass AND dep.objid = d.oid
                  AND dep.refclassid = 'pg_class'::regclass
            ), false)
        FROM pg_attribute a
        LEFT JOIN pg_attrdef d ON d.adrelid = a.attrelid AND d.adnum = a.attnum
        WHERE a.attrelid = $1::regclass AND a.attname = $2`,
		quoteIdent(schema)+"."+quoteIdent(table), pkCol, `^nextval\('([^']|'')+'::regclass\)$`).Scan(&generatedPK)
	if err != nil {
		return nil, &provider.MutationError{Kind: provider.MutationErrExecutionFailed, Reason: fmt.Sprintf("inspect generated primary key: %v", err)}
	}
	if !generatedPK {
		m.createUnsupportedReason = "create requires an identity or sequence-generated primary key"
	}
	public := map[string]bool{}
	if l.feature != nil {
		for _, field := range l.feature.projections {
			if field.output != field.column.name {
				return deny("aliased properties are not writable")
			}
			public[field.column.name] = true
		}
	}
	for name, column := range cols {
		if name == m.idColumn || name == m.geomColumn || (l.feature != nil && !public[name]) {
			continue
		}
		if column.IsGenerated {
			m.readOnly = append(m.readOnly, name)
			continue
		}
		m.writable[name] = name
	}
	// A38: compute schema fingerprint.
	m.schemaFingerprint = computeSchemaFingerprint(cols, pkCol)
	return m, nil
}

// computeSchemaFingerprint hashes the sorted column definitions.
func computeSchemaFingerprint(cols map[string]provider.ColumnDescriptor, pk string) string {
	names := make([]string, 0, len(cols))
	for n := range cols {
		names = append(names, n)
	}
	sort.Strings(names)
	// hash.Hash.Write never returns an error.
	h := sha256.New()
	for _, n := range names {
		_, _ = fmt.Fprintf(h, "%s:%s;", n, cols[n].Type)
	}
	_, _ = fmt.Fprintf(h, "pk=%s", pk)
	return hex.EncodeToString(h.Sum(nil))[:16]
}

func normalizeGeomType(g interface{}) string {
	if g == nil {
		return ""
	}
	s := strings.ToLower(fmt.Sprintf("%T", g))
	for _, t := range []string{"multipolygon", "multilinestring", "multipoint", "polygon", "linestring", "point"} {
		if strings.Contains(s, t) {
			return t
		}
	}
	return ""
}

func quoteIdent(s string) string {
	return `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
}

type featureTx struct {
	failed error
	writer *Writer
	tx     pgx.Tx
	actor  string
	reqID  string
	txID   string // A34
}

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
	// A12: use cached mapping; no pool queries inside tx.
	mp, err := t.writer.mappingCached(m.Collection)
	if err != nil {
		return provider.MutationOutcome{}, err
	}
	if m.Op != provider.MutationInsert {
		if err := checkRevisionCAS(ctx, t.tx, mp.revisionCollection(), m.FeatureID, m.IfRevision); err != nil {
			return provider.MutationOutcome{}, err
		}
	}
	var outcome provider.MutationOutcome
	switch m.Op {
	case provider.MutationInsert:
		outcome, err = t.insert(ctx, mp, m)
	case provider.MutationReplace:
		outcome, err = t.replace(ctx, mp, m)
	case provider.MutationUpdate:
		outcome, err = t.update(ctx, mp, m)
	case provider.MutationDelete:
		outcome, err = t.delete(ctx, mp, m)
	default:
		return provider.MutationOutcome{}, &provider.MutationError{Kind: provider.MutationErrUnsupportedCapability, Reason: "unknown op"}
	}
	if err != nil {
		return provider.MutationOutcome{}, err
	}
	// A03: revision check + bump inside the data transaction.
	bump, rerr := checkAndBumpRevision(ctx, t.tx, mp.revisionCollection(), outcome.FeatureID, m.IfRevision)
	if rerr != nil {
		return provider.MutationOutcome{}, rerr
	}
	if m.Op == provider.MutationDelete {
		if _, err := t.tx.Exec(ctx, `UPDATE tegola_revisions SET incarnation=incarnation+1,revision=0 WHERE collection=$1 AND feature_id=$2`, mp.revisionCollection(), outcome.FeatureID); err != nil {
			return provider.MutationOutcome{}, err
		}
		bump.NewInc++
		bump.New = 0
	}
	if bump.New >= 0 {
		outcome.Revision = fmt.Sprintf("%d.%d", bump.NewInc, bump.New)
		outcome.RevisionBefore = fmt.Sprintf("%d.%d", bump.OldInc, bump.Old)
	}
	// W13: audit in same transaction
	if aerr := pa.RecordPgxTx(ctx, t.tx, m.Collection, m.Op, outcome, t.actor, t.reqID, t.txID); aerr != nil {
		return provider.MutationOutcome{}, aerr
	}
	return outcome, nil
}

func (t *featureTx) Commit(ctx context.Context) (provider.CommitReceipt, error) {
	if t.failed != nil {
		_ = t.tx.Rollback(ctx)
		return provider.CommitReceipt{Status: provider.CommitNotCommitted, TransactionID: t.txID}, t.failed
	}
	if err := t.tx.Commit(ctx); err != nil {
		if errors.Is(err, pgx.ErrTxCommitRollback) {
			return provider.CommitReceipt{Status: provider.CommitNotCommitted, TransactionID: t.txID}, &provider.MutationError{Kind: provider.MutationErrExecutionFailed, Reason: "transaction was rolled back"}
		}
		return provider.CommitReceipt{Status: provider.CommitUnknown, TransactionID: t.txID}, &provider.MutationError{Kind: provider.MutationErrCommitUnknown, Reason: fmt.Sprintf("commit: %v", err)}
	}
	return provider.CommitReceipt{Status: provider.CommitCommitted, TransactionID: t.txID}, nil
}

func (t *featureTx) Rollback(ctx context.Context) error {
	_ = t.tx.Rollback(ctx)
	return nil
}

// checkAndBumpRevision implements A03 for PostGIS: the IfRevision
// precondition is validated against the revision row locked FOR UPDATE
// inside the data transaction, then the revision is bumped atomically.
// pgRevisionBump carries old and new revisions (R12).
type pgRevisionBump = pa.RevisionBump

func checkAndBumpRevision(ctx context.Context, tx pgx.Tx, collection string, featureID uint64, want string) (pgRevisionBump, error) {
	// Materialize revision zero before locking so concurrent first writes serialize.
	if _, err := tx.Exec(ctx, `INSERT INTO tegola_revisions(collection,feature_id,revision,incarnation) VALUES($1,$2,0,0) ON CONFLICT DO NOTHING`, collection, featureID); err != nil {
		return pgRevisionBump{}, err
	}
	var cur, inc int64
	if err := tx.QueryRow(ctx, `SELECT revision,incarnation FROM tegola_revisions WHERE collection=$1 AND feature_id=$2 FOR UPDATE`, collection, featureID).Scan(&cur, &inc); err != nil {
		return pgRevisionBump{}, err
	}
	if want != "" {
		wi, wr, err := pa.ParseRevision(want)
		if err != nil {
			return pgRevisionBump{}, err
		}
		if wi != inc || wr != cur {
			return pgRevisionBump{}, &provider.MutationError{Kind: provider.MutationErrPreconditionFailed, Reason: "revision mismatch"}
		}
	}
	if _, err := tx.Exec(ctx, `UPDATE tegola_revisions SET revision=revision+1 WHERE collection=$1 AND feature_id=$2`, collection, featureID); err != nil {
		return pgRevisionBump{}, err
	}
	return pgRevisionBump{Old: cur, New: cur + 1, OldInc: inc, NewInc: inc}, nil
}

func isMissingTableErr(err error) bool {
	msg := err.Error()
	return strings.Contains(msg, "does not exist") || strings.Contains(msg, "no such table")
}

func (m *writeMapping) revisionCollection() string {
	return quoteIdent(m.schema) + "." + quoteIdent(m.table)
}
