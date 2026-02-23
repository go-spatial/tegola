package postgis

import (
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/go-spatial/tegola/dict"
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
		return connPlan{}, fmt.Errorf("mmmissing %q in config and env mode not selected", ConfigKeyURI)
	}

	uri, err := url.Parse(cp.URIString)
	if err != nil {
		return connPlan{}, ErrInvalidURI{Err: err}
	}

	// we now only validate the scheme. an exhaustive check here
	// is overkill and best handled when establishing and testing the connection.
	// see viable uri schema of libpq:
	// https://www.postgresql.org/docs/current/libpq-connect.html#LIBPQ-CONNSTRING-URIS
	if uri.Scheme != "postgres" && uri.Scheme != "postgresql" {
		return connPlan{}, ErrInvalidURI{
			Msg: "postgis: invalid connection scheme " + uri.Scheme,
		}
	}

	query, err := url.ParseQuery(uri.RawQuery)
	if err != nil {
		return connPlan{}, ErrInvalidURI{
			Err: err,
		}
	}

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
		{
			Name: ConfigKeyPoolMinConns,
			Sources: []valueSource{
				{
					name: sourceConfig,
					key:  ConfigKeyPoolMinConns,
				},
			},
			OmitIf: omitIfEmpty,
		},
		{
			Name: ConfigKeyPoolMinIdleConns,
			Sources: []valueSource{
				{
					name: sourceConfig,
					key:  ConfigKeyPoolMinIdleConns,
				},
			},
			OmitIf: omitIfEmpty,
		},
		{
			Name: ConfigKeyPoolMaxConnLifeTime,
			Sources: []valueSource{
				{
					name: sourceConfig,
					key:  ConfigKeyPoolMaxConnLifeTime,
				},
			},
			OmitIf: omitIfEmpty,
		},
		{
			Name: ConfigKeyPoolMaxConnIdleTime,
			Sources: []valueSource{
				{
					name: sourceConfig,
					key:  ConfigKeyPoolMaxConnIdleTime,
				},
			},
			OmitIf: omitIfEmpty,
		},
		{
			Name: ConfigKeyPoolHealthCheckPeriod,
			Sources: []valueSource{
				{
					name: sourceConfig,
					key:  ConfigKeyPoolHealthCheckPeriod,
				},
			},
			OmitIf: omitIfEmpty,
		},
		{
			Name: ConfigKeyPoolMaxConnLifeTimeJitter,
			Sources: []valueSource{
				{
					name: sourceConfig,
					key:  ConfigKeyPoolMaxConnLifeTimeJitter,
				},
			},
			OmitIf: omitIfEmpty,
		},
	}
}

func omitIfEmpty(v string) bool { return v == "" }

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
