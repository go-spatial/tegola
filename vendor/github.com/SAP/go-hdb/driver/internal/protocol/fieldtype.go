package protocol

import (
	"bytes"
	"database/sql/driver"
	"encoding/hex"
	"io"
	"math"
	"math/big"
	"reflect"
	"time"

	"golang.org/x/text/transform"

	"github.com/SAP/go-hdb/driver/internal/protocol/encoding"
	"github.com/SAP/go-hdb/driver/unicode/cesu8"
)

const (
	minTinyint  = 0
	maxTinyint  = math.MaxUint8
	minSmallint = math.MinInt16
	maxSmallint = math.MaxInt16
	minInteger  = math.MinInt32
	maxInteger  = math.MaxInt32
	minBigint   = math.MinInt64
	maxBigint   = math.MaxInt64
	maxReal     = math.MaxFloat32
	maxDouble   = math.MaxFloat64
)

const (
	realNullValue   uint32 = ^uint32(0)
	doubleNullValue uint64 = ^uint64(0)
)

const (
	booleanFalseValue   byte  = 0
	booleanNullValue    byte  = 1
	booleanTrueValue    byte  = 2
	longdateNullValue   int64 = 3155380704000000001
	seconddateNullValue int64 = 315538070401
	daydateNullValue    int32 = 3652062
	secondtimeNullValue int32 = 86402
)

// LocatorID represents a locotor id.
type LocatorID uint64 // byte[locatorIdSize]

var timeReflectType = reflect.TypeOf((*time.Time)(nil)).Elem()
var bytesReflectType = reflect.TypeOf((*[]byte)(nil)).Elem()
var stringReflectType = reflect.TypeOf((*string)(nil)).Elem()

const lobInputParametersSize = 9

type fieldConverter interface {
	convert(v any) (any, error)
}

type cesu8FieldConverter interface {
	convertCESU8(t transform.Transformer, v any) (any, error)
}

type fieldType interface {
	/*
		statements:
		- first parameter could be many
		- so the check needs to 'fail fast'
		- fmt.Errorf is too slow because contructor formats the error -> use ConvertError
	*/
	prmSize(v any) int
	encodePrm(e *encoding.Encoder, v any) error
	decodeRes(d *encoding.Decoder) (any, error)
	decodePrm(d *encoding.Decoder) (any, error)
}

var (
	booleanType    = _booleanType{}
	tinyintType    = _tinyintType{}
	smallintType   = _smallintType{}
	integerType    = _integerType{}
	bigintType     = _bigintType{}
	realType       = _realType{}
	doubleType     = _doubleType{}
	dateType       = _dateType{}
	timeType       = _timeType{}
	timestampType  = _timestampType{}
	longdateType   = _longdateType{}
	seconddateType = _seconddateType{}
	daydateType    = _daydateType{}
	secondtimeType = _secondtimeType{}
	decimalType    = _decimalType{}
	varType        = _varType{}
	alphaType      = _alphaType{}
	hexType        = _hexType{}
	cesu8Type      = _cesu8Type{}
	lobVarType     = _lobVarType{}
	lobCESU8Type   = _lobCESU8Type{}
)

type (
	_booleanType    struct{}
	_tinyintType    struct{}
	_smallintType   struct{}
	_integerType    struct{}
	_bigintType     struct{}
	_realType       struct{}
	_doubleType     struct{}
	_dateType       struct{}
	_timeType       struct{}
	_timestampType  struct{}
	_longdateType   struct{}
	_seconddateType struct{}
	_daydateType    struct{}
	_secondtimeType struct{}
	_decimalType    struct{}
	_fixed8Type     struct{ prec, scale int }
	_fixed12Type    struct{ prec, scale int }
	_fixed16Type    struct{ prec, scale int }
	_varType        struct{}
	_alphaType      struct{}
	_hexType        struct{}
	_cesu8Type      struct{}
	_lobVarType     struct{}
	_lobCESU8Type   struct{}
)

var (
	_ fieldType = (*_booleanType)(nil)
	_ fieldType = (*_tinyintType)(nil)
	_ fieldType = (*_smallintType)(nil)
	_ fieldType = (*_integerType)(nil)
	_ fieldType = (*_bigintType)(nil)
	_ fieldType = (*_realType)(nil)
	_ fieldType = (*_doubleType)(nil)
	_ fieldType = (*_dateType)(nil)
	_ fieldType = (*_timeType)(nil)
	_ fieldType = (*_timestampType)(nil)
	_ fieldType = (*_longdateType)(nil)
	_ fieldType = (*_seconddateType)(nil)
	_ fieldType = (*_daydateType)(nil)
	_ fieldType = (*_secondtimeType)(nil)
	_ fieldType = (*_decimalType)(nil)
	_ fieldType = (*_fixed8Type)(nil)
	_ fieldType = (*_fixed12Type)(nil)
	_ fieldType = (*_fixed16Type)(nil)
	_ fieldType = (*_varType)(nil)
	_ fieldType = (*_alphaType)(nil)
	_ fieldType = (*_hexType)(nil)
	_ fieldType = (*_cesu8Type)(nil)
	_ fieldType = (*_lobVarType)(nil)
	_ fieldType = (*_lobCESU8Type)(nil)
)

// stringer
func (_booleanType) String() string    { return "booleanType" }
func (_tinyintType) String() string    { return "tinyintType" }
func (_smallintType) String() string   { return "smallintType" }
func (_integerType) String() string    { return "integerType" }
func (_bigintType) String() string     { return "bigintType" }
func (_realType) String() string       { return "realType" }
func (_doubleType) String() string     { return "doubleType" }
func (_dateType) String() string       { return "dateType" }
func (_timeType) String() string       { return "timeType" }
func (_timestampType) String() string  { return "timestampType" }
func (_longdateType) String() string   { return "longdateType" }
func (_seconddateType) String() string { return "seconddateType" }
func (_daydateType) String() string    { return "daydateType" }
func (_secondtimeType) String() string { return "secondtimeType" }
func (_decimalType) String() string    { return "decimalType" }
func (_fixed8Type) String() string     { return "fixed8Type" }
func (_fixed12Type) String() string    { return "fixed12Type" }
func (_fixed16Type) String() string    { return "fixed16Type" }
func (_varType) String() string        { return "varType" }
func (_alphaType) String() string      { return "alphaType" }
func (_hexType) String() string        { return "hexType" }
func (_cesu8Type) String() string      { return "cesu8Type" }
func (_lobVarType) String() string     { return "lobVarType" }
func (_lobCESU8Type) String() string   { return "lobCESU8Type" }

// convert
func (ft _booleanType) convert(v any) (any, error) {
	return convertBool(ft, v)
}
func (ft _tinyintType) convert(v any) (any, error) {
	return convertInteger(ft, v, minTinyint, maxTinyint)
}
func (ft _smallintType) convert(v any) (any, error) {
	return convertInteger(ft, v, minSmallint, maxSmallint)
}
func (ft _integerType) convert(v any) (any, error) {
	return convertInteger(ft, v, minInteger, maxInteger)
}
func (ft _bigintType) convert(v any) (any, error) {
	return convertInteger(ft, v, minBigint, maxBigint)
}

func (ft _realType) convert(v any) (any, error) {
	return convertFloat(ft, v, maxReal)
}
func (ft _doubleType) convert(v any) (any, error) {
	return convertFloat(ft, v, maxDouble)
}

func (ft _dateType) convert(v any) (any, error) {
	return convertTime(ft, v)
}
func (ft _timeType) convert(v any) (any, error) {
	return convertTime(ft, v)
}
func (ft _timestampType) convert(v any) (any, error) {
	return convertTime(ft, v)
}
func (ft _longdateType) convert(v any) (any, error) {
	return convertTime(ft, v)
}
func (ft _seconddateType) convert(v any) (any, error) {
	return convertTime(ft, v)
}
func (ft _daydateType) convert(v any) (any, error) {
	return convertTime(ft, v)
}
func (ft _secondtimeType) convert(v any) (any, error) {
	return convertTime(ft, v)
}

func (ft _decimalType) convert(v any) (any, error) {
	return convertDecimal(ft, v)
}
func (ft _fixed8Type) convert(v any) (any, error) {
	return convertDecimal(ft, v)
}
func (ft _fixed12Type) convert(v any) (any, error) {
	return convertDecimal(ft, v)
}
func (ft _fixed16Type) convert(v any) (any, error) {
	return convertDecimal(ft, v)
}

func (ft _varType) convert(v any) (any, error) {
	return convertBytes(ft, v)
}
func (ft _alphaType) convert(v any) (any, error) {
	return convertBytes(ft, v)
}
func (ft _hexType) convert(v any) (any, error) {
	return convertBytes(ft, v)
}
func (ft _cesu8Type) convert(v any) (any, error) {
	return convertBytes(ft, v)
}

func (ft _lobVarType) convert(v any) (any, error) {
	return convertLob(nil, ft, v)
}
func (ft _lobCESU8Type) convertCESU8(t transform.Transformer, v any) (any, error) {
	return convertLob(t, ft, v)
}

// ReadProvider is the interface wrapping the Reader which provides an io.Reader.
type ReadProvider interface {
	Reader() io.Reader
}

// Lob
func convertLob(t transform.Transformer, ft fieldType, v any) (driver.Value, error) {
	if v == nil {
		return v, nil
	}

	var rd io.Reader

	switch v := v.(type) {
	case io.Reader:
		rd = v
	case ReadProvider:
		rd = v.Reader()
	case []byte:
		rd = bytes.NewReader(v)
	default:
		return nil, newConvertError(ft, v, nil)
	}

	if t != nil { // cesu8Encoder
		rd = transform.NewReader(rd, t)
	}

	return newLobInDescr(rd), nil
}

// prm size
func (_booleanType) prmSize(any) int    { return encoding.BooleanFieldSize }
func (_tinyintType) prmSize(any) int    { return encoding.TinyintFieldSize }
func (_smallintType) prmSize(any) int   { return encoding.SmallintFieldSize }
func (_integerType) prmSize(any) int    { return encoding.IntegerFieldSize }
func (_bigintType) prmSize(any) int     { return encoding.BigintFieldSize }
func (_realType) prmSize(any) int       { return encoding.RealFieldSize }
func (_doubleType) prmSize(any) int     { return encoding.DoubleFieldSize }
func (_dateType) prmSize(any) int       { return encoding.DateFieldSize }
func (_timeType) prmSize(any) int       { return encoding.TimeFieldSize }
func (_timestampType) prmSize(any) int  { return encoding.TimestampFieldSize }
func (_longdateType) prmSize(any) int   { return encoding.LongdateFieldSize }
func (_seconddateType) prmSize(any) int { return encoding.SeconddateFieldSize }
func (_daydateType) prmSize(any) int    { return encoding.DaydateFieldSize }
func (_secondtimeType) prmSize(any) int { return encoding.SecondtimeFieldSize }
func (_decimalType) prmSize(any) int    { return encoding.DecimalFieldSize }
func (_fixed8Type) prmSize(any) int     { return encoding.Fixed8FieldSize }
func (_fixed12Type) prmSize(any) int    { return encoding.Fixed12FieldSize }
func (_fixed16Type) prmSize(any) int    { return encoding.Fixed16FieldSize }
func (_lobVarType) prmSize(v any) int   { return lobInputParametersSize }
func (_lobCESU8Type) prmSize(v any) int { return lobInputParametersSize }

func (ft _varType) prmSize(v any) int {
	switch v := v.(type) {
	case []byte:
		return encoding.VarFieldSize(len(v))
	case string:
		return encoding.VarFieldSize(len(v))
	default:
		return -1
	}
}
func (ft _alphaType) prmSize(v any) int { return varType.prmSize(v) }

func (ft _hexType) prmSize(v any) int { return varType.prmSize(v) / 2 }

func (ft _cesu8Type) prmSize(v any) int {
	switch v := v.(type) {
	case []byte:
		return encoding.VarFieldSize(cesu8.Size(v))
	case string:
		return encoding.VarFieldSize(cesu8.StringSize(v))
	default:
		return -1
	}
}

// encode
func (ft _booleanType) encodePrm(e *encoding.Encoder, v any) error {
	if v == nil {
		e.Byte(booleanNullValue)
		return nil
	}
	b, ok := v.(bool)
	if !ok {
		panic("invalid bool value") // should never happen
	}
	if b {
		e.Byte(booleanTrueValue)
	} else {
		e.Byte(booleanFalseValue)
	}
	return nil
}

func (ft _tinyintType) encodePrm(e *encoding.Encoder, v any) error {
	e.Byte(byte(asInt64(v)))
	return nil
}
func (ft _smallintType) encodePrm(e *encoding.Encoder, v any) error {
	e.Int16(int16(asInt64(v)))
	return nil
}
func (ft _integerType) encodePrm(e *encoding.Encoder, v any) error {
	e.Int32(int32(asInt64(v)))
	return nil
}
func (ft _bigintType) encodePrm(e *encoding.Encoder, v any) error {
	e.Int64(asInt64(v))
	return nil
}

func asInt64(v any) int64 {
	switch v := v.(type) {
	case bool:
		if v {
			return 1
		}
		return 0
	}

	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return rv.Int()
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return int64(rv.Uint())
	default:
		panic("invalid bool value") // should never happen
	}
}

func (ft _realType) encodePrm(e *encoding.Encoder, v any) error {
	switch v := v.(type) {
	case float32:
		e.Float32(v)
	case float64:
		e.Float32(float32(v))
	default:
		panic("invalid real value") // should never happen
	}
	return nil
}

func (ft _doubleType) encodePrm(e *encoding.Encoder, v any) error {
	switch v := v.(type) {
	case float32:
		e.Float64(float64(v))
	case float64:
		e.Float64(v)
	default:
		panic("invalid double value") // should never happen
	}
	return nil
}

func (ft _dateType) encodePrm(e *encoding.Encoder, v any) error {
	encodeDate(e, asTime(v))
	return nil
}
func (ft _timeType) encodePrm(e *encoding.Encoder, v any) error {
	encodeTime(e, asTime(v))
	return nil
}
func (ft _timestampType) encodePrm(e *encoding.Encoder, v any) error {
	t := asTime(v)
	encodeDate(e, t)
	encodeTime(e, t)
	return nil
}

func encodeDate(e *encoding.Encoder, t time.Time) {
	// year: set most sig bit
	// month 0 based
	year, month, day := t.Date()
	e.Uint16(uint16(year) | 0x8000)
	e.Int8(int8(month) - 1)
	e.Int8(int8(day))
}

func encodeTime(e *encoding.Encoder, t time.Time) {
	e.Byte(byte(t.Hour()) | 0x80)
	e.Int8(int8(t.Minute()))
	msec := t.Second()*1000 + t.Nanosecond()/1000000
	e.Uint16(uint16(msec))
}

func (ft _longdateType) encodePrm(e *encoding.Encoder, v any) error {
	e.Int64(convertTimeToLongdate(asTime(v)))
	return nil
}
func (ft _seconddateType) encodePrm(e *encoding.Encoder, v any) error {
	e.Int64(convertTimeToSeconddate(asTime(v)))
	return nil
}
func (ft _daydateType) encodePrm(e *encoding.Encoder, v any) error {
	e.Int32(int32(convertTimeToDayDate(asTime(v))))
	return nil
}
func (ft _secondtimeType) encodePrm(e *encoding.Encoder, v any) error {
	if v == nil {
		e.Int32(secondtimeNullValue)
		return nil
	}
	e.Int32(int32(convertTimeToSecondtime(asTime(v))))
	return nil
}

func asTime(v any) time.Time {
	t, ok := v.(time.Time)
	if !ok {
		panic("invalid time value") // should never happen
	}
	//store in utc
	return t.UTC()
}

func (ft _decimalType) encodePrm(e *encoding.Encoder, v any) error {
	r, ok := v.(*big.Rat)
	if !ok {
		panic("invalid decimal value") // should never happen
	}

	var m big.Int
	exp, df := convertRatToDecimal(r, &m, dec128Digits, dec128MinExp, dec128MaxExp)

	if df&dfOverflow != 0 {
		return ErrDecimalOutOfRange
	}

	if df&dfUnderflow != 0 { // set to zero
		e.Decimal(natZero, 0)
	} else {
		e.Decimal(&m, exp)
	}
	return nil
}

func (ft _fixed8Type) encodePrm(e *encoding.Encoder, v any) error {
	return encodeFixed(e, v, encoding.Fixed8FieldSize, ft.prec, ft.scale)
}

func (ft _fixed12Type) encodePrm(e *encoding.Encoder, v any) error {
	return encodeFixed(e, v, encoding.Fixed12FieldSize, ft.prec, ft.scale)
}

func (ft _fixed16Type) encodePrm(e *encoding.Encoder, v any) error {
	return encodeFixed(e, v, encoding.Fixed16FieldSize, ft.prec, ft.scale)
}

func encodeFixed(e *encoding.Encoder, v any, size, prec, scale int) error {
	r, ok := v.(*big.Rat)
	if !ok {
		panic("invalid decimal value") // should never happen
	}

	var m big.Int
	df := convertRatToFixed(r, &m, prec, scale)

	if df&dfOverflow != 0 {
		return ErrDecimalOutOfRange
	}

	e.Fixed(&m, size)
	return nil
}

func (ft _varType) encodePrm(e *encoding.Encoder, v any) error {
	switch v := v.(type) {
	case []byte:
		return e.LIBytes(v)
	case string:
		return e.LIString(v)
	default:
		panic("invalid var value") // should never happen
	}
}
func (ft _alphaType) encodePrm(e *encoding.Encoder, v any) error {
	return varType.encodePrm(e, v)
}

func (ft _hexType) encodePrm(e *encoding.Encoder, v any) error {
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

func (ft _cesu8Type) encodePrm(e *encoding.Encoder, v any) error {
	switch v := v.(type) {
	case []byte:
		return e.CESU8LIBytes(v)
	case string:
		return e.CESU8LIString(v)
	default:
		panic("invalid cesu8 value") // should never happen
	}
}

func (ft _lobVarType) encodePrm(e *encoding.Encoder, v any) error {
	lobInDescr, ok := v.(*LobInDescr)
	if !ok {
		panic("invalid lob var value") // should never happen
	}
	return encodeLobPrm(e, lobInDescr)
}

func (ft _lobCESU8Type) encodePrm(e *encoding.Encoder, v any) error {
	lobInDescr, ok := v.(*LobInDescr)
	if !ok {
		panic("invalid lob cesu8 value") // should never happen
	}
	return encodeLobPrm(e, lobInDescr)
}

func encodeLobPrm(e *encoding.Encoder, descr *LobInDescr) error {
	e.Byte(byte(descr.opt))
	e.Int32(int32(len(descr.b)))
	e.Int32(int32(descr.pos))
	return nil
}

// field types for which decodePrm is same as decodeRes
func (ft _booleanType) decodePrm(d *encoding.Decoder) (any, error)    { return ft.decodeRes(d) }
func (ft _realType) decodePrm(d *encoding.Decoder) (any, error)       { return ft.decodeRes(d) }
func (ft _doubleType) decodePrm(d *encoding.Decoder) (any, error)     { return ft.decodeRes(d) }
func (ft _dateType) decodePrm(d *encoding.Decoder) (any, error)       { return ft.decodeRes(d) }
func (ft _timeType) decodePrm(d *encoding.Decoder) (any, error)       { return ft.decodeRes(d) }
func (ft _timestampType) decodePrm(d *encoding.Decoder) (any, error)  { return ft.decodeRes(d) }
func (ft _longdateType) decodePrm(d *encoding.Decoder) (any, error)   { return ft.decodeRes(d) }
func (ft _seconddateType) decodePrm(d *encoding.Decoder) (any, error) { return ft.decodeRes(d) }
func (ft _daydateType) decodePrm(d *encoding.Decoder) (any, error)    { return ft.decodeRes(d) }
func (ft _secondtimeType) decodePrm(d *encoding.Decoder) (any, error) { return ft.decodeRes(d) }
func (ft _decimalType) decodePrm(d *encoding.Decoder) (any, error)    { return ft.decodeRes(d) }
func (ft _fixed8Type) decodePrm(d *encoding.Decoder) (any, error)     { return ft.decodeRes(d) }
func (ft _fixed12Type) decodePrm(d *encoding.Decoder) (any, error)    { return ft.decodeRes(d) }
func (ft _fixed16Type) decodePrm(d *encoding.Decoder) (any, error)    { return ft.decodeRes(d) }
func (ft _varType) decodePrm(d *encoding.Decoder) (any, error)        { return ft.decodeRes(d) }
func (ft _alphaType) decodePrm(d *encoding.Decoder) (any, error)      { return ft.decodeRes(d) }
func (ft _hexType) decodePrm(d *encoding.Decoder) (any, error)        { return ft.decodeRes(d) }
func (ft _cesu8Type) decodePrm(d *encoding.Decoder) (any, error)      { return ft.decodeRes(d) }

// decode
func (_booleanType) decodeRes(d *encoding.Decoder) (any, error) {
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

func (_tinyintType) decodePrm(d *encoding.Decoder) (any, error) { return int64(d.Byte()), nil }
func (_smallintType) decodePrm(d *encoding.Decoder) (any, error) {
	return int64(d.Int16()), nil
}
func (_integerType) decodePrm(d *encoding.Decoder) (any, error) { return int64(d.Int32()), nil }
func (_bigintType) decodePrm(d *encoding.Decoder) (any, error)  { return d.Int64(), nil }

func (ft _tinyintType) decodeRes(d *encoding.Decoder) (any, error) {
	if !d.Bool() { //null value
		return nil, nil
	}
	return ft.decodePrm(d)
}
func (ft _smallintType) decodeRes(d *encoding.Decoder) (any, error) {
	if !d.Bool() { //null value
		return nil, nil
	}
	return ft.decodePrm(d)
}
func (ft _integerType) decodeRes(d *encoding.Decoder) (any, error) {
	if !d.Bool() { //null value
		return nil, nil
	}
	return ft.decodePrm(d)
}
func (ft _bigintType) decodeRes(d *encoding.Decoder) (any, error) {
	if !d.Bool() { //null value
		return nil, nil
	}
	return ft.decodePrm(d)
}

func (_realType) decodeRes(d *encoding.Decoder) (any, error) {
	v := d.Uint32()
	if v == realNullValue {
		return nil, nil
	}
	return float64(math.Float32frombits(v)), nil
}
func (_doubleType) decodeRes(d *encoding.Decoder) (any, error) {
	v := d.Uint64()
	if v == doubleNullValue {
		return nil, nil
	}
	return math.Float64frombits(v), nil
}

func (_dateType) decodeRes(d *encoding.Decoder) (any, error) {
	year, month, day, null := decodeDate(d)
	if null {
		return nil, nil
	}
	return time.Date(int(year), time.Month(month), int(day), 0, 0, 0, 0, time.UTC), nil
}
func (_timeType) decodeRes(d *encoding.Decoder) (any, error) {
	// time read gives only seconds (cut), no milliseconds
	hour, min, sec, nsec, null := decodeTime(d)
	if null {
		return nil, nil
	}
	return time.Date(1, 1, 1, hour, min, sec, nsec, time.UTC), nil
}
func (_timestampType) decodeRes(d *encoding.Decoder) (any, error) {
	year, month, day, dateNull := decodeDate(d)
	hour, min, sec, nsec, timeNull := decodeTime(d)
	if dateNull || timeNull {
		return nil, nil
	}
	return time.Date(year, month, day, hour, min, sec, nsec, time.UTC), nil
}

// null values: most sig bit unset
// year: unset second most sig bit (subtract 2^15)
// --> read year as unsigned
// month is 0-based
// day is 1 byte
func decodeDate(d *encoding.Decoder) (int, time.Month, int, bool) {
	year := d.Uint16()
	null := ((year & 0x8000) == 0) //null value
	year &= 0x3fff
	month := d.Int8()
	month++
	day := d.Int8()
	return int(year), time.Month(month), int(day), null
}

func decodeTime(d *encoding.Decoder) (int, int, int, int, bool) {
	hour := d.Byte()
	null := (hour & 0x80) == 0 //null value
	hour &= 0x7f
	min := d.Int8()
	msec := d.Uint16()

	sec := msec / 1000
	msec %= 1000
	nsec := int(msec) * 1000000

	return int(hour), int(min), int(sec), nsec, null
}

func (_longdateType) decodeRes(d *encoding.Decoder) (any, error) {
	longdate := d.Int64()
	if longdate == longdateNullValue {
		return nil, nil
	}
	return convertLongdateToTime(longdate), nil
}
func (_seconddateType) decodeRes(d *encoding.Decoder) (any, error) {
	seconddate := d.Int64()
	if seconddate == seconddateNullValue {
		return nil, nil
	}
	return convertSeconddateToTime(seconddate), nil
}
func (_daydateType) decodeRes(d *encoding.Decoder) (any, error) {
	daydate := d.Int32()
	if daydate == daydateNullValue {
		return nil, nil
	}
	return convertDaydateToTime(int64(daydate)), nil
}
func (_secondtimeType) decodeRes(d *encoding.Decoder) (any, error) {
	secondtime := d.Int32()
	if secondtime == secondtimeNullValue {
		return nil, nil
	}
	return convertSecondtimeToTime(int(secondtime)), nil
}

func (_decimalType) decodeRes(d *encoding.Decoder) (any, error) {
	m, exp, err := d.Decimal()
	if err != nil {
		return nil, err
	}
	if m == nil {
		return nil, nil
	}
	return convertDecimalToRat(m, exp), nil
}

func (ft _fixed8Type) decodeRes(d *encoding.Decoder) (any, error) {
	if !d.Bool() { //null value
		return nil, nil
	}
	return decodeFixed(d, encoding.Fixed8FieldSize, ft.prec, ft.scale)
}
func (ft _fixed12Type) decodeRes(d *encoding.Decoder) (any, error) {
	if !d.Bool() { //null value
		return nil, nil
	}
	return decodeFixed(d, encoding.Fixed12FieldSize, ft.prec, ft.scale)
}
func (ft _fixed16Type) decodeRes(d *encoding.Decoder) (any, error) {
	if !d.Bool() { //null value
		return nil, nil
	}
	return decodeFixed(d, encoding.Fixed16FieldSize, ft.prec, ft.scale)
}

func decodeFixed(d *encoding.Decoder, size, prec, scale int) (any, error) {
	m := d.Fixed(size)
	if m == nil { // important: return nil and not m (as m is of type *big.Int)
		return nil, nil
	}
	return convertFixedToRat(m, scale), nil
}

func (_varType) decodeRes(d *encoding.Decoder) (any, error) {
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

func (_alphaType) decodeRes(d *encoding.Decoder) (any, error) {
	_, b := d.LIBytes()
	/*
	   caution:
	   - result is used as driver.Value and we do need to provide a 'real' nil value
	   - returning b == nil does not work because b is of type []byte
	*/
	if b == nil {
		return nil, nil
	}
	if d.Dfv() == DfvLevel1 { // like _varType
		return b, nil
	}
	/*
		first byte:
		- high bit set -> numeric
		- high bit unset -> alpha
		- bits 0-6: field size

		ignore first byte for now
	*/
	return b[1:], nil
}

func (_hexType) decodeRes(d *encoding.Decoder) (any, error) {
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

func (_cesu8Type) decodeRes(d *encoding.Decoder) (any, error) {
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

func decodeLobPrm(d *encoding.Decoder) (any, error) {
	descr := &LobInDescr{}
	descr.opt = LobOptions(d.Byte())
	descr._size = int(d.Int32())
	descr.pos = int(d.Int32())
	return nil, nil
}

func (_lobVarType) decodePrm(d *encoding.Decoder) (any, error) {
	return decodeLobPrm(d)
}
func (_lobCESU8Type) decodePrm(d *encoding.Decoder) (any, error) {
	return decodeLobPrm(d)
}

func decodeLobRes(d *encoding.Decoder, isCharBased bool) (any, error) {
	descr := &LobOutDescr{IsCharBased: isCharBased}
	descr.ltc = lobTypecode(d.Int8())
	descr.Opt = LobOptions(d.Int8())
	if descr.Opt.isNull() {
		return nil, nil
	}
	d.Skip(2)
	descr.NumChar = d.Int64()
	descr.numByte = d.Int64()
	descr.ID = LocatorID(d.Uint64())
	size := int(d.Int32())
	descr.B = make([]byte, size)
	d.Bytes(descr.B)
	return descr, nil
}

func (_lobVarType) decodeRes(d *encoding.Decoder) (any, error) {
	return decodeLobRes(d, false)
}
func (_lobCESU8Type) decodeRes(d *encoding.Decoder) (any, error) {
	return decodeLobRes(d, true)
}
