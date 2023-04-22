package protocol

import (
	"database/sql"
	"fmt"
	"reflect"
	"time"
)

// DataType is the type definition for data types supported by this package.
type DataType byte

// Data type constants.
const (
	DtUnknown DataType = iota // unknown data type
	DtBoolean
	DtTinyint
	DtSmallint
	DtInteger
	DtBigint
	DtReal
	DtDouble
	DtDecimal
	DtTime
	DtString
	DtBytes
	DtLob
	DtRows
)

// RegisterScanType registers driver owned datatype scantypes (e.g. Decimal, Lob).
func RegisterScanType(dt DataType, scanType reflect.Type) bool {
	scanTypes[dt] = scanType
	return true
}

var scanTypes = []reflect.Type{
	DtUnknown:  reflect.TypeOf((*any)(nil)).Elem(),
	DtBoolean:  reflect.TypeOf((*bool)(nil)).Elem(),
	DtTinyint:  reflect.TypeOf((*uint8)(nil)).Elem(),
	DtSmallint: reflect.TypeOf((*int16)(nil)).Elem(),
	DtInteger:  reflect.TypeOf((*int32)(nil)).Elem(),
	DtBigint:   reflect.TypeOf((*int64)(nil)).Elem(),
	DtReal:     reflect.TypeOf((*float32)(nil)).Elem(),
	DtDouble:   reflect.TypeOf((*float64)(nil)).Elem(),
	DtTime:     reflect.TypeOf((*time.Time)(nil)).Elem(),
	DtString:   reflect.TypeOf((*string)(nil)).Elem(),
	DtBytes:    reflect.TypeOf((*[]byte)(nil)).Elem(),
	DtDecimal:  nil, // to be registered by driver
	DtLob:      nil, // to be registered by driver
	DtRows:     reflect.TypeOf((*sql.Rows)(nil)).Elem(),
}

// ScanType return the scan type (reflect.Type) of the corresponding data type.
func (dt DataType) ScanType() reflect.Type {
	st := scanTypes[dt]
	if st == nil {
		panic(fmt.Sprintf("ScanType for DataType %s not registered", dt))
	}
	return st
}
