package postgis

import (
	"slices"
	"testing"
)

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

	fn := func(t *testing.T, tc tcase) {
		for _, env := range tc.triggers {
			t.Setenv(env, "test")
		}

		es := &envSelector{}

		connMode, triggers := es.Select()
		if connMode != tc.expectedConnMode {
			t.Fatalf("expected ConnMode to be '%s' but got %s", tc.expectedConnMode, connMode)
		}

		// noop for connModeURI
		for _, trigger := range triggers {
			if !slices.Contains(tc.triggers, trigger) {
				t.Fatalf("expected all env triggers to be present in returned triggers: %s is missing", trigger)
			}
		}
	}

	for name, tc := range tcases {
		t.Run(name, func(t *testing.T) {
			fn(t, tc)
		})
	}
}
