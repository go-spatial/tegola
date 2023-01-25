package encoding

import "math"

// Filed size constants.
const (
	BooleanFieldSize    = 1
	TinyintFieldSize    = 1
	SmallintFieldSize   = 2
	IntegerFieldSize    = 4
	BigintFieldSize     = 8
	RealFieldSize       = 4
	DoubleFieldSize     = 8
	DateFieldSize       = 4
	TimeFieldSize       = 4
	TimestampFieldSize  = DateFieldSize + TimeFieldSize
	LongdateFieldSize   = 8
	SeconddateFieldSize = 8
	DaydateFieldSize    = 4
	SecondtimeFieldSize = 4
	DecimalFieldSize    = 16
	Fixed8FieldSize     = 8
	Fixed12FieldSize    = 12
	Fixed16FieldSize    = 16
)

// string / binary length indicators
const (
	bytesLenIndNullValue byte = 255
	bytesLenIndSmall     byte = 245
	bytesLenIndMedium    byte = 246
	bytesLenIndBig       byte = 247
)

// VarFieldSize returns the size of a varible field variable ([]byte, string and unicode variants).
func VarFieldSize(size int) int {
	switch {
	default:
		return -1
	case size <= int(bytesLenIndSmall):
		return size + 1
	case size <= math.MaxInt16:
		return size + 3
	case size <= math.MaxInt32:
		return size + 5
	}
}
