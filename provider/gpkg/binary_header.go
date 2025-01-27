package gpkg

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"
)

type envelopeType uint8

// Magic is the magic number encode in the header. It should be 0x4750
var Magic = [2]byte{0x47, 0x50}

const (
	EnvelopeTypeNone    = envelopeType(0)
	EnvelopeTypeXY      = envelopeType(1)
	EnvelopeTypeXYZ     = envelopeType(2)
	EnvelopeTypeXYM     = envelopeType(3)
	EnvelopeTypeXYZM    = envelopeType(4)
	EnvelopeTypeInvalid = envelopeType(5)
)

// NumberOfElements that the particular Evnelope Type will have.
func (et envelopeType) NumberOfElements() int {
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

func (et envelopeType) String() string {
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
// X GeoPackageBinary type
// Y empty geometry
// E Envelope type
// B ByteOrder
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
func (hf headerFlags) Envelope() envelopeType {
	et := uint8((hf & maskEnvelopeType) >> 1)
	if et >= uint8(EnvelopeTypeInvalid) {
		return EnvelopeTypeInvalid
	}
	return envelopeType(et)
}

// IsEmpty returns whether or not the geometry is empty.
func (hf headerFlags) IsEmpty() bool { return ((hf & maskEmptyGeometry) >> 4) == 1 }

// IsStandard returns weather or not the geometry is a standard GeoPackage geometry type.
func (hf headerFlags) IsStandard() bool { return ((hf & maskGeoPackageBinary) >> 5) == 0 }

// BinaryHeader is the gpkg header that accompainies every feature.
type BinaryHeader struct {
	// See: http://www.geopackage.org/spec/
	magic      [2]byte // should be 0x47 0x50  (GP in ASCII)
	version    uint8
	flags      headerFlags
	srsid      int32
	envelope   []float64
	headerSize int // total bytes in header
}

// NewBinaryHeader decodes the data into the BinaryHeader
func NewBinaryHeader(data []byte) (*BinaryHeader, error) {
	if len(data) < 8 {
		return nil, errors.New("not enough bytes to decode header")
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
		return nil, errors.New("invalid envelope type")
	}
	if et == EnvelopeTypeNone {
		return &bh, nil
	}
	num := et.NumberOfElements()
	// there are 8 bytes per float64 value and we need num of them.
	if len(bytes) < (num * 8) {
		return nil, errors.New("not enough bytes to decode header")
	}

	bh.envelope = make([]float64, 0, num)
	for i := 0; i < num; i++ {
		bits := en.Uint64(bytes[i*8 : (i*8)+8])
		bh.envelope = append(bh.envelope, math.Float64frombits(bits))
	}
	if bh.magic[0] != Magic[0] || bh.magic[1] != Magic[1] {
		return &bh, errors.New("invalid magic number")
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
func (h *BinaryHeader) EnvelopeType() envelopeType {
	if h == nil {
		return EnvelopeTypeInvalid
	}
	return h.flags.Envelope()
}

// SRSId is the SRS id of the feature.
func (h *BinaryHeader) SRSId() int32 {
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
