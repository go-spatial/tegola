package encoding

import (
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"math/big"

	"github.com/SAP/go-hdb/driver/unicode/cesu8"
	"golang.org/x/text/transform"
)

const writeScratchSize = 4096

// Encoder encodes hdb protocol datatypes an basis of an io.Writer.
type Encoder struct {
	wr io.Writer
	b  []byte // scratch buffer (min 15 Bytes - Decimal)
	tr transform.Transformer
}

// NewEncoder creates a new Encoder instance.
func NewEncoder(wr io.Writer, encoder func() transform.Transformer) *Encoder {
	return &Encoder{
		wr: wr,
		b:  make([]byte, writeScratchSize),
		tr: encoder(),
	}
}

// Zeroes encodes cnt zero byte values.
func (e *Encoder) Zeroes(cnt int) {
	// zero out scratch area
	l := cnt
	if l > len(e.b) {
		l = len(e.b)
	}
	for i := 0; i < l; i++ {
		e.b[i] = 0
	}

	for i := 0; i < cnt; {
		j := cnt - i
		if j > len(e.b) {
			j = len(e.b)
		}
		n, _ := e.wr.Write(e.b[:j])
		if n != j {
			return
		}
		i += n
	}
}

// Bytes encodes bytes.
func (e *Encoder) Bytes(p []byte) {
	e.wr.Write(p)
}

// Byte encodes a byte.
func (e *Encoder) Byte(b byte) { // WriteB as sig differs from WriteByte (vet issues)
	e.b[0] = b
	e.Bytes(e.b[:1])
}

// Bool encodes a boolean.
func (e *Encoder) Bool(v bool) {
	if v {
		e.Byte(1)
	} else {
		e.Byte(0)
	}
}

// Int8 encodes an int8.
func (e *Encoder) Int8(i int8) {
	e.Byte(byte(i))
}

// Int16 encodes an int16.
func (e *Encoder) Int16(i int16) {
	binary.LittleEndian.PutUint16(e.b[:2], uint16(i))
	e.wr.Write(e.b[:2])
}

// Uint16 encodes an uint16.
func (e *Encoder) Uint16(i uint16) {
	binary.LittleEndian.PutUint16(e.b[:2], i)
	e.wr.Write(e.b[:2])
}

// Uint16ByteOrder encodes an uint16 in given byte order.
func (e *Encoder) Uint16ByteOrder(i uint16, byteOrder binary.ByteOrder) {
	byteOrder.PutUint16(e.b[:2], i)
	e.wr.Write(e.b[:2])
}

// Int32 encodes an int32.
func (e *Encoder) Int32(i int32) {
	binary.LittleEndian.PutUint32(e.b[:4], uint32(i))
	e.wr.Write(e.b[:4])
}

// Uint32 encodes an uint32.
func (e *Encoder) Uint32(i uint32) {
	binary.LittleEndian.PutUint32(e.b[:4], i)
	e.wr.Write(e.b[:4])
}

// Int64 encodes an int64.
func (e *Encoder) Int64(i int64) {
	binary.LittleEndian.PutUint64(e.b[:8], uint64(i))
	e.wr.Write(e.b[:8])
}

// Uint64 encodes an uint64.
func (e *Encoder) Uint64(i uint64) {
	binary.LittleEndian.PutUint64(e.b[:8], i)
	e.wr.Write(e.b[:8])
}

// Float32 encodes a float32.
func (e *Encoder) Float32(f float32) {
	bits := math.Float32bits(f)
	binary.LittleEndian.PutUint32(e.b[:4], bits)
	e.wr.Write(e.b[:4])
}

// Float64 encodes a float64.
func (e *Encoder) Float64(f float64) {
	bits := math.Float64bits(f)
	binary.LittleEndian.PutUint64(e.b[:8], bits)
	e.wr.Write(e.b[:8])
}

// Decimal encodes a decimal value.
func (e *Encoder) Decimal(m *big.Int, exp int) {
	b := e.b[:decSize]

	// little endian bigint words (significand) -> little endian db decimal format
	j := 0
	for _, d := range m.Bits() {
		for i := 0; i < _S; i++ {
			b[j] = byte(d)
			d >>= 8
			j++
		}
	}

	// clear scratch buffer
	for i := j; i < decSize; i++ {
		b[i] = 0
	}

	exp += dec128Bias
	b[14] |= (byte(exp) << 1)
	b[15] = byte(uint16(exp) >> 7)

	if m.Sign() == -1 {
		b[15] |= 0x80
	}

	e.wr.Write(b)
}

// Fixed encodes a fixed decimal value.
func (e *Encoder) Fixed(m *big.Int, size int) {
	b := e.b[:size]

	neg := m.Sign() == -1
	fill := byte(0)

	if neg {
		// make positive
		m.Neg(m)
		// 2s complement
		bits := m.Bits()
		// - invert all bits
		for i := 0; i < len(bits); i++ {
			bits[i] = ^bits[i]
		}
		// - add 1
		m.Add(m, natOne)
		fill = 0xff
	}

	// little endian bigint words (significand) -> little endian db decimal format
	j := 0
	for _, d := range m.Bits() {
		/*
			check j < size as number of bytes in m.Bits words can exceed number of fixed size bytes
			e.g. 64 bit architecture:
			- two words equals 16 bytes but fixed size might be 12 bytes
			- invariant: all 'skipped' bytes in most significant word are zero
		*/
		for i := 0; i < _S && j < size; i++ {
			b[j] = byte(d)
			d >>= 8
			j++
		}
	}

	// clear scratch buffer
	for i := j; i < size; i++ {
		b[i] = fill
	}

	e.wr.Write(b)
}

// String encodes a string.
func (e *Encoder) String(s string) {
	e.Bytes([]byte(s))
}

// CESU8Bytes encodes UTF-8 bytes into CESU-8 and returns the CESU-8 bytes written.
func (e *Encoder) CESU8Bytes(p []byte) (int, error) {
	e.tr.Reset()
	cnt := 0
	for i := 0; i < len(p); {
		nDst, nSrc, err := e.tr.Transform(e.b, p[i:], true)
		if nDst != 0 {
			n, _ := e.wr.Write(e.b[:nDst])
			cnt += n
		}
		if err != nil && err != transform.ErrShortDst {
			return cnt, err
		}
		i += nSrc
	}
	return cnt, nil
}

// CESU8String encodes an UTF-8 string into CESU-8 and returns the CESU-8 bytes written.
func (e *Encoder) CESU8String(s string) (int, error) { return e.CESU8Bytes([]byte(s)) }

// varFieldInd encodes a variable field indicator.
func (e *Encoder) varFieldInd(size int) error {
	switch {
	default:
		return fmt.Errorf("max argument length %d of string exceeded", size)
	case size <= int(bytesLenIndSmall):
		e.Byte(byte(size))
	case size <= math.MaxInt16:
		e.Byte(bytesLenIndMedium)
		e.Int16(int16(size))
	case size <= math.MaxInt32:
		e.Byte(bytesLenIndBig)
		e.Int32(int32(size))
	}
	return nil
}

// LIBytes encodes bytes with length indicator.
func (e *Encoder) LIBytes(p []byte) error {
	if err := e.varFieldInd(len(p)); err != nil {
		return err
	}
	e.Bytes(p)
	return nil
}

// LIString encodes a string with length indicator.
func (e *Encoder) LIString(s string) error {
	if err := e.varFieldInd(len(s)); err != nil {
		return err
	}
	e.String(s)
	return nil
}

// CESU8LIBytes encodes UTF-8 into CESU-8 bytes with length indicator.
func (e *Encoder) CESU8LIBytes(p []byte) error {
	size := cesu8.Size(p)
	if err := e.varFieldInd(size); err != nil {
		return err
	}
	_, err := e.CESU8Bytes(p)
	return err
}

// CESU8LIString encodes an UTF-8 into a CESU-8 string with length indicator.
func (e *Encoder) CESU8LIString(s string) error {
	size := cesu8.StringSize(s)
	if err := e.varFieldInd(size); err != nil {
		return err
	}
	_, err := e.CESU8String(s)
	return err
}
