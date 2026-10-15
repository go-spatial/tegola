package postgis

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/alexeydott/tegola/dict"
)

// SSLMode represents a PostgreSQL sslmode value.
type SSLMode string

const (
	SSLModeEmpty      SSLMode = ""
	SSLModeDisable    SSLMode = "disable"
	SSLModeAllow      SSLMode = "allow"
	SSLModePrefer     SSLMode = "prefer"
	SSLModeRequire    SSLMode = "require"
	SSLModeVerifyCA   SSLMode = "verify-ca"
	SSLModeVerifyFull SSLMode = "verify-full"
)

type runtimeParams map[string]string

// connPlan is the resolved connection inputs from
// configuration and environment variables. It is the output of the
// planner and the input to the connection builder.
type connPlan struct {
	Mode           connMode
	EnvTriggerKeys []string

	URIProvided bool
	URIString   string

	// resolved ssl inputs (paths + mode) used by ConfigTLS
	SSLMode     SSLMode
	SSLKey      string
	SSLCert     string
	SSLRootCert string

	// resolved runtime params
	RuntimeParams runtimeParams

	// resolved pgxpool settings. Pool sizing is a client-side concern and
	// must not be sent to the server as runtime parameters; pgxpool only
	// honors pool_* keys when they are present at ParseConfig time.
	Pool PoolSettings
}

// PoolSettings holds resolved pgxpool sizing parameters. Zero values keep
// the pgxpool defaults.
type PoolSettings struct {
	MinConns              int32
	MinIdleConns          int32
	MaxConns              int32
	MaxConnLifetime       time.Duration
	MaxConnIdleTime       time.Duration
	MaxConnLifetimeJitter time.Duration
	HealthCheckPeriod     time.Duration
}

// planner defines the interface to creating a connection plan from
// a provider configuration and selected connMode.
type planner interface {
	Plan(cfg dict.Dicter, mode connMode, triggers []string) (connPlan, error)
}

// defaultPlanner is the default PostGIS planner
type defaultPlanner struct{}

// Plan creates parses the configuration, prioritizes environment variables over
// configuration values and creates a connPlan based off of the connMode.
func (dp defaultPlanner) Plan(cfg dict.Dicter, mode connMode, triggers []string) (connPlan, error) {
	cp := connPlan{}

	cp.Mode = mode
	cp.EnvTriggerKeys = triggers

	uriStr, _ := cfg.String(ConfigKeyURI, nil) //nolint:errcheck we validate for empty string instead
	cp.URIProvided = uriStr != ""
	cp.URIString = uriStr

	cp.RuntimeParams = resolveRunTimeParams(cfg, defaultRuntimeParamRules())

	if err := cp.Pool.resolveFromConfig(cfg); err != nil {
		return connPlan{}, err
	}

	// tls defaults from config
	sslMode := DefaultSSLMode
	sslMode, err := cfg.String(ConfigKeySSLMode, &sslMode)
	if err != nil {
		return connPlan{}, err
	}

	sslKey := DefaultSSLKey
	sslKey, err = cfg.String(ConfigKeySSLKey, &sslKey)
	if err != nil {
		return connPlan{}, err
	}

	sslCert := DefaultSSLCert
	sslCert, err = cfg.String(ConfigKeySSLCert, &sslCert)
	if err != nil {
		return connPlan{}, err
	}

	sslRoot := DefaultSSLCert
	sslRoot, err = cfg.String(ConfigKeySSLRootCert, &sslRoot)
	if err != nil {
		return connPlan{}, err
	}

	// this is where we allow env to override tls inputs
	// and return the finished plan
	if mode == connModeEnv {
		if v := strings.TrimSpace(os.Getenv("PGSSLMODE")); v != "" {
			sslMode = v
		}
		if v := strings.TrimSpace(os.Getenv("PGSSLKEY")); v != "" {
			sslKey = v
		}
		if v := strings.TrimSpace(os.Getenv("PGSSLCERT")); v != "" {
			sslCert = v
		}
		if v := strings.TrimSpace(os.Getenv("PGSSLROOTCERT")); v != "" {
			sslRoot = v
		}

		cp.SSLMode, cp.SSLKey, cp.SSLCert, cp.SSLRootCert = SSLMode(sslMode), sslKey, sslCert, sslRoot
		return cp, nil
	}

	if !cp.URIProvided {
		// on this path there's no way for us to create a configuration
		// we do not have environment variables for the connection
		// nor a URI from config.
		return connPlan{}, errors.New("neither env vars nor uri provided")
	}

	uri, err := url.Parse(cp.URIString)
	if err != nil {
		return connPlan{}, &ErrInvalidURI{Err: err}
	}

	// we now only validate the scheme. an exhaustive check here
	// is overkill and best handled when establishing and testing the connection.
	// see viable uri schema of libpq:
	// https://www.postgresql.org/docs/current/libpq-connect.html#LIBPQ-CONNSTRING-URIS
	if uri.Scheme != "postgres" && uri.Scheme != "postgresql" {
		return connPlan{}, &ErrInvalidURI{
			Msg: "invalid connection scheme " + uri.Scheme,
		}
	}

	query, err := url.ParseQuery(uri.RawQuery)
	if err != nil {
		return connPlan{}, &ErrInvalidURI{
			Err: err,
		}
	}

	// keep the raw URI: Build() feeds plan.URIString into pgxpool.ParseConfig,
	// so credentials must survive. The plan is never logged wholesale, and
	// ErrInvalidURI messages deliberately exclude the URI to avoid leaking
	// credentials into error output.
	cp.URIString = uri.String()
	cp.SSLMode, cp.SSLKey, cp.SSLCert, cp.SSLRootCert = SSLMode(sslMode), sslKey, sslCert, sslRoot

	if sslmode := query.Get("sslmode"); sslmode != "" {
		cp.SSLMode = SSLMode(sslmode)
	}

	return cp, nil
}

// sourceName identifies the origin of value used
// during the resolution of configuration values.
type sourceName string

const (
	sourceEnv     sourceName = "env"
	sourceConfig  sourceName = "config"
	sourceDefault sourceName = "default"
)

// valueSource describes the source of avalue when resolving
// configuration inputs. The first non-empty source is selected
// in order.
type valueSource struct {
	name  sourceName
	key   string
	value string
}

// runtimeParamRule describes how to resolve a pg runtime parameter where
// sources the order of what source takes precendence over another.
//
// Optionally allows (in order) to:
//
//	a) transform the value via callback
//	b) omit the value if callback is true
type runtimeParamRule struct {
	Name      string
	Sources   []valueSource
	Transform func(string) string // optional: transform value e.g. uppercase
	OmitIf    func(string) bool   // optional: if true, do not set the param
}

// defaultRuntimeParamSpecs returns the (currently available) runtime param rules
// for the PostGIS provider.
//
// NOTE: @iwpnd - the entire thing seems bloated at first, but makes
// adding, updating or deleting possible runtime paremters so much easier.
func defaultRuntimeParamRules() []runtimeParamRule {
	return []runtimeParamRule{
		{
			Name: "application_name",
			Sources: []valueSource{
				{
					name: sourceEnv,
					key:  "PGAPPNAME",
				},
				{
					name: sourceConfig,
					key:  ConfigKeyApplicationName,
				},
				{
					name:  sourceDefault,
					value: DefaultApplicationName,
				},
			},
		},
		{
			Name: "default_transaction_read_only",
			Sources: []valueSource{
				{
					name: sourceConfig,
					key:  ConfigKeyDefaultTransactionReadOnly,
				},
				{
					name:  sourceDefault,
					value: DefaultDefaultTransactionReadOnly,
				},
			},
			OmitIf: func(v string) bool {
				return v == "" || v == "OFF"
			},
		},
	}
}

// resolveFromConfig fills the pool settings from provider configuration keys.
// Pool sizing keys are deliberately kept out of RuntimeParams: pgxpool only
// interprets pool_* parameters while parsing the connection string, so values
// added to ConnConfig.RuntimeParams afterwards are sent to the server as
// regular GUCs and have no effect on the pool.
func (ps *PoolSettings) resolveFromConfig(cfg dict.Dicter) error {
	str := func(key string) (string, bool, error) {
		v := ""
		v, err := cfg.String(key, &v)
		if err != nil {
			return "", false, err
		}
		return v, v != "", nil
	}

	// count accepts the natural TOML integer form as well as the legacy
	// string form ("10") for backwards compatibility.
	count := func(key string) (int32, bool, error) {
		v, ok := cfg.Interface(key)
		if !ok {
			return 0, false, nil
		}
		switch n := v.(type) {
		case int:
			return int32(n), true, nil
		case string:
			parsed, err := strconv.ParseInt(n, 10, 32)
			if err != nil {
				return 0, false, fmt.Errorf("invalid %v value %q: %w", key, n, err)
			}
			return int32(parsed), true, nil
		default:
			return 0, false, fmt.Errorf("invalid %v value %v: expected an integer", key, v)
		}
	}

	parseDuration := func(key, v string) (time.Duration, error) {
		d, err := time.ParseDuration(v)
		if err != nil {
			return 0, fmt.Errorf("invalid %v value %q: %w", key, v, err)
		}
		return d, nil
	}

	// deprecated typo alias retained for backwards compatibility
	idleTimeKey := func() string {
		if _, ok := cfg.Interface(ConfigKeyPoolMaxConnIdleTimeCanonical); ok {
			return ConfigKeyPoolMaxConnIdleTimeCanonical
		}
		return ConfigKeyPoolMaxConnIdleTime
	}()

	if n, ok, err := count(ConfigKeyPoolMinConns); err != nil {
		return err
	} else if ok {
		ps.MinConns = n
	}

	if n, ok, err := count(ConfigKeyPoolMinIdleConns); err != nil {
		return err
	} else if ok {
		ps.MinIdleConns = n
	}

	if n, ok, err := count(ConfigKeyPoolMaxConns); err != nil {
		return err
	} else if ok {
		ps.MaxConns = n
	}

	if raw, ok, err := str(ConfigKeyPoolMaxConnLifeTime); err != nil {
		return err
	} else if ok {
		d, err := parseDuration(ConfigKeyPoolMaxConnLifeTime, raw)
		if err != nil {
			return err
		}
		ps.MaxConnLifetime = d
	}

	if raw, ok, err := str(idleTimeKey); err != nil {
		return err
	} else if ok {
		d, err := parseDuration(idleTimeKey, raw)
		if err != nil {
			return err
		}
		ps.MaxConnIdleTime = d
	}

	if raw, ok, err := str(ConfigKeyPoolMaxConnLifeTimeJitter); err != nil {
		return err
	} else if ok {
		d, err := parseDuration(ConfigKeyPoolMaxConnLifeTimeJitter, raw)
		if err != nil {
			return err
		}
		ps.MaxConnLifetimeJitter = d
	}

	if raw, ok, err := str(ConfigKeyPoolHealthCheckPeriod); err != nil {
		return err
	} else if ok {
		d, err := parseDuration(ConfigKeyPoolHealthCheckPeriod, raw)
		if err != nil {
			return err
		}
		ps.HealthCheckPeriod = d
	}

	return nil
}

// resolveRunTimeParams resolves runtime parameters according to the
// provided rules. For each rule, sources are evaluated in order and
// the first non-empty value is selected, optionally transformed, and
// conditionally omitted.
func resolveRunTimeParams(cfg dict.Dicter, rules []runtimeParamRule) map[string]string {
	out := map[string]string{}

	for _, rule := range rules {
		value := ""

		for _, src := range rule.Sources {
			candidate := ""

			switch src.name {
			case sourceEnv:
				candidate = os.Getenv(src.key)
			case sourceConfig:
				candidate, _ = cfg.String(src.key, nil) //nolint:errcheck
			default:
				candidate = src.value
			}

			if candidate != "" {
				value = candidate
				break
			}
		}

		if rule.Transform != nil {
			value = rule.Transform(value)
		}

		if rule.OmitIf != nil && rule.OmitIf(value) {
			continue
		}

		if value != "" {
			out[rule.Name] = value
		}
	}

	return out
}
