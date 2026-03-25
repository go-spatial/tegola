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
	"github.com/SAP/go-hdb/driver/internal/protocol/auth"
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
// On cases 1 and 2 this error can be avoided in using a transaction (sql.Tx) on the query or exec statement.
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
	// register as var to execute even before init() funcs are called.
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

var stdConnTracker = &connTracker{}

type connTracker struct {
	mu      sync.Mutex
	_callDB *sql.DB
	numConn int64
}

func (t *connTracker) add() { t.mu.Lock(); t.numConn++; t.mu.Unlock() }

func (t *connTracker) remove() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.numConn--
	if t.numConn > 0 {
		return
	}
	t.numConn = 0
	if t._callDB != nil {
		t._callDB.Close()
		t._callDB = nil
	}
}

func (t *connTracker) callDB() *sql.DB {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t._callDB == nil {
		t._callDB = sql.OpenDB(new(callConnector))
	}
	return t._callDB
}

// Conn is the implementation of the database/sql/driver Conn interface.
type conn struct {
	attrs   *connAttrs
	metrics *metrics
	logger  *slog.Logger
	session *session
	wg      *sync.WaitGroup // wait for concurrent db calls when closing connections.
}

// isAuthError returns true in case of X509 certificate validation errrors or hdb authentication errors, else otherwise.
func isAuthError(err error) bool {
	var certValidationError *auth.CertValidationError
	if errors.As(err, &certValidationError) {
		return true
	}
	var hdbErrors *p.HdbErrors
	if !errors.As(err, &hdbErrors) {
		return false
	}
	return hdbErrors.Code() == p.HdbErrAuthenticationFailed
}

// unique connection number.
var connNo atomic.Uint64

func newConn(ctx context.Context, host string, metrics *metrics, attrs *connAttrs, authHnd *p.AuthHnd) (*conn, error) {
	logger := attrs.logger.With(slog.Uint64("conn", connNo.Add(1)))

	metrics.lazyInit()

	session, err := newSession(ctx, host, logger, metrics, attrs, authHnd)
	if err != nil {
		return nil, err
	}

	stdConnTracker.add()
	metrics.msgCh <- gaugeMsg{idx: gaugeConn, v: 1} // increment open connections.

	return &conn{attrs: attrs, metrics: metrics, logger: logger, session: session, wg: new(sync.WaitGroup)}, nil
}

// Close implements the driver.Conn interface.
func (c *conn) Close() error {
	c.metrics.msgCh <- gaugeMsg{idx: gaugeConn, v: -1} // decrement open connections.
	stdConnTracker.remove()
	return c.session.close()
}

// ResetSession implements the driver.SessionResetter interface.
func (c *conn) ResetSession(ctx context.Context) error {
	if c.session.isBad() {
		return driver.ErrBadConn
	}

	lastRead := c.session.dbConn.lastRead()

	if c.attrs.pingInterval == 0 || lastRead.IsZero() || time.Since(lastRead) < c.attrs.pingInterval {
		return nil
	}

	if _, err := c.session.queryDirect(ctx, pingQuery, tracePing); err != nil {
		return fmt.Errorf("%w: %w", driver.ErrBadConn, err)
	}
	return nil
}

// IsValid implements the driver.Validator interface.
func (c *conn) IsValid() bool { return !c.session.isBad() }

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
		c.session.cancel()
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
		stmt = newStmt(c.session, c.wg, c.attrs, c.metrics, query, pr)
		if stmtMetadata, ok := ctx.Value(stmtMetadataCtxKey).(*StmtMetadata); ok {
			*stmtMetadata = pr
		}
	})

	select {
	case <-ctx.Done():
		c.session.cancel()
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
		c.session.cancel()
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
		c.session.cancel()
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
		c.session.cancel()
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
		c.session.cancel()
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
	conn.metrics.msgCh <- gaugeMsg{idx: gaugeTx, v: 1} // increment number of transactions.
	return &tx{conn: conn}
}

func (t *tx) Commit() error   { return t.close(false) }
func (t *tx) Rollback() error { return t.close(true) }

func (t *tx) close(rollback bool) error {
	c := t.conn

	c.metrics.msgCh <- gaugeMsg{idx: gaugeTx, v: -1} // decrement number of transactions.

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
