package postgis

import (
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestConnValidator(t *testing.T) {
	type tcase struct {
		plan      connPlan
		config    *pgxpool.Config
		expectErr bool
	}

	fn := func(t *testing.T, tc tcase) {
		v := &defaultValidator{}
		meta, err := v.Validate(tc.plan, tc.config)
		if tc.expectErr {
			var e ErrEnvIncomplete
			if !errors.As(err, &e) {
				t.Fatalf("expected ErrEnvIncomplete, got %T (%v)", err, err)
			}
			return
		}

		expectedDatabase := tc.config.ConnConfig.Database
		if meta.Database != expectedDatabase {
			t.Fatalf("expected database to be %s, got %s", expectedDatabase, meta.Database)
		}
		expectedHost := tc.config.ConnConfig.Host
		if meta.Host != expectedHost {
			t.Fatalf("expected host to be %s, got %s", expectedHost, meta.Host)
		}
		expectedUser := tc.config.ConnConfig.User
		if meta.User != expectedUser {
			t.Fatalf("expected user to be %s, got %s", expectedUser, meta.User)
		}
		expectedPort := tc.config.ConnConfig.Port
		if meta.Port != expectedPort {
			t.Fatalf("expected port to be %d, got %d", expectedPort, meta.Port)
		}
	}

	tcases := map[string]tcase{
		"connModeEnv passes": {
			plan: connPlan{
				Mode: connModeEnv,
			},
			config: &pgxpool.Config{
				ConnConfig: &pgx.ConnConfig{
					Config: pgconn.Config{
						Host:     "host",
						Port:     1337,
						User:     "user",
						Database: "database",
					},
				},
			},
			expectErr: false,
		},
		"connModeEnv errs": {
			plan: connPlan{
				Mode: connModeEnv,
			},
			config: &pgxpool.Config{
				ConnConfig: &pgx.ConnConfig{
					Config: pgconn.Config{
						Host: "host",
						Port: 1337,
						User: "user",
					},
				},
			},
			expectErr: true,
		},
		"connModeConfig passes": {
			plan: connPlan{
				Mode: connModeURI,
			},
			config: &pgxpool.Config{
				ConnConfig: &pgx.ConnConfig{
					Config: pgconn.Config{
						Host:     "host",
						Port:     1337,
						User:     "user",
						Database: "database",
					},
				},
			},
			expectErr: false,
		},
	}

	for name, tc := range tcases {
		t.Run(name, func(t *testing.T) {
			fn(t, tc)
		})
	}
}
