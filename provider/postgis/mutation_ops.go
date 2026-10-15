package postgis

import (
	"context"
	"fmt"
	"strings"

	"github.com/alexeydott/geom"
	"github.com/alexeydott/geom/encoding/wkb"
	"github.com/alexeydott/geom/encoding/wkt"
	"github.com/alexeydott/proj"
	"github.com/alexeydott/tegola/mos"
	"github.com/alexeydott/tegola/provider"
	pa "github.com/alexeydott/tegola/provider/audit"
	"github.com/jackc/pgx/v5"
)

// ph returns the $n placeholder for arg index i (1-based).
func ph(i int) string { return fmt.Sprintf("$%d", i) }

// A03: checkRevisionCAS verifies IfRevision against the current revision
// inside the native transaction, BEFORE the data mutation. Uses
// SELECT FOR UPDATE to lock the revision row (mirrors the MySQL helper).
// Empty ifRevision skips the check; "0" requires no existing revision row.
func checkRevisionCAS(ctx context.Context, tx pgx.Tx, collection string, featureID uint64, ifRevision string) error {
	_, err := tx.Exec(ctx, `INSERT INTO tegola_revisions(collection,feature_id,revision,incarnation)
 VALUES($1,$2,0,0) ON CONFLICT DO NOTHING`, collection, featureID)
	if err != nil {
		return err
	}
	var revision, incarnation int64
	if err = tx.QueryRow(ctx, `SELECT revision,incarnation FROM tegola_revisions
 WHERE collection=$1 AND feature_id=$2 FOR UPDATE`, collection, featureID).Scan(&revision, &incarnation); err != nil {
		return err
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

func encodeStorageGeometry(mp *writeMapping, wkbBytes []byte, inputSRID uint64) (enc []byte, encStr string, useSTGeom bool, err error) {
	g, err := wkb.DecodeBytes(wkbBytes)
	if err != nil {
		return nil, "", false, &provider.MutationError{Kind: provider.MutationErrMalformedInput, Reason: fmt.Sprintf("invalid WKB: %v", err)}
	}
	if err := checkGeometryType(g, mp.geomType); err != nil {
		return nil, "", false, err
	}
	srid := inputSRID
	if srid == 0 {
		srid = mp.geomSRID
	}
	if srid != mp.geomSRID {
		g, err = transformGeometry(g, srid, mp.geomSRID)
		if err != nil {
			return nil, "", false, err
		}
	}
	rawWKB, err := wkb.EncodeBytes(g)
	if err != nil {
		return nil, "", false, &provider.MutationError{Kind: provider.MutationErrMalformedInput, Reason: fmt.Sprintf("WKB encode: %v", err)}
	}
	switch mp.geomFormat {
	case "mos":
		blob, err := mos.Encode(g, mp.mosOpts)
		if err != nil {
			return nil, "", false, &provider.MutationError{Kind: provider.MutationErrMalformedInput, Reason: fmt.Sprintf("MOS encode: %v", err)}
		}
		return blob, "", false, nil
	case "wkb":
		return rawWKB, "", false, nil
	case "wkt":
		var sb strings.Builder
		if err := wkt.Encode(&sb, g); err != nil {
			return nil, "", false, &provider.MutationError{Kind: provider.MutationErrMalformedInput, Reason: fmt.Sprintf("WKT encode: %v", err)}
		}
		return nil, sb.String(), false, nil
	default: // "postgis" native
		return rawWKB, "", true, nil
	}
}

// A32: compute bounds from WKB for derived bounds columns.
func geometryBoundsFromWKB(wkbBytes []byte) ([4]float64, error) {
	var b [4]float64
	g, err := wkb.DecodeBytes(wkbBytes)
	if err != nil {
		return b, err
	}
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

func checkGeometryType(g geom.Geometry, want string) error {
	if want == "" {
		return nil
	}
	if normalizeGeomType(g) != want {
		return &provider.MutationError{Kind: provider.MutationErrSchemaViolation, Reason: fmt.Sprintf("geometry type mismatch: layer wants %q", want)}
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
	return nil, &provider.MutationError{Kind: provider.MutationErrUnsupportedCapability, Reason: fmt.Sprintf("CRS transform %d -> %d not supported", from, to)}
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
		return mv.Boolean, nil
	default:
		return nil, fmt.Errorf("unsupported value kind %v", mv.Kind)
	}
}

func (t *featureTx) geomArg(mp *writeMapping, m provider.Mutation, args []interface{}) ([]string, []interface{}, error) {
	enc, encStr, useST, err := encodeStorageGeometry(mp, m.GeometryWKB, m.GeometrySRID)
	if err != nil {
		return nil, nil, err
	}
	if encStr != "" {
		return []string{ph(len(args) + 1)}, append(args, encStr), nil
	}
	if useST {
		// ST_SetSRID(ST_GeomFromWKB($n), srid)
		return []string{fmt.Sprintf("ST_SetSRID(ST_GeomFromWKB(%s), %d)", ph(len(args)+1), mp.geomSRID)}, append(args, enc), nil
	}
	return []string{ph(len(args) + 1)}, append(args, enc), nil
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
		holders = append(holders, ph(len(args)+1))
		args = append(args, v)
	}
	if m.GeometryWKB != nil {
		h, a, err := t.geomArg(mp, m, args)
		if err != nil {
			return provider.MutationOutcome{}, err
		}
		cols = append(cols, quoteIdent(mp.geomColumn))
		holders = append(holders, h[0])
		args = a
		// A32: maintain derived bounds columns.
		if mp.bboxFields[0] != "" {
			b, berr := geometryBoundsFromWKB(m.GeometryWKB)
			if berr == nil {
				for i, bf := range mp.bboxFields {
					cols = append(cols, quoteIdent(bf))
					holders = append(holders, ph(len(args)+1))
					args = append(args, b[i])
				}
			}
		}
	}
	if m.GeometryWKB == nil && m.GeometryAbsent {
		cols = append(cols, quoteIdent(mp.geomColumn))
		holders = append(holders, "NULL")
		if mp.bboxFields[0] != "" {
			for _, column := range mp.bboxFields {
				cols = append(cols, quoteIdent(column))
				holders = append(holders, "NULL")
			}
		}
	}
	if len(cols) == 0 {
		return provider.MutationOutcome{}, &provider.MutationError{Kind: provider.MutationErrMalformedInput, Reason: "insert carries no properties or geometry"}
	}
	tbl := quoteIdent(mp.schema) + "." + quoteIdent(mp.table)
	q := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s) RETURNING %s", tbl, strings.Join(cols, ", "), strings.Join(holders, ", "), quoteIdent(mp.idColumn))
	var id uint64
	if err := t.tx.QueryRow(ctx, q, args...).Scan(&id); err != nil {
		return provider.MutationOutcome{}, &provider.MutationError{Kind: provider.MutationErrExecutionFailed, Reason: fmt.Sprintf("insert: %v", err)}
	}
	return provider.MutationOutcome{FeatureID: id, Affected: 1}, nil
}

// A05: Replace is implemented as UPDATE of the existing row, NOT
// DELETE+INSERT. Deleting first would fire ON DELETE CASCADE (losing child
// rows), run DELETE triggers, and discard server-managed values
// (created_at, incarnation). UPDATE preserves the row identity: PK,
// FK references, and system fields stay intact.
//
// Omitted properties are left unchanged (documented deviation from strict
// PUT "replace entire resource" semantics; nulling them without reliable
// nullability metadata would be unsafe).
func (t *featureTx) replace(ctx context.Context, mp *writeMapping, m provider.Mutation) (provider.MutationOutcome, error) {
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
		sets = append(sets, quoteIdent(col)+" = "+ph(len(args)+1))
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
		h, a, err := t.geomArg(mp, m, args)
		if err != nil {
			return provider.MutationOutcome{}, err
		}
		sets = append(sets, quoteIdent(mp.geomColumn)+" = "+h[0])
		args = a
		// A32: maintain derived bounds columns.
		if mp.bboxFields[0] != "" {
			if b, berr := geometryBoundsFromWKB(m.GeometryWKB); berr == nil {
				for i, bf := range mp.bboxFields {
					sets = append(sets, quoteIdent(bf)+" = "+ph(len(args)+1))
					args = append(args, b[i])
				}
			}
		}
	} else if m.GeometryAbsent {
		// A18: explicit geometry clear.
		sets = append(sets, quoteIdent(mp.geomColumn)+" = NULL")
		// A32: clear derived bounds too.
		if mp.bboxFields[0] != "" {
			for _, bf := range mp.bboxFields {
				sets = append(sets, quoteIdent(bf)+" = NULL")
			}
		}
	}
	if len(sets) == 0 {
		if m.GeometryUnchanged && m.IfRevision != "" {
			return provider.MutationOutcome{FeatureID: m.FeatureID, Affected: 1}, nil
		}
		return provider.MutationOutcome{}, &provider.MutationError{Kind: provider.MutationErrMalformedInput, Reason: "replace carries no changes"}
	}
	tbl := quoteIdent(mp.schema) + "." + quoteIdent(mp.table)
	q := fmt.Sprintf("UPDATE %s SET %s WHERE %s = %s", tbl, strings.Join(sets, ", "), quoteIdent(mp.idColumn), ph(len(args)+1))
	args = append(args, m.FeatureID)
	tag, err := t.tx.Exec(ctx, q, args...)
	if err != nil {
		return provider.MutationOutcome{}, &provider.MutationError{Kind: provider.MutationErrExecutionFailed, Reason: fmt.Sprintf("replace: %v", err)}
	}
	// A08: single-column PK guarantees at most 1 row; >1 is corruption.
	if n := tag.RowsAffected(); n != 1 {
		if n == 0 {
			return provider.MutationOutcome{}, &provider.MutationError{Kind: provider.MutationErrNotFound, Reason: fmt.Sprintf("feature %d not found", m.FeatureID)}
		}
		return provider.MutationOutcome{}, &provider.MutationError{Kind: provider.MutationErrExecutionFailed, Reason: fmt.Sprintf("update affected %d rows, want 1", n)}
	}

	return provider.MutationOutcome{FeatureID: m.FeatureID, Affected: 1}, nil
}

func (t *featureTx) update(ctx context.Context, mp *writeMapping, m provider.Mutation) (provider.MutationOutcome, error) {
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
		sets = append(sets, quoteIdent(col)+" = "+ph(len(args)+1))
		args = append(args, v)
	}
	if m.GeometryWKB != nil {
		h, a, err := t.geomArg(mp, m, args)
		if err != nil {
			return provider.MutationOutcome{}, err
		}
		sets = append(sets, quoteIdent(mp.geomColumn)+" = "+h[0])
		args = a
		// A32: maintain derived bounds columns.
		if mp.bboxFields[0] != "" {
			if b, berr := geometryBoundsFromWKB(m.GeometryWKB); berr == nil {
				for i, bf := range mp.bboxFields {
					sets = append(sets, quoteIdent(bf)+" = "+ph(len(args)+1))
					args = append(args, b[i])
				}
			}
		}
	} else if m.GeometryAbsent {
		// A18: explicit geometry clear.
		sets = append(sets, quoteIdent(mp.geomColumn)+" = NULL")
		// A32: clear derived bounds too.
		if mp.bboxFields[0] != "" {
			for _, bf := range mp.bboxFields {
				sets = append(sets, quoteIdent(bf)+" = NULL")
			}
		}
	}
	if len(sets) == 0 {
		return provider.MutationOutcome{}, &provider.MutationError{Kind: provider.MutationErrMalformedInput, Reason: "update carries no changes"}
	}
	tbl := quoteIdent(mp.schema) + "." + quoteIdent(mp.table)
	q := fmt.Sprintf("UPDATE %s SET %s WHERE %s = %s", tbl, strings.Join(sets, ", "), quoteIdent(mp.idColumn), ph(len(args)+1))
	args = append(args, m.FeatureID)
	tag, err := t.tx.Exec(ctx, q, args...)
	if err != nil {
		return provider.MutationOutcome{}, &provider.MutationError{Kind: provider.MutationErrExecutionFailed, Reason: fmt.Sprintf("update: %v", err)}
	}
	// A08: single-column PK guarantees at most 1 row; >1 is corruption.
	if n := tag.RowsAffected(); n != 1 {
		if n == 0 {
			return provider.MutationOutcome{}, &provider.MutationError{Kind: provider.MutationErrNotFound, Reason: fmt.Sprintf("feature %d not found", m.FeatureID)}
		}
		return provider.MutationOutcome{}, &provider.MutationError{Kind: provider.MutationErrExecutionFailed, Reason: fmt.Sprintf("update affected %d rows, want 1", n)}
	}
	return provider.MutationOutcome{FeatureID: m.FeatureID, Affected: 1}, nil
}

func (t *featureTx) delete(ctx context.Context, mp *writeMapping, m provider.Mutation) (provider.MutationOutcome, error) {
	tbl := quoteIdent(mp.schema) + "." + quoteIdent(mp.table)
	q := fmt.Sprintf("DELETE FROM %s WHERE %s = $1", tbl, quoteIdent(mp.idColumn))
	tag, err := t.tx.Exec(ctx, q, m.FeatureID)
	if err != nil {
		return provider.MutationOutcome{}, &provider.MutationError{Kind: provider.MutationErrExecutionFailed, Reason: fmt.Sprintf("delete: %v", err)}
	}
	// A08: single-column PK guarantees at most 1 row; >1 is corruption.
	if n := tag.RowsAffected(); n != 1 {
		if n == 0 {
			return provider.MutationOutcome{}, &provider.MutationError{Kind: provider.MutationErrNotFound, Reason: fmt.Sprintf("feature %d not found", m.FeatureID)}
		}
		return provider.MutationOutcome{}, &provider.MutationError{Kind: provider.MutationErrExecutionFailed, Reason: fmt.Sprintf("update affected %d rows, want 1", n)}
	}
	return provider.MutationOutcome{FeatureID: m.FeatureID, Affected: 1}, nil
}
