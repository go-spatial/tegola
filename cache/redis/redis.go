package redis

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/alexeydott/tegola"
	"github.com/alexeydott/tegola/cache"
	"github.com/alexeydott/tegola/dict"
)

const CacheType = "redis"

const (
	ConfigKeyURI     = "uri"
	ConfigKeyMaxZoom = "max_zoom"
	ConfigKeyTTL     = "ttl"
)

// legacyConfigKeys are the connection configuration keys that ConfigKeyURI
// replaced. A config that still sets any of them is rejected at startup with a
// migration hint (see ErrUnsupportedConfigKeys).
var legacyConfigKeys = []string{"network", "address", "password", "db", "ssl"}

var (
	// default values
	defaultMaxZoom = uint(tegola.MaxZ)
	defaultTTL     = 0
)

func init() {
	_ = cache.Register(CacheType, New)
}

// CreateOptions creates redis.Options from the cache config. All connection
// settings are read from the 'uri' key and parsed with redis.ParseURL
// (schemes redis://, rediss:// and unix://). Configs that still set one of the
// legacy connection keys, or that carry no parseable uri, fail here so that
// tegola refuses to start instead of silently dialing defaults.
func CreateOptions(c dict.Dicter) (*redis.Options, error) {
	if keys := legacyConfigKeysSet(c); len(keys) > 0 {
		return nil, &ErrUnsupportedConfigKeys{Keys: keys}
	}

	uri, err := c.String(ConfigKeyURI, nil)
	if err != nil {
		// a non-string uri value is a type error on ConfigKeyURI, anything
		// else (e.g. a missing key) means there is no uri to connect to
		var requiredErr dict.ErrKeyRequired
		if errors.As(err, &requiredErr) {
			return nil, &ErrURIMissing{}
		}
		return nil, err
	}

	if strings.TrimSpace(uri) == "" {
		return nil, &ErrURIMissing{}
	}

	opts, err := redis.ParseURL(uri)
	if err != nil {
		// url.Parse errors echo the raw URL, which may contain credentials;
		// unwrap to the cause so the startup error stays safe to log
		var urlErr *url.Error
		if errors.As(err, &urlErr) {
			err = urlErr.Err
		}
		return nil, &ErrInvalidURI{Err: err}
	}

	return opts, nil
}

// legacyConfigKeysSet returns the removed legacy connection keys that are
// still set in the config, in legacyConfigKeys order.
func legacyConfigKeysSet(c dict.Dicter) (keys []string) {
	for _, key := range legacyConfigKeys {
		if _, ok := c.Interface(key); ok {
			keys = append(keys, key)
		}
	}

	return keys
}

func New(c dict.Dicter) (rcache cache.Interface, err error) {
	// validate the whole config before dialing so configuration mistakes fail
	// at startup with a config error rather than a connection error
	opts, err := CreateOptions(c)
	if err != nil {
		return nil, err
	}

	// the c map's underlying value is int
	maxZoom, err := c.Uint(ConfigKeyMaxZoom, &defaultMaxZoom)
	if err != nil {
		return nil, err
	}

	ttl, err := c.Int(ConfigKeyTTL, &defaultTTL)
	if err != nil {
		return nil, err
	}

	ctx := context.Background()
	client := redis.NewClient(opts)

	pong, err := client.Ping(ctx).Result()
	if err != nil {
		return nil, err
	}
	if pong != "PONG" {
		return nil, fmt.Errorf("redis did not respond with 'PONG', '%s'", pong)
	}

	return &RedisCache{
		Redis:      client,
		MaxZoom:    maxZoom,
		Expiration: time.Duration(ttl) * time.Second,
	}, nil
}

type RedisCache struct {
	Redis      *redis.Client
	Expiration time.Duration
	MaxZoom    uint
}

func (rdc *RedisCache) Set(ctx context.Context, key *cache.Key, val []byte) error {
	if key.Z > rdc.MaxZoom {
		return nil
	}

	return rdc.Redis.
		Set(ctx, key.String(), val, rdc.Expiration).
		Err()
}

func (rdc *RedisCache) Get(ctx context.Context, key *cache.Key) (val []byte, hit bool, err error) {
	val, err = rdc.Redis.Get(ctx, key.String()).Bytes()

	switch err {
	case nil: // cache hit
		return val, true, nil
	case redis.Nil: // cache miss
		return val, false, nil
	default: // error
		return val, false, err
	}
}

func (rdc *RedisCache) Purge(ctx context.Context, key *cache.Key) (err error) {
	return rdc.Redis.Del(ctx, key.String()).Err()
}
