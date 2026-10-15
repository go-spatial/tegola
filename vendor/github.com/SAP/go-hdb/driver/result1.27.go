//go:build go1.27

package driver

import (
	"context"
	"database/sql/driver"
	"fmt"
)

// check if rows types do implement the driver.RowsColumnScanner interface.
var (
	_ driver.RowsColumnScanner = (*queryResult)(nil)
	_ driver.RowsColumnScanner = (*queryMultiResult)(nil)
	_ driver.RowsColumnScanner = (*callResult)(nil)
)

// NextRow implements the driver.RowsColumnScanner interface.
func (qr *queryResult) NextRow() error {
	row, err := qr.nextRow()
	if err != nil {
		return err
	}
	return qr.decodeErrors.RowErrors(row)
}

// ScanColumn implements the driver.RowsColumnScanner interface.
func (qr *queryResult) ScanColumn(scanCtx driver.ScanContext, index int, dest any) error {
	row := qr.pos - 1
	fieldValue := qr.fieldValues[row*len(qr.fields)+index]
	return scanColumn(scanCtx, qr.session, qr.fields[index].IsLob(), dest, fieldValue)
}

// NextRow implements the driver.RowsColumnScanner interface.
func (qmr *queryMultiResult) NextRow() error { return qmr.qrs[qmr.idx].NextRow() }

// ScanColumn implements the driver.RowsColumnScanner interface.
func (qmr *queryMultiResult) ScanColumn(scanCtx driver.ScanContext, index int, dest any) error {
	return qmr.qrs[qmr.idx].ScanColumn(scanCtx, index, dest)
}

// NextRow implements the driver.RowsColumnScanner interface.
func (cr *callResult) NextRow() error {
	if err := cr.nextRow(); err != nil {
		return err
	}
	return cr.decodeErrors.RowErrors(0)
}

// ScanColumn implements the driver.RowsColumnScanner interface.
func (cr *callResult) ScanColumn(scanCtx driver.ScanContext, index int, dest any) error {
	return scanColumn(scanCtx, cr.session, cr.outFields[index].IsLob(), dest, cr.fieldValues[index])
}

// convertCallResult converts the stored procedure scalar output parameters.
func convertCallResult(cr *callResult, scanArgs []any) error {
	// table output fields without call arguments are converted into *sql.Rows destinations
	// which require the parent *Rows context provided by database/sql - use the call driver
	// in this case.
	for _, fieldValue := range cr.fieldValues {
		if _, isTable := fieldValue.(*queryResult); isTable {
			return stdCallDB.db().QueryRowContext(context.Background(), "", cr).Scan(scanArgs...)
		}
	}

	for i, dest := range scanArgs {
		if err := scanColumn(driver.ScanContext{}, cr.session, cr.outFields[i].IsLob(), dest, cr.fieldValues[i]); err != nil {
			return fmt.Errorf("sql: Scan error on column index %d, name %q: %w", i, cr.outFields[i].Name(), err)
		}
	}
	return nil
}
