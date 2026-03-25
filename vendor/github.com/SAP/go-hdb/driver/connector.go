package driver

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"database/sql/driver"
	"fmt"
	"log/slog"
	"maps"
	"math"
	"net"
	"os"
	"path"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unique"

	"github.com/SAP/go-hdb/driver/dial"
	p "github.com/SAP/go-hdb/driver/internal/protocol"
	"github.com/SAP/go-hdb/driver/internal/protocol/auth"
	"github.com/SAP/go-hdb/driver/unicode/cesu8"
	"golang.org/x/text/transform"
)

type redirectCacheKey struct {
	host, databaseName string
}

var redirectCache sync.Map

/*
SessionVariables maps session variables to their values.
All defined session variables will be set once after a database connection is opened.
*/
type SessionVariables map[string]string

// conn attributes default values.
const (
	defaultBufferSize   = 16276             // default value bufferSize.
	defaultBulkSize     = 10000             // default value bulkSize.
	defaultTimeout      = 300 * time.Second // default value connection timeout (300 seconds = 5 minutes).
	defaultTCPKeepAlive = 15 * time.Second  // default TCP keep-alive value (copied from net.dial.go)
)

// minimal / maximal values.
const (
	minTimeout  = 0 * time.Second // minimal timeout value.
	minBulkSize = 1               // minimal bulkSize value.
	maxBulkSize = p.MaxNumArg     // maximum bulk size.
)

const (
	defaultFetchSize    = 128         // Default value fetchSize.
	defaultLobChunkSize = 1 << 16     // Default value lobChunkSize.
	defaultDfv          = p.DfvLevel8 // Default data version format level.
)

const (
	minFetchSize    = 1             // Minimal fetchSize value.
	minLobChunkSize = 128           // Minimal lobChunkSize
	maxLobChunkSize = math.MaxInt32 // Maximal lobChunkSize
)

var defaultTCPKeepAliveConfig = net.KeepAliveConfig{Enable: true}

// connAttrs is holding connection relevant attributes.
type connAttrs struct {
	timeout            time.Duration
	pingInterval       time.Duration
	bufferSize         int
	bulkSize           int
	tcpKeepAlive       time.Duration       // see net.Dialer
	tcpKeepAliveConfig net.KeepAliveConfig // see net.Dialer
	tlsConfig          *tls.Config
	defaultSchema      string
	dialer             dial.Dialer
	applicationName    string
	sessionVariables   map[string]string
	locale             string
	fetchSize          int
	lobChunkSize       int
	dfv                int
	cesu8Decoder       transform.Transformer
	cesu8Encoder       transform.Transformer
	emptyDateAsNull    bool
	logger             *slog.Logger
}

func (c *connAttrs) dialContext(ctx context.Context, host string) (net.Conn, error) {
	return c.dialer.DialContext(ctx, host, dial.DialerOptions{Timeout: c.timeout, TCPKeepAlive: c.tcpKeepAlive, TCPKeepAliveConfig: c.tcpKeepAliveConfig})
}

func readCertKeyFiles(certFile, keyFile string) (unique.Handle[string], unique.Handle[string], error) {
	var handle unique.Handle[string]
	cert, err := os.ReadFile(certFile)
	if err != nil {
		return handle, handle, err
	}
	key, err := os.ReadFile(keyFile)
	if err != nil {
		return handle, handle, err
	}
	return unique.Make(string(cert)), unique.Make(string(key)), nil
}

func isJWTToken(token string) bool { return strings.HasPrefix(token, "ey") }

/*
A Connector represents a hdb driver in a fixed configuration.
A Connector can be passed to sql.OpenDB allowing users to bypass a string based data source name.
*/
type Connector struct {
	_host         string
	_databaseName string

	mu sync.RWMutex

	_timeout            time.Duration
	_pingInterval       time.Duration
	_bufferSize         int
	_bulkSize           int
	_tcpKeepAlive       time.Duration       // see net.Dialer
	_tcpKeepAliveConfig net.KeepAliveConfig // see net.Dialer
	_tlsConfig          *tls.Config
	_defaultSchema      string
	_dialer             dial.Dialer
	_applicationName    string
	_sessionVariables   map[string]string
	_locale             string
	_fetchSize          int
	_lobChunkSize       int
	_dfv                int
	_cesu8DecoderFn     func() transform.Transformer
	_cesu8EncoderFn     func() transform.Transformer
	_emptyDateAsNull    bool
	_logger             *slog.Logger

	hasCookie            atomic.Bool
	_username, _password string // basic authentication
	_certFile, _keyFile  string
	_certKey             *auth.CertKey // X509
	_token               string        // JWT
	_logonname           string        // session cookie login does need logon name provided by JWT authentication.
	_sessionCookie       []byte        // authentication via session cookie (HDB currently does support only SAML and JWT - go-hdb JWT)
	_refreshPasswordFn   func() (password string, ok bool)
	_refreshClientCertFn func() (clientCert, clientKey []byte, ok bool)
	_refreshTokenFn      func() (token string, ok bool)
	cbmu                 sync.Mutex // prevents refresh callbacks from being called in parallel

	metrics *metrics
}

// NewConnector returns a new Connector instance with default values.
func NewConnector() *Connector {
	return &Connector{
		_timeout:            defaultTimeout,
		_bufferSize:         defaultBufferSize,
		_bulkSize:           defaultBulkSize,
		_tcpKeepAlive:       defaultTCPKeepAlive,
		_tcpKeepAliveConfig: defaultTCPKeepAliveConfig,
		_dialer:             dial.DefaultDialer,
		_applicationName:    defaultApplicationName,
		_fetchSize:          defaultFetchSize,
		_lobChunkSize:       defaultLobChunkSize,
		_dfv:                defaultDfv,
		_cesu8DecoderFn:     cesu8.DefaultDecoder,
		_cesu8EncoderFn:     cesu8.DefaultEncoder,
		_logger:             slog.Default(),
		metrics:             stdHdbDriver.metrics, // use default stdHdbDriver metrics
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
// Parameters clientCert and clientKey in PEM format, clientKey not password encryped.
func NewX509AuthConnector(host string, clientCert, clientKey []byte) (*Connector, error) {
	c := NewConnector()
	c._host = host
	var err error
	if c._certKey, err = auth.NewCertKey(unique.Make(string(clientCert)), unique.Make(string(clientKey))); err != nil {
		return nil, err
	}
	return c, nil
}

// NewX509AuthConnectorByFiles creates a connector for X509 (client certificate) authentication
// based on client certificate and client key files.
// Parameters clientCertFile and clientKeyFile in PEM format, clientKeyFile not password encryped.
func NewX509AuthConnectorByFiles(host, clientCertFile, clientKeyFile string) (*Connector, error) {
	c := NewConnector()
	c._host = host

	clientCertFile = path.Clean(clientCertFile)
	clientKeyFile = path.Clean(clientKeyFile)

	certHandle, keyHandle, err := readCertKeyFiles(clientCertFile, clientKeyFile)
	if err != nil {
		return nil, err
	}
	if c._certKey, err = auth.NewCertKey(certHandle, keyHandle); err != nil {
		return nil, err
	}

	c._certFile = clientCertFile
	c._keyFile = clientKeyFile

	return c, nil
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
	c._databaseName = dsn.databaseName
	c._pingInterval = dsn.pingInterval
	c._defaultSchema = dsn.defaultSchema
	c.setTimeout(dsn.timeout)
	if dsn.tls != nil {
		if err := c.setTLS(dsn.tls.ServerName, dsn.tls.InsecureSkipVerify, dsn.tls.RootCAFiles); err != nil {
			return nil, err
		}
	}
	c._username = dsn.username
	c._password = dsn.password
	return c, nil
}

// NewDSNConnector creates a connector from a data source name.
func NewDSNConnector(dsnStr string) (*Connector, error) {
	dsn, err := ParseDSN(dsnStr)
	if err != nil {
		return nil, err
	}
	return newDSNConnector(dsn)
}

// NativeDriver returns the concrete underlying Driver of the Connector.
func (c *Connector) NativeDriver() Driver { return stdHdbDriver }

// Host returns the host of the connector.
func (c *Connector) Host() string { return c._host }

// DatabaseName returns the tenant database name of the connector.
func (c *Connector) DatabaseName() string { return c._databaseName }

func (c *Connector) fetchRedirectHost(ctx context.Context) (string, error) {
	conn, err := newConn(ctx, c._host, c.metrics, c.connAttrs(), nil)
	if err != nil {
		return "", err
	}
	defer conn.Close()
	dbi, err := conn.session.dbConnectInfo(ctx, c._databaseName)
	if err != nil {
		return "", err
	}
	if dbi.IsConnected { // if databaseName == "SYSTEMDB" and isConnected == true host and port are initial
		return c._host, nil
	}
	return net.JoinHostPort(dbi.Host, strconv.Itoa(dbi.Port)), nil
}

func (c *Connector) connect(ctx context.Context, host string) (driver.Conn, error) {
	var connAttrs = c.connAttrs()

	// can we connect via cookie?
	if auth := c.cookieAuth(); auth != nil {
		conn, err := newConn(ctx, host, c.metrics, connAttrs, auth)
		if err == nil {
			return conn, nil
		}
		if !isAuthError(err) {
			return nil, err
		}
		c.invalidateCookie() // cookie auth was not successful - do not try again with the same data
	}

	c.cbmu.Lock() // synchronize refresh calls
	defer c.cbmu.Unlock()
	for {
		authHnd := c.authHnd()

		conn, connErr := newConn(ctx, host, c.metrics, connAttrs, authHnd)
		if connErr == nil {
			if method, ok := authHnd.Selected().(auth.CookieGetter); ok {
				c.setCookie(method.Cookie())
			}
			return conn, nil
		}
		if !isAuthError(connErr) {
			return nil, connErr
		}

		ok, err := c.refresh()
		if err != nil {
			return nil, err
		}
		if !ok { // no connection retry in case no refresh took place
			return nil, connErr
		}
	}
}

func (c *Connector) redirect(ctx context.Context) (driver.Conn, error) {
	if redirectHost, found := redirectCache.Load(redirectCacheKey{host: c._host, databaseName: c._databaseName}); found {
		if conn, err := c.connect(ctx, redirectHost.(string)); err == nil {
			return conn, nil
		}
	}

	redirectHost, err := c.fetchRedirectHost(ctx)
	if err != nil {
		return nil, err
	}
	conn, err := c.connect(ctx, redirectHost)
	if err != nil {
		return nil, err
	}

	redirectCache.Store(redirectCacheKey{host: c._host, databaseName: c._databaseName}, redirectHost)

	return conn, err
}

// Connect implements the database/sql/driver/Connector interface.
func (c *Connector) Connect(ctx context.Context) (driver.Conn, error) {
	if c._databaseName != "" {
		return c.redirect(ctx)
	}
	return c.connect(ctx, c._host)
}

// Driver implements the database/sql/driver/Connector interface.
func (c *Connector) Driver() driver.Driver { return stdHdbDriver }

func (c *Connector) clone() *Connector {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return &Connector{
		_host:         c._host,
		_databaseName: c._databaseName,

		_timeout:            c._timeout,
		_pingInterval:       c._pingInterval,
		_bufferSize:         c._bufferSize,
		_bulkSize:           c._bulkSize,
		_tcpKeepAlive:       c._tcpKeepAlive,
		_tcpKeepAliveConfig: c._tcpKeepAliveConfig,
		_tlsConfig:          c._tlsConfig.Clone(),
		_defaultSchema:      c._defaultSchema,
		_dialer:             c._dialer,
		_applicationName:    c._applicationName,
		_sessionVariables:   maps.Clone(c._sessionVariables),
		_locale:             c._locale,
		_fetchSize:          c._fetchSize,
		_lobChunkSize:       c._lobChunkSize,
		_dfv:                c._dfv,
		_cesu8DecoderFn:     c._cesu8DecoderFn,
		_cesu8EncoderFn:     c._cesu8EncoderFn,
		_emptyDateAsNull:    c._emptyDateAsNull,
		_logger:             c._logger,

		_username:            c._username,
		_password:            c._password,
		_certFile:            c._certFile,
		_keyFile:             c._keyFile,
		_certKey:             c._certKey,
		_token:               c._token,
		_refreshPasswordFn:   c._refreshPasswordFn,
		_refreshClientCertFn: c._refreshClientCertFn,
		_refreshTokenFn:      c._refreshTokenFn,

		metrics: c.metrics,
	}
}

// WithDatabase returns a new Connector supporting tenant database connections via database name.
func (c *Connector) WithDatabase(databaseName string) *Connector {
	nc := c.clone()
	nc._databaseName = databaseName
	return nc
}

// conn attributes.
func (c *Connector) connAttrs() *connAttrs {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return &connAttrs{
		timeout:            c._timeout,
		pingInterval:       c._pingInterval,
		bufferSize:         c._bufferSize,
		bulkSize:           c._bulkSize,
		tcpKeepAlive:       c._tcpKeepAlive,
		tcpKeepAliveConfig: c._tcpKeepAliveConfig,
		tlsConfig:          c._tlsConfig.Clone(),
		defaultSchema:      c._defaultSchema,
		dialer:             c._dialer,
		applicationName:    c._applicationName,
		sessionVariables:   maps.Clone(c._sessionVariables),
		locale:             c._locale,
		fetchSize:          c._fetchSize,
		lobChunkSize:       c._lobChunkSize,
		dfv:                c._dfv,
		cesu8Decoder:       c._cesu8DecoderFn(),
		cesu8Encoder:       c._cesu8EncoderFn(),
		emptyDateAsNull:    c._emptyDateAsNull,
		logger:             c._logger,
	}
}

// TCPKeepAliveConfig returns the tcp keep-alive config value of the connector.
func (c *Connector) TCPKeepAliveConfig() net.KeepAliveConfig {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c._tcpKeepAliveConfig
}

/*
SetTCPKeepAliveConfig sets the tcp keep-alive config value of the connector.

For more information please see net.Dialer structure.
*/
func (c *Connector) SetTCPKeepAliveConfig(tcpKeepAliveConfig net.KeepAliveConfig) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c._tcpKeepAliveConfig = tcpKeepAliveConfig
}

func (c *Connector) setTimeout(timeout time.Duration) {
	if timeout < minTimeout {
		timeout = minTimeout
	}
	c._timeout = timeout
}
func (c *Connector) setBulkSize(bulkSize int) {
	switch {
	case bulkSize < minBulkSize:
		bulkSize = minBulkSize
	case bulkSize > maxBulkSize:
		bulkSize = maxBulkSize
	}
	c._bulkSize = bulkSize
}
func (c *Connector) setTLS(serverName string, insecureSkipVerify bool, rootCAFiles []string) error {
	c._tlsConfig = &tls.Config{
		ServerName:         serverName,
		InsecureSkipVerify: insecureSkipVerify, //nolint:gosec
	}
	var certPool *x509.CertPool
	for _, fn := range rootCAFiles {
		rootPEM, err := os.ReadFile(path.Clean(fn))
		if err != nil {
			return err
		}
		if certPool == nil {
			certPool = x509.NewCertPool()
		}
		if ok := certPool.AppendCertsFromPEM(rootPEM); !ok {
			return fmt.Errorf("failed to parse root certificate - filename: %s", fn)
		}
	}
	if certPool != nil {
		c._tlsConfig.RootCAs = certPool
	}
	return nil
}
func (c *Connector) setDialer(dialer dial.Dialer) {
	if dialer == nil {
		dialer = dial.DefaultDialer
	}
	c._dialer = dialer
}
func (c *Connector) setFetchSize(fetchSize int) {
	if fetchSize < minFetchSize {
		fetchSize = minFetchSize
	}
	c._fetchSize = fetchSize
}
func (c *Connector) setLobChunkSize(lobChunkSize int) {
	switch {
	case lobChunkSize < minLobChunkSize:
		lobChunkSize = minLobChunkSize
	case lobChunkSize > maxLobChunkSize:
		lobChunkSize = maxLobChunkSize
	}
	c._lobChunkSize = lobChunkSize
}
func (c *Connector) setDfv(dfv int) {
	if !p.IsSupportedDfv(dfv) {
		dfv = defaultDfv
	}
	c._dfv = dfv
}

// Timeout returns the timeout of the connector.
func (c *Connector) Timeout() time.Duration { c.mu.RLock(); defer c.mu.RUnlock(); return c._timeout }

/*
SetTimeout sets the timeout of the connector.

For more information please see DSNTimeout.
*/
func (c *Connector) SetTimeout(timeout time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.setTimeout(timeout)
}

// PingInterval returns the connection ping interval of the connector.
func (c *Connector) PingInterval() time.Duration {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c._pingInterval
}

/*
SetPingInterval sets the connection ping interval value of the connector.

Using a ping interval supports detecting broken connections. In case the ping
is not successful a new or another connection out of the connection pool would
be used automatically instead of retuning an error.

Parameter d defines the time between the pings as duration.
If d is zero no ping is executed. If d is not zero a database ping is executed if
an idle connection out of the connection pool is reused and the time since the
last connection access is greater or equal than d.
*/
func (c *Connector) SetPingInterval(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c._pingInterval = d
}

// BufferSize returns the bufferSize of the connector.
func (c *Connector) BufferSize() int { c.mu.RLock(); defer c.mu.RUnlock(); return c._bufferSize }

/*
SetBufferSize sets the bufferSize of the connector.
*/
func (c *Connector) SetBufferSize(bufferSize int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c._bufferSize = bufferSize
}

// BulkSize returns the bulkSize of the connector.
func (c *Connector) BulkSize() int { c.mu.RLock(); defer c.mu.RUnlock(); return c._bulkSize }

// SetBulkSize sets the bulkSize of the connector.
func (c *Connector) SetBulkSize(bulkSize int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.setBulkSize(bulkSize)
}

// TCPKeepAlive returns the tcp keep-alive value of the connector.
func (c *Connector) TCPKeepAlive() time.Duration {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c._tcpKeepAlive
}

/*
SetTCPKeepAlive sets the tcp keep-alive value of the connector.

For more information please see net.Dialer structure.
*/
func (c *Connector) SetTCPKeepAlive(tcpKeepAlive time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c._tcpKeepAlive = tcpKeepAlive
}

// DefaultSchema returns the database default schema of the connector.
func (c *Connector) DefaultSchema() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c._defaultSchema
}

// SetDefaultSchema sets the database default schema of the connector.
func (c *Connector) SetDefaultSchema(schema string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c._defaultSchema = schema
}

// TLSConfig returns the TLS configuration of the connector.
func (c *Connector) TLSConfig() *tls.Config {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c._tlsConfig.Clone()
}

// SetTLS sets the TLS configuration of the connector with given parameters. An existing connector TLS configuration is replaced.
func (c *Connector) SetTLS(serverName string, insecureSkipVerify bool, rootCAFiles ...string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.setTLS(serverName, insecureSkipVerify, rootCAFiles)
}

// SetTLSConfig sets the TLS configuration of the connector.
func (c *Connector) SetTLSConfig(tlsConfig *tls.Config) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c._tlsConfig = tlsConfig.Clone()
}

// Dialer returns the dialer object of the connector.
func (c *Connector) Dialer() dial.Dialer { c.mu.RLock(); defer c.mu.RUnlock(); return c._dialer }

// SetDialer sets the dialer object of the connector.
func (c *Connector) SetDialer(dialer dial.Dialer) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.setDialer(dialer)
}

// ApplicationName returns the application name of the connector.
func (c *Connector) ApplicationName() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c._applicationName
}

// SetApplicationName sets the application name of the connector.
func (c *Connector) SetApplicationName(name string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c._applicationName = name
}

// SessionVariables returns the session variables stored in connector.
func (c *Connector) SessionVariables() SessionVariables {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return maps.Clone(c._sessionVariables)
}

// SetSessionVariables sets the session varibles of the connector.
func (c *Connector) SetSessionVariables(sessionVariables SessionVariables) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c._sessionVariables = maps.Clone(sessionVariables)
}

// Locale returns the locale of the connector.
func (c *Connector) Locale() string { c.mu.RLock(); defer c.mu.RUnlock(); return c._locale }

/*
SetLocale sets the locale of the connector.

For more information please see http://help.sap.com/hana/SAP_HANA_SQL_Command_Network_Protocol_Reference_en.pdf.
*/
func (c *Connector) SetLocale(locale string) { c.mu.Lock(); defer c.mu.Unlock(); c._locale = locale }

// FetchSize returns the fetchSize of the connector.
func (c *Connector) FetchSize() int { c.mu.RLock(); defer c.mu.RUnlock(); return c._fetchSize }

/*
SetFetchSize sets the fetchSize of the connector.

For more information please see DSNFetchSize.
*/
func (c *Connector) SetFetchSize(fetchSize int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.setFetchSize(fetchSize)
}

// LobChunkSize returns the lobChunkSize of the connector.
func (c *Connector) LobChunkSize() int { c.mu.RLock(); defer c.mu.RUnlock(); return c._lobChunkSize }

// SetLobChunkSize sets the lobChunkSize of the connector.
func (c *Connector) SetLobChunkSize(lobChunkSize int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.setLobChunkSize(lobChunkSize)
}

// Dfv returns the client data format version of the connector.
func (c *Connector) Dfv() int { c.mu.RLock(); defer c.mu.RUnlock(); return c._dfv }

// SetDfv sets the client data format version of the connector.
func (c *Connector) SetDfv(dfv int) { c.mu.Lock(); defer c.mu.Unlock(); c.setDfv(dfv) }

// CESU8Decoder returns the CESU-8 decoder of the connector.
func (c *Connector) CESU8Decoder() func() transform.Transformer {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c._cesu8DecoderFn
}

// SetCESU8Decoder sets the CESU-8 decoder of the connector.
func (c *Connector) SetCESU8Decoder(cesu8DecoderFn func() transform.Transformer) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if cesu8DecoderFn == nil {
		cesu8DecoderFn = cesu8.DefaultDecoder
	}
	c._cesu8DecoderFn = cesu8DecoderFn
}

// CESU8Encoder returns the CESU-8 encoder of the connector.
func (c *Connector) CESU8Encoder() func() transform.Transformer {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c._cesu8EncoderFn
}

// SetCESU8Encoder sets the CESU-8 encoder of the connector.
func (c *Connector) SetCESU8Encoder(cesu8EncoderFn func() transform.Transformer) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if cesu8EncoderFn == nil {
		cesu8EncoderFn = cesu8.DefaultEncoder
	}
	c._cesu8EncoderFn = cesu8EncoderFn
}

/*
EmptyDateAsNull returns NULL for empty dates ('0000-00-00') if true, otherwise:

For data format version 1 the backend does return the NULL indicator for empty date fields.
For data format version non equal 1 (field type daydate) the NULL indicator is not set and the return value is 0.
As value 1 represents '0001-01-01' (the minimal valid date) without setting EmptyDateAsNull '0000-12-31' is returned,
so that NULL, empty and valid dates can be distinguished.

https://help.sap.com/docs/HANA_SERVICE_CF/7c78579ce9b14a669c1f3295b0d8ca16/3f81ccc7e35d44cbbc595c7d552c202a.html?locale=en-US
*/
func (c *Connector) EmptyDateAsNull() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c._emptyDateAsNull
}

// SetEmptyDateAsNull sets the EmptyDateAsNull flag of the connector.
func (c *Connector) SetEmptyDateAsNull(emptyDateAsNull bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c._emptyDateAsNull = emptyDateAsNull
}

// Logger returns the Logger instance of the connector.
func (c *Connector) Logger() *slog.Logger {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c._logger
}

// SetLogger sets the Logger instance of the connector.
func (c *Connector) SetLogger(logger *slog.Logger) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if logger == nil {
		logger = slog.Default()
	}
	c._logger = logger
}

// auth attributes.
func (c *Connector) cookieAuth() *p.AuthHnd {
	if !c.hasCookie.Load() { // fastpath without lock
		return nil
	}

	c.mu.RLock()
	defer c.mu.RUnlock()

	auth := p.NewAuthHnd(c._logonname)                              // important: for session cookie auth we do need the logonname from JWT auth,
	auth.AddSessionCookie(c._sessionCookie, c._logonname, clientID) // and for HANA onPrem the final session cookie req needs the logonname as well.
	return auth
}

func (c *Connector) authHnd() *p.AuthHnd {
	c.mu.RLock()
	defer c.mu.RUnlock()

	authHnd := p.NewAuthHnd(c._username) // use username as logonname
	if c._certKey != nil {
		authHnd.AddX509(c._certKey)
	}
	if c._token != "" {
		authHnd.AddJWT(c._token)
	}
	// mimic standard drivers and use password as token if user is empty
	if c._token == "" && c._username == "" && isJWTToken(c._password) {
		authHnd.AddJWT(c._password)
	}
	if c._password != "" {
		authHnd.AddBasic(c._username, c._password)
		authHnd.AddLDAP(c._username, c._password)
	}
	return authHnd
}

func (c *Connector) refresh() (bool, error) {
	refreshed := false

	callRefreshPassword := func(refreshPassword func() (string, bool)) (string, bool) {
		defer c.mu.Lock() // finally lock attr again
		c.mu.Unlock()     // unlock attr, so that callback can call attr methods
		return refreshPassword()
	}

	callRefreshToken := func(refreshToken func() (token string, ok bool)) (string, bool) {
		defer c.mu.Lock() // finally lock attr again
		c.mu.Unlock()     // unlock attr, so that callback can call attr methods
		return refreshToken()
	}

	callRefreshClientCert := func(refreshClientCert func() (clientCert, clientKey []byte, ok bool)) (unique.Handle[string], unique.Handle[string], bool) {
		var handle unique.Handle[string]
		defer c.mu.Lock() // finally lock attr again
		c.mu.Unlock()     // unlock attr, so that callback can call attr methods
		clientCert, clientKey, ok := refreshClientCert()
		if !ok {
			return handle, handle, false
		}
		return unique.Make(string(clientCert)), unique.Make(string(clientKey)), true
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if c._refreshPasswordFn != nil {
		if password, ok := callRefreshPassword(c._refreshPasswordFn); ok {
			if password != c._password {
				c._password = password
				refreshed = true
			}
		}
	}
	if c._refreshTokenFn != nil {
		if token, ok := callRefreshToken(c._refreshTokenFn); ok {
			if token != c._token {
				c._token = token
				refreshed = true
			}
		}
	}
	if c._refreshClientCertFn != nil {
		if certHandle, keyHandle, ok := callRefreshClientCert(c._refreshClientCertFn); ok {
			if c._certKey == nil || !c._certKey.Equal(certHandle, keyHandle) {
				certKey, err := auth.NewCertKey(certHandle, keyHandle)
				if err != nil {
					return refreshed, err
				}
				c._certKey = certKey
				refreshed = true
			}
		}
	} else if c._certFile != "" && c._keyFile != "" {
		if certHandle, keyHandle, err := readCertKeyFiles(c._certFile, c._keyFile); err == nil {
			if c._certKey == nil || !c._certKey.Equal(certHandle, keyHandle) {
				certKey, err := auth.NewCertKey(certHandle, keyHandle)
				if err != nil {
					return refreshed, err
				}
				c._certKey = certKey
				refreshed = true
			}
		}
	}
	return refreshed, nil
}

func (c *Connector) invalidateCookie() { c.hasCookie.Store(false) }

func (c *Connector) setCookie(logonname string, sessionCookie []byte) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.hasCookie.Store(true)
	c._logonname = logonname
	c._sessionCookie = sessionCookie
}

// Username returns the username of the connector.
func (c *Connector) Username() string { c.mu.RLock(); defer c.mu.RUnlock(); return c._username }

// Password returns the basic authentication password of the connector.
func (c *Connector) Password() string { c.mu.RLock(); defer c.mu.RUnlock(); return c._password }

// SetPassword sets the basic authentication password of the connector.
func (c *Connector) SetPassword(password string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c._password = password
}

// RefreshPassword returns the callback function for basic authentication password refresh.
func (c *Connector) RefreshPassword() func() (password string, ok bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c._refreshPasswordFn
}

// SetRefreshPassword sets the callback function for basic authentication password refresh.
// The callback function might be called simultaneously from multiple goroutines only if registered
// for more than one Connector.
func (c *Connector) SetRefreshPassword(refreshPasswordFn func() (password string, ok bool)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c._refreshPasswordFn = refreshPasswordFn
}

// ClientCert returns the X509 authentication client certificate and key of the connector.
func (c *Connector) ClientCert() (clientCert, clientKey []byte) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c._certKey == nil {
		return nil, nil
	}
	return c._certKey.Cert(), c._certKey.Key()
}

// RefreshClientCert returns the callback function for X509 authentication client certificate and key refresh.
func (c *Connector) RefreshClientCert() func() (clientCert, clientKey []byte, ok bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c._refreshClientCertFn
}

// SetRefreshClientCert sets the callback function for X509 authentication client certificate and key refresh.
// The callback function might be called simultaneously from multiple goroutines only if registered
// for more than one Connector.
func (c *Connector) SetRefreshClientCert(refreshClientCertFn func() (clientCert, clientKey []byte, ok bool)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c._refreshClientCertFn = refreshClientCertFn
}

// Token returns the JWT authentication token of the connector.
func (c *Connector) Token() string { c.mu.RLock(); defer c.mu.RUnlock(); return c._token }

// RefreshToken returns the callback function for JWT authentication token refresh.
func (c *Connector) RefreshToken() func() (token string, ok bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c._refreshTokenFn
}

// SetRefreshToken sets the callback function for JWT authentication token refresh.
// The callback function might be called simultaneously from multiple goroutines only if registered
// for more than one Connector.
func (c *Connector) SetRefreshToken(refreshTokenFn func() (token string, ok bool)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c._refreshTokenFn = refreshTokenFn
}
