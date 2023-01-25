package driver

import (
	"bufio"
	"context"
	"crypto/tls"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"io"
	"net"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/SAP/go-hdb/driver/dial"
	e "github.com/SAP/go-hdb/driver/internal/errors"
	p "github.com/SAP/go-hdb/driver/internal/protocol"

	"github.com/SAP/go-hdb/driver/internal/protocol/scanner"
	"github.com/SAP/go-hdb/driver/internal/protocol/x509"
	"github.com/SAP/go-hdb/driver/sqltrace"
	"github.com/SAP/go-hdb/driver/unicode/cesu8"
	"golang.org/x/text/transform"
)

// Transaction isolation levels supported by hdb.
const (
	LevelReadCommitted  = "READ COMMITTED"
	LevelRepeatableRead = "REPEATABLE READ"
	LevelSerializable   = "SERIALIZABLE"
)

// Access modes supported by hdb.
const (
	modeReadOnly  = "READ ONLY"
	modeReadWrite = "READ WRITE"
)

// map sql isolation level to hdb isolation level.
var isolationLevel = map[driver.IsolationLevel]string{
	driver.IsolationLevel(sql.LevelDefault):        LevelReadCommitted,
	driver.IsolationLevel(sql.LevelReadCommitted):  LevelReadCommitted,
	driver.IsolationLevel(sql.LevelRepeatableRead): LevelRepeatableRead,
	driver.IsolationLevel(sql.LevelSerializable):   LevelSerializable,
}

// map sql read only flag to hdb access mode.
var readOnly = map[bool]string{
	true:  modeReadOnly,
	false: modeReadWrite,
}

// ErrUnsupportedIsolationLevel is the error raised if a transaction is started with a not supported isolation level.
var ErrUnsupportedIsolationLevel = errors.New("unsupported isolation level")

// ErrNestedTransaction is the error raised if a transaction is created within a transaction as this is not supported by hdb.
var ErrNestedTransaction = errors.New("nested transactions are not supported")

// ErrNestedQuery is the error raised if a sql statement is executed before an "active" statement is closed.
// Example: execute sql statement before rows of previous select statement are closed.
var ErrNestedQuery = errors.New("nested sql queries are not supported")

// queries
const (
	dummyQuery        = "select 1 from dummy"
	setIsolationLevel = "set transaction isolation level"
	setAccessMode     = "set transaction"
	setDefaultSchema  = "set schema"
)

var errBulkExecDeprecated = errors.New("bulk exec option is deprecated")

// bulk statement
const (
	bulk = "b$"
)

var (
	flushTok   = new(struct{})
	noFlushTok = new(struct{})
)

var (
	// NoFlush is to be used as parameter in bulk statements to delay execution.
	// Deprecated
	NoFlush = sql.Named(bulk, &noFlushTok)
	// Flush can be used as optional parameter in bulk statements but is not required to trigger execution.
	// Deprecated
	Flush = sql.Named(bulk, &flushTok)
)

const (
	maxNumTraceArg = 20
)

var (
	// register as var to execute even before init() funcs are called
	_ = p.RegisterScanType(p.DtDecimal, reflect.TypeOf((*Decimal)(nil)).Elem())
	_ = p.RegisterScanType(p.DtLob, reflect.TypeOf((*Lob)(nil)).Elem())
)

// dbConn wraps the database tcp connection. It sets timeouts and handles driver ErrBadConn behavior.
type dbConn struct {
	metrics *metrics
	conn    net.Conn
	timeout time.Duration
}

func (c *dbConn) deadline() (deadline time.Time) {
	if c.timeout == 0 {
		return
	}
	return time.Now().Add(c.timeout)
}

func (c *dbConn) close() error { return c.conn.Close() }

// Read implements the io.Reader interface.
func (c *dbConn) Read(b []byte) (n int, err error) {
	var start time.Time
	//set timeout
	if err = c.conn.SetReadDeadline(c.deadline()); err != nil {
		goto retError
	}
	start = time.Now()
	n, err = c.conn.Read(b)
	c.metrics.chMsg <- timeMsg{idx: timeRead, d: time.Since(start)}
	c.metrics.chMsg <- counterMsg{idx: counterBytesRead, v: uint64(n)}
	if err == nil {
		return
	}
retError:
	dlog.Printf("Connection read error local address %s remote address %s: %s", c.conn.LocalAddr(), c.conn.RemoteAddr(), err)
	// wrap error in driver.ErrBadConn
	return n, fmt.Errorf("%w: %s", driver.ErrBadConn, err)
}

// Write implements the io.Writer interface.
func (c *dbConn) Write(b []byte) (n int, err error) {
	var start time.Time
	//set timeout
	if err = c.conn.SetWriteDeadline(c.deadline()); err != nil {
		goto retError
	}
	start = time.Now()
	n, err = c.conn.Write(b)
	c.metrics.chMsg <- timeMsg{idx: timeWrite, d: time.Since(start)}
	c.metrics.chMsg <- counterMsg{idx: counterBytesWritten, v: uint64(n)}
	if err == nil {
		return
	}
retError:
	dlog.Printf("Connection write error local address %s remote address %s: %s", c.conn.LocalAddr(), c.conn.RemoteAddr(), err)
	// wrap error in driver.ErrBadConn
	return n, fmt.Errorf("%w: %s", driver.ErrBadConn, err)
}

const (
	lrNestedQuery = 1
)

type connLock struct {
	// 64 bit alignment
	lockReason int64 // atomic access

	mu     sync.Mutex // tryLock mutex
	connMu sync.Mutex // connection mutex
}

func (l *connLock) tryLock(lockReason int64) error {
	l.mu.Lock()
	if atomic.LoadInt64(&l.lockReason) == lrNestedQuery {
		l.mu.Unlock()
		return ErrNestedQuery
	}
	l.connMu.Lock()
	atomic.StoreInt64(&l.lockReason, lockReason)
	l.mu.Unlock()
	return nil
}

func (l *connLock) lock() { l.connMu.Lock() }

func (l *connLock) unlock() {
	atomic.StoreInt64(&l.lockReason, 0)
	l.connMu.Unlock()
}

// check if conn implements all required interfaces
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

// connHook is a hook for testing.
var connHook func(c *conn, op int)

// connection hook operations
const (
	choNone = iota
	choStmtExec
)

var errCancelled = fmt.Errorf("%w: %s", driver.ErrBadConn, errors.New("db call cancelled"))

// Conn enhances a connection with go-hdb specific connection functions.
type Conn interface {
	HDBVersion() *Version
	DatabaseName() string
	DBConnectInfo(ctx context.Context, databaseName string) (*DBConnectInfo, error)
}

// Conn is the implementation of the database/sql/driver Conn interface.
type conn struct {
	*connAttrs
	metrics *metrics
	// Holding connection lock in QueryResultSet (see rows.onClose)
	/*
		As long as a session is in query mode no other sql statement must be executed.
		Example:
		- pinger is active
		- select with blob fields is executed
		- scan is hitting the database again (blob streaming)
		- if in between a ping gets executed (ping selects db) hdb raises error
		  "SQL Error 1033 - error while parsing protocol: invalid lob locator id (piecewise lob reading)"
	*/
	connLock

	dbConn  *dbConn
	scanner *scanner.Scanner
	closed  chan struct{}

	inTx bool // in transaction

	lastError error // last error

	trace bool // call sqlTrace.On() only once

	sessionID int64

	// after go.17 support: delete serverOptions and define it again direcly here
	serverOptions p.Options[p.ConnectOption]
	hdbVersion    *Version

	pr *p.Reader
	pw *p.Writer
}

// isAuthError returns true in case of X509 certificate validation errrors or hdb authentication errors, else otherwise.
func isAuthError(err error) bool {
	var validationError *x509.ValidationError
	if errors.As(err, &validationError) {
		return true
	}
	var hdbErrors *p.HdbErrors
	if !errors.As(err, &hdbErrors) {
		return false
	}
	return hdbErrors.Code() == p.HdbErrAuthenticationFailed
}

func newConn(ctx context.Context, metrics *metrics, connAttrs *connAttrs, authAttrs *authAttrs) (driver.Conn, error) {
	// can we connect via cookie?
	if auth := authAttrs.cookieAuth(); auth != nil {
		conn, err := initConn(ctx, metrics, connAttrs, auth)
		if err == nil {
			return conn, nil
		}
		if !isAuthError(err) {
			return nil, err
		}
		authAttrs.invalidateCookie() // cookie auth was not possible - do not try again with the same data
	}

	auth := authAttrs.auth()
	retries := 1
	for {
		conn, err := initConn(ctx, metrics, connAttrs, auth)
		if err == nil {
			if method, ok := auth.Method().(p.AuthCookieGetter); ok {
				authAttrs.setCookie(method.Cookie())
			}
			return conn, nil
		}
		if !isAuthError(err) {
			return nil, err
		}
		if retries < 1 {
			return nil, err
		}
		refresh, refreshErr := authAttrs.refresh(auth)
		if refreshErr != nil {
			return nil, refreshErr
		}
		if !refresh {
			return nil, err
		}
		retries--
	}
}

func initConn(ctx context.Context, metrics *metrics, attrs *connAttrs, auth *p.Auth) (driver.Conn, error) {
	netConn, err := attrs._dialer.DialContext(ctx, attrs._host, dial.DialerOptions{Timeout: attrs._timeout, TCPKeepAlive: attrs._tcpKeepAlive})
	if err != nil {
		return nil, err
	}

	// is TLS connection requested?
	if attrs._tlsConfig != nil {
		netConn = tls.Client(netConn, attrs._tlsConfig)
	}

	dbConn := &dbConn{metrics: metrics, conn: netConn, timeout: attrs._timeout}
	// buffer connection
	rw := bufio.NewReadWriter(bufio.NewReaderSize(dbConn, attrs._bufferSize), bufio.NewWriterSize(dbConn, attrs._bufferSize))

	c := &conn{
		metrics:   metrics,
		connAttrs: attrs,
		dbConn:    dbConn,
		scanner:   &scanner.Scanner{},
		closed:    make(chan struct{}),
		trace:     sqltrace.On(),
	}

	c.pw = p.NewWriter(rw.Writer, attrs._cesu8Encoder, attrs._sessionVariables) // write upstream
	if err := c.pw.WriteProlog(); err != nil {
		return nil, err
	}

	c.pr = p.NewReader(false, rw.Reader, attrs._cesu8Decoder) // read downstream
	if err := c.pr.ReadProlog(); err != nil {
		return nil, err
	}

	c.sessionID = defaultSessionID

	if c.sessionID, c.serverOptions, err = c._authenticate(auth, attrs._applicationName, attrs._dfv, attrs._locale); err != nil {
		return nil, err
	}

	if c.sessionID <= 0 {
		return nil, fmt.Errorf("invalid session id %d", c.sessionID)
	}

	c.hdbVersion = parseVersion(c.versionString())

	if attrs._defaultSchema != "" {
		if _, err := c.ExecContext(ctx, strings.Join([]string{setDefaultSchema, Identifier(attrs._defaultSchema).String()}, " "), nil); err != nil {
			return nil, err
		}
	}

	if attrs._pingInterval != 0 {
		go c.pinger(attrs._pingInterval, c.closed)
	}

	c.metrics.chMsg <- gaugeMsg{idx: gaugeConn, v: 1} // increment open connections.

	return c, nil
}

func (c *conn) versionString() (version string) {
	v, ok := c.serverOptions[p.CoFullVersionString]
	if !ok {
		return
	}
	if s, ok := v.(string); ok {
		return s
	}
	return
}

/*
A better option would be to wrap driver.ErrBadConn directly into a fatal error (instead of using e.ErrFatal).
Then we could get rid of the isBad check executed on next 'roundrip' completely.
But unfortunately go database/sql does not return the original error in any case but returns driver.ErrBadConn in some cases instead.
Tested go versions wrapping driver.ErrBadConn instead of e.ErrFatal:
- go 1.17.13: works ok
- go 1.18.5 : does not work
- go 1.19.2 : does not work
*/
func (c *conn) isBad() bool {
	return errors.Is(c.lastError, driver.ErrBadConn) || errors.Is(c.lastError, e.ErrFatal)
}

func (c *conn) pinger(d time.Duration, done <-chan struct{}) {
	ticker := time.NewTicker(d)
	defer ticker.Stop()

	ctx := context.Background()
	for {
		select {
		case <-done:
			return
		case <-ticker.C:
			c.Ping(ctx)
		}
	}
}

// Ping implements the driver.Pinger interface.
func (c *conn) Ping(ctx context.Context) (err error) {
	if err := c.tryLock(0); err != nil {
		return err
	}
	defer c.unlock()

	if c.isBad() {
		return driver.ErrBadConn
	}

	if c.trace {
		defer traceSQL(time.Now(), dummyQuery, nil)
	}

	done := make(chan struct{})
	go func() {
		_, err = c._queryDirect(dummyQuery, !c.inTx)
		close(done)
	}()

	select {
	case <-ctx.Done():
		c.lastError = errCancelled
		return ctx.Err()
	case <-done:
		c.lastError = err
		return err
	}
}

// ResetSession implements the driver.SessionResetter interface.
func (c *conn) ResetSession(ctx context.Context) error {
	c.lock()
	defer c.unlock()

	stdQueryResultCache.cleanup(c)

	if c.isBad() {
		return driver.ErrBadConn
	}
	return nil
}

// IsValid implements the driver.Validator interface.
func (c *conn) IsValid() bool {
	c.lock()
	defer c.unlock()

	return !c.isBad()
}

// PrepareContext implements the driver.ConnPrepareContext interface.
func (c *conn) PrepareContext(ctx context.Context, query string) (stmt driver.Stmt, err error) {
	if err := c.tryLock(0); err != nil {
		return nil, err
	}
	defer c.unlock()

	if c.isBad() {
		return nil, driver.ErrBadConn
	}

	if c.trace {
		defer traceSQL(time.Now(), query, nil)
	}

	done := make(chan struct{})
	func() {
		var (
			qd *queryDescr
			pr *prepareResult
		)

		if qd, err = newQueryDescr(query, c.scanner); err != nil {
			goto done
		}

		if pr, err = c._prepare(qd.query); err != nil {
			goto done
		}
		if err = pr.check(qd); err != nil {
			goto done
		}

		stmt = newStmt(c, qd, pr)

	done:
		close(done)
	}()

	select {
	case <-ctx.Done():
		c.lastError = errCancelled
		return nil, ctx.Err()
	case <-done:
		c.metrics.chMsg <- gaugeMsg{idx: gaugeStmt, v: 1} // increment number of statements.
		c.lastError = err
		return stmt, err
	}
}

// Close implements the driver.Conn interface.
func (c *conn) Close() error {
	c.lock()
	defer c.unlock()

	c.metrics.chMsg <- gaugeMsg{idx: gaugeConn, v: -1} // decrement open connections.
	close(c.closed)                                    // signal connection close

	// cleanup query cache
	stdQueryResultCache.cleanup(c)

	// if isBad do not disconnect
	if !c.isBad() {
		c._disconnect() // ignore error
	}
	return c.dbConn.close()
}

// BeginTx implements the driver.ConnBeginTx interface.
func (c *conn) BeginTx(ctx context.Context, opts driver.TxOptions) (tx driver.Tx, err error) {
	if err := c.tryLock(0); err != nil {
		return nil, err
	}
	defer c.unlock()

	if c.isBad() {
		return nil, driver.ErrBadConn
	}

	if c.inTx {
		return nil, ErrNestedTransaction
	}

	level, ok := isolationLevel[opts.Isolation]
	if !ok {
		return nil, ErrUnsupportedIsolationLevel
	}

	done := make(chan struct{})
	go func() {
		// set isolation level
		query := strings.Join([]string{setIsolationLevel, level}, " ")
		if _, err = c._execDirect(query, !c.inTx); err != nil {
			goto done
		}
		// set access mode
		query = strings.Join([]string{setAccessMode, readOnly[opts.ReadOnly]}, " ")
		if _, err = c._execDirect(query, !c.inTx); err != nil {
			goto done
		}
		c.inTx = true
		tx = newTx(c)
	done:
		close(done)
	}()

	select {
	case <-ctx.Done():
		c.lastError = errCancelled
		return nil, ctx.Err()
	case <-done:
		c.metrics.chMsg <- gaugeMsg{idx: gaugeTx, v: 1} // increment number of transactions.
		c.lastError = err
		return tx, err
	}
}

// QueryContext implements the driver.QueryerContext interface.
func (c *conn) QueryContext(ctx context.Context, query string, nvargs []driver.NamedValue) (rows driver.Rows, err error) {
	if len(nvargs) != 0 {
		return nil, driver.ErrSkip //fast path not possible (prepare needed)
	}

	if err := c.tryLock(lrNestedQuery); err != nil {
		return nil, err
	}
	hasRowsCloser := false
	defer func() {
		// unlock connection if rows will not do it
		if !hasRowsCloser {
			c.unlock()
		}
	}()

	if c.isBad() {
		return nil, driver.ErrBadConn
	}

	qd, err := newQueryDescr(query, c.scanner)
	if err != nil {
		return nil, err
	}
	switch qd.kind {
	case qkCall:
		// direct execution of call procedure
		// - returns no parameter metadata (sps 82) but only field values
		// --> let's take the 'prepare way' for stored procedures
		return nil, driver.ErrSkip
	case qkID:
		// query call table result
		rows, ok := stdQueryResultCache.Get(qd.id)
		if !ok {
			return nil, fmt.Errorf("invalid result set id %s", query)
		}
		if onCloser, ok := rows.(onCloser); ok {
			onCloser.setOnClose(c.unlock)
			hasRowsCloser = true
		}
		return rows, nil
	}

	if c.trace {
		defer traceSQL(time.Now(), query, nvargs)
	}

	done := make(chan struct{})
	go func() {
		rows, err = c._queryDirect(query, !c.inTx)
		close(done)
	}()

	select {
	case <-ctx.Done():
		c.lastError = errCancelled
		return nil, ctx.Err()
	case <-done:
		if onCloser, ok := rows.(onCloser); ok {
			onCloser.setOnClose(c.unlock)
			hasRowsCloser = true
		}
		c.lastError = err
		return rows, err
	}
}

// ExecContext implements the driver.ExecerContext interface.
func (c *conn) ExecContext(ctx context.Context, query string, nvargs []driver.NamedValue) (r driver.Result, err error) {
	if len(nvargs) != 0 {
		return nil, driver.ErrSkip //fast path not possible (prepare needed)
	}

	if err := c.tryLock(0); err != nil {
		return nil, err
	}
	defer c.unlock()

	if c.isBad() {
		return nil, driver.ErrBadConn
	}

	if c.trace {
		defer traceSQL(time.Now(), query, nvargs)
	}

	done := make(chan struct{})
	go func() {
		/*
			handle call procedure (qd.Kind() == p.QkCall) without parameters here as well
		*/
		var qd *queryDescr

		if qd, err = newQueryDescr(query, c.scanner); err != nil {
			goto done
		}
		r, err = c._execDirect(qd.query, !c.inTx)
	done:
		close(done)
	}()

	select {
	case <-ctx.Done():
		c.lastError = errCancelled
		return nil, ctx.Err()
	case <-done:
		c.lastError = err
		return r, err
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
func (c *conn) HDBVersion() *Version { return c.hdbVersion }

// DatabaseName implements the Conn interface.
func (c *conn) DatabaseName() string { return c._databaseName() }

// DBConnectInfo implements the Conn interface.
func (c *conn) DBConnectInfo(ctx context.Context, databaseName string) (ci *DBConnectInfo, err error) {
	if err := c.tryLock(0); err != nil {
		return nil, err
	}
	defer c.unlock()

	if c.isBad() {
		return nil, driver.ErrBadConn
	}

	done := make(chan struct{})
	go func() {
		ci, err = c._dbConnectInfo(databaseName)
		close(done)
	}()

	select {
	case <-ctx.Done():
		c.lastError = errCancelled
		return nil, ctx.Err()
	case <-done:
		c.lastError = err
		return ci, err
	}
}

func traceSQL(start time.Time, query string, nvargs []driver.NamedValue) {
	ms := time.Since(start).Milliseconds()
	switch {
	case len(nvargs) == 0:
		sqltrace.Trace.Printf("%s duration %dms", query, ms)
	case len(nvargs) > maxNumTraceArg:
		sqltrace.Trace.Printf("%s args(limited to %d) %v duration %dms", query, maxNumTraceArg, nvargs[:maxNumTraceArg], ms)
	default:
		sqltrace.Trace.Printf("%s args %v duration %dms", query, nvargs, ms)
	}
}

func (c *conn) addTimeValue(start time.Time, k int) {
	c.metrics.chMsg <- timeMsg{idx: k, d: time.Since(start)}
}

func (c *conn) addSQLTimeValue(start time.Time, k int) {
	c.metrics.chMsg <- sqlTimeMsg{idx: k, d: time.Since(start)}
}

//transaction

// check if tx implements all required interfaces
var (
	_ driver.Tx = (*tx)(nil)
)

type tx struct {
	conn   *conn
	closed bool
}

func newTx(conn *conn) *tx { return &tx{conn: conn} }

func (t *tx) Commit() error   { return t.close(false) }
func (t *tx) Rollback() error { return t.close(true) }

func (t *tx) close(rollback bool) (err error) {
	c := t.conn

	c.lock()
	defer c.unlock()

	if t.closed {
		return nil
	}
	t.closed = true

	c.inTx = false

	c.metrics.chMsg <- gaugeMsg{idx: gaugeTx, v: -1} // decrement number of transactions.

	if c.isBad() {
		return driver.ErrBadConn
	}

	if rollback {
		err = c._rollback()
	} else {
		err = c._commit()
	}
	return
}

// check if statements implements all required interfaces
var (
	_ driver.Stmt              = (*stmt)(nil)
	_ driver.StmtExecContext   = (*stmt)(nil)
	_ driver.StmtQueryContext  = (*stmt)(nil)
	_ driver.NamedValueChecker = (*stmt)(nil)
)

// statement kind
const (
	skNone = iota
	skExec
	skBulk
	skCall
)

type stmt struct {
	stmtKind int
	conn     *conn
	query    string
	pr       *prepareResult
	flush    bool                // bulk
	numBulk  int                 // bulk
	nvargs   []driver.NamedValue // bulk or many
}

func newStmt(conn *conn, qd *queryDescr, pr *prepareResult) *stmt {
	stmtKind := skNone
	switch {
	case pr.isProcedureCall():
		stmtKind = skCall
	case qd.isBulk:
		stmtKind = skBulk
	default:
		stmtKind = skExec
	}
	return &stmt{stmtKind: stmtKind, conn: conn, query: qd.query, pr: pr}
}

/*
NumInput differs dependent on statement (check is done in QueryContext and ExecContext):
- #args == #param (only in params):    query, exec, exec bulk (non control query)
- #args == #param (in and out params): exec call
- #args == 0:                          exec bulk (control query)
- #args == #input param:               query call
*/
func (s *stmt) NumInput() int { return -1 }

// stmt methods

/*
reset args
- keep slice to avoid additional allocations but
- free elements (GC)
*/
func (s *stmt) resetArgs() {
	for i := 0; i < len(s.nvargs); i++ {
		s.nvargs[i].Value = nil
	}
	s.nvargs = s.nvargs[:0]
}

func (s *stmt) Close() error {
	c := s.conn

	c.lock()
	defer c.unlock()

	s.conn.metrics.chMsg <- gaugeMsg{idx: gaugeStmt, v: -1} // decrement number of statements.

	if c.isBad() {
		return driver.ErrBadConn
	}

	if s.nvargs != nil {
		if len(s.nvargs) != 0 { // log always
			dlog.Printf("close: %s - not flushed records: %d)", s.query, len(s.nvargs)/s.pr.numField())
		}
		s.nvargs = nil
	}

	return c._dropStatementID(s.pr.stmtID)
}

func (s *stmt) convert(field *p.ParameterField, arg any) (any, error) {
	// let fields with own Value converter convert themselves first (e.g. NullInt64, ...)
	var err error
	if valuer, ok := arg.(driver.Valuer); ok {
		if arg, err = valuer.Value(); err != nil {
			return nil, err
		}
	}
	// convert field
	return field.Convert(s.conn._cesu8Encoder(), arg)
}

/*
central function to extend argument handling by
- potentially handle named parameters (HANA does not support them)
- handle out parameters for function calls (HANA supports named out parameters)
*/
func (s *stmt) mapArgs(nvargs []driver.NamedValue) error {
	for i := 0; i < len(nvargs); i++ {

		field := s.pr.parameterField(i)

		out, isOut := nvargs[i].Value.(sql.Out)

		if isOut {
			if !field.Out() {
				return fmt.Errorf("argument %d field %s mismatch - use out argument with non-out field", i, field.Name())
			}
			if out.In && !field.In() {
				return fmt.Errorf("argument %d field %s mismatch - use in argument with out field", i, field.Name())
			}
		}

		// currently we do not support out parameters
		if isOut {
			return fmt.Errorf("argument %d field %s mismatch - out argument not supported", i, field.Name())
		}

		var err error
		if isOut {
			if out.In { // convert only if in parameter
				if out.Dest, err = s.convert(field, out.Dest); err != nil {
					return fmt.Errorf("argument %d field %s conversion error - %w", i, field.Name(), err)
				}
				nvargs[i].Value = out
			}
		} else {
			if nvargs[i].Value, err = s.convert(field, nvargs[i].Value); err != nil {
				return fmt.Errorf("argument %d field %s conversion error - %w", i, field.Name(), err)
			}
		}
	}
	return nil
}

func (s *stmt) QueryContext(ctx context.Context, nvargs []driver.NamedValue) (rows driver.Rows, err error) {
	c := s.conn

	if err := c.tryLock(lrNestedQuery); err != nil {
		return nil, err
	}
	hasRowsCloser := false
	defer func() {
		// unlock connection if rows will not do it
		if !hasRowsCloser {
			c.unlock()
		}
	}()

	if c.isBad() {
		return nil, driver.ErrBadConn
	}

	if c.trace {
		defer traceSQL(time.Now(), s.query, nvargs)
	}

	done := make(chan struct{})
	go func() {
		switch s.stmtKind {
		case skExec:
			rows, err = s._query(nvargs)
		case skCall:
			rows, err = s._queryCall(nvargs)
		default:
			panic(fmt.Sprintf("unsuported statement kind %d", s.stmtKind)) // should never happen
		}
		close(done)
	}()

	select {
	case <-ctx.Done():
		c.lastError = errCancelled
		return nil, ctx.Err()
	case <-done:
		if onCloser, ok := rows.(onCloser); ok {
			onCloser.setOnClose(c.unlock)
			hasRowsCloser = true
		}
		c.lastError = err
		return rows, err
	}
}

func (s *stmt) _query(nvargs []driver.NamedValue) (rows driver.Rows, err error) {
	if len(nvargs) != s.pr.numField() { // all fields needs to be input fields
		return nil, fmt.Errorf("invalid number of arguments %d - %d expected", len(nvargs), s.pr.numField())
	}
	if err := s.mapArgs(nvargs); err != nil {
		return nil, err
	}
	return s.conn._query(s.pr, nvargs, !s.conn.inTx)
}

func (s *stmt) _queryCall(nvargs []driver.NamedValue) (rows driver.Rows, err error) {
	if len(nvargs) != s.pr.numInputField() { // input fields only
		return nil, fmt.Errorf("invalid number of arguments %d - %d expected", len(nvargs), s.pr.numInputField())
	}
	if err := s.mapArgs(nvargs); err != nil {
		return nil, err
	}
	return s.conn._queryCall(s.pr, nvargs)
}

func (s *stmt) ExecContext(ctx context.Context, nvargs []driver.NamedValue) (r driver.Result, err error) {
	c := s.conn

	if err := c.tryLock(0); err != nil {
		return nil, err
	}
	defer c.unlock()

	if c.isBad() {
		return nil, driver.ErrBadConn
	}

	if connHook != nil {
		connHook(c, choStmtExec)
	}

	if c.trace {
		defer traceSQL(time.Now(), s.query, nvargs)
	}

	done := make(chan struct{})
	go func() {
		switch s.stmtKind {
		case skExec:
			r, err = s.execMany(nvargs)
		case skBulk:
			r, err = s.execBulk(nvargs)
		case skCall:
			r, err = s.execCall(nvargs)
		default:
			panic(fmt.Sprintf("unsuported statement kind %d", s.stmtKind)) // should never happen
		}
		close(done)
	}()

	select {
	case <-ctx.Done():
		c.lastError = errCancelled
		return nil, ctx.Err()
	case <-done:
		c.lastError = err
		return r, err
	}
}

type totalRowsAffected int64

func (t *totalRowsAffected) add(r driver.Result) {
	if r == nil {
		return
	}
	rows, err := r.RowsAffected()
	if err != nil {
		return
	}
	*t += totalRowsAffected(rows)
}

/*
Non 'atomic' (transactional) operation due to the split in packages (bulkSize),
execMany data might only be written partially to the database in case of hdb stmt errors.
*/
func (s *stmt) execMany(nvargs []driver.NamedValue) (driver.Result, error) {
	c := s.conn

	if len(nvargs) == 0 { // no parameters
		return c._execBulk(s.pr, nil, !c.inTx)
	}

	numField := s.pr.numField()

	it, err := newArgsScanner(numField, nvargs, c._legacy) //TODO: remove legacy starting with V1.0
	if err != nil {
		return nil, err
	}

	defer func() { s.resetArgs() }() // reset args

	totalRowsAffected := totalRowsAffected(0)
	recOfs := 0
	numRec := 0

	args := make([]driver.NamedValue, numField)

	for {
		err := it.scan(args)
		if err == ErrEndOfRows {
			break
		}
		if err != nil {
			return driver.RowsAffected(totalRowsAffected), err
		}

		if err := s.mapArgs(args); err != nil {
			return driver.RowsAffected(totalRowsAffected), err
		}

		s.nvargs = append(s.nvargs, args...)
		numRec++
		if numRec >= c._bulkSize {
			recOfs += c._bulkSize
			r, err := c._execBulk(s.pr, s.nvargs, !c.inTx)
			totalRowsAffected.add(r)
			if err != nil {
				if hdbErr, ok := err.(*p.HdbErrors); recOfs != 0 && ok {
					hdbErr.SetStmtsNoOfs(recOfs)
				}
				return driver.RowsAffected(totalRowsAffected), err
			}
			numRec = 0
			s.nvargs = s.nvargs[:0]
		}
	}

	if numRec > 0 {
		r, err := c._execBulk(s.pr, s.nvargs, !c.inTx)
		totalRowsAffected.add(r)
		if err != nil {
			if hdbErr, ok := err.(*p.HdbErrors); recOfs != 0 && ok {
				hdbErr.SetStmtsNoOfs(recOfs)
			}
			return driver.RowsAffected(totalRowsAffected), err
		}
	}

	return driver.RowsAffected(totalRowsAffected), nil
}

func (s *stmt) execBulk(nvargs []driver.NamedValue) (driver.Result, error) {
	c := s.conn
	// check deprecated
	if !c._legacy {
		return nil, errBulkExecDeprecated
	}

	flush := s.flush
	s.flush = false

	numArg := len(nvargs)
	numField := s.pr.numField()

	if numArg != 0 && numArg != numField {
		return nil, fmt.Errorf("invalid number of arguments %d - %d expected", numArg, numField)
	}

	switch numArg {
	case 0: // exec without args --> flush
		flush = true
	default: // add to argument buffer

		if err := s.mapArgs(nvargs); err != nil {
			return driver.ResultNoRows, err
		}

		s.nvargs = append(s.nvargs, nvargs...)
		s.numBulk++
		if s.numBulk >= c._bulkSize {
			flush = true
		}
	}

	if !flush || s.numBulk == 0 { // done: no flush
		return driver.ResultNoRows, nil
	}

	// flush
	r, err := c._execBulk(s.pr, s.nvargs, !c.inTx)
	s.resetArgs()
	s.numBulk = 0
	return r, err
}

func (s *stmt) execCall(nvargs []driver.NamedValue) (driver.Result, error) {
	if len(nvargs) != s.pr.numInputField() { // input fields only
		return driver.ResultNoRows, fmt.Errorf("invalid number of arguments %d - %d expected", len(nvargs), s.pr.numInputField())
	}
	if err := s.mapArgs(nvargs); err != nil {
		return driver.ResultNoRows, err
	}
	return s.conn._execCall(s.pr, nvargs)
}

// CheckNamedValue implements NamedValueChecker interface.
func (s *stmt) CheckNamedValue(nv *driver.NamedValue) error {
	// check add arguments only
	// conversion is happening as part of the exec, query call
	if nv.Name == bulk {
		if ptr, ok := nv.Value.(**struct{}); ok {
			switch ptr {
			case &noFlushTok:
				if !s.conn._legacy {
					return errBulkExecDeprecated
				}
				if s.stmtKind == skExec { // turn on bulk
					s.stmtKind = skBulk
				}
				return driver.ErrRemoveArgument
			case &flushTok:
				if !s.conn._legacy {
					return errBulkExecDeprecated
				}
				s.flush = true
				return driver.ErrRemoveArgument
			}
		}
	}
	return nil
}

const defaultSessionID = -1

func (c *conn) _databaseName() string {
	return c.serverOptions[p.CoDatabaseName].(string)
}

func (c *conn) _dbConnectInfo(databaseName string) (*DBConnectInfo, error) {
	ci := p.Options[p.DBConnectInfoType]{p.CiDatabaseName: databaseName}
	if err := c.pw.Write(c.sessionID, p.MtDBConnectInfo, false, ci); err != nil {
		return nil, err
	}

	if err := c.pr.IterateParts(func(ph *p.PartHeader) {
		switch ph.PartKind {
		case p.PkDBConnectInfo:
			c.pr.Read(&ci)
		}
	}); err != nil {
		return nil, err
	}

	host, _ := ci[p.CiHost].(string) //check existencs and covert to string
	port, _ := ci[p.CiPort].(int32)  // check existence and convert to integer
	isConnected, _ := ci[p.CiIsConnected].(bool)

	return &DBConnectInfo{
		DatabaseName: databaseName,
		Host:         host,
		Port:         int(port),
		IsConnected:  isConnected,
	}, nil
}

func (c *conn) _authenticate(auth *p.Auth, applicationName string, dfv int, locale string) (int64, p.Options[p.ConnectOption], error) {
	defer c.addTimeValue(time.Now(), timeAuth)

	// client context
	clientContext := p.Options[p.ClientContextOption]{
		p.CcoClientVersion:            DriverVersion,
		p.CcoClientType:               clientType,
		p.CcoClientApplicationProgram: applicationName,
	}

	initRequest, err := auth.InitRequest()
	if err != nil {
		return 0, nil, err
	}
	if err := c.pw.Write(c.sessionID, p.MtAuthenticate, false, clientContext, initRequest); err != nil {
		return 0, nil, err
	}

	initReply, err := auth.InitReply()
	if err != nil {
		return 0, nil, err
	}
	if err := c.pr.IterateParts(func(ph *p.PartHeader) {
		if ph.PartKind == p.PkAuthentication {
			c.pr.Read(initReply)
		}
	}); err != nil {
		return 0, nil, err
	}

	finalRequest, err := auth.FinalRequest()
	if err != nil {
		return 0, nil, err
	}
	//co := c.defaultClientOptions()

	co := func() p.Options[p.ConnectOption] {
		co := p.Options[p.ConnectOption]{
			p.CoDistributionProtocolVersion: false,
			p.CoSelectForUpdateSupported:    false,
			p.CoSplitBatchCommands:          true,
			p.CoDataFormatVersion2:          int32(dfv),
			p.CoCompleteArrayExecution:      true,
			p.CoClientDistributionMode:      int32(p.CdmOff),
		}
		if locale != "" {
			co[p.CoClientLocale] = locale
		}
		return co
	}()

	if err := c.pw.Write(c.sessionID, p.MtConnect, false, finalRequest, p.ClientID(clientID), co); err != nil {
		return 0, nil, err
	}

	finalReply, err := auth.FinalReply()
	if err != nil {
		return 0, nil, err
	}
	if err := c.pr.IterateParts(func(ph *p.PartHeader) {
		switch ph.PartKind {
		case p.PkAuthentication:
			c.pr.Read(finalReply)
		case p.PkConnectOptions:
			c.pr.Read(&co)
			// set data format version
			// TODO generalize for sniffer
			c.pr.SetDfv(int(co[p.CoDataFormatVersion2].(int32)))
		}
	}); err != nil {
		return 0, nil, err
	}
	return c.pr.SessionID(), co, nil
}

func (c *conn) _queryDirect(query string, commit bool) (driver.Rows, error) {
	defer c.addSQLTimeValue(time.Now(), sqlTimeQuery)

	// allow e.g inserts as query -> handle commit like in _execDirect
	if err := c.pw.Write(c.sessionID, p.MtExecuteDirect, commit, p.Command(query)); err != nil {
		return nil, err
	}

	qr := &queryResult{conn: c}
	meta := &p.ResultMetadata{}
	resSet := &p.Resultset{}

	if err := c.pr.IterateParts(func(ph *p.PartHeader) {
		switch ph.PartKind {
		case p.PkResultMetadata:
			c.pr.Read(meta)
			qr.fields = meta.ResultFields
		case p.PkResultsetID:
			c.pr.Read((*p.ResultsetID)(&qr.rsID))
		case p.PkResultset:
			resSet.ResultFields = qr.fields
			c.pr.Read(resSet)
			qr.fieldValues = resSet.FieldValues
			qr.decodeErrors = resSet.DecodeErrors
			qr.attributes = ph.PartAttributes
		}
	}); err != nil {
		return nil, err
	}
	if qr.rsID == 0 { // non select query
		return noResult, nil
	}
	return qr, nil
}

func (c *conn) _execDirect(query string, commit bool) (driver.Result, error) {
	defer c.addSQLTimeValue(time.Now(), sqlTimeExec)

	if err := c.pw.Write(c.sessionID, p.MtExecuteDirect, commit, p.Command(query)); err != nil {
		return nil, err
	}

	rows := &p.RowsAffected{}
	var numRow int64
	if err := c.pr.IterateParts(func(ph *p.PartHeader) {
		if ph.PartKind == p.PkRowsAffected {
			c.pr.Read(rows)
			numRow = rows.Total()
		}
	}); err != nil {
		return nil, err
	}
	if c.pr.FunctionCode() == p.FcDDL {
		return driver.ResultNoRows, nil
	}
	return driver.RowsAffected(numRow), nil
}

func (c *conn) _prepare(query string) (*prepareResult, error) {
	defer c.addSQLTimeValue(time.Now(), sqlTimePrepare)

	if err := c.pw.Write(c.sessionID, p.MtPrepare, false, p.Command(query)); err != nil {
		return nil, err
	}

	pr := &prepareResult{}
	resMeta := &p.ResultMetadata{}
	prmMeta := &p.ParameterMetadata{}

	if err := c.pr.IterateParts(func(ph *p.PartHeader) {
		switch ph.PartKind {
		case p.PkStatementID:
			c.pr.Read((*p.StatementID)(&pr.stmtID))
		case p.PkResultMetadata:
			c.pr.Read(resMeta)
			pr.resultFields = resMeta.ResultFields
		case p.PkParameterMetadata:
			c.pr.Read(prmMeta)
			pr.parameterFields = prmMeta.ParameterFields
		}
	}); err != nil {
		return nil, err
	}
	pr.fc = c.pr.FunctionCode()
	return pr, nil
}

// fetchFirstLobChunk reads the first LOB data ckunk.
func (c *conn) _fetchFirstLobChunk(nvargs []driver.NamedValue) (bool, error) {
	hasNext := false
	for _, arg := range nvargs {
		if lobInDescr, ok := arg.Value.(*p.LobInDescr); ok {
			last, err := lobInDescr.FetchNext(c._lobChunkSize)
			if !last {
				hasNext = true
			}
			if err != nil {
				return hasNext, err
			}
		}
	}
	return hasNext, nil
}

/*
Exec executes a sql statement.

Bulk insert containing LOBs:
  - Precondition:
    .Sending more than one row with partial LOB data.
  - Observations:
    .In hdb version 1 and 2 'piecewise' LOB writing does work.
    .Same does not work in case of geo fields which are LOBs en,- decoded as well.
    .In hana version 4 'piecewise' LOB writing seems not to work anymore at all.
  - Server implementation (not documented):
    .'piecewise' LOB writing is only supported for the last row of a 'bulk insert'.
  - Current implementation:
    One server call in case of
    .'non bulk' execs or
    .'bulk' execs without LOBs
    else potential several server calls (split into packages).
  - Package invariant:
    .for all packages except the last one, the last row contains 'incomplete' LOB data ('piecewise' writing)
*/
func (c *conn) _execBulk(pr *prepareResult, nvargs []driver.NamedValue, commit bool) (driver.Result, error) {
	defer c.addSQLTimeValue(time.Now(), sqlTimeExec)

	hasLob := func() bool {
		for _, f := range pr.parameterFields {
			if f.IsLob() {
				return true
			}
		}
		return false
	}()

	// no split needed: no LOB or only one row
	if !hasLob || len(pr.parameterFields) == len(nvargs) {
		return c._exec(pr, nvargs, hasLob, commit)
	}

	// args need to be potentially splitted (piecewise LOB handling)
	numColumns := len(pr.parameterFields)
	numRows := len(nvargs) / numColumns
	totRowsAffected := int64(0)
	lastFrom := 0

	for i := 0; i < numRows; i++ { // row-by-row

		from := i * numColumns
		to := from + numColumns

		hasNext, err := c._fetchFirstLobChunk(nvargs[from:to])
		if err != nil {
			return driver.RowsAffected(totRowsAffected), err
		}

		/*
			trigger server call (exec) if piecewise lob handling is needed
			or we did reach the last row
		*/
		if hasNext || i == (numRows-1) {
			r, err := c._exec(pr, nvargs[lastFrom:to], true, commit)
			if rowsAffected, err := r.RowsAffected(); err == nil {
				totRowsAffected += rowsAffected
			}
			if err != nil {
				return driver.RowsAffected(totRowsAffected), err
			}
			lastFrom = to
		}
	}
	return driver.RowsAffected(totRowsAffected), nil
}

func (c *conn) _exec(pr *prepareResult, nvargs []driver.NamedValue, hasLob, commit bool) (driver.Result, error) {
	inputParameters, err := p.NewInputParameters(pr.parameterFields, nvargs, hasLob)
	if err != nil {
		return nil, err
	}
	if err := c.pw.Write(c.sessionID, p.MtExecute, commit, p.StatementID(pr.stmtID), inputParameters); err != nil {
		return nil, err
	}

	rows := &p.RowsAffected{}
	var ids []p.LocatorID
	lobReply := &p.WriteLobReply{}
	var rowsAffected int64

	if err := c.pr.IterateParts(func(ph *p.PartHeader) {
		switch ph.PartKind {
		case p.PkRowsAffected:
			c.pr.Read(rows)
			rowsAffected = rows.Total()
		case p.PkWriteLobReply:
			c.pr.Read(lobReply)
			ids = lobReply.IDs
		}
	}); err != nil {
		return nil, err
	}
	fc := c.pr.FunctionCode()

	if len(ids) != 0 {
		/*
			writeLobParameters:
			- chunkReaders
			- nil (no callResult, exec does not have output parameters)
		*/
		if err := c.encodeLobs(nil, ids, pr.parameterFields, nvargs); err != nil {
			return nil, err
		}
	}

	if fc == p.FcDDL {
		return driver.ResultNoRows, nil
	}
	return driver.RowsAffected(rowsAffected), nil
}

func (c *conn) _queryCall(pr *prepareResult, nvargs []driver.NamedValue) (driver.Rows, error) {
	defer c.addSQLTimeValue(time.Now(), sqlTimeCall)

	/*
		only in args
		invariant: #inPrmFields == #args
	*/
	var inPrmFields, outPrmFields []*p.ParameterField
	hasInLob := false
	for _, f := range pr.parameterFields {
		if f.In() {
			inPrmFields = append(inPrmFields, f)
			if f.IsLob() {
				hasInLob = true
			}
		}
		if f.Out() {
			outPrmFields = append(outPrmFields, f)
		}
	}

	if hasInLob {
		if _, err := c._fetchFirstLobChunk(nvargs); err != nil {
			return nil, err
		}
	}
	inputParameters, err := p.NewInputParameters(inPrmFields, nvargs, hasInLob)
	if err != nil {
		return nil, err
	}
	if err := c.pw.Write(c.sessionID, p.MtExecute, false, p.StatementID(pr.stmtID), inputParameters); err != nil {
		return nil, err
	}

	/*
		call without lob input parameters:
		--> callResult output parameter values are set after read call
		call with lob input parameters:
		--> callResult output parameter values are set after last lob input write
	*/

	cr, ids, _, err := c._readCall(outPrmFields) // ignore numRow
	if err != nil {
		return nil, err
	}

	if len(ids) != 0 {
		/*
			writeLobParameters:
			- chunkReaders
			- cr (callResult output parameters are set after all lob input parameters are written)
		*/
		if err := c.encodeLobs(cr, ids, inPrmFields, nvargs); err != nil {
			return nil, err
		}
	}

	// legacy mode?
	if c._legacy {
		cr.appendTableRefFields()
		for _, qr := range cr.qrs {
			// add to cache
			stdQueryResultCache.set(qr.rsID, qr)
		}
	} else {
		cr.appendTableRowsFields()
	}
	return cr, nil
}

func (c *conn) _execCall(pr *prepareResult, nvargs []driver.NamedValue) (driver.Result, error) {
	defer c.addSQLTimeValue(time.Now(), sqlTimeCall)

	/*
		in,- and output args
		invariant: #prmFields == #args
	*/
	var (
		inPrmFields, outPrmFields []*p.ParameterField
		inArgs                    []driver.NamedValue
		// outArgs []driver.NamedValue
	)
	hasInLob := false
	for i, f := range pr.parameterFields {
		if f.In() {
			inPrmFields = append(inPrmFields, f)
			inArgs = append(inArgs, nvargs[i])
			if f.IsLob() {
				hasInLob = true
			}
		}
		// handle output parameters
		if f.Out() {
			outPrmFields = append(outPrmFields, f)
			//outArgs = append(outArgs, nvargs[i])
		}
	}

	// TODO support out parameters
	if len(outPrmFields) != 0 {
		return nil, fmt.Errorf("stmt.Exec: support of output parameters not implemented yet")
	}

	if hasInLob {
		if _, err := c._fetchFirstLobChunk(inArgs); err != nil {
			return nil, err
		}
	}
	inputParameters, err := p.NewInputParameters(inPrmFields, inArgs, hasInLob)
	if err != nil {
		return nil, err
	}
	if err := c.pw.Write(c.sessionID, p.MtExecute, false, p.StatementID(pr.stmtID), inputParameters); err != nil {
		return nil, err
	}

	/*
		call without lob input parameters:
		--> callResult output parameter values are set after read call
		call with lob output parameters:
		--> callResult output parameter values are set after last lob input write
	*/

	cr, ids, numRow, err := c._readCall(outPrmFields)
	if err != nil {
		return nil, err
	}

	if len(ids) != 0 {
		/*
			writeLobParameters:
			- chunkReaders
			- cr (callResult output parameters are set after all lob input parameters are written)
		*/
		if err := c.encodeLobs(cr, ids, inPrmFields, inArgs); err != nil {
			return nil, err
		}
	}
	return driver.RowsAffected(numRow), nil
}

func (c *conn) _readCall(outputFields []*p.ParameterField) (*callResult, []p.LocatorID, int64, error) {
	cr := &callResult{conn: c, outputFields: outputFields}

	var qr *queryResult
	rows := &p.RowsAffected{}
	var ids []p.LocatorID
	outPrms := &p.OutputParameters{}
	meta := &p.ResultMetadata{}
	resSet := &p.Resultset{}
	lobReply := &p.WriteLobReply{}
	var numRow int64

	if err := c.pr.IterateParts(func(ph *p.PartHeader) {
		switch ph.PartKind {
		case p.PkRowsAffected:
			c.pr.Read(rows)
			numRow = rows.Total()
		case p.PkOutputParameters:
			outPrms.OutputFields = cr.outputFields
			c.pr.Read(outPrms)
			cr.fieldValues = outPrms.FieldValues
			cr.decodeErrors = outPrms.DecodeErrors
		case p.PkResultMetadata:
			/*
				procedure call with table parameters does return metadata for each table
				sequence: metadata, resultsetID, resultset
				but:
				- resultset might not be provided for all tables
				- so, 'additional' query result is detected by new metadata part
			*/
			qr = &queryResult{conn: c}
			cr.qrs = append(cr.qrs, qr)
			c.pr.Read(meta)
			qr.fields = meta.ResultFields
		case p.PkResultset:
			resSet.ResultFields = qr.fields
			c.pr.Read(resSet)
			qr.fieldValues = resSet.FieldValues
			qr.decodeErrors = resSet.DecodeErrors
			qr.attributes = ph.PartAttributes
		case p.PkResultsetID:
			c.pr.Read((*p.ResultsetID)(&qr.rsID))
		case p.PkWriteLobReply:
			c.pr.Read(lobReply)
			ids = lobReply.IDs
		}
	}); err != nil {
		return nil, nil, 0, err
	}
	return cr, ids, numRow, nil
}

func (c *conn) _query(pr *prepareResult, nvargs []driver.NamedValue, commit bool) (driver.Rows, error) {
	defer c.addSQLTimeValue(time.Now(), sqlTimeQuery)

	// allow e.g inserts as query -> handle commit like in exec

	hasLob := func() bool {
		for _, f := range pr.parameterFields {
			if f.IsLob() {
				return true
			}
		}
		return false
	}()

	if hasLob {
		if _, err := c._fetchFirstLobChunk(nvargs); err != nil {
			return nil, err
		}
	}
	inputParameters, err := p.NewInputParameters(pr.parameterFields, nvargs, hasLob)
	if err != nil {
		return nil, err
	}
	if err := c.pw.Write(c.sessionID, p.MtExecute, commit, p.StatementID(pr.stmtID), inputParameters); err != nil {
		return nil, err
	}

	qr := &queryResult{conn: c, fields: pr.resultFields}
	resSet := &p.Resultset{}

	if err := c.pr.IterateParts(func(ph *p.PartHeader) {
		switch ph.PartKind {
		case p.PkResultsetID:
			c.pr.Read((*p.ResultsetID)(&qr.rsID))
		case p.PkResultset:
			resSet.ResultFields = qr.fields
			c.pr.Read(resSet)
			qr.fieldValues = resSet.FieldValues
			qr.decodeErrors = resSet.DecodeErrors
			qr.attributes = ph.PartAttributes
		}
	}); err != nil {
		return nil, err
	}
	if qr.rsID == 0 { // non select query
		return noResult, nil
	}
	return qr, nil
}

func (c *conn) _fetchNext(qr *queryResult) error {
	defer c.addSQLTimeValue(time.Now(), sqlTimeFetch)

	if err := c.pw.Write(c.sessionID, p.MtFetchNext, false, p.ResultsetID(qr.rsID), p.Fetchsize(c._fetchSize)); err != nil {
		return err
	}

	resSet := &p.Resultset{ResultFields: qr.fields, FieldValues: qr.fieldValues} // reuse field values

	return c.pr.IterateParts(func(ph *p.PartHeader) {
		if ph.PartKind == p.PkResultset {
			c.pr.Read(resSet)
			qr.fieldValues = resSet.FieldValues
			qr.decodeErrors = resSet.DecodeErrors
			qr.attributes = ph.PartAttributes
		}
	})
}

func (c *conn) _dropStatementID(id uint64) error {
	if err := c.pw.Write(c.sessionID, p.MtDropStatementID, false, p.StatementID(id)); err != nil {
		return err
	}
	return c.pr.ReadSkip()
}

func (c *conn) _closeResultsetID(id uint64) error {
	if err := c.pw.Write(c.sessionID, p.MtCloseResultset, false, p.ResultsetID(id)); err != nil {
		return err
	}
	return c.pr.ReadSkip()
}

func (c *conn) _commit() error {
	defer c.addSQLTimeValue(time.Now(), sqlTimeCommit)

	if err := c.pw.Write(c.sessionID, p.MtCommit, false); err != nil {
		return err
	}
	if err := c.pr.ReadSkip(); err != nil {
		return err
	}
	return nil
}

func (c *conn) _rollback() error {
	defer c.addSQLTimeValue(time.Now(), sqlTimeRollback)

	if err := c.pw.Write(c.sessionID, p.MtRollback, false); err != nil {
		return err
	}
	if err := c.pr.ReadSkip(); err != nil {
		return err
	}
	return nil
}

func (c *conn) _disconnect() error {
	if err := c.pw.Write(c.sessionID, p.MtDisconnect, false); err != nil {
		return err
	}
	/*
		Do not read server reply as on slow connections the TCP/IP connection is closed (by Server)
		before the reply can be read completely.

		// if err := s.pr.readSkip(); err != nil {
		// 	return err
		// }

	*/
	return nil
}

// decodeLobs decodes (reads from db) output lob or result lob parameters.

// read lob reply
// - seems like readLobreply returns only a result for one lob - even if more then one is requested
// --> read single lobs
func (c *conn) decodeLob(descr *p.LobOutDescr, wr io.Writer) error {
	defer c.addSQLTimeValue(time.Now(), sqlTimeFetchLob)

	var err error

	if descr.IsCharBased {
		wrcl := transform.NewWriter(wr, c._cesu8Decoder()) // CESU8 transformer
		err = c._decodeLob(descr, wrcl, func(b []byte) (int64, error) {
			// Caution: hdb counts 4 byte utf-8 encodings (cesu-8 6 bytes) as 2 (3 byte) chars
			numChars := int64(0)
			for len(b) > 0 {
				if !cesu8.FullRune(b) { //
					return 0, fmt.Errorf("lob chunk consists of incomplete CESU-8 runes")
				}
				_, size := cesu8.DecodeRune(b)
				b = b[size:]
				numChars++
				if size == cesu8.CESUMax {
					numChars++
				}
			}
			return numChars, nil
		})
	} else {
		err = c._decodeLob(descr, wr, func(b []byte) (int64, error) { return int64(len(b)), nil })
	}

	if pw, ok := wr.(*io.PipeWriter); ok { // if the writer is a pipe-end -> close at the end
		if err != nil {
			pw.CloseWithError(err)
		} else {
			pw.Close()
		}
	}
	return err
}

func (c *conn) _decodeLob(descr *p.LobOutDescr, wr io.Writer, countChars func(b []byte) (int64, error)) error {
	lobChunkSize := int64(c._lobChunkSize)

	chunkSize := func(numChar, ofs int64) int32 {
		chunkSize := numChar - ofs
		if chunkSize > lobChunkSize {
			return int32(lobChunkSize)
		}
		return int32(chunkSize)
	}

	if _, err := wr.Write(descr.B); err != nil {
		return err
	}

	lobRequest := &p.ReadLobRequest{}
	lobRequest.ID = descr.ID

	lobReply := &p.ReadLobReply{}

	eof := descr.Opt.IsLastData()

	ofs, err := countChars(descr.B)
	if err != nil {
		return err
	}

	for !eof {

		lobRequest.Ofs += ofs
		lobRequest.ChunkSize = chunkSize(descr.NumChar, ofs)

		if err := c.pw.Write(c.sessionID, p.MtWriteLob, false, lobRequest); err != nil {
			return err
		}

		if err := c.pr.IterateParts(func(ph *p.PartHeader) {
			if ph.PartKind == p.PkReadLobReply {
				c.pr.Read(lobReply)
			}
		}); err != nil {
			return err
		}

		if lobReply.ID != lobRequest.ID {
			return fmt.Errorf("internal error: invalid lob locator %d - expected %d", lobReply.ID, lobRequest.ID)
		}

		if _, err := wr.Write(lobReply.B); err != nil {
			return err
		}

		ofs, err = countChars(lobReply.B)
		if err != nil {
			return err
		}
		eof = lobReply.Opt.IsLastData()
	}
	return nil
}

// encodeLobs encodes (write to db) input lob parameters.
func (c *conn) encodeLobs(cr *callResult, ids []p.LocatorID, inPrmFields []*p.ParameterField, nvargs []driver.NamedValue) error {

	descrs := make([]*p.WriteLobDescr, 0, len(ids))

	numInPrmField := len(inPrmFields)

	j := 0
	for i, arg := range nvargs { // range over args (mass / bulk operation)
		f := inPrmFields[i%numInPrmField]
		if f.IsLob() {
			lobInDescr, ok := arg.Value.(*p.LobInDescr)
			if !ok {
				return fmt.Errorf("protocol error: invalid lob parameter %[1]T %[1]v - *lobInDescr expected", arg)
			}
			if j >= len(ids) {
				return fmt.Errorf("protocol error: invalid number of lob parameter ids %d", len(ids))
			}
			descrs = append(descrs, &p.WriteLobDescr{LobInDescr: lobInDescr, ID: ids[j]})
			j++
		}
	}

	writeLobRequest := &p.WriteLobRequest{}

	for len(descrs) != 0 {

		if len(descrs) != len(ids) {
			return fmt.Errorf("protocol error: invalid number of lob parameter ids %d - expected %d", len(descrs), len(ids))
		}
		for i, descr := range descrs { // check if ids and descrs are in sync
			if descr.ID != ids[i] {
				return fmt.Errorf("protocol error: lob parameter id mismatch %d - expected %d", descr.ID, ids[i])
			}
		}

		// TODO check total size limit
		for _, descr := range descrs {
			if err := descr.FetchNext(c._lobChunkSize); err != nil {
				return err
			}
		}

		writeLobRequest.Descrs = descrs

		if err := c.pw.Write(c.sessionID, p.MtReadLob, false, writeLobRequest); err != nil {
			return err
		}

		lobReply := &p.WriteLobReply{}
		outPrms := &p.OutputParameters{}

		if err := c.pr.IterateParts(func(ph *p.PartHeader) {
			switch ph.PartKind {
			case p.PkOutputParameters:
				outPrms.OutputFields = cr.outputFields
				c.pr.Read(outPrms)
				cr.fieldValues = outPrms.FieldValues
				cr.decodeErrors = outPrms.DecodeErrors
			case p.PkWriteLobReply:
				c.pr.Read(lobReply)
				ids = lobReply.IDs
			}
		}); err != nil {
			return err
		}

		// remove done descr
		j := 0
		for _, descr := range descrs {
			if !descr.Opt.IsLastData() {
				descrs[j] = descr
				j++
			}
		}
		descrs = descrs[:j]
	}
	return nil
}
