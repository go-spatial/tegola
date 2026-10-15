package mysql

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"

	"github.com/alexeydott/geom"
	"github.com/alexeydott/geom/encoding/wkb"
	"github.com/alexeydott/geom/encoding/wkt"
	"github.com/alexeydott/proj"
	"github.com/alexeydott/tegola/basic"
	"github.com/alexeydott/tegola/mos"
	"github.com/alexeydott/tegola/provider"
	pa "github.com/alexeydott/tegola/provider/audit"
	codec "github.com/alexeydott/tegola/provider/geometrycodec"
)

// A03: checkRevisionCAS verifies IfRevision against the current revision
// inside the transaction. Uses SELECT FOR UPDATE to lock the revision row.
func checkRevisionCAS(ctx context.Context, tx *sql.Tx, collection string, featureID uint64, ifRevision string) error {
	// Materializing zero also serializes two first edits of an existing feature.
	_, err := tx.ExecContext(ctx, `INSERT INTO tegola_revisions(collection,feature_id,revision,incarnation)
 VALUES(?,?,0,0) ON DUPLICATE KEY UPDATE feature_id=VALUES(feature_id)`, collection, featureID)
	if err != nil {
		return mapSQLError(err)
	}
	var revision, incarnation int64
	err = tx.QueryRowContext(ctx, `SELECT revision,incarnation FROM tegola_revisions
 WHERE collection=? AND feature_id=? FOR UPDATE`, collection, featureID).Scan(&revision, &incarnation)
	if err != nil {
		return mapSQLError(err)
	}
	if ifRevision == "" {
		return nil
	}
	wantInc, wantRev, err := pa.ParseRevision(ifRevision)
	if err != nil {
		return err
	}
	if wantInc != incarnation || wantRev != revision {
		return &provider.MutationError{Kind: provider.MutationErrPreconditionFailed, Reason: "revision mismatch"}
	}
	return nil
}

type featureTx struct {
	failed error
	writer *Writer
	tx     *sql.Tx
	actor  string
	reqID  string
	txID   string // A34: unique transaction ID for audit/outbox
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
	mp, err := t.writer.mapping(ctx, m.Collection)
	if err != nil {
		return provider.MutationOutcome{}, err
	}
	if m.Op != provider.MutationInsert {
		if err := checkRevisionCAS(ctx, t.tx, mp.revisionCollection(), m.FeatureID, m.IfRevision); err != nil {
			return provider.MutationOutcome{}, err
		}
		if mp.geomFormat == "mos" {
			if err := t.checkStoredMOSMutation(ctx, mp, m); err != nil {
				return provider.MutationOutcome{}, err
			}
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
		return provider.MutationOutcome{}, &provider.MutationError{Kind: provider.MutationErrUnsupportedCapability, Reason: fmt.Sprintf("unknown op %v", m.Op)}
	}
	if err != nil {
		return provider.MutationOutcome{}, err
	}
	// A03: revision check + bump inside the data transaction.
	bump, rerr := pa.CheckAndBumpRevisionSQL(ctx, t.tx, mp.revisionCollection(), outcome.FeatureID, m.IfRevision, "mysql")
	if rerr != nil {
		return provider.MutationOutcome{}, rerr
	}
	if m.Op == provider.MutationDelete {
		if err := pa.BumpIncarnationOnDelete(ctx, t.tx, mp.revisionCollection(), outcome.FeatureID, "mysql"); err != nil {
			return provider.MutationOutcome{}, err
		}
		bump.NewInc++
		bump.New = 0
	}
	if bump.New >= 0 {
		// A38: revision format is "incarnation.revision".
		outcome.Revision = strconv.FormatInt(bump.NewInc, 10) + "." + strconv.FormatInt(bump.New, 10)
		outcome.RevisionBefore = strconv.FormatInt(bump.OldInc, 10) + "." + strconv.FormatInt(bump.Old, 10)
	}
	// W13: audit in same transaction
	if aerr := t.recordAudit(ctx, m, outcome); aerr != nil {
		return provider.MutationOutcome{}, aerr
	}
	return outcome, nil
}

// recordAudit writes W13 audit/outbox entries in the transaction.
func (t *featureTx) recordAudit(ctx context.Context, m provider.Mutation, outcome provider.MutationOutcome) error {
	return pa.RecordTx(ctx, t.tx, m.Collection, m.Op, outcome, t.actor, t.reqID, t.txID)
}

func (t *featureTx) Commit(ctx context.Context) (provider.CommitReceipt, error) {
	if t.failed != nil {
		_ = t.tx.Rollback()
		return provider.CommitReceipt{Status: provider.CommitNotCommitted, TransactionID: t.txID}, t.failed
	}
	if err := t.tx.Commit(); err != nil {
		return provider.CommitReceipt{Status: provider.CommitUnknown, TransactionID: t.txID}, &provider.MutationError{Kind: provider.MutationErrCommitUnknown, Reason: fmt.Sprintf("commit: %v", err)}
	}
	return provider.CommitReceipt{Status: provider.CommitCommitted, TransactionID: t.txID}, nil
}

func (t *featureTx) Rollback(ctx context.Context) error {
	_ = t.tx.Rollback()
	return nil
}

// encodeStorageGeometry validates the input WKB and encodes to the layer's
// storage format: mos blob, wkb bytes, wkt text, or MySQL/MariaDB native.
// GeometryAssignment is a typed geometry value for SQL construction (R03).
// It carries either a bind parameter (safe) or a SQL expression template
// with bind args (for native functions). Never a raw interpolated string.
type GeometryAssignment struct {
	// RawBounds is the encoded MOS envelope in minX,maxX,minY,maxY order.
	RawBounds *[4]int32
	// BindValue is used when the geometry is a simple bind parameter.
	BindValue interface{}
	// ExprTemplate is a SQL fragment like "ST_GeomFromText(?, ?)".
	ExprTemplate string
	// ExprArgs are bound to the template placeholders.
	ExprArgs []interface{}
}

// IsExpr reports whether this is a SQL expression (vs plain bind).
func (ga GeometryAssignment) IsExpr() bool {
	return ga.ExprTemplate != ""
}

func encodeStorageGeometry(mp *writeMapping, wkbBytes []byte, inputSRID uint64) (GeometryAssignment, error) {
	g, err := wkb.DecodeBytes(wkbBytes)
	if err != nil {
		return GeometryAssignment{}, &provider.MutationError{Kind: provider.MutationErrMalformedInput, Reason: fmt.Sprintf("invalid WKB: %v", err)}
	}
	if err := checkGeometryType(g, mp.geomType); err != nil {
		return GeometryAssignment{}, err
	}
	if mp.geomFormat == "mos" {
		transformed, err := codec.TransformStorageGeometry(g, inputSRID, mp.geomSRID, mp.storageProjection)
		if err != nil {
			return GeometryAssignment{}, &provider.MutationError{Kind: provider.MutationErrUnsupportedCapability, Reason: err.Error()}
		}
		encoded, err := codec.EncodeMOSStorage(transformed, mp.mosOpts)
		if err != nil {
			return GeometryAssignment{}, &provider.MutationError{Kind: provider.MutationErrMalformedInput, Reason: err.Error()}
		}
		return GeometryAssignment{BindValue: encoded.Blob, RawBounds: &encoded.RawBounds}, nil
	}
	srid := inputSRID
	if srid == 0 {
		srid = mp.geomSRID
	}
	if srid != mp.geomSRID {
		g, err = transformGeometry(g, srid, mp.geomSRID)
		if err != nil {
			return GeometryAssignment{}, err
		}
	}
	rawWKB, err := wkb.EncodeBytes(g)
	if err != nil {
		return GeometryAssignment{}, &provider.MutationError{Kind: provider.MutationErrMalformedInput, Reason: fmt.Sprintf("WKB encode: %v", err)}
	}
	switch mp.geomFormat {
	case "wkb":
		return GeometryAssignment{BindValue: rawWKB}, nil
	case "wkt":
		var sb strings.Builder
		if err := wkt.Encode(&sb, g); err != nil {
			return GeometryAssignment{}, &provider.MutationError{Kind: provider.MutationErrMalformedInput, Reason: fmt.Sprintf("WKT encode: %v", err)}
		}
		return GeometryAssignment{BindValue: sb.String()}, nil
	case "mariadb", "mysql", "auto":
		// Native server-side construction via ST_GeomFromText: the Go
		// driver binds []byte as MYSQL_TYPE_STRING, which the GEOMETRY
		// column rejects (Error 1416). WKT text avoids the issue entirely
		// and lets the server assign the SRID.
		var sb strings.Builder
		if err := wkt.Encode(&sb, g); err != nil {
			return GeometryAssignment{}, &provider.MutationError{Kind: provider.MutationErrMalformedInput, Reason: fmt.Sprintf("WKT encode: %v", err)}
		}
		// R03: parameterized, no Go quoting (ANSI_QUOTES safe).
		return GeometryAssignment{
			ExprTemplate: nativeGeometryConstructor(mp),
			ExprArgs:     []interface{}{sb.String(), mp.geomSRID},
		}, nil
	default:
		return GeometryAssignment{}, &provider.MutationError{Kind: provider.MutationErrUnsupportedCapability, Reason: fmt.Sprintf("unsupported geometry format %q", mp.geomFormat)}
	}
}

func nativeGeometryConstructor(mp *writeMapping) string {
	if mp.geomFormat == "mysql" {
		return "ST_GeomFromText(?, ?, 'axis-order=long-lat')"
	}
	return "ST_GeomFromText(?, ?)"
}

func checkGeometryType(g geom.Geometry, want string) error {
	if want == "" {
		return nil
	}
	got := normalizeGeomType(g)
	if got != want {
		// Allow multi/single mismatch only if exact? Strict: reject.
		return &provider.MutationError{Kind: provider.MutationErrSchemaViolation, Reason: fmt.Sprintf("geometry type %q does not match layer type %q", got, want)}
	}
	return nil
}

func transformGeometry(g geom.Geometry, from, to uint64) (geom.Geometry, error) {
	if from == to {
		return g, nil
	}
	if (from == 4326 && to == 3857) || (from == 3857 && to == 4326) {
		return transform4326_3857(g, from == 4326)
	}
	// Custom proj4 CRS (e.g. etmerc) registered under a synthetic SRID:
	// use the vendored proj engine directly.
	if from == 4326 && basic.IsSyntheticSRID(to) {
		return transformViaProj(g, proj.EPSGCode(to), true)
	}
	if to == 4326 && basic.IsSyntheticSRID(from) {
		return transformViaProj(g, proj.EPSGCode(from), false)
	}
	return nil, &provider.MutationError{Kind: provider.MutationErrUnsupportedCapability, Reason: fmt.Sprintf("CRS transform %d -> %d not supported", from, to)}
}

// transformViaProj converts between 4326 and a custom proj4 CRS registered
// under a synthetic SRID. forward=true: 4326 -> custom; false: custom -> 4326.
func transformViaProj(g geom.Geometry, code proj.EPSGCode, forward bool) (geom.Geometry, error) {
	xform := func(x, y float64) ([2]float64, error) {
		var out []float64
		var err error
		if forward {
			out, err = proj.Convert(code, []float64{x, y})
		} else {
			out, err = proj.Inverse(code, []float64{x, y})
		}
		if err != nil {
			return [2]float64{}, err
		}
		return [2]float64{out[0], out[1]}, nil
	}
	return mapGeometryPoints(g, xform)
}

func transform4326_3857(g geom.Geometry, forward bool) (geom.Geometry, error) {
	xform := func(x, y float64) ([2]float64, error) {
		var out []float64
		var err error
		if forward {
			out, err = proj.Convert(proj.EPSG3857, []float64{x, y})
		} else {
			out, err = proj.Inverse(proj.EPSG3857, []float64{x, y})
		}
		if err != nil {
			return [2]float64{}, err
		}
		return [2]float64{out[0], out[1]}, nil
	}
	mapPts := func(pts [][2]float64) ([][2]float64, error) {
		out := make([][2]float64, len(pts))
		for i, p := range pts {
			q, err := xform(p[0], p[1])
			if err != nil {
				return nil, err
			}
			out[i] = q
		}
		return out, nil
	}
	switch t := g.(type) {
	case geom.Point:
		q, err := xform(t[0], t[1])
		return geom.Point(q), err
	case geom.LineString:
		pts, err := mapPts(t)
		return geom.LineString(pts), err
	case geom.Polygon:
		out := make(geom.Polygon, len(t))
		for i, r := range t {
			pts, err := mapPts(r)
			if err != nil {
				return nil, err
			}
			out[i] = pts
		}
		return out, nil
	case geom.MultiPoint:
		pts, err := mapPts([][2]float64(t))
		return geom.MultiPoint(pts), err
	case geom.MultiLineString:
		out := make(geom.MultiLineString, len(t))
		for i, l := range t {
			pts, err := mapPts(l)
			if err != nil {
				return nil, err
			}
			out[i] = pts
		}
		return out, nil
	case geom.MultiPolygon:
		out := make(geom.MultiPolygon, len(t))
		for i, p := range t {
			pp := make(geom.Polygon, len(p))
			for j, r := range p {
				pts, err := mapPts(r)
				if err != nil {
					return nil, err
				}
				pp[j] = pts
			}
			out[i] = pp
		}
		return out, nil
	default:
		return nil, &provider.MutationError{Kind: provider.MutationErrUnsupportedCapability, Reason: fmt.Sprintf("cannot reproject %T", g)}
	}
}

func valueToSQL(mv provider.MutationValue) (interface{}, error) {
	if mv.Null {
		return nil, nil
	}
	switch mv.Kind {
	case provider.MutationValueInteger:
		return mv.Integer, nil
	case provider.MutationValueDecimal:
		return mv.Decimal, nil
	case provider.MutationValueString:
		return mv.String, nil
	case provider.MutationValueBoolean:
		if mv.Boolean {
			return 1, nil
		}
		return 0, nil
	default:
		return nil, fmt.Errorf("unsupported value kind %v", mv.Kind)
	}
}

func mapSQLError(err error) error {
	msg := err.Error()
	if strings.Contains(msg, "Duplicate entry") {
		return &provider.MutationError{Kind: provider.MutationErrPreconditionFailed, Reason: msg}
	}
	return &provider.MutationError{Kind: provider.MutationErrExecutionFailed, Reason: msg}
}

func (t *featureTx) insert(ctx context.Context, mp *writeMapping, m provider.Mutation) (provider.MutationOutcome, error) {
	if mp.createUnsupportedReason != "" {
		return provider.MutationOutcome{}, &provider.MutationError{Kind: provider.MutationErrUnsupportedCapability, Reason: mp.createUnsupportedReason}
	}

	var cols, holders []string
	var args []interface{}
	for pub, mv := range m.Properties {
		col, ok := mp.writable[pub]
		if !ok {
			return provider.MutationOutcome{}, &provider.MutationError{Kind: provider.MutationErrSchemaViolation, Reason: fmt.Sprintf("property %q is not writable", pub)}
		}
		v, err := valueToSQL(mv)
		if err != nil {
			return provider.MutationOutcome{}, &provider.MutationError{Kind: provider.MutationErrMalformedInput, Reason: err.Error()}
		}
		cols = append(cols, quoteIdent(col))
		holders = append(holders, "?")
		args = append(args, v)
	}
	if m.GeometryWKB != nil {
		ga, err := encodeStorageGeometry(mp, m.GeometryWKB, m.GeometrySRID)
		if err != nil {
			return provider.MutationOutcome{}, err
		}
		cols = append(cols, quoteIdent(mp.geomColumn))
		// R03: typed assignment - expression template or bind value.
		if ga.IsExpr() {
			holders = append(holders, ga.ExprTemplate)
			args = append(args, ga.ExprArgs...)
		} else {
			holders = append(holders, "?")
			args = append(args, ga.BindValue)
		}
		values, err := mosBoundsValues(mp, ga.RawBounds)
		if err != nil {
			return provider.MutationOutcome{}, err
		}
		for i, value := range values {
			cols = append(cols, quoteIdent(mp.bboxFields[i]))
			holders = append(holders, "?")
			args = append(args, value)
		}
	} else {
		if mp.bboxFields[0] != "" || m.GeometryAbsent {
			cols = append(cols, quoteIdent(mp.geomColumn))
			holders = append(holders, "NULL")
		}
		values, err := mosBoundsValues(mp, nil)
		if err != nil {
			return provider.MutationOutcome{}, err
		}
		for i, value := range values {
			cols = append(cols, quoteIdent(mp.bboxFields[i]))
			holders = append(holders, "?")
			args = append(args, value)
		}
	}
	if len(cols) == 0 {
		return provider.MutationOutcome{}, &provider.MutationError{Kind: provider.MutationErrMalformedInput, Reason: "insert carries no properties or geometry"}
	}
	q := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)", quoteIdent(mp.table), strings.Join(cols, ", "), strings.Join(holders, ", "))
	res, err := t.tx.ExecContext(ctx, q, args...)
	if err != nil {
		return provider.MutationOutcome{}, mapSQLError(err)
	}
	id, err := res.LastInsertId()
	if err != nil || id <= 0 {
		return provider.MutationOutcome{}, &provider.MutationError{Kind: provider.MutationErrExecutionFailed, Reason: "insert did not return a row id"}
	}
	return provider.MutationOutcome{FeatureID: uint64(id), Affected: 1}, nil
}

// A05: Replace is UPDATE of the existing row, NOT DELETE+INSERT.
// Deleting first would fire ON DELETE CASCADE, run DELETE triggers, and
// discard server-managed values. UPDATE preserves row identity.
// Omitted writable properties use NULL or their database defaults. Internal
// bounds are assigned only alongside a new or explicitly cleared geometry.
func (t *featureTx) replace(ctx context.Context, mp *writeMapping, m provider.Mutation) (provider.MutationOutcome, error) {
	// R04: verify existence before REPLACE. RowsAffected==0 after
	// a verified existence means no-op (identical values), not 404.
	exists, err := t.existsInTx(ctx, mp, m.FeatureID)
	if err != nil {
		return provider.MutationOutcome{}, err
	}
	if !exists {
		return provider.MutationOutcome{}, &provider.MutationError{Kind: provider.MutationErrNotFound, Reason: fmt.Sprintf("feature %d not found", m.FeatureID)}
	}
	var sets []string
	var args []interface{}
	for pub, mv := range m.Properties {
		col, ok := mp.writable[pub]
		if !ok {
			return provider.MutationOutcome{}, &provider.MutationError{Kind: provider.MutationErrSchemaViolation, Reason: fmt.Sprintf("property %q is not writable", pub)}
		}
		v, err := valueToSQL(mv)
		if err != nil {
			return provider.MutationOutcome{}, &provider.MutationError{Kind: provider.MutationErrMalformedInput, Reason: err.Error()}
		}
		sets = append(sets, quoteIdent(col)+" = ?")
		args = append(args, v)
	}
	for pub, col := range mp.writable {
		if _, ok := m.Properties[pub]; ok {
			continue
		}
		value := "NULL"
		if mp.columns[col].IsDefault {
			value = "DEFAULT"
		}
		sets = append(sets, quoteIdent(col)+" = "+value)
	}

	if m.GeometryWKB != nil {
		ga, err := encodeStorageGeometry(mp, m.GeometryWKB, m.GeometrySRID)
		if err != nil {
			return provider.MutationOutcome{}, err
		}
		// R03: typed assignment - expression template or bind value.
		if ga.IsExpr() {
			sets = append(sets, quoteIdent(mp.geomColumn)+" = "+ga.ExprTemplate)
			args = append(args, ga.ExprArgs...)
		} else {
			sets = append(sets, quoteIdent(mp.geomColumn)+" = ?")
			args = append(args, ga.BindValue)
		}
		values, err := mosBoundsValues(mp, ga.RawBounds)
		if err != nil {
			return provider.MutationOutcome{}, err
		}
		for i, value := range values {
			sets = append(sets, quoteIdent(mp.bboxFields[i])+" = ?")
			args = append(args, value)
		}
	} else if m.GeometryAbsent {
		// A18: explicit geometry clear -> SET NULL.
		sets = append(sets, quoteIdent(mp.geomColumn)+" = NULL")
		values, err := mosBoundsValues(mp, nil)
		if err != nil {
			return provider.MutationOutcome{}, err
		}
		for i, value := range values {
			sets = append(sets, quoteIdent(mp.bboxFields[i])+" = ?")
			args = append(args, value)
		}
	}
	if len(sets) == 0 {
		if m.GeometryUnchanged && m.IfRevision != "" {
			return provider.MutationOutcome{FeatureID: m.FeatureID, Affected: 1}, nil
		}
		return provider.MutationOutcome{}, &provider.MutationError{Kind: provider.MutationErrMalformedInput, Reason: "replace carries no changes"}
	}
	args = append(args, m.FeatureID)
	q := fmt.Sprintf("UPDATE %s SET %s WHERE %s = ?", quoteIdent(mp.table), strings.Join(sets, ", "), quoteIdent(mp.idColumn))
	res, err := t.tx.ExecContext(ctx, q, args...)
	if err != nil {
		return provider.MutationOutcome{}, mapSQLError(err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return provider.MutationOutcome{}, mapSQLError(err)
	}
	if n > 1 {
		return provider.MutationOutcome{}, &provider.MutationError{Kind: provider.MutationErrExecutionFailed, Reason: "replace affected multiple rows"}
	}
	return provider.MutationOutcome{FeatureID: m.FeatureID, Affected: 1}, nil
}

func (t *featureTx) update(ctx context.Context, mp *writeMapping, m provider.Mutation) (provider.MutationOutcome, error) {
	// R04: verify existence before UPDATE to distinguish not-found
	// from no-op (RowsAffected==0 ambiguous with default flags).
	exists, err := t.existsInTx(ctx, mp, m.FeatureID)
	if err != nil {
		return provider.MutationOutcome{}, err
	}
	if !exists {
		return provider.MutationOutcome{}, &provider.MutationError{Kind: provider.MutationErrNotFound, Reason: fmt.Sprintf("feature %d not found", m.FeatureID)}
	}
	var sets []string
	var args []interface{}
	for pub, mv := range m.Properties {
		col, ok := mp.writable[pub]
		if !ok {
			return provider.MutationOutcome{}, &provider.MutationError{Kind: provider.MutationErrSchemaViolation, Reason: fmt.Sprintf("property %q is not writable", pub)}
		}
		v, err := valueToSQL(mv)
		if err != nil {
			return provider.MutationOutcome{}, &provider.MutationError{Kind: provider.MutationErrMalformedInput, Reason: err.Error()}
		}
		sets = append(sets, quoteIdent(col)+" = ?")
		args = append(args, v)
	}
	if m.GeometryWKB != nil {
		ga, err := encodeStorageGeometry(mp, m.GeometryWKB, m.GeometrySRID)
		if err != nil {
			return provider.MutationOutcome{}, err
		}
		// R03: typed assignment - expression template or bind value.
		if ga.IsExpr() {
			sets = append(sets, quoteIdent(mp.geomColumn)+" = "+ga.ExprTemplate)
			args = append(args, ga.ExprArgs...)
		} else {
			sets = append(sets, quoteIdent(mp.geomColumn)+" = ?")
			args = append(args, ga.BindValue)
		}
		values, err := mosBoundsValues(mp, ga.RawBounds)
		if err != nil {
			return provider.MutationOutcome{}, err
		}
		for i, value := range values {
			sets = append(sets, quoteIdent(mp.bboxFields[i])+" = ?")
			args = append(args, value)
		}
	} else if m.GeometryAbsent {
		// A18: explicit geometry clear -> SET NULL.
		sets = append(sets, quoteIdent(mp.geomColumn)+" = NULL")
		values, err := mosBoundsValues(mp, nil)
		if err != nil {
			return provider.MutationOutcome{}, err
		}
		for i, value := range values {
			sets = append(sets, quoteIdent(mp.bboxFields[i])+" = ?")
			args = append(args, value)
		}
	}
	if len(sets) == 0 {
		return provider.MutationOutcome{}, &provider.MutationError{Kind: provider.MutationErrMalformedInput, Reason: "update carries no changes"}
	}
	args = append(args, m.FeatureID)
	q := fmt.Sprintf("UPDATE %s SET %s WHERE %s = ?", quoteIdent(mp.table), strings.Join(sets, ", "), quoteIdent(mp.idColumn))
	res, err := t.tx.ExecContext(ctx, q, args...)
	if err != nil {
		return provider.MutationOutcome{}, mapSQLError(err)
	}
	// R04: existence was verified before UPDATE; RowsAffected==0 means
	// no-op (identical values), not not-found. Return success.
	n, _ := res.RowsAffected()
	return provider.MutationOutcome{FeatureID: m.FeatureID, Affected: int(n)}, nil
}

// existsInTx checks feature existence within the tx (R04).
// Distinguishes not-found from no-op UPDATE (RowsAffected==0 ambiguous).
func (t *featureTx) existsInTx(ctx context.Context, mp *writeMapping, featureID uint64) (bool, error) {
	var one int
	q := fmt.Sprintf("SELECT 1 FROM %s WHERE %s = ? LIMIT 1 FOR UPDATE", quoteIdent(mp.table), quoteIdent(mp.idColumn))
	err := t.tx.QueryRowContext(ctx, q, featureID).Scan(&one)
	if err != nil {
		if err == sql.ErrNoRows {
			return false, nil
		}
		return false, mapSQLError(err)
	}
	return true, nil
}

func (t *featureTx) delete(ctx context.Context, mp *writeMapping, m provider.Mutation) (provider.MutationOutcome, error) {
	q := fmt.Sprintf("DELETE FROM %s WHERE %s = ?", quoteIdent(mp.table), quoteIdent(mp.idColumn))
	res, err := t.tx.ExecContext(ctx, q, m.FeatureID)
	if err != nil {
		return provider.MutationOutcome{}, mapSQLError(err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return provider.MutationOutcome{}, &provider.MutationError{Kind: provider.MutationErrNotFound, Reason: fmt.Sprintf("feature %d not found", m.FeatureID)}
	}
	return provider.MutationOutcome{FeatureID: m.FeatureID, Affected: int(n)}, nil
}

// mapGeometryPoints applies xform to every vertex of g, preserving structure.
func mapGeometryPoints(g geom.Geometry, xform func(x, y float64) ([2]float64, error)) (geom.Geometry, error) {
	mapPts := func(pts [][2]float64) ([][2]float64, error) {
		out := make([][2]float64, len(pts))
		for i, p := range pts {
			q, err := xform(p[0], p[1])
			if err != nil {
				return nil, err
			}
			out[i] = q
		}
		return out, nil
	}
	switch t := g.(type) {
	case geom.Point:
		q, err := xform(t[0], t[1])
		if err != nil {
			return nil, err
		}
		return geom.Point(q), nil
	case geom.MultiPoint:
		pts, err := mapPts([][2]float64(t))
		if err != nil {
			return nil, err
		}
		return geom.MultiPoint(pts), nil
	case geom.LineString:
		pts, err := mapPts([][2]float64(t))
		if err != nil {
			return nil, err
		}
		return geom.LineString(pts), nil
	case geom.MultiLineString:
		out := make(geom.MultiLineString, len(t))
		for i, ls := range t {
			pts, err := mapPts(ls)
			if err != nil {
				return nil, err
			}
			out[i] = pts
		}
		return out, nil
	case geom.Polygon:
		rings := t.LinearRings()
		out := make(geom.Polygon, len(rings))
		for i, lr := range rings {
			pts, err := mapPts(lr)
			if err != nil {
				return nil, err
			}
			out[i] = pts
		}
		return out, nil
	case geom.MultiPolygon:
		out := make(geom.MultiPolygon, len(t))
		for i, poly := range t {
			rings := geom.Polygon(poly).LinearRings()
			p2 := make(geom.Polygon, len(rings))
			for j, lr := range rings {
				pts, err := mapPts(lr)
				if err != nil {
					return nil, err
				}
				p2[j] = pts
			}
			out[i] = p2
		}
		return out, nil
	}
	return nil, &provider.MutationError{Kind: provider.MutationErrUnsupportedCapability, Reason: fmt.Sprintf("cannot reproject %T", g)}
}

// mosBoundsValues supplies all derived columns in the same DML as geometry.
func mosBoundsValues(mp *writeMapping, bounds *[4]int32) ([]interface{}, error) {
	if mp.bboxFields[0] == "" {
		return nil, nil
	}
	values := make([]interface{}, 4)
	for i, name := range mp.bboxFields {
		if bounds != nil {
			values[i] = int64(bounds[i])
			continue
		}
		if !mp.columns[name].Nullable {
			return nil, &provider.MutationError{Kind: provider.MutationErrSchemaViolation, Reason: "geometry clearing requires nullable bounds columns"}
		}
	}
	return values, nil
}

// SystemInfo records are never features. Canonical object metadata also requires
// preserving geometry family and refusing opaque MOS payloads on geometry edits.
func (t *featureTx) checkStoredMOSMutation(ctx context.Context, mp *writeMapping, m provider.Mutation) error {
	var raw []byte
	err := t.tx.QueryRowContext(ctx, "SELECT "+quoteIdent(mp.geomColumn)+" FROM "+quoteIdent(mp.table)+" WHERE "+quoteIdent(mp.idColumn)+"=? FOR UPDATE", m.FeatureID).Scan(&raw)
	if err == sql.ErrNoRows {
		return &provider.MutationError{Kind: provider.MutationErrNotFound, Reason: "feature not found"}
	}
	if err != nil {
		return mapSQLError(err)
	}
	if codec.IsSystemInfoValue(raw) {
		return &provider.MutationError{Kind: provider.MutationErrNotFound, Reason: "feature not found"}
	}
	if !mp.canonicalMapplGIS || m.GeometryWKB == nil {
		return nil
	}
	if len(raw) == 0 || raw[0] > mos.TypePoint {
		return &provider.MutationError{Kind: provider.MutationErrUnsupportedCapability, Reason: "canonical geometry has unsupported MOS metadata"}
	}
	stored, err := mos.Decode(raw, mp.mosOpts)
	if err != nil {
		return err
	}
	canonical, err := mos.Encode(stored, mp.mosOpts)
	if err != nil {
		return err
	}
	if !bytes.Equal(raw, canonical) {
		return &provider.MutationError{Kind: provider.MutationErrUnsupportedCapability, Reason: "canonical geometry has opaque MOS metadata that cannot be preserved"}
	}
	proposed, err := wkb.DecodeBytes(m.GeometryWKB)
	if err != nil {
		return err
	}
	return checkGeometryType(proposed, normalizeGeomType(stored))
}
