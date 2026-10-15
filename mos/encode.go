// Encode mirrors TMapObjectStructureBase.PutToBufInternal for the geometry
// prefix: it serializes a geometry into a MapplGIS MOS blob.
//
// Wire layout (all little-endian), matching the Delphi THeaderObject:
//
//	12-byte header: oType(1), oTypeModification(1)=0, AddFlag(2)=0,
//	  subObjectsCount(2), pointsCount(4), ofl(2)=0
//	subObjectsCount x uint32 point counts
//	pointsCount x (int32 x, int32 y) contiguous
//
// Quantization mirrors DoublePointToPoint: stored = Round((coord-Offset) *
// 10^Precision), round half away from zero; decode recovers
// stored/10^Precision*UnitFactor + Offset. Only the geometry prefix is
// written; labels, markers and other attribute blocks are out of scope.
package mos

import (
	"encoding/binary"
	"fmt"
	"math"

	"github.com/alexeydott/geom"
)

// Encode serializes g into a MOS geometry blob using opts. Supported types:
// Point, MultiPoint, LineString, MultiLineString, Polygon, MultiPolygon.
// Z/M ordinates are dropped (MOS stores XY int32 pairs).
func Encode(g geom.Geometry, opts Options) ([]byte, error) {
	k, err := opts.kPrecision()
	if err != nil {
		return nil, err
	}
	oType, subObjects, err := splitSubObjects(g)
	if err != nil {
		return nil, err
	}
	uf := opts.unitFactor()
	// Quantize: inverse of Decode, stored = Round((real/UnitFactor - Offset) * k).
	qSubs := make([][][2]int32, len(subObjects))
	total := 0
	for i, so := range subObjects {
		q := make([][2]int32, len(so))
		for j, p := range so {
			qx, err := quantizeCoord(p[0]/uf, opts.OffsetX, k)
			if err != nil {
				return nil, fmt.Errorf("mos: x ordinate out of int32 range: %v", p[0])
			}
			qy, err := quantizeCoord(p[1]/uf, opts.OffsetY, k)
			if err != nil {
				return nil, fmt.Errorf("mos: y ordinate out of int32 range: %v", p[1])
			}
			q[j] = [2]int32{qx, qy}
		}
		qSubs[i] = q
		total += len(q)
	}
	if total > math.MaxInt32 {
		return nil, fmt.Errorf("mos: too many points (%v)", total)
	}
	if len(qSubs) > math.MaxUint16 {
		return nil, fmt.Errorf("mos: too many subobjects (%v)", len(qSubs))
	}
	// Layout: 12-byte header + counts + points.
	out := make([]byte, 0, 12+4*len(qSubs)+8*total)
	out = append(out, oType, 0) // oType, oTypeModification
	var tmp [4]byte
	binary.LittleEndian.PutUint16(tmp[:2], 0) // AddFlag
	out = append(out, tmp[:2]...)
	binary.LittleEndian.PutUint16(tmp[:2], uint16(len(qSubs)))
	out = append(out, tmp[:2]...)
	binary.LittleEndian.PutUint32(tmp[:4], uint32(total))
	out = append(out, tmp[:4]...)
	binary.LittleEndian.PutUint16(tmp[:2], 0) // ofl
	out = append(out, tmp[:2]...)
	for _, q := range qSubs {
		binary.LittleEndian.PutUint32(tmp[:4], uint32(len(q)))
		out = append(out, tmp[:4]...)
	}
	var ptmp [8]byte
	for _, q := range qSubs {
		for _, p := range q {
			binary.LittleEndian.PutUint32(ptmp[:4], uint32(p[0]))
			binary.LittleEndian.PutUint32(ptmp[4:], uint32(p[1]))
			out = append(out, ptmp[:]...)
		}
	}
	return out, nil
}

// quantizeCoord mirrors DoublePointToPoint((c-offset)*k): round half away
// from zero into int32.
func quantizeCoord(c, offset, k float64) (int32, error) {
	v := (c - offset) * k
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return 0, fmt.Errorf("mos: non-finite ordinate %v", c)
	}
	r := math.Round(v)
	if r < math.MinInt32 || r > math.MaxInt32 {
		return 0, fmt.Errorf("mos: ordinate %v out of int32 range at precision", c)
	}
	return int32(r), nil
}

// splitSubObjects maps a geometry to (oType, subobjects).
func splitSubObjects(g geom.Geometry) (byte, [][][2]float64, error) {
	switch t := g.(type) {
	case geom.Point:
		return TypePoint, [][][2]float64{{{t[0], t[1]}}}, nil
	case geom.MultiPoint:
		subs := make([][][2]float64, len(t))
		for i, p := range t {
			subs[i] = [][2]float64{{p[0], p[1]}}
		}
		return TypePoint, subs, nil
	case geom.LineString:
		pts := make([][2]float64, len(t))
		copy(pts, t)
		return TypePolyline, [][][2]float64{pts}, nil
	case geom.MultiLineString:
		subs := make([][][2]float64, len(t))
		for i, l := range t {
			pts := make([][2]float64, len(l))
			copy(pts, l)
			subs[i] = pts
		}
		return TypePolyline, subs, nil
	case geom.Polygon:
		subs := make([][][2]float64, len(t))
		for i, r := range t {
			subs[i] = withoutClosingDup(r)
		}
		return TypePolygon, subs, nil
	case geom.MultiPolygon:
		var subs [][][2]float64
		for _, p := range t {
			for _, r := range p {
				subs = append(subs, withoutClosingDup(r))
			}
		}
		return TypePolygon, subs, nil
	default:
		return 0, nil, fmt.Errorf("mos: cannot encode geometry type %T", g)
	}
}

// withoutClosingDup drops the closing vertex (decoder re-closes rings and
// drops consecutive duplicates on read, mirroring GetFromBufInternal).
func withoutClosingDup(r [][2]float64) [][2]float64 {
	if len(r) > 1 && r[0] == r[len(r)-1] {
		return r[:len(r)-1]
	}
	return r
}
