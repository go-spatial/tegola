package encoding

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"math"
	"math/big"
	"slices"
	"time"

	"github.com/SAP/go-hdb/driver/internal/unsafe"
	"github.com/SAP/go-hdb/driver/unicode/cesu8"
	"golang.org/x/text/transform"
)

// Fields.
func asInt[E byte | int16 | int32 | int64](v any) E {
	i64, ok := v.(int64)
	if !ok {
		panic("invalid integer") // should never happen
	}
	return E(i64)
}

func asTime(v any) time.Time {
	t, ok := v.(time.Time)
	if !ok {
		panic("invalid time") // should never happen
	}
	// store in utc
	return t.UTC()
}

// Encoder encodes hdb protocol datatypes on basis of an io.Writer.
type Encoder struct {
	buf []byte
	tr  transform.Transformer
}

// NewEncoder returns a new Encoder over buf using tr for UTF-8 -> CESU-8 field
// encoding. Either argument may be nil (nil buf: append from empty; nil tr: no
// CESU-8 field encoding on this encoder).
func NewEncoder(buf []byte, tr transform.Transformer) *Encoder {
	return &Encoder{buf: buf, tr: tr}
}

// SetBuffer points the encoder at buf (its write head is len(buf)). The
// transformer is retained.
func (e *Encoder) SetBuffer(buf []byte) { e.buf = buf }

// Buffer returns the encoder's accumulated buffer.
func (e *Encoder) Buffer() []byte { return e.buf }

// Transformer returns the encoder's CESU-8 transformer, e.g. to seed a separate
// encoder that must encode CESU-8 with the same transformer (see auth.Prms).
func (e *Encoder) Transformer() transform.Transformer { return e.tr }

// Zeroes encodes cnt zero byte values.
func (e *Encoder) Zeroes(n int) {
	l := len(e.buf)
	e.buf = slices.Grow(e.buf, n)
	e.buf = e.buf[:l+n]
	for i := l; i < l+n; i++ {
		e.buf[i] = 0
	}
}

// Bytes encodes bytes.
func (e *Encoder) Bytes(p []byte) {
	e.buf = append(e.buf, p...)
}

// Byte encodes a byte.
func (e *Encoder) Byte(b byte) {
	e.buf = append(e.buf, b)
}

// Bool encodes a boolean.
func (e *Encoder) Bool(v bool) {
	if v {
		e.buf = append(e.buf, 1)
	} else {
		e.buf = append(e.buf, 0)
	}
}

// Int8 encodes an int8.
func (e *Encoder) Int8(i8 int8) {
	e.buf = append(e.buf, byte(i8)) //nolint: gosec
}

// Int16 encodes an int16.
func (e *Encoder) Int16(i16 int16) {
	e.buf = binary.LittleEndian.AppendUint16(e.buf, uint16(i16)) //nolint: gosec
}

// Uint16 encodes an uint16.
func (e *Encoder) Uint16(u16 uint16) {
	e.buf = binary.LittleEndian.AppendUint16(e.buf, u16)
}

// Uint16ByteOrder encodes an uint16 in given byte order.
func (e *Encoder) Uint16ByteOrder(u16 uint16, byteOrder binary.ByteOrder) {
	e.buf = byteOrder.(binary.AppendByteOrder).AppendUint16(e.buf, u16)
}

// Int32 encodes an int32.
func (e *Encoder) Int32(i32 int32) {
	e.buf = binary.LittleEndian.AppendUint32(e.buf, uint32(i32)) //nolint:gosec
}

// Uint32 encodes an uint32.
func (e *Encoder) Uint32(u32 uint32) {
	e.buf = binary.LittleEndian.AppendUint32(e.buf, u32)
}

// Int64 encodes an int64.
func (e *Encoder) Int64(i64 int64) {
	e.buf = binary.LittleEndian.AppendUint64(e.buf, uint64(i64)) //nolint:gosec
}

// Uint64 encodes an uint64.
func (e *Encoder) Uint64(u64 uint64) {
	e.buf = binary.LittleEndian.AppendUint64(e.buf, u64)
}

// Float32 encodes a float32.
func (e *Encoder) Float32(f float32) {
	bits := math.Float32bits(f)
	e.buf = binary.LittleEndian.AppendUint32(e.buf, bits)
}

// Float64 encodes a float64.
func (e *Encoder) Float64(f float64) {
	bits := math.Float64bits(f)
	e.buf = binary.LittleEndian.AppendUint64(e.buf, bits)
}

// Decimal encodes a decimal value.
func (e *Encoder) Decimal(m *big.Int, exp int) {
	l := len(e.buf)
	e.buf = slices.Grow(e.buf, decSize)
	e.buf = e.buf[:l+decSize]
	b := e.buf[l:]

	// little endian bigint words (significand) -> little endian db decimal format
	j := 0
	for _, d := range m.Bits() {
		for range _S {
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
	b[14] |= (byte(exp) << 1)      //nolint: gosec
	b[15] = byte(uint16(exp) >> 7) //nolint: gosec

	if m.Sign() == -1 {
		b[15] |= 0x80
	}
}

// Fixed encodes a fixed decimal value.
// Fixed consumes m: for negative values it mutates m in place (two's complement).
// Callers must pass a freshly allocated, single-use *big.Int (do not reuse m afterwards).
func (e *Encoder) Fixed(m *big.Int, size int) {
	l := len(e.buf)
	e.buf = slices.Grow(e.buf, size)
	e.buf = e.buf[:l+size]
	b := e.buf[l:]

	neg := m.Sign() == -1
	fill := byte(0)

	if neg {
		// make positive
		m.Neg(m)
		// 2s complement
		bits := m.Bits()
		// - invert all bits
		for i := range bits {
			bits[i] = ^bits[i]
		}
		// - add 1
		m.Add(m, natOne)
		fill = 0xff
	}

	// little endian bigint words (significand) -> little endian db decimal format
	j := 0
	for _, d := range m.Bits() {
		//	check j < size as number of bytes in m.Bits words can exceed number of fixed size bytes
		//	e.g. 64 bit architecture:
		//	- two words equals 16 bytes but fixed size might be 12 bytes
		//	- invariant: all 'skipped' bytes in most significant word are zero
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
}

// String encodes a string.
func (e *Encoder) String(s string) { e.Bytes(unsafe.String2ByteSlice(s)) }

// CESU8Bytes encodes UTF-8 bytes into CESU-8 and returns the CESU-8 bytes written.
func (e *Encoder) CESU8Bytes(p []byte) (int, error) {
	var err error
	var n int

	// reset before use: this is a self-contained (atEOF) encode of one value; a
	// stateful (custom) transformer must not carry state in from a prior use.
	e.tr.Reset()
	e.buf, n, err = transform.Append(e.tr, e.buf, p)
	return n, err
}

// CESU8String encodes a UTF-8 string into CESU-8 and returns the CESU-8 bytes written.
func (e *Encoder) CESU8String(s string) (int, error) {
	return e.CESU8Bytes(unsafe.String2ByteSlice(s))
}

// varFieldInd encodes a variable field indicator.
func (e *Encoder) varFieldInd(size int) error {
	switch {
	default:
		return fmt.Errorf("max argument length %d of string exceeded", size)
	case size <= int(varFieldLenIndSmall):
		e.Byte(byte(size)) //nolint: gosec
	case size <= math.MaxInt16:
		e.Byte(varFieldLenIndMedium)
		e.Int16(int16(size))
	case size <= math.MaxInt32:
		e.Byte(varFieldLenIndBig)
		e.Int32(int32(size))
	}
	return nil
}

// LIBytes encodes bytes with length indicator.
func (e *Encoder) LIBytes(p []byte) error {
	if err := e.varFieldInd(len(p)); err != nil {
		return err
	}
	e.buf = append(e.buf, p...)
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

// CESU8LIString encodes a UTF-8 string into a CESU-8 string with length indicator.
func (e *Encoder) CESU8LIString(s string) error {
	size := cesu8.StringSize(s)
	if err := e.varFieldInd(size); err != nil {
		return err
	}
	_, err := e.CESU8String(s)
	return err
}

// BooleanField encodes a boolean field.
func (e *Encoder) BooleanField(v any) error {
	if v == nil {
		e.Byte(booleanNullValue)
		return nil
	}
	b, ok := v.(bool)
	if !ok {
		panic("invalid boolean") // should never happen
	}
	if b {
		e.Byte(booleanTrueValue)
	} else {
		e.Byte(booleanFalseValue)
	}
	return nil
}

// TinyintField encodes a tinyint field.
func (e *Encoder) TinyintField(v any) error {
	e.Byte(asInt[byte](v))
	return nil
}

// SmallintField encodes a smallint field.
func (e *Encoder) SmallintField(v any) error {
	e.Int16(asInt[int16](v))
	return nil
}

// IntegerField encodes an integer field.
func (e *Encoder) IntegerField(v any) error {
	e.Int32(asInt[int32](v))
	return nil
}

// BigintField encodes a bigint field.
func (e *Encoder) BigintField(v any) error {
	e.Int64(asInt[int64](v))
	return nil
}

// RealField encodes a real field.
func (e *Encoder) RealField(v any) error {
	f64, ok := v.(float64)
	if !ok {
		panic("invalid real") // should never happen
	}
	e.Float32(float32(f64))
	return nil
}

// DoubleField encodes a double field.
func (e *Encoder) DoubleField(v any) error {
	f64, ok := v.(float64)
	if !ok {
		panic("invalid double") // should never happen
	}
	e.Float64(f64)
	return nil
}

func (e *Encoder) encodeDate(t time.Time) {
	// year: set most sig bit
	// month 0 based
	year, month, day := t.Date()
	e.Uint16(uint16(year) | 0x8000) //nolint: gosec
	e.Int8(int8(month) - 1)         //nolint: gosec
	e.Int8(int8(day))               //nolint: gosec
}

// DateField encodes a date field.
func (e *Encoder) DateField(v any) error {
	e.encodeDate(asTime(v))
	return nil
}

func (e *Encoder) encodeTime(t time.Time) {
	e.Byte(byte(t.Hour()) | 0x80) //nolint: gosec
	e.Int8(int8(t.Minute()))      //nolint: gosec
	msec := t.Second()*1000 + t.Nanosecond()/1000000
	e.Uint16(uint16(msec)) //nolint: gosec
}

// TimeField encodes a time field.
func (e *Encoder) TimeField(v any) error {
	e.encodeTime(asTime(v))
	return nil
}

// TimestampField encodes a timestamp field.
func (e *Encoder) TimestampField(v any) error {
	t := asTime(v)
	e.encodeDate(t)
	e.encodeTime(t)
	return nil
}

// LongdateField encodes a longdate field.
func (e *Encoder) LongdateField(v any) error {
	e.Int64(convertTimeToLongdate(asTime(v)))
	return nil
}

// SeconddateField encodes a seconddate field.
func (e *Encoder) SeconddateField(v any) error {
	e.Int64(convertTimeToSeconddate(asTime(v)))
	return nil
}

// DaydateField encodes a daydate field.
func (e *Encoder) DaydateField(v any) error {
	e.Int32(int32(convertTimeToDayDate(asTime(v)))) //nolint: gosec
	return nil
}

// SecondtimeField encodes a secondtime field.
func (e *Encoder) SecondtimeField(v any) error {
	if v == nil {
		e.Int32(secondtimeNullValue)
		return nil
	}
	e.Int32(int32(convertTimeToSecondtime(asTime(v)))) //nolint: gosec
	return nil
}

// DecimalField encodes a decimal field.
func (e *Encoder) DecimalField(v any) error {
	d, ok := v.(Decimal)
	if !ok {
		panic("invalid decimal") // should never happen
	}
	e.Decimal(d.m, d.exp)
	return nil
}

// FixedField encodes a fixed field.
func (e *Encoder) FixedField(v any, size int) error {
	d, ok := v.(Decimal)
	if !ok {
		panic("invalid fixed") // should never happen
	}
	e.Fixed(d.m, size)
	return nil
}

// VarField encodes a var field.
func (e *Encoder) VarField(v any) error {
	switch v := v.(type) {
	case []byte:
		return e.LIBytes(v)
	case string:
		return e.LIString(v)
	default:
		panic("invalid var value") // should never happen
	}
}

// Cesu8Field encodes a cesu8 field.
func (e *Encoder) Cesu8Field(v any) error {
	switch v := v.(type) {
	case []byte:
		return e.CESU8LIBytes(v)
	case string:
		return e.CESU8LIString(v)
	default:
		panic("invalid cesu8 value") // should never happen
	}
}

// HexField encodes a hex field.
func (e *Encoder) HexField(v any) error {
	switch v := v.(type) {
	case []byte:
		b, err := hex.DecodeString(string(v))
		if err != nil {
			return err
		}
		return e.LIBytes(b)
	case string:
		b, err := hex.DecodeString(v)
		if err != nil {
			return err
		}
		return e.LIBytes(b)
	default:
		panic("invalid hex value") // should never happen
	}
}
