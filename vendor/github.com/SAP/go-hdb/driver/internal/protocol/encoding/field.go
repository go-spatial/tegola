package encoding

const (
	booleanFalseValue byte = 0
	booleanNullValue  byte = 1
	booleanTrueValue  byte = 2
)

const (
	realNullValue   uint32 = ^uint32(0)
	doubleNullValue uint64 = ^uint64(0)
)

const (
	longdateNullValue   int64 = 3155380704000000001
	seconddateNullValue int64 = 315538070401
	daydateNullValue    int32 = 3652062
	secondtimeNullValue int32 = 86402
)

// IntegerFieldSize constant.
const IntegerFieldSize = 4

// string / binary length indicators.
const (
	varFieldLenIndNullValue byte = 255
	varFieldLenIndSmall     byte = 245
	varFieldLenIndMedium    byte = 246
	varFieldLenIndBig       byte = 247
)
