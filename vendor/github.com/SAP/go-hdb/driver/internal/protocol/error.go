package protocol

import (
	"fmt"

	"github.com/SAP/go-hdb/driver/internal/protocol/encoding"
)

// ErrorLevel send from database server.
type errorLevel int8

func (e errorLevel) String() string {
	switch e {
	case 0:
		return "Warning"
	case 1:
		return "Error"
	case 2:
		return "Fatal Error"
	default:
		return ""
	}
}

// HDB error level constants.
const (
	errorLevelWarning    errorLevel = 0
	errorLevelError      errorLevel = 1
	errorLevelFatalError errorLevel = 2
)

const (
	sqlStateSize = 5
	// bytes of fix length fields mod 8
	// - errorCode = 4, errorPosition = 4, errortextLength = 4, errorLevel = 1, sqlState = 5 => 18 bytes
	// - 18 mod 8 = 2
	fixLength = 2
)

// HANA Database errors.
const (
	HdbErrAuthenticationFailed = 10
)

type sqlState [sqlStateSize]byte

type hdbError struct {
	errorCode       int32
	errorPosition   int32
	errorTextLength int32
	errorLevel      errorLevel
	sqlState        sqlState
	stmtNo          int
	errorText       []byte
}

func (e *hdbError) String() string {
	return fmt.Sprintf("errorCode %d errorPosition %d errorTextLength %d errorLevel %s sqlState %s stmtNo %d errorText %s",
		e.errorCode,
		e.errorPosition,
		e.errorTextLength,
		e.errorLevel,
		e.sqlState,
		e.stmtNo,
		e.errorText,
	)
}

func (e *hdbError) Error() string {
	if e.stmtNo != -1 {
		return fmt.Sprintf("SQL %s %d - %s (statement no: %d)", e.errorLevel, e.errorCode, e.errorText, e.stmtNo)
	}
	return fmt.Sprintf("SQL %s %d - %s", e.errorLevel, e.errorCode, e.errorText)
}

// HdbErrors represent the collection of errors return by the server.
type HdbErrors struct {
	errors []*hdbError
	//numArg int
	idx int
}

func (e *HdbErrors) String() string { return e.errors[e.idx].String() }
func (e *HdbErrors) Error() string  { return e.errors[e.idx].Error() }

// ErrorsFunc executes fn on all hdb errors.
func (e *HdbErrors) ErrorsFunc(fn func(err error)) {
	for _, err := range e.errors {
		fn(err)
	}
}

// NumError implements the driver.Error interface.
func (e *HdbErrors) NumError() int { return len(e.errors) }

// SetIdx implements the driver.Error interface.
func (e *HdbErrors) SetIdx(idx int) {
	numError := e.NumError()
	switch {
	case idx < 0:
		e.idx = 0
	case idx >= numError:
		e.idx = numError - 1
	default:
		e.idx = idx
	}
}

// StmtNo implements the driver.Error interface.
func (e *HdbErrors) StmtNo() int { return e.errors[e.idx].stmtNo }

// Code implements the driver.Error interface.
func (e *HdbErrors) Code() int { return int(e.errors[e.idx].errorCode) }

// Position implements the driver.Error interface.
func (e *HdbErrors) Position() int { return int(e.errors[e.idx].errorPosition) }

// Level implements the driver.Error interface.
func (e *HdbErrors) Level() int { return int(e.errors[e.idx].errorLevel) }

// Text implements the driver.Error interface.
func (e *HdbErrors) Text() string { return string(e.errors[e.idx].errorText) }

// IsWarning implements the driver.Error interface.
func (e *HdbErrors) IsWarning() bool { return e.errors[e.idx].errorLevel == errorLevelWarning }

// IsError implements the driver.Error interface.
func (e *HdbErrors) IsError() bool { return e.errors[e.idx].errorLevel == errorLevelError }

// IsFatal implements the driver.Error interface.
func (e *HdbErrors) IsFatal() bool { return e.errors[e.idx].errorLevel == errorLevelFatalError }

// SetStmtNo sets the statement number of the error.
func (e *HdbErrors) SetStmtNo(idx, no int) {
	if idx >= 0 && idx < e.NumError() {
		e.errors[idx].stmtNo = no
	}
}

// SetStmtsNoOfs adds an offset to the statement numbers of the errors (bulk operations).
func (e *HdbErrors) SetStmtsNoOfs(ofs int) {
	for _, hdbErr := range e.errors {
		hdbErr.stmtNo += ofs
	}
}

// HasWarnings returns true if the error collection contains warnings, false otherwise.
func (e *HdbErrors) HasWarnings() bool {
	for _, err := range e.errors {
		if err.errorLevel != errorLevelWarning {
			return false
		}
	}
	return true
}

func (e *HdbErrors) decode(dec *encoding.Decoder, ph *PartHeader) error {
	e.idx = 0
	e.errors = resizeSlice(e.errors, ph.numArg())

	numArg := ph.numArg()
	for i := 0; i < numArg; i++ {
		err := e.errors[i]
		if err == nil {
			err = new(hdbError)
			e.errors[i] = err
		}

		// err.stmtNo = -1
		err.stmtNo = 0
		/*
			in case of an hdb error when inserting one record (e.g. duplicate)
			- hdb does not return a rowsAffected part
			- SetStmtNo is not called and
			- the default value (formerly -1) is kept
			--> initialize stmtNo with zero
		*/
		err.errorCode = dec.Int32()
		err.errorPosition = dec.Int32()
		err.errorTextLength = dec.Int32()
		err.errorLevel = errorLevel(dec.Int8())
		dec.Bytes(err.sqlState[:])

		// read error text as ASCII data as some errors return invalid CESU-8 characters
		// e.g: SQL HdbError 7 - feature not supported: invalid character encoding: <invaid CESU-8 characters>
		//	if e.errorText, err = rd.ReadCesu8(int(e.errorTextLength)); err != nil {
		//		return err
		//	}
		err.errorText = make([]byte, int(err.errorTextLength))
		dec.Bytes(err.errorText)

		if numArg == 1 {
			// Error (protocol error?):
			// if only one error (numArg == 1): s.ph.bufferLength is one byte greater than data to be read
			// if more than one error: s.ph.bufferlength matches read bytes + padding
			//
			// Examples:
			// driver test TestHDBWarning
			//   --> 18 bytes fix error bytes + 103 bytes error text => 121 bytes (7 bytes padding needed)
			//   but s.ph.bufferLength = 122 (standard padding would only consume 6 bytes instead of 7)
			// driver test TestBulkInsertDuplicates
			//   --> returns 3 errors (number of total bytes matches s.ph.bufferLength)
			dec.Skip(1)
			break
		}

		pad := padBytes(int(fixLength + err.errorTextLength))
		if pad != 0 {
			dec.Skip(pad)
		}
	}
	return dec.Error()
}
