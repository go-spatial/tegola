package driver

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"log/slog"
	"reflect"
	"sync"
	"sync/atomic"
	"time"

	p "github.com/SAP/go-hdb/driver/internal/protocol"
)

// ErrUnsupportedIsolationLevel is the error raised if a transaction is started with a not supported isolation level.
var ErrUnsupportedIsolationLevel = errors.New("unsupported isolation level")

// ErrNestedTransaction is the error raised if a transaction is created within a transaction as this is not supported by hdb.
var ErrNestedTransaction = errors.New("nested transactions are not supported")

// ErrNestedQuery is deprecated, so currently not used (raised as an error) by the driver.
var ErrNestedQuery = errors.New("nested sql queries are not supported") // deprecated

// errInvalidLobLocatorID is the error raised if HANA DB returns error 1033 (invalid lob locator id).
// Currently this only can happen if stream enabled fields (LOBs) are part of the resultset.
// case 1:
// - a new sql statement is sent to the database server before the resultset processing of a previous sql query statement is finalized.
// case 2:
// - procedure call with table result set (out parameter) and lob(s) part of resultset.
// In cases 1 and 2 this error can be avoided by using a transaction (sql.Tx) for the query or exec statement.
var errInvalidLobLocatorID = errors.New("invalid lob locator id - please use a transaction on the query or exec statement")

// queries.
const (
	pingQuery                       = "select 1 from dummy"
	setIsolationLevelReadCommitted  = "set transaction isolation level read committed"
	setIsolationLevelRepeatableRead = "set transaction isolation level repeatable read"
	setIsolationLevelSerializable   = "set transaction isolation level serializable"
	setAccessModeReadOnly           = "set transaction read only"
	setAccessModeReadWrite          = "set transaction read write"
)

var (
	// register as var to execute even before init functions are called.
	_ = p.RegisterScanType(p.DtBytes, reflect.TypeFor[[]byte](), reflect.TypeFor[NullBytes]())
	_ = p.RegisterScanType(p.DtDecimal, reflect.TypeFor[Decimal](), reflect.TypeFor[NullDecimal]())
	_ = p.RegisterScanType(p.DtLob, reflect.TypeFor[Lob](), reflect.TypeFor[NullLob]())
)

// check if conn implements all required interfaces.
var (
	_ driver.Conn               = (*conn)(nil)
	_ driver.ConnPrepareContext = (*conn)(nil)
	_ driver.Pinger             = (*conn)(nil)
	_ driver.ConnBeginTx        = (*conn)(nil)
	_ driver.ExecerContext      = (*conn)(nil)
	_ driver.QueryerContext     = (*conn)(nil)
	_ driver.NamedValueChecker  = (*conn)(nil)
	_ driver.SessionResetter    = (*conn)(nil)
	_ driver.Validator          = (*conn)(nil)
	_ Conn                      = (*conn)(nil) // go-hdb enhancements
)

// connection hook for testing.
// use unexported type to avoid key collisions.
type connHookCtxKeyType struct{}

var connHookCtxKey connHookCtxKeyType

// ...connection hook operations.
const (
	choNone = iota
	choStmtExec
)

// ...connection hook function.
type connHookFn func(op int)

func withConnHook(ctx context.Context, fn connHookFn) context.Context {
	return context.WithValue(ctx, connHookCtxKey, fn)
}

// Conn enhances a connection with go-hdb specific connection functions.
type Conn interface {
	HDBVersion() *Version
	DatabaseName() string
	DBConnectInfo(ctx context.Context, databaseName string) (*DBConnectInfo, error)
}

// tableOutTracker retires the connection of a procedure call with table
// output parameters once its rows are no longer needed. It is created only on
// the successful call, simultaneously owning the fake parent rows (see
// stmt.execCall); on any error path no tracker exists, the resultsets close
// without accounting, and never hold the connection.
//
// The caller assigns the tracker to each table resultset and increments n once
// per resultset after the call succeeded, and registers it on the connection
// (conn.tableOutTracker). The registration order matters: the tracker must reach
// the connection only after the call succeeded, so no validation runs before
// every table rowset is tracked.
//
// Discard hands the tracker to the lifecycle worker (`Close` queues it -- the sole
// handoff, validated or not). The worker finalizes the tracker and pools the
// connection for reuse; detached at queue time, so a reused connection
// never carries a stale tracker. While the tracker is set, app-driven statement
// closes are no-ops (see stmt.Close).
type tableOutTracker struct {
	stmt *stmt          // the connection is reached through the statement (stmt.conn)
	rows *sql.Rows      // fake parent rows, closed inline by IsValid or by the lifecycle worker
	lc   *connLifecycle // worker to wake when the last table rowset closes (see release)
	done chan struct{}  // closed once the worker finalizes the tracker (see process)
	n    atomic.Int64   // table resultsets not yet closed
}

func (tc *tableOutTracker) release() {
	if tc.n.Add(-1) == 0 {
		tc.lc.wake() // last table rowset closed: recheck pending immediately
	}
}

// Conn is the implementation of the database/sql/driver Conn interface.
type conn struct {
	attrs     *connAttrs
	metrics   *metrics
	logger    *slog.Logger
	dbConn    dbConn
	session   *session
	wg        *sync.WaitGroup // wait for concurrent db calls when closing connections.
	lifecycle *connLifecycle  // lifecycle of the owning connector (see newConn).

	// closed guards close against repeated teardown.
	closed atomic.Bool

	// tableOutTracker is set by stmt.execCall when a procedure call hands table
	// output rows to the caller. While set, it holds the connection: statement
	// closes are no-ops and Close stands down. Discard hands the tracker to the
	// lifecycle worker, which pools the connection for reuse. Detached at pool
	// handoff (`get`), so a reused connection never carries a stale tracker.
	tableOutTracker *tableOutTracker
}

// unique connection number.
var connNo atomic.Uint64

// newConn returns a connection bound to the owning connector's lifecycle.
func newConn(ctx context.Context, host string, metrics *metrics, routing *routing, attrs *connAttrs, lifecycle *connLifecycle) (*conn, error) {
	logger := attrs.logger.With(slog.Uint64("conn", connNo.Add(1)))

	metrics.incrConn()

	dbConn, err := newDBConn(ctx, logger, host, metrics, attrs)
	if err != nil {
		metrics.decrConn()
		return nil, err
	}

	session, err := newSession(ctx, dbConn, logger, metrics, routing, attrs)
	if err != nil {
		dbConn.Close()
		metrics.decrConn()
		return nil, err
	}

	stdCallDB.incrConn()
	metrics.addGauge(gaugeConn, 1) // increment open connections.

	c := &conn{attrs: attrs, metrics: metrics, logger: logger, dbConn: dbConn, session: session, wg: new(sync.WaitGroup), lifecycle: lifecycle}
	return c, nil
}

func (c *conn) authenticate(ctx context.Context, host string, authHnd *p.AuthHnd) error {
	return c.session.authenticate(ctx, host, authHnd)
}

// Close implements the driver.Conn interface. A discarded connection with an
// attached tracker hands it to the lifecycle worker; teardown runs there.
// Discard always runs Close, validated or not.
func (c *conn) Close() error {
	if c.tableOutTracker != nil {
		c.lifecycle.queue(c.tableOutTracker)
		c.tableOutTracker = nil
		return nil
	}
	return c.close()
}

// close performs the real connection teardown.
func (c *conn) close() error {
	if !c.closed.CompareAndSwap(false, true) {
		return nil
	}
	c.metrics.addGauge(gaugeConn, -1) // decrement open connections.
	stdCallDB.decrConn()
	sessionErr := c.session.close()
	dbConnErr := c.dbConn.Close()
	c.wg.Wait()
	c.metrics.decrConn()
	return errors.Join(sessionErr, dbConnErr)
}

// terminateSession marks the session as canceled and requests the
// asynchronous termination of the server session.
func (c *conn) terminateSession() {
	c.session.cancel()
	c.lifecycle.terminate(c.session.host, c.session.serverConnID)
}

// ResetSession implements the driver.SessionResetter interface.
func (c *conn) ResetSession(ctx context.Context) error { return c.resetSession(ctx) }

// resetSession validates a connection before reuse: shared by ResetSession
// (database/sql free-pool reuse) and the lifecycle pool handoff (see
// Connector.Connect), so both paths apply the same liveness check.
func (c *conn) resetSession(ctx context.Context) error {
	if c.session.isBad() {
		return driver.ErrBadConn
	}

	lastRead := c.dbConn.lastRead()

	if c.attrs.pingInterval == 0 || lastRead.IsZero() || time.Since(lastRead) < c.attrs.pingInterval {
		return nil
	}

	if _, err := c.session.queryDirect(ctx, pingQuery, tracePing); err != nil {
		return fmt.Errorf("%w: %w", driver.ErrBadConn, err)
	}
	return nil
}

// IsValid implements the driver.Validator interface: a connection with handed-out
// table rows is reported invalid, so database/sql discards it to Close.
func (c *conn) IsValid() bool {
	return !c.session.isBad() && c.tableOutTracker == nil
}

// Ping implements the driver.Pinger interface.
func (c *conn) Ping(ctx context.Context) error {
	var sqlErr error
	done := make(chan struct{})
	c.wg.Go(func() {
		defer close(done)
		_, sqlErr = c.session.queryDirect(ctx, pingQuery, tracePing)
	})

	select {
	case <-ctx.Done():
		c.terminateSession()
		return ctx.Err()
	case <-done:
		return sqlErr
	}
}

// PrepareContext implements the driver.ConnPrepareContext interface.
func (c *conn) PrepareContext(ctx context.Context, query string) (driver.Stmt, error) {
	var sqlErr error
	var stmt driver.Stmt
	done := make(chan struct{})
	c.wg.Go(func() {
		defer close(done)
		if sqlErr = c.session.switchUser(ctx); sqlErr != nil {
			return
		}
		var pr *prepareResult
		if pr, sqlErr = c.session.prepare(ctx, query); sqlErr != nil {
			return
		}
		stmt = newStmt(c, c.wg, c.attrs, c.metrics, query, pr)
		if stmtMetadata, ok := ctx.Value(stmtMetadataCtxKey).(*StmtMetadata); ok {
			*stmtMetadata = pr
		}
	})

	select {
	case <-ctx.Done():
		c.terminateSession()
		return nil, ctx.Err()
	case <-done:
		return stmt, sqlErr
	}
}

// BeginTx implements the driver.ConnBeginTx interface.
func (c *conn) BeginTx(ctx context.Context, opts driver.TxOptions) (driver.Tx, error) {
	if c.session.inTx.Load() {
		return nil, ErrNestedTransaction
	}

	var isolationLevelQuery string
	switch sql.IsolationLevel(opts.Isolation) {
	case sql.LevelDefault, sql.LevelReadCommitted:
		isolationLevelQuery = setIsolationLevelReadCommitted
	case sql.LevelRepeatableRead:
		isolationLevelQuery = setIsolationLevelRepeatableRead
	case sql.LevelSerializable:
		isolationLevelQuery = setIsolationLevelSerializable
	default:
		return nil, ErrUnsupportedIsolationLevel
	}

	var accessModeQuery string
	if opts.ReadOnly {
		accessModeQuery = setAccessModeReadOnly
	} else {
		accessModeQuery = setAccessModeReadWrite
	}

	var sqlErr error
	var tx driver.Tx
	done := make(chan struct{})
	c.wg.Go(func() {
		defer close(done)
		if sqlErr = c.session.switchUser(ctx); sqlErr != nil {
			return
		}
		// set isolation level
		if _, sqlErr = c.session.execDirect(ctx, isolationLevelQuery); sqlErr != nil {
			return
		}
		// set access mode
		if _, sqlErr = c.session.execDirect(ctx, accessModeQuery); sqlErr != nil {
			return
		}
		tx = newTx(c)
		c.session.inTx.Store(true)
	})

	select {
	case <-ctx.Done():
		c.terminateSession()
		return nil, ctx.Err()
	case <-done:
		return tx, sqlErr
	}
}

// QueryContext implements the driver.QueryerContext interface.
func (c *conn) QueryContext(ctx context.Context, query string, nvargs []driver.NamedValue) (driver.Rows, error) {
	// accepts stored procedures (call) without parameters to avoid parsing
	// the query string which might have comments, etc.
	if len(nvargs) != 0 {
		return nil, driver.ErrSkip // fast path not possible (prepare needed)
	}

	var sqlErr error
	var rows driver.Rows
	done := make(chan struct{})
	c.wg.Go(func() {
		defer close(done)
		if sqlErr = c.session.switchUser(ctx); sqlErr != nil {
			return
		}
		rows, sqlErr = c.session.queryDirect(ctx, query, traceQuery)
	})

	select {
	case <-ctx.Done():
		c.terminateSession()
		return nil, ctx.Err()
	case <-done:
		return rows, sqlErr
	}
}

// ExecContext implements the driver.ExecerContext interface.
func (c *conn) ExecContext(ctx context.Context, query string, nvargs []driver.NamedValue) (driver.Result, error) {
	if len(nvargs) != 0 {
		return nil, driver.ErrSkip // fast path not possible (prepare needed)
	}

	var sqlErr error
	var result driver.Result
	done := make(chan struct{})
	c.wg.Go(func() {
		defer close(done)
		if sqlErr = c.session.switchUser(ctx); sqlErr != nil {
			return
		}
		// handle procedure call without parameters here as well
		result, sqlErr = c.session.execDirect(ctx, query)
	})

	select {
	case <-ctx.Done():
		c.terminateSession()
		return nil, ctx.Err()
	case <-done:
		return result, sqlErr
	}
}

// CheckNamedValue implements the NamedValueChecker interface.
func (c *conn) CheckNamedValue(nv *driver.NamedValue) error {
	// - called by sql driver for ExecContext and QueryContext
	// - no check needs to be performed as ExecContext and QueryContext provided
	//   with parameters will force the 'prepare way' (driver.ErrSkip)
	// - Anyway, CheckNamedValue must be implemented to avoid default sql driver checks
	//   which would fail for custom arg types like Lob
	return nil
}

// Conn Raw access methods

// HDBVersion implements the Conn interface.
func (c *conn) HDBVersion() *Version { return c.session.hdbVersion }

// DatabaseName implements the Conn interface.
func (c *conn) DatabaseName() string { return c.session.databaseName }

// DBConnectInfo implements the Conn interface.
func (c *conn) DBConnectInfo(ctx context.Context, databaseName string) (*DBConnectInfo, error) {
	var sqlErr error
	var ci *DBConnectInfo
	done := make(chan struct{})
	c.wg.Go(func() {
		defer close(done)
		ci, sqlErr = c.session.dbConnectInfo(ctx, databaseName)
	})

	select {
	case <-ctx.Done():
		c.terminateSession()
		return nil, ctx.Err()
	case <-done:
		return ci, sqlErr
	}
}

// transaction.

// check if tx implements all required interfaces.
var (
	_ driver.Tx = (*tx)(nil)
)

type tx struct {
	conn   *conn
	closed atomic.Bool
}

func newTx(conn *conn) *tx {
	conn.metrics.addGauge(gaugeTx, 1) // increment number of transactions.
	return &tx{conn: conn}
}

func (t *tx) Commit() error   { return t.close(false) }
func (t *tx) Rollback() error { return t.close(true) }

func (t *tx) close(rollback bool) error {
	c := t.conn

	c.metrics.addGauge(gaugeTx, -1) // decrement number of transactions.

	defer func() {
		c.session.inTx.Store(false)
	}()

	if c.session.isBad() {
		return driver.ErrBadConn
	}
	if closed := t.closed.Swap(true); closed {
		return nil
	}

	if rollback {
		return c.session.rollback(context.Background())
	}
	return c.session.commit(context.Background())
}
