package hana

import (
	"context"
	"database/sql/driver"
	"errors"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/SAP/go-hdb/driver/dial"
	"github.com/alexeydott/tegola/provider"
)

type featureTestSocket struct {
	net.Conn
	closes   atomic.Int32
	signal   chan struct{}
	once     sync.Once
	closeErr error
}

func (c *featureTestSocket) Close() error {
	c.closes.Add(1)
	if c.signal != nil {
		c.once.Do(func() { close(c.signal) })
	}
	return c.closeErr
}

type featureTestDialer struct {
	fn func(context.Context) (net.Conn, error)
}

func (d featureTestDialer) DialContext(ctx context.Context, _ string, _ dial.DialerOptions) (net.Conn, error) {
	return d.fn(ctx)
}

type featureTestConnector struct {
	connect func(context.Context) (driver.Conn, error)
}

func (c featureTestConnector) Connect(ctx context.Context) (driver.Conn, error) {
	return c.connect(ctx)
}
func (c featureTestConnector) Driver() driver.Driver { return featureMockDriver{&featureMock{}} }

func TestFeatureSocketAssociationAndRouting(t *testing.T) {
	for _, mode := range []string{"unique", "replaced", "missing", "ambiguous", "failed"} {
		t.Run(mode, func(t *testing.T) {
			sockets := []*featureTestSocket{}
			d := featureCapturingDialer{delegate: featureTestDialer{fn: func(context.Context) (net.Conn, error) {
				s := &featureTestSocket{signal: make(chan struct{})}
				sockets = append(sockets, s)
				return s, nil
			}}}
			failure := errors.New("authentication failed")
			connector := featureConnector{delegate: featureTestConnector{connect: func(ctx context.Context) (driver.Conn, error) {
				if mode != "missing" {
					first, err := d.DialContext(ctx, "first", dial.DialerOptions{})
					if err != nil {
						return nil, err
					}
					if mode == "replaced" || mode == "ambiguous" {
						if mode == "replaced" {
							_ = first.Close()
						}
						if _, err := d.DialContext(ctx, "second", dial.DialerOptions{}); err != nil {
							return nil, err
						}
					}
				}
				if mode == "failed" {
					return nil, failure
				}
				return &featureMockConn{&featureMock{}}, nil
			}}}
			inner, err := connector.Connect(context.Background())
			if mode == "failed" {
				if !errors.Is(err, failure) || sockets[0].closes.Load() != 1 {
					t.Fatal("failed connect socket leaked")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			stop, err := inner.(*featureConnection).featureWatchContext(ctx)
			if mode == "missing" || mode == "ambiguous" {
				if !errors.Is(err, provider.ErrUnsupported) {
					t.Fatal("unproven association admitted")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			cancel()
			<-sockets[len(sockets)-1].signal
			_ = stop()
			for _, s := range sockets {
				if s.closes.Load() != 1 {
					t.Fatal("wrong routed socket ownership")
				}
			}
		})
	}
}

func TestFeatureSocketWatcherIsolationAndStopRace(t *testing.T) {
	for i := 0; i < 100; i++ {
		a, b := &featureTestSocket{}, &featureTestSocket{}
		feature := &featureConnection{socket: &featureSocket{Conn: a}}
		tile := &featureConnection{socket: &featureSocket{Conn: b}}
		ctx, cancel := context.WithCancel(context.Background())
		stop, err := feature.featureWatchContext(ctx)
		if err != nil {
			t.Fatal(err)
		}
		var group sync.WaitGroup
		group.Add(2)
		go func() { defer group.Done(); cancel() }()
		go func() { defer group.Done(); _ = stop() }()
		group.Wait()
		_ = stop()
		before := a.closes.Load()
		if before > 1 || tile.socket.closed.Load() || b.closes.Load() != 0 {
			t.Fatal("watcher closed wrong socket")
		}
		cancel()
		if a.closes.Load() != before {
			t.Fatal("socket closed after joined watcher")
		}
	}
	owned := &featureTestSocket{}
	c := &featureConnection{socket: &featureSocket{Conn: owned}}
	ctx, cancel := context.WithCancel(context.Background())
	stop, err := c.featureWatchContext(ctx)
	if err != nil {
		t.Fatal(err)
	}
	_ = stop()
	cancel()
	if owned.closes.Load() != 0 {
		t.Fatal("successful query watcher remained armed")
	}
}

func TestFeatureConcurrentConnectCaptures(t *testing.T) {
	d := featureCapturingDialer{delegate: featureTestDialer{fn: func(context.Context) (net.Conn, error) { return &featureTestSocket{}, nil }}}
	c := featureConnector{delegate: featureTestConnector{connect: func(ctx context.Context) (driver.Conn, error) {
		if _, err := d.DialContext(ctx, "owned", dial.DialerOptions{}); err != nil {
			return nil, err
		}
		return &featureMockConn{&featureMock{}}, nil
	}}}
	var group sync.WaitGroup
	var mutex sync.Mutex
	seen := map[*featureSocket]bool{}
	for i := 0; i < 16; i++ {
		group.Add(1)
		go func() {
			defer group.Done()
			conn, err := c.Connect(context.Background())
			if err != nil {
				t.Error(err)
				return
			}
			socket := conn.(*featureConnection).socket
			mutex.Lock()
			defer mutex.Unlock()
			if socket == nil || seen[socket] {
				t.Error("shared connect association")
			}
			seen[socket] = true
		}()
	}
	group.Wait()
	if len(seen) != 16 {
		t.Fatal("missing captured connection")
	}
}

func TestFeatureContextErrorPreservesCompleteChain(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	marker := errors.New("cleanup marker")
	source := errors.New("source marker")
	joined := fmt.Errorf("outer wrapper: %w", errors.Join(provider.FeatureDataError{Err: source}, marker))
	err := featureContextError(ctx, joined)
	var data provider.FeatureDataError
	if !errors.Is(err, context.Canceled) || !errors.Is(err, marker) || !errors.Is(err, source) || !errors.As(err, &data) {
		t.Fatal("context priority lost original chain")
	}
	err = featureContextSourceError(ctx, source)
	if !errors.Is(err, context.Canceled) || !errors.Is(err, source) || errors.As(err, &data) {
		t.Fatal("cancelled primitive incorrectly classified as source integrity")
	}
}

func TestFeatureSocketCloseErrorAfterWatcherJoin(t *testing.T) {
	marker := errors.New("socket close marker")
	socket := &featureTestSocket{signal: make(chan struct{}), closeErr: marker}
	c := &featureConnection{socket: &featureSocket{Conn: socket}}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	stop, err := c.featureWatchContext(ctx)
	if err != nil {
		t.Fatal(err)
	}
	cancel()
	<-socket.signal
	err = featureContextError(ctx, stop())
	if !errors.Is(err, marker) || !errors.Is(err, context.Canceled) {
		t.Fatal("cancellation cleanup chain lost")
	}
	if err := c.socket.Close(); err != nil || socket.closes.Load() != 1 {
		t.Fatal("socket close was not idempotent")
	}
}

type featureDelegateProof struct {
	driver.Conn
	ctx     context.Context
	marker  error
	calls   map[string]bool
	options driver.TxOptions
	args    []driver.NamedValue
	value   *driver.NamedValue
}

func (c *featureDelegateProof) check(name string, ctx context.Context) {
	if ctx != c.ctx {
		panic("delegated context changed")
	}
	c.calls[name] = true
}
func (c *featureDelegateProof) PrepareContext(ctx context.Context, q string) (driver.Stmt, error) {
	c.check("prepare:"+q, ctx)
	return nil, c.marker
}
func (c *featureDelegateProof) BeginTx(ctx context.Context, opts driver.TxOptions) (driver.Tx, error) {
	c.check("begin", ctx)
	c.options = opts
	return nil, c.marker
}
func (c *featureDelegateProof) QueryContext(ctx context.Context, q string, args []driver.NamedValue) (driver.Rows, error) {
	c.check("query:"+q, ctx)
	c.args = args
	return nil, c.marker
}
func (c *featureDelegateProof) ExecContext(ctx context.Context, q string, args []driver.NamedValue) (driver.Result, error) {
	c.check("exec:"+q, ctx)
	c.args = args
	return nil, c.marker
}
func (c *featureDelegateProof) Ping(ctx context.Context) error { c.check("ping", ctx); return c.marker }
func (c *featureDelegateProof) ResetSession(ctx context.Context) error {
	c.check("reset", ctx)
	return c.marker
}
func (c *featureDelegateProof) IsValid() bool { c.calls["valid"] = true; return true }
func (c *featureDelegateProof) CheckNamedValue(value *driver.NamedValue) error {
	c.calls["value"] = true
	c.value = value
	return c.marker
}

func TestFeatureConnectionOptionalInterfaceDelegation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	marker := errors.New("delegate marker")
	inner := &featureDelegateProof{ctx: ctx, marker: marker, calls: map[string]bool{}}
	c := &featureConnection{Conn: inner}
	args := []driver.NamedValue{{Ordinal: 1, Value: "exact"}}
	options := driver.TxOptions{Isolation: 4, ReadOnly: true}
	_, err := c.PrepareContext(ctx, "prepared")
	if !errors.Is(err, marker) {
		t.Fatal(err)
	}
	_, err = c.BeginTx(ctx, options)
	if !errors.Is(err, marker) || inner.options != options {
		t.Fatal("begin options changed")
	}
	_, err = c.QueryContext(ctx, "query", args)
	if !errors.Is(err, marker) || &inner.args[0] != &args[0] {
		t.Fatal("query args changed")
	}
	_, err = c.ExecContext(ctx, "exec", args)
	if !errors.Is(err, marker) || &inner.args[0] != &args[0] {
		t.Fatal("exec args changed")
	}
	if !errors.Is(c.Ping(ctx), marker) || !errors.Is(c.ResetSession(ctx), marker) || !c.IsValid() {
		t.Fatal("connection optional results changed")
	}
	if !errors.Is(c.CheckNamedValue(&args[0]), marker) || inner.value != &args[0] {
		t.Fatal("named value pointer changed")
	}
	if len(inner.calls) != 8 {
		t.Fatalf("missing interface forwarding %+v", inner.calls)
	}
}
