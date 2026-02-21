package postgis

import "testing"

func TestEnvSelectorConnModeEnv(t *testing.T) {
	type tcase struct {
		triggers         []string
		expectedConnMode connMode
	}

	tcases := map[string]tcase{
		"connModeEnv": {
			triggers:         connModeEnvTriggers,
			expectedConnMode: connModeEnv,
		},
		"connModeURI": {
			triggers: []string{
				"PGAPPNAME",
				"PGCONNECT_TIMEOUT",
				"PGTARGETSESSIONATTRS",
			},
			expectedConnMode: connModeURI,
		},
	}

	fn := func(t *testing.T, env string, expectedConnMode connMode) {
		t.Setenv(env, "something")
		es := &envSelector{}

		connMode := es.Select()
		if connMode != expectedConnMode {
			t.Fatalf("expected ConnMode to be '%s' but got %s", expectedConnMode, connMode)
		}
	}

	for name, tc := range tcases {
		for _, env := range tc.triggers {
			t.Run(name, func(t *testing.T) {
				fn(t, env, tc.expectedConnMode)
			})
		}
	}
}
