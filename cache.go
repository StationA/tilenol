package tilenol

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
	if config.Redis != nil {
		cache, err := NewRedisCache(config.Redis)
		if err != nil {
			return nil, err
		}
		return cache, nil
	}
	// TODO: Instead of return nil, maybe consider implementing a dummy "NilCache"?
	return nil, nil
}
