package encode

import (
	"encoding/binary"
	"errors"
	"io"

	"github.com/terranodo/tegola/geom/encoding/wkb/internal/consts"
)

type Encoder struct {
	// W is the writer to which the binary data will be writen to.
	W io.Writer
	// ByteOrder is the Byte Order Marker, it defaults to binary.LittleEndian
	ByteOrder binary.ByteOrder
	err       error
}

var EncoderIsNilErr = errors.New("Encoder can not be nil")

func (en *Encoder) conti() bool { return !(en == nil || en.err != nil) }

func (en *Encoder) Write(data ...interface{}) *Encoder {
	if !en.conti() {
		return en
	}
	if en.ByteOrder == nil {
		en.ByteOrder = binary.LittleEndian
	}
	for i := range data {
		en.err = binary.Write(en.W, en.ByteOrder, data[i])
		if en.err != nil {
			return en
		}
	}
	return en
}

func (en *Encoder) Err() error {
	if en == nil {
		return EncoderIsNilErr
	}
	return en.err
}

func (en *Encoder) BOM() *Encoder {
	if !en.conti() {
		return en
	}
	if en.ByteOrder != nil && en.ByteOrder == binary.BigEndian {
		en.Write(byte(0))
		return en
	}
	en.Write(byte(1))
	return en
}

func (en *Encoder) Point(pt [2]float64) {
	en.BOM().Write(consts.Point, pt[0], pt[1])
}
func (en *Encoder) MultiPoint(pts [][2]float64) {
	en.BOM()
	en.Write(consts.MultiPoint, uint32(len(pts)))

	for _, p := range pts {
		en.Point(p)
	}
}
func (en *Encoder) LineString(ln [][2]float64) {
	en.BOM().Write(consts.LineString, uint32(len(ln)))
	for _, p := range ln {
		en.Write(p[0], p[1])
	}
}

func (en *Encoder) MultiLineString(lns [][][2]float64) {
	en.BOM().Write(consts.MultiLineString, uint32(len(lns)))
	for _, l := range lns {
		en.LineString(l)
	}
}

func (en *Encoder) Polygon(ply [][][2]float64) {
	en.BOM().Write(consts.Polygon, uint32(len(ply)))
	for _, r := range ply {
		// close defination is:
		// •  Verify that the line segments close (z coordinates at start and endpoints must also be the same) and don't cross.
		// gotten from: http://edndoc.esri.com/arcsde/9.0/general_topics/shape_validation.htm
		var needToClose bool
		length := uint32(len(r))

		if length > 0 && (r[0][0] != r[length-1][0] || r[0][1] != r[length-1][1]) {
			// Let's close the ring.
			length += 1
			needToClose = true
		}
		en.Write(length)
		for _, pt := range r {
			en.Write(pt[0], pt[1])
		}
		if needToClose {
			en.Write(r[0][0], r[0][1])
		}
	}
}

func (en *Encoder) MultiPolygon(mply [][][][2]float64) {
	en.BOM().Write(consts.MultiPolygon, uint32(len(mply)))
	for _, p := range mply {
		en.Polygon(p)
	}
}
