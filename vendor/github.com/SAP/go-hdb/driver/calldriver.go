package driver

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"sync"
)

// Boilerplate to define a minimal sql driver implementation.
// To be used for converting stored procedure output parameters
// including sql.Rows output table parameters to guarantee
// exactly the same conversion behavior as for sql.Query.
var (
	_ driver.Driver            = (*callDriver)(nil)
	_ driver.Connector         = (*callConnector)(nil)
	_ driver.Conn              = (*callConn)(nil)
	_ driver.NamedValueChecker = (*callConn)(nil)
	_ driver.QueryerContext    = (*callConn)(nil)
)

type callDriver struct{}

var (
	defCallDriver = &callDriver{}
	defCallConn   = &callConn{}
)

func (d *callDriver) Open(name string) (driver.Conn, error) { return defCallConn, nil }

type callConnector struct{}

func (c *callConnector) Connect(context.Context) (driver.Conn, error) { return defCallConn, nil }
func (c *callConnector) Driver() driver.Driver                        { return defCallDriver }

type callConn struct{}

func (c *callConn) Prepare(query string) (driver.Stmt, error)   { panic("not implemented") }
func (c *callConn) Close() error                                { return nil }
func (c *callConn) Begin() (driver.Tx, error)                   { panic("not implemented") }
func (c *callConn) CheckNamedValue(nv *driver.NamedValue) error { return nil }

// QueryContext is used to convert the stored procedure output parameters.
func (c *callConn) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	if len(args) != 1 {
		return nil, fmt.Errorf("invalid argument length %d - expected 1", len(args))
	}
	cr, ok := args[0].Value.(*callResult)
	if !ok {
		return nil, fmt.Errorf("invalid argument type %T", args[0])
	}
	return cr, nil
}

// callDB supplies the process-global scratch database/sql DB used to convert
// stored procedure output parameters (including table output) with exactly the
// same conversion behavior as a regular sql.Query (see callConn.QueryContext).
// The scratch DB is built on the minimal call driver stack above: it is
// created lazily on first use, and its lifetime follows the driver connection
// count - it stays alive while at least one connection is open and is closed
// when the last connection goes away, since a closed *sql.DB cannot be reused.
type callDB struct {
	mu  sync.Mutex
	n   int64
	_db *sql.DB
}

// stdCallDB is the process-global scratch DB for stored procedure output
// parameter conversion.
var stdCallDB = &callDB{}

// incrConn accounts a new driver connection.
func (c *callDB) incrConn() { c.mu.Lock(); c.n++; c.mu.Unlock() }

// decrConn releases a driver connection. When the last connection is closed,
// the scratch DB is closed as well - sql.DB.Close terminates the
// connectionOpener goroutine spawned by sql.OpenDB - and reset, so the next
// db() call builds a fresh one.
func (c *callDB) decrConn() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.n--
	if c.n > 0 {
		return
	}
	c.n = 0
	if c._db != nil {
		c._db.Close()
		c._db = nil
	}
}

// db returns the scratch DB, creating it on first use.
func (c *callDB) db() *sql.DB {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c._db == nil {
		c._db = sql.OpenDB(new(callConnector))
	}
	return c._db
}
