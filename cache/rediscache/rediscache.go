package rediscache

import (
	"time"

	"github.com/go-redis/redis"
	"github.com/terranodo/tegola/cache"
	"github.com/terranodo/tegola/util/dict"
	"fmt"
)

const CacheType = "redis"

const (
	ConfigKeyNetwork  = "network"
	ConfigKeyAddress  = "address"
	ConfigKeyPassword = "password"
	ConfigKeyDB       = "db"
	ConfigKeyMaxZoom  = "max_zoom"
)

func init() {
	cache.Register(CacheType, New)
}

func New(config map[string]interface{}) (rcache cache.Interface, err error) {

	// default values
	defaultNetwork := "tcp"
	defaultAddress := "127.0.0.1:6379"
	defaultPassword := ""
	defaultDB := 0
	defaultMaxZoom := 0

	c := dict.M(config)

	network, err := c.String(ConfigKeyNetwork, &defaultNetwork)
	if err != nil {
		return nil, err
	}

	addr, err := c.String(ConfigKeyAddress, &defaultAddress)
	if err != nil {
		return nil, err
	}

	password, err := c.String(ConfigKeyPassword, &defaultPassword)
	if err != nil {
		return nil, err
	}

	db, err := c.Int(ConfigKeyDB, &defaultDB)
	if err != nil {
		return nil, err
	}

	client := redis.NewClient(&redis.Options{
		Network:     network,
		Addr:        addr,
		Password:    password,
		DB:          db,
		PoolSize:    2,
		DialTimeout: 3 * time.Second,
	})

	pong, err := client.Ping().Result()
	if err != nil {
		return nil, err
	}
	if pong != "PONG" {
		return nil, fmt.Errorf("redis did not resoind with 'PONG', '%s'", pong)
	}

	maxZoom, err := c.Int(ConfigKeyMaxZoom, &defaultMaxZoom)
	if err != nil {
		return nil, err
	}

	return &RedisCache{
		Redis:   client,
		MaxZoom: maxZoom,
	}, nil
}

type RedisCache struct {
	Redis      *redis.Client
	Expiration time.Duration
	MaxZoom    int
}

func (rdc *RedisCache) Set(key *cache.Key, val []byte) (error) {
	if key.Z > rdc.MaxZoom {
		return nil
	}

	return rdc.Redis.
		Set(key.String(), val, rdc.Expiration).
		Err()
}

func (rdc *RedisCache) Get(key *cache.Key) (val []byte, hit bool, err error) {
	val, err = rdc.Redis.Get(key.String()).Bytes()

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
	return rdc.Redis.Del(key.String()).Err()
}
