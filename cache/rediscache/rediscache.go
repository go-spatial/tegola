package rediscache

import (
	"time"

	"github.com/go-redis/redis"
	"github.com/terranodo/tegola/cache"
	"github.com/terranodo/tegola/util/dict"
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
		return
	}

	addr, err := c.String(ConfigKeyAddress, &defaultAddress)
	if err != nil {
		return
	}

	password, err := c.String(ConfigKeyPassword, &defaultPassword)
	if err != nil {
		return
	}

	db, err := c.Int(ConfigKeyDB, &defaultDB)
	if err != nil {
		return
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
	if err != nil || pong != "PONG" {
		return
	}

	maxZoom, err := c.Int(ConfigKeyMaxZoom, &defaultMaxZoom)
	if err != nil {
		return
	}

	rcache = &RedisCache{
		Redis:   client,
		MaxZoom: maxZoom,
	}

	return
}

type RedisCache struct {
	Redis      *redis.Client
	Expiration time.Duration
	MaxZoom    int
}

func (rdc *RedisCache) Set(key *cache.Key, val []byte) (err error) {
	if key.Z > rdc.MaxZoom {
		return
	}

	return rdc.Redis.
		Set(key.String(), val, rdc.Expiration).
		Err()
}

func (rdc *RedisCache) Get(key *cache.Key) (val []byte, hit bool, err error) {
	val, err = rdc.Redis.Get(key.String()).Bytes()

	if err == nil { // cache hit
		hit = true
	} else if err == redis.Nil { // cache miss
		err = nil // clear error
	}

	return
}

func (rdc *RedisCache) Purge(key *cache.Key) (err error) {
	err = rdc.Redis.Del(key.String()).Err()
	return
}
