package hana

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/alexeydott/geom"
	"github.com/alexeydott/tegola/dict"
	"github.com/alexeydott/tegola/provider"
)

// This driver verifies executor sequencing and database/sql contracts only.
// It cannot establish HANA isolation, catalog or planner behavior.
type featureMock struct {
	rows           [][]driver.Value
	queries        []string
	args           [][]driver.NamedValue
	options        driver.TxOptions
	rolled         bool
	corruptCatalog bool
	active         bool
	closed         int
	beginFailure   error
	execFailure    string
	tableFlag      int
	cancelDataNext context.CancelFunc
	dataNextCalls  int
	closedSignal   chan struct{}
	rollbackError  error
	dataCloseError error
	badDataType    bool
	filterVersion  string
	filterColumns  []featureColumn
}
type featureMockConnector struct{ state *featureMock }

func (c featureMockConnector) Connect(context.Context) (driver.Conn, error) {
	return &featureMockConn{c.state}, nil
}
func (c featureMockConnector) Driver() driver.Driver { return featureMockDriver(c) }

type featureMockDriver struct{ state *featureMock }

func (d featureMockDriver) Open(string) (driver.Conn, error) { return &featureMockConn{d.state}, nil }

type featureMockConn struct{ state *featureMock }

func (c *featureMockConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("not supported")
}
func (c *featureMockConn) Close() error {
	c.state.closed++
	if c.state.closedSignal != nil {
		close(c.state.closedSignal)
	}
	return nil
}
func (c *featureMockConn) Begin() (driver.Tx, error) { return nil, errors.New("BeginTx required") }

// Match go-hdb's Validator/SessionResetter interfaces: database/sql then retains
// the reserved connection during automatic rollback for our explicit discard.
func (c *featureMockConn) IsValid() bool                          { return true }
func (c *featureMockConn) ResetSession(ctx context.Context) error { return ctx.Err() }
func (c *featureMockConn) featureWatchContext(context.Context) (func() error, error) {
	return func() error { return nil }, nil
}
func (c *featureMockConn) BeginTx(_ context.Context, options driver.TxOptions) (driver.Tx, error) {
	c.state.options = options
	if c.state.beginFailure != nil {
		return nil, c.state.beginFailure
	}
	return featureMockTx{c.state}, nil
}

func (c *featureMockConn) ExecContext(ctx context.Context, q string, _ []driver.NamedValue) (driver.Result, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	c.state.queries = append(c.state.queries, q)
	if c.state.execFailure != "" && strings.HasPrefix(q, c.state.execFailure) {
		return nil, errors.New("mock setup failure")
	}
	return driver.RowsAffected(0), nil
}

type featureMockTx struct{ state *featureMock }

func (t featureMockTx) Commit() error   { return errors.New("read transaction must rollback") }
func (t featureMockTx) Rollback() error { t.state.rolled = true; return t.state.rollbackError }
func (c *featureMockConn) QueryContext(ctx context.Context, q string, args []driver.NamedValue) (driver.Rows, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s := c.state
	if s.active {
		return nil, errors.New("cursor was not closed before catalog read")
	}
	s.queries = append(s.queries, q)
	s.args = append(s.args, append([]driver.NamedValue(nil), args...))
	var labels, types []string
	var rows [][]driver.Value
	switch {
	case strings.Contains(q, "SELECT VERSION FROM SYS.M_DATABASE"):
		labels = []string{"VERSION"}
		rows = [][]driver.Value{{s.filterVersion}}
	case strings.Contains(q, "SELECT CURRENT_CONNECTION"):
		labels = []string{"CURRENT_CONNECTION"}
		rows = [][]driver.Value{{int64(1)}}
	case strings.Contains(q, "FROM SYS.TABLES"):
		labels = []string{"TABLE_OID", "TABLE_TYPE", "IS_TEMPORARY", "HAS_STRUCTURED_PRIVILEGE_CHECK", "HAS_MASKED_COLUMNS", "TEMPORAL_TYPE", "SESSION_TYPE"}
		oid := int64(1)
		if s.corruptCatalog {
			oid = 2
		}
		rows = [][]driver.Value{{oid, "ROW", "FALSE", "FALSE", "FALSE", nil, "NONE"}}
		if s.tableFlag > 0 {
			rows[0][s.tableFlag] = "TRUE"
		}
	case strings.Contains(q, "FROM SYS.TABLE_COLUMNS"):
		labels = []string{"COLUMN_NAME", "COLUMN_ID", "DATA_TYPE_NAME", "LENGTH", "SCALE", "IS_NULLABLE", "COLLATION", "GENERATED_ALWAYS_AS", "GENERATION_TYPE", "IS_HIDDEN", "IS_MASKED"}
		columns := s.filterColumns
		if columns == nil {
			columns = featureTestLayer().feature.Catalog.Columns
		}
		for _, column := range columns {
			rows = append(rows, []driver.Value{column.Name, column.ID, column.Type, column.Length, column.Scale, "FALSE", nil, nil, nil, "FALSE", "FALSE"})
		}
	case strings.Contains(q, "FROM SYS.INDEXES"):
		labels = []string{"INDEX_NAME", "COLUMN_NAME"}
		rows = [][]driver.Value{{"PK", "id"}}
	default:
		labels = []string{"id", "geom", "at", "name"}
		types = []string{"BIGINT", "VARCHAR", "BIGINT", "NVARCHAR"}
		if s.badDataType {
			types[1] = "UNKNOWN"
		}
		rows = s.rows
	}
	s.active = true
	return &featureMockRows{state: s, labels: labels, types: types, rows: rows}, nil
}

type featureMockRows struct {
	state         *featureMock
	labels, types []string
	rows          [][]driver.Value
	next          int
}

func (r *featureMockRows) Columns() []string { return r.labels }
func (r *featureMockRows) Close() error {
	r.state.active = false
	if len(r.types) != 0 {
		return r.state.dataCloseError
	}
	return nil
}
func (r *featureMockRows) Next(dst []driver.Value) error {
	if len(r.types) != 0 {
		r.state.dataNextCalls++
		if r.state.cancelDataNext != nil {
			r.state.cancelDataNext()
			return context.Canceled
		}
	}
	if r.next == len(r.rows) {
		return io.EOF
	}
	copy(dst, r.rows[r.next])
	r.next++
	return nil
}
func (r *featureMockRows) ColumnTypeDatabaseTypeName(i int) string {
	if len(r.types) == 0 {
		return ""
	}
	return r.types[i]
}

func featureMockProvider(t *testing.T, state *featureMock) *Provider {
	t.Helper()
	db := sql.OpenDB(featureMockConnector{state})
	t.Cleanup(func() { _ = db.Close() })
	l := featureTestLayer()
	l.feature.Catalog.TableType = "ROW"
	l.feature.Catalog.SessionType = "NONE"
	l.feature.Catalog.Temporary = "FALSE"
	l.feature.Catalog.Policy = "FALSE"
	l.feature.Catalog.Masked = "FALSE"
	l.name = "features"
	return &Provider{pool: &connectionPoolCollector{pool: db}, layers: map[string]Layer{"features": l}}
}

func TestFeatureSnapshotExactPagingAndSourceGuard(t *testing.T) {
	s := &featureMock{rows: [][]driver.Value{
		{int64(1), "LINESTRING(-1 3,3 -1)", int64(0), "false positive"},
		{int64(2), "POINT(0.2 0.2)", int64(0), "selected"},
		{int64(3), nil, int64(0), "absence lookahead"},
	}}
	p := featureMockProvider(t, s)
	var returned []uint64
	r, err := p.queryFeaturesGuarded(context.Background(), "features", provider.FeatureQuery{Bounds: []geom.Extent{{0, 0, 0.5, 0.5}}, BoundsSRID: 4326, Limit: 1}, func(f *provider.Feature) error { returned = append(returned, f.ID); return nil })
	if err != nil || len(returned) != 1 || returned[0] != 2 || !r.HasMore || r.NumberMatched != nil {
		t.Fatalf("result=%+v IDs=%v err=%v", r, returned, err)
	}
	if !s.rolled || s.options.ReadOnly || s.options.Isolation != driver.IsolationLevel(sql.LevelRepeatableRead) {
		t.Fatalf("snapshot %+v", s)
	}
	if !strings.Contains(strings.Join(s.queries, " "), "LIMIT 256") || len(s.queries) < 4 {
		t.Fatal(s.queries)
	}
	s = &featureMock{rows: [][]driver.Value{{int64(1), "POINT(0 0)", int64(0), "hidden"}}, corruptCatalog: true}
	p = featureMockProvider(t, s)
	called := false
	_, err = p.queryFeaturesGuarded(context.Background(), "features", provider.FeatureQuery{Limit: 1}, func(*provider.Feature) error { called = true; return nil })
	var data provider.FeatureDataError
	if called || !errors.As(err, &data) || !s.rolled {
		t.Fatalf("schema guard called=%v err=%v", called, err)
	}
}

func TestFeatureSnapshotCancellationAndErrors(t *testing.T) {
	callbackErr := errors.New("callback sentinel")
	for _, cancelAfterCallback := range []bool{false, true} {
		s := &featureMock{rows: [][]driver.Value{{int64(1), "POINT(0 0)", int64(0), "first"}, {int64(2), "bad WKT", int64(0), "later malformed"}}}
		p := featureMockProvider(t, s)
		ctx, cancel := context.WithCancel(context.Background())
		called := 0
		_, err := p.queryFeaturesGuarded(ctx, "features", provider.FeatureQuery{Limit: 10}, func(*provider.Feature) error {
			called++
			if cancelAfterCallback {
				cancel()
				return nil
			}
			return callbackErr
		})
		cancel()
		want := callbackErr
		if cancelAfterCallback {
			want = context.Canceled
		}
		if !errors.Is(err, want) || called != 1 || !s.rolled {
			t.Fatalf("called=%d err=%v rollback=%v", called, err, s.rolled)
		}
	}
	s := &featureMock{rows: [][]driver.Value{{int64(1), "bad WKT", int64(0), "bad"}}}
	p := featureMockProvider(t, s)
	_, err := p.queryFeaturesGuarded(context.Background(), "features", provider.FeatureQuery{Limit: 1}, func(*provider.Feature) error { t.Fatal("corruption exposed"); return nil })
	var data provider.FeatureDataError
	if !errors.As(err, &data) {
		t.Fatalf("source integrity classification: %v", err)
	}
	s = &featureMock{}
	p = featureMockProvider(t, s)
	_, err = p.queryFeaturesGuarded(context.Background(), "features", provider.FeatureQuery{Limit: 1, Fields: []string{"malicious; DROP TABLE"}}, func(*provider.Feature) error { return nil })
	var invalid provider.InvalidFeatureQueryError
	if !errors.As(err, &invalid) || len(s.queries) != 0 {
		t.Fatalf("invalid query reached I/O: %v %v", err, s.queries)
	}
}

func TestFeatureConfiguredPublicFieldsAndSourceCRS(t *testing.T) {
	s := &featureMock{rows: [][]driver.Value{{int64(1), "POINT(0 0)", int64(0), "name"}}}
	p := featureMockProvider(t, s)
	l := featureTestLayer()
	l.featureTable = `"S"."T"`
	l.srid = 1000004326
	if err := p.registerFeatureSource(&l, dict.Dict{"fields": []string{"name"}, "temporal_field": "at", "temporal_storage": "unix_seconds"}, ProviderType); err != nil {
		t.Fatal(err)
	}
	if l.featureError != nil || l.FeatureSourceSRID() != 4326 || l.SRID() != 1000004326 {
		t.Fatalf("profile source=%d tile=%d err=%v", l.FeatureSourceSRID(), l.SRID(), l.FeatureQuerySupported())
	}
	p.layers["features"] = l
	_, err := p.queryFeaturesGuarded(context.Background(), "features", provider.FeatureQuery{Limit: 1}, func(f *provider.Feature) error {
		if len(f.Tags) != 1 || f.Tags["name"] != "name" || f.SRID != 4326 {
			t.Fatal(f)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	before := len(s.queries)
	_, err = p.queryFeaturesGuarded(context.Background(), "features", provider.FeatureQuery{Limit: 1, Fields: []string{"at"}}, func(*provider.Feature) error { return nil })
	var invalid provider.InvalidFeatureQueryError
	if !errors.As(err, &invalid) || len(s.queries) != before {
		t.Fatalf("unpublished field admitted: %v", err)
	}
}

func TestFeatureProductionAdmissionAndPhysicalDiscard(t *testing.T) {
	s := &featureMock{}
	p := featureMockProvider(t, s)
	_, err := p.QueryFeatures(context.Background(), "features", provider.FeatureQuery{Limit: 1}, func(*provider.Feature) error { return nil })
	if err != nil || s.closed != 1 || !s.rolled || s.options.ReadOnly {
		t.Fatalf("guarded query failed physical discard: %v %+v", err, s)
	}
	setupErr := errors.New("partial setup changed access mode")
	s = &featureMock{beginFailure: setupErr}
	p = featureMockProvider(t, s)
	_, err = p.queryFeaturesGuarded(context.Background(), "features", provider.FeatureQuery{Limit: 1}, func(*provider.Feature) error { return nil })
	if !errors.Is(err, setupErr) || s.closed != 1 {
		t.Fatalf("setup failure returned session to pool: %v %+v", err, s)
	}
}

func TestFeatureRequestedTemporalProperties(t *testing.T) {
	for _, customAlias := range []bool{false, true} {
		for _, selectedTime := range []bool{false, true} {
			s := &featureMock{rows: [][]driver.Value{{int64(1), "POINT(0 0)", nil, "name"}}}
			p := featureMockProvider(t, s)
			output := "at"
			if customAlias {
				l := p.layers["features"]
				l.feature.Temporal.InstantField = "observed"
				l.feature.Projections[2].Output = "observed"
				p.layers["features"] = l
				output = "observed"
			}
			fields := []string{"name"}
			if selectedTime {
				fields = []string{output}
			}
			_, err := p.queryFeaturesGuarded(context.Background(), "features", provider.FeatureQuery{Limit: 1, Fields: fields}, func(f *provider.Feature) error {
				if len(f.Tags) != 1 {
					t.Fatalf("required temporal read leaked tags: %#v", f.Tags)
				}
				value, present := f.Tags[output]
				if selectedTime {
					if !present || value != nil {
						t.Fatalf("explicit NULL property lost: %#v", f.Tags)
					}
				} else if present || f.Tags["name"] != "name" {
					t.Fatalf("unrequested temporal property leaked: %#v", f.Tags)
				}
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
		}
	}
}

func TestFeatureSetupFailuresDiscardPhysicalSession(t *testing.T) {
	for _, prefix := range []string{"SET TRANSACTION", "LOCK TABLE"} {
		s := &featureMock{execFailure: prefix}
		p := featureMockProvider(t, s)
		calls := 0
		_, err := p.QueryFeatures(context.Background(), "features", provider.FeatureQuery{Limit: 1}, func(*provider.Feature) error { calls++; return nil })
		if err == nil || calls != 0 || s.closed != 1 || !s.rolled {
			t.Fatalf("setup failure leaked session %+v error%v", s, err)
		}
	}
}

func TestFeatureTableFlagsFailBeforeCallbacks(t *testing.T) {
	for _, flag := range []int{2, 3, 4, 5, 6} {
		s := &featureMock{tableFlag: flag}
		p := featureMockProvider(t, s)
		calls := 0
		_, err := p.QueryFeatures(context.Background(), "features", provider.FeatureQuery{Limit: 1}, func(*provider.Feature) error { calls++; return nil })
		var data provider.FeatureDataError
		if calls != 0 || !errors.As(err, &data) || s.closed != 1 || !s.rolled {
			t.Fatalf("table policy drift admitted flag%d %+v %v", flag, s, err)
		}
	}
}

func TestFeatureCancellationDuringRowsNext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s := &featureMock{cancelDataNext: cancel, closedSignal: make(chan struct{})}
	p := featureMockProvider(t, s)
	calls := 0
	_, err := p.QueryFeatures(ctx, "features", provider.FeatureQuery{Limit: 1}, func(*provider.Feature) error { calls++; return nil })
	select {
	case <-s.closedSignal:
	case <-time.After(time.Second):
		t.Fatal("cancelled physical session did not close")
	}
	var data provider.FeatureDataError
	if !errors.Is(err, context.Canceled) || errors.As(err, &data) || calls != 0 || s.dataNextCalls != 1 || s.closed != 1 || s.active {
		t.Fatalf("row-phase cancellation broken error%v state%+v callbacks%d", err, s, calls)
	}
}

func TestFeaturePrimaryAndCleanupErrorChains(t *testing.T) {
	primary := errors.New("callback marker")
	rollback := errors.New("rollback marker")
	s := &featureMock{rows: [][]driver.Value{{int64(1), "POINT(0 0)", nil, "name"}}, rollbackError: rollback}
	p := featureMockProvider(t, s)
	_, err := p.QueryFeatures(context.Background(), "features", provider.FeatureQuery{Limit: 1}, func(*provider.Feature) error { return primary })
	if !errors.Is(err, primary) || !errors.Is(err, rollback) {
		t.Fatal("callback/rollback chain lost")
	}
	closeMarker := errors.New("rows close marker")
	s = &featureMock{badDataType: true, dataCloseError: closeMarker}
	p = featureMockProvider(t, s)
	_, err = p.QueryFeatures(context.Background(), "features", provider.FeatureQuery{Limit: 1}, func(*provider.Feature) error { t.Fatal("bad metadata callback"); return nil })
	var invalid provider.InvalidFeatureQueryError
	if !errors.Is(err, closeMarker) || !errors.As(err, &invalid) {
		t.Fatal("metadata/rows close chain lost")
	}
}
