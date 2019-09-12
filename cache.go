package tilenol

import (
	"errors"
)

var (
	// ErrNoValue occurs when trying to access a value that doesn't exist in the cache
	ErrNoValue = errors.New("No value exists in cache")
)

// CacheConfig is a generic YAML cache configuration object
type CacheConfig struct {
	// Redis is an optional YAML key for configuring a RedisCache
	Redis *RedisConfig `yaml:"redis"`
}

// Cache is a generic interface for a tile server cache
type Cache interface {
	// Exists determines whether or not there is a cache value for the given key
	Exists(key string) bool
	// Get retrieves the cached data given a key
	Get(key string) ([]byte, error)
	// Put stores a new value in the cache at a given key
	Put(key string, val []byte) error
}

// CreateCache creates a new generic Cache from a CacheConfig
func CreateCache(config *CacheConfig) (Cache, error) {
	if config != nil {
		if config.Redis != nil {
			Logger.Debug("Using RedisCache configuration")
			cache, err := NewRedisCache(config.Redis)
			if err != nil {
				return nil, err
			}
			return cache, nil
		}
	}
	Logger.Debug("No cache configured, falling back to NilCache implementation")
	return &NilCache{}, nil
}
