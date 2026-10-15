package driver

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"iter"
	"slices"
	"sync"
	"sync/atomic"
)

// check if statements implements all required interfaces.
var (
	_ driver.Stmt              = (*stmt)(nil)
	_ driver.StmtExecContext   = (*stmt)(nil)
	_ driver.StmtQueryContext  = (*stmt)(nil)
	_ driver.NamedValueChecker = (*stmt)(nil)
)

type stmt struct {
	conn    *conn
	wg      *sync.WaitGroup // from conn
	attrs   *connAttrs
	metrics *metrics
	query   string
	pr      *prepareResult
	closed  atomic.Bool // guards close against repeated teardown.
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

func newStmt(conn *conn, wg *sync.WaitGroup, attrs *connAttrs, metrics *metrics, query string, pr *prepareResult) *stmt {
	metrics.addGauge(gaugeStmt, 1) // increment number of statements.
	return &stmt{conn: conn, wg: wg, attrs: attrs, metrics: metrics, query: query, pr: pr}
}

/*
NumInput depends on the statement (checked in QueryContext and ExecContext):
- #args == #param (only in params):    query, exec, exec bulk (non control query)
- #args == #param (in and out params): exec call
- #args == 0:                          exec bulk (control query)
- #args == #input param:               query call.
*/
func (s *stmt) NumInput() int { return -1 }

func (s *stmt) Close() error {
	// While a table-out tracker is attached app-driven closes
	// are no-ops; teardown runs on the worker.
	if s.conn.tableOutTracker != nil {
		return nil
	}
	return s.close()
}

// close drops the server-side statement and decrements the statement gauge.
func (s *stmt) close() error {
	if !s.closed.CompareAndSwap(false, true) {
		return nil
	}
	s.metrics.addGauge(gaugeStmt, -1) // decrement number of statements.

	if s.conn.session.isBad() {
		return driver.ErrBadConn
	}
	return s.conn.session.dropStatementID(context.Background(), s.pr.stmtID)
}

// CheckNamedValue implements NamedValueChecker interface.
func (s *stmt) CheckNamedValue(nv *driver.NamedValue) error {
	// conversion is happening as part of the exec, query call
	return nil
}

func (s *stmt) QueryContext(ctx context.Context, nvargs []driver.NamedValue) (driver.Rows, error) {
	if s.pr.isProcedureCall() {
		return nil, fmt.Errorf("invalid procedure call %s - please use Exec instead", s.query)
	}
	if err := s.conn.session.preventSwitchUser(ctx); err != nil {
		return nil, err
	}

	var sqlErr error
	var rows driver.Rows
	done := make(chan struct{})
	s.wg.Go(func() {
		defer close(done)
		rows, sqlErr = s.conn.session.query(ctx, s.query, s.pr, nvargs)
	})

	select {
	case <-ctx.Done():
		s.conn.terminateSession()
		return nil, ctx.Err()
	case <-done:
		return rows, sqlErr
	}
}

func (s *stmt) ExecContext(ctx context.Context, nvargs []driver.NamedValue) (driver.Result, error) {
	if hookFn, ok := ctx.Value(connHookCtxKey).(connHookFn); ok {
		hookFn(choStmtExec)
	}
	if err := s.conn.session.preventSwitchUser(ctx); err != nil {
		return nil, err
	}

	var sqlErr error
	var result driver.Result
	done := make(chan struct{})
	s.wg.Go(func() {
		defer close(done)
		if s.pr.isProcedureCall() {
			result, sqlErr = s.execCall(ctx, s.pr, nvargs)
		} else {
			result, sqlErr = s.execDefault(ctx, nvargs)
		}
	})

	select {
	case <-ctx.Done():
		s.conn.terminateSession()
		return nil, ctx.Err()
	case <-done:
		return result, sqlErr
	}
}

func (s *stmt) execCall(ctx context.Context, pr *prepareResult, nvargs []driver.NamedValue) (driver.Result, error) {
	/*
		call without lob input parameters:
		--> callResult output parameter values are set after read call
		call with lob output parameters:
		--> callResult output parameter values are set after last lob input write
	*/

	cr, callArgs, numRow, err := s.conn.session.execCall(ctx, s.query, pr, nvargs)
	if err != nil {
		return nil, err
	}

	numOutArgs := len(callArgs.outArgs)
	// no output args -> done
	if numOutArgs == 0 {
		return driver.RowsAffected(numRow), nil
	}

	numOutputField := len(cr.outFields)
	scanArgs := make([]any, numOutputField)
	for i := range numOutArgs {
		scanArgs[i] = callArgs.outArgs[i].Value.(sql.Out).Dest
	}
	// account for table output fields without call arguments.
	for i := numOutArgs; i < numOutputField; i++ {
		scanArgs[i] = new(sql.Rows)
	}

	// no table output parameters -> convert scalar output parameters
	if len(callArgs.outFields) == numOutArgs {
		if err := convertCallResult(cr, scanArgs); err != nil {
			return nil, err
		}
		return driver.RowsAffected(numRow), nil
	}

	// table output resultset -> Query (kept open on success; the table-out
	// tracker owns the fake parent rows).
	rows, err := stdCallDB.db().QueryContext(context.Background(), "", cr)
	if err != nil {
		return nil, err
	}
	if !rows.Next() {
		_ = rows.Close()
		return nil, rows.Err()
	}
	if err := rows.Scan(scanArgs...); err != nil {
		_ = rows.Close() //nolint: sqlclosecheck // fake parent, closed explicitly on the error paths only
		return nil, err
	}
	// the call succeeded and handed the table output rows to the caller: wire
	// the tracker to the resultsets and the connection, holding it for the
	// worker. On any error path the resultsets stay tracker-less,
	// close without accounting, and never hold.
	tracker := &tableOutTracker{stmt: s, rows: rows, lc: s.conn.lifecycle, done: make(chan struct{})}
	for _, v := range cr.fieldValues {
		if qr, ok := v.(*queryResult); ok {
			qr.tableOutTracker = tracker
			tracker.n.Add(1)
		}
	}
	s.conn.tableOutTracker = tracker
	return driver.RowsAffected(numRow), nil
}

func (s *stmt) execDefault(ctx context.Context, nvargs []driver.NamedValue) (driver.Result, error) {
	numNVArg, numField := len(nvargs), s.pr.numField()

	if numNVArg == 0 {
		if numField != 0 {
			return nil, fmt.Errorf("invalid number of arguments %d - expected %d", numNVArg, numField)
		}
		return s.conn.session.exec(ctx, s.query, s.pr, nvargs, 0)
	}
	if numNVArg == 1 {
		switch nvargs[0].Value.(type) {
		case func(args []any) error:
			return s.execFct(ctx, nvargs)
		case iter.Seq[[]any]:
			return s.execSeq(ctx, nvargs)
		}
	}
	if numNVArg == numField {
		return s.exec(ctx, s.pr, nvargs, 0)
	}
	if numField == 0 {
		return nil, fmt.Errorf("invalid number of arguments %d - expected 0", numNVArg)
	}
	if numNVArg%numField != 0 {
		return nil, fmt.Errorf("invalid number of arguments %d - multiple of %d expected", numNVArg, numField)
	}
	return s.execMany(ctx, nvargs)
}

// ErrEndOfRows is the error to be returned using a function based bulk exec to indicate
// the end of rows.
var ErrEndOfRows = errors.New("end of rows")

/*
Non 'atomic' (transactional) operation due to the split in packages (bulkSize),
execMany data might only be written partially to the database in case of hdb stmt errors.
*/
func (s *stmt) execFct(ctx context.Context, nvargs []driver.NamedValue) (driver.Result, error) {
	bulkSize := s.attrs.bulkSize

	totalRowsAffected := totalRowsAffected(0)
	args := make([]driver.NamedValue, 0, s.pr.numField())
	scanArgs := make([]any, s.pr.numField())

	fct, ok := nvargs[0].Value.(func(args []any) error)
	if !ok {
		panic("invalid argument") // should never happen
	}

	done := false
	batch := 0
	for !done {
		args = args[:0]
		for range bulkSize {
			err := fct(scanArgs)
			if errors.Is(err, ErrEndOfRows) {
				done = true
				break
			}
			if err != nil {
				return driver.RowsAffected(totalRowsAffected), err
			}

			args = slices.Grow(args, len(scanArgs))
			for i, scanArg := range scanArgs {
				nv := driver.NamedValue{Ordinal: i + 1}
				if t, ok := scanArg.(sql.NamedArg); ok {
					nv.Name = t.Name
					nv.Value = t.Value
				} else {
					nv.Name = ""
					nv.Value = scanArg
				}
				args = append(args, nv)
			}
		}

		r, err := s.exec(ctx, s.pr, args, batch*bulkSize)
		totalRowsAffected.add(r)
		if err != nil {
			return driver.RowsAffected(totalRowsAffected), err
		}
		batch++
	}
	return driver.RowsAffected(totalRowsAffected), nil
}

func (s *stmt) execSeq(ctx context.Context, nvargs []driver.NamedValue) (driver.Result, error) {
	bulkSize := s.attrs.bulkSize

	totalRowsAffected := totalRowsAffected(0)
	args := make([]driver.NamedValue, 0, s.pr.numField())

	seq, ok := nvargs[0].Value.(iter.Seq[[]any])
	if !ok {
		panic("invalid argument") // should never happen
	}

	batch, n := 0, 0
	for scanArgs := range seq {
		if len(scanArgs) != s.pr.numField() {
			return driver.RowsAffected(totalRowsAffected), fmt.Errorf("invalid number of args %d - expected %d", len(scanArgs), s.pr.numField())
		}

		args = slices.Grow(args, len(scanArgs))
		for i, scanArg := range scanArgs {
			nv := driver.NamedValue{Ordinal: i + 1}
			if t, ok := scanArg.(sql.NamedArg); ok {
				nv.Name = t.Name
				nv.Value = t.Value
			} else {
				nv.Name = ""
				nv.Value = scanArg
			}
			args = append(args, nv)
		}

		n++
		if n >= bulkSize {
			r, err := s.exec(ctx, s.pr, args, batch*bulkSize)
			totalRowsAffected.add(r)
			if err != nil {
				return driver.RowsAffected(totalRowsAffected), err
			}
			args = args[:0]
			n = 0
			batch++
		}
	}

	if n > 0 {
		r, err := s.exec(ctx, s.pr, args, batch*bulkSize)
		totalRowsAffected.add(r)
		if err != nil {
			return driver.RowsAffected(totalRowsAffected), err
		}
	}

	return driver.RowsAffected(totalRowsAffected), nil
}

/*
Non 'atomic' (transactional) operation due to the split in packages (bulkSize),
execMany data might only be written partially to the database in case of hdb stmt errors.
*/
func (s *stmt) execMany(ctx context.Context, nvargs []driver.NamedValue) (driver.Result, error) {
	bulkSize := s.attrs.bulkSize

	totalRowsAffected := totalRowsAffected(0)
	numField := s.pr.numField()
	numNVArg := len(nvargs)
	numRec := numNVArg / numField
	numBatch := numRec / bulkSize
	if numRec%bulkSize != 0 {
		numBatch++
	}

	for i := range numBatch {
		from := i * numField * bulkSize
		to := min((i+1)*numField*bulkSize, numNVArg)
		r, err := s.exec(ctx, s.pr, nvargs[from:to], i*bulkSize)
		totalRowsAffected.add(r)
		if err != nil {
			return driver.RowsAffected(totalRowsAffected), err
		}
	}
	return driver.RowsAffected(totalRowsAffected), nil
}

/*
exec executes a sql statement.

Bulk insert containing LOBs:
  - Precondition:
    .Sending more than one row with partial LOB data.
  - Observations:
    .In hdb version 1 and 2 'piecewise' LOB writing does work.
    .Same does not work in case of geo fields which are LOBs encoded and decoded as well.
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
func (s *stmt) exec(ctx context.Context, pr *prepareResult, nvargs []driver.NamedValue, ofs int) (driver.Result, error) {
	addLobDataRecs, err := convertExecArgs(pr.parameterFields, nvargs, s.attrs.cesu8EncoderFn, s.attrs.lobChunkSize)
	if err != nil {
		return driver.ResultNoRows, err
	}

	// piecewise LOB handling
	numColumn := len(pr.parameterFields)
	totalRowsAffected := totalRowsAffected(0)
	from := 0
	for _, row := range addLobDataRecs {
		to := (row + 1) * numColumn

		r, err := s.conn.session.exec(ctx, s.query, pr, nvargs[from:to], ofs+from/numColumn)
		totalRowsAffected.add(r)
		if err != nil {
			return driver.RowsAffected(totalRowsAffected), err
		}
		from = to
	}
	return driver.RowsAffected(totalRowsAffected), nil
}
