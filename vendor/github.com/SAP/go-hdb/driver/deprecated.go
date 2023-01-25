package driver

import (
	"database/sql"
	"database/sql/driver"
)

// NullTime represents an time.Time that may be null.
//
// Deprecated: Please use database/sql NullTime instead.
type NullTime = sql.NullTime

// deprecated driver interface methods
func (*conn) Prepare(query string) (driver.Stmt, error)                     { panic("deprecated") }
func (*conn) Begin() (driver.Tx, error)                                     { panic("deprecated") }
func (*conn) Exec(query string, args []driver.Value) (driver.Result, error) { panic("deprecated") }
func (*conn) Query(query string, args []driver.Value) (driver.Rows, error)  { panic("deprecated") }
func (*stmt) Exec(args []driver.Value) (driver.Result, error)               { panic("deprecated") }
func (*stmt) Query(args []driver.Value) (rows driver.Rows, err error)       { panic("deprecated") }
