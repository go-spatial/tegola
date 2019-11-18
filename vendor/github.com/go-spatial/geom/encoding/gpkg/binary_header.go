// +build cgo

package gpkg

import (
	"bytes"
	"database/sql/driver"
	"encoding/binary"
	"fmt"
	"math"

	"github.com/gdey/errors"

	"github.com/go-spatial/geom"
	"github.com/go-spatial/geom/encoding/wkb"
)

type EnvelopeType uint8

// Magic is the magic number encode in the header. It should be 0x4750
var Magic = [2]byte{0x47, 0x50}

const (
	EnvelopeTypeNone    = EnvelopeType(0)
	EnvelopeTypeXY      = EnvelopeType(1)
	EnvelopeTypeXYZ     = EnvelopeType(2)
	EnvelopeTypeXYM     = EnvelopeType(3)
	EnvelopeTypeXYZM    = EnvelopeType(4)
	EnvelopeTypeInvalid = EnvelopeType(5)
)

// NumberOfElements that the particular Evnelope Type will have.
func (et EnvelopeType) NumberOfElements() int {
	switch et {
	case EnvelopeTypeNone:
		return 0
	case EnvelopeTypeXY:
		return 4
	case EnvelopeTypeXYZ:
		return 6
	case EnvelopeTypeXYM:
		return 6
	case EnvelopeTypeXYZM:
		return 8
	default:
		return -1
	}
}

func (et EnvelopeType) String() string {
	str := "NONEXYZMXYMINVALID"
	switch et {
	case EnvelopeTypeNone:
		return str[0:4]
	case EnvelopeTypeXY:
		return str[4 : 4+2]
	case EnvelopeTypeXYZ:
		return str[4 : 4+3]
	case EnvelopeTypeXYM:
		return str[8 : 8+3]
	case EnvelopeTypeXYZM:
		return str[4 : 4+4]
	default:
		return str[11:]
	}
}

// HEADER FLAG LAYOUT
// 7 6 5 4 3 2 1 0
// R R X Y E E E B
// R Reserved for future use. (should be set to 0)
// X GeoPackageBinary type // Normal or extented
// Y empty geometry
// E Envelope type
// B ByteOrder
// http://www.geopackage.org/spec/#flags_layout
const (
	maskByteOrder        = 1 << 0
	maskEnvelopeType     = 1<<3 | 1<<2 | 1<<1
	maskEmptyGeometry    = 1 << 4
	maskGeoPackageBinary = 1 << 5
)

type headerFlags byte

func (hf headerFlags) String() string { return fmt.Sprintf("0x%02x", uint8(hf)) }

// Endian will return the encoded Endianess
func (hf headerFlags) Endian() binary.ByteOrder {
	if hf&maskByteOrder == 0 {
		return binary.BigEndian
	}
	return binary.LittleEndian
}

// EnvelopeType returns the type of the envelope.
func (hf headerFlags) Envelope() EnvelopeType {
	et := uint8((hf & maskEnvelopeType) >> 1)
	if et >= uint8(EnvelopeTypeInvalid) {
		return EnvelopeTypeInvalid
	}
	return EnvelopeType(et)
}

// IsEmpty returns whether or not the geometry is empty.
func (hf headerFlags) IsEmpty() bool { return ((hf & maskEmptyGeometry) >> 4) == 1 }

// IsStandard returns weather or not the geometry is a standard GeoPackage geometry type.
func (hf headerFlags) IsStandard() bool { return ((hf & maskGeoPackageBinary) >> 5) == 0 }

func EncodeHeaderFlags(byteOrder binary.ByteOrder, envelope EnvelopeType, extendGeom bool, emptyGeom bool) headerFlags {
	var hf byte
	if byteOrder == binary.LittleEndian {
		hf = 1
	}
	hf = hf | byte(envelope)<<1
	if emptyGeom {
		hf = hf | maskEmptyGeometry
	}
	if extendGeom {
		hf = hf | maskGeoPackageBinary
	}
	return headerFlags(hf)
}

// BinaryHeader is the gpkg header that accompainies every feature.
type BinaryHeader struct {
	// See: http://www.geopackage.org/spec/
	magic    [2]byte // should be 0x47 0x50  (GP in ASCII)
	version  uint8   // should be 0
	flags    headerFlags
	srsid    int32
	envelope []float64
}

// NewBinaryHeader returns a new binary header
func NewBinaryHeader(byteOrder binary.ByteOrder, srsid int32, envelope []float64, et EnvelopeType, extendGeom bool, emptyGeom bool) (*BinaryHeader, error) {
	if et.NumberOfElements() != len(envelope) {
		return nil, ErrEnvelopeEnvelopeTypeMismatch
	}
	return &BinaryHeader{
		magic:    Magic,
		flags:    EncodeHeaderFlags(byteOrder, et, extendGeom, emptyGeom),
		srsid:    srsid,
		envelope: envelope,
	}, nil
}

// DecodeBinaryHeader decodes the data into the BinaryHeader
func DecodeBinaryHeader(data []byte) (*BinaryHeader, error) {
	if len(data) < 8 {
		return nil, ErrInsufficentBytes
	}

	var bh BinaryHeader
	bh.magic[0] = data[0]
	bh.magic[1] = data[1]
	bh.version = data[2]
	bh.flags = headerFlags(data[3])
	en := bh.flags.Endian()
	bh.srsid = int32(en.Uint32(data[4 : 4+4]))

	bytes := data[8:]
	et := bh.flags.Envelope()
	if et == EnvelopeTypeInvalid {
		return nil, ErrInvalidEnvelopeType
	}
	if et == EnvelopeTypeNone {
		return &bh, nil
	}
	num := et.NumberOfElements()
	// there are 8 bytes per float64 value and we need num of them.
	if len(bytes) < (num * 8) {
		return nil, ErrInsufficentBytes
	}

	bh.envelope = make([]float64, 0, num)
	for i := 0; i < num; i++ {
		bits := en.Uint64(bytes[i*8 : (i*8)+8])
		bh.envelope = append(bh.envelope, math.Float64frombits(bits))
	}
	if bh.magic[0] != Magic[0] || bh.magic[1] != Magic[1] {
		return &bh, ErrInvalidMagicNumber
	}
	return &bh, nil

}

// Magic is the magic number encode in the header. It should be 0x4750
func (h *BinaryHeader) Magic() [2]byte {
	if h == nil {
		return Magic
	}
	return h.magic
}

// Version is the version number encode in the header.
func (h *BinaryHeader) Version() uint8 {
	if h == nil {
		return 0
	}
	return h.version
}

// EnvelopeType is the type of the envelope that is provided.
func (h *BinaryHeader) EnvelopeType() EnvelopeType {
	if h == nil {
		return EnvelopeTypeInvalid
	}
	return h.flags.Envelope()
}

// SRSID is the SRS id of the feature.
func (h *BinaryHeader) SRSID() int32 {
	if h == nil {
		return 0
	}
	return h.srsid
}

// Envelope is the bounding box of the feature, used for searching. If the EnvelopeType is EvelopeTypeNone, then there isn't a envelope encoded
// and a search without an index will need to be preformed. This is to save space.
func (h *BinaryHeader) Envelope() []float64 {
	if h == nil {
		return nil
	}
	return h.envelope
}

// IsGeometryEmpty tells us if the geometry should be considered empty.
func (h *BinaryHeader) IsGeometryEmpty() bool {
	if h == nil {
		return true
	}
	return h.flags.IsEmpty()
}

// IsStandardGeometry is the geometry a core/extended geometry type, or a user defined geometry type.
func (h *BinaryHeader) IsStandardGeometry() bool {
	if h == nil {
		return true
	}
	return h.flags.IsStandard()
}

// Size is the size of the header in bytes.
func (h *BinaryHeader) Size() int {
	if h == nil {
		return 0
	}
	return (len(h.envelope) * 8) + 8
}

func (h *BinaryHeader) EncodeTo(data *bytes.Buffer) error {
	if data == nil {
		return errors.String("buffer is nil")
	}
	var err error
	hh := h
	if hh == nil {
		hh, err = NewBinaryHeader(h.Endian(), 0, []float64{}, h.EnvelopeType(), false, true)
		if err != nil {
			return err
		}
	}
	en := h.Endian()
	data.Write([]byte{h.magic[0], h.magic[1], byte(h.version), byte(h.flags)})
	err = binary.Write(data, en, h.srsid)
	if err != nil {
		return err
	}
	return binary.Write(data, en, h.envelope)
}

func (h *BinaryHeader) Encode() ([]byte, error) {
	var data bytes.Buffer
	if err := h.EncodeTo(&data); err != nil {
		return nil, err
	}
	return data.Bytes(), nil
}
func (h *BinaryHeader) Endian() binary.ByteOrder {
	if h == nil {
		return binary.BigEndian
	}
	return h.flags.Endian()
}

// StandardBinary is the binary encoding pluse some meta data
// should be stored as a blob
type StandardBinary struct {
	Header   *BinaryHeader
	SRSID    int32
	Geometry geom.Geometry
}

func DecodeGeometry(bytes []byte) (*StandardBinary, error) {
	h, err := DecodeBinaryHeader(bytes)
	if err != nil {
		return nil, err
	}

	geo, err := wkb.DecodeBytes(bytes[h.Size():])
	if err != nil {
		return nil, err
	}
	return &StandardBinary{
		Header:   h,
		SRSID:    h.SRSID(),
		Geometry: geo,
	}, nil
}

func (sb StandardBinary) Encode() ([]byte, error) {
	var data bytes.Buffer
	err := sb.Header.EncodeTo(&data)
	if err != nil {
		return nil, err
	}

	err = wkb.EncodeWithByteOrder(sb.Header.Endian(), &data, sb.Geometry)
	if err != nil {
		return nil, err
	}
	return data.Bytes(), nil
}

func NewBinary(srs int32, geo geom.Geometry) (*StandardBinary, error) {

	var (
		emptyGeo = geom.IsEmpty(geo)
		err      error
		extent   = []float64{nan, nan, nan, nan}
		h        *BinaryHeader
	)

	if !emptyGeo {
		ext, err := geom.NewExtentFromGeometry(geo)
		if err != nil {
			return nil, err
		}
		extent = ext[:]
	}

	h, err = NewBinaryHeader(binary.LittleEndian, srs, extent, EnvelopeTypeXY, false, emptyGeo)
	if err != nil {
		return nil, err
	}

	return &StandardBinary{
		Header:   h,
		SRSID:    srs,
		Geometry: geo,
	}, nil
}

func (sb *StandardBinary) Extent() *geom.Extent {
	if sb == nil {
		return nil
	}
	if geom.IsEmpty(sb.Geometry) {
		return nil
	}
	extent, err := geom.NewExtentFromGeometry(sb.Geometry)
	if err != nil {
		return nil
	}
	return extent
}

func (sb *StandardBinary) Value() (driver.Value, error) {
	if sb == nil {
		return nil, ErrNilStandardBinary
	}
	return sb.Encode()
}

func (sb *StandardBinary) Scan(value interface{}) error {
	if sb == nil {
		return ErrNilStandardBinary
	}
	data, ok := value.([]byte)
	if !ok {
		return errors.String("only support byte slice for Geometry")
	}
	sb1, err := DecodeGeometry(data)
	if err != nil {
		return err
	}
	sb.Header = sb1.Header
	sb.SRSID = sb1.SRSID
	sb.Geometry = sb1.Geometry
	return nil
}
