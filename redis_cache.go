package tilenol

import (
	"fmt"
	"time"

	"github.com/go-redis/redis"
)

// RedisConfig is the YAML configuration for a RedisCache
type RedisConfig struct {
	// Host is the Redis cluster host name
	Host string `yaml:"host"`
	// Port is the Redis cluster port number
	Port int `yaml:"port"`
	// TTL is how long each cache entry should remain before refresh
	TTL time.Duration `yaml:"ttl"`
}

// RedisCache is a Redis-backed Cache implementation
type RedisCache struct {
	// Client is the backend Redis cluster client
	Client *redis.Client
	// TTL is how long each cache entry should remain before refresh
	TTL time.Duration
}

// NewRedisCache creates a new RedisCache given a RedisConfig
func NewRedisCache(config *RedisConfig) (Cache, error) {
	client := redis.NewClient(&redis.Options{
		Addr: fmt.Sprintf("%s:%d", config.Host, config.Port),
	})
	// TODO: Try to ping the server on boot?
	return &RedisCache{
		Client: client,
		TTL:    config.TTL,
	}, nil
}

// Exists tries to get the cached value for a key, returning whether or not the key
// exists
func (r *RedisCache) Exists(key string) bool {
	_, err := r.Client.Get(key).Bytes()
	if err == redis.Nil {
		return false
	} else if err != nil {
		// Log an error in case the connection to Redis fails, but recompute the response
		Logger.Errorf("Could not talk to Redis: %v", err)
		return false
	}
	return true
}

// Get retrieves the cached value for a given key
func (r *RedisCache) Get(key string) ([]byte, error) {
	val, err := r.Client.Get(key).Bytes()
	if err == redis.Nil {
		return nil, ErrNoValue
	} else if err != nil {
		return []byte{}, err
	}
	return val, nil
}

// Put attempts to store a new value into Redis at a given key
func (r *RedisCache) Put(key string, val []byte) error {
	err := r.Client.Set(key, val, r.TTL).Err()
	if err != nil {
		// Log an error in case the key can't be stored in Redis, but continue
		Logger.Errorf("Could not store key [%s] in Redis: %v", key, err)
		return err
	}
	return nil
}
