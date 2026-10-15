package driver

import (
	"context"
	"crypto/tls"
	"database/sql/driver"
	"fmt"
	"io"
	"log/slog"
	"net"
	"runtime/pprof"
	"time"

	"github.com/SAP/go-hdb/driver/internal/profile"
)

type dbConn interface {
	io.ReadWriteCloser
	lastRead() time.Time
	lastWrite() time.Time
}

type profileDBConn struct {
	dbConn
}

func (c *profileDBConn) Read(b []byte) (n int, err error) {
	pprof.Do(context.Background(), pprof.Labels("db", "read"), func(ctx context.Context) {
		n, err = c.dbConn.Read(b)
	})
	return
}

func (c *profileDBConn) Write(b []byte) (n int, err error) {
	pprof.Do(context.Background(), pprof.Labels("db", "write"), func(ctx context.Context) {
		n, err = c.dbConn.Write(b)
	})
	return
}

// stdDBConn wraps the database tcp connection. It sets timeouts and handles driver ErrBadConn behavior.
type stdDBConn struct {
	metrics    *metrics
	conn       net.Conn
	timeout    time.Duration
	logger     *slog.Logger
	_lastRead  time.Time
	_lastWrite time.Time
}

func newDBConn(ctx context.Context, logger *slog.Logger, host string, metrics *metrics, attrs *connAttrs) (dbConn, error) {
	conn, err := attrs.dialContext(ctx, host)
	if err != nil {
		return nil, err
	}
	// is TLS connection requested?
	if attrs.tlsConfig != nil {
		conn = tls.Client(conn, attrs.tlsConfig)
	}

	dbConn := &stdDBConn{metrics: metrics, conn: conn, timeout: attrs.timeout, logger: logger}
	if profile.Active {
		return &profileDBConn{dbConn: dbConn}, nil
	}
	return dbConn, nil
}

func (c *stdDBConn) lastRead() time.Time  { return c._lastRead }
func (c *stdDBConn) lastWrite() time.Time { return c._lastWrite }

func (c *stdDBConn) deadline() (deadline time.Time) {
	if c.timeout == 0 {
		return
	}
	return time.Now().Add(c.timeout)
}

func (c *stdDBConn) Close() error { return c.conn.Close() }

func (c *stdDBConn) errLogAttrs(err error, now time.Time) []slog.Attr {
	attrs := []slog.Attr{
		slog.String("error", err.Error()),
		slog.String("local address", c.conn.LocalAddr().String()),
		slog.String("remote address", c.conn.RemoteAddr().String()),
		slog.String("timeout", c.timeout.String()),
	}
	lastAccess := c._lastRead
	if c._lastWrite.Compare(c._lastRead) == 1 {
		lastAccess = c._lastWrite
	}
	if !lastAccess.IsZero() {
		attrs = append(attrs, slog.String("since last access", now.Sub(lastAccess).String()))
	}
	return attrs
}

// Read implements the io.Reader interface.
func (c *stdDBConn) Read(b []byte) (int, error) {
	// set timeout
	if err := c.conn.SetReadDeadline(c.deadline()); err != nil {
		return 0, fmt.Errorf("%w: %w", driver.ErrBadConn, err)
	}
	now := time.Now()
	n, err := c.conn.Read(b)
	c.metrics.addTimeCounter(timeRead, time.Since(now), counterBytesRead, uint64(n)) //nolint:gosec
	if err != nil {
		c.logger.LogAttrs(context.Background(), slog.LevelError, "DB conn read error", c.errLogAttrs(err, now)...)
		err = fmt.Errorf("%w: %w", driver.ErrBadConn, err) // wrap error in driver.ErrBadConn
	}
	c._lastRead = now
	return n, err
}

// Write implements the io.Writer interface.
func (c *stdDBConn) Write(b []byte) (int, error) {
	// set timeout
	if err := c.conn.SetWriteDeadline(c.deadline()); err != nil {
		return 0, fmt.Errorf("%w: %w", driver.ErrBadConn, err)
	}
	now := time.Now()
	n, err := c.conn.Write(b)
	c.metrics.addTimeCounter(timeWrite, time.Since(now), counterBytesWritten, uint64(n)) //nolint:gosec
	if err != nil {
		c.logger.LogAttrs(context.Background(), slog.LevelError, "DB conn write error", c.errLogAttrs(err, now)...)
		err = fmt.Errorf("%w: %w", driver.ErrBadConn, err) // wrap error in driver.ErrBadConn
	}
	c._lastWrite = now
	return n, err
}
