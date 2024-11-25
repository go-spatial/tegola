package redis

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/go-spatial/tegola"
	"github.com/go-spatial/tegola/cache"
	"github.com/go-spatial/tegola/dict"
	"github.com/go-spatial/tegola/internal/log"
)

const CacheType = "redis"

const (
	ConfigKeyNetwork  = "network"
	ConfigKeyAddress  = "address"
	ConfigKeyPassword = "password"
	ConfigKeyDB       = "db"
	ConfigKeyMaxZoom  = "max_zoom"
	ConfigKeyTTL      = "ttl"
	ConfigKeySSL      = "ssl"
	ConfigKeyURI      = "uri"
)

var (
	// default values
	defaultNetwork  = "tcp"
	defaultAddress  = "127.0.0.1:6379"
	defaultPassword = ""
	defaultURI      = ""
	defaultDB       = 0
	defaultMaxZoom  = uint(tegola.MaxZ)
	defaultTTL      = 0
	defaultSSL      = false
)

func init() {
	cache.Register(CacheType, New)
}

// TODO @iwpnd: deprecate connection with Addr
// CreateOptions creates redis.Options from an implicit or explicit c
func CreateOptions(c dict.Dicter) (opts *redis.Options, err error) {
	uri, err := c.String(ConfigKeyURI, &defaultURI)
	if err != nil {
		return nil, err
	}

	if uri != "" {
		opts, err := redis.ParseURL(uri)
		if err != nil {
			return nil, err
		}

		return opts, nil
	}

	log.Warn("connecting to redis using 'Addr' is deprecated. use 'uri' instead.")

	network, err := c.String(ConfigKeyNetwork, &defaultNetwork)
	if err != nil {
		return nil, err
	}

	addr, err := c.String(ConfigKeyAddress, &defaultAddress)
	if err != nil {
		return nil, err
	}

	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, err
	}

	if host == "" {
		return nil, &ErrHostMissing{msg: fmt.Sprintf("no host provided in '%s'", addr)}
	}

	password, err := c.String(ConfigKeyPassword, &defaultPassword)
	if err != nil {
		return nil, err
	}

	db, err := c.Int(ConfigKeyDB, &defaultDB)
	if err != nil {
		return nil, err
	}

	o := &redis.Options{
		Network:     network,
		Addr:        addr,
		Password:    password,
		DB:          db,
		PoolSize:    2,
		DialTimeout: 3 * time.Second,
	}

	ssl, err := c.Bool(ConfigKeySSL, &defaultSSL)
	if err != nil {
		return nil, err
	}

	if ssl {
		o.TLSConfig = &tls.Config{ServerName: host}
	}

	return o, nil
}

func New(c dict.Dicter) (rcache cache.Interface, err error) {
	ctx := context.Background()
	opts, err := CreateOptions(c)
	if err != nil {
		return nil, err
	}

	client := redis.NewClient(opts)

	pong, err := client.Ping(ctx).Result()
	if err != nil {
		return nil, err
	}
	if pong != "PONG" {
		return nil, fmt.Errorf("redis did not respond with 'PONG', '%s'", pong)
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

func (rdc *RedisCache) Set(key *cache.Key, val []byte) error {
	ctx := context.Background()
	if key.Z > rdc.MaxZoom {
		return nil
	}

	return rdc.Redis.
		Set(ctx, key.String(), val, rdc.Expiration).
		Err()
}

func (rdc *RedisCache) Get(key *cache.Key) (val []byte, hit bool, err error) {
	ctx := context.Background()
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

func (rdc *RedisCache) Purge(key *cache.Key) (err error) {
	ctx := context.Background()
	return rdc.Redis.Del(ctx, key.String()).Err()
}
