package postgis

import "github.com/jackc/pgx/v5/pgxpool"

// connMeta contains sanitized, resolved connection metadata.
// safe to use in logs and error messages.
type connMeta struct {
	Mode           connMode
	EnvTriggerKeys []string

	Host     string
	Port     uint16
	Database string
	User     string

	SSLMode SSLMode
}

// newConnMeta constructs a connMeta from connPlan and pgxpool configuration.
func newConnMeta(plan connPlan, cfg *pgxpool.Config) connMeta {
	return connMeta{
		Mode:           plan.Mode,
		EnvTriggerKeys: plan.EnvTriggerKeys,
		Host:           cfg.ConnConfig.Host,
		Port:           cfg.ConnConfig.Port,
		Database:       cfg.ConnConfig.Database,
		User:           cfg.ConnConfig.User,
		SSLMode:        plan.SSLMode,
	}
}

// validator defines the interfaces responsible for validation a resolved pool
// configuration.
type validator interface {
	Validate(plan connPlan, cfg pgxpool.Config) error
}

// defaultValidator is the default implementation of validator for
// the PostGIS provider.
type defaultValidator struct{}

// Validate performs structural validation of the resolved pgxpool configuration.
// There is no validation when in URI mode. There is strict validation when in ENV mode.
func (v defaultValidator) Validate(plan connPlan, cfg *pgxpool.Config) (connMeta, error) {
	meta := newConnMeta(plan, cfg)

	if meta.Mode != connModeEnv {
		// we do not validate the content of the uri because it is
		// explicit and self-cointained.
		// the amount of viable combinations in a uri is too big
		// to validate too. if it is malformed or incomplete
		// pgxpool.ParseConfig or the connection will fail for us on
		// test ping. see issue#1058.
		return meta, nil
	}

	// connModeEnv however is implicit because it is triggered by
	// ambiant variables. it can also override a perfectly valid
	// uri so we need to make validation strict and a distriptive error
	// visible for the user on-connect.
	missing := []string{}
	if meta.Host == "" {
		missing = append(missing, "host")
	}
	if meta.Port == 0 {
		missing = append(missing, "port")
	}
	if meta.Database == "" {
		missing = append(missing, "database")
	}
	if meta.User == "" {
		missing = append(missing, "user")
	}

	if len(missing) > 0 {
		return connMeta{}, ErrEnvIncomplete{
			Triggers:      meta.EnvTriggerKeys,
			MissingFields: missing,
			URIWasIgnored: plan.URIProvided,
		}
	}

	return meta, nil
}
