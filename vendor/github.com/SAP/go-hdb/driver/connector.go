package driver

import (
	"context"
	"database/sql/driver"
	"os"

	"github.com/SAP/go-hdb/driver/internal/protocol/x509"
)

/*
A Connector represents a hdb driver in a fixed configuration.
A Connector can be passed to sql.OpenDB (starting from go 1.10) allowing users to bypass a string based data source name.
*/
type Connector struct {
	*connAttrs
	*authAttrs

	connHook func(driver.Conn) driver.Conn
	newConn  func(ctx context.Context, connAttrs *connAttrs, authAttrs *authAttrs) (driver.Conn, error)
}

// NewConnector returns a new Connector instance with default values.
func NewConnector() *Connector {
	return &Connector{
		connAttrs: newConnAttrs(),
		authAttrs: &authAttrs{},
		newConn: func(ctx context.Context, connAttrs *connAttrs, authAttrs *authAttrs) (driver.Conn, error) {
			return newConn(ctx, stdHdbDriver.metrics, connAttrs, authAttrs) // use default stdHdbDriver metrics
		},
	}
}

// NewBasicAuthConnector creates a connector for basic authentication.
func NewBasicAuthConnector(host, username, password string) *Connector {
	c := NewConnector()
	c._host = host
	c._username = username
	c._password = password
	return c
}

// NewX509AuthConnector creates a connector for X509 (client certificate) authentication.
func NewX509AuthConnector(host string, clientCert, clientKey []byte) (*Connector, error) {
	c := NewConnector()
	c._host = host
	var err error
	if c._certKey, err = x509.NewCertKey(clientCert, clientKey); err != nil {
		return nil, err
	}
	return c, nil
}

// NewX509AuthConnectorByFiles creates a connector for X509 (client certificate) authentication
// based on client certificate and client key files.
func NewX509AuthConnectorByFiles(host, clientCertFile, clientKeyFile string) (*Connector, error) {
	clientCert, err := os.ReadFile(clientCertFile)
	if err != nil {
		return nil, err
	}
	clientKey, err := os.ReadFile(clientKeyFile)
	if err != nil {
		return nil, err
	}
	return NewX509AuthConnector(host, clientCert, clientKey)
}

// NewJWTAuthConnector creates a connector for token (JWT) based authentication.
func NewJWTAuthConnector(host, token string) *Connector {
	c := NewConnector()
	c._host = host
	c._token = token
	return c
}

func newDSNConnector(dsn *DSN) (*Connector, error) {
	c := NewConnector()
	c._host = dsn.host
	c._pingInterval = dsn.pingInterval
	c._defaultSchema = dsn.defaultSchema
	c.setTimeout(dsn.timeout)
	if dsn.tls != nil {
		if err := c.connAttrs.setTLS(dsn.tls.ServerName, dsn.tls.InsecureSkipVerify, dsn.tls.RootCAFiles); err != nil {
			return nil, err
		}
	}
	c._username = dsn.username
	c._password = dsn.password
	return c, nil
}

// NewDSNConnector creates a connector from a data source name.
func NewDSNConnector(dsnStr string) (*Connector, error) {
	dsn, err := parseDSN(dsnStr)
	if err != nil {
		return nil, err
	}
	return newDSNConnector(dsn)
}

// NativeDriver returns the concrete underlying Driver of the Connector.
func (c *Connector) NativeDriver() Driver { return stdHdbDriver }

// Connect implements the database/sql/driver/Connector interface.
func (c *Connector) Connect(ctx context.Context) (driver.Conn, error) {
	conn, err := c.newConn(ctx, c.connAttrs.clone(), c.authAttrs)
	if err != nil {
		return nil, err
	}
	if c.connHook != nil {
		conn = c.connHook(conn)
	}
	return conn, err
}

// Driver implements the database/sql/driver/Connector interface.
func (c *Connector) Driver() driver.Driver { return stdHdbDriver }

// SetConnHook sets a function for intercepting connection creation.
// This is for internal use only and might be changed or disabled in future.
func (c *Connector) SetConnHook(fn func(driver.Conn) driver.Conn) { c.connHook = fn }
