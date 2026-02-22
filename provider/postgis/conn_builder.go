package postgis

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"os"
	"time"

	pgxuuid "github.com/jackc/pgx-gofrs-uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/tracelog"
)

// builder defines the interface responsible for constructing a pgxpool
// configuration from a resolved connection plan. It is wiring up
// the configuration, it is not responsible for validation.
type builder interface {
	Build(ctx context.Context, plan connPlan) (*pgxpool.Config, error)
}

type parseConfigFn func(uri string) (*pgxpool.Config, error)

// defaultBuilder is the default pgxpool config builder for the postgis provider.
// It construct the pgxpool.Config from a connPlan, appplyes tracer configuration,
// connection hooks, runtime parameters and TLS settings.
type defaultBuilder struct {
	parse parseConfigFn
}

// newDefaultBuilder creates a new defaultBuilder..
func newDefaultBuilder() *defaultBuilder {
	return &defaultBuilder{parse: pgxpool.ParseConfig}
}

// Build builds the connection pgxpool config from the connection plan.
func (b *defaultBuilder) Build(ctx context.Context, plan connPlan) (*pgxpool.Config, error) {
	uri := ""
	if plan.Mode == connModeURI {
		uri = plan.URIString
	}

	cfg, err := b.parse(uri)
	if err != nil {
		return nil, err
	}

	b.applyTracer(cfg)
	b.applyAfterConnectHook(cfg)
	b.applyRunTimeParams(cfg, plan)

	if tErr := b.applyTLS(cfg, plan); tErr != nil {
		return nil, tErr
	}

	return cfg, nil
}

// applyTracer injects our logger into the pgx logger.
func (b *defaultBuilder) applyTracer(cfg *pgxpool.Config) {
	logAdapter := NewLoggerAdapter()
	tracer := &tracelog.TraceLog{
		Logger:   logAdapter,
		LogLevel: tracelog.LogLevelWarn,
	}
	cfg.ConnConfig.Tracer = tracer
}

// applyAfterConnectHook adds the hstore and uuid type to the pool configs
// after connect hooks.
func (b *defaultBuilder) applyAfterConnectHook(cfg *pgxpool.Config) {
	type hstoreOID struct {
		OID     uint32
		hasInit bool
	}
	var hstore hstoreOID
	cfg.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
		// The AfterConnect call runs everytime a new connection is acquired,
		// including everytime the connection pool expands. The hstore OID
		// is not constant, so we lookup the OID once per provider and store it.
		// Extensions have to be registered for every new connection.
		if !hstore.hasInit {
			row := conn.QueryRow(ctx, "SELECT oid FROM pg_type WHERE typname = 'hstore';")
			if err := row.Scan(&hstore.OID); err != nil {
				switch {
				case errors.Is(err, pgx.ErrNoRows):
					// do nothing, because query can be empty if hstore is not installed
					break
				default:
					return fmt.Errorf("error fetching hstore oid: %w", err)
				}
			}

			hstore.hasInit = true
		}

		// dont register hstore data type if hstore extension is not installed
		if hstore.OID != 0 {
			conn.TypeMap().RegisterType(&pgtype.Type{
				Name:  "hstore",
				OID:   hstore.OID,
				Codec: pgtype.HstoreCodec{},
			})
		}

		// register UUID type, see https://github.com/jackc/pgx/wiki/UUID-Support
		pgxuuid.Register(conn.TypeMap())

		return nil
	}
}

// applyRunTimeParams adds adds runtime parameters to the pool config.
func (b *defaultBuilder) applyRunTimeParams(cfg *pgxpool.Config, plan connPlan) {
	cfg.ConnConfig.RuntimeParams = plan.RuntimeParams
}

// applyTLS applys the connection plan TLS configuration to the pool config.
func (b *defaultBuilder) applyTLS(cfg *pgxpool.Config, plan connPlan) error {
	switch plan.SSLMode {
	case SSLModeDisable, SSLModeEmpty:
		cfg.ConnConfig.TLSConfig = nil
		return nil

	case SSLModeAllow, SSLModePrefer, SSLModeRequire:
		cfg.ConnConfig.TLSConfig = &tls.Config{
			InsecureSkipVerify: true,
		}

	case SSLModeVerifyFull:
		// Standard Go verification: chain + hostname.
		// ServerName is required for hostname verification.
		cfg.ConnConfig.TLSConfig = &tls.Config{
			ServerName: cfg.ConnConfig.Host,
		}

	case SSLModeVerifyCA:
		// Go doesn't support "verify chain but skip hostname" directly.
		// We must set InsecureSkipVerify=true and perform chain verification ourselves.
		cfg.ConnConfig.TLSConfig = &tls.Config{
			InsecureSkipVerify: true,
		}

		installVerifyCA(cfg.ConnConfig.TLSConfig)

	default:
		return ErrInvalidSSLMode(plan.SSLMode)
	}

	// Load root certs if provided (otherwise system roots apply).
	if plan.SSLRootCert != "" {
		pool := x509.NewCertPool()

		pem, err := os.ReadFile(plan.SSLRootCert)
		if err != nil {
			return fmt.Errorf("unable to read CA file (%q): %w", plan.SSLRootCert, err)
		}
		if !pool.AppendCertsFromPEM(pem) {
			return fmt.Errorf("unable to add CA to cert pool")
		}

		cfg.ConnConfig.TLSConfig.RootCAs = pool
	}

	// Optional client cert auth.
	if (plan.SSLCert == "") != (plan.SSLKey == "") {
		return fmt.Errorf("both 'sslcert' and 'sslkey' are required")
	}
	if plan.SSLCert != "" {
		cert, err := tls.LoadX509KeyPair(plan.SSLCert, plan.SSLKey)
		if err != nil {
			return fmt.Errorf("unable to read cert: %w", err)
		}
		cfg.ConnConfig.TLSConfig.Certificates = []tls.Certificate{cert}
	}

	return nil
}

// installVerifyCA configures VerifyPeerCertificate to validate the server cert chain
// against RootCAs (or system roots if RootCAs is nil), without hostname verification.
func installVerifyCA(tcfg *tls.Config) {
	tcfg.VerifyPeerCertificate = func(rawCerts [][]byte, _ [][]*x509.Certificate) error {
		if len(rawCerts) == 0 {
			return fmt.Errorf("no server certificates presented")
		}

		leaf, err := x509.ParseCertificate(rawCerts[0])
		if err != nil {
			return fmt.Errorf("parse leaf certificate: %w", err)
		}

		roots := tcfg.RootCAs
		if roots == nil {
			sys, err := x509.SystemCertPool()
			if err != nil {
				return fmt.Errorf("load system cert pool: %w", err)
			}
			roots = sys
		}

		intermediates := x509.NewCertPool()
		for i := 1; i < len(rawCerts); i++ {
			c, err := x509.ParseCertificate(rawCerts[i])
			if err != nil {
				return fmt.Errorf("parse intermediate certificate: %w", err)
			}
			intermediates.AddCert(c)
		}

		opts := x509.VerifyOptions{
			Roots:         roots,
			Intermediates: intermediates,
			CurrentTime:   time.Now(),
			KeyUsages:     []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
			// DNSName intentionally empty -> no hostname verification
		}

		if _, err := leaf.Verify(opts); err != nil {
			return fmt.Errorf("verify-ca failed: %w", err)
		}
		return nil
	}
}
