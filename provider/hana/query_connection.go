package hana

import (
	"context"
	"database/sql/driver"
	"errors"
	"net"
	"sync"
	"sync/atomic"

	hdb "github.com/SAP/go-hdb/driver"
	"github.com/SAP/go-hdb/driver/dial"
)

type featureDialCaptureKey struct{}
type featureSocket struct {
	net.Conn
	closed atomic.Bool
}

func (c *featureSocket) Close() error {
	if !c.closed.CompareAndSwap(false, true) {
		return nil
	}
	return c.Conn.Close()
}

type featureDialCapture struct {
	mutex   sync.Mutex
	sockets []*featureSocket
}

func (c *featureDialCapture) socket() (*featureSocket, error) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	var selected *featureSocket
	for _, socket := range c.sockets {
		if socket.closed.Load() {
			continue
		}
		if selected != nil {
			return nil, featureUnsupported("ambiguous physical socket association")
		}
		selected = socket
	}
	if selected == nil {
		return nil, featureUnsupported("physical socket association unavailable")
	}
	return selected, nil
}

type featureCapturingDialer struct{ delegate dial.Dialer }

func (d featureCapturingDialer) DialContext(ctx context.Context, address string, options dial.DialerOptions) (net.Conn, error) {
	socket, err := d.delegate.DialContext(ctx, address, options)
	if err != nil {
		return nil, err
	}
	capture, ok := ctx.Value(featureDialCaptureKey{}).(*featureDialCapture)
	if !ok {
		return socket, nil
	}
	tracked := &featureSocket{Conn: socket}
	capture.mutex.Lock()
	capture.sockets = append(capture.sockets, tracked)
	capture.mutex.Unlock()
	return tracked, nil
}

// The dialer is installed once, before the connector is given to database/sql.
// Each Connect gets a private capture; concurrent tile connections cannot be
// associated with the reserved feature connection's cancellation watcher.
type featureConnector struct{ delegate driver.Connector }

func featureCancellableConnector(connector *hdb.Connector) driver.Connector {
	connector.SetDialer(featureCapturingDialer{delegate: connector.Dialer()})
	return featureConnector{delegate: connector}
}
func (c featureConnector) Driver() driver.Driver { return c.delegate.Driver() }
func (c featureConnector) Connect(ctx context.Context) (driver.Conn, error) {
	capture := &featureDialCapture{}
	inner, err := c.delegate.Connect(context.WithValue(ctx, featureDialCaptureKey{}, capture))
	if err != nil {
		capture.mutex.Lock()
		for _, socket := range capture.sockets {
			err = errors.Join(err, socket.Close())
		}
		capture.mutex.Unlock()
		return nil, err
	}
	socket, associationErr := capture.socket()
	return &featureConnection{Conn: inner, socket: socket, associationErr: associationErr}, nil
}

type featureConnection struct {
	driver.Conn
	socket         *featureSocket
	associationErr error
}

func (c *featureConnection) PrepareContext(ctx context.Context, q string) (driver.Stmt, error) {
	if conn, ok := c.Conn.(driver.ConnPrepareContext); ok {
		return conn.PrepareContext(ctx, q)
	}
	return c.Conn.Prepare(q)
}

func (c *featureConnection) BeginTx(ctx context.Context, options driver.TxOptions) (driver.Tx, error) {
	if conn, ok := c.Conn.(driver.ConnBeginTx); ok {
		return conn.BeginTx(ctx, options)
	}
	return nil, driver.ErrSkip
}
func (c *featureConnection) QueryContext(ctx context.Context, q string, args []driver.NamedValue) (driver.Rows, error) {
	if conn, ok := c.Conn.(driver.QueryerContext); ok {
		return conn.QueryContext(ctx, q, args)
	}
	return nil, driver.ErrSkip
}
func (c *featureConnection) ExecContext(ctx context.Context, q string, args []driver.NamedValue) (driver.Result, error) {
	if conn, ok := c.Conn.(driver.ExecerContext); ok {
		return conn.ExecContext(ctx, q, args)
	}
	return nil, driver.ErrSkip
}
func (c *featureConnection) CheckNamedValue(value *driver.NamedValue) error {
	if conn, ok := c.Conn.(driver.NamedValueChecker); ok {
		return conn.CheckNamedValue(value)
	}
	return driver.ErrSkip
}
func (c *featureConnection) Ping(ctx context.Context) error {
	if conn, ok := c.Conn.(driver.Pinger); ok {
		return conn.Ping(ctx)
	}
	return driver.ErrSkip
}
func (c *featureConnection) ResetSession(ctx context.Context) error {
	if conn, ok := c.Conn.(driver.SessionResetter); ok {
		return conn.ResetSession(ctx)
	}
	return driver.ErrBadConn
}
func (c *featureConnection) IsValid() bool {
	if conn, ok := c.Conn.(driver.Validator); ok {
		return conn.IsValid()
	}
	return false
}

func (c *featureConnection) featureWatchContext(ctx context.Context) (func() error, error) {
	if c.associationErr != nil {
		return nil, c.associationErr
	}
	if c.socket == nil || c.socket.closed.Load() {
		return nil, featureUnsupported("physical socket unavailable")
	}
	stop := make(chan struct{})
	joined := make(chan struct{})
	var closeErr error
	go func() {
		defer close(joined)
		select {
		case <-ctx.Done():
			closeErr = c.socket.Close()
		case <-stop:
		}
	}()
	var once sync.Once
	return func() error { once.Do(func() { close(stop) }); <-joined; return closeErr }, nil
}

func featureContextError(ctx context.Context, err error) error {
	if ctx.Err() == nil {
		return err
	}
	return errors.Join(ctx.Err(), err)
}

func featureContextSourceError(ctx context.Context, err error) error {
	if ctx.Err() != nil {
		return errors.Join(ctx.Err(), err)
	}
	return featureSourceError(err)
}
