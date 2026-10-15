package mysql

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/alexeydott/geom"
	"github.com/alexeydott/tegola/provider"
)

// This driver is SQL/lifecycle unit evidence, not backend parity evidence.
type featureTestStore struct {
	rolledBack                    chan struct{}
	mu                            sync.Mutex
	profile                       *featureProfile
	data                          [][]driver.Value
	physicalID                    string
	queries                       []string
	options                       driver.TxOptions
	opened, closed, rollbacks     int
	rowsCloseError, rowsReadError error
}

type featureTestConnector struct{ store *featureTestStore }

func (c featureTestConnector) Connect(context.Context) (driver.Conn, error) {
	return &featureTestConn{store: c.store}, nil
}
func (c featureTestConnector) Driver() driver.Driver { return featureTestDriver(c) }

type featureTestDriver struct{ store *featureTestStore }

func (d featureTestDriver) Open(string) (driver.Conn, error) {
	return &featureTestConn{store: d.store}, nil
}

type featureTestConn struct{ store *featureTestStore }

func (*featureTestConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("unused prepare")
}
func (*featureTestConn) Close() error                             { return nil }
func (*featureTestConn) Begin() (driver.Tx, error)                { return nil, errors.New("must use BeginTx") }
func (*featureTestConn) CheckNamedValue(*driver.NamedValue) error { return nil }
func (c *featureTestConn) BeginTx(ctx context.Context, opts driver.TxOptions) (driver.Tx, error) {
	c.store.mu.Lock()
	defer c.store.mu.Unlock()
	c.store.options = opts
	return &featureTestTx{store: c.store}, ctx.Err()
}

type featureTestTx struct{ store *featureTestStore }

func (*featureTestTx) Commit() error { return errors.New("read snapshot must roll back") }
func (tx *featureTestTx) Rollback() error {
	tx.store.mu.Lock()
	defer tx.store.mu.Unlock()
	tx.store.rollbacks++
	close(tx.store.rolledBack)
	return nil
}

func (c *featureTestConn) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s := c.store
	s.mu.Lock()
	defer s.mu.Unlock()
	s.queries = append(s.queries, query)
	f := s.profile
	result := &featureTestRows{store: s}
	switch {
	case query == "SELECT @@lower_case_table_names":
		result.columns = []string{"mode"}
		result.data = [][]driver.Value{{int64(f.schema.lowerCaseTables)}}
	case strings.Contains(query, "FROM INFORMATION_SCHEMA.TABLES"):
		if strings.HasPrefix(query, "SELECT TABLE_SCHEMA,TABLE_NAME FROM") {
			result.columns = []string{"database", "table"}
			result.data = [][]driver.Value{{f.schema.database, f.schema.table}}
		} else {
			result.columns = []string{"database", "table", "type", "engine"}
			result.data = [][]driver.Value{{f.schema.database, f.schema.table, f.schema.tableType, f.schema.engine}}
		}
	case query == "SELECT VERSION()":
		result.columns = []string{"version"}
		result.data = [][]driver.Value{{f.schema.serverVersion}}
	case strings.Contains(query, "SELECT TABLE_ID"):
		result.columns = []string{"id"}
		result.data = [][]driver.Value{{s.physicalID}}
	case strings.Contains(query, "FROM INFORMATION_SCHEMA.COLUMNS"):
		result.columns = []string{"name", "type", "column_type", "nullable", "extra", "collation", "charset", "srid"}
		for _, column := range f.schema.columns {
			var srid driver.Value
			if column.srid.Valid {
				srid = column.srid.Int64
			}
			result.data = append(result.data, []driver.Value{column.name, column.dataType, column.columnType, column.nullable, column.extra, column.collation, column.characterSet, srid})
		}
	case strings.Contains(query, "FROM INFORMATION_SCHEMA.STATISTICS"):
		result.columns = []string{"name", "sequence", "non_unique", "column", "prefix"}
		for _, index := range f.schema.indexes {
			var prefix driver.Value
			if index.prefix.Valid {
				prefix = index.prefix.Int64
			}
			result.data = append(result.data, []driver.Value{index.name, int64(index.position), int64(index.nonUnique), index.column, prefix})
		}
	case strings.HasSuffix(query, "LIMIT 0"):
		result.columns = []string{f.id}
	default:
		result.columns, _ = f.queryColumns(f.properties)
		for _, row := range s.data {
			if row[0] == nil {
				continue
			}
			if strings.Contains(query, ">?") && len(args) != 0 {
				cursor, _ := provider.ConvertFeatureID(args[len(args)-1].Value)
				id, _ := provider.ConvertFeatureID(row[0])
				if id <= cursor {
					continue
				}
			}
			fixtureValues := map[string]driver.Value{"id": row[0], "geom": row[1], "name": row[2], "start": row[3], "end": row[4]}
			selected := make([]driver.Value, len(result.columns))
			for i, column := range result.columns {
				selected[i] = fixtureValues[column]
			}
			result.data = append(result.data, selected)
			if len(result.data) == featureChunkSize {
				break
			}
		}
		result.closeError, result.readError = s.rowsCloseError, s.rowsReadError
	}
	s.opened++
	return result, nil
}

type featureTestRows struct {
	store                 *featureTestStore
	columns               []string
	data                  [][]driver.Value
	next                  int
	closeError, readError error
}

func (r *featureTestRows) Columns() []string { return r.columns }
func (r *featureTestRows) Close() error {
	r.store.mu.Lock()
	defer r.store.mu.Unlock()
	r.store.closed++
	return r.closeError
}
func (r *featureTestRows) Next(target []driver.Value) error {
	if r.next == len(r.data) {
		if r.readError != nil {
			return r.readError
		}
		return io.EOF
	}
	copy(target, r.data[r.next])
	r.next++
	return nil
}

func newFeatureTestProvider(t *testing.T, rows [][]driver.Value) (*Provider, *featureTestStore) {
	t.Helper()
	f := testFeatureProfile(t, false)
	f.schema.physicalID, f.schema.serverVersion = "42", "8.4.2"
	store := &featureTestStore{profile: f, data: rows, physicalID: "42", rolledBack: make(chan struct{})}
	db := sql.OpenDB(featureTestConnector{store: store})
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Error(err)
		}
	})
	return &Provider{db: db, layers: map[string]Layer{"items": {name: "items", feature: f}}}, store
}

func TestFeatureSnapshotChunkExactBeforePaging(t *testing.T) {
	rows := make([][]driver.Value, 600)
	for i := range rows {
		geometry := "POINT(50 50)"
		if i%100 == 0 {
			geometry = "POINT(1 1)"
		}
		rows[i] = []driver.Value{int64(i + 1), geometry, []byte("name"), nil, nil}
	}
	p, store := newFeatureTestProvider(t, rows)
	query := provider.FeatureQuery{Limit: 2, Offset: 2, BoundsSRID: 4326, Bounds: []geom.Extent{{0, 0, 2, 2}}}
	ids := []uint64{}
	result, err := p.QueryFeatures(context.Background(), "items", query, func(feature *provider.Feature) error { ids = append(ids, feature.ID); return nil })
	if err != nil || !reflect.DeepEqual(ids, []uint64{201, 301}) || !result.HasMore || result.NumberMatched != nil {
		t.Fatalf("page: %v %#v %v", ids, result, err)
	}
	if store.options.Isolation != driver.IsolationLevel(sql.LevelRepeatableRead) || !store.options.ReadOnly || store.rollbacks != 1 {
		t.Fatalf("snapshot setup: %#v", store)
	}
	if store.opened != store.closed {
		t.Fatalf("cursor leak: %d/%d", store.opened, store.closed)
	}
	ids = []uint64{}
	store.rolledBack = make(chan struct{})
	query.Limit, query.Offset = 10, 0
	result, err = p.QueryFeatures(context.Background(), "items", query, func(feature *provider.Feature) error { ids = append(ids, feature.ID); return nil })
	if err != nil || result.HasMore || result.NumberMatched == nil || *result.NumberMatched != 6 || len(ids) != 6 {
		t.Fatalf("exhausted total: %#v %v", result, err)
	}
}

func TestFeatureRequestedFieldsHidePrivateTemporalReads(t *testing.T) {
	p, store := newFeatureTestProvider(t, [][]driver.Value{{int64(1), "POINT(1 1)", []byte("name"), nil, nil}})
	f := store.profile
	f.publicTemporalNulls = true
	f.properties = []string{"name", "start", "end"}
	f.temporal = provider.TemporalMapping{StartField: "start", EndField: "end"}
	f.temporalScale = 1
	_, err := p.QueryFeatures(context.Background(), "items", provider.FeatureQuery{Limit: 10, Fields: []string{"name"}}, func(feature *provider.Feature) error {
		if !reflect.DeepEqual(feature.Tags, map[string]any{"name": "name"}) {
			t.Errorf("requested fields leaked private temporal reads: %#v", feature.Tags)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestFeaturePhysicalReplacementFailsBeforeCallbacks(t *testing.T) {
	p, store := newFeatureTestProvider(t, [][]driver.Value{{int64(1), "POINT(1 1)", nil, nil, nil}})
	store.physicalID = "43" // All column/index metadata remains byte-for-byte identical.
	_, err := p.QueryFeatures(context.Background(), "items", provider.FeatureQuery{Limit: 1}, func(*provider.Feature) error { t.Fatal("replacement emitted feature"); return nil })
	var data provider.FeatureDataError
	if !errors.As(err, &data) || store.opened != store.closed || store.rollbacks != 1 {
		t.Fatalf("replacement guard: %v %#v", err, store)
	}
}

func TestFeatureRowsAndCallbackErrorChains(t *testing.T) {
	for _, mode := range []string{"callback", "close", "iteration", "cancel"} {
		t.Run(mode, func(t *testing.T) {
			p, store := newFeatureTestProvider(t, [][]driver.Value{{int64(1), "POINT(1 1)", nil, nil, nil}})
			sentinel := errors.New("unit sentinel")
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if mode == "close" {
				store.rowsCloseError = sentinel
			}
			if mode == "iteration" {
				store.rowsReadError = sentinel
			}
			_, err := p.QueryFeatures(ctx, "items", provider.FeatureQuery{Limit: 10}, func(*provider.Feature) error {
				if mode == "callback" {
					return sentinel
				}
				if mode == "cancel" {
					cancel()
				}
				return nil
			})
			want := sentinel
			if mode == "cancel" {
				want = context.Canceled
			}
			select {
			case <-store.rolledBack:
			case <-time.After(time.Second):
				t.Fatal("snapshot cleanup did not complete")
			}
			store.mu.Lock()
			opened, closed, rollbacks := store.opened, store.closed, store.rollbacks
			store.mu.Unlock()
			if !errors.Is(err, want) || opened != closed || rollbacks != 1 {
				t.Fatalf("chain/cleanup: %v opened%d closed%d rollback%d", err, opened, closed, rollbacks)
			}
		})
	}
}

func TestFeatureAtomicIdentifierQuoting(t *testing.T) {
	for input, expected := range map[string]string{"secret.name": "`secret.name`", "`quoted`": "```quoted```", "a`b": "`a``b`"} {
		if got := featureQuoteIdentifier(input); got != expected {
			t.Fatalf("atomic quote %q: %s", input, got)
		}
	}
}
