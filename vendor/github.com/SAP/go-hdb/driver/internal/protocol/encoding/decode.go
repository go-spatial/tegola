package encoding

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"math"
	"math/big"
	"time"

	"github.com/SAP/go-hdb/driver/internal/unsafe"
	"golang.org/x/text/transform"
)

// ShortBufferError is panicked by a Decoder read that would consume more bytes
// than remain in the buffer - a truncated or internally inconsistent part
// payload, which a correct server never sends. It is recovered at the
// part-decode boundary and turned into a normal statement error; it is not
// meant to escape the protocol layer.
type ShortBufferError struct{ Need, Have int }

func (e ShortBufferError) Error() string {
	return fmt.Sprintf("decode: short buffer: need %d bytes, have %d", e.Need, e.Have)
}

// Decoder decodes hdb protocol datatypes on basis of an buffer.
//
// Decoder primitives slice the buffer using lengths/counts taken from the wire.
// A read is length-checked first: if the buffer is shorter than the read
// requires - an internally inconsistent frame, which a correct server never
// sends - the read panics ShortBufferError rather than a raw slice-bounds
// error. The typed panic lets the part-decode boundary recover it
// deterministically (see protocol.recoverShortBuffer) and fail just the current
// statement, while a genuine driver bug (a different panic) still crashes.
type Decoder struct {
	buf []byte
	tr  transform.Transformer
}

// NewDecoder returns a new Decoder over buf using tr for CESU-8 -> UTF-8 field
// decoding. Either argument may be nil (nil buf: set later via SetBuffer; nil
// tr: no CESU-8 field decoding on this decoder).
func NewDecoder(buf []byte, tr transform.Transformer) *Decoder {
	return &Decoder{buf: buf, tr: tr}
}

// SetBuffer points the decoder at buf, resetting its read position. The
// transformer is retained.
func (d *Decoder) SetBuffer(buf []byte) { d.buf = buf }

// Buffer returns the decoder's (remaining) buffer.
func (d *Decoder) Buffer() []byte { return d.buf }

// Transformer returns the decoder's CESU-8 transformer.
func (d *Decoder) Transformer() transform.Transformer { return d.tr }

// Skip skips cnt bytes from reading.
func (d *Decoder) Skip(cnt int) {
	if len(d.buf) < cnt {
		panic(ShortBufferError{Need: cnt, Have: len(d.buf)})
	}
	d.buf = d.buf[cnt:]
}

// Byte decodes a byte.
func (d *Decoder) Byte() byte {
	if len(d.buf) < 1 {
		panic(ShortBufferError{Need: 1, Have: len(d.buf)})
	}
	b := d.buf[0]
	d.buf = d.buf[1:]
	return b
}

// Bytes decodes bytes.
func (d *Decoder) Bytes(n int) []byte {
	if len(d.buf) < n {
		panic(ShortBufferError{Need: n, Have: len(d.buf)})
	}
	b := d.buf[:n]
	d.buf = d.buf[n:]
	return b
}

// Str decodes strings.
func (d *Decoder) Str(n int) string {
	if len(d.buf) < n {
		panic(ShortBufferError{Need: n, Have: len(d.buf)})
	}
	b := d.buf[:n]
	d.buf = d.buf[n:]
	return unsafe.ByteSlice2String(b)
}

// Bool decodes a boolean.
func (d *Decoder) Bool() bool {
	return d.Byte() != 0
}

// Int8 decodes an int8.
func (d *Decoder) Int8() int8 {
	return int8(d.Byte()) //nolint:gosec
}

// Int16 decodes an int16.
func (d *Decoder) Int16() int16 {
	if len(d.buf) < 2 {
		panic(ShortBufferError{Need: 2, Have: len(d.buf)})
	}
	i16 := int16(binary.LittleEndian.Uint16(d.buf)) //nolint: gosec
	d.buf = d.buf[2:]
	return i16
}

// Uint16 decodes an uint16.
func (d *Decoder) Uint16() uint16 {
	if len(d.buf) < 2 {
		panic(ShortBufferError{Need: 2, Have: len(d.buf)})
	}
	u16 := binary.LittleEndian.Uint16(d.buf)
	d.buf = d.buf[2:]
	return u16
}

// Uint16ByteOrder decodes an uint16 in given byte order.
func (d *Decoder) Uint16ByteOrder(byteOrder binary.ByteOrder) uint16 {
	if len(d.buf) < 2 {
		panic(ShortBufferError{Need: 2, Have: len(d.buf)})
	}
	u16 := byteOrder.Uint16(d.buf)
	d.buf = d.buf[2:]
	return u16
}

// Int32 decodes an int32.
func (d *Decoder) Int32() int32 {
	if len(d.buf) < 4 {
		panic(ShortBufferError{Need: 4, Have: len(d.buf)})
	}
	i32 := int32(binary.LittleEndian.Uint32(d.buf)) //nolint: gosec
	d.buf = d.buf[4:]
	return i32
}

// Uint32 decodes an uint32.
func (d *Decoder) Uint32() uint32 {
	if len(d.buf) < 4 {
		panic(ShortBufferError{Need: 4, Have: len(d.buf)})
	}
	u32 := binary.LittleEndian.Uint32(d.buf)
	d.buf = d.buf[4:]
	return u32
}

// Uint32ByteOrder decodes an uint32 in given byte order.
func (d *Decoder) Uint32ByteOrder(byteOrder binary.ByteOrder) uint32 {
	if len(d.buf) < 4 {
		panic(ShortBufferError{Need: 4, Have: len(d.buf)})
	}
	u32 := byteOrder.Uint32(d.buf)
	d.buf = d.buf[4:]
	return u32
}

// Int64 decodes an int64.
func (d *Decoder) Int64() int64 {
	if len(d.buf) < 8 {
		panic(ShortBufferError{Need: 8, Have: len(d.buf)})
	}
	i64 := int64(binary.LittleEndian.Uint64(d.buf)) //nolint: gosec
	d.buf = d.buf[8:]
	return i64
}

// Uint64 decodes an uint64.
func (d *Decoder) Uint64() uint64 {
	if len(d.buf) < 8 {
		panic(ShortBufferError{Need: 8, Have: len(d.buf)})
	}
	u64 := binary.LittleEndian.Uint64(d.buf)
	d.buf = d.buf[8:]
	return u64
}

// Float32 decodes a float32.
func (d *Decoder) Float32() float32 {
	if len(d.buf) < 4 {
		panic(ShortBufferError{Need: 4, Have: len(d.buf)})
	}
	bits := binary.LittleEndian.Uint32(d.buf)
	d.buf = d.buf[4:]
	return math.Float32frombits(bits)
}

// Float64 decodes a float64.
func (d *Decoder) Float64() float64 {
	if len(d.buf) < 8 {
		panic(ShortBufferError{Need: 8, Have: len(d.buf)})
	}
	bits := binary.LittleEndian.Uint64(d.buf)
	d.buf = d.buf[8:]
	return math.Float64frombits(bits)
}

// Decimal decodes a decimal.
// - error is only returned in case of conversion errors.
func (d *Decoder) Decimal() (*big.Int, int, error) { // m, exp
	if len(d.buf) < decSize {
		panic(ShortBufferError{Need: decSize, Have: len(d.buf)})
	}
	bs := d.buf[:decSize]
	d.buf = d.buf[decSize:]

	if (bs[15] & 0x70) == 0x70 { // null value (bit 4,5,6 set)
		return nil, 0, nil
	}

	if (bs[15] & 0x60) == 0x60 {
		return nil, 0, fmt.Errorf("decimal: format (infinity, nan, ...) not supported : %v", bs)
	}

	neg := (bs[15] & 0x80) != 0
	exp := int((((uint16(bs[15])<<8)|uint16(bs[14]))<<1)>>2) - dec128Bias

	// b14 := b[14]  // save b[14]
	bs[14] &= 0x01 // keep the mantissa bit (rest: sign and exp)

	// most significand byte
	msb := 14
	for msb > 0 && bs[msb] == 0 {
		msb--
	}

	// calc number of words
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
	if len(d.buf) < size {
		panic(ShortBufferError{Need: size, Have: len(d.buf)})
	}
	bs := d.buf[:size]
	d.buf = d.buf[size:]

	neg := (bs[size-1] & 0x80) != 0 // is negative number (2s complement)

	// most significand byte
	msb := size - 1
	for msb > 0 && bs[msb] == 0 {
		msb--
	}

	// calc number of words
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
	if len(d.buf) < size {
		panic(ShortBufferError{Need: size, Have: len(d.buf)})
	}
	p := d.buf[:size]
	d.buf = d.buf[size:]

	// reset before use: this is a self-contained (atEOF) decode of one value; a
	// stateful (custom) transformer must not carry state in from a prior use.
	d.tr.Reset()
	n, _, err := d.tr.Transform(p, p, true) // transform inline
	if err != nil {
		return nil, err
	}
	return p[:n], nil
}

// varFieldInd decodes a variable field indicator.
func (d *Decoder) varFieldInd() (n, size int, null bool) {
	ind := d.Byte() // length indicator
	switch {
	default:
		return 1, 0, false
	case ind == varFieldLenIndNullValue:
		return 1, 0, true
	case ind <= varFieldLenIndSmall:
		return 1, int(ind), false
	case ind == varFieldLenIndMedium:
		return 3, int(d.Int16()), false
	case ind == varFieldLenIndBig:
		return 5, int(d.Int32()), false
	}
}

// LIBytes decodes bytes with length indicator.
func (d *Decoder) LIBytes() (int, []byte) {
	n, size, null := d.varFieldInd()
	if null {
		return n, nil
	}
	return n + size, d.Bytes(size)
}

// LIString decodes a string with length indicator.
func (d *Decoder) LIString() (n int, s string) {
	n, b := d.LIBytes()
	return n, unsafe.ByteSlice2String(b)
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
	return n, unsafe.ByteSlice2String(b), err
}

// Fields.

// BooleanField decodes a boolean field.
func (d *Decoder) BooleanField() (any, error) {
	b := d.Byte()
	switch b {
	case booleanNullValue:
		return nil, nil
	case booleanFalseValue:
		return false, nil
	default:
		return true, nil
	}
}

// RealField decodes a real field.
func (d *Decoder) RealField() (any, error) {
	v := d.Uint32()
	if v == realNullValue {
		return nil, nil
	}
	return float64(math.Float32frombits(v)), nil
}

// DoubleField decodes a double field.
func (d *Decoder) DoubleField() (any, error) {
	v := d.Uint64()
	if v == doubleNullValue {
		return nil, nil
	}
	return math.Float64frombits(v), nil
}

func (d *Decoder) decodeDate() (int, time.Month, int, bool) {
	// decode.
	/*
	   null values: most sig bit unset
	   year: unset second most sig bit (subtract 2^15)
	   --> read year as unsigned
	   month is 0-based
	   day is 1 byte.
	*/
	year := d.Uint16()
	null := ((year & 0x8000) == 0) // null value
	year &= 0x3fff
	month := d.Int8()
	month++
	day := d.Int8()
	return int(year), time.Month(month), int(day), null
}

// DateField decodes a date field.
func (d *Decoder) DateField() (any, error) {
	year, month, day, null := d.decodeDate()
	if null {
		return nil, nil
	}
	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC), nil
}

func (d *Decoder) decodeTime() (int, int, int, int, bool) {
	hour := d.Byte()
	null := (hour & 0x80) == 0 // null value
	hour &= 0x7f
	minute := d.Int8()
	msec := d.Uint16()

	sec := msec / 1000
	msec %= 1000
	nsec := int(msec) * 1000000

	return int(hour), int(minute), int(sec), nsec, null
}

// TimeField decodes a time field.
func (d *Decoder) TimeField() (any, error) {
	// time read gives only seconds (cut), no milliseconds
	hour, minute, sec, nsec, null := d.decodeTime()
	if null {
		return nil, nil
	}
	return time.Date(1, 1, 1, hour, minute, sec, nsec, time.UTC), nil
}

// TimestampField decodes a timestamp field.
func (d *Decoder) TimestampField() (any, error) {
	year, month, day, dateNull := d.decodeDate()
	hour, minute, sec, nsec, timeNull := d.decodeTime()
	if dateNull || timeNull {
		return nil, nil
	}
	return time.Date(year, month, day, hour, minute, sec, nsec, time.UTC), nil
}

// LongdateField decodes a longdate field.
func (d *Decoder) LongdateField() (any, error) {
	longdate := d.Int64()
	if longdate == longdateNullValue {
		return nil, nil
	}
	return convertLongdateToTime(longdate), nil
}

// SeconddateField decodes a seconddate field.
func (d *Decoder) SeconddateField() (any, error) {
	seconddate := d.Int64()
	if seconddate == seconddateNullValue {
		return nil, nil
	}
	return convertSeconddateToTime(seconddate), nil
}

// DaydateField decodes a daydate field.
func (d *Decoder) DaydateField(emptyDateAsNull bool) (any, error) {
	daydate := d.Int32()
	if daydate == daydateNullValue || (emptyDateAsNull && daydate == 0) {
		return nil, nil
	}
	return convertDaydateToTime(int64(daydate)), nil
}

// SecondtimeField decodes a secondtime field.
func (d *Decoder) SecondtimeField() (any, error) {
	secondtime := d.Int32()
	if secondtime == secondtimeNullValue {
		return nil, nil
	}
	return convertSecondtimeToTime(int(secondtime)), nil
}

// DecimalField decodes a decimal field.
func (d *Decoder) DecimalField() (any, error) {
	m, exp, err := d.Decimal()
	if err != nil {
		return nil, err
	}
	if m == nil {
		return nil, nil
	}
	return Decimal{m: m, exp: exp}, nil
}

func (d *Decoder) decodeFixed(size, scale int) (any, error) {
	if scale < 0 {
		panic(fmt.Sprintf("fixed: invalid scale %d", scale))
	}
	m := d.Fixed(size)
	if m == nil { // important: return nil and not m (as m is of type *big.Int)
		return nil, nil
	}
	return Decimal{m: m, exp: -scale}, nil
}

// Fixed8Field decodes a fixed8 field.
func (d *Decoder) Fixed8Field(scale int) (any, error) {
	if !d.Bool() { // null value
		return nil, nil
	}
	return d.decodeFixed(8, scale)
}

// Fixed12Field decodes a fixed12 field.
func (d *Decoder) Fixed12Field(scale int) (any, error) {
	if !d.Bool() { // null value
		return nil, nil
	}
	return d.decodeFixed(12, scale)
}

// Fixed16Field decodes a fixed16 field.
func (d *Decoder) Fixed16Field(scale int) (any, error) {
	if !d.Bool() { // null value
		return nil, nil
	}
	return d.decodeFixed(16, scale)
}

// VarField decodes a var field.
func (d *Decoder) VarField() (any, error) {
	_, b := d.LIBytes()
	/*
	   caution:
	   - result is used as driver.Value and we do need to provide a 'real' nil value
	   - returning b == nil does not work because b is of type []byte
	*/
	if b == nil {
		return nil, nil
	}
	return b, nil
}

// AlphanumField decodes an alphanum field.
func (d *Decoder) AlphanumField(alphanumDfv1 bool) (any, error) {
	if alphanumDfv1 { // like VarField
		return d.VarField()
	}
	_, b := d.LIBytes()
	/*
	   caution:
	   - result is used as driver.Value and we do need to provide a 'real' nil value
	   - returning b == nil does not work because b is of type []byte
	*/
	if b == nil {
		return nil, nil
	}
	/*
	   first byte:
	   - high bit set -> numeric
	   - high bit unset -> alpha
	   - bits 0-6: field size

	   ignore first byte for now
	*/
	if len(b) == 0 { // non-null indicator but no leading size byte: truncated payload
		panic(ShortBufferError{Need: 1, Have: 0})
	}
	return b[1:], nil
}

// Cesu8Field decodes a cesu8 field.
func (d *Decoder) Cesu8Field() (any, error) {
	_, b, err := d.CESU8LIBytes()
	if err != nil {
		return nil, err
	}
	/*
	   caution:
	   - result is used as driver.Value and we do need to provide a 'real' nil value
	   - returning b == nil does not work because b is of type []byte
	*/
	if b == nil {
		return nil, nil
	}
	return b, nil
}

// HexField decodes a hex field.
func (d *Decoder) HexField() (any, error) {
	_, b := d.LIBytes()
	/*
	   caution:
	   - result is used as driver.Value and we do need to provide a 'real' nil value
	   - returning b == nil does not work because b is of type []byte
	*/
	if b == nil {
		return nil, nil
	}
	return hex.EncodeToString(b), nil
}
