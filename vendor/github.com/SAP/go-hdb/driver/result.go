package driver

import (
	"database/sql/driver"
	"fmt"
	"io"
	"reflect"

	p "github.com/SAP/go-hdb/driver/internal/protocol"
)

// check if rows types do implement all driver row interfaces.
var (
	_ driver.Rows = (*noResultType)(nil)

	_ driver.Rows                           = (*queryResult)(nil)
	_ driver.RowsColumnTypeDatabaseTypeName = (*queryResult)(nil)
	_ driver.RowsColumnTypeLength           = (*queryResult)(nil)
	_ driver.RowsColumnTypeNullable         = (*queryResult)(nil)
	_ driver.RowsColumnTypePrecisionScale   = (*queryResult)(nil)
	_ driver.RowsColumnTypeScanType         = (*queryResult)(nil)
	_ driver.RowsNextResultSet              = (*queryResult)(nil)

	_ driver.Rows                           = (*callResult)(nil)
	_ driver.RowsColumnTypeDatabaseTypeName = (*callResult)(nil)
	_ driver.RowsColumnTypeLength           = (*callResult)(nil)
	_ driver.RowsColumnTypeNullable         = (*callResult)(nil)
	_ driver.RowsColumnTypePrecisionScale   = (*callResult)(nil)
	_ driver.RowsColumnTypeScanType         = (*callResult)(nil)
	_ driver.RowsNextResultSet              = (*callResult)(nil)
)

type prepareResult struct {
	fc              p.FunctionCode
	stmtID          uint64
	parameterFields []*p.ParameterField
	resultFields    []*p.ResultField
}

// check checks consistency of the prepare result.
func (pr *prepareResult) check(qd *queryDescr) error {
	call := qd.kind == qkCall
	if call != pr.fc.IsProcedureCall() {
		return fmt.Errorf("function code mismatch: query descriptor %s - function code %s", qd.kind, pr.fc)
	}

	if !call {
		// only input parameters allowed
		for _, f := range pr.parameterFields {
			if f.Out() {
				return fmt.Errorf("invalid parameter %s", f)
			}
		}
	}
	return nil
}

// isProcedureCall returns true if the statement is a call statement.
func (pr *prepareResult) isProcedureCall() bool { return pr.fc.IsProcedureCall() }

// numField returns the number of parameter fields in a database statement.
func (pr *prepareResult) numField() int { return len(pr.parameterFields) }

// numInputField returns the number of input fields in a database statement.
func (pr *prepareResult) numInputField() int {
	if !pr.fc.IsProcedureCall() {
		return len(pr.parameterFields) // only input fields
	}
	numField := 0
	for _, f := range pr.parameterFields {
		if f.In() {
			numField++
		}
	}
	return numField
}

// parameterField returns the parameter field at index idx.
func (pr *prepareResult) parameterField(idx int) *p.ParameterField {
	return pr.parameterFields[idx]
}

// onCloser defines getter and setter for a function which should be called when closing.
type onCloser interface {
	onClose() func()
	setOnClose(func())
}

// NoResult is the driver.Rows drop-in replacement if driver Query or QueryRow is used for statements that do not return rows.
var noResult = new(noResultType)

var noColumns = []string{}

type noResultType struct{}

func (r *noResultType) Columns() []string              { return noColumns }
func (r *noResultType) Close() error                   { return nil }
func (r *noResultType) Next(dest []driver.Value) error { return io.EOF }

// queryResult represents the resultset of a query.
type queryResult struct {
	// field alignment
	fields       []*p.ResultField
	fieldValues  []driver.Value
	decodeErrors p.DecodeErrors
	_columns     []string
	lastErr      error
	conn         *conn
	rsID         uint64
	pos          int
	_onClose     func()
	attributes   p.PartAttributes
	closed       bool
}

// onClose implements the onCloser interface
func (qr *queryResult) onClose() func() { return qr._onClose }

// setOnClose implements the onCloser interface
func (qr *queryResult) setOnClose(f func()) { qr._onClose = f }

// Columns implements the driver.Rows interface.
func (qr *queryResult) Columns() []string {
	if qr._columns == nil {
		numField := len(qr.fields)
		qr._columns = make([]string, numField)
		for i := 0; i < numField; i++ {
			qr._columns[i] = qr.fields[i].Name()
		}
	}
	return qr._columns
}

// Close implements the driver.Rows interface.
func (qr *queryResult) Close() error {
	if !qr.closed && qr._onClose != nil {
		defer qr._onClose()
	}
	qr.closed = true

	if qr.attributes.ResultsetClosed() {
		return nil
	}
	// if lastError is set, attrs are nil
	if qr.lastErr != nil {
		return qr.lastErr
	}
	return qr.conn._closeResultsetID(qr.rsID)
}

func (qr *queryResult) numRow() int {
	if len(qr.fieldValues) == 0 {
		return 0
	}
	return len(qr.fieldValues) / len(qr.fields)
}

func (qr *queryResult) copyRow(idx int, dest []driver.Value) {
	cols := len(qr.fields)
	copy(dest, qr.fieldValues[idx*cols:(idx+1)*cols])
}

// Next implements the driver.Rows interface.
func (qr *queryResult) Next(dest []driver.Value) error {
	if qr.pos >= qr.numRow() {
		if qr.attributes.LastPacket() {
			return io.EOF
		}
		if err := qr.conn._fetchNext(qr); err != nil {
			qr.lastErr = err //fieldValues and attrs are nil
			return err
		}
		if qr.numRow() == 0 {
			return io.EOF
		}
		qr.pos = 0
	}

	qr.copyRow(qr.pos, dest)
	err := qr.decodeErrors.RowError(qr.pos)
	qr.pos++

	for _, v := range dest {
		if v, ok := v.(p.LobDecoderSetter); ok {
			v.SetDecoder(qr.conn.decodeLob)
		}
	}
	return err
}

// ColumnTypeDatabaseTypeName implements the driver.RowsColumnTypeDatabaseTypeName interface.
func (qr *queryResult) ColumnTypeDatabaseTypeName(idx int) string { return qr.fields[idx].TypeName() }

// ColumnTypeLength implements the driver.RowsColumnTypeLength interface.
func (qr *queryResult) ColumnTypeLength(idx int) (int64, bool) { return qr.fields[idx].TypeLength() }

// ColumnTypeNullable implements the driver.RowsColumnTypeNullable interface.
func (qr *queryResult) ColumnTypeNullable(idx int) (bool, bool) {
	return qr.fields[idx].Nullable(), true
}

// ColumnTypePrecisionScale implements the driver.RowsColumnTypePrecisionScale interface.
func (qr *queryResult) ColumnTypePrecisionScale(idx int) (int64, int64, bool) {
	return qr.fields[idx].TypePrecisionScale()
}

// ColumnTypeScanType implements the driver.RowsColumnTypeScanType interface.
func (qr *queryResult) ColumnTypeScanType(idx int) reflect.Type {
	return qr.fields[idx].ScanType()
}

/*
driver.RowsNextResultSet:
- currently not used
- could be implemented as pointer to next queryResult (advancing by copying data from next)
*/

// HasNextResultSet implements the driver.RowsNextResultSet interface.
func (qr *queryResult) HasNextResultSet() bool { return false }

// NextResultSet implements the driver.RowsNextResultSet interface.
func (qr *queryResult) NextResultSet() error { return io.EOF }

type callResult struct { // call output parameters
	conn         *conn
	outputFields []*p.ParameterField
	fieldValues  []driver.Value
	decodeErrors p.DecodeErrors
	_columns     []string
	qrs          []*queryResult // table output parameters
	eof          bool
	closed       bool
	_onClose     func()
}

// onClose implements the onCloser interface
func (cr *callResult) onClose() func() { return cr._onClose }

// setOnClose implements the onCloser interface
func (cr *callResult) setOnClose(f func()) { cr._onClose = f }

// Columns implements the driver.Rows interface.
func (cr *callResult) Columns() []string {
	if cr._columns == nil {
		numField := len(cr.outputFields)
		cr._columns = make([]string, numField)
		for i := 0; i < numField; i++ {
			cr._columns[i] = cr.outputFields[i].Name()
		}
	}
	return cr._columns
}

// / Next implements the driver.Rows interface.
func (cr *callResult) Next(dest []driver.Value) error {
	if len(cr.fieldValues) == 0 || cr.eof {
		return io.EOF
	}

	copy(dest, cr.fieldValues)
	err := cr.decodeErrors.RowError(0)
	cr.eof = true
	for _, v := range dest {
		if v, ok := v.(p.LobDecoderSetter); ok {
			v.SetDecoder(cr.conn.decodeLob)
		}
	}
	return err
}

// Close implements the driver.Rows interface.
func (cr *callResult) Close() error {
	if !cr.closed && cr._onClose != nil {
		cr._onClose()
	}
	cr.closed = true
	return nil
}

// ColumnTypeDatabaseTypeName implements the driver.RowsColumnTypeDatabaseTypeName interface.
func (cr *callResult) ColumnTypeDatabaseTypeName(idx int) string {
	return cr.outputFields[idx].TypeName()
}

// ColumnTypeLength implements the driver.RowsColumnTypeLength interface.
func (cr *callResult) ColumnTypeLength(idx int) (int64, bool) {
	return cr.outputFields[idx].TypeLength()
}

// ColumnTypeNullable implements the driver.RowsColumnTypeNullable interface.
func (cr *callResult) ColumnTypeNullable(idx int) (bool, bool) {
	return cr.outputFields[idx].Nullable(), true
}

// ColumnTypePrecisionScale implements the driver.RowsColumnTypePrecisionScale interface.
func (cr *callResult) ColumnTypePrecisionScale(idx int) (int64, int64, bool) {
	return cr.outputFields[idx].TypePrecisionScale()
}

// ColumnTypeScanType implements the driver.RowsColumnTypeScanType interface.
func (cr *callResult) ColumnTypeScanType(idx int) reflect.Type {
	return cr.outputFields[idx].ScanType()
}

/*
driver.RowsNextResultSet:
- currently not used
- could be implemented as pointer to next queryResult (advancing by copying data from next)
*/

// HasNextResultSet implements the driver.RowsNextResultSet interface.
func (cr *callResult) HasNextResultSet() bool { return false }

// NextResultSet implements the driver.RowsNextResultSet interface.
func (cr *callResult) NextResultSet() error { return io.EOF }

func (cr *callResult) appendTableRefFields() {
	for i, qr := range cr.qrs {
		cr.outputFields = append(cr.outputFields, p.NewTableRefParameterField(i))
		cr.fieldValues = append(cr.fieldValues, encodeID(qr.rsID))
	}
}

func (cr *callResult) appendTableRowsFields() {
	for i, qr := range cr.qrs {
		cr.outputFields = append(cr.outputFields, p.NewTableRowsParameterField(i))
		cr.fieldValues = append(cr.fieldValues, qr)
	}
}
