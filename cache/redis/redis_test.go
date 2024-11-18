package redis_test

import (
	"crypto/tls"
	"net"
	"os"
	"reflect"
	"syscall"
	"testing"

	goredis "github.com/go-redis/redis"
	"github.com/go-spatial/tegola/cache"
	"github.com/go-spatial/tegola/cache/redis"
	"github.com/go-spatial/tegola/dict"
	"github.com/go-spatial/tegola/internal/ttools"
)

// TESTENV is the environment variable that must be set to "yes" to run the redis tests.
const TESTENV = "RUN_REDIS_TESTS"

func TestCreateOptions(t *testing.T) {
	ttools.ShouldSkip(t, TESTENV)

	type tcase struct {
		name        string
		config      dict.Dict
		expected    *goredis.Options
		expectedErr error
	}

	fn := func(tc tcase) func(*testing.T) {
		return func(t *testing.T) {
			t.Parallel()

			actual, err := redis.CreateOptions(tc.config)
			if tc.expectedErr == nil && err != nil {
				t.Fatalf("unexpected error: %q", err)
				return
			}
			if tc.expectedErr != nil && err != nil {
				if reflect.TypeOf(err) != reflect.TypeOf(tc.expectedErr) {
					t.Errorf("invalid error type. expected %T, got %T", tc.expectedErr, err)
					return
				}
				return
			}
			compareOptions(t, actual, tc.expected)
		}
	}

	tests := map[string]tcase{
		"test complete config": {
			config: map[string]any{
				"network":  "tcp",
				"address":  "127.0.0.1:6379",
				"password": "test",
				"db":       0,
				"max_zoom": uint(10),
				"ssl":      false,
			},
			expected: &goredis.Options{
				Network:  "tcp",
				DB:       0,
				Addr:     "127.0.0.1:6379",
				Password: "test",
			},
		},
		"test with uri no ssl": {
			config: map[string]any{
				"uri": "redis://user:test@127.0.0.1:6379/0",
			},
			expected: &goredis.Options{
				Network:  "tcp",
				DB:       0,
				Addr:     "127.0.0.1:6379",
				Password: "test",
			},
		},
		"test with uri with ssl": {
			config: map[string]any{
				"uri": "rediss://user:test@127.0.0.1:6379/0",
			},
			expected: &goredis.Options{
				Network:   "tcp",
				DB:        0,
				Addr:      "127.0.0.1:6379",
				Password:  "test",
				TLSConfig: &tls.Config{ /* no deep comparison */ },
			},
		},
		"test empty config": {
			config: map[string]any{},
			expected: &goredis.Options{
				Network:  "tcp",
				DB:       0,
				Addr:     "127.0.0.1:6379",
				Password: "",
			},
		},
		"test ssl config": {
			name: "test test ssl config",
			config: map[string]any{
				"network":  "tcp",
				"address":  "127.0.0.1:6379",
				"password": "test",
				"db":       0,
				"max_zoom": uint(10),
				"ssl":      true,
			},
			expected: &goredis.Options{
				Network:   "tcp",
				DB:        0,
				Addr:      "127.0.0.1:6379",
				Password:  "test",
				TLSConfig: &tls.Config{ /* no deep comparison */ },
			},
		},
		"test bad address": {
			name: "test test ssl config",
			config: map[string]any{
				"network":  "tcp",
				"address":  2,
				"password": "test",
				"db":       0,
			},
			expectedErr: dict.ErrKeyType{
				Key:   "addr",
				Value: 2,
				T:     reflect.TypeOf(""),
			},
		},
		"test bad host": {
			name: "test test ssl config",
			config: map[string]any{
				"network": "tcp",
				"address": "::8080",
				"db":      0,
			},
			expectedErr: &net.AddrError{ /* no deep comparison */ },
		},
		"test missing host": {
			name: "test test ssl config",
			config: map[string]any{
				"network": "tcp",
				"address": ":8080",
				"db":      0,
			},
			expectedErr: &redis.ErrHostMissing{},
		},
		"test missing port": {
			name: "test test ssl config",
			config: map[string]any{
				"network": "tcp",
				"address": "localhost",
				"db":      0,
			},
			expectedErr: &net.AddrError{ /* no deep comparison */ },
		},
		"test bad db": {
			name: "test test ssl config",
			config: map[string]any{
				"network": "tcp",
				"address": "127.0.0.1:6379",
				"db":      "fails",
			},
			expectedErr: dict.ErrKeyType{
				Key:   "db",
				Value: "fails",
				T:     reflect.TypeOf(1),
			},
		},
		"test bad password": {
			name: "test test ssl config",
			config: map[string]any{
				"network":  "tcp",
				"address":  "127.0.0.1:6379",
				"password": 0,
			},
			expectedErr: dict.ErrKeyType{
				Key:   "password",
				Value: 0,
				T:     reflect.TypeOf(""),
			},
		},
		"test bad network": {
			name: "test test ssl config",
			config: map[string]any{
				"network": 0,
				"address": "127.0.0.1:6379",
			},
			expectedErr: dict.ErrKeyType{
				Key:   "network",
				Value: 0,
				T:     reflect.TypeOf(1),
			},
		},
		"test bad ssl": {
			name: "test test ssl config",
			config: map[string]any{
				"network": "tcp",
				"address": "127.0.0.1:6379",
				"ssl":     0,
			},
			expectedErr: dict.ErrKeyType{
				Key:   "ssl",
				Value: 0,
				T:     reflect.TypeOf(true),
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, fn(tc))
	}
}

func compareOptions(t *testing.T, actual, expected *goredis.Options) {
	t.Helper()

	if actual.Addr != expected.Addr {
		t.Errorf("got %q, want %q", actual.Addr, expected.Addr)
	}
	if actual.DB != expected.DB {
		t.Errorf("DB: got %q, expected %q", actual.DB, expected.DB)
	}
	if actual.TLSConfig == nil && expected.TLSConfig != nil {
		t.Errorf("got nil TLSConfig, expected a TLSConfig")
	}
	if actual.TLSConfig != nil && expected.TLSConfig == nil {
		t.Errorf("got TLSConfig, expected no TLSConfig")
	}
	if actual.Password != expected.Password {
		t.Errorf("Password: got %q, expected %q", actual.Password, expected.Password)
	}
}

// TestNew will run tests against a local redis instance
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
		"explicit config": {
			config: map[string]any{
				"network":  "tcp",
				"address":  "127.0.0.1:6379",
				"password": "",
				"db":       0,
				"max_zoom": uint(10),
				"ssl":      false,
			},
		},
		"explicit config with uri": {
			config: map[string]any{
				"uri": "redis://127.0.0.1:6379/0",
			},
		},
		"implicit config": {
			config: map[string]any{},
		},
		"bad config address": {
			config: map[string]any{"address": 0},
			expectedErr: dict.ErrKeyType{
				Key:   "address",
				Value: 0,
				T:     reflect.TypeOf(""),
			},
		},
		"bad config uri": {
			config: map[string]any{"uri": 1},
			expectedErr: dict.ErrKeyType{
				Key:   "uri",
				Value: 1,
				T:     reflect.TypeOf(""),
			},
		},
		"bad config ttl": {
			config: map[string]any{"ttl": "fails"},
			expectedErr: dict.ErrKeyType{
				Key:   "ttl",
				Value: "fails",
				T:     reflect.TypeOf(1),
			},
		},
		"bad address": {
			config: map[string]any{
				"address": "127.0.0.1:6000",
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
		"bad max_zoom": {
			config: map[string]any{
				"max_zoom": "2",
			},
			expectedErr: dict.ErrKeyType{
				Key:   "max_zoom",
				Value: "2",
				T:     reflect.TypeOf(uint(0)),
			},
		},
		"bad max_zoom 2": {
			config: map[string]any{
				"max_zoom": -2,
			},
			expectedErr: dict.ErrKeyType{
				Key:   "max_zoom",
				Value: -2,
				T:     reflect.TypeOf(uint(0)),
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, fn(tc))
	}
}

func TestSetGetPurge(t *testing.T) {
	ttools.ShouldSkip(t, TESTENV)

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
				err = rc.Set(&tc.key, tc.expectedData)
				if err != nil {
					t.Errorf("unexpected err, expected %v got %v", nil, err)
				}
				return
			}

			// test read
			output, hit, err := rc.Get(&tc.key)
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
				err = rc.Purge(&tc.key)
				if err != nil {
					t.Errorf("purge failed with err, expected %v got %v", nil, err)
					return
				}
			}
		}
	}

	testcases := map[string]tcase{
		"redis cache hit": {
			config: map[string]any{},
			key: cache.Key{
				Z: 0,
				X: 1,
				Y: 2,
			},
			expectedData: []byte("\x53\x69\x6c\x61\x73"),
			expectedHit:  true,
		},
		"redis cache miss": {
			config: map[string]any{},
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
			if err = rc.Set(&tc.key, tc.bytes1); err != nil {
				t.Errorf("write failed with err, expected %v got %v", nil, err)
				return
			}

			// test write2
			if err = rc.Set(&tc.key, tc.bytes2); err != nil {
				t.Errorf("write failed with err, expected %v got %v", nil, err)
				return
			}

			// fetch the cache entry
			output, hit, err := rc.Get(&tc.key)
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
			if err = rc.Purge(&tc.key); err != nil {
				t.Errorf("purge failed with err, expected %v got %v", nil, err)
				return
			}
		}
	}

	testcases := map[string]tcase{
		"redis overwrite": {
			config: map[string]any{},
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
				err = rc.Set(&tc.key, tc.bytes)
				if err != nil {
					t.Fatalf("unexpected err, expected %v got %v", nil, err)
				}
			}

			// test read
			_, hit, err := rc.Get(&tc.key)
			if err != nil {
				t.Fatalf("read failed with error, expected %v got %v", nil, err)
			}
			if tc.expectedHit != hit {
				t.Fatalf("read failed, wrong 'hit' value expected %t got %t", tc.expectedHit, hit)
			}

			// test purge
			if tc.expectedHit {
				err = rc.Purge(&tc.key)
				if err != nil {
					t.Fatalf("purge failed with err, expected %v got %v", nil, err)
				}
			}
		}
	}

	tests := map[string]tcase{
		"over max zoom": {
			config: map[string]any{
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
