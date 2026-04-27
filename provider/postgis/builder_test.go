package postgis

import (
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestBuilderParseInput(t *testing.T) {
	type tcase struct {
		plan        connPlan
		expectedURI string
	}

	tcases := map[string]tcase{
		"connModeEnv": {
			plan: connPlan{
				Mode:    connModeEnv,
				SSLMode: "disable",
			},
		},
		"connModeConfig": {
			plan: connPlan{
				Mode:      connModeURI,
				URIString: "postgres://user:password@localhost:5432/db?sslmode=disable",
			},
		},
	}

	fn := func(t *testing.T, tc tcase) {
		t.Helper()

		ctx := t.Context()
		var got string
		b := &defaultBuilder{
			parse: func(s string) (*pgxpool.Config, error) {
				got = s
				return pgxpool.ParseConfig("")
			},
		}
		_, err := b.Build(ctx, tc.plan)
		if err != nil {
			t.Fatal(err)
		}
		if got != tc.plan.URIString {
			t.Fatalf("expected parse input uri to be %q, got %q", tc.expectedURI, got)
		}
	}

	for name, tc := range tcases {
		t.Run(name, func(t *testing.T) {
			fn(t, tc)
		})
	}
}

func TestBuilderApplyRuntimeParams(t *testing.T) {
	type tcase struct {
		plan connPlan
	}

	tcases := map[string]tcase{
		"connModeEnv": {
			plan: connPlan{
				Mode:    connModeEnv,
				SSLMode: "disable",

				RuntimeParams: runtimeParams{
					"application_name": "testing",
				},
			},
		},
		"connModeConfig": {
			plan: connPlan{
				Mode:      connModeURI,
				URIString: "postgres://user:password@localhost:5432/db?sslmode=disable",
				RuntimeParams: runtimeParams{
					"application_name":              "testing",
					"default_transaction_read_only": "true",
				},
			},
		},
	}

	fn := func(t *testing.T, tc tcase) {
		ctx := t.Context()
		b := newDefaultBuilder()
		cfg, err := b.Build(ctx, tc.plan)
		if err != nil {
			t.Fatal(err)
		}

		for k, v := range tc.plan.RuntimeParams {
			got := cfg.ConnConfig.RuntimeParams[k]
			if got != v {
				t.Fatalf("expected %q but got %q", v, got)
			}
		}
	}

	for name, tc := range tcases {
		t.Run(name, func(t *testing.T) {
			fn(t, tc)
		})
	}
}

func TestBuilderApplyTLS(t *testing.T) {
	type tcase struct {
		sslmode          SSLMode
		expectNil        bool
		expectVerifyCA   bool
		expectServerName bool
	}

	tcases := map[SSLMode]tcase{
		SSLModeDisable: {
			sslmode:          SSLModeDisable,
			expectNil:        true,
			expectVerifyCA:   false,
			expectServerName: false,
		},
		SSLModeEmpty: {
			sslmode:          SSLModeEmpty,
			expectNil:        true,
			expectVerifyCA:   false,
			expectServerName: false,
		},
		SSLModeRequire: {
			sslmode:          SSLModeRequire,
			expectNil:        false,
			expectVerifyCA:   false,
			expectServerName: false,
		},
		SSLModeVerifyFull: {
			sslmode:          SSLModeVerifyFull,
			expectNil:        false,
			expectVerifyCA:   false,
			expectServerName: true,
		},
		SSLModeVerifyCA: {
			sslmode:          SSLModeVerifyCA,
			expectNil:        false,
			expectVerifyCA:   true,
			expectServerName: false,
		},
	}

	fn := func(t *testing.T, tc tcase) {
		ctx := t.Context()
		b := newDefaultBuilder()
		cfg, err := b.Build(ctx, connPlan{Mode: connModeEnv, SSLMode: tc.sslmode})
		if err != nil {
			t.Fatal(err)
		}

		if tc.expectNil {
			if cfg.ConnConfig.TLSConfig != nil {
				t.Fatal("expected TLSConfig to be nil")
			}
			return
		}

		fmt.Println(cfg.ConnConfig.TLSConfig)

		if cfg.ConnConfig.TLSConfig == nil {
			t.Fatal("expected TLSConfig to be not-nil")
		}

		if tc.expectVerifyCA && cfg.ConnConfig.TLSConfig.VerifyPeerCertificate == nil {
			t.Fatalf("expected VerifyPeerCertificate set when SSLMode=%q", string(tc.sslmode))
		}

		if tc.expectServerName && cfg.ConnConfig.TLSConfig.ServerName == "" {
			t.Fatal("expected ServerName to be set")
		}
	}

	for sslmode, tc := range tcases {
		t.Run(string(sslmode), func(t *testing.T) {
			fn(t, tc)
		})
	}
}

func TestBuilderApplyAfterConnectHook(t *testing.T) {
	ctx := t.Context()
	b := newDefaultBuilder()
	cfg, err := b.Build(ctx, connPlan{Mode: connModeEnv, SSLMode: SSLModeDisable})
	if err != nil {
		t.Fatal(err)
	}

	if cfg.AfterConnect == nil {
		t.Fatal("expected AfterConnect hooks to be not-nil")
	}
}
