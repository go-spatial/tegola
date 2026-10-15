package encoding

import (
	"encoding/binary"
	"fmt"

	"github.com/SAP/go-hdb/driver/internal/unsafe"
)

/*
Auth variable field size indicator - used for decoding server to client
auth reply fields.

Documented in "SAP HANA SQL Command Network Protocol Reference" version 1.2
chapter 2.3.7.20 with:
- a size <= 250 encoded in one byte or
- an unsigned 2 byte integer size encoded in three bytes
  . first byte equals 255
  . second and third byte are a big endian encoded uint16

The reference implementation (hana-clients) diverges from the documentation:
- Layout.hpp names the byte 255 DataLengthIndicator_NullValue although in the
  auth context it denotes a length indicator (a big endian uint16 follows),
  not a null value
- the writer (CodecParameterWriter.cpp) caps the one byte length at 245
  (DataLengthIndicator_Max1ByteLength) and encodes any larger size as
  255 + big endian uint16
- the reader (CodecParameterReader.cpp) additionally accepts the bytes
  246 and 247 (little endian uint16 / uint32 lengths) and treats any other
  byte as a one byte length

Weirdly enough, the auth prms (client to server) follow the standard
length/size indicator rules (see varFieldInd) instead of this one.
*/

const (
	authFieldLenInd1Byte = 245
	authFieldLenInd2Byte = 246
	authFieldLenInd4Byte = 247
	authFieldLenIndBE    = 255
)

// AuthVarFieldInd decodes an auth variable field indicator.
func (d *Decoder) AuthVarFieldInd() int {
	b := d.Byte()
	switch {
	case b <= authFieldLenInd1Byte:
		return int(b)
	case b == authFieldLenInd2Byte:
		return int(d.Int16())
	case b == authFieldLenIndBE:
		return int(d.Uint16ByteOrder(binary.BigEndian))
	case b == authFieldLenInd4Byte:
		return int(d.Int32())
	default:
		return int(b) // 248..254 one byte length
	}
}

// AuthBytes decodes an auth variable bytes field.
func (d *Decoder) AuthBytes() []byte {
	size := d.AuthVarFieldInd()
	if size == 0 {
		return nil
	}
	return d.Bytes(size)
}

// AuthString decodes an auth variable string field.
func (d *Decoder) AuthString() string {
	size := d.AuthVarFieldInd()
	if size == 0 {
		return ""
	}
	return unsafe.ByteSlice2String(d.Bytes(size))
}

// AuthCesu8String decodes an auth variable cesu8 string field.
func (d *Decoder) AuthCesu8String() (string, error) {
	size := d.AuthVarFieldInd()
	if size == 0 {
		return "", nil
	}
	b, err := d.CESU8Bytes(size)
	if err != nil {
		return "", err
	}
	return unsafe.ByteSlice2String(b), nil
}

// AuthBigUint32 decodes an auth big uint32 field.
func (d *Decoder) AuthBigUint32() (uint32, error) {
	size := d.Byte()
	if size != IntegerFieldSize { // 4 bytes
		return 0, fmt.Errorf("invalid auth uint32 size %d - expected %d", size, IntegerFieldSize)
	}
	return d.Uint32ByteOrder(binary.BigEndian), nil // big endian coded (e.g. rounds param)
}
