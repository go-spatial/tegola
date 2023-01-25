package encoding

import (
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"math/big"

	"golang.org/x/text/transform"
)

const readScratchSize = 4096

// Decoder decodes hdb protocol datatypes an basis of an io.Reader.
type Decoder struct {
	rd io.Reader
	/* err: fatal read error
	- not set by conversion errors
	- conversion errors are returned by the reader function itself
	*/
	err error
	b   []byte // scratch buffer (used for skip, CESU8Bytes - define size not too small!)
	tr  transform.Transformer
	cnt int
	dfv int
}

// NewDecoder creates a new Decoder instance based on an io.Reader.
func NewDecoder(rd io.Reader, decoder func() transform.Transformer) *Decoder {
	return &Decoder{
		rd: rd,
		b:  make([]byte, readScratchSize),
		tr: decoder(),
	}
}

// Dfv returns the data format version.
func (d *Decoder) Dfv() int {
	return d.dfv
}

// SetDfv sets the data format version.
func (d *Decoder) SetDfv(dfv int) {
	d.dfv = dfv
}

// ResetCnt resets the byte read counter.
func (d *Decoder) ResetCnt() {
	d.cnt = 0
}

// Cnt returns the value of the byte read counter.
func (d *Decoder) Cnt() int {
	return d.cnt
}

// Error returns the last decoder error.
func (d *Decoder) Error() error {
	return d.err
}

// ResetError return and resets reader error.
func (d *Decoder) ResetError() error {
	err := d.err
	d.err = nil
	return err
}

// readFull reads data from reader + read counter and error handling
func (d *Decoder) readFull(buf []byte) (int, error) {
	if d.err != nil {
		return 0, d.err
	}
	var n int
	n, d.err = io.ReadFull(d.rd, buf)
	d.cnt += n
	if d.err != nil {
		return n, d.err
	}
	return n, nil
}

// Skip skips cnt bytes from reading.
func (d *Decoder) Skip(cnt int) {
	var n int
	for n < cnt {
		to := cnt - n
		if to > readScratchSize {
			to = readScratchSize
		}
		m, err := d.readFull(d.b[:to])
		n += m
		if err != nil {
			return
		}
	}
}

// Byte decodes a byte.
func (d *Decoder) Byte() byte {
	if _, err := d.readFull(d.b[:1]); err != nil {
		return 0
	}
	return d.b[0]
}

// Bytes decodes bytes.
func (d *Decoder) Bytes(p []byte) {
	d.readFull(p)
}

// Bool decodes a boolean.
func (d *Decoder) Bool() bool {
	return d.Byte() != 0
}

// Int8 decodes an int8.
func (d *Decoder) Int8() int8 {
	return int8(d.Byte())
}

// Int16 decodes an int16.
func (d *Decoder) Int16() int16 {
	if _, err := d.readFull(d.b[:2]); err != nil {
		return 0
	}
	return int16(binary.LittleEndian.Uint16(d.b[:2]))
}

// Uint16 decodes an uint16.
func (d *Decoder) Uint16() uint16 {
	if _, err := d.readFull(d.b[:2]); err != nil {
		return 0
	}
	return binary.LittleEndian.Uint16(d.b[:2])
}

// Uint16ByteOrder decodes an uint16 in given byte order.
func (d *Decoder) Uint16ByteOrder(byteOrder binary.ByteOrder) uint16 {
	if _, err := d.readFull(d.b[:2]); err != nil {
		return 0
	}
	return byteOrder.Uint16(d.b[:2])
}

// Int32 decodes an int32.
func (d *Decoder) Int32() int32 {
	if _, err := d.readFull(d.b[:4]); err != nil {
		return 0
	}
	return int32(binary.LittleEndian.Uint32(d.b[:4]))
}

// Uint32 decodes an uint32.
func (d *Decoder) Uint32() uint32 {
	if _, err := d.readFull(d.b[:4]); err != nil {
		return 0
	}
	return binary.LittleEndian.Uint32(d.b[:4])
}

// Uint32ByteOrder decodes an uint32 in given byte order.
func (d *Decoder) Uint32ByteOrder(byteOrder binary.ByteOrder) uint32 {
	if _, err := d.readFull(d.b[:4]); err != nil {
		return 0
	}
	return byteOrder.Uint32(d.b[:4])
}

// Int64 decodes an int64.
func (d *Decoder) Int64() int64 {
	if _, err := d.readFull(d.b[:8]); err != nil {
		return 0
	}
	return int64(binary.LittleEndian.Uint64(d.b[:8]))
}

// Uint64 decodes an uint64.
func (d *Decoder) Uint64() uint64 {
	if _, err := d.readFull(d.b[:8]); err != nil {
		return 0
	}
	return binary.LittleEndian.Uint64(d.b[:8])
}

// Float32 decodes a float32.
func (d *Decoder) Float32() float32 {
	if _, err := d.readFull(d.b[:4]); err != nil {
		return 0
	}
	bits := binary.LittleEndian.Uint32(d.b[:4])
	return math.Float32frombits(bits)
}

// Float64 decodes a float64.
func (d *Decoder) Float64() float64 {
	if _, err := d.readFull(d.b[:8]); err != nil {
		return 0
	}
	bits := binary.LittleEndian.Uint64(d.b[:8])
	return math.Float64frombits(bits)
}

// Decimal decodes a decimal.
// - error is only returned in case of conversion errors.
func (d *Decoder) Decimal() (*big.Int, int, error) { // m, exp
	bs := d.b[:decSize]

	if _, err := d.readFull(bs); err != nil {
		return nil, 0, nil
	}

	if (bs[15] & 0x70) == 0x70 { //null value (bit 4,5,6 set)
		return nil, 0, nil
	}

	if (bs[15] & 0x60) == 0x60 {
		return nil, 0, fmt.Errorf("decimal: format (infinity, nan, ...) not supported : %v", bs)
	}

	neg := (bs[15] & 0x80) != 0
	exp := int((((uint16(bs[15])<<8)|uint16(bs[14]))<<1)>>2) - dec128Bias

	// b14 := b[14]  // save b[14]
	bs[14] &= 0x01 // keep the mantissa bit (rest: sign and exp)

	//most significand byte
	msb := 14
	for msb > 0 && bs[msb] == 0 {
		msb--
	}

	//calc number of words
	numWords := (msb / _S) + 1
	ws := make([]big.Word, numWords)

	bs = bs[:msb+1]
	for i, b := range bs {
		ws[i/_S] |= (big.Word(b) << (i % _S * 8))
	}

	m := new(big.Int).SetBits(ws)
	if neg {
		m = m.Neg(m)
	}
	return m, exp, nil
}

// Fixed decodes a fixed decimal.
func (d *Decoder) Fixed(size int) *big.Int { // m, exp
	bs := d.b[:size]

	if _, err := d.readFull(bs); err != nil {
		return nil
	}

	neg := (bs[size-1] & 0x80) != 0 // is negative number (2s complement)

	//most significand byte
	msb := size - 1
	for msb > 0 && bs[msb] == 0 {
		msb--
	}

	//calc number of words
	numWords := (msb / _S) + 1
	ws := make([]big.Word, numWords)

	bs = bs[:msb+1]
	for i, b := range bs {
		// if negative: invert byte (2s complement)
		if neg {
			b = ^b
		}
		ws[i/_S] |= (big.Word(b) << (i % _S * 8))
	}

	m := new(big.Int).SetBits(ws)

	if neg {
		m.Add(m, natOne) // 2s complement - add 1
		m.Neg(m)         // set sign
	}
	return m
}

// CESU8Bytes decodes CESU-8 into UTF-8 bytes.
// - error is only returned in case of conversion errors.
func (d *Decoder) CESU8Bytes(size int) ([]byte, error) {
	if d.err != nil {
		return nil, nil
	}

	var p []byte
	if size > readScratchSize {
		p = make([]byte, size)
	} else {
		p = d.b[:size]
	}

	if _, err := d.readFull(p); err != nil {
		return nil, nil
	}

	b, _, err := transform.Bytes(d.tr, p)
	return b, err
}

// varFieldInd decodes a variable field indicator.
func (d *Decoder) varFieldInd() (n, size int, null bool) {
	ind := d.Byte() //length indicator
	switch {
	default:
		return 1, 0, false
	case ind == bytesLenIndNullValue:
		return 1, 0, true
	case ind <= bytesLenIndSmall:
		return 1, int(ind), false
	case ind == bytesLenIndMedium:
		return 3, int(d.Int16()), false
	case ind == bytesLenIndBig:
		return 5, int(d.Int32()), false
	}
}

// LIBytes decodes bytes with length indicator.
func (d *Decoder) LIBytes() (n int, b []byte) {
	n, size, null := d.varFieldInd()
	if null {
		return n, nil
	}
	b = make([]byte, size)
	d.Bytes(b)
	return n + size, b
}

// LIString decodes a string with length indicator.
func (d *Decoder) LIString() (n int, s string) {
	n, b := d.LIBytes()
	return n, string(b)
}

// CESU8LIBytes decodes CESU-8 into UTF-8 bytes with length indicator.
func (d *Decoder) CESU8LIBytes() (int, []byte, error) {
	n, size, null := d.varFieldInd()
	if null {
		return n, nil, nil
	}
	b, err := d.CESU8Bytes(size)
	return n + size, b, err
}

// CESU8LIString decodes a CESU-8 into a UTF-8 string with length indicator.
func (d *Decoder) CESU8LIString() (int, string, error) {
	n, b, err := d.CESU8LIBytes()
	return n, string(b), err
}
