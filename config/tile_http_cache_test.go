package config

import (
	"strings"
	"testing"
)

func TestTileHTTPMaxAge(t *testing.T) {
	t.Setenv("TEST_TILE_HTTP_MAX_AGE", "300")
	for _, tc := range []struct {
		value   string
		want    int
		invalid bool
	}{
		{"", 0, false}, {"0", 0, false}, {"300", 300, false},
		{`"${TEST_TILE_HTTP_MAX_AGE}"`, 300, false}, {"-1", -1, true},
	} {
		source := "[webserver]\n"
		if tc.value != "" {
			source += "tile_http_max_age = " + tc.value + "\n"
		}
		c, err := Parse(strings.NewReader(source), "test.toml")
		if err != nil {
			t.Fatal(err)
		}
		if int(c.Webserver.TileHTTPMaxAge) != tc.want {
			t.Fatalf("value %q: got %d", tc.value, c.Webserver.TileHTTPMaxAge)
		}
		err = c.Validate()
		if tc.invalid {
			if err == nil || !strings.Contains(err.Error(), "tile_http_max_age") {
				t.Fatalf("expected max age error, got %v", err)
			}
		} else if err != nil {
			t.Fatal(err)
		}
	}
}
