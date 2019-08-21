package tilenol

import (
	"fmt"
	"time"

	"github.com/go-redis/redis"
)

type RedisConfig struct {
	Host string        `yaml:"host"`
	Port int           `yaml:"port"`
	TTL  time.Duration `yaml:"ttl"`
}

type RedisCache struct {
	Client *redis.Client
	TTL    time.Duration
}

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

func (r *RedisCache) Get(key string) ([]byte, error) {
	val, err := r.Client.Get(key).Bytes()
	if err != nil {
		return []byte{}, err
	}
	return val, nil
}

func (r *RedisCache) Put(key string, val []byte) error {
	err := r.Client.Set(key, val, r.TTL).Err()
	if err != nil {
		// Log an error in case the key can't be stored in Redis, but continue
		Logger.Errorf("Could not store key [%s] in Redis: %v", key, err)
		return err
	}
	return nil
}
