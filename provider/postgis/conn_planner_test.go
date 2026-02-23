package postgis

import (
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
		expectErr      bool
		expectedError  error
	}

	fn := func(t *testing.T, tc tcase) {
		planer := defaultPlanner{}

		plan, err := planer.Plan(tc.config, tc.mode, tc.envTriggerKeys)
		if err != nil {
			if !tc.expectErr {
				t.Fatalf("expected no error but got: %s", err)
			}

			if tc.expectedError != nil {
				if tc.expectedError != err {
					t.Fatalf("expected error to be %s but got: %s", tc.expectedError, err)
				}
			}

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
			envTriggerKeys: connEnvVars,
			setupFn: func(t *testing.T, triggers []string) {
				for _, trigger := range triggers {
					t.Setenv(trigger, "test")
				}
			},
			expectedPlan: connPlan{
				Mode:           connModeEnv,
				EnvTriggerKeys: connEnvVars,
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
