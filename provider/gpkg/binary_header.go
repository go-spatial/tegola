package gpkg

import (
	"encoding/binary"
	"fmt"
	"math"

	"github.com/terranodo/tegola/internal/log"
)

// Byte ordering flags
const wkbXDR = 0 // Big Endian
const wkbNDR = 1 // Little Endian

type BinaryHeader struct {
	// See: http://www.geopackage.org/spec/
	initialized bool
	magic       uint16 // should be 0x4750 (18256)
	version     uint8
	flags       uint8
	flagsReady  bool
	srs_id      int32
	envelope    []float64
	headerSize  int // total bytes in header
}

func bytesToFloat64(bytes []byte, byteOrder uint8) float64 {
	if len(bytes) != 8 {
		//TODO(arolek): seems like we should return an error rather than kill the program
		err := fmt.Errorf("bytesToFloat64(): Need 8 bytes, received %v", len(bytes))
		log.Fatal(err)
	}

	var bitConversion binary.ByteOrder
	if byteOrder == wkbXDR {
		bitConversion = binary.BigEndian
	} else if byteOrder == wkbNDR {
		bitConversion = binary.LittleEndian
	} else {
		//TODO(arolek): seems like we should return an error rather than kill the program
		err := fmt.Errorf("Invalid byte order flag leading WKBGeometry: %v", byteOrder)
		log.Fatal(err)
	}

	bits := bitConversion.Uint64(bytes[:])
	value := math.Float64frombits(bits)

	return value
}

func bytesToInt32(bytes []byte, byteOrder uint8) int32 {
	if len(bytes) != 4 {
		//TODO(arolek): seems like we should return an error rather than kill the program
		err := fmt.Errorf("expecting 4 bytes, got %v", len(bytes))
		log.Error(err)
		return -1
	}

	valueLittleEndian := int32(bytes[3])
	valueLittleEndian <<= 8
	valueLittleEndian |= int32(bytes[2])
	valueLittleEndian <<= 8
	valueLittleEndian |= int32(bytes[1])
	valueLittleEndian <<= 8
	valueLittleEndian |= int32(bytes[0])

	valueBigEndian := int32(bytes[0])
	valueBigEndian <<= 8
	valueBigEndian |= int32(bytes[1])
	valueBigEndian <<= 8
	valueBigEndian |= int32(bytes[2])
	valueBigEndian <<= 8
	valueBigEndian |= int32(bytes[3])

	var value int32
	if byteOrder == wkbNDR {
		value = valueLittleEndian
	} else if byteOrder == wkbXDR {
		value = valueBigEndian
	}

	return value
}

func (h *BinaryHeader) Init(geom []byte) {
	const littleEndian = 1
	const bigEndian = 0

	h.flags = geom[3]
	h.flagsReady = true

	headerByteOrder := h.flags & 0x01

	var magic uint16

	if headerByteOrder == littleEndian {
		magic = uint16(geom[0])
		magic <<= 8
		magic |= uint16(geom[1])
	} else {
		magic = uint16(geom[1])
		magic <<= 8
		magic |= uint16(geom[0])
	}
	h.magic = magic

	h.version = geom[2]

	h.srs_id = bytesToInt32(geom[4:8], wkbNDR)

	eType := h.EnvelopeType()
	hSize := 8
	float64size := 8
	switch eType {
	case 0:
		h.envelope = make([]float64, 0)
	case 1:
		h.envelope = make([]float64, 4)
		hSize += 4 * float64size
		h.envelope[0] = bytesToFloat64(geom[8:16], headerByteOrder)
		h.envelope[1] = bytesToFloat64(geom[16:24], headerByteOrder)
		h.envelope[2] = bytesToFloat64(geom[24:32], headerByteOrder)
		h.envelope[3] = bytesToFloat64(geom[32:40], headerByteOrder)
	case 2:
		h.envelope = make([]float64, 6)
		hSize += 6 * float64size
		h.envelope[0] = bytesToFloat64(geom[8:16], headerByteOrder)
		h.envelope[1] = bytesToFloat64(geom[16:24], headerByteOrder)
		h.envelope[2] = bytesToFloat64(geom[24:32], headerByteOrder)
		h.envelope[3] = bytesToFloat64(geom[32:40], headerByteOrder)
		h.envelope[4] = bytesToFloat64(geom[40:48], headerByteOrder)
		h.envelope[5] = bytesToFloat64(geom[48:56], headerByteOrder)
	case 3:
		h.envelope = make([]float64, 6)
		hSize += 6 * float64size
		h.envelope[0] = bytesToFloat64(geom[8:16], headerByteOrder)
		h.envelope[1] = bytesToFloat64(geom[16:24], headerByteOrder)
		h.envelope[2] = bytesToFloat64(geom[24:32], headerByteOrder)
		h.envelope[3] = bytesToFloat64(geom[32:40], headerByteOrder)
		h.envelope[4] = bytesToFloat64(geom[40:48], headerByteOrder)
		h.envelope[5] = bytesToFloat64(geom[48:56], headerByteOrder)
	case 4:
		h.envelope = make([]float64, 8)
		hSize += 8 * float64size
		h.envelope[0] = bytesToFloat64(geom[8:16], headerByteOrder)
		h.envelope[1] = bytesToFloat64(geom[16:24], headerByteOrder)
		h.envelope[2] = bytesToFloat64(geom[24:32], headerByteOrder)
		h.envelope[3] = bytesToFloat64(geom[32:40], headerByteOrder)
		h.envelope[4] = bytesToFloat64(geom[40:48], headerByteOrder)
		h.envelope[5] = bytesToFloat64(geom[48:56], headerByteOrder)
		h.envelope[6] = bytesToFloat64(geom[56:64], headerByteOrder)
		h.envelope[7] = bytesToFloat64(geom[64:72], headerByteOrder)
	default:
		log.Errorf("invalid envelope type: %v", eType)
		h.envelope = make([]float64, 0)
	}

	h.headerSize = hSize

	log.Debugf("BinaryHeader.Init() header size: %v, geom blob size: %v", hSize, len(geom))

	h.initialized = true
}

func (h *BinaryHeader) isInitialized(caller string) bool {
	if !h.initialized {
		log.Errorf("%v: BinaryHeader not initialized", caller)
		return false
	}

	return true
}

func (h *BinaryHeader) Magic() uint16 {
	if h.isInitialized("Magic()") {
		return h.magic
	}

	return uint16(0)
}

func (h *BinaryHeader) Version() uint8 {
	if h.isInitialized("Version()") {
		return h.version
	}

	return 0
}

func (h *BinaryHeader) EnvelopeType() uint8 {
	/*  0: no envelope (space saving slower indexing option), 0 bytes
	    1: envelope is [minx, maxx, miny, maxy], 32 bytes
	    2: envelope is [minx, maxx, miny, maxy, minz, maxz], 48 bytes
	    3: envelope is [minx, maxx, miny, maxy, minm, maxm], 48 bytes
	    4: envelope is [minx, maxx, miny, maxy, minz, maxz, minm, maxm], 64 bytes
	    5-7: invalid
	*/
	var envelope uint8
	if h.flagsReady {
		envelope = (h.flags & 0xE) >> 1
	} else {
		log.Error("BinaryHeader.flags must be ready before calling this function")
		envelope = 0
	}

	return envelope
}

func (h *BinaryHeader) SRSId() int32 {
	if h.isInitialized("SRSId()") {
		return h.srs_id
	}

	return -1
}

func (h *BinaryHeader) Envelope() []float64 {
	if h.isInitialized("Envelope()") {
		return h.envelope
	}

	return nil
}

func (h *BinaryHeader) Size() int {
	if h.isInitialized("Size()") {
		return h.headerSize
	}

	return -1
}
