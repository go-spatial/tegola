package protocol

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"math"
	"math/big"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/SAP/go-hdb/driver/internal/protocol/encoding"
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

var (
	timeReflectType   = reflect.TypeFor[time.Time]()
	bytesReflectType  = reflect.TypeFor[[]byte]()
	stringReflectType = reflect.TypeFor[string]()
	ratReflectType    = reflect.TypeFor[big.Rat]()
)

var (
	errConversionNotSupported = errors.New("conversion not supported")
	errUint64OutOfRange       = errors.New("uint64 values with high bit set are not supported")
	errIntegerOutOfRange      = errors.New("integer out of range")
	errFloatOutOfRange        = errors.New("float out of range")
)

/*
Conversion routines hdb parameters
  - return value is any to avoid allocations in case
    parameter is already of target type
*/

func convertBool(v any) (any, error) {
	// check needs to be done on each type individually as if combining types in one case
	// the v type stays on any and the comparison v != 0 would always be true.
	switch v := v.(type) {
	case bool:
		return v, nil
	case int:
		return v != 0, nil
	case int8:
		return v != 0, nil
	case int16:
		return v != 0, nil
	case int32:
		return v != 0, nil
	case int64:
		return v != 0, nil
	case uint:
		return v != 0, nil
	case uint8:
		return v != 0, nil
	case uint16:
		return v != 0, nil
	case uint32:
		return v != 0, nil
	case uint64:
		return v != 0, nil
	case float32:
		return v != 0, nil
	case float64:
		return v != 0, nil
	case string:
		return strconv.ParseBool(v)
	}

	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Bool:
		return rv.Bool(), nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return rv.Int() != 0, nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return rv.Uint() != 0, nil
	case reflect.Float32, reflect.Float64:
		return rv.Float() != 0, nil
	case reflect.String:
		return strconv.ParseBool(rv.String())
	case reflect.Pointer:
		if rv.IsNil() {
			return nil, nil
		}
		return convertBool(rv.Elem().Interface())
	default:
		if rv.Type().ConvertibleTo(stringReflectType) {
			return convertBool(rv.Convert(stringReflectType).Interface())
		}
		return nil, errConversionNotSupported
	}
}

var (
	i64Zero = int64(0)
	i64One  = int64(1)
)

func convertInteger(v any, minI64, maxI64 int64) (any, error) { //nolint: gocyclo
	switch v := v.(type) {
	case bool:
		if v {
			return i64One, nil
		}
		return i64Zero, nil
	case int:
		i64 := int64(v)
		if i64 > maxI64 || i64 < minI64 {
			return nil, errIntegerOutOfRange
		}
		return i64, nil
	case int8:
		i64 := int64(v)
		if i64 > maxI64 || i64 < minI64 {
			return nil, errIntegerOutOfRange
		}
		return i64, nil
	case int16:
		i64 := int64(v)
		if i64 > maxI64 || i64 < minI64 {
			return nil, errIntegerOutOfRange
		}
		return i64, nil
	case int32:
		i64 := int64(v)
		if i64 > maxI64 || i64 < minI64 {
			return nil, errIntegerOutOfRange
		}
		return i64, nil
	case int64:
		if v > maxI64 || v < minI64 {
			return nil, errIntegerOutOfRange
		}
		return v, nil
	case uint:
		u64 := uint64(v)
		if u64 > math.MaxInt64 {
			return nil, errUint64OutOfRange
		}
		i64 := int64(u64)
		if i64 > maxI64 || i64 < minI64 {
			return nil, errIntegerOutOfRange
		}
		return i64, nil
	case uint8:
		i64 := int64(v)
		if i64 > maxI64 || i64 < minI64 {
			return nil, errIntegerOutOfRange
		}
		return i64, nil
	case uint16:
		i64 := int64(v)
		if i64 > maxI64 || i64 < minI64 {
			return nil, errIntegerOutOfRange
		}
		return i64, nil
	case uint32:
		i64 := int64(v)
		if i64 > maxI64 || i64 < minI64 {
			return nil, errIntegerOutOfRange
		}
		return i64, nil
	case uint64:
		if v > math.MaxInt64 {
			return nil, errUint64OutOfRange
		}
		i64 := int64(v)
		if i64 > maxI64 || i64 < minI64 {
			return nil, errIntegerOutOfRange
		}
		return i64, nil
	case float32:
		i64 := int64(v)
		if v != float32(i64) { // should work for overflow, NaN, +-INF as well
			return nil, errConversionNotSupported
		}
		if i64 > maxI64 || i64 < minI64 {
			return nil, errIntegerOutOfRange
		}
		return i64, nil
	case float64:
		i64 := int64(v)
		if v != float64(i64) { // should work for overflow, NaN, +-INF as well
			return nil, errConversionNotSupported
		}
		if i64 > maxI64 || i64 < minI64 {
			return nil, errIntegerOutOfRange
		}
		return i64, nil
	case string:
		i64, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return nil, err
		}
		if i64 > maxI64 || i64 < minI64 {
			return nil, errIntegerOutOfRange
		}
		return i64, nil
	}

	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Bool:
		if rv.Bool() {
			return i64One, nil
		}
		return i64Zero, nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		i64 := rv.Int()
		if i64 > maxI64 || i64 < minI64 {
			return nil, errIntegerOutOfRange
		}
		return i64, nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32:
		i64 := int64(rv.Uint()) //nolint: gosec
		if i64 > maxI64 || i64 < minI64 {
			return nil, errIntegerOutOfRange
		}
		return i64, nil
	case reflect.Uint64:
		u64 := rv.Uint()
		if u64 > math.MaxInt64 {
			return nil, errUint64OutOfRange
		}
		i64 := int64(u64)
		if i64 > maxI64 || i64 < minI64 {
			return nil, errIntegerOutOfRange
		}
		return i64, nil
	case reflect.Float32, reflect.Float64:
		f64 := rv.Float()
		i64 := int64(f64)
		if f64 != float64(i64) { // should work for overflow, NaN, +-INF as well
			return nil, errConversionNotSupported
		}
		if i64 > maxI64 || i64 < minI64 {
			return nil, errIntegerOutOfRange
		}
		return i64, nil
	case reflect.String:
		i64, err := strconv.ParseInt(rv.String(), 10, 64)
		if err != nil {
			return nil, errConversionNotSupported
		}
		if i64 > maxI64 || i64 < minI64 {
			return nil, errIntegerOutOfRange
		}
		return i64, nil
	case reflect.Pointer:
		if rv.IsNil() {
			return nil, nil
		}
		return convertInteger(rv.Elem().Interface(), minI64, maxI64)
	default:
		if rv.Type().ConvertibleTo(stringReflectType) {
			return convertInteger(rv.Convert(stringReflectType).Interface(), minI64, maxI64)
		}
		return nil, errConversionNotSupported
	}
}

var (
	f64Zero = float64(0.0)
	f64One  = float64(1.0)
)

func convertFloat(v any, maxF64 float64) (any, error) { //nolint: gocyclo
	switch v := v.(type) {
	case float32:
		f64 := float64(v)
		if math.Abs(f64) > maxF64 {
			return nil, errFloatOutOfRange
		}
		return f64, nil
	case float64:
		if math.Abs(v) > maxF64 {
			return nil, errFloatOutOfRange
		}
		return v, nil
	case bool:
		if v {
			return f64One, nil
		}
		return f64Zero, nil
	case int:
		f64 := float64(v)
		if math.Abs(f64) > maxF64 {
			return nil, errFloatOutOfRange
		}
		return f64, nil
	case int8:
		f64 := float64(v)
		if math.Abs(f64) > maxF64 {
			return nil, errFloatOutOfRange
		}
		return f64, nil
	case int16:
		f64 := float64(v)
		if math.Abs(f64) > maxF64 {
			return nil, errFloatOutOfRange
		}
		return f64, nil
	case int32:
		f64 := float64(v)
		if math.Abs(f64) > maxF64 {
			return nil, errFloatOutOfRange
		}
		return f64, nil
	case int64:
		f64 := float64(v)
		if math.Abs(f64) > maxF64 {
			return nil, errFloatOutOfRange
		}
		return f64, nil
	case uint:
		f64 := float64(v)
		if math.Abs(f64) > maxF64 {
			return nil, errFloatOutOfRange
		}
		return f64, nil
	case uint8:
		f64 := float64(v)
		if math.Abs(f64) > maxF64 {
			return nil, errFloatOutOfRange
		}
		return f64, nil
	case uint16:
		f64 := float64(v)
		if math.Abs(f64) > maxF64 {
			return nil, errFloatOutOfRange
		}
		return f64, nil
	case uint32:
		f64 := float64(v)
		if math.Abs(f64) > maxF64 {
			return nil, errFloatOutOfRange
		}
		return f64, nil
	case uint64:
		f64 := float64(v)
		if math.Abs(f64) > maxF64 {
			return nil, errFloatOutOfRange
		}
		return f64, nil
	case string:
		f64, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return nil, err
		}
		if math.Abs(f64) > maxF64 {
			return nil, errFloatOutOfRange
		}
		return f64, nil
	}

	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Bool:
		if rv.Bool() {
			return f64One, nil
		}
		return f64Zero, nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		f64 := float64(rv.Int())
		if math.Abs(f64) > maxF64 {
			return nil, errFloatOutOfRange
		}
		return f64, nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		f64 := float64(rv.Uint())
		if math.Abs(f64) > maxF64 {
			return nil, errFloatOutOfRange
		}
		return f64, nil
	case reflect.Float32, reflect.Float64:
		f64 := rv.Float()
		if math.Abs(f64) > maxF64 {
			return nil, errFloatOutOfRange
		}
		return f64, nil
	case reflect.String:
		f64, err := strconv.ParseFloat(rv.String(), 64)
		if err != nil {
			return nil, err
		}
		if math.Abs(f64) > maxF64 {
			return nil, errFloatOutOfRange
		}
		return f64, nil
	case reflect.Pointer:
		if rv.IsNil() {
			return nil, nil
		}
		return convertFloat(rv.Elem().Interface(), maxF64)
	default:
		if rv.Type().ConvertibleTo(stringReflectType) {
			return convertFloat(rv.Convert(stringReflectType).Interface(), maxF64)
		}
		return nil, errConversionNotSupported
	}
}

func convertTime(v any) (any, error) {
	if v, ok := v.(time.Time); ok {
		return v, nil
	}

	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Pointer:
		if rv.IsNil() {
			return nil, nil
		}
		return convertTime(rv.Elem().Interface())
	default:
		if rv.Type().ConvertibleTo(timeReflectType) {
			tv := rv.Convert(timeReflectType)
			return tv.Interface().(time.Time), nil
		}
		return nil, errConversionNotSupported
	}
}

var (
	ratZero = big.NewRat(0, 1)
	ratOne  = big.NewRat(1, 1)
)

// decimalDecompose is the database/sql decimal decompose interface (input side).
type decimalDecompose interface {
	Decompose(buf []byte) (form byte, negative bool, coefficient []byte, exponent int32)
}

// convertDecimal and convertFixed are intentional copy-and-paste of each other,
// differing only in the target constructor (decimal vs fixed). They sit on the
// parameter hot path - a single bulk insert calls them once per decimal/fixed
// cell (numRow * numCols times). Deduplicating would add per-call cost we want
// to avoid here, so the duplication is deliberate.
// NOTE: keep both functions in sync - a change in one (added input type, changed
// routing) almost always needs the same change in the other.
func convertDecimal(v any) (any, error) { //nolint: gocyclo
	switch v := v.(type) {
	case decimalDecompose:
		form, negative, coefficient, exponent := v.Decompose(nil)
		return encoding.NewDecimalFromDecompose(form, negative, coefficient, exponent)
	case *big.Int:
		return encoding.NewDecimalFromRat(new(big.Rat).SetInt(v))
	case *big.Rat:
		return encoding.NewDecimalFromRat(v)
	case *big.Float:
		r, _ := v.Rat(nil) // ignore accuracy
		return encoding.NewDecimalFromRat(r)
	case bool:
		if v {
			return encoding.NewDecimalFromRat(ratOne)
		}
		return encoding.NewDecimalFromRat(ratZero)
	case int:
		return encoding.NewDecimalFromRat(new(big.Rat).SetInt64(int64(v)))
	case int8:
		return encoding.NewDecimalFromRat(new(big.Rat).SetInt64(int64(v)))
	case int16:
		return encoding.NewDecimalFromRat(new(big.Rat).SetInt64(int64(v)))
	case int32:
		return encoding.NewDecimalFromRat(new(big.Rat).SetInt64(int64(v)))
	case int64:
		return encoding.NewDecimalFromRat(new(big.Rat).SetInt64(v))
	case uint:
		return encoding.NewDecimalFromRat(new(big.Rat).SetUint64(uint64(v)))
	case uint8:
		return encoding.NewDecimalFromRat(new(big.Rat).SetUint64(uint64(v)))
	case uint16:
		return encoding.NewDecimalFromRat(new(big.Rat).SetUint64(uint64(v)))
	case uint32:
		return encoding.NewDecimalFromRat(new(big.Rat).SetUint64(uint64(v)))
	case uint64:
		return encoding.NewDecimalFromRat(new(big.Rat).SetUint64(v))
	case float32:
		r := new(big.Rat).SetFloat64(float64(v))
		if r == nil {
			return nil, errConversionNotSupported
		}
		return encoding.NewDecimalFromRat(r)
	case float64:
		r := new(big.Rat).SetFloat64(v)
		if r == nil {
			return nil, errConversionNotSupported
		}
		return encoding.NewDecimalFromRat(r)
	case string:
		r, ok := new(big.Rat).SetString(v)
		if !ok {
			return nil, errConversionNotSupported
		}
		return encoding.NewDecimalFromRat(r)
	}

	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Bool:
		if rv.Bool() {
			return encoding.NewDecimalFromRat(ratOne)
		}
		return encoding.NewDecimalFromRat(ratZero)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return encoding.NewDecimalFromRat(new(big.Rat).SetInt64(rv.Int()))
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return encoding.NewDecimalFromRat(new(big.Rat).SetUint64(rv.Uint()))
	case reflect.Float32, reflect.Float64:
		r := new(big.Rat).SetFloat64(rv.Float())
		if r == nil {
			return nil, errConversionNotSupported
		}
		return encoding.NewDecimalFromRat(r)
	case reflect.String:
		r, ok := new(big.Rat).SetString(rv.String())
		if !ok {
			return nil, errConversionNotSupported
		}
		return encoding.NewDecimalFromRat(r)
	case reflect.Pointer:
		if rv.IsNil() {
			return nil, nil
		}
		return convertDecimal(rv.Elem().Interface())
	default:
		if rv.Type().ConvertibleTo(ratReflectType) {
			tv := rv.Convert(ratReflectType)
			r := tv.Interface().(big.Rat)
			return encoding.NewDecimalFromRat(&r)
		}
		return nil, errConversionNotSupported
	}
}

func convertFixed(v any, prec, scale int) (any, error) { //nolint: gocyclo
	switch v := v.(type) {
	case decimalDecompose:
		form, negative, coefficient, exponent := v.Decompose(nil)
		return encoding.NewFixedFromDecompose(form, negative, coefficient, exponent, prec, scale)
	case *big.Int:
		return encoding.NewFixedFromRat(new(big.Rat).SetInt(v), prec, scale)
	case *big.Rat:
		return encoding.NewFixedFromRat(v, prec, scale)
	case *big.Float:
		r, _ := v.Rat(nil) // ignore accuracy
		return encoding.NewFixedFromRat(r, prec, scale)
	case bool:
		if v {
			return encoding.NewFixedFromRat(ratOne, prec, scale)
		}
		return encoding.NewFixedFromRat(ratZero, prec, scale)
	case int:
		return encoding.NewFixedFromRat(new(big.Rat).SetInt64(int64(v)), prec, scale)
	case int8:
		return encoding.NewFixedFromRat(new(big.Rat).SetInt64(int64(v)), prec, scale)
	case int16:
		return encoding.NewFixedFromRat(new(big.Rat).SetInt64(int64(v)), prec, scale)
	case int32:
		return encoding.NewFixedFromRat(new(big.Rat).SetInt64(int64(v)), prec, scale)
	case int64:
		return encoding.NewFixedFromRat(new(big.Rat).SetInt64(v), prec, scale)
	case uint:
		return encoding.NewFixedFromRat(new(big.Rat).SetUint64(uint64(v)), prec, scale)
	case uint8:
		return encoding.NewFixedFromRat(new(big.Rat).SetUint64(uint64(v)), prec, scale)
	case uint16:
		return encoding.NewFixedFromRat(new(big.Rat).SetUint64(uint64(v)), prec, scale)
	case uint32:
		return encoding.NewFixedFromRat(new(big.Rat).SetUint64(uint64(v)), prec, scale)
	case uint64:
		return encoding.NewFixedFromRat(new(big.Rat).SetUint64(v), prec, scale)
	case float32:
		r := new(big.Rat).SetFloat64(float64(v))
		if r == nil {
			return nil, errConversionNotSupported
		}
		return encoding.NewFixedFromRat(r, prec, scale)
	case float64:
		r := new(big.Rat).SetFloat64(v)
		if r == nil {
			return nil, errConversionNotSupported
		}
		return encoding.NewFixedFromRat(r, prec, scale)
	case string:
		r, ok := new(big.Rat).SetString(v)
		if !ok {
			return nil, errConversionNotSupported
		}
		return encoding.NewFixedFromRat(r, prec, scale)
	}

	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Bool:
		if rv.Bool() {
			return encoding.NewFixedFromRat(ratOne, prec, scale)
		}
		return encoding.NewFixedFromRat(ratZero, prec, scale)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return encoding.NewFixedFromRat(new(big.Rat).SetInt64(rv.Int()), prec, scale)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return encoding.NewFixedFromRat(new(big.Rat).SetUint64(rv.Uint()), prec, scale)
	case reflect.Float32, reflect.Float64:
		r := new(big.Rat).SetFloat64(rv.Float())
		if r == nil {
			return nil, errConversionNotSupported
		}
		return encoding.NewFixedFromRat(r, prec, scale)
	case reflect.String:
		r, ok := new(big.Rat).SetString(rv.String())
		if !ok {
			return nil, errConversionNotSupported
		}
		return encoding.NewFixedFromRat(r, prec, scale)
	case reflect.Pointer:
		if rv.IsNil() {
			return nil, nil
		}
		return convertFixed(rv.Elem().Interface(), prec, scale)
	default:
		if rv.Type().ConvertibleTo(ratReflectType) {
			tv := rv.Convert(ratReflectType)
			r := tv.Interface().(big.Rat)
			return encoding.NewFixedFromRat(&r, prec, scale)
		}
		return nil, errConversionNotSupported
	}
}

func convertBytes(v any) (any, error) {
	switch v := v.(type) {
	case string, []byte:
		return v, nil
	}

	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.String:
		return rv.String(), nil
	case reflect.Pointer:
		if rv.IsNil() {
			return nil, nil
		}
		return convertBytes(rv.Elem().Interface())
	case reflect.Slice:
		if rv.Type() == bytesReflectType {
			return rv.Bytes(), nil
		}
		fallthrough
	default:
		if rv.Type().ConvertibleTo(bytesReflectType) {
			bv := rv.Convert(bytesReflectType)
			return bv.Interface().([]byte), nil
		}
		return nil, errConversionNotSupported
	}
}

// readProvider is the interface wrapping the Reader which provides an io.Reader.
type readProvider interface {
	Reader() io.Reader
}

func convertLob(v any, cesu8 bool) (any, error) {
	// All lob input values are normalized to an io.Reader here and later streamed
	// in chunks (transcoded on the fly for N-LOBs, see LobInDescr.FetchFirst).
	//
	// TODO (fast path, optional perf): when the value already fits in a single
	// chunk, we could transcode/write it whole via the scalar CESU8 primitives
	// and skip the reader wrap + the extra writeLobs round-trip. For string/[]byte
	// the (CESU-8) size is known up front (cesu8.StringSize / cesu8.Size); for a
	// bare io.Reader it is only known after the first fetch. Mind the == chunkSize
	// boundary (it must count as last-data) and keep the wire bytes / descriptor
	// size+opt identical to the streamed path.
	var rd io.Reader = nil
	switch v := v.(type) {
	case io.Reader:
		rd = v
	case readProvider:
		rd = v.Reader()
	default:
		// check if string or []byte
		if v, err := convertBytes(v); err == nil {
			switch v := v.(type) {
			case string:
				rd = strings.NewReader(v)
			case []byte:
				rd = bytes.NewReader(v)
			}
		}
	}
	if rd != nil {
		return newLobInDescr(rd, cesu8), nil
	}

	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Pointer:
		if rv.IsNil() {
			return nil, nil
		}
		return convertLob(rv.Elem().Interface(), cesu8)
	default:
		return nil, errConversionNotSupported
	}
}

func convertField(tc typeCode, v any, prec, scale int) (any, error) {
	if v == nil {
		return nil, nil
	}

	switch tc {
	case tcBoolean:
		return convertBool(v)
	case tcTinyint:
		return convertInteger(v, minTinyint, maxTinyint)
	case tcSmallint:
		return convertInteger(v, minSmallint, maxSmallint)
	case tcInteger:
		return convertInteger(v, minInteger, maxInteger)
	case tcBigint:
		return convertInteger(v, minBigint, maxBigint)
	case tcReal:
		return convertFloat(v, maxReal)
	case tcDouble:
		return convertFloat(v, maxDouble)
	case tcDate, tcTime, tcTimestamp, tcLongdate, tcSeconddate, tcDaydate, tcSecondtime:
		return convertTime(v)
	case tcDecimal:
		return convertDecimal(v)
	case tcFixed8, tcFixed12, tcFixed16:
		return convertFixed(v, prec, scale)
	case tcChar, tcVarchar, tcString, tcBstring, tcAlphanum, tcNchar, tcNvarchar, tcNstring, tcShorttext, tcBinary, tcVarbinary, tcStPoint, tcStGeometry:
		return convertBytes(v)
	case tcBlob, tcClob, tcLocator:
		return convertLob(v, false)
	case tcNclob, tcText, tcNlocator:
		return convertLob(v, true)
	case tcBintext: // ?? lobCESU8Type
		return convertLob(v, false)
	default:
		panic(fmt.Errorf("invalid type code %[1]d %[1]s", tc)) // should never happen
	}
}
