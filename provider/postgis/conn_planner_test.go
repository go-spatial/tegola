package postgis

import (
	"errors"
	"reflect"
	"testing"

	"github.com/go-spatial/tegola/dict"
)

func TestConnPlanerConnModeEnv(t *testing.T) {
	type tcase struct {
		mode           connMode
		config         dict.Dicter
		envTriggerKeys []string
		setupFn        func(t *testing.T, triggers []string)
		expectedPlan   connPlan

		expectErrFn func(t *testing.T, err error)
	}

	fn := func(t *testing.T, tc tcase) {
		planer := defaultPlanner{}

		plan, err := planer.Plan(tc.config, tc.mode, tc.envTriggerKeys)
		if err != nil && tc.expectErrFn == nil {
			t.Fatalf("expected plan to succeed but got: %s", err)
		}
		if err != nil && tc.expectErrFn != nil {
			tc.expectErrFn(t, err)
			return
		}

		if !reflect.DeepEqual(tc.expectedPlan, plan) {
			t.Fatalf("plans differ\nexpected: %#v\nactual:   %#v", tc.expectedPlan, plan)
		}
	}

	tcases := map[string]tcase{
		"all env overwrites": {
			mode:           connModeEnv,
			config:         dict.Dict{},
			envTriggerKeys: append(connModeEnvTriggers, sslEnvVars...),
			setupFn: func(t *testing.T, triggers []string) {
				for _, trigger := range triggers {
					t.Setenv(trigger, "test")
				}
			},
			expectedPlan: connPlan{
				Mode:           connModeEnv,
				EnvTriggerKeys: append(connModeEnvTriggers, sslEnvVars...),
				URIProvided:    false,
				URIString:      "",

				SSLMode:     "test",
				SSLKey:      "test",
				SSLCert:     "test",
				SSLRootCert: "test",

				RuntimeParams: resolveRunTimeParams(dict.Dict{}, defaultRuntimeParamRules()),
			},
		},
		"connection env overwrites, ssl defaults": {
			mode:           connModeEnv,
			config:         dict.Dict{},
			envTriggerKeys: connModeEnvTriggers,
			setupFn: func(t *testing.T, triggers []string) {
				for _, trigger := range triggers {
					t.Setenv(trigger, "test")
				}
			},
			expectedPlan: connPlan{
				Mode:           connModeEnv,
				EnvTriggerKeys: connModeEnvTriggers,
				URIProvided:    false,
				URIString:      "",

				SSLMode:     DefaultSSLMode,
				SSLKey:      DefaultSSLKey,
				SSLCert:     DefaultSSLCert,
				SSLRootCert: "",

				RuntimeParams: resolveRunTimeParams(dict.Dict{}, defaultRuntimeParamRules()),
			},
		},
		"ssl overwrites": {
			mode:           connModeEnv,
			config:         dict.Dict{},
			envTriggerKeys: sslEnvVars,
			setupFn: func(t *testing.T, triggers []string) {
				for _, trigger := range triggers {
					t.Setenv(trigger, "test")
				}
			},
			expectedPlan: connPlan{
				Mode:           connModeEnv,
				EnvTriggerKeys: sslEnvVars,
				URIProvided:    false,
				URIString:      "",

				SSLMode:     "test",
				SSLKey:      "test",
				SSLCert:     "test",
				SSLRootCert: "test",

				RuntimeParams: resolveRunTimeParams(dict.Dict{}, defaultRuntimeParamRules()),
			},
		},
		"no env overwrites and uri provided": {
			mode:           connModeURI,
			config:         dict.Dict(map[string]any{"uri": "postgres://user:password@host:1337/dbname"}),
			envTriggerKeys: connModeEnvTriggers,
			expectedPlan: connPlan{
				Mode:           connModeURI,
				EnvTriggerKeys: connModeEnvTriggers,

				URIProvided: true,
				URIString:   "postgres://user:password@host:1337/dbname",

				SSLMode:     DefaultSSLMode,
				SSLKey:      DefaultSSLKey,
				SSLCert:     DefaultSSLCert,
				SSLRootCert: "",

				RuntimeParams: resolveRunTimeParams(dict.Dict{}, defaultRuntimeParamRules()),
			},
		},
		"no env overwrites and uri provided and runtime params from config": {
			mode: connModeURI,
			config: dict.Dict(map[string]any{
				"uri":                           "postgres://user:password@host:1337/dbname",
				"application_name":              "ratatata",
				"default_transaction_read_only": "true",
			}),
			envTriggerKeys: connModeEnvTriggers,
			expectedPlan: connPlan{
				Mode:           connModeURI,
				EnvTriggerKeys: connModeEnvTriggers,

				URIProvided: true,
				URIString:   "postgres://user:password@host:1337/dbname",

				SSLMode:     DefaultSSLMode,
				SSLKey:      DefaultSSLKey,
				SSLCert:     DefaultSSLCert,
				SSLRootCert: "",

				RuntimeParams: map[string]string{
					"application_name":              "ratatata",
					"default_transaction_read_only": "true",
				},
			},
		},
		"no env overwrites and uri provided and runtime params from config with omission": {
			mode: connModeURI,
			config: dict.Dict(map[string]any{
				"uri":                           "postgres://user:password@host:1337/dbname",
				"application_name":              "ratatata",
				"default_transaction_read_only": "OFF",
				"pool_min_conns":                "1",
			}),
			envTriggerKeys: connModeEnvTriggers,
			expectedPlan: connPlan{
				Mode:           connModeURI,
				EnvTriggerKeys: connModeEnvTriggers,

				URIProvided: true,
				URIString:   "postgres://user:password@host:1337/dbname",

				SSLMode:     DefaultSSLMode,
				SSLKey:      DefaultSSLKey,
				SSLCert:     DefaultSSLCert,
				SSLRootCert: "",

				RuntimeParams: map[string]string{
					"application_name": "ratatata",
					"pool_min_conns":   "1",
				},
			},
		},
		"no env overwrites and uri provided and ssl mode overwrite from query": {
			mode: connModeURI,
			config: dict.Dict(map[string]any{
				"uri": "postgres://user:password@host:1337/dbname?sslmode=disable",
			}),
			envTriggerKeys: connModeEnvTriggers,
			expectedPlan: connPlan{
				Mode:           connModeURI,
				EnvTriggerKeys: connModeEnvTriggers,

				URIProvided: true,
				URIString:   "postgres://user:password@host:1337/dbname?sslmode=disable",

				SSLMode:     SSLModeDisable,
				SSLKey:      DefaultSSLKey,
				SSLCert:     DefaultSSLCert,
				SSLRootCert: "",

				RuntimeParams: resolveRunTimeParams(dict.Dict{}, defaultRuntimeParamRules()),
			},
		},
		"not env mode no uri errs": {
			mode:           connModeURI,
			config:         dict.Dict(map[string]any{}),
			envTriggerKeys: []string{},
			expectErrFn: func(t *testing.T, err error) {
				t.Helper()

				if err.Error() != "neither env vars nor uri provided" {
					t.Fatalf("expected empty uri error, got %T (%v)", err, err)
				}
			},
		},
		"invalid uri scheme": {
			mode:           connModeURI,
			config:         dict.Dict(map[string]any{"uri": "https://postgres:postgres@localhost:5432/tegola"}),
			envTriggerKeys: []string{},
			expectErrFn: func(t *testing.T, err error) {
				var e *ErrInvalidURI
				if !errors.As(err, &e) {
					t.Fatalf("expected invalid uri error, got %T (%v)", err, err)
				}
			},
		},
		"invalid character in uri": {
			mode: connModeURI,
			config: dict.Dict(map[string]any{
				"uri": "postgresql://postgres:post<gres@localhost:5432/tegola",
			}),
			envTriggerKeys: []string{},
			expectErrFn: func(t *testing.T, err error) {
				var e *ErrInvalidURI
				if !errors.As(err, &e) {
					t.Fatalf("expected invalid uri error, got %T (%v)", err, err)
				}
			},
		},
		"invalid character in query params": {
			mode: connModeURI,
			config: dict.Dict(map[string]any{
				"uri": "postgresql://postgres:postgres@localhost:5432/tegola?foo=;bar",
			}),
			envTriggerKeys: []string{},
			expectErrFn: func(t *testing.T, err error) {
				var e *ErrInvalidURI
				if !errors.As(err, &e) {
					t.Fatalf("expected invalid uri error, got %T (%v)", err, err)
				}
			},
		},
	}

	for name, tc := range tcases {
		t.Run(name, func(t *testing.T) {
			if tc.setupFn != nil {
				tc.setupFn(t, tc.envTriggerKeys)
			}

			fn(t, tc)
		})
	}
}
