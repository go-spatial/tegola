package cmd

import (
	"strings"
	"testing"

	"github.com/alexeydott/tegola/config"
	"github.com/alexeydott/tegola/server"
)

// TestResolveServerPort asserts the CLI --port / config precedence for the
// serve command: an explicitly-passed --port always wins over the config value,
// while an unset flag falls back to the config, then to the flag default.
func TestResolveServerPort(t *testing.T) {
	tests := []struct {
		name       string
		flagSet    bool
		flagPort   string
		configPort string
		want       string
	}{
		{
			name:       "explicit flag overrides config even when equal to default",
			flagSet:    true,
			flagPort:   ":8080",
			configPort: ":9000",
			want:       ":8080",
		},
		{
			name:       "explicit flag overrides config",
			flagSet:    true,
			flagPort:   ":7000",
			configPort: ":9000",
			want:       ":7000",
		},
		{
			name:       "unset flag uses config",
			flagSet:    false,
			flagPort:   ":8080",
			configPort: ":9000",
			want:       ":9000",
		},
		{
			name:       "unset flag and empty config uses flag default",
			flagSet:    false,
			flagPort:   ":8080",
			configPort: "",
			want:       ":8080",
		},
		{
			name:       "explicit flag with empty config uses flag",
			flagSet:    true,
			flagPort:   ":7000",
			configPort: "",
			want:       ":7000",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := resolveServerPort(tc.flagSet, tc.flagPort, tc.configPort)
			if got != tc.want {
				t.Errorf("resolveServerPort(%v, %q, %q) = %q, want %q",
					tc.flagSet, tc.flagPort, tc.configPort, got, tc.want)
			}
		})
	}
}

// TestConfigureTileOperations asserts the [webserver.tile_operations] TOML
// section reaches the server's tile operation gate: an explicit section is
// copied verbatim (including env-expanded token values), an absent section
// leaves the gate disabled, and zero rate/concurrent values pass through as
// zeros — the server resolves them to its defaults (60/min, 4 concurrent).
func TestConfigureTileOperations(t *testing.T) {
	t.Setenv("WS_SRV_TEST_TILE_OPS_TOKEN", "tok-from-env")

	tests := []struct {
		name string
		toml string
		want server.TileOperationsConfig
	}{
		{
			name: "full section with env expanded token",
			toml: `
[webserver.tile_operations]
enabled = true
token = "${WS_SRV_TEST_TILE_OPS_TOKEN}"
rate_per_minute = 120
max_concurrent = 8
`,
			want: server.TileOperationsConfig{
				Enabled:       true,
				Token:         "tok-from-env",
				RatePerMinute: 120,
				MaxConcurrent: 8,
			},
		},
		{
			name: "absent section leaves gate disabled",
			toml: `
[webserver]
port = ":8080"
`,
			want: server.TileOperationsConfig{},
		},
		{
			name: "zero rate and max pass through to server defaults",
			toml: `
[webserver.tile_operations]
enabled = true
token = "s3cret"
`,
			want: server.TileOperationsConfig{
				Enabled: true,
				Token:   "s3cret",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			conf, err := config.Parse(strings.NewReader(tc.toml), "")
			if err != nil {
				t.Fatalf("config.Parse: %v", err)
			}

			// assign through the same global the serve command wires up
			saved := server.TileOperations
			defer func() { server.TileOperations = saved }()
			server.TileOperations = configureTileOperations(conf.Webserver.TileOperations)

			if server.TileOperations != tc.want {
				t.Errorf("server.TileOperations = %+v, want %+v", server.TileOperations, tc.want)
			}
		})
	}
}
