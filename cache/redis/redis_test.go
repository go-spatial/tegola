package redis_test

import (
	"errors"
	"net"
	"reflect"
	"testing"

	"github.com/go-spatial/tegola/cache"
	"github.com/go-spatial/tegola/cache/redis"
	"github.com/go-spatial/tegola/internal/dict"
	"github.com/go-spatial/tegola/internal/ttools"
)

// TESTENV is the environment variable that must be set to "yes" to run the redis tests.
const TESTENV = "RUN_REDIS_TESTS"

// TestNew will run tests against a local redis instance
// on 127.0.0.1:6379
func TestNew(t *testing.T) {
	ttools.ShouldSkip(t, TESTENV)

	type tcase struct {
		config      dict.Dict
		expectedErr error
	}

	fn := func(t *testing.T, tc tcase) {
		t.Parallel()

		_, err := redis.New(tc.config)
		if tc.expectedErr != nil {
			if err == nil {
				t.Errorf("expected err %v, got nil", tc.expectedErr.Error())
				return
			}
			if err.Error() != tc.expectedErr.Error() {
				//log.Println("typeof ", reflect.TypeOf(err))
				t.Errorf("invalid error. expected: %v, got %v", tc.expectedErr, err.Error())
			}
			return
		}
		if err != nil {
			t.Errorf("unexpected err: %v", err)
			return
		}
	}

	tests := map[string]tcase{
		"explicit config": {
			config: map[string]interface{}{
				"network":  "tcp",
				"address":  "127.0.0.1:6379",
				"password": "",
				"db":       0,
				"max_zoom": uint(10),
			},
		},
		"implicit config": {
			config: map[string]interface{}{},
		},
		"bad address": {
			config: map[string]interface{}{
				"address": "127.0.0.1:6000",
			},
			expectedErr: &net.OpError{
				Op:  "dial",
				Net: "tcp",
				Addr: &net.TCPAddr{
					IP:   net.ParseIP("127.0.0.1"),
					Port: 6000,
				},
				Err: errors.New("getsockopt: connection refused"),
			},
		},
		"bad max_zoom": {
			config: map[string]interface{}{
				"max_zoom": "2",
			},
			expectedErr: dict.ErrKeyType{
				Key:   "max_zoom",
				Value: "2",
				T:     reflect.TypeOf(uint(0)),
			},
		},
		"bad max_zoom 2": {
			config: map[string]interface{}{
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
		tc := tc
		t.Run(name, func(t *testing.T) {
			fn(t, tc)
		})
	}
}

func TestSetGetPurge(t *testing.T) {
	ttools.ShouldSkip(t, TESTENV)

	type tc struct {
		config       dict.Dict
		key          cache.Key
		expectedData []byte
		expectedHit  bool
	}

	testcases := map[string]tc{
		"redis cache hit": {
			config: map[string]interface{}{},
			key: cache.Key{
				Z: 0,
				X: 1,
				Y: 2,
			},
			expectedData: []byte("\x53\x69\x6c\x61\x73"),
			expectedHit:  true,
		},
		"redis cache miss": {
			config: map[string]interface{}{},
			key: cache.Key{
				Z: 0,
				X: 0,
				Y: 0,
			},
			expectedData: []byte(nil),
			expectedHit:  false,
		},
	}

	for k, tc := range testcases {
		rc, err := redis.New(tc.config)
		if err != nil {
			t.Errorf("[%v] unexpected err, expected %v got %v", k, nil, err)
			continue
		}

		// test write
		if tc.expectedHit {
			err = rc.Set(&tc.key, tc.expectedData)
			if err != nil {
				t.Errorf("[%v] unexpected err, expected %v got %v", k, nil, err)
			}
			continue
		}

		// test read
		output, hit, err := rc.Get(&tc.key)
		if err != nil {
			t.Errorf("[%v] read failed with error, expected %v got %v", k, nil, err)
			continue
		}
		if tc.expectedHit != hit {
			t.Errorf("[%v] read failed, wrong 'hit' value expected %t got %t", k, tc.expectedHit, hit)
			continue
		}

		if !reflect.DeepEqual(output, tc.expectedData) {
			t.Errorf("[%v] read failed, expected %v got %v", k, output, tc.expectedData)
			continue
		}

		// test purge
		if tc.expectedHit {
			err = rc.Purge(&tc.key)
			if err != nil {
				t.Errorf("[%v] purge failed with err, expected %v got %v", k, nil, err)
				continue
			}
		}
	}
}

func TestSetOverwrite(t *testing.T) {
	ttools.ShouldSkip(t, TESTENV)
	type tc struct {
		config   dict.Dict
		key      cache.Key
		bytes1   []byte
		bytes2   []byte
		expected []byte
	}

	testcases := map[string]tc{
		"redis overwrite": {
			config: map[string]interface{}{},
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

	for k, tc := range testcases {
		rc, err := redis.New(tc.config)
		if err != nil {
			t.Errorf("[%v] unexpected err, expected %v got %v", k, nil, err)
			continue
		}

		// test write1
		if err = rc.Set(&tc.key, tc.bytes1); err != nil {
			t.Errorf("[%v] write failed with err, expected %v got %v", k, nil, err)
			continue
		}

		// test write2
		if err = rc.Set(&tc.key, tc.bytes2); err != nil {
			t.Errorf("[%v] write failed with err, expected %v got %v", k, nil, err)
			continue
		}

		// fetch the cache entry
		output, hit, err := rc.Get(&tc.key)
		if err != nil {
			t.Errorf("[%v] read failed with err, expected %v got %v", k, nil, err)
			continue
		}
		if !hit {
			t.Errorf("[%v] read failed, expected hit %t got %t", k, true, hit)
			continue
		}

		if !reflect.DeepEqual(output, tc.expected) {
			t.Errorf("[%v] read failed, expected %v got %v)", k, output, tc.expected)
			continue
		}

		// clean up
		if err = rc.Purge(&tc.key); err != nil {
			t.Errorf("[%v] purge failed with err, expected %v got %v", k, nil, err)
			continue
		}
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

	fn := func(t *testing.T, tc tcase) {
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

	tests := map[string]tcase{
		"over max zoom": {
			config: map[string]interface{}{
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
			config: map[string]interface{}{
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
			config: map[string]interface{}{
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
		tc := tc
		t.Run(name, func(t *testing.T) {
			fn(t, tc)
		})
	}
}
