// Package dial provides types to implement go-hdb custom dialers.
package dial

import (
	"context"
	"net"
	"time"
)

// DialerOptions contains optional parameters that might be used by a Dialer.
type DialerOptions struct {
	Timeout, TCPKeepAlive time.Duration
	TCPKeepAliveConfig    net.KeepAliveConfig
}

// The Dialer interface needs to be implemented by custom Dialers. A Dialer for providing a custom driver connection
// to the database can be set in the driver.Connector object.
type Dialer interface {
	DialContext(ctx context.Context, address string, options DialerOptions) (net.Conn, error)
}

// DefaultDialer is the default driver Dialer implementation.
// TCP4 connections are preferred over TCP6 connections until HANA cloud would fully support IPv6.
// see https://github.com/SAP/go-hdb/issues/157.
var DefaultDialer Dialer = &tcp4PrefDialer{}
var _ = &dialer{}

// default dialer implementation.
type dialer struct{}

func (d *dialer) DialContext(ctx context.Context, address string, options DialerOptions) (net.Conn, error) {
	dialer := net.Dialer{Timeout: options.Timeout, KeepAlive: options.TCPKeepAlive, KeepAliveConfig: options.TCPKeepAliveConfig}
	return dialer.DialContext(ctx, "tcp", address)
}

// dialer which prefers tcp4 connections over tcp6 implementation.
type tcp4PrefDialer struct{}

func (d *tcp4PrefDialer) DialContext(ctx context.Context, address string, options DialerOptions) (net.Conn, error) {
	dialer := net.Dialer{Timeout: options.Timeout, KeepAlive: options.TCPKeepAlive, KeepAliveConfig: options.TCPKeepAliveConfig}
	if conn, err := dialer.DialContext(ctx, "tcp4", address); err == nil {
		return conn, nil
	}
	return dialer.DialContext(ctx, "tcp", address)
}
