package fixture

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"sync/atomic"
	"testing"
)

// SQLRows describes a fixed query result. Every query gets a fresh cursor;
// SQL text and arguments are deliberately not evaluated. Optional logs are
// intended for sequential tests and must not be read or written concurrently.
type SQLRows struct {
	Columns    []string
	Rows       [][]driver.Value
	TypeNames  []string
	QueryLog   *[]string
	ContextLog *[]context.Context
	Closes     *int32
}

// OpenSQLRows exposes fixture rows through real database/sql scanning without
// registering a global driver name. Configuration must remain immutable while
// the database is in use (except for the optional logs and close counter).
func OpenSQLRows(t testing.TB, result SQLRows) *sql.DB {
	t.Helper()
	db := sql.OpenDB(rowsConnector{result})
	closeDB(t, db)
	return db
}

// DriverRows converts int fixture values to int64 for database/sql drivers.
// Other values are retained verbatim, including nil and malformed blobs.
func DriverRows(rows [][]any) [][]driver.Value {
	out := make([][]driver.Value, len(rows))
	for i, row := range rows {
		out[i] = make([]driver.Value, len(row))
		for j, v := range row {
			if n, ok := v.(int); ok {
				out[i][j] = int64(n)
			} else {
				out[i][j] = v
			}
		}
	}
	return out
}

type rowsConnector struct{ result SQLRows }

func (c rowsConnector) Connect(context.Context) (driver.Conn, error) {
	return &rowsConn{c.result}, nil
}
func (c rowsConnector) Driver() driver.Driver { return rowsDriver(c) }

type rowsDriver struct{ result SQLRows }

func (d rowsDriver) Open(string) (driver.Conn, error) { return &rowsConn{d.result}, nil }

type rowsConn struct{ result SQLRows }

func (c *rowsConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("fixture: prepared statements not supported")
}
func (c *rowsConn) Close() error { return nil }
func (c *rowsConn) Begin() (driver.Tx, error) {
	return nil, errors.New("fixture: transactions not supported")
}
func (c *rowsConn) QueryContext(ctx context.Context, query string, _ []driver.NamedValue) (driver.Rows, error) {
	if c.result.QueryLog != nil {
		*c.result.QueryLog = append(*c.result.QueryLog, query)
	}
	if c.result.ContextLog != nil {
		*c.result.ContextLog = append(*c.result.ContextLog, ctx)
	}
	return &sqlRows{result: c.result}, nil
}

type sqlRows struct {
	result SQLRows
	next   int
}

func (r *sqlRows) Columns() []string { return r.result.Columns }
func (r *sqlRows) Close() error {
	if r.result.Closes != nil {
		atomic.AddInt32(r.result.Closes, 1)
	}
	return nil
}
func (r *sqlRows) ColumnTypeDatabaseTypeName(idx int) string {
	if idx >= 0 && idx < len(r.result.TypeNames) {
		return r.result.TypeNames[idx]
	}
	return ""
}
func (r *sqlRows) Next(dest []driver.Value) error {
	if r.next >= len(r.result.Rows) {
		return io.EOF
	}
	copy(dest, r.result.Rows[r.next])
	r.next++
	return nil
}
