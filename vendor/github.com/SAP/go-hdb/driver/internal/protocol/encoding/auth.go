package encoding

import (
	"encoding/binary"
	"fmt"
	"math"

	"github.com/SAP/go-hdb/driver/internal/unsafe"
)

/*
Field size methods (used for decoding) of
- bytes, string, unicode string

- a size <= 250 encoded in one byte or
- an unsigned 2 byte integer size encoded in three bytes
  . first byte equals 255
  . second and third byte is an big endian encoded uint16

See also "SAP HANA SQL Command Network Protocol Reference" version 1.2 chapter 2.3.7.20.

Weirdly enough:
- encoding follows the standard rules for length/size indicators
- see auth prms on details
*/

const (
	authMaxFieldSize1ByteLen    = 250
	authFieldSize2ByteIndicator = 255
)

// AuthVarFieldSize returns the field size of an auth variable field indicator.
func AuthVarFieldSize(size int) int {
	if size > authMaxFieldSize1ByteLen {
		return 3
	}
	return 1
}

// AuthVarFieldInd encodes an auth variable field indicator.
func (e *Encoder) AuthVarFieldInd(size int) error {
	switch {
	case size <= authMaxFieldSize1ByteLen:
		e.Byte(byte(size))
	case size <= math.MaxUint16:
		e.Byte(authFieldSize2ByteIndicator)
		e.Uint16ByteOrder(uint16(size), binary.BigEndian)
	default:
		return fmt.Errorf("invalid field size %d - maximum %d", size, math.MaxUint16)
	}
	return nil
}

// AuthVarFieldInd decodes an auth variable field indicator.
func (d *Decoder) AuthVarFieldInd() int {
	b := d.Byte()
	switch {
	case b <= authMaxFieldSize1ByteLen:
		return int(b)
	case b == authFieldSize2ByteIndicator:
		return int(d.Uint16ByteOrder(binary.BigEndian))
	default:
		panic("invalid sub parameter size indicator")
	}
}

// AuthBytes decodes an auth variable bytes field.
func (d *Decoder) AuthBytes() []byte {
	size := d.AuthVarFieldInd()
	if size == 0 {
		return nil
	}
	b := make([]byte, size)
	d.Bytes(b)
	return b
}

// AuthString decodes an auth variable string field.
func (d *Decoder) AuthString() string {
	size := d.AuthVarFieldInd()
	if size == 0 {
		return ""
	}
	b := make([]byte, size)
	d.Bytes(b)
	return unsafe.ByteSlice2String(b)
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
