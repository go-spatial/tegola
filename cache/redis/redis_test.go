package redis_test

import (
	"context"
	"crypto/tls"
	"errors"
	"net"
	"os"
	"reflect"
	"strings"
	"syscall"
	"testing"
	"time"

	goredis "github.com/redis/go-redis/v9"

	"github.com/alexeydott/tegola/cache"
	"github.com/alexeydott/tegola/cache/redis"
	"github.com/alexeydott/tegola/dict"
	"github.com/alexeydott/tegola/internal/ttools"
)

// TESTENV is the environment variable that must be set to "yes" to run the
// redis tests that require a live redis instance on 127.0.0.1:6379. The
// config parsing and rejection tests run unconditionally.
const TESTENV = "RUN_REDIS_TESTS"

// testURI connects to the local redis used by the gated integration tests.
const testURI = "redis://127.0.0.1:6379/0"

// TestCreateOptions tests parsing of the uri based connection config.
func TestCreateOptions(t *testing.T) {
	t.Parallel()

	type tcase struct {
		config   dict.Dict
		expected *goredis.Options
	}

	fn := func(tc tcase) func(*testing.T) {
		return func(t *testing.T) {
			t.Parallel()

			actual, err := redis.CreateOptions(tc.config)
			if err != nil {
				t.Fatalf("unexpected error: %q", err)
			}
			compareOptions(t, actual, tc.expected)
		}
	}

	tests := map[string]tcase{
		"uri with password and db": {
			config: map[string]any{
				"uri": "redis://:secret@127.0.0.1:6379/3",
			},
			expected: &goredis.Options{
				Network:  "tcp",
				Addr:     "127.0.0.1:6379",
				Password: "secret",
				DB:       3,
			},
		},
		"uri with username and password": {
			config: map[string]any{
				"uri": "redis://alice:secret@example.org:6380/1",
			},
			expected: &goredis.Options{
				Network:  "tcp",
				Addr:     "example.org:6380",
				Username: "alice",
				Password: "secret",
				DB:       1,
			},
		},
		"uri without credentials": {
			config: map[string]any{
				"uri": "redis://example.org",
			},
			expected: &goredis.Options{
				Network: "tcp",
				Addr:    "example.org:6379",
			},
		},
		"uri uses host and port defaults": {
			config: map[string]any{
				"uri": "redis://",
			},
			expected: &goredis.Options{
				Network: "tcp",
				Addr:    "localhost:6379",
			},
		},
		"rediss scheme enables tls": {
			config: map[string]any{
				"uri": "rediss://:secret@example.org:6379/2",
			},
			expected: &goredis.Options{
				Network:   "tcp",
				Addr:      "example.org:6379",
				Password:  "secret",
				DB:        2,
				TLSConfig: &tls.Config{ /* no deep comparison */ },
			},
		},
		"unix socket with db query parameter": {
			config: map[string]any{
				"uri": "unix://:secret@/var/run/redis.sock?db=4",
			},
			expected: &goredis.Options{
				Network:  "unix",
				Addr:     "/var/run/redis.sock",
				Password: "secret",
				DB:       4,
			},
		},
		"db query parameter wins over path": {
			config: map[string]any{
				"uri": "redis://127.0.0.1:6379/3?db=5",
			},
			expected: &goredis.Options{
				Network: "tcp",
				Addr:    "127.0.0.1:6379",
				DB:      5,
			},
		},
		"pool and dial options as query parameters": {
			config: map[string]any{
				"uri": "redis://127.0.0.1:6379/0?pool_size=2&dial_timeout=3s",
			},
			expected: &goredis.Options{
				Network:     "tcp",
				Addr:        "127.0.0.1:6379",
				PoolSize:    2,
				DialTimeout: 3 * time.Second,
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, fn(tc))
	}
}

// TestCreateOptionsRejectsLegacyConfig asserts that configs still setting the
// removed connection keys fail with a migration hint.
func TestCreateOptionsRejectsLegacyConfig(t *testing.T) {
	t.Parallel()

	singleKeyMsg := `cache/redis: the 'address' key is no longer supported; replace it with uri = "redis://:<password>@<host>:<port>/<db>", e.g. before: address = "127.0.0.1:6379", password = "secret", db = 3, ssl = false. after: uri = "redis://:secret@127.0.0.1:6379/3" (schemes: redis://, rediss://, unix://; see cache/redis/README.md)`
	allKeysMsg := `cache/redis: the 'network', 'address', 'password', 'db', 'ssl' keys are no longer supported; replace them with uri = "redis://:<password>@<host>:<port>/<db>", e.g. before: address = "127.0.0.1:6379", password = "secret", db = 3, ssl = false. after: uri = "redis://:secret@127.0.0.1:6379/3" (schemes: redis://, rediss://, unix://; see cache/redis/README.md)`

	type tcase struct {
		config       dict.Dict
		expectedKeys []string
		// expectedMsg pins the exact error message when set
		expectedMsg string
	}

	tests := map[string]tcase{
		"network is rejected": {
			config:       map[string]any{"network": "tcp"},
			expectedKeys: []string{"network"},
		},
		"address is rejected": {
			config:       map[string]any{"address": "127.0.0.1:6379"},
			expectedKeys: []string{"address"},
			expectedMsg:  singleKeyMsg,
		},
		"password is rejected": {
			config:       map[string]any{"password": "secret"},
			expectedKeys: []string{"password"},
		},
		"db is rejected": {
			config:       map[string]any{"db": 0},
			expectedKeys: []string{"db"},
		},
		"ssl is rejected": {
			config:       map[string]any{"ssl": true},
			expectedKeys: []string{"ssl"},
		},
		"legacy keys are listed in canonical order": {
			config: map[string]any{
				"db":       0,
				"ssl":      true,
				"address":  "127.0.0.1:6379",
				"network":  "tcp",
				"password": "secret",
			},
			expectedKeys: []string{"network", "address", "password", "db", "ssl"},
			expectedMsg:  allKeysMsg,
		},
		"legacy key beside a valid uri is still rejected": {
			config: map[string]any{
				"uri":     testURI,
				"address": "127.0.0.1:6379",
			},
			expectedKeys: []string{"address"},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			_, err := redis.CreateOptions(tc.config)
			if err == nil {
				t.Fatal("expected err, got nil")
			}

			var unsupportedErr *redis.ErrUnsupportedConfigKeys
			if !errors.As(err, &unsupportedErr) {
				t.Fatalf("invalid error type. expected %T, got %T: %v", &redis.ErrUnsupportedConfigKeys{}, err, err)
			}
			if !reflect.DeepEqual(unsupportedErr.Keys, tc.expectedKeys) {
				t.Errorf("Keys: got %v, expected %v", unsupportedErr.Keys, tc.expectedKeys)
			}
			if tc.expectedMsg != "" && err.Error() != tc.expectedMsg {
				t.Errorf("message:\n got %q\nwant %q", err.Error(), tc.expectedMsg)
			}
		})
	}
}

// TestCreateOptionsRequiresURI asserts that a missing or empty uri fails
// instead of silently falling back to defaults.
func TestCreateOptionsRequiresURI(t *testing.T) {
	t.Parallel()

	expectedMsg := `cache/redis: the 'uri' config key is required and must not be empty, e.g. uri = "redis://127.0.0.1:6379/0" (schemes: redis://, rediss://, unix://; see cache/redis/README.md)`

	configs := map[string]dict.Dict{
		"empty config":   {},
		"empty uri":      {"uri": ""},
		"whitespace uri": {"uri": "   "},
	}

	for name, config := range configs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			_, err := redis.CreateOptions(config)
			var missingErr *redis.ErrURIMissing
			if !errors.As(err, &missingErr) {
				t.Fatalf("invalid error type. expected %T, got %T: %v", &redis.ErrURIMissing{}, err, err)
			}
			if err.Error() != expectedMsg {
				t.Errorf("message:\n got %q\nwant %q", err.Error(), expectedMsg)
			}
		})
	}

	t.Run("non-string uri is a type error", func(t *testing.T) {
		t.Parallel()

		_, err := redis.CreateOptions(dict.Dict{"uri": 1})
		var typeErr dict.ErrType
		if !errors.As(err, &typeErr) {
			t.Fatalf("invalid error type. expected %T, got %T: %v", dict.ErrType{}, err, err)
		}
		if typeErr.Key != "uri" {
			t.Errorf("Key: got %q, expected %q", typeErr.Key, "uri")
		}
	})
}

// TestCreateOptionsRejectsMalformedURI asserts that malformed URIs are
// rejected at startup with the parse cause in the error.
func TestCreateOptionsRejectsMalformedURI(t *testing.T) {
	t.Parallel()

	type tcase struct {
		uri string
		// expectedPart is contained in the resulting error message
		expectedPart string
	}

	tests := map[string]tcase{
		"unsupported scheme": {
			uri:          "http://127.0.0.1:6379",
			expectedPart: "redis: invalid URL scheme: http",
		},
		"invalid db index": {
			uri:          "redis://127.0.0.1:6379/notadb",
			expectedPart: `redis: invalid database number: "notadb"`,
		},
		"unix socket without path": {
			uri:          "unix://",
			expectedPart: "redis: empty unix socket path",
		},
		"unexpected query parameter": {
			uri:          "redis://127.0.0.1:6379/0?nope=1",
			expectedPart: "redis: unexpected option: nope",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			_, err := redis.CreateOptions(dict.Dict{"uri": tc.uri})
			var invalidErr *redis.ErrInvalidURI
			if !errors.As(err, &invalidErr) {
				t.Fatalf("invalid error type. expected %T, got %T: %v", &redis.ErrInvalidURI{}, err, err)
			}
			msg := err.Error()
			if !strings.HasPrefix(msg, "cache/redis: could not parse the 'uri' config key (") {
				t.Errorf("message %q misses the 'could not parse' prefix", msg)
			}
			if !strings.Contains(msg, tc.expectedPart) {
				t.Errorf("message %q does not contain %q", msg, tc.expectedPart)
			}
		})
	}

	t.Run("error never echoes credentials from the uri", func(t *testing.T) {
		t.Parallel()

		_, err := redis.CreateOptions(dict.Dict{"uri": "redis://:supersecret@127.0.0.1:6379/%zz"})
		var invalidErr *redis.ErrInvalidURI
		if !errors.As(err, &invalidErr) {
			t.Fatalf("invalid error type. expected %T, got %T: %v", &redis.ErrInvalidURI{}, err, err)
		}
		msg := err.Error()
		if strings.Contains(msg, "supersecret") {
			t.Errorf("message %q leaks the uri password", msg)
		}
		if !strings.Contains(msg, "invalid URL escape") {
			t.Errorf("message %q does not contain the parse cause", msg)
		}
	})
}

// TestNewConfigValidation asserts that configuration mistakes fail before the
// cache dials redis, so bad configs are reported at startup, not as
// connection errors.
func TestNewConfigValidation(t *testing.T) {
	t.Parallel()

	type tcase struct {
		config      dict.Dict
		expectedErr error
	}

	tests := map[string]tcase{
		"missing uri": {
			config:      map[string]any{},
			expectedErr: &redis.ErrURIMissing{},
		},
		"legacy config": {
			config:      map[string]any{"address": "127.0.0.1:6379"},
			expectedErr: &redis.ErrUnsupportedConfigKeys{Keys: []string{"address"}},
		},
		"bad config uri": {
			config: map[string]any{"uri": 1},
			expectedErr: dict.ErrType{
				Key:   "uri",
				Value: 1,
				T:     reflect.TypeOf(""),
			},
		},
		"bad config ttl": {
			config: map[string]any{"uri": testURI, "ttl": "fails"},
			expectedErr: dict.ErrType{
				Key:   "ttl",
				Value: "fails",
				T:     reflect.TypeOf(1),
			},
		},
		"bad max_zoom": {
			config: map[string]any{"uri": testURI, "max_zoom": "2"},
			expectedErr: dict.ErrType{
				Key:   "max_zoom",
				Value: "2",
				T:     reflect.TypeOf(uint(0)),
			},
		},
		"bad max_zoom 2": {
			config: map[string]any{"uri": testURI, "max_zoom": -2},
			expectedErr: dict.ErrType{
				Key:   "max_zoom",
				Value: -2,
				T:     reflect.TypeOf(uint(0)),
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			_, err := redis.New(tc.config)
			if err == nil {
				t.Fatalf("expected err %v, got nil", tc.expectedErr)
			}
			if !reflect.DeepEqual(err, tc.expectedErr) {
				t.Errorf("invalid error. expected %v, got %v", tc.expectedErr, err)
			}
		})
	}
}

// TestNoLegacyFallbackInSource is a grep-level guarantee that none of the
// removed legacy connection keys have a code path left in cache/redis.
func TestNoLegacyFallbackInSource(t *testing.T) {
	t.Parallel()

	forbidden := []string{
		"ConfigKeyNetwork",
		"ConfigKeyAddress",
		"ConfigKeyPassword",
		"ConfigKeyDB",
		"ConfigKeySSL",
		"defaultNetwork",
		"defaultAddress",
		"defaultPassword",
		"defaultDB",
		"defaultSSL",
		"defaultURI",
		"ErrHostMissing",
		"SplitHostPort",
		"is deprecated",
	}

	for _, file := range []string{"redis.go", "errors.go"} {
		content, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("could not read %s: %v", file, err)
		}
		for _, marker := range forbidden {
			if strings.Contains(string(content), marker) {
				t.Errorf("%s still contains legacy fallback marker %q", file, marker)
			}
		}
	}
}

func compareOptions(t *testing.T, actual, expected *goredis.Options) {
	t.Helper()

	if actual.Network != expected.Network {
		t.Errorf("Network: got %q, want %q", actual.Network, expected.Network)
	}
	if actual.Addr != expected.Addr {
		t.Errorf("Addr: got %q, want %q", actual.Addr, expected.Addr)
	}
	if actual.Username != expected.Username {
		t.Errorf("Username: got %q, want %q", actual.Username, expected.Username)
	}
	if actual.Password != expected.Password {
		t.Errorf("Password: got %q, want %q", actual.Password, expected.Password)
	}
	if actual.DB != expected.DB {
		t.Errorf("DB: got %d, want %d", actual.DB, expected.DB)
	}
	if actual.TLSConfig == nil && expected.TLSConfig != nil {
		t.Errorf("got nil TLSConfig, expected a TLSConfig")
	}
	if actual.TLSConfig != nil && expected.TLSConfig == nil {
		t.Errorf("got TLSConfig, expected no TLSConfig")
	}
	if expected.PoolSize != 0 && actual.PoolSize != expected.PoolSize {
		t.Errorf("PoolSize: got %d, want %d", actual.PoolSize, expected.PoolSize)
	}
	if expected.DialTimeout != 0 && actual.DialTimeout != expected.DialTimeout {
		t.Errorf("DialTimeout: got %s, want %s", actual.DialTimeout, expected.DialTimeout)
	}
}

// TestNew will run tests against a live redis instance
// on 127.0.0.1:6379
func TestNew(t *testing.T) {
	ttools.ShouldSkip(t, TESTENV)

	type tcase struct {
		config      dict.Dict
		expectedErr error
	}

	fn := func(tc tcase) func(*testing.T) {
		return func(t *testing.T) {
			t.Parallel()

			_, err := redis.New(tc.config)
			if tc.expectedErr != nil {
				if err == nil {
					t.Errorf("expected err %v, got nil", tc.expectedErr.Error())
					return
				}

				// check error types
				if reflect.TypeOf(err) != reflect.TypeOf(tc.expectedErr) {
					t.Errorf("invalid error type. expected %T, got %T", tc.expectedErr, err)
					return
				}

				switch e := err.(type) {
				case *net.OpError:
					expectedErr := tc.expectedErr.(*net.OpError)

					if reflect.TypeOf(e.Err) != reflect.TypeOf(expectedErr.Err) {
						t.Errorf("invalid error type. expected %T, got %T", expectedErr.Err, e.Err)
						return
					}
				default:
					// check error messages
					if err.Error() != tc.expectedErr.Error() {
						t.Errorf("invalid error. expected %v, got %v", tc.expectedErr, err.Error())
						return
					}
				}

				return
			}
			if err != nil {
				t.Errorf("unexpected err: %v", err)
				return
			}
		}
	}

	tests := map[string]tcase{
		"explicit config with uri": {
			config: map[string]any{
				"uri":      testURI,
				"max_zoom": uint(10),
			},
		},
		"bad address": {
			config: map[string]any{
				"uri": "redis://127.0.0.1:6000/0",
			},
			expectedErr: &net.OpError{
				Op:  "dial",
				Net: "tcp",
				Addr: &net.TCPAddr{
					IP:   net.ParseIP("127.0.0.1"),
					Port: 6000,
				},
				Err: &os.SyscallError{
					Err: syscall.ECONNREFUSED,
				},
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, fn(tc))
	}
}

func TestSetGetPurge(t *testing.T) {
	ttools.ShouldSkip(t, TESTENV)

	ctx := context.Background()
	type tcase struct {
		config       dict.Dict
		key          cache.Key
		expectedData []byte
		expectedHit  bool
	}

	fn := func(tc tcase) func(*testing.T) {
		return func(t *testing.T) {
			rc, err := redis.New(tc.config)
			if err != nil {
				t.Errorf("unexpected err, expected %v got %v", nil, err)
				return
			}

			// test write
			if tc.expectedHit {
				err = rc.Set(ctx, &tc.key, tc.expectedData)
				if err != nil {
					t.Errorf("unexpected err, expected %v got %v", nil, err)
				}
				return
			}

			// test read
			output, hit, err := rc.Get(ctx, &tc.key)
			if err != nil {
				t.Errorf("read failed with error, expected %v got %v", nil, err)
				return
			}
			if tc.expectedHit != hit {
				t.Errorf("read failed, wrong 'hit' value expected %t got %t", tc.expectedHit, hit)
				return
			}

			if !reflect.DeepEqual(output, tc.expectedData) {
				t.Errorf("read failed, expected %v got %v", output, tc.expectedData)
				return
			}

			// test purge
			if tc.expectedHit {
				err = rc.Purge(ctx, &tc.key)
				if err != nil {
					t.Errorf("purge failed with err, expected %v got %v", nil, err)
					return
				}
			}
		}
	}

	testcases := map[string]tcase{
		"redis cache hit": {
			config: map[string]any{"uri": testURI},
			key: cache.Key{
				Z: 0,
				X: 1,
				Y: 2,
			},
			expectedData: []byte("\x53\x69\x6c\x61\x73"),
			expectedHit:  true,
		},
		"redis cache miss": {
			config: map[string]any{"uri": testURI},
			key: cache.Key{
				Z: 0,
				X: 0,
				Y: 0,
			},
			expectedData: []byte(nil),
			expectedHit:  false,
		},
	}

	for name, tc := range testcases {
		t.Run(name, fn(tc))
	}

}

func TestSetOverwrite(t *testing.T) {
	ttools.ShouldSkip(t, TESTENV)

	ctx := context.Background()
	type tcase struct {
		config   dict.Dict
		key      cache.Key
		bytes1   []byte
		bytes2   []byte
		expected []byte
	}

	fn := func(tc tcase) func(*testing.T) {
		return func(t *testing.T) {
			rc, err := redis.New(tc.config)
			if err != nil {
				t.Errorf("unexpected err, expected %v got %v", nil, err)
				return
			}

			// test write1
			if err = rc.Set(ctx, &tc.key, tc.bytes1); err != nil {
				t.Errorf("write failed with err, expected %v got %v", nil, err)
				return
			}

			// test write2
			if err = rc.Set(ctx, &tc.key, tc.bytes2); err != nil {
				t.Errorf("write failed with err, expected %v got %v", nil, err)
				return
			}

			// fetch the cache entry
			output, hit, err := rc.Get(ctx, &tc.key)
			if err != nil {
				t.Errorf("read failed with err, expected %v got %v", nil, err)
				return
			}
			if !hit {
				t.Errorf("read failed, expected hit %t got %t", true, hit)
				return
			}

			if !reflect.DeepEqual(output, tc.expected) {
				t.Errorf("read failed, expected %v got %v)", output, tc.expected)
				return
			}

			// clean up
			if err = rc.Purge(ctx, &tc.key); err != nil {
				t.Errorf("purge failed with err, expected %v got %v", nil, err)
				return
			}
		}
	}

	testcases := map[string]tcase{
		"redis overwrite": {
			config: map[string]any{"uri": testURI},
			key: cache.Key{
				Z: 0,
				X: 1,
				Y: 1,
			},
			bytes1:   []byte("\x66\x6f\x6f"),
			bytes2:   []byte("\x53\x69\x6c\x61\x73"),
			expected: []byte("\x53\x69\x6c\x61\x73"),
		},
	}

	for name, tc := range testcases {
		t.Run(name, fn(tc))
	}
}

func TestMaxZoom(t *testing.T) {
	ttools.ShouldSkip(t, TESTENV)
	ctx := context.Background()

	type tcase struct {
		config      dict.Dict
		key         cache.Key
		bytes       []byte
		expectedHit bool
	}

	fn := func(tc tcase) func(*testing.T) {
		return func(t *testing.T) {
			t.Parallel()

			rc, err := redis.New(tc.config)
			if err != nil {
				t.Fatalf("unexpected err, expected %v got %v", nil, err)
			}

			// test write
			if tc.expectedHit {
				err = rc.Set(ctx, &tc.key, tc.bytes)
				if err != nil {
					t.Fatalf("unexpected err, expected %v got %v", nil, err)
				}
			}

			// test read
			_, hit, err := rc.Get(ctx, &tc.key)
			if err != nil {
				t.Fatalf("read failed with error, expected %v got %v", nil, err)
			}
			if tc.expectedHit != hit {
				t.Fatalf("read failed, wrong 'hit' value expected %t got %t", tc.expectedHit, hit)
			}

			// test purge
			if tc.expectedHit {
				err = rc.Purge(ctx, &tc.key)
				if err != nil {
					t.Fatalf("purge failed with err, expected %v got %v", nil, err)
				}
			}
		}
	}

	tests := map[string]tcase{
		"over max zoom": {
			config: map[string]any{
				"uri":      testURI,
				"max_zoom": uint(10),
			},
			key: cache.Key{
				Z: 11,
				X: 1,
				Y: 1,
			},
			bytes:       []byte("\x41\x64\x61"),
			expectedHit: false,
		},
		"under max zoom": {
			config: map[string]any{
				"uri":      testURI,
				"max_zoom": uint(10),
			},
			key: cache.Key{
				Z: 9,
				X: 1,
				Y: 1,
			},
			bytes:       []byte("\x41\x64\x61"),
			expectedHit: true,
		},
		"equals max zoom": {
			config: map[string]any{
				"uri":      testURI,
				"max_zoom": uint(10),
			},
			key: cache.Key{
				Z: 10,
				X: 1,
				Y: 1,
			},
			bytes:       []byte("\x41\x64\x61"),
			expectedHit: true,
		},
	}

	for name, tc := range tests {
		t.Run(name, fn(tc))
	}
}
