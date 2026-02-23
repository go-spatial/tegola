package postgis

import (
	"context"
	"fmt"
	"time"

	"github.com/go-spatial/tegola/dict"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	// defaultPingTimeout defines the default timeout used when no deadline
	// is present on the provided context during Connect.
	defaultPingTimeout = time.Duration(2 * time.Second)
)

// connector defines the interface responsible for establishing a
// validated PostgreSQL connection pool.
type connector interface {
	Connect(ctx context.Context) (*pgxpool.Pool, *pgxpool.Config, connMeta, error)
}

// newDefaultConnector constructs a defaultConnector using the provided
// configuration.
func newDefaultConnector(cfg dict.Dicter) *defaultConnector {
	return &defaultConnector{
		cfg:         cfg,
		pingTimeout: defaultPingTimeout,

		selector:  &envSelector{},
		planner:   &defaultPlanner{},
		builder:   newDefaultBuilder(),
		validator: &defaultValidator{},
	}
}

// defaultConnector is the default implementation of connector for
// the PostGIS provider. It coordinates mode selection, planning,
// configuration building, validation, and pool creation.
type defaultConnector struct {
	cfg         dict.Dicter
	pingTimeout time.Duration

	selector  selector
	planner   planner
	builder   builder
	validator validator
}

// Connect establishes a PostGIS connection pool using the configured
// selector, planner, builder, and validator.
//
// It resolves the connection mode, builds a pgxpool.Config from the
// resulting plan, validates the resolved configuration, creates the
// pool, and performs a connectivity check via Ping.
//
// If the provided context does not contain a deadline, a timeout based
// on defaultPingTimeout (or the configured value) is applied to the
// connect and ping operations.
func (c *defaultConnector) Connect(ctx context.Context) (*pgxpool.Pool, *pgxpool.Config, connMeta, error) {
	ctxConnect := ctx
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctxConnect, cancel = context.WithTimeout(ctx, c.pingTimeout)
		defer cancel()
	}

	mode, triggers := c.selector.Select()
	plan, err := c.planner.Plan(c.cfg, mode, triggers)
	if err != nil {
		return nil, nil, connMeta{}, err
	}

	pgxCfg, err := c.builder.Build(ctxConnect, plan)
	if err != nil {
		return nil, nil, connMeta{}, err
	}

	meta, err := c.validator.Validate(plan, pgxCfg)
	if err != nil {
		return nil, nil, meta, err
	}

	pool, err := pgxpool.NewWithConfig(ctxConnect, pgxCfg)
	if err != nil {
		return nil, nil, meta, err
	}

	err = pool.Ping(ctxConnect)
	if err != nil {
		pool.Close()
		return nil, nil, meta, wrapMeta(meta, err)
	}

	return pool, pgxCfg, meta, nil
}

func (c *defaultConnector) WithPingTimeout(d time.Duration) *defaultConnector {
	c.pingTimeout = d
	return c
}

// wrapMeta wraps a connection error with sanitized connection metadata.
func wrapMeta(meta connMeta, err error) error {
	return fmt.Errorf(
		"connection failed (mode=%s, triggers=%s, host=%s, db=%s, user=%s, sslmode=%s): %w",
		string(meta.Mode), buildBuffer(meta.EnvTriggerKeys),
		meta.Host, meta.Database, meta.User,
		string(meta.SSLMode), err,
	)
}
