//go:build cgo

package gpkg

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/alexeydott/geom"
	"github.com/alexeydott/geom/encoding/wkb"
	"github.com/alexeydott/geom/encoding/wkt"
	"github.com/alexeydott/proj"
	"github.com/alexeydott/tegola/mos"
	"github.com/alexeydott/tegola/provider"
	pa "github.com/alexeydott/tegola/provider/audit"
	codec "github.com/alexeydott/tegola/provider/geometrycodec"
)

// valueToSQL converts a neutral MutationValue to a driver value using the
// storage column type. Placeholders carry values; identifiers come only
// from the admission whitelist.
func valueToSQL(mv provider.MutationValue, colType string) (interface{}, error) {
	if mv.Null {
		return nil, nil
	}
	upper := strings.ToUpper(strings.TrimSpace(colType))
	switch mv.Kind {
	case provider.MutationValueInteger:
		return mv.Integer, nil
	case provider.MutationValueDecimal:
		// REAL columns take float64; NUMERIC/DECIMAL keep the text so no
		// precision is lost through float64.
		if strings.Contains(upper, "REAL") || strings.Contains(upper, "FLOAT") || strings.Contains(upper, "DOUBLE") {
			var f float64
			if _, err := fmt.Sscanf(mv.Decimal, "%g", &f); err != nil {
				return nil, fmt.Errorf("invalid decimal %q", mv.Decimal)
			}
			return f, nil
		}
		return mv.Decimal, nil
	case provider.MutationValueString:
		if mv.Empty {
			return "", nil
		}
		return mv.String, nil
	case provider.MutationValueBoolean:
		if mv.Boolean {
			return int64(1), nil
		}
		return int64(0), nil
	default:
		return nil, fmt.Errorf("unsupported value kind")
	}
}

// encodeStorageGeometry validates the input WKB against the admitted
// geometry type, transforms CRS when needed, and encodes to the layer's
// storage format (gpkg binary / wkb / wkt). Attribute-only updates never
// call this, so stored geometry bytes are preserved then.
func encodeStorageGeometry(mp *writeMapping, wkbBytes []byte, inputSRID uint64) ([]byte, string, [4]float64, error) {
	var noBounds [4]float64
	g, err := wkb.DecodeBytes(wkbBytes)
	if err != nil {
		return nil, "", noBounds, &provider.MutationError{Kind: provider.MutationErrMalformedInput, Reason: fmt.Sprintf("invalid WKB geometry: %v", err)}
	}
	if err := checkGeometryType(g, mp.geomType); err != nil {
		return nil, "", noBounds, err
	}
	if mp.layer != nil && mp.layer.featureCRSProjection != nil {
		g, err = codec.TransformStorageGeometry(g, inputSRID, mp.geomSRID, mp.layer.featureCRSProjection)
	} else {
		srid := inputSRID
		if srid == 0 {
			srid = mp.geomSRID
		}
		if srid != mp.geomSRID {
			g, err = transformGeometry(g, srid, mp.geomSRID)
		}
	}
	if err != nil {
		return nil, "", noBounds, err
	}
	bounds, err := geometryBounds(g)
	if err != nil {
		return nil, "", noBounds, &provider.MutationError{Kind: provider.MutationErrMalformedInput, Reason: fmt.Sprintf("geometry bounds: %v", err)}
	}
	rawWKB, err := wkb.EncodeBytes(g)
	if err != nil {
		return nil, "", noBounds, &provider.MutationError{Kind: provider.MutationErrMalformedInput, Reason: fmt.Sprintf("WKB encode: %v", err)}
	}
	switch mp.geomFormat {
	case "gpkg", "":
		envelope := geometryEnvelope(g)
		return encodeGPKGGeometry(rawWKB, int32(mp.geomSRID), envelope), "", bounds, nil
	case "wkb":
		return rawWKB, "", bounds, nil
	case "wkt":
		var sb strings.Builder
		if err := wkt.Encode(&sb, g); err != nil {
			return nil, "", noBounds, &provider.MutationError{Kind: provider.MutationErrMalformedInput, Reason: fmt.Sprintf("WKT encode: %v", err)}
		}
		stored, err := wkt.DecodeString(sb.String())
		if err != nil {
			return nil, "", noBounds, fmt.Errorf("decode stored WKT: %w", err)
		}
		bounds, err = geometryBounds(stored)
		if err != nil {
			return nil, "", noBounds, fmt.Errorf("stored WKT bounds: %w", err)
		}
		return nil, sb.String(), bounds, nil
	case "mos":
		stored, err := codec.EncodeMOSStorage(g, mp.mosOpts)
		if err != nil {
			return nil, "", noBounds, &provider.MutationError{Kind: provider.MutationErrMalformedInput, Reason: fmt.Sprintf("MOS encode: %v", err)}
		}
		bounds, err = geometryBounds(stored.Geometry)
		if err != nil {
			return nil, "", noBounds, err
		}
		return stored.Blob, "", bounds, nil
	default:
		return nil, "", noBounds, &provider.MutationError{Kind: provider.MutationErrUnsupportedCapability, Reason: "geometry format " + mp.geomFormat}
	}
}

// nativeMOSBounds indexes the encoded ticks rather than pre-quantized coordinates.
func nativeMOSBounds(blob []byte) ([4]float64, error) {
	g, err := mos.Decode(blob, mos.Options{Precision: 0, UnitFactor: 1})
	if err != nil {
		return [4]float64{}, err
	}
	return geometryBounds(g)
}

func checkGeometryType(g geom.Geometry, admitted string) error {
	var got string
	switch g.(type) {
	case geom.Point:
		got = "point"
	case geom.LineString:
		got = "linestring"
	case geom.Polygon:
		got = "polygon"
	case geom.MultiPoint:
		got = "multipoint"
	case geom.MultiLineString:
		got = "multilinestring"
	case geom.MultiPolygon:
		got = "multipolygon"
	default:
		return &provider.MutationError{Kind: provider.MutationErrMalformedInput, Reason: fmt.Sprintf("unsupported geometry type %T", g)}
	}
	if admitted != "" && admitted != "geometry" && got != admitted {
		return &provider.MutationError{
			Kind:   provider.MutationErrSchemaViolation,
			Reason: fmt.Sprintf("geometry type %q not admitted for %q layer", got, admitted),
		}
	}
	return nil
}

// geometryEnvelope computes the XY envelope for the GPKG binary header.
func geometryEnvelope(g geom.Geometry) []float64 {
	pts, err := geom.GetCoordinates(g)
	if err != nil || len(pts) == 0 {
		return nil
	}
	minX, minY, maxX, maxY := pts[0][0], pts[0][1], pts[0][0], pts[0][1]
	for _, p := range pts[1:] {
		if p[0] < minX {
			minX = p[0]
		}
		if p[0] > maxX {
			maxX = p[0]
		}
		if p[1] < minY {
			minY = p[1]
		}
		if p[1] > maxY {
			maxY = p[1]
		}
	}
	return []float64{minX, maxX, minY, maxY}
}

// encodeGPKGGeometry builds the GeoPackage binary header + WKB.
// Flags: little endian (bit 0) + envelope type XY (bits 1-3 = 001).
func encodeGPKGGeometry(rawWKB []byte, srsID int32, envelope []float64) []byte {
	flags := byte(0x01 | (0x01 << 1))
	headerLen := 8
	if envelope != nil {
		headerLen += 32
	} else {
		flags &^= 0x0E // envelope type none
	}
	out := make([]byte, 0, headerLen+len(rawWKB))
	out = append(out, 0x47, 0x50) // GP magic
	out = append(out, 0x00)       // version
	out = append(out, flags)
	var tmp [4]byte
	binary.LittleEndian.PutUint32(tmp[:], uint32(srsID))
	out = append(out, tmp[:]...)
	if envelope != nil {
		var fbuf [8]byte
		for _, v := range envelope {
			binary.LittleEndian.PutUint64(fbuf[:], math.Float64bits(v))
			out = append(out, fbuf[:]...)
		}
	}
	out = append(out, rawWKB...)
	return out
}

// transformGeometry reprojects between SRIDs. Identical SRIDs pass
// through; 4326<->3857 uses the vendored projector. Anything else is an
// explicit error, never a silent passthrough.
func transformGeometry(g geom.Geometry, from, to uint64) (geom.Geometry, error) {
	if from == to {
		return g, nil
	}
	if (from == 4326 && to == 3857) || (from == 3857 && to == 4326) {
		return transform4326_3857(g, from == 4326)
	}
	return nil, &provider.MutationError{
		Kind:   provider.MutationErrUnsupportedCapability,
		Reason: fmt.Sprintf("CRS transform %d -> %d not supported by this writer profile", from, to),
	}
}

// transform4326_3857 reprojects all vertices between lon/lat degrees and
// Web-Mercator metres. forward=true is 4326->3857.
func transform4326_3857(g geom.Geometry, forward bool) (geom.Geometry, error) {
	xform := func(x, y float64) ([2]float64, error) {
		var in []float64
		if forward {
			in = []float64{x, y} // lon, lat
		} else {
			in = []float64{x, y} // mercator x, y
		}
		var out []float64
		var err error
		if forward {
			out, err = proj.Convert(proj.EPSG3857, in)
		} else {
			out, err = proj.Inverse(proj.EPSG3857, in)
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
		pts, err := mapPts(t)
		if err != nil {
			return nil, err
		}
		return geom.LineString(pts), nil
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
		return nil, &provider.MutationError{
			Kind:   provider.MutationErrUnsupportedCapability,
			Reason: fmt.Sprintf("cannot reproject geometry type %T", g),
		}
	}
}

func (t *featureTx) insert(ctx context.Context, mp *writeMapping, m provider.Mutation) (provider.MutationOutcome, error) {
	cols := []string{}
	placeholders := []string{}
	args := []interface{}{}
	for pub, mv := range m.Properties {
		col, ok := mp.writable[pub]
		if !ok {
			return provider.MutationOutcome{}, &provider.MutationError{Kind: provider.MutationErrSchemaViolation, Reason: fmt.Sprintf("property %q is not writable", pub)}
		}
		v, err := valueToSQL(mv, mp.columns[col].Type)
		if err != nil {
			return provider.MutationOutcome{}, &provider.MutationError{Kind: provider.MutationErrMalformedInput, Reason: err.Error()}
		}
		cols = append(cols, quoteIdent(col))
		placeholders = append(placeholders, "?")
		args = append(args, v)
	}
	var geomBounds *[4]float64
	if m.GeometryWKB != nil {
		enc, encStr, bounds, err := encodeStorageGeometry(mp, m.GeometryWKB, m.GeometrySRID)
		if err != nil {
			return provider.MutationOutcome{}, err
		}
		geomBounds = &bounds
		if mp.bboxColumns[0] != "" {
			raw, err := nativeMOSBounds(enc)
			if err != nil {
				return provider.MutationOutcome{}, err
			}
			for i, column := range mp.bboxColumns {
				cols = append(cols, quoteIdent(column))
				placeholders = append(placeholders, "?")
				args = append(args, int64(raw[i]))
			}
		}
		cols = append(cols, quoteIdent(mp.geomColumn))
		placeholders = append(placeholders, "?")
		if encStr != "" {
			args = append(args, encStr)
		} else {
			args = append(args, enc)
		}
	}
	if m.GeometryWKB == nil && (mp.bboxColumns[0] != "" || m.GeometryAbsent) {
		cols = append(cols, quoteIdent(mp.geomColumn))
		placeholders = append(placeholders, "?")
		args = append(args, nil)
		if mp.bboxColumns[0] != "" {
			for _, column := range mp.bboxColumns {
				cols = append(cols, quoteIdent(column))
				placeholders = append(placeholders, "?")
				args = append(args, nil)
			}
		}
	}
	if len(cols) == 0 && m.GeometryWKB == nil {
		return provider.MutationOutcome{}, &provider.MutationError{Kind: provider.MutationErrMalformedInput, Reason: "insert carries no properties or geometry"}
	}
	q := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)", quoteIdent(mp.table), strings.Join(cols, ", "), strings.Join(placeholders, ", "))
	res, err := t.tx.ExecContext(ctx, q, args...)
	if err != nil {
		return provider.MutationOutcome{}, mapSQLError(err)
	}
	id, err := res.LastInsertId()
	if err != nil || id <= 0 {
		return provider.MutationOutcome{}, &provider.MutationError{Kind: provider.MutationErrExecutionFailed, Reason: "insert did not return a row id"}
	}
	t.recordMod(mp, uint64(id), geomBounds, true)
	return provider.MutationOutcome{FeatureID: uint64(id), Affected: 1}, nil
}

func (t *featureTx) replace(ctx context.Context, mp *writeMapping, m provider.Mutation) (provider.MutationOutcome, error) {
	// R04: verify existence to distinguish not-found from no-op.
	exists, err := t.existsInTx(ctx, mp, m.FeatureID)
	if err != nil {
		return provider.MutationOutcome{}, err
	}
	if !exists {
		return provider.MutationOutcome{}, &provider.MutationError{Kind: provider.MutationErrNotFound, Reason: fmt.Sprintf("feature %d not found", m.FeatureID)}
	}
	_, found, err := t.selectRow(ctx, mp, m.FeatureID)
	if err != nil {
		return provider.MutationOutcome{}, err
	}
	if !found {
		return provider.MutationOutcome{}, &provider.MutationError{Kind: provider.MutationErrNotFound, Reason: fmt.Sprintf("feature %d not found", m.FeatureID)}
	}
	if err := t.checkRevision(ctx, mp, m); err != nil {
		return provider.MutationOutcome{}, err
	}
	set := []string{}
	args := []interface{}{}
	for pub, mv := range m.Properties {
		col, ok := mp.writable[pub]
		if !ok {
			return provider.MutationOutcome{}, &provider.MutationError{Kind: provider.MutationErrSchemaViolation, Reason: fmt.Sprintf("property %q is not writable", pub)}
		}
		v, err := valueToSQL(mv, mp.columns[col].Type)
		if err != nil {
			return provider.MutationOutcome{}, &provider.MutationError{Kind: provider.MutationErrMalformedInput, Reason: err.Error()}
		}
		set = append(set, quoteIdent(col)+" = ?")
		args = append(args, v)
	}
	// Omitted columns use their storage default, or NULL when there is none.
	for pub, col := range mp.writable {
		if _, ok := m.Properties[pub]; ok {
			continue
		}
		expression := "NULL"
		if mp.columns[col].IsDefault {
			if err := t.tx.QueryRowContext(ctx, `SELECT dflt_value FROM pragma_table_xinfo(?) WHERE name=?`, mp.table, col).Scan(&expression); err != nil {
				return provider.MutationOutcome{}, err
			}
		}
		set = append(set, quoteIdent(col)+" = "+expression)
	}

	var geomBounds *[4]float64
	geomTouched := false
	if m.GeometryWKB != nil || m.GeometryAbsent {
		geomTouched = true
		set = append(set, quoteIdent(mp.geomColumn)+" = ?")
		if m.GeometryAbsent {
			args = append(args, nil)
		} else {
			enc, encStr, bounds, err := encodeStorageGeometry(mp, m.GeometryWKB, m.GeometrySRID)
			if err != nil {
				return provider.MutationOutcome{}, err
			}
			geomBounds = &bounds
			if encStr != "" {
				args = append(args, encStr)
			} else {
				args = append(args, enc)
			}
			if mp.bboxColumns[0] != "" {
				raw, err := nativeMOSBounds(enc)
				if err != nil {
					return provider.MutationOutcome{}, err
				}
				for i, column := range mp.bboxColumns {
					set = append(set, quoteIdent(column)+" = ?")
					args = append(args, int64(raw[i]))
				}
			}
		}
		if m.GeometryAbsent && mp.bboxColumns[0] != "" {
			for _, column := range mp.bboxColumns {
				set = append(set, quoteIdent(column)+" = NULL")
			}
		}
	}
	if len(set) == 0 {
		if m.GeometryUnchanged && m.IfRevision != "" {
			return provider.MutationOutcome{FeatureID: m.FeatureID, Affected: 1}, nil
		}
		return provider.MutationOutcome{}, &provider.MutationError{Kind: provider.MutationErrMalformedInput, Reason: "replace carries nothing to set"}
	}
	args = append(args, m.FeatureID)
	q := fmt.Sprintf("UPDATE %s SET %s WHERE %s = ?", quoteIdent(mp.table), strings.Join(set, ", "), quoteIdent(mp.idColumn))
	res, err := t.tx.ExecContext(ctx, q, args...)
	if err != nil {
		return provider.MutationOutcome{}, mapSQLError(err)
	}
	n, _ := res.RowsAffected()
	// R04: n==0 can be no-op; existence verified separately.
	_ = n
	t.recordMod(mp, m.FeatureID, geomBounds, geomTouched)
	return provider.MutationOutcome{FeatureID: m.FeatureID, Affected: 1}, nil
}

// ObjectType is application metadata, not the MOS header byte. Preserve its
// meaning by requiring geometry edits to retain the stored geometry family.
func (t *featureTx) validateCanonicalGeometry(ctx context.Context, mp *writeMapping, m provider.Mutation) error {
	var raw []byte
	if err := t.tx.QueryRowContext(ctx, "SELECT "+quoteIdent(mp.geomColumn)+" FROM "+quoteIdent(mp.table)+" WHERE "+quoteIdent(mp.idColumn)+"=?", m.FeatureID).Scan(&raw); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return &provider.MutationError{Kind: provider.MutationErrNotFound, Reason: "feature not found"}
		}
		return err
	}
	if codec.IsSystemInfoValue(raw) {
		return &provider.MutationError{Kind: provider.MutationErrNotFound, Reason: "feature not found"}
	}
	if raw == nil {
		return &provider.MutationError{Kind: provider.MutationErrUnsupportedCapability, Reason: "canonical geometry family cannot be established from an absent geometry"}
	}
	stored, err := mos.Decode(raw, mp.mosOpts)
	if err != nil {
		return err
	}
	header, err := mos.DecodeHeader(raw)
	if err != nil {
		return err
	}
	canonical, err := mos.Encode(stored, mp.mosOpts)
	if err != nil {
		return err
	}
	if (header.ObjectType != mos.TypePoint && header.ObjectType != mos.TypePolyline && header.ObjectType != mos.TypePolygon) || !bytes.Equal(raw, canonical) {
		return &provider.MutationError{Kind: provider.MutationErrUnsupportedCapability, Reason: "canonical MOS geometry rewrite would discard header metadata or opaque payload"}
	}
	proposed, err := wkb.DecodeBytes(m.GeometryWKB)
	if err != nil {
		return err
	}
	return checkGeometryType(proposed, normalizeGeomType(stored))
}

// existsInTx checks feature existence within the tx (R04).
func (t *featureTx) existsInTx(ctx context.Context, mp *writeMapping, featureID uint64) (bool, error) {
	var one int
	q := fmt.Sprintf("SELECT 1 FROM %s WHERE %s = ? LIMIT 1", quoteIdent(mp.table), quoteIdent(mp.idColumn))
	err := t.tx.QueryRowContext(ctx, q, featureID).Scan(&one)
	if err != nil {
		if err == sql.ErrNoRows {
			return false, nil
		}
		return false, mapSQLError(err)
	}
	return true, nil
}

func (t *featureTx) update(ctx context.Context, mp *writeMapping, m provider.Mutation) (provider.MutationOutcome, error) {
	// A03: fail-fast CAS check inside the native transaction.
	if err := checkRevisionCAS(ctx, t.tx, mp.revisionCollection(), m.FeatureID, m.IfRevision); err != nil {
		return provider.MutationOutcome{}, err
	}
	// R04: verify existence to distinguish not-found from no-op.
	exists, err := t.existsInTx(ctx, mp, m.FeatureID)
	if err != nil {
		return provider.MutationOutcome{}, err
	}
	if !exists {
		return provider.MutationOutcome{}, &provider.MutationError{Kind: provider.MutationErrNotFound, Reason: fmt.Sprintf("feature %d not found", m.FeatureID)}
	}
	_, found, err := t.selectRow(ctx, mp, m.FeatureID)
	if err != nil {
		return provider.MutationOutcome{}, err
	}
	if !found {
		return provider.MutationOutcome{}, &provider.MutationError{Kind: provider.MutationErrNotFound, Reason: fmt.Sprintf("feature %d not found", m.FeatureID)}
	}
	if err := t.checkRevision(ctx, mp, m); err != nil {
		return provider.MutationOutcome{}, err
	}
	set := []string{}
	args := []interface{}{}
	for pub, mv := range m.Properties {
		col, ok := mp.writable[pub]
		if !ok {
			return provider.MutationOutcome{}, &provider.MutationError{Kind: provider.MutationErrSchemaViolation, Reason: fmt.Sprintf("property %q is not writable", pub)}
		}
		v, err := valueToSQL(mv, mp.columns[col].Type)
		if err != nil {
			return provider.MutationOutcome{}, &provider.MutationError{Kind: provider.MutationErrMalformedInput, Reason: err.Error()}
		}
		set = append(set, quoteIdent(col)+" = ?")
		args = append(args, v)
	}
	var geomBounds *[4]float64
	geomTouched := false
	// Attribute-only update never touches the geometry column: stored
	// bytes are preserved bit-identically (and the RTree entry stays valid).
	if m.GeometryWKB != nil || m.GeometryAbsent {
		geomTouched = true
		set = append(set, quoteIdent(mp.geomColumn)+" = ?")
		if m.GeometryAbsent {
			args = append(args, nil)
		} else {
			enc, encStr, bounds, err := encodeStorageGeometry(mp, m.GeometryWKB, m.GeometrySRID)
			if err != nil {
				return provider.MutationOutcome{}, err
			}
			geomBounds = &bounds
			if encStr != "" {
				args = append(args, encStr)
			} else {
				args = append(args, enc)
			}
			if mp.bboxColumns[0] != "" {
				raw, err := nativeMOSBounds(enc)
				if err != nil {
					return provider.MutationOutcome{}, err
				}
				for i, column := range mp.bboxColumns {
					set = append(set, quoteIdent(column)+" = ?")
					args = append(args, int64(raw[i]))
				}
			}
		}
		if m.GeometryAbsent && mp.bboxColumns[0] != "" {
			for _, column := range mp.bboxColumns {
				set = append(set, quoteIdent(column)+" = NULL")
			}
		}
	}
	if len(set) == 0 {
		return provider.MutationOutcome{}, &provider.MutationError{Kind: provider.MutationErrMalformedInput, Reason: "update carries nothing to set"}
	}
	args = append(args, m.FeatureID)
	q := fmt.Sprintf("UPDATE %s SET %s WHERE %s = ?", quoteIdent(mp.table), strings.Join(set, ", "), quoteIdent(mp.idColumn))
	res, err := t.tx.ExecContext(ctx, q, args...)
	if err != nil {
		return provider.MutationOutcome{}, mapSQLError(err)
	}
	n, _ := res.RowsAffected()
	// R04: n==0 can be no-op; existence verified separately.
	_ = n
	t.recordMod(mp, m.FeatureID, geomBounds, geomTouched)
	return provider.MutationOutcome{FeatureID: m.FeatureID, Affected: 1}, nil
}

func (t *featureTx) delete(ctx context.Context, mp *writeMapping, m provider.Mutation) (provider.MutationOutcome, error) {
	// A03: fail-fast CAS check inside the native transaction.
	if err := checkRevisionCAS(ctx, t.tx, mp.revisionCollection(), m.FeatureID, m.IfRevision); err != nil {
		return provider.MutationOutcome{}, err
	}
	_, found, err := t.selectRow(ctx, mp, m.FeatureID)
	if err != nil {
		return provider.MutationOutcome{}, err
	}
	if !found {
		return provider.MutationOutcome{}, &provider.MutationError{Kind: provider.MutationErrNotFound, Reason: fmt.Sprintf("feature %d not found", m.FeatureID)}
	}
	if err := t.checkRevision(ctx, mp, m); err != nil {
		return provider.MutationOutcome{}, err
	}
	q := fmt.Sprintf("DELETE FROM %s WHERE %s = ?", quoteIdent(mp.table), quoteIdent(mp.idColumn))
	res, err := t.tx.ExecContext(ctx, q, m.FeatureID)
	if err != nil {
		return provider.MutationOutcome{}, mapSQLError(err)
	}
	n, _ := res.RowsAffected()
	if n != 1 {
		return provider.MutationOutcome{}, &provider.MutationError{Kind: provider.MutationErrPreconditionFailed, Reason: "delete affected unexpected row count"}
	}

	t.recordMod(mp, m.FeatureID, nil, true)
	return provider.MutationOutcome{FeatureID: m.FeatureID, Affected: 1}, nil
}

// checkRevisionCAS implements the A03 If-Match check inside the native
// transaction. SQLite has no SELECT ... FOR UPDATE, but the tx holds the
// single connection write lock, so the read is already serialized.
// An empty ifRevision skips the check; "0" means "no revision yet".
func checkRevisionCAS(ctx context.Context, tx *sql.Tx, collection string, featureID uint64, ifRevision string) error {
	if ifRevision == "" {
		return nil
	}
	// A38: ifRevision is "incarnation.revision" format.
	var wantInc, wantRev string
	if parts := strings.Split(ifRevision, "."); len(parts) == 2 {
		wantInc, wantRev = parts[0], parts[1]
	} else {
		wantInc, wantRev = "0", ifRevision
	}
	var curRev, curInc int64
	err := tx.QueryRowContext(ctx,
		`SELECT revision, incarnation FROM tegola_revisions WHERE collection = ? AND feature_id = ?`,
		collection, featureID).Scan(&curRev, &curInc)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			if wantRev != "0" || wantInc != "0" {
				return &provider.MutationError{Kind: provider.MutationErrPreconditionFailed, Reason: fmt.Sprintf("revision mismatch: expected %s, got 0.0", ifRevision)}
			}
			return nil
		}
		if pa.IsMissingTable(err) {
			// Cannot enforce the precondition without the revisions table.
			return &provider.MutationError{Kind: provider.MutationErrUnsupportedCapability, Reason: "revision precondition requires tegola_revisions table (run migration)"}
		}
		return mapSQLError(err)
	}
	curStr := strconv.FormatInt(curInc, 10) + "." + strconv.FormatInt(curRev, 10)
	wantStr := wantInc + "." + wantRev
	if curStr != wantStr {
		return &provider.MutationError{Kind: provider.MutationErrPreconditionFailed, Reason: fmt.Sprintf("revision mismatch: expected %s, got %s", wantStr, curStr)}
	}
	return nil
}

func (t *featureTx) checkRevision(ctx context.Context, mp *writeMapping, m provider.Mutation) error {
	// A03: delegate to the in-transaction CAS check.
	return checkRevisionCAS(ctx, t.tx, mp.revisionCollection(), m.FeatureID, m.IfRevision)
}

// mapSQLError classifies storage errors without leaking internals.
func mapSQLError(err error) error {
	msg := err.Error()
	switch {
	case strings.Contains(msg, "UNIQUE constraint failed"):
		return &provider.MutationError{Kind: provider.MutationErrPreconditionFailed, Reason: "unique constraint violated"}
	case strings.Contains(msg, "NOT NULL constraint failed"):
		return &provider.MutationError{Kind: provider.MutationErrSchemaViolation, Reason: "not-null constraint violated"}
	case strings.Contains(msg, "FOREIGN KEY constraint failed"):
		return &provider.MutationError{Kind: provider.MutationErrSchemaViolation, Reason: "foreign key constraint violated"}
	case strings.Contains(msg, "database is locked") || strings.Contains(msg, "database table is locked"):
		return &provider.MutationError{Kind: provider.MutationErrLockConflict, Reason: "database is locked"}
	default:
		return &provider.MutationError{Kind: provider.MutationErrMalformedInput, Reason: "storage error"}
	}
}
